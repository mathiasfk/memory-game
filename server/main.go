package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"memory-game-server/api"
	"memory-game-server/config"
	"memory-game-server/loghandler"
	"memory-game-server/matchmaking"
	"memory-game-server/powerup"
	"memory-game-server/storage"
	"memory-game-server/ws"
)

func main() {
	// Set default logger so packages using slog during config load get a valid handler.
	slog.SetDefault(slog.New(loghandler.NewCompactHandler(os.Stderr, slog.LevelInfo)))

	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("server/.env"); err2 != nil {
			slog.Info("No .env file found; using environment variables. For local dev, run from server/ or set NEON_AUTH_BASE_URL and WS_PORT.", "tag", "server")
		}
	}

	cfg := config.Load()
	// Apply configured log level.
	slog.SetDefault(slog.New(loghandler.NewCompactHandler(os.Stderr, cfg.SlogLevel())))

	if cfg.NeonAuthBaseURL == "" {
		slog.Info("NEON_AUTH_BASE_URL is not set — WebSocket auth will reject clients with 'Server auth not configured.'", "tag", "auth")
	} else {
		slog.Info("configured", "tag", "auth", "base_url", cfg.NeonAuthBaseURL)
	}

	slog.Info("Configuration",
		"tag", "server",
		"board_rows", cfg.BoardRows, "board_cols", cfg.BoardCols,
		"reveal_duration_ms", cfg.RevealDurationMS, "ws_port", cfg.WSPort)

	// Set up power-up registry (power-ups are earned by matching pairs; use has no point cost)
	registry := powerup.NewRegistry()
	powerup.RegisterAll(registry, &cfg.PowerUps)

	// Game history storage (optional; DATABASE_URL empty = no persistence)
	ctx := context.Background()
	historyStore, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database", "tag", "server", "err", err)
		os.Exit(1)
	}
	if historyStore != nil {
		defer historyStore.Close()
	}

	// Context for graceful shutdown: cancel signals hub and matchmaker to stop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up matchmaker
	mm := matchmaking.NewMatchmaker(cfg, registry, historyStore)
	go mm.Run(ctx)

	// Set up WebSocket hub
	hub := ws.NewHub(cfg, mm)
	go hub.Run(ctx)

	// HTTP handler for WebSocket upgrades
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	})

	// REST API handlers
	apiHandler := api.NewHandler(cfg, historyStore)
	http.HandleFunc("/api/history", apiHandler.History)
	http.HandleFunc("/api/leaderboard", apiHandler.Leaderboard)
	http.HandleFunc("/api/telemetry/metrics", apiHandler.TelemetryMetrics)

	addr := fmt.Sprintf(":%d", cfg.WSPort)
	srv := &http.Server{Addr: addr}
	go func() {
		slog.Info("Memory Game server listening", "tag", "server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server", "tag", "server", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down server...", "tag", "server")
	cancel() // stop hub and matchmaker (no new connections or matches)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server shutdown", "tag", "server", "err", err)
	}
	slog.Info("Server stopped", "tag", "server")
}

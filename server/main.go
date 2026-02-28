package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"memory-game-server/api"
	"memory-game-server/config"
	"memory-game-server/matchmaking"
	"memory-game-server/powerup"
	"memory-game-server/storage"
	"memory-game-server/ws"
)

func main() {
	if err := godotenv.Load(); err != nil {
		if err2 := godotenv.Load("server/.env"); err2 != nil {
			log.Print("No .env file found; using environment variables. For local dev, run from server/ or set NEON_AUTH_BASE_URL and WS_PORT.")
		}
	}

	cfg := config.Load()

	if cfg.NeonAuthBaseURL == "" {
		log.Print("Auth: NEON_AUTH_BASE_URL is not set — WebSocket auth will reject clients with 'Server auth not configured.'")
	} else {
		log.Printf("Auth: configured (base URL: %s)", cfg.NeonAuthBaseURL)
	}

	log.Printf("Configuration: BoardRows=%d, BoardCols=%d, RevealDurationMS=%d, WSPort=%d",
		cfg.BoardRows, cfg.BoardCols, cfg.RevealDurationMS, cfg.WSPort)

	// Set up power-up registry (power-ups are earned by matching pairs; use has no point cost)
	registry := powerup.NewRegistry()
	powerup.RegisterAll(registry, &cfg.PowerUps)

	// Game history storage (optional; DATABASE_URL empty = no persistence)
	ctx := context.Background()
	historyStore, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
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
		log.Printf("Memory Game server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("Shutting down server...")
	cancel() // stop hub and matchmaker (no new connections or matches)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown: %v", err)
	}
	log.Print("Server stopped")
}

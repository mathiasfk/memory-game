package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"memory-game-server/auth"
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

	log.Printf("Configuration: BoardRows=%d, BoardCols=%d, ComboBasePoints=%d, RevealDurationMS=%d, ShuffleCost=%d, WSPort=%d",
		cfg.BoardRows, cfg.BoardCols, cfg.ComboBasePoints, cfg.RevealDurationMS, cfg.PowerUps.Shuffle.Cost, cfg.WSPort)

	// Set up power-up registry
	registry := powerup.NewRegistry()
	registry.Register(&powerup.ShufflePowerUp{CostValue: cfg.PowerUps.Shuffle.Cost})
	registry.Register(&powerup.SecondChancePowerUp{CostValue: cfg.PowerUps.SecondChance.Cost, DurationRounds: cfg.PowerUps.SecondChance.DurationRounds})
	radarRevealSec := cfg.PowerUps.Radar.RevealDurationMS / 1000
	if radarRevealSec < 1 {
		radarRevealSec = 1
	}
	registry.Register(&powerup.RadarPowerUp{CostValue: cfg.PowerUps.Radar.Cost, RevealDuration: radarRevealSec})

	// Game history storage (optional; DATABASE_URL empty = no persistence)
	ctx := context.Background()
	historyStore, err := storage.NewStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if historyStore != nil {
		defer historyStore.Close()
	}

	// Set up matchmaker
	mm := matchmaking.NewMatchmaker(cfg, registry, historyStore)
	go mm.Run()

	// Set up WebSocket hub
	hub := ws.NewHub(cfg, mm)
	go hub.Run()

	// HTTP handler for WebSocket upgrades
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	})

	// GET /api/history — returns game history for the authenticated user (JWT required)
	http.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		// CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// JWT
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "authorization required", http.StatusUnauthorized)
			return
		}
		const prefix = "Bearer "
		if !strings.HasPrefix(authHeader, prefix) {
			http.Error(w, "invalid authorization", http.StatusUnauthorized)
			return
		}
		token := strings.TrimSpace(authHeader[len(prefix):])
		claims, err := auth.ValidateNeonToken(cfg.NeonAuthBaseURL, token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		userID := auth.UserIDFromClaims(claims)
		if userID == "" {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		// Return list (empty if no DB)
		list := []storage.GameRecord{}
		if historyStore != nil {
			list, err = historyStore.ListByUserID(r.Context(), userID)
			if err != nil {
				log.Printf("ListByUserID: %v", err)
				http.Error(w, "failed to load history", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})

	// GET /api/leaderboard — returns global leaderboard ordered by ELO (public)
	http.HandleFunc("/api/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		list := []storage.LeaderboardEntry{}
		if historyStore != nil {
			var err error
			list, err = historyStore.ListLeaderboard(r.Context(), limit, offset)
			if err != nil {
				log.Printf("ListLeaderboard: %v", err)
				http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(list)
	})

	addr := fmt.Sprintf(":%d", cfg.WSPort)
	log.Printf("Memory Game server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

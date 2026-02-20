package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"memory-game-server/config"
	"memory-game-server/matchmaking"
	"memory-game-server/powerup"
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

	// Set up matchmaker
	mm := matchmaking.NewMatchmaker(cfg, registry)
	go mm.Run()

	// Set up WebSocket hub
	hub := ws.NewHub(cfg, mm)
	go hub.Run()

	// HTTP handler for WebSocket upgrades
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	})

	addr := fmt.Sprintf(":%d", cfg.WSPort)
	log.Printf("Memory Game server listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

package main

import (
	"fmt"
	"log"
	"net/http"

	"memory-game-server/config"
	"memory-game-server/matchmaking"
	"memory-game-server/powerup"
	"memory-game-server/ws"
)

func main() {
	cfg := config.Load()

	log.Printf("Configuration: BoardRows=%d, BoardCols=%d, ComboBasePoints=%d, RevealDurationMS=%d, ShuffleCost=%d, WSPort=%d",
		cfg.BoardRows, cfg.BoardCols, cfg.ComboBasePoints, cfg.RevealDurationMS, cfg.PowerUpShuffleCost, cfg.WSPort)

	// Set up power-up registry
	registry := powerup.NewRegistry()
	registry.Register(&powerup.ShufflePowerUp{CostValue: cfg.PowerUpShuffleCost})

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

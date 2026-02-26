package ai

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"memory-game-server/config"
	"memory-game-server/game"
)

type mockPowerUpProvider struct{}

func (m *mockPowerUpProvider) GetPowerUp(id string) (game.PowerUpDef, bool) {
	return game.PowerUpDef{}, false
}

func (m *mockPowerUpProvider) AllPowerUps() []game.PowerUpDef {
	return nil
}

func (m *mockPowerUpProvider) PickArcanaForMatch(n int) []game.PowerUpDef {
	return nil
}

func TestRunExitsOnGameOver(t *testing.T) {
	cfg := &config.Config{AIPairTimeoutSec: 60}
	params := &config.AIParams{Name: "Mnemosyne", DelayMinMS: 10, DelayMaxMS: 20, UseKnownPairChance: 85}

	aiSend := make(chan []byte, 4)
	board := game.NewBoard(2, 2, 0)
	p0 := game.NewPlayer("Human", make(chan []byte, 4))
	p1 := game.NewPlayer("Mnemosyne", aiSend)
	g := game.NewGame("test", cfg, p0, p1, &mockPowerUpProvider{})
	g.Board = board

	go g.Run()

	done := make(chan struct{})
	go func() {
		Run(aiSend, g, 1, params)
		close(done)
	}()

	// Send game_over so the AI exits
	gameOver := map[string]string{"type": "game_over"}
	data, _ := json.Marshal(gameOver)
	aiSend <- data
	close(aiSend)

	select {
	case <-done:
		// AI exited
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after game_over")
	}
}

func TestRunExitsOnClosedChannel(t *testing.T) {
	cfg := config.Defaults()
	params := &cfg.AIProfiles[0]
	aiSend := make(chan []byte, 4)
	board := game.NewBoard(2, 2, 0)
	p0 := game.NewPlayer("Human", make(chan []byte, 4))
	p1 := game.NewPlayer(params.Name, aiSend)
	g := game.NewGame("test", cfg, p0, p1, &mockPowerUpProvider{})
	g.Board = board

	go g.Run()

	close(aiSend)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		Run(aiSend, g, 1, params)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit when channel closed")
	}
}

func TestHiddenIndices(t *testing.T) {
	cards := []game.CardView{
		{Index: 0, State: "hidden"},
		{Index: 1, State: "revealed"},
		{Index: 2, State: "matched"},
		{Index: 3, State: "hidden"},
	}
	hidden := hiddenIndices(cards)
	if len(hidden) != 2 {
		t.Fatalf("expected 2 hidden, got %d", len(hidden))
	}
	if hidden[0] != 0 || hidden[1] != 3 {
		t.Errorf("expected hidden [0 3], got %v", hidden)
	}
}

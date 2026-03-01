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
	params := &config.AIParams{Name: "Mnemosyne", DelayMinMS: 10, DelayMaxMS: 20, UseKnownPairChance: 85, ArcanaRandomness: 0}

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

func TestPairsRemaining(t *testing.T) {
	cards := []game.CardView{
		{Index: 0, State: "hidden"},
		{Index: 1, State: "hidden"},
		{Index: 2, State: "matched"},
		{Index: 3, State: "matched"},
	}
	if got := pairsRemaining(cards); got != 1 {
		t.Errorf("expected 1 pair remaining, got %d", got)
	}
	cards[2].State = "hidden"
	cards[3].State = "hidden"
	if got := pairsRemaining(cards); got != 2 {
		t.Errorf("expected 2 pairs remaining, got %d", got)
	}
}

func TestRandomMatchProb(t *testing.T) {
	if got := randomMatchProb(0); got != 0 {
		t.Errorf("randomMatchProb(0) = %v, want 0", got)
	}
	if got := randomMatchProb(1); got != 1 {
		t.Errorf("randomMatchProb(1) = 1/(2*1-1) = 1, got %v", got)
	}
	// P=18 -> 1/35
	if got := randomMatchProb(18); got <= 0 || got >= 0.03 {
		t.Errorf("randomMatchProb(18) should be 1/35 ≈ 0.0286, got %v", got)
	}
}

func TestHasKnownPair(t *testing.T) {
	memory := map[int]int{0: 5, 1: 5}
	hidden := []int{0, 1, 2, 3}
	if !hasKnownPair(memory, hidden) {
		t.Error("expected true: indices 0,1 are same pairID 5 and both hidden")
	}
	memory[1] = 6
	if hasKnownPair(memory, hidden) {
		t.Error("expected false: no complete pair in memory")
	}
}

func TestEVNoCard(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	state.Cards[0].State = "matched"
	state.Cards[1].State = "matched"
	hidden := hiddenIndices(state.Cards)
	P := pairsRemaining(state.Cards)
	memory := map[int]int{}
	ev := evNoCard(state, memory, hidden, P)
	if ev != randomMatchProb(P) {
		t.Errorf("no known pair: ev should equal randomMatchProb(P), got %v", ev)
	}
	memory[2] = 10
	memory[3] = 10
	ev = evNoCard(state, memory, hidden, P)
	if ev != 1 {
		t.Errorf("known pair: ev should be 1, got %v", ev)
	}
}

func TestEVWithCard_Elemental(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndices(state.Cards)
	P := 18
	// pairID 10 -> (10-6)/3 = 1 -> water
	memory := map[int]int{0: 10, 1: 10}
	ev := evWithCard(state, memory, hidden, "water_elemental", P)
	if ev < 1 {
		t.Errorf("water elemental with known water pair: ev should be >= 1, got %v", ev)
	}
	ev = evWithCard(state, memory, hidden, "fire_elemental", P)
	if ev != 0 {
		t.Errorf("fire elemental with known water pair: ev should be 0, got %v", ev)
	}
}

func TestEVWithCard_Chaos(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndices(state.Cards)
	P := 18
	memory := map[int]int{}
	ev := evWithCard(state, memory, hidden, "chaos", P)
	if ev != randomMatchProb(P) {
		t.Errorf("chaos EV should equal randomMatchProb(P), got %v", ev)
	}
}

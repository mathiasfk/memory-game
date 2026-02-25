package game

import (
	"encoding/json"
	"testing"
	"time"

	"memory-game-server/config"
)

// mockPowerUpProvider is a test double for PowerUpProvider.
// Register power-ups with Register() so AllPowerUps() returns them in deterministic order.
type mockPowerUpProvider struct {
	powerUps map[string]PowerUpDef
	order    []string
}

func newMockPowerUpProvider() *mockPowerUpProvider {
	return &mockPowerUpProvider{
		powerUps: make(map[string]PowerUpDef),
		order:    nil,
	}
}

// Register adds a power-up so it appears in AllPowerUps() in registration order.
func (m *mockPowerUpProvider) Register(id string, def PowerUpDef) {
	m.powerUps[id] = def
	for _, o := range m.order {
		if o == id {
			return
		}
	}
	m.order = append(m.order, id)
}

func (m *mockPowerUpProvider) GetPowerUp(id string) (PowerUpDef, bool) {
	p, ok := m.powerUps[id]
	return p, ok
}

func (m *mockPowerUpProvider) AllPowerUps() []PowerUpDef {
	defs := make([]PowerUpDef, 0, len(m.order))
	for _, id := range m.order {
		defs = append(defs, m.powerUps[id])
	}
	return defs
}

func testConfig() *config.Config {
	return &config.Config{
		BoardRows:        4,
		BoardCols:        4,
		ComboBasePoints:  1,
		RevealDurationMS: 100, // Short for testing
		MaxNameLength:    24,
		WSPort:           8080,
		PowerUps: config.PowerUpsConfig{
			Chaos:        config.ChaosPowerUpConfig{},
			Clairvoyance: config.ClairvoyancePowerUpConfig{},
		},
	}
}

// createTestGame creates a game with deterministic board for testing.
// It returns the game, both player send channels, and starts the game loop.
func createTestGame(cfg *config.Config) (*Game, chan []byte, chan []byte, *mockPowerUpProvider) {
	send0 := make(chan []byte, 100)
	send1 := make(chan []byte, 100)

	p0 := NewPlayer("Alice", send0)
	p1 := NewPlayer("Bob", send1)

	pups := newMockPowerUpProvider()
	g := NewGame("test-1", cfg, p0, p1, pups)

	return g, send0, send1, pups
}

// drainChannel reads all available messages from a channel.
func drainChannel(ch chan []byte) [][]byte {
	var msgs [][]byte
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// waitForMessages waits briefly for messages to arrive, then drains the channel.
func waitForMessages(ch chan []byte, timeout time.Duration) [][]byte {
	var msgs [][]byte
	timer := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		case <-timer:
			// Drain any remaining
			for {
				select {
				case msg := <-ch:
					msgs = append(msgs, msg)
				default:
					return msgs
				}
			}
		}
	}
}

// findPair finds two card indices that form a pair on the board.
func findPair(board *Board) (int, int) {
	pairMap := make(map[int][]int)
	for _, card := range board.Cards {
		if card.State == Hidden {
			pairMap[card.PairID] = append(pairMap[card.PairID], card.Index)
		}
	}
	for _, indices := range pairMap {
		if len(indices) >= 2 {
			return indices[0], indices[1]
		}
	}
	return -1, -1
}

// findNonPair finds two card indices that do NOT form a pair on the board.
func findNonPair(board *Board) (int, int) {
	var hidden []Card
	for _, card := range board.Cards {
		if card.State == Hidden {
			hidden = append(hidden, card)
		}
	}
	for i := 0; i < len(hidden); i++ {
		for j := i + 1; j < len(hidden); j++ {
			if hidden[i].PairID != hidden[j].PairID {
				return hidden[i].Index, hidden[j].Index
			}
		}
	}
	return -1, -1
}

func TestNewGame(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	_ = send0
	_ = send1

	if g.ID != "test-1" {
		t.Errorf("expected ID='test-1', got %q", g.ID)
	}
	if g.CurrentTurn != 0 && g.CurrentTurn != 1 {
		t.Errorf("expected CurrentTurn to be 0 or 1, got %d", g.CurrentTurn)
	}
	if g.TurnPhase != FirstFlip {
		t.Errorf("expected TurnPhase=FirstFlip, got %v", g.TurnPhase)
	}
	if len(g.FlippedIndices) != 0 {
		t.Errorf("expected empty FlippedIndices, got %v", g.FlippedIndices)
	}
	if g.Players[0].Name != "Alice" {
		t.Errorf("expected player 0 name='Alice', got %q", g.Players[0].Name)
	}
	if g.Players[1].Name != "Bob" {
		t.Errorf("expected player 1 name='Bob', got %q", g.Players[1].Name)
	}
}

func TestFlipCard_WrongTurn(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() { g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0} }()

	// Wait for initial state broadcast
	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	// Try to flip as the wrong player
	wrongPlayer := 1 - g.CurrentTurn
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: wrongPlayer, Index: 0}

	time.Sleep(50 * time.Millisecond)

	var ch chan []byte
	if wrongPlayer == 0 {
		ch = send0
	} else {
		ch = send1
	}

	msgs := drainChannel(ch)
	if len(msgs) == 0 {
		t.Fatal("expected error message for wrong turn")
	}

	var errMsg map[string]string
	json.Unmarshal(msgs[0], &errMsg)
	if errMsg["type"] != "error" {
		t.Errorf("expected error message, got type=%q", errMsg["type"])
	}
}

func TestFlipCard_OutOfBounds(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() { g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0} }()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	// Flip out-of-bounds card
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: g.CurrentTurn, Index: 100}
	time.Sleep(50 * time.Millisecond)

	var ch chan []byte
	if g.CurrentTurn == 0 {
		ch = send0
	} else {
		ch = send1
	}

	msgs := drainChannel(ch)
	if len(msgs) == 0 {
		t.Fatal("expected error message for out-of-bounds")
	}

	var errMsg map[string]string
	json.Unmarshal(msgs[0], &errMsg)
	if errMsg["type"] != "error" {
		t.Errorf("expected error message, got type=%q", errMsg["type"])
	}
}

func TestFlipCard_SuccessfulMatch(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Find a matching pair
	idx1, idx2 := findPair(g.Board)
	if idx1 == -1 {
		t.Fatal("could not find a matching pair on the board")
	}

	// Flip first card
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx1}
	time.Sleep(50 * time.Millisecond)

	// Flip second card (matching)
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx2}
	time.Sleep(50 * time.Millisecond)

	// Check that both cards are now matched
	if g.Board.Cards[idx1].State != Matched {
		t.Errorf("expected card[%d] to be Matched, got %v", idx1, g.Board.Cards[idx1].State)
	}
	if g.Board.Cards[idx2].State != Matched {
		t.Errorf("expected card[%d] to be Matched, got %v", idx2, g.Board.Cards[idx2].State)
	}

	// Check combo and score
	player := g.Players[currentPlayer]
	if player.ComboStreak != 1 {
		t.Errorf("expected ComboStreak=1, got %d", player.ComboStreak)
	}
	if player.Score != 1 {
		t.Errorf("expected Score=1, got %d", player.Score)
	}

	// Turn should remain with the same player
	if g.CurrentTurn != currentPlayer {
		t.Errorf("expected turn to remain with player %d after match, got %d", currentPlayer, g.CurrentTurn)
	}

	// Phase should be FirstFlip again
	if g.TurnPhase != FirstFlip {
		t.Errorf("expected TurnPhase=FirstFlip after match, got %v", g.TurnPhase)
	}
}

func TestFlipCard_Mismatch(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Find a non-matching pair
	idx1, idx2 := findNonPair(g.Board)
	if idx1 == -1 {
		t.Fatal("could not find a non-matching pair on the board")
	}

	// Flip first card
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx1}
	time.Sleep(50 * time.Millisecond)

	// Phase should be SecondFlip
	if g.TurnPhase != SecondFlip {
		t.Errorf("expected TurnPhase=SecondFlip, got %v", g.TurnPhase)
	}

	// Flip second card (non-matching)
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx2}
	time.Sleep(50 * time.Millisecond)

	// Phase should be Resolve (waiting for timer)
	if g.TurnPhase != Resolve {
		t.Errorf("expected TurnPhase=Resolve, got %v", g.TurnPhase)
	}

	// Wait for reveal duration + buffer
	time.Sleep(time.Duration(cfg.RevealDurationMS+100) * time.Millisecond)

	// Cards should be hidden again
	if g.Board.Cards[idx1].State != Hidden {
		t.Errorf("expected card[%d] to be Hidden after mismatch, got %v", idx1, g.Board.Cards[idx1].State)
	}
	if g.Board.Cards[idx2].State != Hidden {
		t.Errorf("expected card[%d] to be Hidden after mismatch, got %v", idx2, g.Board.Cards[idx2].State)
	}

	// Turn should switch
	if g.CurrentTurn != 1-currentPlayer {
		t.Errorf("expected turn to switch to player %d after mismatch, got %d", 1-currentPlayer, g.CurrentTurn)
	}

	// Phase should be FirstFlip
	if g.TurnPhase != FirstFlip {
		t.Errorf("expected TurnPhase=FirstFlip after resolve, got %v", g.TurnPhase)
	}

	// Combo should be 0
	if g.Players[currentPlayer].ComboStreak != 0 {
		t.Errorf("expected ComboStreak=0 after mismatch, got %d", g.Players[currentPlayer].ComboStreak)
	}
}

func TestComboScoring(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Find and match multiple pairs to test combo scoring
	pairMap := make(map[int][]int)
	for _, card := range g.Board.Cards {
		if card.State == Hidden {
			pairMap[card.PairID] = append(pairMap[card.PairID], card.Index)
		}
	}

	pairs := make([][2]int, 0)
	for _, indices := range pairMap {
		if len(indices) >= 2 {
			pairs = append(pairs, [2]int{indices[0], indices[1]})
		}
	}

	if len(pairs) < 3 {
		t.Skip("need at least 3 pairs for combo test")
	}

	// Match 3 pairs consecutively
	for i := 0; i < 3; i++ {
		g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: pairs[i][0]}
		time.Sleep(30 * time.Millisecond)
		g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: pairs[i][1]}
		time.Sleep(30 * time.Millisecond)
	}

	player := g.Players[currentPlayer]

	// Expected: combo=3, score = 1+2+3 = 6
	if player.ComboStreak != 3 {
		t.Errorf("expected ComboStreak=3, got %d", player.ComboStreak)
	}
	if player.Score != 6 {
		t.Errorf("expected Score=6, got %d", player.Score)
	}
}

func TestUsePowerUp_Chaos(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, pups := createTestGame(cfg)

	pups.Register("chaos", PowerUpDef{
		ID:          "chaos",
		Name:        "Chaos",
		Description: "Reshuffles all unmatched cards.",
		Cost:        0,
		Apply: func(board *Board, active *Player, opponent *Player, ctx *PowerUpContext) error {
			ShuffleUnmatched(board)
			return nil
		},
	})

	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Give the player one shuffle in hand
	g.Players[currentPlayer].Hand["chaos"] = 1

	// Use the power-up
	g.Actions <- Action{Type: ActionUsePowerUp, PlayerIdx: currentPlayer, PowerUpID: "chaos"}
	time.Sleep(50 * time.Millisecond)

	// Hand should have consumed the power-up
	if g.Players[currentPlayer].Hand["chaos"] != 0 {
		t.Errorf("expected Hand[chaos]=0 after use, got %d", g.Players[currentPlayer].Hand["chaos"])
	}

	// Turn phase should still be FirstFlip (power-up doesn't end turn)
	if g.TurnPhase != FirstFlip {
		t.Errorf("expected TurnPhase=FirstFlip after power-up, got %v", g.TurnPhase)
	}

	// Turn should still be the same player's
	if g.CurrentTurn != currentPlayer {
		t.Errorf("expected turn to remain with player %d after power-up, got %d", currentPlayer, g.CurrentTurn)
	}
}

func TestUsePowerUp_NotInHand(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, pups := createTestGame(cfg)

	pups.Register("chaos", PowerUpDef{
		ID:   "chaos",
		Cost: 0,
		Apply: func(board *Board, active *Player, opponent *Player, ctx *PowerUpContext) error {
			return nil
		},
	})

	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Player has empty hand; try to use chaos
	g.Actions <- Action{Type: ActionUsePowerUp, PlayerIdx: currentPlayer, PowerUpID: "chaos"}
	time.Sleep(50 * time.Millisecond)

	var ch chan []byte
	if currentPlayer == 0 {
		ch = send0
	} else {
		ch = send1
	}

	msgs := drainChannel(ch)
	foundError := false
	for _, msg := range msgs {
		var m map[string]string
		json.Unmarshal(msg, &m)
		if m["type"] == "error" {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error when using power-up not in hand")
	}
}

func TestUsePowerUp_WrongPhase(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, pups := createTestGame(cfg)

	pups.Register("chaos", PowerUpDef{
		ID:   "chaos",
		Cost: 0,
		Apply: func(board *Board, active *Player, opponent *Player, ctx *PowerUpContext) error {
			return nil
		},
	})

	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn
	g.Players[currentPlayer].Hand["chaos"] = 1

	// Flip first card to move to SecondFlip phase
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: 0}
	time.Sleep(50 * time.Millisecond)

	var ch chan []byte
	if currentPlayer == 0 {
		ch = send0
	} else {
		ch = send1
	}
	drainChannel(ch)

	// Try to use power-up in SecondFlip phase
	g.Actions <- Action{Type: ActionUsePowerUp, PlayerIdx: currentPlayer, PowerUpID: "chaos"}
	time.Sleep(50 * time.Millisecond)

	msgs := drainChannel(ch)
	foundError := false
	for _, msg := range msgs {
		var m map[string]string
		json.Unmarshal(msg, &m)
		if m["type"] == "error" {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error when using power-up after flipping a card")
	}
}

func TestDisconnect(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	// Player 0 disconnects
	g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}
	time.Sleep(50 * time.Millisecond)

	// Player 1 should receive opponent_disconnected
	msgs := drainChannel(send1)
	foundDisconnect := false
	for _, msg := range msgs {
		var m map[string]string
		json.Unmarshal(msg, &m)
		if m["type"] == "opponent_disconnected" {
			foundDisconnect = true
			break
		}
	}

	if !foundDisconnect {
		t.Error("expected opponent_disconnected message for remaining player")
	}
}

func TestGameState_PairIdHiddenForHiddenCards(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	// Wait for initial state
	time.Sleep(50 * time.Millisecond)

	// Check the initial state message
	var ch chan []byte
	if g.CurrentTurn == 0 {
		ch = send0
	} else {
		ch = send1
	}

	msgs := drainChannel(ch)
	if len(msgs) == 0 {
		t.Fatal("expected at least one game state message")
	}

	var state GameStateMsg
	json.Unmarshal(msgs[len(msgs)-1], &state)

	for _, card := range state.Cards {
		if card.State == "hidden" && card.PairID != nil {
			t.Errorf("hidden card at index %d should not have pairId, but has %d", card.Index, *card.PairID)
		}
	}

	// Drain other channel too
	drainChannel(send0)
	drainChannel(send1)
}

func TestTurnPhaseString(t *testing.T) {
	tests := []struct {
		phase    TurnPhase
		expected string
	}{
		{FirstFlip, "first_flip"},
		{SecondFlip, "second_flip"},
		{Resolve, "resolve"},
	}

	for _, test := range tests {
		if got := test.phase.String(); got != test.expected {
			t.Errorf("TurnPhase(%d).String() = %q, want %q", test.phase, got, test.expected)
		}
	}
}

func TestFlipCard_AlreadyRevealed(t *testing.T) {
	cfg := testConfig()
	g, send0, send1, _ := createTestGame(cfg)
	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Flip a card
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: 0}
	time.Sleep(50 * time.Millisecond)

	var ch chan []byte
	if currentPlayer == 0 {
		ch = send0
	} else {
		ch = send1
	}
	drainChannel(ch)

	// Try to flip the same card again
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: 0}
	time.Sleep(50 * time.Millisecond)

	msgs := drainChannel(ch)
	foundError := false
	for _, msg := range msgs {
		var m map[string]string
		json.Unmarshal(msg, &m)
		if m["type"] == "error" {
			foundError = true
			break
		}
	}

	if !foundError {
		t.Error("expected error when flipping the same card twice in one turn")
	}
}

func TestMatchGrantsPowerUp(t *testing.T) {
	cfg := testConfig()
	send0 := make(chan []byte, 100)
	send1 := make(chan []byte, 100)
	p0 := NewPlayer("Alice", send0)
	p1 := NewPlayer("Bob", send1)
	pups := newMockPowerUpProvider()
	pups.Register("chaos", PowerUpDef{
		ID:   "chaos",
		Apply: func(board *Board, active *Player, opponent *Player, ctx *PowerUpContext) error { return nil },
	})
	g := NewGame("match-grants-test", cfg, p0, p1, pups)

	go g.Run()
	defer func() {
		select {
		case g.Actions <- Action{Type: ActionDisconnect, PlayerIdx: 0}:
		default:
		}
	}()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Find the two cards with pairId 0
	var pair0Indices []int
	for _, card := range g.Board.Cards {
		if card.PairID == 0 {
			pair0Indices = append(pair0Indices, card.Index)
		}
	}
	if len(pair0Indices) < 2 {
		t.Fatal("board has no pair with pairId 0 (need at least 4 cards)")
	}
	idx0, idx1 := pair0Indices[0], pair0Indices[1]

	if g.Players[currentPlayer].Hand == nil {
		g.Players[currentPlayer].Hand = make(map[string]int)
	}
	if g.Players[currentPlayer].Hand["chaos"] != 0 {
		t.Fatalf("expected initial Hand[chaos]=0, got %d", g.Players[currentPlayer].Hand["chaos"])
	}

	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx0}
	time.Sleep(30 * time.Millisecond)
	g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: idx1}
	time.Sleep(50 * time.Millisecond)

	if g.Players[currentPlayer].Hand["chaos"] != 1 {
		t.Errorf("expected Hand[chaos]=1 after matching pairId 0, got %d", g.Players[currentPlayer].Hand["chaos"])
	}
}

func TestFullGame(t *testing.T) {
	// Use a small 2x2 board for a quick full game
	cfg := &config.Config{
		BoardRows:          2,
		BoardCols:          2,
		ComboBasePoints:    1,
		RevealDurationMS:   50,
		MaxNameLength:      24,
		WSPort:             8080,
		PowerUps: config.PowerUpsConfig{
			Chaos:        config.ChaosPowerUpConfig{},
			Clairvoyance: config.ClairvoyancePowerUpConfig{},
		},
	}

	send0 := make(chan []byte, 100)
	send1 := make(chan []byte, 100)

	p0 := NewPlayer("Alice", send0)
	p1 := NewPlayer("Bob", send1)

	pups := newMockPowerUpProvider()
	g := NewGame("full-game-test", cfg, p0, p1, pups)
	go g.Run()

	time.Sleep(50 * time.Millisecond)
	drainChannel(send0)
	drainChannel(send1)

	currentPlayer := g.CurrentTurn

	// Find all pairs and match them
	pairMap := make(map[int][]int)
	for _, card := range g.Board.Cards {
		pairMap[card.PairID] = append(pairMap[card.PairID], card.Index)
	}

	for _, indices := range pairMap {
		g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: indices[0]}
		time.Sleep(30 * time.Millisecond)
		g.Actions <- Action{Type: ActionFlipCard, PlayerIdx: currentPlayer, Index: indices[1]}
		time.Sleep(30 * time.Millisecond)
	}

	// Wait for game to end
	select {
	case <-g.Done:
		// Game finished successfully
	case <-time.After(2 * time.Second):
		t.Fatal("game did not finish in time")
	}

	// Check that the game is finished
	if !g.Finished {
		t.Error("expected game to be finished")
	}

	// Check game over message was sent
	msgs0 := drainChannel(send0)
	msgs1 := drainChannel(send1)

	allMsgs := append(msgs0, msgs1...)
	foundGameOver := false
	for _, msg := range allMsgs {
		var m map[string]interface{}
		json.Unmarshal(msg, &m)
		if m["type"] == "game_over" {
			foundGameOver = true
			break
		}
	}

	if !foundGameOver {
		t.Error("expected game_over message")
	}
}

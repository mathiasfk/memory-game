package ai

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"memory-game-server/ai/heuristic"
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
	if got := heuristic.RandomMatchProb(0); got != 0 {
		t.Errorf("RandomMatchProb(0) = %v, want 0", got)
	}
	if got := heuristic.RandomMatchProb(1); got != 1 {
		t.Errorf("RandomMatchProb(1) = 1/(2*1-1) = 1, got %v", got)
	}
	// P=18 -> 1/35
	if got := heuristic.RandomMatchProb(18); got <= 0 || got >= 0.03 {
		t.Errorf("RandomMatchProb(18) should be 1/35 ≈ 0.0286, got %v", got)
	}
}

func TestHasKnownPair(t *testing.T) {
	memory := map[int]int{0: 5, 1: 5}
	hidden := []int{0, 1, 2, 3}
	if !heuristic.HasKnownPair(memory, hidden) {
		t.Error("expected true: indices 0,1 are same pairID 5 and both hidden")
	}
	memory[1] = 6
	if heuristic.HasKnownPair(memory, hidden) {
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
	if ev != heuristic.RandomMatchProb(P) {
		t.Errorf("no known pair: ev should equal RandomMatchProb(P), got %v", ev)
	}
	memory[2] = 10
	memory[3] = 10
	ev = evNoCard(state, memory, hidden, P)
	// With known pair: 1 point + EV of bonus turn (one flip with P-1 pairs)
	wantKnown := 1 + heuristic.RandomMatchProb(P-1)
	if ev != wantKnown {
		t.Errorf("known pair: ev should be 1 + randomMatchProb(P-1) = %v, got %v", wantKnown, ev)
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
	if ev != heuristic.RandomMatchProb(P) {
		t.Errorf("chaos EV should equal RandomMatchProb(P), got %v", ev)
	}
}

func TestEVWithCard_Elemental_Partial(t *testing.T) {
	// Ex1: 7 pairs total, 6 arcana → 1 normal pair (pairID 6 = fire). 1 tile known → elemental reveals the only other fire tile → guaranteed match
	state := &game.GameStateMsg{Cards: make([]game.CardView, 14), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndices(state.Cards)
	P := 7
	memory := map[int]int{0: 6} // pairID 6 = fire, only one tile in memory → partial
	ev := evWithCard(state, memory, hidden, "fire_elemental", P)
	// 1 fire pair: otherTiles = 2*1-1 = 1 → matchProb = 1, then bonus turn
	if ev < 1 {
		t.Errorf("elemental with 1 fire pair and 1 known tile: EV should be >= 1 (guaranteed match), got %v", ev)
	}

	// Ex2: 18 pairs, 2 fire pairs remaining (one fire pair already matched), 1 tile known → 1/3 chance among 3 other fire tiles
	state = &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	state.Cards[12].State = "matched"
	state.Cards[13].State = "matched"
	pairID6 := 6
	state.Cards[12].PairID = &pairID6
	state.Cards[13].PairID = &pairID6
	hidden = hiddenIndices(state.Cards)
	P = 17
	memory = map[int]int{14: 6} // one fire tile known (pairID 6), not the matched one
	ev = evWithCard(state, memory, hidden, "fire_elemental", P)
	// 2 fire pairs remaining: otherTiles = 2*2-1 = 3, matchProb = 1/3
	wantBase := 1.0 / 3.0
	if ev < wantBase {
		t.Errorf("elemental with 2 fire pairs and 1 known tile: EV should be >= 1/3, got %v", ev)
	}
}

func TestPickPair_UsesHighlightWhenPresent(t *testing.T) {
	hidden := []int{0, 1, 2, 3, 4, 5}
	hiddenHighlighted := []int{2, 4} // e.g. after fire elemental
	memory := map[int]int{}
	// With highlight, first card should be one of the highlighted (so we can use elemental result)
	for i := 0; i < 20; i++ {
		first, _, _ := pickPair(memory, hidden, false, hiddenHighlighted, nil)
		if first != 2 && first != 4 {
			t.Errorf("pickPair with highlight should return one of %v, got %d", hiddenHighlighted, first)
		}
	}
	// Without highlight, any hidden is allowed
	first, _, _ := pickPair(memory, hidden, false, nil, nil)
	if first < 0 || first > 5 {
		t.Errorf("pickPair without highlight should return a hidden index, got %d", first)
	}
}

func TestPickSecondCard_UsesHighlightWhenPresent(t *testing.T) {
	hidden := []int{0, 1, 2} // firstIdx was 3 (revealed), so not in hidden
	hiddenHighlighted := []int{0, 1} // the two other tiles of that element still hidden
	memory := map[int]int{}
	// With highlight, second should be one of the highlighted
	for i := 0; i < 20; i++ {
		second, _ := pickSecondCard(memory, hidden, 3, false, hiddenHighlighted, nil, nil)
		if second != 0 && second != 1 {
			t.Errorf("pickSecondCard with highlight should return one of %v, got %d", hiddenHighlighted, second)
		}
	}
	// Without highlight, any hidden except firstIdx
	second, _ := pickSecondCard(memory, hidden, 0, false, nil, nil, nil)
	if second < 0 || second == 0 {
		t.Errorf("pickSecondCard without highlight should return a hidden index != firstIdx, got %d", second)
	}
}

func TestPickPair_PrefersElementMemoryWhenNoHighlight(t *testing.T) {
	hidden := []int{0, 1, 2, 3, 4, 5}
	memory := map[int]int{}
	// We know indices 1 and 3 are fire (e.g. from a previous turn's elemental)
	hiddenByElement := map[string][]int{"fire": {1, 3}}
	for i := 0; i < 20; i++ {
		first, _, _ := pickPair(memory, hidden, false, nil, hiddenByElement)
		if first != 1 && first != 3 {
			t.Errorf("pickPair with element memory should prefer same-element tiles, got %d (expected 1 or 3)", first)
		}
	}
}

func TestPickSecondCard_PrefersSameElementFromElementMemory(t *testing.T) {
	hidden := []int{0, 1, 2}   // firstIdx was 3 (already flipped)
	elementMemory := map[int]string{3: "fire"} // we know the flipped card is fire (e.g. from highlight)
	hiddenByElement := map[string][]int{"fire": {0, 1}} // the other fire tiles still hidden
	memory := map[int]int{}
	for i := 0; i < 20; i++ {
		second, _ := pickSecondCard(memory, hidden, 3, false, nil, elementMemory, hiddenByElement)
		if second != 0 && second != 1 {
			t.Errorf("pickSecondCard with element memory should return one of same element {0,1}, got %d", second)
		}
	}
}

func TestHiddenIndicesByElement(t *testing.T) {
	elementMemory := map[int]string{0: "fire", 1: "fire", 2: "water"}
	hidden := []int{0, 1, 2, 3}
	byElem := hiddenIndicesByElement(elementMemory, hidden)
	if len(byElem["fire"]) != 2 || len(byElem["water"]) != 1 {
		t.Errorf("hiddenIndicesByElement: expected fire=[0,1] water=[2], got %v", byElem)
	}
	// Index 3 is hidden but not in elementMemory, so it does not appear in any element list
	if len(byElem["air"]) != 0 {
		t.Errorf("hiddenIndicesByElement: air should be empty or absent, got %v", byElem["air"])
	}
}

func TestEVWithCard_Clairvoyance(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndices(state.Cards)
	P := 18
	memory := map[int]int{}
	ev := evWithCard(state, memory, hidden, "clairvoyance", P)
	if ev < 0 {
		t.Errorf("evWithCard(clairvoyance) should return positive EV, got %v", ev)
	}
	// Clairvoyance EV for P=18 should be positive (expected pairs from revealing 9 cards; ~1.03 for P=18)
	if ev <= 0 || ev > 5 {
		t.Errorf("evWithCard(clairvoyance) for P=18 want positive EV <= 5, got %v", ev)
	}
	// Unregistered power-up returns -1
	if evWithCard(state, memory, hidden, "oblivion", P) != -1 {
		t.Errorf("evWithCard(oblivion) should return -1")
	}
}

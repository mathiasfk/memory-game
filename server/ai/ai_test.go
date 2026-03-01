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
	// With known pair: 1 point + EV of bonus turn (one flip with P-1 pairs)
	wantKnown := 1 + randomMatchProb(P-1)
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
	if ev != randomMatchProb(P) {
		t.Errorf("chaos EV should equal randomMatchProb(P), got %v", ev)
	}
}

func TestPairsOfElementRemaining(t *testing.T) {
	// 18 pairs total, 6 arcana → 12 normal → 3 per element
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	if got := pairsOfElementRemaining(state, "fire"); got != 3 {
		t.Errorf("no matched: expected 3 fire pairs remaining, got %d", got)
	}
	// Match one fire pair (pairID 6 = first normal = fire)
	state.Cards[0].State = "matched"
	state.Cards[1].State = "matched"
	pairID6 := 6
	state.Cards[0].PairID = &pairID6
	state.Cards[1].PairID = &pairID6
	if got := pairsOfElementRemaining(state, "fire"); got != 2 {
		t.Errorf("one fire pair matched: expected 2 fire pairs remaining, got %d", got)
	}
}

func TestHasPartialKnownOfElement(t *testing.T) {
	hidden := []int{0, 1, 2, 3}
	arcanaPairs := 6
	// pairID 6 = fire. One tile known (index 0), not a full pair → partial
	memory := map[int]int{0: 6}
	if !hasPartialKnownOfElement(memory, hidden, arcanaPairs, "fire") {
		t.Error("expected partial known of fire (one tile)")
	}
	// Full pair known (0 and 1 both pairID 6) → not partial
	memory[1] = 6
	if hasPartialKnownOfElement(memory, hidden, arcanaPairs, "fire") {
		t.Error("expected not partial when full pair known")
	}
	// No tile of fire in memory → not partial
	memory = map[int]int{0: 9} // 9 = water
	if hasPartialKnownOfElement(memory, hidden, arcanaPairs, "fire") {
		t.Error("expected not partial when no fire in memory")
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

func TestExpectedPairsFromReveal(t *testing.T) {
	// P=5, k=9: 2P=10 cards, E = 5 * C(8,7)/C(10,9) = 5*8/10 = 4
	got := expectedPairsFromReveal(5, 9)
	if got < 3.9 || got > 4.1 {
		t.Errorf("expectedPairsFromReveal(5, 9) want ~4, got %v", got)
	}
	// P=6, k=9: E ≈ 3.27
	got = expectedPairsFromReveal(6, 9)
	if got < 3.1 || got > 3.5 {
		t.Errorf("expectedPairsFromReveal(6, 9) want ~3.27, got %v", got)
	}
	// P=8, k=9: E ≈ 2.4
	got = expectedPairsFromReveal(8, 9)
	if got < 2.2 || got > 2.6 {
		t.Errorf("expectedPairsFromReveal(8, 9) want ~2.4, got %v", got)
	}
	// Degenerate: 2*P < k
	if expectedPairsFromReveal(1, 9) != 0 {
		t.Errorf("expectedPairsFromReveal(1, 9) want 0 (2*1 < 9)")
	}
}

func TestEvClairvoyance(t *testing.T) {
	ev := evClairvoyance(6)
	if ev < 3 || ev > 4 {
		t.Errorf("evClairvoyance(6) want ~3.27, got %v", ev)
	}
	if evClairvoyance(2) != 0 {
		t.Errorf("evClairvoyance(2) want 0 (2*2=4 < 9)")
	}
}

func TestRadarRegionIndices(t *testing.T) {
	// 4x4 board: center at index 5 (row 1, col 1) -> full 3x3 = 9 indices
	got := radarRegionIndices(5, 4, 4)
	if len(got) != 9 {
		t.Errorf("radarRegionIndices(5, 4, 4) want 9, got %d: %v", len(got), got)
	}
	// Corner 0 (row 0, col 0) -> 2x2 = 4 indices
	got = radarRegionIndices(0, 4, 4)
	if len(got) != 4 {
		t.Errorf("radarRegionIndices(0, 4, 4) corner want 4, got %d: %v", len(got), got)
	}
	// Edge index 1 (row 0, col 1) -> 2x3 = 6 indices
	got = radarRegionIndices(1, 4, 4)
	if len(got) != 6 {
		t.Errorf("radarRegionIndices(1, 4, 4) edge want 6, got %d: %v", len(got), got)
	}
	// 6x6 board: center at 14 (row 2, col 2) -> full 3x3
	got = radarRegionIndices(14, 6, 6)
	if len(got) != 9 {
		t.Errorf("radarRegionIndices(14, 6, 6) want 9, got %d", len(got))
	}
	// Match game.RadarRegionIndices for 6x6 center 14 (row 2, col 2)
	board := game.NewBoard(6, 6, 0)
	gameRegion := game.RadarRegionIndices(board, 14)
	aiRegion := radarRegionIndices(14, 6, 6)
	if len(gameRegion) != len(aiRegion) {
		t.Errorf("radarRegionIndices(14,6,6) len %d vs game.RadarRegionIndices %d", len(aiRegion), len(gameRegion))
	}
	for i, idx := range gameRegion {
		if aiRegion[i] != idx {
			t.Errorf("radarRegionIndices(14,6,6) at %d: want %d got %d", i, idx, aiRegion[i])
		}
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
	if ev != evClairvoyance(P) {
		t.Errorf("evWithCard(clairvoyance) should equal evClairvoyance(P), got %v want %v", ev, evClairvoyance(P))
	}
	// Unknown power-up still returns -1
	if evWithCard(state, memory, hidden, "oblivion", P) != -1 {
		t.Errorf("evWithCard(oblivion) should return -1")
	}
}

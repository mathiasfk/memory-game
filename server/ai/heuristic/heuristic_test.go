package heuristic

import (
	"testing"

	"memory-game-server/game"
)

func TestEV_Clairvoyance(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	P := 18
	memory := map[int]int{}
	ev := EV("clairvoyance", state, memory, hidden, P)
	if ev <= 0 {
		t.Errorf("EV(clairvoyance) should be positive, got %v", ev)
	}
	// P=6 with 12 cards: EV ~3.27
	state6 := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state6.Cards {
		state6.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden6 := hiddenIndicesForTest(state6.Cards)
	ev6 := EV("clairvoyance", state6, memory, hidden6, 6)
	if ev6 < 3 || ev6 > 4 {
		t.Errorf("EV(clairvoyance) for P=6 want ~3.27, got %v", ev6)
	}
}

func TestEV_UnregisteredReturnsNegative(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	hidden := []int{0, 1, 2}
	memory := map[int]int{}
	if got := EV("oblivion", state, memory, hidden, 18); got != -1 {
		t.Errorf("EV(oblivion) should return -1, got %v", got)
	}
}

func TestPickTarget_Clairvoyance(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 36), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	memory := map[int]int{}
	idx := PickTarget("clairvoyance", state, memory, hidden, 6, 6)
	if idx < 0 || idx >= 36 {
		t.Errorf("PickTarget(clairvoyance) should return valid index, got %d", idx)
	}
}

func TestPickTarget_ChaosReturnsNegative(t *testing.T) {
	state := &game.GameStateMsg{}
	idx := PickTarget("chaos", state, nil, nil, 6, 6)
	if idx != -1 {
		t.Errorf("PickTarget(chaos) should return -1, got %d", idx)
	}
}

func TestEV_Leech_WithKnownPair(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	// Known pair: indices 0 and 1 both have pairID 0 in memory
	memory := map[int]int{0: 0, 1: 0}
	P := 6
	ev := EV("leech", state, memory, hidden, P)
	if ev <= 0 {
		t.Errorf("EV(leech) with known pair should be positive, got %v", ev)
	}
	// With P=1 (last pair), EV should be exactly 1
	memory1 := map[int]int{0: 0, 1: 0}
	hidden1 := []int{0, 1}
	ev1 := EV("leech", state, memory1, hidden1, 1)
	if ev1 != 1 {
		t.Errorf("EV(leech) with known pair and P=1 want 1, got %v", ev1)
	}
}

func TestEV_Leech_WithoutKnownPair(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	memory := map[int]int{}
	P := 6
	ev := EV("leech", state, memory, hidden, P)
	if ev != 0 {
		t.Errorf("EV(leech) without known pair should be 0, got %v", ev)
	}
	// Partial knowledge (only one card of a pair known) is not a known pair
	memoryPartial := map[int]int{0: 0}
	evPartial := EV("leech", state, memoryPartial, hidden, P)
	if evPartial != 0 {
		t.Errorf("EV(leech) with only one card of pair known should be 0, got %v", evPartial)
	}
}

func TestEV_BloodPact_WithThreeKnownPairs(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	// Three known pairs: (0,1)=pair0, (2,3)=pair1, (4,5)=pair2
	memory := map[int]int{0: 0, 1: 0, 2: 1, 3: 1, 4: 2, 5: 2}
	P := 6
	ev := EV("blood_pact", state, memory, hidden, P)
	if ev <= 0 {
		t.Errorf("EV(blood_pact) with 3 known pairs should be positive (5), got %v", ev)
	}
	if ev != 5 {
		t.Errorf("EV(blood_pact) with 3 known pairs want 5, got %v", ev)
	}
}

func TestEV_BloodPact_WithTwoKnownPairs(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	memory := map[int]int{0: 0, 1: 0, 2: 1, 3: 1}
	P := 6
	ev := EV("blood_pact", state, memory, hidden, P)
	if ev != 0 {
		t.Errorf("EV(blood_pact) with 2 known pairs should be 0 (do not risk), got %v", ev)
	}
}

func TestEV_BloodPact_WithoutKnownPair(t *testing.T) {
	state := &game.GameStateMsg{Cards: make([]game.CardView, 12), ArcanaPairs: 6}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	memory := map[int]int{}
	P := 6
	ev := EV("blood_pact", state, memory, hidden, P)
	if ev != 0 {
		t.Errorf("EV(blood_pact) without known pair should be 0, got %v", ev)
	}
	// Partial knowledge (only one card per pair) is not enough
	memoryPartial := map[int]int{0: 0, 2: 1, 4: 2}
	evPartial := EV("blood_pact", state, memoryPartial, hidden, P)
	if evPartial != 0 {
		t.Errorf("EV(blood_pact) with only partial pairs should be 0, got %v", evPartial)
	}
}

func TestEV_Necromancy_WhenBehindAndHasRevivedPairs(t *testing.T) {
	// 12 cards: 4 matched (pairs 1 and 2), 8 hidden. Necromancy is pair 0, not matched. AI behind.
	state := &game.GameStateMsg{
		Cards:          make([]game.CardView, 12),
		ArcanaPairs:    6,
		PairIDToPowerUp: map[int]string{0: "necromancy", 1: "chaos", 2: "clairvoyance"},
		You:            game.PlayerView{Score: 0},
		Opponent:       game.PlayerView{Score: 2},
	}
	for i := 0; i < 4; i++ {
		state.Cards[i] = game.CardView{Index: i, State: "matched", PairID: intPtr(1 + i/2)}
	}
	for i := 4; i < 12; i++ {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	memory := map[int]int{}
	P := 4 // 8 hidden = 4 pairs remaining
	ev := EV("necromancy", state, memory, hidden, P)
	if ev <= 0 {
		t.Errorf("EV(necromancy) when behind with revived pairs should be positive, got %v", ev)
	}
}

func TestEV_Necromancy_WhenTiedOrAhead_ReturnsNegative(t *testing.T) {
	state := &game.GameStateMsg{
		Cards:           make([]game.CardView, 12),
		ArcanaPairs:     6,
		PairIDToPowerUp: map[int]string{0: "necromancy"},
		You:             game.PlayerView{Score: 2},
		Opponent:       game.PlayerView{Score: 2},
	}
	for i := 0; i < 4; i++ {
		state.Cards[i] = game.CardView{Index: i, State: "matched", PairID: intPtr(1)}
	}
	for i := 4; i < 12; i++ {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	ev := EV("necromancy", state, nil, hidden, 4)
	if ev != -1 {
		t.Errorf("EV(necromancy) when tied should be -1, got %v", ev)
	}
	state.You.Score = 3
	evAhead := EV("necromancy", state, nil, hidden, 4)
	if evAhead != -1 {
		t.Errorf("EV(necromancy) when ahead should be -1, got %v", evAhead)
	}
}

func TestEV_Necromancy_WhenNoMatchedToRevive_ReturnsNegative(t *testing.T) {
	state := &game.GameStateMsg{
		Cards:           make([]game.CardView, 12),
		ArcanaPairs:     6,
		PairIDToPowerUp: map[int]string{0: "necromancy"},
		You:             game.PlayerView{Score: 0},
		Opponent:       game.PlayerView{Score: 2},
	}
	for i := range state.Cards {
		state.Cards[i] = game.CardView{Index: i, State: "hidden"}
	}
	hidden := hiddenIndicesForTest(state.Cards)
	ev := EV("necromancy", state, nil, hidden, 6)
	if ev != -1 {
		t.Errorf("EV(necromancy) when no matched to revive should be -1, got %v", ev)
	}
}

func TestEV_Necromancy_WhenNecromancyPairIsMatched_ExcludesItFromRevived(t *testing.T) {
	// 6 cards: all matched (3 pairs). Pair 0 = Necromancy. So revivedPairs = 2 (only pairs 1 and 2).
	state := &game.GameStateMsg{
		Cards:           make([]game.CardView, 6),
		ArcanaPairs:     6,
		PairIDToPowerUp: map[int]string{0: "necromancy", 1: "chaos", 2: "clairvoyance"},
		You:             game.PlayerView{Score: 0},
		Opponent:       game.PlayerView{Score: 2},
	}
	state.Cards[0] = game.CardView{Index: 0, State: "matched", PairID: intPtr(0)}
	state.Cards[1] = game.CardView{Index: 1, State: "matched", PairID: intPtr(0)}
	state.Cards[2] = game.CardView{Index: 2, State: "matched", PairID: intPtr(1)}
	state.Cards[3] = game.CardView{Index: 3, State: "matched", PairID: intPtr(1)}
	state.Cards[4] = game.CardView{Index: 4, State: "matched", PairID: intPtr(2)}
	state.Cards[5] = game.CardView{Index: 5, State: "matched", PairID: intPtr(2)}
	hidden := hiddenIndicesForTest(state.Cards) // empty
	ev := EV("necromancy", state, nil, hidden, 0)
	if ev <= 0 {
		t.Errorf("EV(necromancy) when behind with 2 revived pairs (Necromancy pair excluded) should be positive, got %v", ev)
	}
}

func intPtr(i int) *int { return &i }

func hiddenIndicesForTest(cards []game.CardView) []int {
	var out []int
	for _, c := range cards {
		if c.State == "hidden" {
			out = append(out, c.Index)
		}
	}
	return out
}

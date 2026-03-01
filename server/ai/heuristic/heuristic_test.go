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

func hiddenIndicesForTest(cards []game.CardView) []int {
	var out []int
	for _, c := range cards {
		if c.State == "hidden" {
			out = append(out, c.Index)
		}
	}
	return out
}

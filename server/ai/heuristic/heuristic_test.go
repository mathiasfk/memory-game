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

func hiddenIndicesForTest(cards []game.CardView) []int {
	var out []int
	for _, c := range cards {
		if c.State == "hidden" {
			out = append(out, c.Index)
		}
	}
	return out
}

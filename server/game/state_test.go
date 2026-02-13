package game

import (
	"encoding/json"
	"testing"
)

func TestBuildCardViews_HiddenCardsOmitPairID(t *testing.T) {
	board := NewBoard(2, 2)

	views := BuildCardViews(board)

	for _, cv := range views {
		if cv.State != "hidden" {
			t.Errorf("expected state 'hidden', got %q", cv.State)
		}
		if cv.PairID != nil {
			t.Errorf("hidden card should not have pairId, but got %d", *cv.PairID)
		}
	}
}

func TestBuildCardViews_RevealedCardsIncludePairID(t *testing.T) {
	board := NewBoard(2, 2)
	board.Cards[0].State = Revealed

	views := BuildCardViews(board)

	if views[0].PairID == nil {
		t.Error("revealed card should have pairId")
	}
	if *views[0].PairID != board.Cards[0].PairID {
		t.Errorf("expected pairId=%d, got %d", board.Cards[0].PairID, *views[0].PairID)
	}
}

func TestBuildCardViews_MatchedCardsIncludePairID(t *testing.T) {
	board := NewBoard(2, 2)
	board.Cards[0].State = Matched

	views := BuildCardViews(board)

	if views[0].PairID == nil {
		t.Error("matched card should have pairId")
	}
}

func TestBuildPlayerView(t *testing.T) {
	p := &Player{Name: "Alice", Score: 5, ComboStreak: 2}
	v := BuildPlayerView(p)

	if v.Name != "Alice" {
		t.Errorf("expected Name='Alice', got %q", v.Name)
	}
	if v.Score != 5 {
		t.Errorf("expected Score=5, got %d", v.Score)
	}
	if v.ComboStreak != 2 {
		t.Errorf("expected ComboStreak=2, got %d", v.ComboStreak)
	}
}

func TestCardViewJSON_HiddenOmitsPairID(t *testing.T) {
	cv := CardView{Index: 0, State: "hidden", PairID: nil}
	data, err := json.Marshal(cv)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if _, ok := m["pairId"]; ok {
		t.Error("hidden card JSON should not contain pairId field")
	}
}

func TestCardViewJSON_RevealedIncludesPairID(t *testing.T) {
	pairID := 3
	cv := CardView{Index: 0, State: "revealed", PairID: &pairID}
	data, err := json.Marshal(cv)
	if err != nil {
		t.Fatal(err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if _, ok := m["pairId"]; !ok {
		t.Error("revealed card JSON should contain pairId field")
	}
}

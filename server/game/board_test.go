package game

import (
	"testing"
)

func TestNewBoard(t *testing.T) {
	rows, cols := 4, 4
	board := NewBoard(rows, cols)

	if board.Rows != rows {
		t.Errorf("expected Rows=%d, got %d", rows, board.Rows)
	}
	if board.Cols != cols {
		t.Errorf("expected Cols=%d, got %d", cols, board.Cols)
	}

	totalCards := rows * cols
	if len(board.Cards) != totalCards {
		t.Fatalf("expected %d cards, got %d", totalCards, len(board.Cards))
	}

	// Check that indices are 0..totalCards-1
	for i, card := range board.Cards {
		if card.Index != i {
			t.Errorf("expected card[%d].Index=%d, got %d", i, i, card.Index)
		}
	}

	// Check that all cards start hidden
	for i, card := range board.Cards {
		if card.State != Hidden {
			t.Errorf("expected card[%d].State=Hidden, got %v", i, card.State)
		}
	}

	// Check that there are exactly 2 cards per pair
	pairCount := make(map[int]int)
	for _, card := range board.Cards {
		pairCount[card.PairID]++
	}

	numPairs := totalCards / 2
	if len(pairCount) != numPairs {
		t.Errorf("expected %d distinct pairs, got %d", numPairs, len(pairCount))
	}
	for pairID, count := range pairCount {
		if count != 2 {
			t.Errorf("pair %d has %d cards, expected 2", pairID, count)
		}
	}
}

func TestNewBoardSmall(t *testing.T) {
	board := NewBoard(2, 2)
	if len(board.Cards) != 4 {
		t.Fatalf("expected 4 cards, got %d", len(board.Cards))
	}

	pairCount := make(map[int]int)
	for _, card := range board.Cards {
		pairCount[card.PairID]++
	}
	if len(pairCount) != 2 {
		t.Errorf("expected 2 distinct pairs, got %d", len(pairCount))
	}
}

func TestShuffleUnmatched(t *testing.T) {
	board := NewBoard(4, 4)

	// Mark some cards as matched
	board.Cards[0].State = Matched
	board.Cards[1].State = Matched

	matchedPairID0 := board.Cards[0].PairID
	matchedPairID1 := board.Cards[1].PairID

	ShuffleUnmatched(board)

	// Matched cards should retain their pairID
	if board.Cards[0].PairID != matchedPairID0 {
		t.Errorf("matched card[0] pairID changed from %d to %d", matchedPairID0, board.Cards[0].PairID)
	}
	if board.Cards[1].PairID != matchedPairID1 {
		t.Errorf("matched card[1] pairID changed from %d to %d", matchedPairID1, board.Cards[1].PairID)
	}

	// All cards should still be present (same number of each pairID)
	pairCount := make(map[int]int)
	for _, card := range board.Cards {
		pairCount[card.PairID]++
	}
	for pairID, count := range pairCount {
		if count != 2 {
			t.Errorf("after shuffle, pair %d has %d cards, expected 2", pairID, count)
		}
	}
}

func TestAllMatched(t *testing.T) {
	board := NewBoard(2, 2)

	if AllMatched(board) {
		t.Error("newly created board should not be all matched")
	}

	for i := range board.Cards {
		board.Cards[i].State = Matched
	}

	if !AllMatched(board) {
		t.Error("all cards are matched but AllMatched returned false")
	}
}

func TestCardStateString(t *testing.T) {
	tests := []struct {
		state    CardState
		expected string
	}{
		{Hidden, "hidden"},
		{Revealed, "revealed"},
		{Matched, "matched"},
	}

	for _, test := range tests {
		if got := test.state.String(); got != test.expected {
			t.Errorf("CardState(%d).String() = %q, want %q", test.state, got, test.expected)
		}
	}
}

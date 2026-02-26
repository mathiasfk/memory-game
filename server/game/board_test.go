package game

import (
	"testing"
)

func TestNewBoard(t *testing.T) {
	rows, cols, arcanaPairs := 4, 4, 0
	board := NewBoard(rows, cols, arcanaPairs)

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
	if board.ArcanaPairs != arcanaPairs {
		t.Errorf("expected ArcanaPairs=%d, got %d", arcanaPairs, board.ArcanaPairs)
	}
}

func TestNewBoardSmall(t *testing.T) {
	board := NewBoard(2, 2, 0)
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
	board := NewBoard(4, 4, 0)

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

func TestShufflePairIDsAmongIndices(t *testing.T) {
	board := NewBoard(4, 4, 0)

	// Mark indices 2 and 5 as matched (they will be "revived" in necromancy terms)
	board.Cards[2].State = Matched
	board.Cards[5].State = Matched
	revivedIndices := []int{2, 5}
	pairID2 := board.Cards[2].PairID
	pairID5 := board.Cards[5].PairID

	// Record pairIDs outside revived indices (e.g. 0 and 1)
	orig0 := board.Cards[0].PairID
	orig1 := board.Cards[1].PairID

	ShufflePairIDsAmongIndices(board, revivedIndices)

	// Only indices 2 and 5 should have changed (their pairIDs swapped between them or stayed)
	// The two values that were at 2 and 5 should still be at 2 and 5 (just possibly swapped)
	got2 := board.Cards[2].PairID
	got5 := board.Cards[5].PairID
	if got2 != pairID2 && got2 != pairID5 {
		t.Errorf("index 2 has pairID %d, expected one of {%d, %d}", got2, pairID2, pairID5)
	}
	if got5 != pairID2 && got5 != pairID5 {
		t.Errorf("index 5 has pairID %d, expected one of {%d, %d}", got5, pairID2, pairID5)
	}
	// If the two indices originally had different pairIDs, they must still differ after shuffle.
	if pairID2 != pairID5 && got2 == got5 {
		t.Errorf("indices 2 and 5 had different pairIDs (%d, %d) but have same pairID %d after shuffle", pairID2, pairID5, got2)
	}

	// Positions outside revivedIndices must be unchanged
	if board.Cards[0].PairID != orig0 {
		t.Errorf("index 0 pairID changed from %d to %d", orig0, board.Cards[0].PairID)
	}
	if board.Cards[1].PairID != orig1 {
		t.Errorf("index 1 pairID changed from %d to %d", orig1, board.Cards[1].PairID)
	}
}

func TestAllMatched(t *testing.T) {
	board := NewBoard(2, 2, 0)

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

func TestNewBoard_ElementSamePerPair(t *testing.T) {
	// 6 arcana + 12 normal pairs = 18 pairs = 36 cards
	board := NewBoard(6, 6, 6)
	if len(board.Cards) != 36 {
		t.Fatalf("expected 36 cards, got %d", len(board.Cards))
	}
	pairToElement := make(map[int]string)
	for _, card := range board.Cards {
		if card.PairID >= 6 && card.Element != "" {
			if elem, ok := pairToElement[card.PairID]; ok && elem != card.Element {
				t.Errorf("pair %d has cards with different elements %q and %q", card.PairID, elem, card.Element)
			}
			pairToElement[card.PairID] = card.Element
		}
	}
	// Check we have 3 pairs per element (fire, water, air, earth)
	elementCount := make(map[string]int)
	for _, elem := range pairToElement {
		elementCount[elem]++
	}
	for elem, count := range elementCount {
		if count != 3 {
			t.Errorf("element %q has %d pairs, expected 3", elem, count)
		}
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
		{Removed, "removed"},
	}

	for _, test := range tests {
		if got := test.state.String(); got != test.expected {
			t.Errorf("CardState(%d).String() = %q, want %q", test.state, got, test.expected)
		}
	}
}

package game

import (
	"math/rand"
)

// CardState represents the current state of a card.
type CardState int

const (
	Hidden   CardState = iota
	Revealed
	Matched
)

// String returns the string representation of a CardState.
func (cs CardState) String() string {
	switch cs {
	case Hidden:
		return "hidden"
	case Revealed:
		return "revealed"
	case Matched:
		return "matched"
	default:
		return "unknown"
	}
}

// Card represents a single card on the board.
type Card struct {
	Index  int
	PairID int
	State  CardState
}

// Board represents the game board.
type Board struct {
	Rows  int
	Cols  int
	Cards []Card
}

// NewBoard creates a new board with randomly shuffled pairs.
func NewBoard(rows, cols int) *Board {
	totalCards := rows * cols
	numPairs := totalCards / 2

	// Create pairs: two cards for each pair ID
	cards := make([]Card, totalCards)
	for i := 0; i < numPairs; i++ {
		cards[2*i] = Card{PairID: i, State: Hidden}
		cards[2*i+1] = Card{PairID: i, State: Hidden}
	}

	// Shuffle card positions
	rand.Shuffle(totalCards, func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})

	// Assign indices after shuffle
	for i := range cards {
		cards[i].Index = i
	}

	return &Board{
		Rows:  rows,
		Cols:  cols,
		Cards: cards,
	}
}

// ShuffleUnmatched re-randomizes the positions of all Hidden cards.
// Matched cards remain in place.
func ShuffleUnmatched(board *Board) {
	// Collect indices and pairIDs of hidden cards
	var hiddenIndices []int
	var hiddenPairIDs []int

	for i, card := range board.Cards {
		if card.State == Hidden {
			hiddenIndices = append(hiddenIndices, i)
			hiddenPairIDs = append(hiddenPairIDs, card.PairID)
		}
	}

	// Shuffle the pairIDs
	rand.Shuffle(len(hiddenPairIDs), func(i, j int) {
		hiddenPairIDs[i], hiddenPairIDs[j] = hiddenPairIDs[j], hiddenPairIDs[i]
	})

	// Reassign pairIDs to hidden card positions
	for i, idx := range hiddenIndices {
		board.Cards[idx].PairID = hiddenPairIDs[i]
	}
}

// AllMatched returns true if every card on the board is in the Matched state.
func AllMatched(board *Board) bool {
	for _, card := range board.Cards {
		if card.State != Matched {
			return false
		}
	}
	return true
}

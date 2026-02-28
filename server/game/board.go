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
	Removed
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
	case Removed:
		return "removed"
	default:
		return "unknown"
	}
}

// Element constants for normal cards (empty for arcana).
const (
	ElementFire  = "fire"
	ElementWater = "water"
	ElementAir   = "air"
	ElementEarth = "earth"
)

// Card represents a single card on the board.
type Card struct {
	Index   int
	PairID  int
	State   CardState
	Element string // fire, water, air, earth for normal pairs; empty for arcana
}

// Board represents the game board.
type Board struct {
	Rows         int
	Cols         int
	Cards        []Card
	ArcanaPairs  int // number of arcana pairs (pairIDs 0..ArcanaPairs-1); used to assign Element for normal pairs
}

// elementForNormalPair returns the element for a normal pair (pairID >= arcanaPairs).
// 3 pairs per element: 6,7,8->fire; 9,10,11->water; 12,13,14->air; 15,16,17->earth.
func elementForNormalPair(pairID, arcanaPairs int) string {
	normalPairIndex := pairID - arcanaPairs
	if normalPairIndex < 0 {
		return ""
	}
	elementIndex := normalPairIndex / 3
	switch elementIndex {
	case 0:
		return ElementFire
	case 1:
		return ElementWater
	case 2:
		return ElementAir
	case 3:
		return ElementEarth
	default:
		return ""
	}
}

// assignElementsForNormalPairs sets Element on each card with PairID >= arcanaPairs (same element per pair).
func assignElementsForNormalPairs(board *Board, arcanaPairs int) {
	for i := range board.Cards {
		if board.Cards[i].PairID >= arcanaPairs {
			board.Cards[i].Element = elementForNormalPair(board.Cards[i].PairID, arcanaPairs)
		}
	}
}

// NewBoard creates a new board with randomly shuffled pairs.
// arcanaPairs is the number of arcana pairs (pairIDs 0..arcanaPairs-1); remaining pairs are normal and get an element.
func NewBoard(rows, cols, arcanaPairs int) *Board {
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

	board := &Board{
		Rows:        rows,
		Cols:        cols,
		Cards:       cards,
		ArcanaPairs: arcanaPairs,
	}
	assignElementsForNormalPairs(board, arcanaPairs)
	return board
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
	// Update Element for normal pairs after pairID shuffle
	assignElementsForNormalPairs(board, board.ArcanaPairs)
}

// ShufflePairIDsAmongIndices shuffles the pairIDs of the cards at the given indices
// and reassigns them to the same indices. Only these positions change; the rest of the board is unchanged.
func ShufflePairIDsAmongIndices(board *Board, indices []int) {
	if len(indices) == 0 {
		return
	}
	pairIDs := make([]int, len(indices))
	for i, idx := range indices {
		pairIDs[i] = board.Cards[idx].PairID
	}
	rand.Shuffle(len(pairIDs), func(i, j int) {
		pairIDs[i], pairIDs[j] = pairIDs[j], pairIDs[i]
	})
	for i, idx := range indices {
		board.Cards[idx].PairID = pairIDs[i]
	}
	// Update Element for normal pairs after pairID shuffle
	assignElementsForNormalPairs(board, board.ArcanaPairs)
}

// AllMatched returns true when no card is left to match (all are Matched or Removed).
func AllMatched(board *Board) bool {
	for _, card := range board.Cards {
		if card.State != Matched && card.State != Removed {
			return false
		}
	}
	return true
}

// CountMatchedPairs returns the number of pairs currently in Matched state (each pair counted once).
func CountMatchedPairs(board *Board) int {
	n := 0
	for _, card := range board.Cards {
		if card.State == Matched {
			n++
		}
	}
	return n / 2
}

// RadarRegionIndices returns the board indices of the 3x3 region centered on the given card index.
// Indices are clipped to board bounds, so corners yield 4 indices, edges 6, center 9.
func RadarRegionIndices(board *Board, centerIndex int) []int {
	cols := board.Cols
	rows := board.Rows
	if centerIndex < 0 || centerIndex >= rows*cols {
		return nil
	}
	centerRow := centerIndex / cols
	centerCol := centerIndex % cols
	minR := centerRow - 1
	if minR < 0 {
		minR = 0
	}
	maxR := centerRow + 1
	if maxR >= rows {
		maxR = rows - 1
	}
	minC := centerCol - 1
	if minC < 0 {
		minC = 0
	}
	maxC := centerCol + 1
	if maxC >= cols {
		maxC = cols - 1
	}
	var out []int
	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			out = append(out, r*cols+c)
		}
	}
	return out
}

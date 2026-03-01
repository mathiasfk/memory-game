package ai

import (
	"math/rand"
)

// flipReason describes why the AI chose to flip a given tile (for logging).
const (
	flipReasonKnownPair    = "known_pair"
	flipReasonHighlight    = "highlight_elemental"
	flipReasonElementKnown = "element_known"
	flipReasonUnseen       = "unseen" // roll passed but no known pair/highlight/element; reveal an unseen (never-revealed) tile
	flipReasonRandom       = "random"
)

// pickPair returns (firstIndex, secondIndex, reason). secondIndex may be -1 if we're guessing (we'll pick on next state).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after an elemental).
// hiddenByElement: hidden indices we know per element (from element memory); used to prefer flipping within same element.
// knownIndicesSet: indices ever revealed by any player (from game state); used so "unseen" excludes opponent's reveals.
func pickPair(memory map[int]int, hidden []int, useBestMove bool, hiddenHighlighted []int, hiddenByElement map[string][]int, knownIndicesSet map[int]struct{}) (first, second int, reason string) {
	if !useBestMove {
		if len(hidden) == 0 {
			return -1, -1, flipReasonRandom
		}
		return hidden[rand.Intn(len(hidden))], -1, flipReasonRandom
	}
	// Build pairID -> list of hidden indices we know
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	// Find a complete known pair (both cards still hidden)
	for _, indices := range pairToIndices {
		if len(indices) >= 2 {
			return indices[0], indices[1], flipReasonKnownPair
		}
	}
	// If we have current-turn highlight, pick first from the highlighted set.
	if len(hiddenHighlighted) > 0 {
		first = hiddenHighlighted[rand.Intn(len(hiddenHighlighted))]
		return first, -1, flipReasonHighlight
	}
	// No highlight: prefer flipping among tiles we know are the same element (from previous elemental use), to increase match chance.
	if len(hiddenByElement) > 0 {
		var best []int
		for _, indices := range hiddenByElement {
			if len(indices) >= 2 && (best == nil || len(indices) > len(best)) {
				best = indices
			}
		}
		if len(best) > 0 {
			first = best[rand.Intn(len(best))]
			return first, -1, flipReasonElementKnown
		}
	}
	// Roll passed but no best move: prefer unseen tiles (never revealed by any player), then any hidden
	if len(hidden) == 0 {
		return -1, -1, flipReasonRandom
	}
	unseen := unseenHiddenIndices(hidden, knownIndicesSet)
	if len(unseen) > 0 {
		first = unseen[rand.Intn(len(unseen))]
		return first, -1, flipReasonUnseen
	}
	first = hidden[rand.Intn(len(hidden))]
	return first, -1, flipReasonUnseen
}

// pickSecondCard chooses the second card to flip. Returns (index, reason).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after elemental).
// elementMemory and hiddenByElement: tiles we know the element of; if first card is one of them, prefer second from same element.
// knownIndicesSet: indices ever revealed by any player; used so "unseen" excludes opponent's reveals.
func pickSecondCard(memory map[int]int, hidden []int, firstIdx int, useBestMove bool, hiddenHighlighted []int, elementMemory map[int]string, hiddenByElement map[string][]int, knownIndicesSet map[int]struct{}) (int, string) {
	// Always complete a known pair when we can (first card's pairID known and the other tile still hidden)
	pairID, known := memory[firstIdx]
	if known {
		for _, idx := range hidden {
			if idx != firstIdx && memory[idx] == pairID {
				return idx, flipReasonKnownPair
			}
		}
	}
	if !useBestMove {
		var candidates []int
		for _, idx := range hidden {
			if idx != firstIdx {
				candidates = append(candidates, idx)
			}
		}
		if len(candidates) == 0 {
			return -1, flipReasonRandom
		}
		return candidates[rand.Intn(len(candidates))], flipReasonRandom
	}
	// Current-turn highlight: first card was from that element; pick second from other still-hidden highlighted tiles.
	if len(hiddenHighlighted) > 0 {
		return hiddenHighlighted[rand.Intn(len(hiddenHighlighted))], flipReasonHighlight
	}
	// We know the first card's element (from element memory): prefer second from other hidden tiles of that element.
	if elementMemory != nil && hiddenByElement != nil {
		if elem, ok := elementMemory[firstIdx]; ok && len(hiddenByElement[elem]) > 0 {
			candidates := hiddenByElement[elem]
			return candidates[rand.Intn(len(candidates))], flipReasonElementKnown
		}
	}
	// Roll passed but no best move: prefer unseen tiles (never revealed by any player), then any candidate
	var candidates []int
	for _, idx := range hidden {
		if idx != firstIdx {
			candidates = append(candidates, idx)
		}
	}
	if len(candidates) == 0 {
		return -1, flipReasonRandom
	}
	unseen := unseenFromCandidates(candidates, knownIndicesSet)
	if len(unseen) > 0 {
		return unseen[rand.Intn(len(unseen))], flipReasonUnseen
	}
	return candidates[rand.Intn(len(candidates))], flipReasonUnseen
}

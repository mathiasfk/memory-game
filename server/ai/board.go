package ai

import (
	"memory-game-server/game"
)

// pairsRemaining returns the number of pairs still on the board (both cards not yet matched/removed).
func pairsRemaining(cards []game.CardView) int {
	matched := 0
	for _, c := range cards {
		if c.State == "matched" {
			matched++
		}
	}
	total := len(cards) / 2
	return total - matched/2
}

func hiddenIndices(cards []game.CardView) []int {
	var out []int
	for _, c := range cards {
		if c.State == "hidden" {
			out = append(out, c.Index)
		}
	}
	return out
}

// unseenHiddenIndices returns hidden indices that have never been revealed by any player (not in knownIndicesSet).
// Used to prefer flipping truly unseen tiles; considers opponent's reveals via the game's KnownIndices.
func unseenHiddenIndices(hidden []int, knownIndicesSet map[int]struct{}) []int {
	var out []int
	for _, idx := range hidden {
		if _, known := knownIndicesSet[idx]; !known {
			out = append(out, idx)
		}
	}
	return out
}

// unseenFromCandidates returns candidates that have never been revealed by any player (not in knownIndicesSet).
func unseenFromCandidates(candidates []int, knownIndicesSet map[int]struct{}) []int {
	var out []int
	for _, idx := range candidates {
		if _, known := knownIndicesSet[idx]; !known {
			out = append(out, idx)
		}
	}
	return out
}

// hiddenIndicesByElement returns, for each element, the hidden indices that we know (from elementMemory) have that element.
// Used to prefer flipping among tiles of the same element when we don't have a full known pair.
func hiddenIndicesByElement(elementMemory map[int]string, hidden []int) map[string][]int {
	hiddenSet := make(map[int]struct{}, len(hidden))
	for _, idx := range hidden {
		hiddenSet[idx] = struct{}{}
	}
	out := make(map[string][]int)
	for idx, elem := range elementMemory {
		if _, ok := hiddenSet[idx]; ok && elem != "" {
			out[elem] = append(out[elem], idx)
		}
	}
	return out
}

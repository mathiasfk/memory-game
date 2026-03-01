package heuristic

import (
	"memory-game-server/game"
)

// RandomMatchProb returns the probability of matching a pair by random guess when P pairs remain.
// Formula: 1/(2P - 1). Returns 0 if P <= 0. Exported for use by ai.evNoCard.
func RandomMatchProb(P int) float64 {
	if P <= 0 {
		return 0
	}
	denom := 2*P - 1
	if denom <= 0 {
		return 0
	}
	return 1 / float64(denom)
}

// HasKnownPair returns true if memory contains a complete pair still in hidden (both indices hidden).
// Exported for use by ai.evNoCard.
func HasKnownPair(memory map[int]int, hidden []int) bool {
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	for _, indices := range pairToIndices {
		if len(indices) >= 2 {
			return true
		}
	}
	return false
}

func binom(n, k int) float64 {
	if k < 0 || k > n {
		return 0
	}
	if k > n/2 {
		k = n - k
	}
	r := 1.0
	for i := 0; i < k; i++ {
		r *= float64(n-i) / float64(i+1)
	}
	return r
}

func expectedPairsFromReveal(P, k int) float64 {
	n := 2 * P
	if P < 2 || k < 2 || n < k {
		return 0
	}
	num := binom(n-2, k-2)
	den := binom(n, k)
	if den == 0 {
		return 0
	}
	return float64(P) * num / den
}

func knownPairElement(memory map[int]int, hidden []int, arcanaPairs int) string {
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	for pairID, indices := range pairToIndices {
		if len(indices) >= 2 {
			return game.ElementForNormalPair(pairID, arcanaPairs)
		}
	}
	return ""
}

func pairsOfElementRemaining(state *game.GameStateMsg, element string) int {
	totalPairs := len(state.Cards) / 2
	matchedOfElement := make(map[int]struct{})
	for _, c := range state.Cards {
		if c.State != "matched" || c.PairID == nil {
			continue
		}
		if *c.PairID >= state.ArcanaPairs && game.ElementForNormalPair(*c.PairID, state.ArcanaPairs) == element {
			matchedOfElement[*c.PairID] = struct{}{}
		}
	}
	totalOfElement := 0
	for pairID := state.ArcanaPairs; pairID < totalPairs; pairID++ {
		if game.ElementForNormalPair(pairID, state.ArcanaPairs) == element {
			totalOfElement++
		}
	}
	remaining := totalOfElement - len(matchedOfElement)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func hasPartialKnownOfElement(memory map[int]int, hidden []int, arcanaPairs int, element string) bool {
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok && game.ElementForNormalPair(p, arcanaPairs) == element {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	hasAny := false
	for _, indices := range pairToIndices {
		if len(indices) >= 2 {
			return false
		}
		if len(indices) >= 1 {
			hasAny = true
		}
	}
	return hasAny
}

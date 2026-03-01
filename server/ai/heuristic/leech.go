package heuristic

import (
	"memory-game-server/game"
)

func init() {
	Register("leech", evLeech, nil)
}

// evLeech returns the expected value of using Leech. Leech makes points earned this turn
// be subtracted from the opponent, so we only use it when we're sure to score (known pair).
// When we have a known pair: EV = 1 + possible chain (same as evNoCard). Otherwise: 0.
func evLeech(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	if !HasKnownPair(memory, hidden) {
		return 0
	}
	PAfter := P - 1
	if PAfter <= 0 {
		return 1
	}
	return 1 + RandomMatchProb(PAfter)
}

package heuristic

import (
	"memory-game-server/game"
)

func init() {
	Register("chaos", evChaos, nil)
}

// evChaos: Chaos clears memory; EV is random guess only. Use only when we have no known pair.
func evChaos(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	return RandomMatchProb(P)
}

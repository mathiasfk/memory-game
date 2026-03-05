package heuristic

import (
	"memory-game-server/game"
)

func init() {
	Register("blood_pact", evBloodPact, nil)
}

// evBloodPact returns the expected value of using Blood Pact. Blood Pact grants +5 if the
// player matches 3 pairs in a row, or -3 on first failure. We only use it when the AI has
// at least 3 known pairs in memory (certain to score); otherwise EV = 0 (do not risk).
func evBloodPact(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	pairToIndices := pairToHiddenIndices(memory, hidden)
	count := 0
	for _, indices := range pairToIndices {
		if len(indices) >= 2 {
			count++
		}
	}
	if count >= 3 {
		return 5
	}
	return 0
}

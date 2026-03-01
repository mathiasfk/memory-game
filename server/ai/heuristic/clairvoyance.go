package heuristic

import (
	"math/rand"

	"memory-game-server/game"
)

func init() {
	Register("clairvoyance", evClairvoyance, pickClairvoyanceTarget)
}

// evClairvoyance returns the expected value (in points) of using Clairvoyance when P pairs remain.
func evClairvoyance(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	return expectedPairsFromReveal(P, 9)
}

func radarRegionIndices(centerIndex, rows, cols int) []int {
	total := rows * cols
	if centerIndex < 0 || centerIndex >= total {
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

func pickClairvoyanceTarget(state *game.GameStateMsg, memory map[int]int, hidden []int, rows, cols int) int {
	hiddenSet := make(map[int]struct{}, len(hidden))
	for _, idx := range hidden {
		hiddenSet[idx] = struct{}{}
	}
	var bestIndices []int
	bestCount := -1
	for _, center := range hidden {
		region := radarRegionIndices(center, rows, cols)
		count := 0
		for _, idx := range region {
			if _, ok := hiddenSet[idx]; ok {
				count++
			}
		}
		if count > bestCount {
			bestCount = count
			bestIndices = []int{center}
		} else if count == bestCount && bestCount >= 0 {
			bestIndices = append(bestIndices, center)
		}
	}
	if len(bestIndices) == 0 {
		return -1
	}
	return bestIndices[rand.Intn(len(bestIndices))]
}

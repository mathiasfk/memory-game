package heuristic

import (
	"memory-game-server/game"
)

func init() {
	Register("fire_elemental", elementalEV(game.ElementFire), nil)
	Register("water_elemental", elementalEV(game.ElementWater), nil)
	Register("air_elemental", elementalEV(game.ElementAir), nil)
	Register("earth_elemental", elementalEV(game.ElementEarth), nil)
}

func elementalEV(need string) EVFunc {
	return func(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
		// Full known pair of this element: we can match without elemental; using it gives same EV (we don't prefer it).
		elem := knownPairElement(memory, hidden, state.ArcanaPairs)
		if elem == need {
			PAfter := P - 1
			if PAfter <= 0 {
				return 1
			}
			return 1 + RandomMatchProb(PAfter)
		}
		// Partial knowledge: we know at least one tile of this element but not its pair.
		if hasPartialKnownOfElement(memory, hidden, state.ArcanaPairs, need) {
			K := pairsOfElementRemaining(state, need)
			if K <= 0 {
				return 0
			}
			otherTiles := 2*K - 1
			if otherTiles <= 0 {
				return 1
			}
			matchProb := 1.0 / float64(otherTiles)
			PAfter := P - 1
			if PAfter <= 0 {
				return matchProb
			}
			return matchProb * (1 + RandomMatchProb(PAfter))
		}
		return 0
	}
}

package ai

import (
	"fmt"
	"math/rand"
	"strings"

	"memory-game-server/ai/heuristic"
	"memory-game-server/config"
	"memory-game-server/game"
)

// powerUpIDToElement returns the element for an elemental power-up ID, or "" if not an elemental.
func powerUpIDToElement(powerUpID string) string {
	switch powerUpID {
	case PowerUpFireElemental:
		return game.ElementFire
	case PowerUpWaterElemental:
		return game.ElementWater
	case PowerUpAirElemental:
		return game.ElementAir
	case PowerUpEarthElemental:
		return game.ElementEarth
	default:
		return ""
	}
}

// evNoCard returns expected value (points) for this turn without using any arcana.
// When we have a known pair we get 1 point and keep the turn, so we include the EV of the extra flip.
func evNoCard(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	if heuristic.HasKnownPair(memory, hidden) {
		PAfter := P - 1
		if PAfter <= 0 {
			return 1
		}
		return 1 + heuristic.RandomMatchProb(PAfter)
	}
	return heuristic.RandomMatchProb(P)
}

// evWithCard returns expected value when using the given power-up. Returns a negative value if the card
// has no registered heuristic (e.g. Oblivion, Necromancy). Uses the heuristic registry.
func evWithCard(state *game.GameStateMsg, memory map[int]int, hidden []int, powerUpID string, P int) float64 {
	return heuristic.EV(powerUpID, state, memory, hidden, P)
}

// arcanaDecision holds the result of pickArcanaToUse. Reason is "ev" (maximize EV), "random" (randomness applied), or "no_improvement" (no card improved EV).
// CardIndex is the target for power-ups that need it (e.g. Clairvoyance); -1 otherwise.
type arcanaDecision struct {
	powerUpID string
	use       bool
	reason    string
	CardIndex int
}

// pickArcanaToUse decides whether to use an arcana this turn and which one.
// Applies ArcanaRandomness: with that probability we may skip using a good card or randomize.
// rows and cols are the board dimensions (for Clairvoyance target choice).
func pickArcanaToUse(state *game.GameStateMsg, memory map[int]int, hidden []int, rows, cols int, params *config.AIParams) arcanaDecision {
	P := pairsRemaining(state.Cards)
	if P <= 0 {
		return arcanaDecision{reason: "no_improvement"}
	}
	evNo := evNoCard(state, memory, hidden, P)

	// Collect usable cards and their EV
	type choice struct {
		powerUpID string
		ev        float64
	}
	var candidates []choice
	for _, slot := range state.Hand {
		if slot.UsableCount <= 0 {
			continue
		}
		ev := evWithCard(state, memory, hidden, slot.PowerUpID, P)
		if ev < 0 {
			continue
		}
		// Use card when it improves or equals EV (e.g. Chaos when no known pair: same EV, may use to disrupt).
		if ev >= evNo {
			candidates = append(candidates, choice{slot.PowerUpID, ev})
		}
	}
	if len(candidates) == 0 {
		return arcanaDecision{reason: "no_improvement"}
	}

	// Best: highest EV
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.ev > best.ev {
			best = c
		}
	}

	// ArcanaRandomness: with probability ArcanaRandomness/100, skip using or pick randomly
	randomness := clampPercent(params.ArcanaRandomness)
	if randomness > 0 && rand.Intn(100) < randomness {
		if rand.Intn(2) == 0 {
			return arcanaDecision{reason: "random"}
		}
		best = candidates[rand.Intn(len(candidates))]
		dec := arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "random"}
		dec.CardIndex = heuristic.PickTarget(best.powerUpID, state, memory, hidden, rows, cols)
		if dec.CardIndex == -1 && needsTarget(best.powerUpID) {
			return arcanaDecision{reason: "no_improvement"}
		}
		return dec
	}

	dec := arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "ev"}
	dec.CardIndex = heuristic.PickTarget(best.powerUpID, state, memory, hidden, rows, cols)
	if dec.CardIndex == -1 && needsTarget(best.powerUpID) {
		return arcanaDecision{reason: "no_improvement"}
	}
	return dec
}

// needsTarget returns true if the power-up requires a card target (e.g. Clairvoyance, Oblivion).
func needsTarget(powerUpID string) bool {
	switch powerUpID {
	case PowerUpClairvoyance, PowerUpOblivion:
		return true
	default:
		return false
	}
}

// formatHand returns a short description of the AI hand for logging (e.g. "fire_elemental(1), chaos(2, 1 usable)").
func formatHand(hand []game.PowerUpInHand) string {
	if len(hand) == 0 {
		return "none"
	}
	var parts []string
	for _, slot := range hand {
		if slot.UsableCount == slot.Count {
			parts = append(parts, fmt.Sprintf("%s(%d)", slot.PowerUpID, slot.Count))
		} else {
			parts = append(parts, fmt.Sprintf("%s(%d, %d usable)", slot.PowerUpID, slot.Count, slot.UsableCount))
		}
	}
	return strings.Join(parts, ", ")
}

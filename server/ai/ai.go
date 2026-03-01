package ai

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"memory-game-server/config"
	"memory-game-server/game"
)

// Element names for normal pairs (must match game board logic: pairID >= ArcanaPairs → (pairID-arcanaPairs)/3).
const (
	elementFire  = "fire"
	elementWater = "water"
	elementAir   = "air"
	elementEarth = "earth"
)

// elementForNormalPair returns the element for a normal pair (pairID >= arcanaPairs).
// Matches game board: 3 pairs per element → fire, water, air, earth.
func elementForNormalPair(pairID, arcanaPairs int) string {
	if pairID < arcanaPairs {
		return ""
	}
	normalPairIndex := pairID - arcanaPairs
	elementIndex := normalPairIndex / 3
	switch elementIndex {
	case 0:
		return elementFire
	case 1:
		return elementWater
	case 2:
		return elementAir
	case 3:
		return elementEarth
	default:
		return ""
	}
}

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

// randomMatchProb returns the probability of matching a pair by random guess when P pairs remain.
// Formula: 1/(2P - 1). Returns 0 if P <= 0.
func randomMatchProb(P int) float64 {
	if P <= 0 {
		return 0
	}
	denom := 2*P - 1
	if denom <= 0 {
		return 0
	}
	return 1 / float64(denom)
}

// hasKnownPair returns true if memory contains a complete pair still in hidden (both indices hidden).
func hasKnownPair(memory map[int]int, hidden []int) bool {
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

// knownPairElement returns the element of a known pair (normal pair only), or "" if none or arcana.
func knownPairElement(memory map[int]int, hidden []int, arcanaPairs int) string {
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	for pairID, indices := range pairToIndices {
		if len(indices) >= 2 {
			return elementForNormalPair(pairID, arcanaPairs)
		}
	}
	return ""
}

// evNoCard returns expected value (points) for this turn without using any arcana.
func evNoCard(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	if hasKnownPair(memory, hidden) {
		return 1
	}
	return randomMatchProb(P)
}

// evWithCard returns expected value when using the given power-up. Returns a negative value if the card
// is not evaluated in v1 (e.g. Clairvoyance, Oblivion). Only elementals and chaos are considered.
func evWithCard(state *game.GameStateMsg, memory map[int]int, hidden []int, powerUpID string, P int) float64 {
	switch powerUpID {
	case "fire_elemental", "water_elemental", "air_elemental", "earth_elemental":
		elem := knownPairElement(memory, hidden, state.ArcanaPairs)
		var need string
		switch powerUpID {
		case "fire_elemental":
			need = elementFire
		case "water_elemental":
			need = elementWater
		case "air_elemental":
			need = elementAir
		case "earth_elemental":
			need = elementEarth
		default:
			return 0
		}
		if elem != need {
			return 0
		}
		// We have a known pair of this element: guaranteed 1 point + bonus term (P-1 pairs after).
		PAfter := P - 1
		if PAfter <= 0 {
			return 1
		}
		return 1 + randomMatchProb(PAfter)
	case "chaos":
		// Chaos clears memory; EV is random guess only. Use only when we have no known pair.
		return randomMatchProb(P)
	default:
		return -1
	}
}

// arcanaDecision holds the result of pickArcanaToUse. Reason is "ev" (maximize EV), "random" (randomness applied), or "no_improvement" (no card improved EV).
type arcanaDecision struct {
	powerUpID string
	use       bool
	reason    string
}

// pickArcanaToUse decides whether to use an arcana this turn and which one.
// Applies ArcanaRandomness: with that probability we may skip using a good card or randomize.
func pickArcanaToUse(state *game.GameStateMsg, memory map[int]int, hidden []int, params *config.AIParams) arcanaDecision {
	P := pairsRemaining(state.Cards)
	if P <= 0 {
		return arcanaDecision{reason: "no_improvement"}
	}
	evNo := evNoCard(state, memory, hidden, P)

	// Collect usable cards and their EV
	type choice struct {
		powerUpID string
		ev       float64
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
	randomness := params.ArcanaRandomness
	if randomness < 0 {
		randomness = 0
	}
	if randomness > 100 {
		randomness = 100
	}
	if randomness > 0 && rand.Intn(100) < randomness {
		if rand.Intn(2) == 0 {
			return arcanaDecision{reason: "random"}
		}
		// Pick randomly among candidates instead of best
		best = candidates[rand.Intn(len(candidates))]
		return arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "random"}
	}

	return arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "ev"}
}

// Run receives game state messages from the given channel and sends actions to the game
// when it is the AI's turn. It only uses information from the game_state payload (no
// access to board internals). It runs until the channel is closed or a game_over is received.
func Run(aiSend <-chan []byte, g *game.Game, playerIdx int, params *config.AIParams) {
	memory := make(map[int]int) // index -> pairID (what we've seen at each position)

	for data := range aiSend {
		var typeEnvelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &typeEnvelope); err != nil {
			continue
		}

		switch typeEnvelope.Type {
		case "game_over":
			return
		case "game_state":
			var state game.GameStateMsg
			if err := json.Unmarshal(data, &state); err != nil {
				continue
			}

			// Update memory from current view: any revealed or matched card exposes its pairID
			for _, c := range state.Cards {
				if (c.State == "revealed" || c.State == "matched") && c.PairID != nil {
					memory[c.Index] = *c.PairID
				}
			}

			// Forget: with ForgetChance probability, remove each known card from memory
			forgetChance := params.ForgetChance
			if forgetChance < 0 {
				forgetChance = 0
			}
			if forgetChance > 100 {
				forgetChance = 100
			}
			if forgetChance > 0 && len(memory) > 0 {
				var toForget []int
				for idx := range memory {
					if rand.Intn(100) < forgetChance {
						toForget = append(toForget, idx)
					}
				}
				for _, idx := range toForget {
					delete(memory, idx)
				}
			}

			if !state.YourTurn {
				continue
			}

			hidden := hiddenIndices(state.Cards)
			if len(hidden) == 0 {
				continue
			}

			// Human-like delay before acting
			delayMS := params.DelayMinMS
			if params.DelayMaxMS > params.DelayMinMS {
				delayMS = params.DelayMinMS + rand.Intn(params.DelayMaxMS-params.DelayMinMS)
			}
			time.Sleep(time.Duration(delayMS) * time.Millisecond)

			// Re-read state after delay in case game ended (e.g. opponent disconnected)
			// We don't have a way to re-read; just send. Game will ignore if not our turn.

			chance := params.UseKnownPairChance
			if chance < 0 {
				chance = 0
			}
			if chance > 100 {
				chance = 100
			}
			useKnownPair := rand.Intn(100) < chance

			if state.Phase == "second_flip" && len(state.FlippedIndices) > 0 {
				// We already flipped one card; choose the second
				firstIdx := state.FlippedIndices[0]
				secondIdx := pickSecondCard(memory, hidden, firstIdx, useKnownPair)
				if secondIdx >= 0 {
					sendAction(g, playerIdx, secondIdx)
				}
				continue
			}

			// Phase is first_flip: consider using an arcana before flipping
			hasUsableArcana := false
			for _, slot := range state.Hand {
				if slot.UsableCount > 0 {
					hasUsableArcana = true
					break
				}
			}
			if hasUsableArcana {
				dec := pickArcanaToUse(&state, memory, hidden, params)
				if dec.use && dec.powerUpID != "" {
					log.Printf("[AI] %s decided to use arcana: %s (reason: %s)", params.Name, dec.powerUpID, dec.reason)
					sendUsePowerUp(g, playerIdx, dec.powerUpID)
					continue
				}
				log.Printf("[AI] %s decided not to use arcana (reason: %s)", params.Name, dec.reason)
			}

			// Phase is first_flip: choose first card
			firstIdx, _ := pickPair(memory, hidden, useKnownPair)
			if firstIdx < 0 {
				continue
			}
			sendAction(g, playerIdx, firstIdx)
		}
	}
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

// pickPair returns (firstIndex, secondIndex). secondIndex may be -1 if we're guessing (we'll pick on next state).
func pickPair(memory map[int]int, hidden []int, useKnownPair bool) (first, second int) {
	// Build pairID -> list of hidden indices we know
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	// Find a complete known pair (both cards still hidden)
	for _, indices := range pairToIndices {
		if len(indices) >= 2 && useKnownPair {
			return indices[0], indices[1]
		}
	}
	// No known pair or chose to guess: pick two random hidden indices (we only send first now)
	if len(hidden) == 0 {
		return -1, -1
	}
	first = hidden[rand.Intn(len(hidden))]
	return first, -1
}

func pickSecondCard(memory map[int]int, hidden []int, firstIdx int, useKnownPair bool) int {
	pairID, known := memory[firstIdx]
	if known && useKnownPair {
		for _, idx := range hidden {
			if idx != firstIdx && memory[idx] == pairID {
				return idx
			}
		}
	}
	// Guess: any hidden except firstIdx
	var candidates []int
	for _, idx := range hidden {
		if idx != firstIdx {
			candidates = append(candidates, idx)
		}
	}
	if len(candidates) == 0 {
		return -1
	}
	return candidates[rand.Intn(len(candidates))]
}

func sendAction(g *game.Game, playerIdx int, cardIndex int) {
	select {
	case g.Actions <- game.Action{Type: game.ActionFlipCard, PlayerIdx: playerIdx, Index: cardIndex}:
	case <-g.Done:
	}
}

func sendUsePowerUp(g *game.Game, playerIdx int, powerUpID string) {
	select {
	case g.Actions <- game.Action{Type: game.ActionUsePowerUp, PlayerIdx: playerIdx, PowerUpID: powerUpID, CardIndex: -1}:
	case <-g.Done:
	}
}

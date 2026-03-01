package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"memory-game-server/ai/heuristic"
	"memory-game-server/config"
	"memory-game-server/game"
)

// powerUpIDToElement returns the element for an elemental power-up ID, or "" if not an elemental.
func powerUpIDToElement(powerUpID string) string {
	switch powerUpID {
	case "fire_elemental":
		return game.ElementFire
	case "water_elemental":
		return game.ElementWater
	case "air_elemental":
		return game.ElementAir
	case "earth_elemental":
		return game.ElementEarth
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
	case "clairvoyance", "oblivion":
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

// Run receives game state messages from the given channel and sends actions to the game
// when it is the AI's turn. It only uses information from the game_state payload (no
// access to board internals). It runs until the channel is closed or a game_over is received.
func Run(aiSend <-chan []byte, g *game.Game, playerIdx int, params *config.AIParams) {
	memory := make(map[int]int)       // index -> pairID (what we've seen at each position)
	elementMemory := make(map[int]string) // index -> element (tiles we know the element of, e.g. from elemental highlight)
	var lastElementalUsed string          // element of the elemental we just used; next state's HighlightIndices will be that element
	var clearElementMemoryNext bool       // true after we use Chaos (board shuffles, so element-by-index is stale)

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

			// After Chaos the board is shuffled; element memory by index is no longer valid.
			if clearElementMemoryNext {
				elementMemory = make(map[int]string)
				clearElementMemoryNext = false
			}
			// Prune element memory: only keep indices that are still hidden (matched/removed/revealed no longer useful).
			hiddenSet := make(map[int]struct{}, len(hidden))
			for _, idx := range hidden {
				hiddenSet[idx] = struct{}{}
			}
			for idx := range elementMemory {
				if _, ok := hiddenSet[idx]; !ok {
					delete(elementMemory, idx)
				}
			}
			// When we just used an elemental, this state's HighlightIndices are tiles of that element — store in element memory.
			if len(state.HighlightIndices) > 0 && lastElementalUsed != "" {
				for _, idx := range state.HighlightIndices {
					elementMemory[idx] = lastElementalUsed
				}
				lastElementalUsed = ""
			}

			// Hidden indices that are currently highlighted (e.g. after using an elemental — those tiles are of that element).
			var hiddenHighlighted []int
			if len(state.HighlightIndices) > 0 {
				for _, idx := range state.HighlightIndices {
					if _, inHidden := hiddenSet[idx]; inHidden {
						hiddenHighlighted = append(hiddenHighlighted, idx)
					}
				}
			}

			// Human-like delay before acting
			delayMS := params.DelayMinMS
			if params.DelayMaxMS > params.DelayMinMS {
				delayMS = params.DelayMinMS + rand.Intn(params.DelayMaxMS-params.DelayMinMS)
			}
			time.Sleep(time.Duration(delayMS) * time.Millisecond)

			// Re-read state after delay in case game ended (e.g. opponent disconnected)
			// We don't have a way to re-read; just send. Game will ignore if not our turn.

			chance := params.UseBestMoveChance
			if chance < 0 {
				chance = 0
			}
			if chance > 100 {
				chance = 100
			}
			useBestMove := rand.Intn(100) < chance

			hiddenByElement := hiddenIndicesByElement(elementMemory, hidden)

			if state.Phase == "second_flip" && len(state.FlippedIndices) > 0 {
				// We already flipped one card; choose the second
				firstIdx := state.FlippedIndices[0]
				secondIdx, flipReason := pickSecondCard(memory, hidden, firstIdx, useBestMove, hiddenHighlighted, elementMemory, hiddenByElement)
				if secondIdx >= 0 {
					slog.Debug("flipping tile (second)", "tag", "ai", "name", params.Name, "tile", secondIdx, "reason", flipReason)
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
				rows, cols := 0, 0
				if g.Config != nil {
					rows, cols = g.Config.BoardRows, g.Config.BoardCols
				}
				dec := pickArcanaToUse(&state, memory, hidden, rows, cols, params)
				handStr := formatHand(state.Hand)
				if dec.use && dec.powerUpID != "" {
					slog.Debug("decided to use arcana", "tag", "ai", "name", params.Name, "arcana", dec.powerUpID, "reason", dec.reason, "hand", handStr)
					if elem := powerUpIDToElement(dec.powerUpID); elem != "" {
						lastElementalUsed = elem
					}
					if dec.powerUpID == "chaos" {
						clearElementMemoryNext = true
					}
					sendUsePowerUp(g, playerIdx, dec.powerUpID, dec.CardIndex)
					continue
				}
				slog.Debug("decided not to use arcana", "tag", "ai", "name", params.Name, "reason", dec.reason, "hand", handStr)
			}

			// Phase is first_flip: choose first card
			firstIdx, _, flipReason := pickPair(memory, hidden, useBestMove, hiddenHighlighted, hiddenByElement)
			if firstIdx < 0 {
				continue
			}
			slog.Debug("flipping tile (first)", "tag", "ai", "name", params.Name, "tile", firstIdx, "reason", flipReason)
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

// unknownHiddenIndices returns hidden indices we have never seen (not in memory). Used to prefer flipping unknown tiles when guessing.
func unknownHiddenIndices(memory map[int]int, hidden []int) []int {
	var out []int
	for _, idx := range hidden {
		if _, ok := memory[idx]; !ok {
			out = append(out, idx)
		}
	}
	return out
}

// unknownFromCandidates returns candidates that are not in memory (unknown positions).
func unknownFromCandidates(memory map[int]int, candidates []int) []int {
	var out []int
	for _, idx := range candidates {
		if _, ok := memory[idx]; !ok {
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

// flipReason describes why the AI chose to flip a given tile (for logging).
const (
	flipReasonKnownPair    = "known_pair"
	flipReasonHighlight    = "highlight_elemental"
	flipReasonElementKnown = "element_known"
	flipReasonUnseen = "unseen" // roll passed but no known pair/highlight/element; reveal an unseen (never-revealed) tile
	flipReasonRandom = "random"
)

// pickPair returns (firstIndex, secondIndex, reason). secondIndex may be -1 if we're guessing (we'll pick on next state).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after an elemental).
// hiddenByElement: hidden indices we know per element (from element memory); used to prefer flipping within same element.
func pickPair(memory map[int]int, hidden []int, useBestMove bool, hiddenHighlighted []int, hiddenByElement map[string][]int) (first, second int, reason string) {
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
	// Roll passed but no best move: prefer unseen tiles (never revealed), then any hidden
	if len(hidden) == 0 {
		return -1, -1, flipReasonRandom
	}
	unknown := unknownHiddenIndices(memory, hidden)
	if len(unknown) > 0 {
		first = unknown[rand.Intn(len(unknown))]
		return first, -1, flipReasonUnseen
	}
	first = hidden[rand.Intn(len(hidden))]
	return first, -1, flipReasonUnseen
}

// pickSecondCard chooses the second card to flip. Returns (index, reason).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after elemental).
// elementMemory and hiddenByElement: tiles we know the element of; if first card is one of them, prefer second from same element.
func pickSecondCard(memory map[int]int, hidden []int, firstIdx int, useBestMove bool, hiddenHighlighted []int, elementMemory map[int]string, hiddenByElement map[string][]int) (int, string) {
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
	pairID, known := memory[firstIdx]
	if known {
		for _, idx := range hidden {
			if idx != firstIdx && memory[idx] == pairID {
				return idx, flipReasonKnownPair
			}
		}
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
	// Roll passed but no best move: prefer unseen tiles (never revealed), then any candidate
	var candidates []int
	for _, idx := range hidden {
		if idx != firstIdx {
			candidates = append(candidates, idx)
		}
	}
	if len(candidates) == 0 {
		return -1, flipReasonRandom
	}
	unknown := unknownFromCandidates(memory, candidates)
	if len(unknown) > 0 {
		return unknown[rand.Intn(len(unknown))], flipReasonUnseen
	}
	return candidates[rand.Intn(len(candidates))], flipReasonUnseen
}

func sendAction(g *game.Game, playerIdx int, cardIndex int) {
	select {
	case g.Actions <- game.Action{Type: game.ActionFlipCard, PlayerIdx: playerIdx, Index: cardIndex}:
	case <-g.Done:
	}
}

func sendUsePowerUp(g *game.Game, playerIdx int, powerUpID string, cardIndex int) {
	select {
	case g.Actions <- game.Action{Type: game.ActionUsePowerUp, PlayerIdx: playerIdx, PowerUpID: powerUpID, CardIndex: cardIndex}:
	case <-g.Done:
	}
}

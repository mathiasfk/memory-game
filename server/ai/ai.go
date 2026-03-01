package ai

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
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

// powerUpIDToElement returns the element for an elemental power-up ID, or "" if not an elemental.
func powerUpIDToElement(powerUpID string) string {
	switch powerUpID {
	case "fire_elemental":
		return elementFire
	case "water_elemental":
		return elementWater
	case "air_elemental":
		return elementAir
	case "earth_elemental":
		return elementEarth
	default:
		return ""
	}
}

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

// binom returns C(n,k) = n!/(k!(n-k)!) as float64 for small n,k to avoid overflow.
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

// expectedPairsFromReveal returns the expected number of complete pairs seen when revealing k cards
// from 2*P cards (P pairs). Formula: P * C(2P-2, k-2) / C(2P, k). Returns 0 if 2P < k or P < 2.
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

// evClairvoyance returns the expected value (in points) of using Clairvoyance when P pairs remain:
// expected number of complete pairs we learn from revealing a 3x3 (9 cards), as a proxy for future points.
func evClairvoyance(P int) float64 {
	return expectedPairsFromReveal(P, 9)
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

// pairsOfElementRemaining returns how many pairs of the given element are still on the board (not matched/removed).
func pairsOfElementRemaining(state *game.GameStateMsg, element string) int {
	totalPairs := len(state.Cards) / 2
	matchedOfElement := make(map[int]struct{})
	for _, c := range state.Cards {
		if c.State != "matched" || c.PairID == nil {
			continue
		}
		if *c.PairID >= state.ArcanaPairs && elementForNormalPair(*c.PairID, state.ArcanaPairs) == element {
			matchedOfElement[*c.PairID] = struct{}{}
		}
	}
	// Count total pairIDs of this element (pairIDs >= arcanaPairs with this element)
	totalOfElement := 0
	for pairID := state.ArcanaPairs; pairID < totalPairs; pairID++ {
		if elementForNormalPair(pairID, state.ArcanaPairs) == element {
			totalOfElement++
		}
	}
	remaining := totalOfElement - len(matchedOfElement)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// hasPartialKnownOfElement returns true if we have at least one hidden tile of this element in memory,
// but we do not have a complete pair of this element (both tiles) in memory.
func hasPartialKnownOfElement(memory map[int]int, hidden []int, arcanaPairs int, element string) bool {
	pairToIndices := make(map[int][]int)
	for _, idx := range hidden {
		if p, ok := memory[idx]; ok && elementForNormalPair(p, arcanaPairs) == element {
			pairToIndices[p] = append(pairToIndices[p], idx)
		}
	}
	// We have partial knowledge if there is at least one tile of this element in memory,
	// but no pair has both tiles known.
	hasAny := false
	for _, indices := range pairToIndices {
		if len(indices) >= 2 {
			return false // full pair known, not partial
		}
		if len(indices) >= 1 {
			hasAny = true
		}
	}
	return hasAny
}

// evNoCard returns expected value (points) for this turn without using any arcana.
// When we have a known pair we get 1 point and keep the turn, so we include the EV of the extra flip.
func evNoCard(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	if hasKnownPair(memory, hidden) {
		// 1 point from matching the known pair + EV of the bonus turn (one more flip with P-1 pairs left)
		PAfter := P - 1
		if PAfter <= 0 {
			return 1
		}
		return 1 + randomMatchProb(PAfter)
	}
	return randomMatchProb(P)
}

// evWithCard returns expected value when using the given power-up. Returns a negative value if the card
// is not evaluated in v1 (e.g. Clairvoyance, Oblivion). Only elementals and chaos are considered.
func evWithCard(state *game.GameStateMsg, memory map[int]int, hidden []int, powerUpID string, P int) float64 {
	switch powerUpID {
	case "fire_elemental", "water_elemental", "air_elemental", "earth_elemental":
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
		// Full known pair of this element: we can match without elemental; using it gives same EV (we don't prefer it).
		elem := knownPairElement(memory, hidden, state.ArcanaPairs)
		if elem == need {
			PAfter := P - 1
			if PAfter <= 0 {
				return 1
			}
			return 1 + randomMatchProb(PAfter)
		}
		// Partial knowledge: we know at least one tile of this element but not its pair. Elemental reveals
		// all tiles of that element; we flip our known tile then pick among the other (2*K-1) highlighted; 1 is the match.
		if hasPartialKnownOfElement(memory, hidden, state.ArcanaPairs, need) {
			K := pairsOfElementRemaining(state, need)
			if K <= 0 {
				return 0
			}
			otherTiles := 2*K - 1 // one is our known tile, the rest are the other positions of that element
			if otherTiles <= 0 {
				return 1 // only one pair left of this element: guaranteed match
			}
			matchProb := 1.0 / float64(otherTiles)
			// If we match we get 1 point and keep the turn
			PAfter := P - 1
			if PAfter <= 0 {
				return matchProb
			}
			return matchProb * (1 + randomMatchProb(PAfter))
		}
		return 0
	case "chaos":
		// Chaos clears memory; EV is random guess only. Use only when we have no known pair.
		return randomMatchProb(P)
	case "clairvoyance":
		return evClairvoyance(P)
	default:
		return -1
	}
}

// radarRegionIndices returns the board indices of the 3x3 region centered on the given card index.
// Same logic as game.RadarRegionIndices; used by the AI to choose Clairvoyance target (no access to board).
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

// pickClairvoyanceTarget returns a hidden card index to center the 3x3 on, maximizing the number of
// hidden cards in that region. Ties are broken randomly.
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
		// Pick randomly among candidates instead of best
		best = candidates[rand.Intn(len(candidates))]
		dec := arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "random"}
		if best.powerUpID == "clairvoyance" {
			dec.CardIndex = pickClairvoyanceTarget(state, memory, hidden, rows, cols)
			if dec.CardIndex < 0 {
				return arcanaDecision{reason: "no_improvement"}
			}
		}
		return dec
	}

	dec := arcanaDecision{powerUpID: best.powerUpID, use: true, reason: "ev"}
	if best.powerUpID == "clairvoyance" {
		dec.CardIndex = pickClairvoyanceTarget(state, memory, hidden, rows, cols)
		if dec.CardIndex < 0 {
			return arcanaDecision{reason: "no_improvement"}
		}
	}
	return dec
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

			chance := params.UseKnownPairChance
			if chance < 0 {
				chance = 0
			}
			if chance > 100 {
				chance = 100
			}
			useKnownPair := rand.Intn(100) < chance

			hiddenByElement := hiddenIndicesByElement(elementMemory, hidden)

			if state.Phase == "second_flip" && len(state.FlippedIndices) > 0 {
				// We already flipped one card; choose the second
				firstIdx := state.FlippedIndices[0]
				secondIdx, flipReason := pickSecondCard(memory, hidden, firstIdx, useKnownPair, hiddenHighlighted, elementMemory, hiddenByElement)
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
					cardIndex := -1
					if dec.powerUpID == "clairvoyance" {
						cardIndex = dec.CardIndex
					}
					sendUsePowerUp(g, playerIdx, dec.powerUpID, cardIndex)
					continue
				}
				slog.Debug("decided not to use arcana", "tag", "ai", "name", params.Name, "reason", dec.reason, "hand", handStr)
			}

			// Phase is first_flip: choose first card
			firstIdx, _, flipReason := pickPair(memory, hidden, useKnownPair, hiddenHighlighted, hiddenByElement)
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
	flipReasonKnownPair     = "known_pair"
	flipReasonHighlight     = "highlight_elemental"
	flipReasonElementKnown  = "element_known"
	flipReasonRandom        = "random"
)

// pickPair returns (firstIndex, secondIndex, reason). secondIndex may be -1 if we're guessing (we'll pick on next state).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after an elemental).
// hiddenByElement: hidden indices we know per element (from element memory); used to prefer flipping within same element.
func pickPair(memory map[int]int, hidden []int, useKnownPair bool, hiddenHighlighted []int, hiddenByElement map[string][]int) (first, second int, reason string) {
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
	// No known pair or chose to guess: pick random hidden index
	if len(hidden) == 0 {
		return -1, -1, flipReasonRandom
	}
	first = hidden[rand.Intn(len(hidden))]
	return first, -1, flipReasonRandom
}

// pickSecondCard chooses the second card to flip. Returns (index, reason).
// hiddenHighlighted: hidden indices currently highlighted (e.g. after elemental).
// elementMemory and hiddenByElement: tiles we know the element of; if first card is one of them, prefer second from same element.
func pickSecondCard(memory map[int]int, hidden []int, firstIdx int, useKnownPair bool, hiddenHighlighted []int, elementMemory map[int]string, hiddenByElement map[string][]int) (int, string) {
	pairID, known := memory[firstIdx]
	if known && useKnownPair {
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
	// Guess: any hidden except firstIdx
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

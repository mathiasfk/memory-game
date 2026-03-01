package ai

import (
	"encoding/json"
	"log/slog"
	"math/rand"
	"time"

	"memory-game-server/config"
	"memory-game-server/game"
)

// clampPercent clamps v to the range [0, 100].
func clampPercent(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// Run receives game state messages from the given channel and sends actions to the game
// when it is the AI's turn. It only uses information from the game_state payload (no
// access to board internals). It runs until the channel is closed or a game_over is received.
func Run(aiSend <-chan []byte, g *game.Game, playerIdx int, params *config.AIParams) {
	memory := make(map[int]int)             // index -> pairID (what we've seen at each position)
	elementMemory := make(map[int]string)    // index -> element (tiles we know the element of, e.g. from elemental highlight)
	var lastElementalUsed string             // element of the elemental we just used; next state's HighlightIndices will be that element
	var clearElementMemoryNext bool          // true after we use Chaos (board shuffles, so element-by-index is stale)
	var useBestMoveForSecondFlip bool        // when in second_flip, use same decision as first_flip so we complete known pairs

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

			// When Chaos was used the board was shuffled; KnownIndices is cleared by the game. Our index->pairID memory
			// would be stale (positions now hold different cards), so clear it in clearElementMemoryNext below.
			// Only clear at true game start: all cards hidden and no reveal has ever happened (KnownIndices empty).
			// Do not clear when merely allHidden: after a mismatch all cards are hidden again, so that would wipe
			// memory every turn and prevent known_pair from ever being used.
			allHidden := true
			for _, c := range state.Cards {
				if c.State != "hidden" {
					allHidden = false
					break
				}
			}
			if allHidden && len(state.KnownIndices) == 0 {
				memory = make(map[int]int)
			}
			// Update memory from current view: any revealed or matched card exposes its pairID.
			// Never overwrite an index with a different pairID than we've already seen (avoids stale/wrong
			// state from overwriting correct memory, e.g. index 0 already known as pairID 4).
			// Only allow memory[0] to be set from state.Cards[0] (slice position 0); if any other
			// card reports Index==0 (e.g. zero value / serialization bug), ignore it so we don't
			// wrongly associate pairID with tile 0.
			for i, c := range state.Cards {
				if (c.State == "revealed" || c.State == "matched") && c.PairID != nil {
					if c.Index == 0 && i != 0 {
						continue
					}
					pairID := *c.PairID
					if existing, ok := memory[c.Index]; !ok || existing == pairID {
						memory[c.Index] = pairID
					}
				}
			}

			// Forget: with ForgetChance probability, remove each known card from memory
			forgetChance := clampPercent(params.ForgetChance)
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

			// Do not act during resolve phase (wait for next state after mismatch timer or turn switch)
			if state.Phase == "resolve" {
				continue
			}

			// After Chaos the board is shuffled; index->pairID and element memory are no longer valid.
			if clearElementMemoryNext {
				memory = make(map[int]int)
				elementMemory = make(map[int]string)
				clearElementMemoryNext = false
			}
			// Prune element memory: only keep indices that are still hidden (matched/removed/revealed no longer useful).
			// In second_flip, keep the first flipped index in elementMemory so pickSecondCard can look up its element
			// and choose another tile of the same element; we'll prune it on the next state.
			hiddenSet := make(map[int]struct{}, len(hidden))
			for _, idx := range hidden {
				hiddenSet[idx] = struct{}{}
			}
			var keepInElementMemory map[int]struct{} // first flipped index when in second_flip
			if state.Phase == "second_flip" && len(state.FlippedIndices) > 0 {
				keepInElementMemory = map[int]struct{}{state.FlippedIndices[0]: {}}
			}
			for idx := range elementMemory {
				if _, ok := hiddenSet[idx]; !ok {
					if keepInElementMemory != nil {
						if _, keep := keepInElementMemory[idx]; keep {
							continue
						}
					}
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

			useBestMove := rand.Intn(100) < clampPercent(params.UseBestMoveChance)

			hiddenByElement := hiddenIndicesByElement(elementMemory, hidden)
			knownIndicesSet := make(map[int]struct{}, len(state.KnownIndices))
			for _, idx := range state.KnownIndices {
				knownIndicesSet[idx] = struct{}{}
			}

			if state.Phase == "second_flip" && len(state.FlippedIndices) > 0 {
				// We already flipped one card; choose the second (use same useBestMove as when we chose the first, so we complete known pairs)
				firstIdx := state.FlippedIndices[0]
				secondIdx, flipReason := pickSecondCard(memory, hidden, firstIdx, useBestMoveForSecondFlip, hiddenHighlighted, elementMemory, hiddenByElement, knownIndicesSet)
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
					if dec.powerUpID == PowerUpChaos {
						clearElementMemoryNext = true
					}
					sendUsePowerUp(g, playerIdx, dec.powerUpID, dec.CardIndex)
					continue
				}
				slog.Debug("decided not to use arcana", "tag", "ai", "name", params.Name, "reason", dec.reason, "hand", handStr)
			}

			// Phase is first_flip: choose first card
			firstIdx, _, flipReason := pickPair(memory, hidden, useBestMove, hiddenHighlighted, hiddenByElement, knownIndicesSet)
			if firstIdx < 0 {
				continue
			}
			// Persist useBestMove for second flip so we complete known pairs (set only when we actually send the first flip)
			useBestMoveForSecondFlip = useBestMove
			slog.Debug("flipping tile (first)", "tag", "ai", "name", params.Name, "tile", firstIdx, "reason", flipReason)
			sendAction(g, playerIdx, firstIdx)
		}
	}
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

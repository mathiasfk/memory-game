package ai

import (
	"encoding/json"
	"math/rand"
	"time"

	"memory-game-server/config"
	"memory-game-server/game"
)

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

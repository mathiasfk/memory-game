package game

import (
	"encoding/json"
	"time"

	"memory-game-server/wsutil"
)

func (g *Game) handleFlipCard(playerIdx int, cardIndex int) {
	// Validate it's this player's turn
	if playerIdx != g.CurrentTurn {
		g.sendError(playerIdx, "It is not your turn.")
		return
	}

	// Validate turn phase (must be FirstFlip or SecondFlip)
	if g.TurnPhase == Resolve {
		g.sendError(playerIdx, "Please wait for the current turn to resolve.")
		return
	}

	// Validate card index bounds
	totalCards := g.Config.BoardRows * g.Config.BoardCols
	if cardIndex < 0 || cardIndex >= totalCards {
		g.sendError(playerIdx, "Card index out of bounds.")
		return
	}

	// Validate card is hidden (not revealed, matched, or removed)
	card := &g.Board.Cards[cardIndex]
	if card.State != Hidden {
		g.sendError(playerIdx, "That card is already revealed, matched, or removed.")
		return
	}

	// Validate not flipping the same card twice in one turn
	for _, fi := range g.FlippedIndices {
		if fi == cardIndex {
			g.sendError(playerIdx, "You already flipped that card this turn.")
			return
		}
	}

	// Flip the card
	card.State = Revealed
	if g.KnownIndices != nil {
		g.KnownIndices[cardIndex] = struct{}{}
	}
	g.FlippedIndices = append(g.FlippedIndices, cardIndex)

	if g.TurnPhase == FirstFlip {
		// First card flipped - advance to SecondFlip phase
		g.TurnPhase = SecondFlip
		g.broadcastState()
		return
	}

	// Second card flipped - check for match
	card1 := &g.Board.Cards[g.FlippedIndices[0]]
	card2 := &g.Board.Cards[g.FlippedIndices[1]]

	if card1.PairID == card2.PairID {
		// Match found!
		card1.State = Matched
		card2.State = Matched

		player := g.Players[playerIdx]
		points := 1
		player.Score += points
		// Leech: subtract same amount from opponent (minimum 0)
		if player.LeechActive {
			opponent := g.Players[1-playerIdx]
			opponent.Score -= points
			if opponent.Score < 0 {
				opponent.Score = 0
			}
		}
		// Blood Pact: count consecutive matches; at 3 grant +5 and clear
		if player.BloodPactActive {
			player.BloodPactMatchesCount++
			if player.BloodPactMatchesCount >= 3 {
				player.Score += 5
				g.broadcastPowerUpEffectResolved(player.Name, "Blood Pact", player.Name+" honored the Pact and gained 5 points")
				player.BloodPactActive = false
				player.BloodPactMatchesCount = 0
			}
		}

		// Grant power-up for this pair if mapped (pairId 0, 1, 2 -> first power-ups in registry order)
		if powerUpID, ok := g.PairIDToPowerUp[card1.PairID]; ok {
			if player.Hand == nil {
				player.Hand = make(map[string]int)
			}
			if player.HandCooldown == nil {
				player.HandCooldown = make(map[string]int)
			}
			player.Hand[powerUpID]++
			player.HandCooldown[powerUpID]++
		}

		g.FlippedIndices = g.FlippedIndices[:0]
		g.TurnPhase = FirstFlip

		// End of match: clear highlight; Leech lasts whole turn (cleared on mismatch/timeout)
		player.HighlightIndices = nil
		// Blood Pact is not cleared on match; only on mismatch or timeout

		// Check if game is over
		if AllMatched(g.Board) {
			g.cancelTurnTimer()
			g.broadcastState()
			g.broadcastGameOver()
			g.Finished = true
			return
		}

		// Same player keeps the turn; reset turn timer
		g.cancelTurnTimer()
		g.startTurnTimer()
		g.broadcastState()
	} else {
		// No match - enter resolve phase, broadcast the revealed state,
		// then schedule hiding the cards after the reveal duration.
		g.TurnPhase = Resolve
		g.broadcastState()

		// Schedule the mismatch resolution via the actions channel
		// so it is processed serially.
		go func(pIdx int, revealMS int) {
			time.Sleep(time.Duration(revealMS) * time.Millisecond)
			// Send resolve action. If the game is already finished or the channel
			// is closed, this is safely ignored via the select default.
			select {
			case g.Actions <- Action{Type: ActionResolveMismatch, PlayerIdx: pIdx}:
			case <-g.Done:
			}
		}(playerIdx, g.Config.RevealDurationMS)
	}
}

func (g *Game) handleResolveMismatch(playerIdx int) {
	// Turn may have already passed (e.g. due to turn timeout)
	if g.CurrentTurn != playerIdx {
		return
	}
	player := g.Players[playerIdx]

	// Hide the two flipped cards
	for _, idx := range g.FlippedIndices {
		g.Board.Cards[idx].State = Hidden
	}

	// End of turn: clear highlight and Leech (effects last only this turn)
	player.HighlightIndices = nil
	player.LeechActive = false
	// Blood Pact: failed (mismatch); lose 3 points and clear pact
	if player.BloodPactActive {
		player.Score -= 3
		if player.Score < 0 {
			player.Score = 0
		}
		g.broadcastPowerUpEffectResolved(player.Name, "Blood Pact", player.Name+" broke the Pact and lost 3 points")
		player.BloodPactActive = false
		player.BloodPactMatchesCount = 0
	}

	g.FlippedIndices = g.FlippedIndices[:0]
	// Record turn telemetry for the turn that just ended (before advancing Round/CurrentTurn)
	if g.TelemetrySink != nil {
		pidx := g.CurrentTurn
		scoreAfter := g.Players[pidx].Score
		oppScoreAfter := g.Players[1-pidx].Score
		deltaPlayer := scoreAfter - g.TurnStartScores[pidx]
		deltaOpponent := oppScoreAfter - g.TurnStartScores[1-pidx]
		g.TelemetrySink.RecordTurn(g.ID, g.Round, pidx, scoreAfter, oppScoreAfter, deltaPlayer, deltaOpponent)
	}
	g.Round++
	g.CurrentTurn = 1 - g.CurrentTurn
	g.TurnPhase = FirstFlip
	g.TurnStartScores[0] = g.Players[0].Score
	g.TurnStartScores[1] = g.Players[1].Score

	g.clearHandCooldownForPlayer(g.CurrentTurn)
	g.cancelTurnTimer()
	g.startTurnTimer()
	g.broadcastState()
}

func (g *Game) clearHandCooldownForPlayer(playerIdx int) {
	if p := g.Players[playerIdx]; p != nil && p.HandCooldown != nil {
		p.HandCooldown = make(map[string]int)
	}
}

func (g *Game) handleTurnTimeout() {
	// Timer may have been cancelled (e.g. resolve already switched turn)
	if g.turnTimerCancel == nil {
		return
	}
	g.cancelTurnTimer()

	g.broadcastTurnTimeout()

	// Hide any flipped cards
	for _, idx := range g.FlippedIndices {
		g.Board.Cards[idx].State = Hidden
	}
	player := g.Players[g.CurrentTurn]
	if player != nil {
		// End of turn: clear highlight and Leech (effects last only this turn)
		player.HighlightIndices = nil
		player.LeechActive = false
		// Blood Pact: turn timeout counts as failure; lose 3 points and clear pact
		if player.BloodPactActive {
			player.Score -= 3
			if player.Score < 0 {
				player.Score = 0
			}
			g.broadcastPowerUpEffectResolved(player.Name, "Blood Pact", player.Name+" broke the Pact and lost 3 points")
			player.BloodPactActive = false
			player.BloodPactMatchesCount = 0
		}
	}
	g.FlippedIndices = g.FlippedIndices[:0]
	// Record turn telemetry for the turn that just ended (before advancing Round/CurrentTurn)
	if g.TelemetrySink != nil {
		pidx := g.CurrentTurn
		scoreAfter := g.Players[pidx].Score
		oppScoreAfter := g.Players[1-pidx].Score
		deltaPlayer := scoreAfter - g.TurnStartScores[pidx]
		deltaOpponent := oppScoreAfter - g.TurnStartScores[1-pidx]
		g.TelemetrySink.RecordTurn(g.ID, g.Round, pidx, scoreAfter, oppScoreAfter, deltaPlayer, deltaOpponent)
	}
	g.Round++
	g.CurrentTurn = 1 - g.CurrentTurn
	g.TurnPhase = FirstFlip
	g.TurnStartScores[0] = g.Players[0].Score
	g.TurnStartScores[1] = g.Players[1].Score

	g.clearHandCooldownForPlayer(g.CurrentTurn)
	g.startTurnTimer()
	g.broadcastState()
}

func (g *Game) broadcastTurnTimeout() {
	msg := map[string]string{"type": "turn_timeout"}
	data, _ := json.Marshal(msg)
	for i := 0; i < 2; i++ {
		if g.Players[i] != nil && g.Players[i].Send != nil {
			wsutil.SafeSend(g.Players[i].Send, data)
		}
	}
}

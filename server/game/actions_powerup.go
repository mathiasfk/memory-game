package game

import (
	"time"
)

func (g *Game) handleUsePowerUp(playerIdx int, powerUpID string, cardIndex int) {
	// Validate it's this player's turn
	if playerIdx != g.CurrentTurn {
		g.sendError(playerIdx, "It is not your turn.")
		return
	}

	// Validate turn phase must be FirstFlip (before any card is flipped)
	if g.TurnPhase != FirstFlip {
		g.sendError(playerIdx, "Power-ups can only be used before flipping any card.")
		return
	}

	// Look up the power-up
	pup, ok := g.PowerUps.GetPowerUp(powerUpID)
	if !ok {
		g.sendError(playerIdx, "Unknown power-up.")
		return
	}

	player := g.Players[playerIdx]
	if player.Hand[powerUpID] < 1 {
		g.sendError(playerIdx, "You don't have this power-up in hand.")
		return
	}
	cooldown := 0
	if player.HandCooldown != nil {
		cooldown = player.HandCooldown[powerUpID]
	}
	if player.Hand[powerUpID]-cooldown < 1 {
		g.sendError(playerIdx, "This arcana can only be used on your next turn.")
		return
	}

	totalCards := g.Config.BoardRows * g.Config.BoardCols

	// Clairvoyance: require a valid card target (hidden card)
	if powerUpID == "clairvoyance" {
		if cardIndex < 0 || cardIndex >= totalCards {
			g.sendError(playerIdx, "Clairvoyance requires a valid card target.")
			return
		}
		if g.Board.Cards[cardIndex].State != Hidden {
			g.sendError(playerIdx, "Clairvoyance target card must be hidden.")
			return
		}
	}
	// Oblivion: require a valid hidden card target
	if powerUpID == "oblivion" {
		if cardIndex < 0 || cardIndex >= totalCards {
			g.sendError(playerIdx, "Oblivion requires a valid card target.")
			return
		}
		if g.Board.Cards[cardIndex].State != Hidden {
			g.sendError(playerIdx, "Oblivion target card must be hidden.")
			return
		}
	}

	// Consume one from hand
	player.Hand[powerUpID]--
	if player.Hand[powerUpID] == 0 {
		delete(player.Hand, powerUpID)
	}

	// Clairvoyance: reveal 3x3 region and schedule hiding after duration
	var clairvoyanceRevealIndices []int
	if powerUpID == "clairvoyance" {
		region := RadarRegionIndices(g.Board, cardIndex)
		for _, idx := range region {
			c := &g.Board.Cards[idx]
			if c.State == Hidden {
				c.State = Revealed
				clairvoyanceRevealIndices = append(clairvoyanceRevealIndices, idx)
				if g.KnownIndices != nil {
					g.KnownIndices[idx] = struct{}{}
				}
			}
		}
	}
	// Oblivion: remove target tile and its pair from the game (no points)
	if powerUpID == "oblivion" {
		targetPairID := g.Board.Cards[cardIndex].PairID
		for i := range g.Board.Cards {
			if g.Board.Cards[i].PairID == targetPairID {
				g.Board.Cards[i].State = Removed
			}
		}
	}

	// Apply effect (Clairvoyance has no-op Apply; logic is above)
	opponent := g.Players[1-playerIdx]
	selfPairID := -1
	for pairID, id := range g.PairIDToPowerUp {
		if id == powerUpID {
			selfPairID = pairID
			break
		}
	}
	playerScoreBefore := g.Players[playerIdx].Score
	opponentScoreBefore := g.Players[1-playerIdx].Score
	pairsMatchedBefore := CountMatchedPairs(g.Board)

	ctx := &PowerUpContext{SelfPairID: selfPairID}
	if err := pup.Apply(g.Board, player, opponent, ctx); err != nil {
		// Revert Clairvoyance reveals if any; power-up already consumed
		for _, idx := range clairvoyanceRevealIndices {
			g.Board.Cards[idx].State = Hidden
		}
		g.sendError(playerIdx, "Power-up failed: "+err.Error())
		return
	}

	if g.TelemetrySink != nil {
		targetIdx := cardIndex
		if powerUpID != "clairvoyance" && powerUpID != "oblivion" {
			targetIdx = -1
		}
		g.TelemetrySink.RecordArcanaUse(g.ID, g.Round, playerIdx, powerUpID, targetIdx, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore)
	}

	// Chaos: clear known indices and highlight for both players
	if powerUpID == "chaos" {
		g.KnownIndices = make(map[int]struct{})
		for i := 0; i < 2; i++ {
			if g.Players[i] != nil {
				g.Players[i].HighlightIndices = nil
			}
		}
	}

	// Elemental powerups: highlight all tiles of the chosen element (this turn only; no symbol reveal)
	if powerUpID == "earth_elemental" || powerUpID == "fire_elemental" || powerUpID == "water_elemental" || powerUpID == "air_elemental" {
		var targetElement string
		switch powerUpID {
		case "earth_elemental":
			targetElement = ElementEarth
		case "fire_elemental":
			targetElement = ElementFire
		case "water_elemental":
			targetElement = ElementWater
		case "air_elemental":
			targetElement = ElementAir
		default:
			targetElement = ""
		}
		if targetElement != "" {
			var indices []int
			for i := range g.Board.Cards {
				c := &g.Board.Cards[i]
				if c.State != Removed && c.Element == targetElement {
					indices = append(indices, i)
				}
			}
			player.HighlightIndices = indices
		}
	}

	// Unveiling: highlight all hidden tiles that have never been revealed (this turn only)
	if powerUpID == "unveiling" {
		var indices []int
		for i := range g.Board.Cards {
			c := &g.Board.Cards[i]
			if c.State == Hidden && !g.isKnown(i) {
				indices = append(indices, i)
			}
		}
		player.HighlightIndices = indices
	}
	// Leech: this turn, match points are subtracted from opponent
	if powerUpID == "leech" {
		player.LeechActive = true
	}
	// Blood Pact: next 3 matches grant +5; first mismatch or timeout loses 3
	if powerUpID == "blood_pact" {
		player.BloodPactActive = true
		player.BloodPactMatchesCount = 0
	}

	// Determine if the power-up had no effect (for UX message)
	noEffect := false
	switch {
	case powerUpID == "clairvoyance":
		noEffect = len(clairvoyanceRevealIndices) == 0
	case powerUpID == "unveiling", powerUpID == "earth_elemental", powerUpID == "fire_elemental", powerUpID == "water_elemental", powerUpID == "air_elemental":
		noEffect = len(player.HighlightIndices) == 0
	}
	powerUpLabel := pup.Name
	g.broadcastPowerUpUsed(player.Name, powerUpLabel, noEffect)

	// Broadcast updated state (turn does not end)
	g.broadcastState()

	// Oblivion may have removed the last pair(s); check for game over
	if powerUpID == "oblivion" && AllMatched(g.Board) {
		g.cancelTurnTimer()
		g.broadcastGameOver()
		g.Finished = true
		return
	}

	// Clairvoyance: schedule hiding the revealed cards after duration
	if powerUpID == "clairvoyance" && len(clairvoyanceRevealIndices) > 0 {
		durationMS := g.Config.PowerUps.Clairvoyance.RevealDurationMS
		if durationMS <= 0 {
			durationMS = 1000
		}
		indices := make([]int, len(clairvoyanceRevealIndices))
		copy(indices, clairvoyanceRevealIndices)
		go func() {
			time.Sleep(time.Duration(durationMS) * time.Millisecond)
			select {
			case g.Actions <- Action{Type: ActionHideClairvoyanceReveal, ClairvoyanceRevealIndices: indices}:
			case <-g.Done:
			}
		}()
	}
}

// handleHideClairvoyanceReveal hides cards that were temporarily revealed by Clairvoyance.
// Only cards still Revealed and not in FlippedIndices are hidden.
func (g *Game) handleHideClairvoyanceReveal(indices []int) {
	flippedSet := make(map[int]bool)
	for _, idx := range g.FlippedIndices {
		flippedSet[idx] = true
	}
	for _, idx := range indices {
		if idx < 0 || idx >= len(g.Board.Cards) {
			continue
		}
		c := &g.Board.Cards[idx]
		if c.State == Revealed && !flippedSet[idx] {
			c.State = Hidden
		}
	}
	g.broadcastState()
}

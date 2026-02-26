package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"memory-game-server/config"
	"memory-game-server/wsutil"
)

// TurnPhase represents the current phase within a turn.
type TurnPhase int

const (
	FirstFlip  TurnPhase = iota
	SecondFlip
	Resolve
)

// String returns the protocol string for a TurnPhase.
func (tp TurnPhase) String() string {
	switch tp {
	case FirstFlip:
		return "first_flip"
	case SecondFlip:
		return "second_flip"
	case Resolve:
		return "resolve"
	default:
		return "unknown"
	}
}

// ActionType enumerates the kinds of actions a game can process.
type ActionType int

const (
	ActionFlipCard ActionType = iota
	ActionUsePowerUp
	ActionDisconnect
	ActionPlayerDisconnected   // player lost connection; start reconnection window
	ActionReconnectionTimeout // reconnection window expired; end game
	ActionRejoinCompleted      // player rejoined; restore Send and clear disconnect state
	ActionResolveMismatch      // internal: fired after reveal timer expires
	ActionHideClairvoyanceReveal  // internal: hide cards that were temporarily revealed by Clairvoyance
	ActionTurnTimeout          // internal: fired when turn time limit is reached
)

// Action represents a player action sent into the game's action channel.
type Action struct {
	Type               ActionType
	PlayerIdx          int       // 0 or 1
	Index              int       // card index (for FlipCard)
	PowerUpID          string    // power-up ID (for UsePowerUp)
	CardIndex             int       // card index for power-ups that need a target (e.g. Clairvoyance); -1 when not used
	ClairvoyanceRevealIndices []int // indices to hide (for ActionHideClairvoyanceReveal)
	NewSend            chan []byte // for ActionRejoinCompleted: new send channel for the reconnected player
}

// ArcanaPairsPerMatch is the number of board pairs that grant power-ups in each match.
const ArcanaPairsPerMatch = 6

// PowerUpProvider abstracts the power-up registry so the game package
// does not import the powerup package directly (avoids circular deps).
type PowerUpProvider interface {
	GetPowerUp(id string) (PowerUpDef, bool)
	AllPowerUps() []PowerUpDef
	PickArcanaForMatch(n int) []PowerUpDef
}

// PowerUpContext is passed to power-up Apply when the game has context (e.g. which pairID is the power-up tile).
type PowerUpContext struct {
	// SelfPairID is the pairID of the power-up tile on the board for the power-up being used; -1 if not applicable.
	SelfPairID int
}

// PowerUpDef holds the definition of a power-up as seen by the game package.
// Rarity is used for weighted selection when picking arcana for a match (higher = more likely).
type PowerUpDef struct {
	ID          string
	Name        string
	Description string
	Cost        int
	Rarity      int
	Apply       func(board *Board, active *Player, opponent *Player, ctx *PowerUpContext) error
}

// Game manages a single match between two players.
type Game struct {
	ID             string
	Board          *Board
	Players        [2]*Player
	CurrentTurn    int
	TurnPhase      TurnPhase
	FlippedIndices []int
	Config         *config.Config
	PowerUps       PowerUpProvider
	Finished       bool

	// PairIDToPowerUp maps board pairId (0, 1, 2, ...) to power-up ID for this match. Filled in NewGame from registry order.
	PairIDToPowerUp map[int]string

	// KnownIndices are card indices that have been revealed at some point (by any player). Cleared when Chaos is used.
	KnownIndices map[int]struct{}

	// Round increments each time the turn passes after a mismatch (used for Second Chance duration).
	Round int

	// turnEndsAt is when the current turn ends (zero = timer disabled).
	turnEndsAt        time.Time
	turnTimerCancel   chan struct{}

	// RejoinTokens allow a disconnected player to rejoin; set by matchmaker.
	RejoinTokens [2]string

	// PlayerUserIDs are the auth user IDs for each seat (index 0 and 1); used for rejoin by user (cross-device). Set by matchmaker.
	PlayerUserIDs [2]string

	// DisconnectedPlayerIdx is the player who lost connection (-1 = none); game is paused until rejoin or timeout.
	DisconnectedPlayerIdx  int
	ReconnectionDeadline   time.Time
	reconnectionTimerCancel chan struct{}

	Actions chan Action
	Done    chan struct{}

	// OnGameEnd is called when the game ends (normal finish or opponent disconnect). winnerIndex is 0, 1, or -1 for draw.
	OnGameEnd func(gameID, player0UserID, player1UserID, player0Name, player1Name string, player0Score, player1Score int, winnerIndex int, endReason string)
}

// NewGame creates a new Game between two players.
func NewGame(id string, cfg *config.Config, p0, p1 *Player, pups PowerUpProvider) *Game {
	board := NewBoard(cfg.BoardRows, cfg.BoardCols, ArcanaPairsPerMatch)
	firstTurn := rand.Intn(2)

	pairIDToPowerUp := make(map[int]string)
	if pups != nil {
		arcana := pups.PickArcanaForMatch(ArcanaPairsPerMatch)
		for i, pup := range arcana {
			pairIDToPowerUp[i] = pup.ID
		}
	}

	knownIndices := make(map[int]struct{})

	return &Game{
		ID:                id,
		Board:             board,
		Players:           [2]*Player{p0, p1},
		CurrentTurn:       firstTurn,
		TurnPhase:         FirstFlip,
		FlippedIndices:    make([]int, 0, 2),
		Config:            cfg,
		PowerUps:          pups,
		Finished:          false,
		PairIDToPowerUp:   pairIDToPowerUp,
		KnownIndices:      knownIndices,
		DisconnectedPlayerIdx: -1,
		Actions:           make(chan Action, 16),
		Done:              make(chan struct{}),
	}
}

// Run is the main game loop. It processes actions sequentially.
// It should be run as a goroutine.
func (g *Game) Run() {
	defer close(g.Done)

	// Broadcast initial game state to both players
	g.broadcastState()
	g.startTurnTimer()

	for {
		action, ok := <-g.Actions
		if !ok || g.Finished {
			return
		}
		switch action.Type {
		case ActionFlipCard:
			if g.DisconnectedPlayerIdx >= 0 {
				continue
			}
			g.handleFlipCard(action.PlayerIdx, action.Index)
		case ActionUsePowerUp:
			if g.DisconnectedPlayerIdx >= 0 {
				continue
			}
			g.handleUsePowerUp(action.PlayerIdx, action.PowerUpID, action.CardIndex)
		case ActionDisconnect:
			g.handleDisconnect(action.PlayerIdx)
			return
		case ActionPlayerDisconnected:
			g.handlePlayerDisconnected(action.PlayerIdx)
		case ActionReconnectionTimeout:
			g.handleReconnectionTimeout()
			return
		case ActionRejoinCompleted:
			g.handleRejoinCompleted(action.PlayerIdx, action.NewSend)
		case ActionResolveMismatch:
			g.handleResolveMismatch(action.PlayerIdx)
		case ActionHideClairvoyanceReveal:
			g.handleHideClairvoyanceReveal(action.ClairvoyanceRevealIndices)
		case ActionTurnTimeout:
			g.handleTurnTimeout()
		}
		if g.Finished {
			return
		}
	}
}

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
		player.ComboStreak++
		points := g.Config.ComboBasePoints * player.ComboStreak
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
				player.BloodPactActive = false
				player.BloodPactMatchesCount = 0
			}
		}

		// Grant power-up for this pair if mapped (pairId 0, 1, 2 -> first power-ups in registry order)
		if powerUpID, ok := g.PairIDToPowerUp[card1.PairID]; ok {
			if player.Hand == nil {
				player.Hand = make(map[string]int)
			}
			player.Hand[powerUpID]++
		}

		g.FlippedIndices = g.FlippedIndices[:0]
		g.TurnPhase = FirstFlip

		// End of turn: clear highlight and Leech for current player (effects last only this turn)
		player.HighlightIndices = nil
		player.LeechActive = false
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

	// Reset combo for current player
	player.ComboStreak = 0

	// End of turn: clear highlight and Leech (effects last only this turn)
	player.HighlightIndices = nil
	player.LeechActive = false
	// Blood Pact: failed (mismatch); lose 3 points and clear pact
	if player.BloodPactActive {
		player.Score -= 3
		if player.Score < 0 {
			player.Score = 0
		}
		player.BloodPactActive = false
		player.BloodPactMatchesCount = 0
	}

	g.FlippedIndices = g.FlippedIndices[:0]
	g.Round++
	g.CurrentTurn = 1 - g.CurrentTurn
	g.TurnPhase = FirstFlip

	g.cancelTurnTimer()
	g.startTurnTimer()
	g.broadcastState()
}

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
	ctx := &PowerUpContext{SelfPairID: selfPairID}
	if err := pup.Apply(g.Board, player, opponent, ctx); err != nil {
		// Revert Clairvoyance reveals if any; power-up already consumed
		for _, idx := range clairvoyanceRevealIndices {
			g.Board.Cards[idx].State = Hidden
		}
		g.sendError(playerIdx, "Power-up failed: "+err.Error())
		return
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

// cancelTurnTimer closes the turn timer cancel channel so the timer goroutine exits. Safe if already nil.
func (g *Game) cancelTurnTimer() {
	if g.turnTimerCancel != nil {
		close(g.turnTimerCancel)
		g.turnTimerCancel = nil
	}
	g.turnEndsAt = time.Time{}
}

// startTurnTimer starts a timer for the current turn. If it expires, ActionTurnTimeout is sent.
// No-op if Config.TurnLimitSec <= 0. Cancels any existing turn timer first.
func (g *Game) startTurnTimer() {
	if g.Config.TurnLimitSec <= 0 {
		return
	}
	g.cancelTurnTimer()
	g.turnEndsAt = time.Now().Add(time.Duration(g.Config.TurnLimitSec) * time.Second)
	g.turnTimerCancel = make(chan struct{})
	cancel := g.turnTimerCancel
	limit := time.Duration(g.Config.TurnLimitSec) * time.Second
	go func() {
		select {
		case <-time.After(limit):
			select {
			case g.Actions <- Action{Type: ActionTurnTimeout}:
			case <-g.Done:
			}
		case <-cancel:
		}
	}()
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
		player.ComboStreak = 0
		// End of turn: clear highlight and Leech (effects last only this turn)
		player.HighlightIndices = nil
		player.LeechActive = false
		// Blood Pact: turn timeout counts as failure; lose 3 points and clear pact
		if player.BloodPactActive {
			player.Score -= 3
			if player.Score < 0 {
				player.Score = 0
			}
			player.BloodPactActive = false
			player.BloodPactMatchesCount = 0
		}
	}
	g.FlippedIndices = g.FlippedIndices[:0]
	g.Round++
	g.CurrentTurn = 1 - g.CurrentTurn
	g.TurnPhase = FirstFlip

	g.startTurnTimer()
	g.broadcastState()
}

func (g *Game) handleDisconnect(playerIdx int) {
	g.Finished = true
	opponentIdx := 1 - playerIdx
	if g.OnGameEnd != nil {
		g.OnGameEnd(g.ID, g.PlayerUserIDs[0], g.PlayerUserIDs[1], g.Players[0].Name, g.Players[1].Name, g.Players[0].Score, g.Players[1].Score, opponentIdx, "opponent_disconnected")
	}
	// Notify the opponent
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]string{"type": "opponent_disconnected"}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
}

func (g *Game) cancelReconnectionTimer() {
	if g.reconnectionTimerCancel != nil {
		close(g.reconnectionTimerCancel)
		g.reconnectionTimerCancel = nil
	}
	g.DisconnectedPlayerIdx = -1
}

func (g *Game) handlePlayerDisconnected(playerIdx int) {
	if g.DisconnectedPlayerIdx >= 0 {
		return
	}
	g.cancelTurnTimer()
	g.DisconnectedPlayerIdx = playerIdx
	timeoutSec := g.Config.ReconnectTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	g.ReconnectionDeadline = time.Now().Add(time.Duration(timeoutSec) * time.Second)
	opponentIdx := 1 - playerIdx
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]interface{}{
			"type":                        "opponent_reconnecting",
			"reconnectionDeadlineUnixMs": g.ReconnectionDeadline.UnixMilli(),
		}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
	g.reconnectionTimerCancel = make(chan struct{})
	cancel := g.reconnectionTimerCancel
	go func() {
		select {
		case <-time.After(time.Duration(timeoutSec) * time.Second):
			select {
			case g.Actions <- Action{Type: ActionReconnectionTimeout}:
			case <-g.Done:
			}
		case <-cancel:
		}
	}()
}

func (g *Game) handleReconnectionTimeout() {
	idx := g.DisconnectedPlayerIdx
	g.cancelReconnectionTimer()
	g.handleDisconnect(idx)
}

func (g *Game) handleRejoinCompleted(playerIdx int, newSend chan []byte) {
	g.cancelReconnectionTimer()
	if playerIdx >= 0 && playerIdx <= 1 && g.Players[playerIdx] != nil && newSend != nil {
		g.Players[playerIdx].Send = newSend
	}
	opponentIdx := 1 - playerIdx
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]string{"type": "opponent_reconnected"}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
	g.startTurnTimer()
	g.broadcastState()
}

func (g *Game) sendError(playerIdx int, message string) {
	player := g.Players[playerIdx]
	if player == nil || player.Send == nil {
		return
	}
	msg := map[string]string{
		"type":    "error",
		"message": message,
	}
	data, _ := json.Marshal(msg)
	wsutil.SafeSend(player.Send, data)
}

func (g *Game) broadcastState() {
	for i := 0; i < 2; i++ {
		state := g.BuildStateForPlayer(i)
		data, err := json.Marshal(state)
		if err != nil {
			log.Printf("Error marshaling game state: %v", err)
			continue
		}
		if g.Players[i] != nil && g.Players[i].Send != nil {
			wsutil.SafeSend(g.Players[i].Send, data)
		}
	}
}

// isKnown returns whether the card at index idx has ever been revealed (used for Unveiling highlight).
func (g *Game) isKnown(idx int) bool {
	if g.KnownIndices == nil {
		return false
	}
	_, ok := g.KnownIndices[idx]
	return ok
}

// BuildStateForPlayer returns the game state view for the given player (0 or 1).
func (g *Game) BuildStateForPlayer(playerIdx int) GameStateMsg {
	opponentIdx := 1 - playerIdx

	// Build hand from player's power-up hand, in registry order so it stays stable across turn changes.
	h := g.Players[playerIdx].Hand
	hand := make([]PowerUpInHand, 0, len(h))
	for _, def := range g.PowerUps.AllPowerUps() {
		if count := h[def.ID]; count > 0 {
			hand = append(hand, PowerUpInHand{PowerUpID: def.ID, Count: count})
		}
	}

	flipped := g.FlippedIndices
	if flipped == nil {
		flipped = []int{}
	}

	var knownIndices []int
	if len(g.KnownIndices) > 0 {
		knownIndices = make([]int, 0, len(g.KnownIndices))
		for idx := range g.KnownIndices {
			knownIndices = append(knownIndices, idx)
		}
	}

	state := GameStateMsg{
		Type:                        "game_state",
		Cards:                       BuildCardViews(g.Board),
		You:                         BuildPlayerView(g.Players[playerIdx], g.Round),
		Opponent:                    BuildPlayerView(g.Players[opponentIdx], g.Round),
		YourTurn:                    playerIdx == g.CurrentTurn,
		Hand:                        hand,
		FlippedIndices:              flipped,
		Phase:                       g.TurnPhase.String(),
		KnownIndices:     knownIndices,
		PairIDToPowerUp:  g.PairIDToPowerUp,
		HighlightIndices: g.Players[playerIdx].HighlightIndices,
	}
	if playerIdx == g.CurrentTurn && !g.turnEndsAt.IsZero() && g.Config.TurnLimitSec > 0 {
		state.TurnEndsAtUnixMs = g.turnEndsAt.UnixMilli()
		state.TurnCountdownShowSec = g.Config.TurnCountdownShowSec
	}
	return state
}

func (g *Game) broadcastGameOver() {
	for i := 0; i < 2; i++ {
		opponentIdx := 1 - i
		var result string
		if g.Players[i].Score > g.Players[opponentIdx].Score {
			result = "win"
		} else if g.Players[i].Score < g.Players[opponentIdx].Score {
			result = "lose"
		} else {
			result = "draw"
		}

		msg := map[string]interface{}{
			"type":   "game_over",
			"result": result,
			"you": map[string]interface{}{
				"name":  g.Players[i].Name,
				"score": g.Players[i].Score,
			},
			"opponent": map[string]interface{}{
				"name":  g.Players[opponentIdx].Name,
				"score": g.Players[opponentIdx].Score,
			},
		}
		data, _ := json.Marshal(msg)
		if g.Players[i] != nil && g.Players[i].Send != nil {
			wsutil.SafeSend(g.Players[i].Send, data)
		}
	}
	// Persist once for history (winnerIndex: 0, 1, or -1 for draw)
	if g.OnGameEnd != nil {
		winnerIdx := -1
		if g.Players[0].Score > g.Players[1].Score {
			winnerIdx = 0
		} else if g.Players[1].Score > g.Players[0].Score {
			winnerIdx = 1
		}
		g.OnGameEnd(g.ID, g.PlayerUserIDs[0], g.PlayerUserIDs[1], g.Players[0].Name, g.Players[1].Name, g.Players[0].Score, g.Players[1].Score, winnerIdx, "completed")
	}
}

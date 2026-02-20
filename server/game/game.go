package game

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"memory-game-server/config"
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
	ActionHideRadarReveal       // internal: hide cards that were temporarily revealed by Radar
	ActionTurnTimeout          // internal: fired when turn time limit is reached
)

// Action represents a player action sent into the game's action channel.
type Action struct {
	Type               ActionType
	PlayerIdx          int       // 0 or 1
	Index              int       // card index (for FlipCard)
	PowerUpID          string    // power-up ID (for UsePowerUp)
	CardIndex          int       // card index for power-ups that need a target (e.g. Radar); -1 when not used
	RadarRevealIndices []int     // indices to hide (for ActionHideRadarReveal)
	NewSend            chan []byte // for ActionRejoinCompleted: new send channel for the reconnected player
}

// PowerUpProvider abstracts the power-up registry so the game package
// does not import the powerup package directly (avoids circular deps).
type PowerUpProvider interface {
	GetPowerUp(id string) (PowerUpDef, bool)
	AllPowerUps() []PowerUpDef
}

// PowerUpDef holds the definition of a power-up as seen by the game package.
type PowerUpDef struct {
	ID          string
	Name        string
	Description string
	Cost        int
	Apply       func(board *Board, active *Player, opponent *Player) error
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
}

// NewGame creates a new Game between two players.
func NewGame(id string, cfg *config.Config, p0, p1 *Player, pups PowerUpProvider) *Game {
	board := NewBoard(cfg.BoardRows, cfg.BoardCols)
	firstTurn := rand.Intn(2)

	return &Game{
		ID:                    id,
		Board:                 board,
		Players:               [2]*Player{p0, p1},
		CurrentTurn:           firstTurn,
		TurnPhase:             FirstFlip,
		FlippedIndices:        make([]int, 0, 2),
		Config:                cfg,
		PowerUps:              pups,
		Finished:              false,
		DisconnectedPlayerIdx: -1,
		Actions:               make(chan Action, 16),
		Done:                  make(chan struct{}),
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
		case ActionHideRadarReveal:
			g.handleHideRadarReveal(action.RadarRevealIndices)
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

	// Validate card is hidden
	card := &g.Board.Cards[cardIndex]
	if card.State != Hidden {
		g.sendError(playerIdx, "That card is already revealed or matched.")
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
		player.Score += g.Config.ComboBasePoints * player.ComboStreak

		g.FlippedIndices = g.FlippedIndices[:0]
		g.TurnPhase = FirstFlip

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

	// Second Chance: +1 point per error while active
	if player.SecondChanceActiveUntilRound > 0 && g.Round <= player.SecondChanceActiveUntilRound {
		player.Score += 1
	}

	// Hide the two flipped cards
	for _, idx := range g.FlippedIndices {
		g.Board.Cards[idx].State = Hidden
	}

	// Reset combo for current player
	player.ComboStreak = 0

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

	// Validate the player can afford it
	player := g.Players[playerIdx]
	if player.Score < pup.Cost {
		g.sendError(playerIdx, "Not enough points to use this power-up.")
		return
	}

	totalCards := g.Config.BoardRows * g.Config.BoardCols

	// Radar: require a valid card target (hidden card)
	if powerUpID == "radar" {
		if cardIndex < 0 || cardIndex >= totalCards {
			g.sendError(playerIdx, "Radar requires a valid card target.")
			return
		}
		if g.Board.Cards[cardIndex].State != Hidden {
			g.sendError(playerIdx, "Radar target card must be hidden.")
			return
		}
	}

	// Deduct cost
	player.Score -= pup.Cost

	// Radar: reveal 3x3 region and schedule hiding after duration
	var radarRevealIndices []int
	if powerUpID == "radar" {
		region := RadarRegionIndices(g.Board, cardIndex)
		for _, idx := range region {
			c := &g.Board.Cards[idx]
			if c.State == Hidden {
				c.State = Revealed
				radarRevealIndices = append(radarRevealIndices, idx)
			}
		}
	}

	// Apply effect (Radar has no-op Apply; logic is above)
	opponent := g.Players[1-playerIdx]
	if err := pup.Apply(g.Board, player, opponent); err != nil {
		// Refund on error and revert Radar reveals if any
		player.Score += pup.Cost
		for _, idx := range radarRevealIndices {
			g.Board.Cards[idx].State = Hidden
		}
		g.sendError(playerIdx, "Power-up failed: "+err.Error())
		return
	}

	// Second Chance: activate for the next N rounds (game state, not board effect)
	if powerUpID == "second_chance" && g.Config.PowerUps.SecondChance.DurationRounds > 0 {
		player.SecondChanceActiveUntilRound = g.Round + g.Config.PowerUps.SecondChance.DurationRounds
	}

	// Broadcast updated state (turn does not end)
	g.broadcastState()

	// Radar: schedule hiding the revealed cards after duration
	if powerUpID == "radar" && len(radarRevealIndices) > 0 {
		durationMS := g.Config.PowerUps.Radar.RevealDurationMS
		if durationMS <= 0 {
			durationMS = 1000
		}
		indices := make([]int, len(radarRevealIndices))
		copy(indices, radarRevealIndices)
		go func() {
			time.Sleep(time.Duration(durationMS) * time.Millisecond)
			select {
			case g.Actions <- Action{Type: ActionHideRadarReveal, RadarRevealIndices: indices}:
			case <-g.Done:
			}
		}()
	}
}

// handleHideRadarReveal hides cards that were temporarily revealed by Radar.
// Only cards still Revealed and not in FlippedIndices are hidden.
func (g *Game) handleHideRadarReveal(indices []int) {
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
			safeSend(g.Players[i].Send, data)
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
	// Notify the opponent
	opponentIdx := 1 - playerIdx
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]string{"type": "opponent_disconnected"}
		data, _ := json.Marshal(msg)
		safeSend(opponent.Send, data)
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
		safeSend(opponent.Send, data)
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
		safeSend(opponent.Send, data)
	}
	g.startTurnTimer()
	g.broadcastState()
}

// safeSend sends data to a channel without panicking if the channel is closed.
func safeSend(ch chan []byte, data []byte) {
	defer func() {
		recover() // swallow panic from send on closed channel
	}()
	select {
	case ch <- data:
	default:
	}
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
	safeSend(player.Send, data)
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
			safeSend(g.Players[i].Send, data)
		}
	}
}

// BuildStateForPlayer returns the game state view for the given player (0 or 1).
func (g *Game) BuildStateForPlayer(playerIdx int) GameStateMsg {
	opponentIdx := 1 - playerIdx

	// Build available power-ups list
	var powerUpViews []PowerUpView
	if g.PowerUps != nil {
		allPups := g.PowerUps.AllPowerUps()
		powerUpViews = make([]PowerUpView, len(allPups))
		for i, pup := range allPups {
			powerUpViews[i] = PowerUpView{
				ID:          pup.ID,
				Name:        pup.Name,
				Description: pup.Description,
				Cost:        pup.Cost,
				CanAfford:   g.Players[playerIdx].Score >= pup.Cost,
			}
		}
	}

	flipped := g.FlippedIndices
	if flipped == nil {
		flipped = []int{}
	}

	state := GameStateMsg{
		Type:              "game_state",
		Cards:             BuildCardViews(g.Board),
		You:               BuildPlayerView(g.Players[playerIdx], g.Round),
		Opponent:          BuildPlayerView(g.Players[opponentIdx], g.Round),
		YourTurn:          playerIdx == g.CurrentTurn,
		AvailablePowerUps: powerUpViews,
		FlippedIndices:    flipped,
		Phase:             g.TurnPhase.String(),
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
			safeSend(g.Players[i].Send, data)
		}
	}
}

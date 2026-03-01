package game

import (
	"encoding/json"
	"log/slog"
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

// TelemetrySink is called to record turn and arcana use events. Optional; may be nil.
type TelemetrySink interface {
	RecordTurn(matchID string, round, playerIdx int, playerScoreAfter, opponentScoreAfter, deltaPlayer, deltaOpponent int)
	RecordArcanaUse(matchID string, round, playerIdx int, powerUpID string, targetCardIndex int, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore int)
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

	// TurnStartScores are the scores at the start of the current turn (for telemetry deltas).
	TurnStartScores [2]int

	// turnEndsAt is when the current turn ends (zero = timer disabled).
	turnEndsAt        time.Time
	turnTimerCancel   chan struct{}

	// TelemetrySink records turn and arcana use events; optional, set by matchmaker.
	TelemetrySink TelemetrySink

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
	g.TurnStartScores[0] = g.Players[0].Score
	g.TurnStartScores[1] = g.Players[1].Score
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

func (g *Game) broadcastPowerUpUsed(playerName, powerUpLabel string, noEffect bool) {
	msg := map[string]interface{}{
		"type":         "powerup_used",
		"playerName":  playerName,
		"powerUpLabel": powerUpLabel,
		"noEffect":     noEffect,
	}
	data, _ := json.Marshal(msg)
	for i := 0; i < 2; i++ {
		if g.Players[i] != nil && g.Players[i].Send != nil {
			wsutil.SafeSend(g.Players[i].Send, data)
		}
	}
}

// broadcastPowerUpEffectResolved notifies both players when a delayed powerup effect (e.g. Blood Pact) is resolved.
// message is the full announcement text, e.g. "Mathias honored the Pact and gained 5 points".
func (g *Game) broadcastPowerUpEffectResolved(playerName, powerUpLabel, message string) {
	msg := map[string]interface{}{
		"type":         "powerup_effect_resolved",
		"playerName":  playerName,
		"powerUpLabel": powerUpLabel,
		"message":      message,
	}
	data, _ := json.Marshal(msg)
	for i := 0; i < 2; i++ {
		if g.Players[i] != nil && g.Players[i].Send != nil {
			wsutil.SafeSend(g.Players[i].Send, data)
		}
	}
}

func (g *Game) broadcastState() {
	for i := 0; i < 2; i++ {
		state := g.BuildStateForPlayer(i)
		data, err := json.Marshal(state)
		if err != nil {
			slog.Error("marshaling game state", "tag", "game", "err", err)
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
	p := g.Players[playerIdx]
	h := p.Hand
	cooldown := p.HandCooldown
	if cooldown == nil {
		cooldown = make(map[string]int)
	}
	hand := make([]PowerUpInHand, 0, len(h))
	for _, def := range g.PowerUps.AllPowerUps() {
		if count := h[def.ID]; count > 0 {
			usable := count - cooldown[def.ID]
			if usable < 0 {
				usable = 0
			}
			hand = append(hand, PowerUpInHand{PowerUpID: def.ID, Count: count, UsableCount: usable})
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
		ArcanaPairs:      g.Board.ArcanaPairs,
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

package matchmaking

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"memory-game-server/ai"
	"memory-game-server/config"
	"memory-game-server/game"
	"memory-game-server/matcherrors"
	"memory-game-server/storage"
	"memory-game-server/ws"
	"memory-game-server/wsutil"

	"github.com/google/uuid"
)

// turnEvent and arcanaEvent hold telemetry data for async flush.
type turnEvent struct {
	matchID             string
	round               int
	playerIdx           int
	playerScoreAfter    int
	opponentScoreAfter  int
	deltaPlayer         int
	deltaOpponent       int
}

type arcanaEvent struct {
	matchID              string
	round                int
	playerIdx            int
	powerUpID            string
	targetCardIndex      int
	playerScoreBefore    int
	opponentScoreBefore  int
	pairsMatchedBefore   int
}

// queuedTelemetrySink implements game.TelemetrySink by enqueueing events and
// persisting them in a background goroutine (batch insert), so the game loop
// does not block on I/O.
type queuedTelemetrySink struct {
	store        *storage.Store
	mu           sync.Mutex
	turnEvents   []turnEvent
	arcanaEvents []arcanaEvent
}

// newQueuedTelemetrySink returns a sink that queues turn and arcana_use events.
// Events are persisted only when FlushMatch(matchID) is called (after InsertGameResult).
func newQueuedTelemetrySink(store *storage.Store) *queuedTelemetrySink {
	return &queuedTelemetrySink{
		store:        store,
		turnEvents:   nil,
		arcanaEvents: nil,
	}
}

// RecordTurn enqueues a turn event; non-blocking.
func (s *queuedTelemetrySink) RecordTurn(matchID string, round, playerIdx int, playerScoreAfter, opponentScoreAfter, deltaPlayer, deltaOpponent int) {
	s.mu.Lock()
	s.turnEvents = append(s.turnEvents, turnEvent{
		matchID:            matchID,
		round:              round,
		playerIdx:          playerIdx,
		playerScoreAfter:   playerScoreAfter,
		opponentScoreAfter: opponentScoreAfter,
		deltaPlayer:        deltaPlayer,
		deltaOpponent:      deltaOpponent,
	})
	s.mu.Unlock()
}

// RecordArcanaUse enqueues an arcana use event; non-blocking.
func (s *queuedTelemetrySink) RecordArcanaUse(matchID string, round, playerIdx int, powerUpID string, targetCardIndex int, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore int) {
	s.mu.Lock()
	s.arcanaEvents = append(s.arcanaEvents, arcanaEvent{
		matchID:             matchID,
		round:               round,
		playerIdx:           playerIdx,
		powerUpID:           powerUpID,
		targetCardIndex:     targetCardIndex,
		playerScoreBefore:   playerScoreBefore,
		opponentScoreBefore: opponentScoreBefore,
		pairsMatchedBefore:  pairsMatchedBefore,
	})
	s.mu.Unlock()
}

// FlushMatch persists queued turn and arcana_use events for the given match.
// Must be called after the game_history row exists (e.g. after InsertGameResult in OnGameEnd),
// since turn and arcana_use reference game_history(id).
func (s *queuedTelemetrySink) FlushMatch(matchID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	var turns []turnEvent
	var arcanas []arcanaEvent
	for _, e := range s.turnEvents {
		if e.matchID == matchID {
			turns = append(turns, e)
		}
	}
	for _, e := range s.arcanaEvents {
		if e.matchID == matchID {
			arcanas = append(arcanas, e)
		}
	}
	// Remove flushed events from queue
	newTurns := s.turnEvents[:0]
	for _, e := range s.turnEvents {
		if e.matchID != matchID {
			newTurns = append(newTurns, e)
		}
	}
	newArcanas := s.arcanaEvents[:0]
	for _, e := range s.arcanaEvents {
		if e.matchID != matchID {
			newArcanas = append(newArcanas, e)
		}
	}
	s.turnEvents = newTurns
	s.arcanaEvents = newArcanas
	s.mu.Unlock()
	ctx := context.Background()
	for _, e := range turns {
		_ = s.store.InsertTurn(ctx, e.matchID, e.round, e.playerIdx, e.playerScoreAfter, e.opponentScoreAfter, e.deltaPlayer, e.deltaOpponent)
	}
	for _, e := range arcanas {
		_ = s.store.InsertArcanaUse(ctx, e.matchID, e.round, e.playerIdx, e.powerUpID, e.targetCardIndex, e.playerScoreBefore, e.opponentScoreBefore, e.pairsMatchedBefore)
	}
}

// Matchmaker manages the queue of players waiting for a match.
type Matchmaker struct {
	waiting       map[*ws.Client]chan struct{} // client -> cancel channel (closed when client leaves queue)
	waitMu        sync.Mutex
	notify        chan struct{}               // buffered; signaled when a client is enqueued
	pendingClient *ws.Client                  // client currently waiting for pair (not in waiting map)
	pendingCancel chan struct{}               // closed when pending client cancels
	pendingMu     sync.Mutex
	config        *config.Config
	powerUps      game.PowerUpProvider
	historyStore  *storage.Store
	queuedSink    *queuedTelemetrySink        // async telemetry when historyStore != nil
	activeGames   map[string]*game.Game
	userIDToGame  map[string]string // userID -> gameID for rejoin by user (cross-device)
	mu            sync.RWMutex
}

// NewMatchmaker creates a new Matchmaker. historyStore may be nil to disable game history persistence.
// When historyStore is set, a shared queued telemetry sink is used; turn/arcana_use are persisted only
// when a game ends (FlushMatch after InsertGameResult), since those tables reference game_history(id).
func NewMatchmaker(cfg *config.Config, pups game.PowerUpProvider, historyStore *storage.Store) *Matchmaker {
	var queuedSink *queuedTelemetrySink
	if historyStore != nil {
		queuedSink = newQueuedTelemetrySink(historyStore)
	}
	return &Matchmaker{
		waiting:      make(map[*ws.Client]chan struct{}),
		notify:       make(chan struct{}, 1),
		config:       cfg,
		powerUps:     pups,
		historyStore: historyStore,
		queuedSink:   queuedSink,
		activeGames:  make(map[string]*game.Game),
		userIDToGame: make(map[string]string),
	}
}

func generateRejoinToken() (string, error) {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Enqueue adds a client to the matchmaking queue.
func (m *Matchmaker) Enqueue(c *ws.Client) {
	m.waitMu.Lock()
	defer m.waitMu.Unlock()
	if _, ok := m.waiting[c]; ok {
		return // already in queue
	}
	m.waiting[c] = make(chan struct{})
	log.Printf("[matchmaking] started for player %q (user=%s)", c.Name, c.UserID)
	select {
	case m.notify <- struct{}{}:
	default:
	}
}

// LeaveQueue removes a client from the matchmaking queue. Idempotent.
// The client may still be in waiting, or already be the "pending" client (taken by Run() and waiting for a second player or timeout).
func (m *Matchmaker) LeaveQueue(c *ws.Client) {
	m.waitMu.Lock()
	ch, ok := m.waiting[c]
	if ok {
		delete(m.waiting, c)
		m.waitMu.Unlock()
		close(ch)
		log.Printf("[matchmaking] cancelled for player %q (user=%s)", c.Name, c.UserID)
		return
	}
	m.waitMu.Unlock()

	m.pendingMu.Lock()
	if m.pendingClient == c && m.pendingCancel != nil {
		close(m.pendingCancel)
		m.pendingClient = nil
		m.pendingCancel = nil
		m.pendingMu.Unlock()
		log.Printf("[matchmaking] cancelled for player %q (user=%s)", c.Name, c.UserID)
		return
	}
	m.pendingMu.Unlock()
}

// Run is the matchmaker's main loop. It waits for a first player, then either
// a second player within AIPairTimeoutSec or starts a game vs the AI.
// Should be run as a goroutine.
func (m *Matchmaker) Run() {
	timeout := time.Duration(m.config.AIPairTimeoutSec) * time.Second
	if timeout < 0 {
		timeout = 0
	}
	for {
		<-m.notify
		m.waitMu.Lock()
		if len(m.waiting) == 0 {
			m.waitMu.Unlock()
			continue
		}
		var client1 *ws.Client
		var cancelCh1 chan struct{}
		for c, ch := range m.waiting {
			client1 = c
			cancelCh1 = ch
			delete(m.waiting, c)
			break
		}
		// If a second client is already waiting, pair immediately
		var client2 *ws.Client
		for c := range m.waiting {
			client2 = c
			delete(m.waiting, c)
			break
		}
		m.waitMu.Unlock()

		if client2 != nil {
			m.createGame(client1, client2)
			continue
		}

		// Register as pending so LeaveQueue can cancel us
		m.pendingMu.Lock()
		m.pendingClient = client1
		m.pendingCancel = cancelCh1
		m.pendingMu.Unlock()

		select {
		case <-m.notify:
			m.pendingMu.Lock()
			m.pendingClient = nil
			m.pendingCancel = nil
			m.pendingMu.Unlock()
			m.waitMu.Lock()
			client2 = nil
			for c := range m.waiting {
				client2 = c
				delete(m.waiting, c)
				break
			}
			m.waitMu.Unlock()
			if client2 != nil {
				m.createGame(client1, client2)
			} else {
				m.createGameVsAI(client1)
			}
		case <-time.After(timeout):
			m.pendingMu.Lock()
			m.pendingClient = nil
			m.pendingCancel = nil
			m.pendingMu.Unlock()
			m.createGameVsAI(client1)
		case <-cancelCh1:
			// client1 left queue (LeaveQueue closed the channel and logged)
			m.pendingMu.Lock()
			m.pendingClient = nil
			m.pendingCancel = nil
			m.pendingMu.Unlock()
		}
	}
}

func (m *Matchmaker) createGame(client1, client2 *ws.Client) {
	matchID := uuid.New().String()

	t0, _ := generateRejoinToken()
	t1, _ := generateRejoinToken()

	p0 := game.NewPlayer(client1.Name, client1.Send)
	p1 := game.NewPlayer(client2.Name, client2.Send)

	g := game.NewGame(matchID, m.config, p0, p1, m.powerUps)
	g.RejoinTokens[0] = t0
	g.RejoinTokens[1] = t1
	g.PlayerUserIDs[0] = client1.UserID
	g.PlayerUserIDs[1] = client2.UserID
	if m.historyStore != nil {
		store := m.historyStore
		g.TelemetrySink = m.queuedSink
		g.OnGameEnd = func(matchID, p0UID, p1UID, p0Name, p1Name string, p0Score, p1Score int, winnerIdx int, endReason string) {
			var e0Before, e0After, e1Before, e1After *int
			if endReason == "completed" {
				eb0, ea0, eb1, ea1, err := store.UpdateRatingsAfterGame(context.Background(), p0UID, p1UID, p0Name, p1Name, winnerIdx)
				if err == nil {
					e0Before, e0After = &eb0, &ea0
					e1Before, e1After = &eb1, &ea1
				}
			}
			_ = store.InsertGameResult(context.Background(), matchID, p0UID, p1UID, p0Name, p1Name, p0Score, p1Score, winnerIdx, endReason, e0Before, e0After, e1Before, e1After)
			m.queuedSink.FlushMatch(matchID)
			var powerUpIDs []string
			for i := 0; i < 6; i++ {
				if id, ok := g.PairIDToPowerUp[i]; ok {
					powerUpIDs = append(powerUpIDs, id)
				}
			}
			_ = store.InsertMatchArcana(context.Background(), matchID, powerUpIDs)
		}
	}

	m.mu.Lock()
	m.activeGames[matchID] = g
	m.userIDToGame[client1.UserID] = matchID
	m.userIDToGame[client2.UserID] = matchID
	m.mu.Unlock()

	client1.Game = g
	client1.PlayerID = 0
	client2.Game = g
	client2.PlayerID = 1

	log.Printf("Match created: %s — %s vs %s", matchID, client1.Name, client2.Name)

	m.sendMatchFound(client1, client2.Name, g, 0)
	m.sendMatchFound(client2, client1.Name, g, 1)

	go func() {
		g.Run()
		m.removeGame(matchID)
	}()
}

func (m *Matchmaker) createGameVsAI(client1 *ws.Client) {
	matchID := uuid.New().String()

	t0, _ := generateRejoinToken()
	t1, _ := generateRejoinToken()

	profiles := m.config.AIProfiles
	if len(profiles) == 0 {
		profiles = config.Defaults().AIProfiles
	}
	profile := &profiles[rand.Intn(len(profiles))]

	aiSend := make(chan []byte, 256)
	p0 := game.NewPlayer(client1.Name, client1.Send)
	p1 := game.NewPlayer(profile.Name, aiSend)

	g := game.NewGame(matchID, m.config, p0, p1, m.powerUps)
	g.RejoinTokens[0] = t0
	g.RejoinTokens[1] = t1
	g.PlayerUserIDs[0] = client1.UserID
	g.PlayerUserIDs[1] = "ai:" + profile.Name // fixed ID per bot for ELO and leaderboard
	if m.historyStore != nil {
		store := m.historyStore
		g.TelemetrySink = m.queuedSink
		g.OnGameEnd = func(matchID, p0UID, p1UID, p0Name, p1Name string, p0Score, p1Score int, winnerIdx int, endReason string) {
			var e0Before, e0After, e1Before, e1After *int
			if endReason == "completed" {
				eb0, ea0, eb1, ea1, err := store.UpdateRatingsAfterGame(context.Background(), p0UID, p1UID, p0Name, p1Name, winnerIdx)
				if err == nil {
					e0Before, e0After = &eb0, &ea0
					e1Before, e1After = &eb1, &ea1
				}
			}
			_ = store.InsertGameResult(context.Background(), matchID, p0UID, p1UID, p0Name, p1Name, p0Score, p1Score, winnerIdx, endReason, e0Before, e0After, e1Before, e1After)
			m.queuedSink.FlushMatch(matchID)
			var powerUpIDs []string
			for i := 0; i < 6; i++ {
				if id, ok := g.PairIDToPowerUp[i]; ok {
					powerUpIDs = append(powerUpIDs, id)
				}
			}
			_ = store.InsertMatchArcana(context.Background(), matchID, powerUpIDs)
		}
	}

	m.mu.Lock()
	m.activeGames[matchID] = g
	m.userIDToGame[client1.UserID] = matchID
	m.mu.Unlock()

	client1.Game = g
	client1.PlayerID = 0

	log.Printf("Match created: %s — %s vs %s (AI)", matchID, client1.Name, profile.Name)

	m.sendMatchFound(client1, profile.Name, g, 0)

	go func() {
		g.Run()
		m.removeGame(matchID)
	}()
	go ai.Run(aiSend, g, 1, profile)
}

func (m *Matchmaker) sendMatchFound(client *ws.Client, opponentName string, g *game.Game, playerIdx int) {
	yourTurn := playerIdx == g.CurrentTurn
	token := ""
	if playerIdx >= 0 && playerIdx <= 1 {
		token = g.RejoinTokens[playerIdx]
	}
	msg := ws.MatchFoundMsg{
		Type:           "match_found",
		GameID:         g.ID,
		RejoinToken:    token,
		OpponentName:   opponentName,
		OpponentUserID: g.PlayerUserIDs[1-playerIdx],
		BoardRows:      m.config.BoardRows,
		BoardCols:      m.config.BoardCols,
		YourTurn:       yourTurn,
	}
	data, _ := json.Marshal(msg)
	wsutil.SafeSend(client.Send, data)
}

func (m *Matchmaker) removeGame(gameID string) {
	m.mu.Lock()
	g := m.activeGames[gameID]
	delete(m.activeGames, gameID)
	if g != nil {
		for i := 0; i < 2; i++ {
			if g.PlayerUserIDs[i] != "" {
				delete(m.userIDToGame, g.PlayerUserIDs[i])
			}
		}
	}
	m.mu.Unlock()
}

// Rejoin looks up a game by ID and rejoin token, and returns the game and player index if the token
// matches the disconnected player. Caller must then attach the client and send ActionRejoinCompleted.
func (m *Matchmaker) Rejoin(gameID, rejoinToken, name string) (*game.Game, int, error) {
	m.mu.RLock()
	g, ok := m.activeGames[gameID]
	m.mu.RUnlock()
	if !ok || g == nil {
		return nil, -1, matcherrors.ErrGameNotFound
	}
	if g.Finished {
		return nil, -1, matcherrors.ErrGameFinished
	}
	playerIdx := -1
	for i := 0; i < 2; i++ {
		if g.RejoinTokens[i] == rejoinToken {
			playerIdx = i
			break
		}
	}
	if playerIdx < 0 {
		return nil, -1, matcherrors.ErrInvalidToken
	}
	if g.DisconnectedPlayerIdx != playerIdx {
		return nil, -1, matcherrors.ErrNotDisconnected
	}
	if len(name) < 1 || len(name) > m.config.MaxNameLength {
		return nil, -1, errors.New("invalid name length")
	}
	return g, playerIdx, nil
}

// RejoinByUser looks up the active game for the given user ID (for cross-device rejoin).
// Only the disconnected player can rejoin. Returns the game, player index, and rejoin token for that player.
func (m *Matchmaker) RejoinByUser(userID string) (*game.Game, int, string, error) {
	m.mu.RLock()
	gameID, ok := m.userIDToGame[userID]
	m.mu.RUnlock()
	if !ok || gameID == "" {
		return nil, -1, "", matcherrors.ErrNoActiveGame
	}
	m.mu.RLock()
	g := m.activeGames[gameID]
	m.mu.RUnlock()
	if g == nil {
		return nil, -1, "", matcherrors.ErrGameNotFound
	}
	if g.Finished {
		return nil, -1, "", matcherrors.ErrGameFinished
	}
	playerIdx := -1
	for i := 0; i < 2; i++ {
		if g.PlayerUserIDs[i] == userID {
			playerIdx = i
			break
		}
	}
	if playerIdx < 0 {
		return nil, -1, "", matcherrors.ErrNoActiveGame
	}
	if g.DisconnectedPlayerIdx != playerIdx {
		return nil, -1, "", matcherrors.ErrNotDisconnected
	}
	token := g.RejoinTokens[playerIdx]
	return g, playerIdx, token, nil
}


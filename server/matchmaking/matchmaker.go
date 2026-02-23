package matchmaking

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"memory-game-server/ai"
	"memory-game-server/config"
	"memory-game-server/game"
	"memory-game-server/storage"
	"memory-game-server/ws"
)

// gameCounter provides unique game IDs.
var gameCounter uint64

// Matchmaker manages the queue of players waiting for a match.
type Matchmaker struct {
	waiting      map[*ws.Client]chan struct{} // client -> cancel channel (closed when client leaves queue)
	waitMu       sync.Mutex
	notify       chan struct{} // buffered; signaled when a client is enqueued
	config       *config.Config
	powerUps     game.PowerUpProvider
	historyStore *storage.Store
	activeGames  map[string]*game.Game
	userIDToGame map[string]string // userID -> gameID for rejoin by user (cross-device)
	mu           sync.RWMutex
}

// NewMatchmaker creates a new Matchmaker. historyStore may be nil to disable game history persistence.
func NewMatchmaker(cfg *config.Config, pups game.PowerUpProvider, historyStore *storage.Store) *Matchmaker {
	return &Matchmaker{
		waiting:      make(map[*ws.Client]chan struct{}),
		notify:       make(chan struct{}, 1),
		config:       cfg,
		powerUps:     pups,
		historyStore: historyStore,
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
	select {
	case m.notify <- struct{}{}:
	default:
	}
}

// LeaveQueue removes a client from the matchmaking queue. Idempotent.
func (m *Matchmaker) LeaveQueue(c *ws.Client) {
	m.waitMu.Lock()
	defer m.waitMu.Unlock()
	ch, ok := m.waiting[c]
	if !ok {
		return
	}
	delete(m.waiting, c)
	close(ch)
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

		select {
		case <-m.notify:
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
			m.createGameVsAI(client1)
		case <-cancelCh1:
			// client1 left queue, continue
		}
	}
}

func (m *Matchmaker) createGame(client1, client2 *ws.Client) {
	gameID := fmt.Sprintf("game-%d", atomic.AddUint64(&gameCounter, 1))

	t0, _ := generateRejoinToken()
	t1, _ := generateRejoinToken()

	p0 := game.NewPlayer(client1.Name, client1.Send)
	p1 := game.NewPlayer(client2.Name, client2.Send)

	g := game.NewGame(gameID, m.config, p0, p1, m.powerUps)
	g.RejoinTokens[0] = t0
	g.RejoinTokens[1] = t1
	g.PlayerUserIDs[0] = client1.UserID
	g.PlayerUserIDs[1] = client2.UserID
	if m.historyStore != nil {
		store := m.historyStore
		g.OnGameEnd = func(gameID, p0UID, p1UID, p0Name, p1Name string, p0Score, p1Score int, winnerIdx int, endReason string) {
			var e0Before, e0After, e1Before, e1After *int
			if endReason == "completed" {
				eb0, ea0, eb1, ea1, err := store.UpdateRatingsAfterGame(context.Background(), p0UID, p1UID, p0Name, p1Name, winnerIdx)
				if err == nil {
					e0Before, e0After = &eb0, &ea0
					e1Before, e1After = &eb1, &ea1
				}
			}
			_ = store.InsertGameResult(context.Background(), gameID, p0UID, p1UID, p0Name, p1Name, p0Score, p1Score, winnerIdx, endReason, e0Before, e0After, e1Before, e1After)
		}
	}

	m.mu.Lock()
	m.activeGames[gameID] = g
	m.userIDToGame[client1.UserID] = gameID
	m.userIDToGame[client2.UserID] = gameID
	m.mu.Unlock()

	client1.Game = g
	client1.PlayerID = 0
	client2.Game = g
	client2.PlayerID = 1

	log.Printf("Match created: %s — %s vs %s", gameID, client1.Name, client2.Name)

	m.sendMatchFound(client1, client2.Name, g, 0)
	m.sendMatchFound(client2, client1.Name, g, 1)

	go func() {
		g.Run()
		m.removeGame(gameID)
	}()
}

func (m *Matchmaker) createGameVsAI(client1 *ws.Client) {
	gameID := fmt.Sprintf("game-%d", atomic.AddUint64(&gameCounter, 1))

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

	g := game.NewGame(gameID, m.config, p0, p1, m.powerUps)
	g.RejoinTokens[0] = t0
	g.RejoinTokens[1] = t1
	g.PlayerUserIDs[0] = client1.UserID
	g.PlayerUserIDs[1] = "ai:" + profile.Name // fixed ID per bot for ELO and leaderboard
	if m.historyStore != nil {
		store := m.historyStore
		g.OnGameEnd = func(gameID, p0UID, p1UID, p0Name, p1Name string, p0Score, p1Score int, winnerIdx int, endReason string) {
			var e0Before, e0After, e1Before, e1After *int
			if endReason == "completed" {
				eb0, ea0, eb1, ea1, err := store.UpdateRatingsAfterGame(context.Background(), p0UID, p1UID, p0Name, p1Name, winnerIdx)
				if err == nil {
					e0Before, e0After = &eb0, &ea0
					e1Before, e1After = &eb1, &ea1
				}
			}
			_ = store.InsertGameResult(context.Background(), gameID, p0UID, p1UID, p0Name, p1Name, p0Score, p1Score, winnerIdx, endReason, e0Before, e0After, e1Before, e1After)
		}
	}

	m.mu.Lock()
	m.activeGames[gameID] = g
	m.userIDToGame[client1.UserID] = gameID
	m.mu.Unlock()

	client1.Game = g
	client1.PlayerID = 0

	log.Printf("Match created: %s — %s vs %s (AI)", gameID, client1.Name, profile.Name)

	m.sendMatchFound(client1, profile.Name, g, 0)

	go func() {
		g.Run()
		m.removeGame(gameID)
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
		Type:         "match_found",
		GameID:       g.ID,
		RejoinToken:  token,
		OpponentName: opponentName,
		BoardRows:    m.config.BoardRows,
		BoardCols:    m.config.BoardCols,
		YourTurn:     yourTurn,
	}
	data, _ := json.Marshal(msg)
	safeSend(client.Send, data)
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

var (
	ErrGameNotFound    = errors.New("game not found")
	ErrGameFinished    = errors.New("game finished")
	ErrInvalidToken    = errors.New("invalid rejoin token")
	ErrNotDisconnected = errors.New("this player is not disconnected")
	ErrNoActiveGame    = errors.New("no active game for this user")
)

// Rejoin looks up a game by ID and rejoin token, and returns the game and player index if the token
// matches the disconnected player. Caller must then attach the client and send ActionRejoinCompleted.
func (m *Matchmaker) Rejoin(gameID, rejoinToken, name string) (*game.Game, int, error) {
	m.mu.RLock()
	g, ok := m.activeGames[gameID]
	m.mu.RUnlock()
	if !ok || g == nil {
		return nil, -1, ErrGameNotFound
	}
	if g.Finished {
		return nil, -1, ErrGameFinished
	}
	playerIdx := -1
	for i := 0; i < 2; i++ {
		if g.RejoinTokens[i] == rejoinToken {
			playerIdx = i
			break
		}
	}
	if playerIdx < 0 {
		return nil, -1, ErrInvalidToken
	}
	if g.DisconnectedPlayerIdx != playerIdx {
		return nil, -1, ErrNotDisconnected
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
		return nil, -1, "", ErrNoActiveGame
	}
	m.mu.RLock()
	g := m.activeGames[gameID]
	m.mu.RUnlock()
	if g == nil {
		return nil, -1, "", ErrGameNotFound
	}
	if g.Finished {
		return nil, -1, "", ErrGameFinished
	}
	playerIdx := -1
	for i := 0; i < 2; i++ {
		if g.PlayerUserIDs[i] == userID {
			playerIdx = i
			break
		}
	}
	if playerIdx < 0 {
		return nil, -1, "", ErrNoActiveGame
	}
	if g.DisconnectedPlayerIdx != playerIdx {
		return nil, -1, "", ErrNotDisconnected
	}
	token := g.RejoinTokens[playerIdx]
	return g, playerIdx, token, nil
}

// safeSend sends data to a channel without panicking if the channel is closed.
func safeSend(ch chan []byte, data []byte) {
	defer func() {
		recover()
	}()
	select {
	case ch <- data:
	default:
	}
}

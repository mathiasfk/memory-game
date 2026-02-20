package matchmaking

import (
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
	"memory-game-server/ws"
)

// gameCounter provides unique game IDs.
var gameCounter uint64

// Matchmaker manages the queue of players waiting for a match.
type Matchmaker struct {
	queue        chan *ws.Client
	config       *config.Config
	powerUps     game.PowerUpProvider
	activeGames  map[string]*game.Game
	userIDToGame map[string]string // userID -> gameID for rejoin by user (cross-device)
	mu           sync.RWMutex
}

// NewMatchmaker creates a new Matchmaker.
func NewMatchmaker(cfg *config.Config, pups game.PowerUpProvider) *Matchmaker {
	return &Matchmaker{
		queue:        make(chan *ws.Client, 100),
		config:       cfg,
		powerUps:     pups,
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
	m.queue <- c
}

// Run is the matchmaker's main loop. It waits for a first player, then either
// a second player within AIPairTimeoutSec or starts a game vs the AI.
// Should be run as a goroutine.
func (m *Matchmaker) Run() {
	for {
		client1 := <-m.queue

		timeout := time.Duration(m.config.AIPairTimeoutSec) * time.Second
		if timeout < 0 {
			timeout = 0
		}

		var client2 *ws.Client
		select {
		case client2 = <-m.queue:
			// Two human players — create normal game
			m.createGame(client1, client2)
		case <-time.After(timeout):
			// Timeout — create game vs AI
			m.createGameVsAI(client1)
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
	// PlayerUserIDs[1] left empty for AI

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

package matchmaking

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
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
	queue    chan *ws.Client
	config   *config.Config
	powerUps game.PowerUpProvider
}

// NewMatchmaker creates a new Matchmaker.
func NewMatchmaker(cfg *config.Config, pups game.PowerUpProvider) *Matchmaker {
	return &Matchmaker{
		queue:    make(chan *ws.Client, 100),
		config:   cfg,
		powerUps: pups,
	}
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

	p0 := game.NewPlayer(client1.Name, client1.Send)
	p1 := game.NewPlayer(client2.Name, client2.Send)

	g := game.NewGame(gameID, m.config, p0, p1, m.powerUps)

	client1.Game = g
	client1.PlayerID = 0
	client2.Game = g
	client2.PlayerID = 1

	log.Printf("Match created: %s — %s vs %s", gameID, client1.Name, client2.Name)

	m.sendMatchFound(client1, client2.Name, g)
	m.sendMatchFound(client2, client1.Name, g)

	go g.Run()
}

func (m *Matchmaker) createGameVsAI(client1 *ws.Client) {
	gameID := fmt.Sprintf("game-%d", atomic.AddUint64(&gameCounter, 1))

	profiles := m.config.AIProfiles
	if len(profiles) == 0 {
		profiles = config.Defaults().AIProfiles
	}
	profile := &profiles[rand.Intn(len(profiles))]

	aiSend := make(chan []byte, 256)
	p0 := game.NewPlayer(client1.Name, client1.Send)
	p1 := game.NewPlayer(profile.Name, aiSend)

	g := game.NewGame(gameID, m.config, p0, p1, m.powerUps)

	client1.Game = g
	client1.PlayerID = 0

	log.Printf("Match created: %s — %s vs %s (AI)", gameID, client1.Name, profile.Name)

	m.sendMatchFound(client1, profile.Name, g)

	go g.Run()
	go ai.Run(aiSend, g, 1, profile)
}

func (m *Matchmaker) sendMatchFound(client *ws.Client, opponentName string, g *game.Game) {
	yourTurn := client.PlayerID == g.CurrentTurn
	msg := ws.MatchFoundMsg{
		Type:         "match_found",
		OpponentName: opponentName,
		BoardRows:    m.config.BoardRows,
		BoardCols:    m.config.BoardCols,
		YourTurn:     yourTurn,
	}
	data, _ := json.Marshal(msg)
	safeSend(client.Send, data)
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

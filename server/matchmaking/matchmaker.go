package matchmaking

import (
	"encoding/json"
	"fmt"
	"log"
	"sync/atomic"

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

// Run is the matchmaker's main loop. It blocks reading pairs of
// clients from the queue and creates games for them.
// Should be run as a goroutine.
func (m *Matchmaker) Run() {
	for {
		// Wait for the first player
		client1 := <-m.queue
		// Wait for the second player
		client2 := <-m.queue

		// Create a new game
		gameID := fmt.Sprintf("game-%d", atomic.AddUint64(&gameCounter, 1))

		p0 := game.NewPlayer(client1.Name, client1.Send)
		p1 := game.NewPlayer(client2.Name, client2.Send)

		g := game.NewGame(gameID, m.config, p0, p1, m.powerUps)

		// Assign game references to clients
		client1.Game = g
		client1.PlayerID = 0
		client2.Game = g
		client2.PlayerID = 1

		log.Printf("Match created: %s — %s vs %s", gameID, client1.Name, client2.Name)

		// Send MatchFound to both players
		m.sendMatchFound(client1, client2.Name, g)
		m.sendMatchFound(client2, client1.Name, g)

		// Start the game goroutine (it broadcasts initial state automatically)
		go g.Run()
	}
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

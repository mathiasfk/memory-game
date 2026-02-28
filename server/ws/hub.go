package ws

import (
	"context"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"memory-game-server/config"
	"memory-game-server/game"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for development; restrict in production.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// MatchmakerInterface defines what the Hub needs from the Matchmaker.
type MatchmakerInterface interface {
	Enqueue(c *Client)
	LeaveQueue(c *Client)
	Rejoin(gameID, rejoinToken, name string) (*game.Game, int, error)
	RejoinByUser(userID string) (*game.Game, int, string, error)
}

// Hub maintains the set of active clients and routes messages.
type Hub struct {
	Clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Matchmaker MatchmakerInterface
	Config     *config.Config
}

// NewHub creates a new Hub.
func NewHub(cfg *config.Config, mm MatchmakerInterface) *Hub {
	return &Hub{
		Clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Matchmaker: mm,
		Config:     cfg,
	}
}

// Run starts the hub's main loop. Should be run as a goroutine.
// When ctx is cancelled (e.g. on server shutdown), Run returns and no longer accepts new registrations.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Print("Hub: shutdown signal received, stopping")
			return
		case client := <-h.Register:
			h.Clients[client] = true
			log.Printf("Client connected. Total clients: %d", len(h.Clients))

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				log.Printf("Client disconnected. Total clients: %d", len(h.Clients))

				// If the client was in a game, start reconnection window (do not end game immediately)
				if client.Game != nil && !client.Game.Finished {
					select {
					case client.Game.Actions <- game.Action{
						Type:      game.ActionPlayerDisconnected,
						PlayerIdx: client.PlayerID,
					}:
					default:
					}
				}
			}
		}
	}
}

// ServeWS handles WebSocket upgrade requests and creates a new Client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		Hub:  h,
		Conn: conn,
		Send: make(chan []byte, 256),
	}

	h.Register <- client

	go client.WritePump()
	go client.ReadPump()
}

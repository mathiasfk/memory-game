package ws

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
	SignalHumanReady(gameID string)
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

// uniqueAuthenticatedUsers returns the number of distinct authenticated users (by UserID).
// Multiple connections from the same user (e.g. React Strict Mode) count as one.
func (h *Hub) uniqueAuthenticatedUsers() int {
	seen := make(map[string]bool)
	for c := range h.Clients {
		if c.Authenticated && c.UserID != "" {
			seen[c.UserID] = true
		}
	}
	return len(seen)
}

// Run starts the hub's main loop. Should be run as a goroutine.
// When ctx is cancelled (e.g. on server shutdown), Run returns and no longer accepts new registrations.
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown signal received, stopping", "tag", "hub")
			return
		case client := <-h.Register:
			h.Clients[client] = true
			slog.Info("Client connected", "tag", "hub", "total_connections", len(h.Clients), "total_users", h.uniqueAuthenticatedUsers())

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				slog.Info("Client disconnected", "tag", "hub", "total_connections", len(h.Clients), "total_users", h.uniqueAuthenticatedUsers())

				// Notify game so it can clear player.Send before we close the channel.
				// Use a goroutine with blocking send so we never drop this action; if we dropped it,
				// the game would never clear the Send reference and every broadcast would panic on closed channel.
				if client.Game != nil && !client.Game.Finished {
					act := game.Action{
						Type:      game.ActionPlayerDisconnected,
						PlayerIdx: client.PlayerID,
					}
					go func() {
						client.Game.Actions <- act
					}()
				}

				// Close Send after a short delay so the game loop can process the action and clear its reference.
				go func(c *Client) {
					time.Sleep(200 * time.Millisecond)
					close(c.Send)
				}(client)
			}
		}
	}
}

// ServeWS handles WebSocket upgrade requests and creates a new Client.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("WebSocket upgrade error", "tag", "hub", "err", err)
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

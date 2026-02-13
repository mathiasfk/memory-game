package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
	"memory-game-server/game"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	Name     string
	Game     *game.Game
	PlayerID int // 0 or 1 within the game
}

// ReadPump pumps messages from the websocket connection to the hub.
// It runs in its own goroutine per connection.
func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		c.handleMessage(message)
	}
}

// WritePump pumps messages from the send channel to the websocket connection.
// It runs in its own goroutine per connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(data []byte) {
	var envelope InboundEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		c.sendError("Invalid message format.")
		return
	}

	switch envelope.Type {
	case "set_name":
		c.handleSetName(envelope.Raw)
	case "flip_card":
		c.handleFlipCard(envelope.Raw)
	case "use_power_up":
		c.handleUsePowerUp(envelope.Raw)
	case "play_again":
		c.handlePlayAgain()
	default:
		c.sendError("Unknown message type: " + envelope.Type)
	}
}

func (c *Client) handleSetName(raw json.RawMessage) {
	var msg SetNameMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("Invalid set_name message.")
		return
	}

	// Validate name length
	if len(msg.Name) < 1 || len(msg.Name) > c.Hub.Config.MaxNameLength {
		c.sendError("Name must be between 1 and " + intToStr(c.Hub.Config.MaxNameLength) + " characters.")
		return
	}

	// Cannot set name if already in a game
	if c.Game != nil {
		c.sendError("Cannot change name while in a game.")
		return
	}

	c.Name = msg.Name

	// Enter matchmaking queue
	c.Hub.Matchmaker.Enqueue(c)

	// Send WaitingForMatch
	waitMsg := WaitingForMatchMsg{Type: "waiting_for_match"}
	data, _ := json.Marshal(waitMsg)
	select {
	case c.Send <- data:
	default:
	}
}

func (c *Client) handleFlipCard(raw json.RawMessage) {
	if c.Game == nil {
		c.sendError("You are not in a game.")
		return
	}

	var msg FlipCardMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("Invalid flip_card message.")
		return
	}

	c.Game.Actions <- game.Action{
		Type:      game.ActionFlipCard,
		PlayerIdx: c.PlayerID,
		Index:     msg.Index,
	}
}

func (c *Client) handleUsePowerUp(raw json.RawMessage) {
	if c.Game == nil {
		c.sendError("You are not in a game.")
		return
	}

	var msg UsePowerUpMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("Invalid use_power_up message.")
		return
	}

	c.Game.Actions <- game.Action{
		Type:      game.ActionUsePowerUp,
		PlayerIdx: c.PlayerID,
		PowerUpID: msg.PowerUpID,
	}
}

func (c *Client) handlePlayAgain() {
	if c.Game != nil && !c.Game.Finished {
		c.sendError("Cannot play again while in an active game.")
		return
	}

	// Reset game reference
	c.Game = nil
	c.PlayerID = 0

	// Re-enter matchmaking queue
	c.Hub.Matchmaker.Enqueue(c)

	// Send WaitingForMatch
	waitMsg := WaitingForMatchMsg{Type: "waiting_for_match"}
	data, _ := json.Marshal(waitMsg)
	select {
	case c.Send <- data:
	default:
	}
}

func (c *Client) sendError(message string) {
	msg := ErrorMsg{Type: "error", Message: message}
	data, _ := json.Marshal(msg)
	select {
	case c.Send <- data:
	default:
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

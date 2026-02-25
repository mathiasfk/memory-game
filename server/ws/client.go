package ws

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"memory-game-server/auth"
	"memory-game-server/game"
	"memory-game-server/matcherrors"
	"memory-game-server/wsutil"
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
	Hub           *Hub
	Conn          *websocket.Conn
	Send          chan []byte
	Name          string
	Game          *game.Game
	PlayerID      int    // 0 or 1 within the game
	UserID        string // from JWT sub claim
	Authenticated bool
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

	// When auth is not configured, allow set_name without auth (tests, local dev).
	allowedWithoutAuth := envelope.Type == "auth" || (envelope.Type == "set_name" && c.Hub.Config.NeonAuthBaseURL == "")
	if !c.Authenticated && !allowedWithoutAuth {
		c.sendError("Authentication required. Send an auth message first.")
		return
	}

	switch envelope.Type {
	case "auth":
		c.handleAuth(envelope.Raw)
	case "set_name":
		c.handleSetName(envelope.Raw)
	case "rejoin":
		c.handleRejoin(envelope.Raw)
	case "rejoin_my_game":
		c.handleRejoinMyGame()
	case "flip_card":
		c.handleFlipCard(envelope.Raw)
	case "use_power_up":
		c.handleUsePowerUp(envelope.Raw)
	case "play_again":
		c.handlePlayAgain()
	case "leave_game":
		c.handleLeaveGame()
	case "leave_queue":
		c.handleLeaveQueue()
	default:
		c.sendError("Unknown message type: " + envelope.Type)
	}
}

func (c *Client) handleAuth(raw json.RawMessage) {
	if c.Authenticated {
		log.Printf("[auth] client already authenticated, rejecting")
		c.sendError("Already authenticated.")
		return
	}
	var msg AuthMsg
	if err := json.Unmarshal(raw, &msg); err != nil || msg.Token == "" {
		log.Printf("[auth] invalid auth message: unmarshal err=%v, token empty=%v", err != nil, msg.Token == "")
		c.sendError("Invalid auth message.")
		return
	}
	baseURL := c.Hub.Config.NeonAuthBaseURL
	if baseURL == "" {
		log.Printf("[auth] NEON_AUTH_BASE_URL not set on server; cannot validate token")
		c.sendError("Server auth not configured.")
		return
	}
	claims, err := auth.ValidateNeonToken(baseURL, msg.Token)
	if err != nil {
		log.Printf("[auth] token validation failed: %v", err)
		c.sendError("Invalid or expired token.")
		return
	}
	c.UserID = auth.UserIDFromClaims(claims)
	c.Name = auth.FirstNameFromClaims(claims)
	c.Authenticated = true
	log.Printf("[auth] authenticated user id=%s name=%s", c.UserID, c.Name)
}

func (c *Client) handleSetName(raw json.RawMessage) {
	var msg SetNameMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("Invalid set_name message.")
		return
	}

	// When auth is configured, name comes from JWT (set in handleAuth). Otherwise use client-sent name (tests/local dev).
	if c.Hub.Config.NeonAuthBaseURL == "" {
		c.Name = strings.TrimSpace(msg.Name)
		c.Authenticated = true // allow subsequent messages (flip_card, etc.) without JWT
	}

	// Validate name length
	if len(c.Name) < 1 || len(c.Name) > c.Hub.Config.MaxNameLength {
		c.sendError("Name must be between 1 and " + strconv.Itoa(c.Hub.Config.MaxNameLength) + " characters.")
		return
	}

	// Cannot set name if already in a game
	if c.Game != nil {
		c.sendError("Cannot change name while in a game.")
		return
	}

	// Enter matchmaking queue (c.Name already set from JWT)
	c.Hub.Matchmaker.Enqueue(c)

	// Send WaitingForMatch
	waitMsg := WaitingForMatchMsg{Type: "waiting_for_match"}
	data, _ := json.Marshal(waitMsg)
	wsutil.SafeSend(c.Send, data)
}

func (c *Client) handleRejoin(raw json.RawMessage) {
	if c.Game != nil {
		c.sendError("Already in a game.")
		return
	}
	var msg RejoinMsg
	if err := json.Unmarshal(raw, &msg); err != nil {
		c.sendError("Invalid rejoin message.")
		return
	}
	if msg.GameID == "" || msg.RejoinToken == "" || msg.Name == "" {
		c.sendError("Missing gameId, rejoinToken, or name.")
		return
	}
	g, playerIdx, err := c.Hub.Matchmaker.Rejoin(msg.GameID, msg.RejoinToken, msg.Name)
	if err != nil {
		switch {
		case errors.Is(err, matcherrors.ErrGameNotFound), errors.Is(err, matcherrors.ErrGameFinished):
			c.sendError("Game not found or already ended.")
		case errors.Is(err, matcherrors.ErrInvalidToken):
			c.sendError("Invalid rejoin token.")
		case errors.Is(err, matcherrors.ErrNotDisconnected):
			c.sendError("Cannot rejoin: you are already connected.")
		default:
			c.sendError(err.Error())
		}
		return
	}
	c.Game = g
	c.PlayerID = playerIdx
	c.Name = msg.Name

	// Tell the game loop to update the player's Send channel and clear reconnection state
	select {
	case g.Actions <- game.Action{
		Type:      game.ActionRejoinCompleted,
		PlayerIdx: playerIdx,
		NewSend:   c.Send,
	}:
	default:
		c.sendError("Game is busy. Try again.")
		c.Game = nil
		c.PlayerID = 0
		return
	}

	// Send match_found so the client can show the game screen; game_state will follow from broadcastState()
	opponentIdx := 1 - playerIdx
	opponentName := ""
	if g.Players[opponentIdx] != nil {
		opponentName = g.Players[opponentIdx].Name
	}
	matchMsg := MatchFoundMsg{
		Type:           "match_found",
		GameID:         g.ID,
		RejoinToken:    msg.RejoinToken,
		OpponentName:   opponentName,
		OpponentUserID: g.PlayerUserIDs[opponentIdx],
		BoardRows:      c.Hub.Config.BoardRows,
		BoardCols:      c.Hub.Config.BoardCols,
		YourTurn:       playerIdx == g.CurrentTurn,
	}
	matchData, _ := json.Marshal(matchMsg)
	wsutil.SafeSend(c.Send, matchData)

	// Send current game state so client has it immediately (game loop will also broadcastState)
	state := g.BuildStateForPlayer(playerIdx)
	stateData, _ := json.Marshal(state)
	wsutil.SafeSend(c.Send, stateData)
}

func (c *Client) handleRejoinMyGame() {
	if c.Game != nil {
		c.sendError("Already in a game.")
		return
	}
	g, playerIdx, rejoinToken, err := c.Hub.Matchmaker.RejoinByUser(c.UserID)
	if err != nil {
		switch {
		case errors.Is(err, matcherrors.ErrGameNotFound), errors.Is(err, matcherrors.ErrGameFinished):
			c.sendError("Game not found or already ended.")
		case errors.Is(err, matcherrors.ErrNoActiveGame):
			c.sendError("No active game for this user.")
		case errors.Is(err, matcherrors.ErrNotDisconnected):
			c.sendError("Cannot rejoin: you are already connected.")
		default:
			c.sendError(err.Error())
		}
		return
	}
	c.Game = g
	c.PlayerID = playerIdx
	// c.Name already set from JWT at auth time

	select {
	case g.Actions <- game.Action{
		Type:      game.ActionRejoinCompleted,
		PlayerIdx: playerIdx,
		NewSend:   c.Send,
	}:
	default:
		c.sendError("Game is busy. Try again.")
		c.Game = nil
		c.PlayerID = 0
		return
	}

	opponentIdx := 1 - playerIdx
	opponentName := ""
	if g.Players[opponentIdx] != nil {
		opponentName = g.Players[opponentIdx].Name
	}
	matchMsg := MatchFoundMsg{
		Type:           "match_found",
		GameID:         g.ID,
		RejoinToken:    rejoinToken,
		OpponentName:   opponentName,
		OpponentUserID: g.PlayerUserIDs[opponentIdx],
		BoardRows:      c.Hub.Config.BoardRows,
		BoardCols:      c.Hub.Config.BoardCols,
		YourTurn:       playerIdx == g.CurrentTurn,
	}
	matchData, _ := json.Marshal(matchMsg)
	wsutil.SafeSend(c.Send, matchData)

	state := g.BuildStateForPlayer(playerIdx)
	stateData, _ := json.Marshal(state)
	wsutil.SafeSend(c.Send, stateData)
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

	cardIndex := -1
	if msg.CardIndex >= 0 {
		cardIndex = msg.CardIndex
	}
	c.Game.Actions <- game.Action{
		Type:      game.ActionUsePowerUp,
		PlayerIdx: c.PlayerID,
		PowerUpID: msg.PowerUpID,
		CardIndex: cardIndex,
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
	wsutil.SafeSend(c.Send, data)
}

func (c *Client) handleLeaveQueue() {
	if c.Game != nil {
		c.sendError("Cannot leave queue while in a game.")
		return
	}
	c.Hub.Matchmaker.LeaveQueue(c)
}

func (c *Client) handleLeaveGame() {
	if c.Game == nil {
		c.sendError("You are not in a game.")
		return
	}
	if c.Game.Finished {
		c.sendError("Game already ended.")
		return
	}

	g := c.Game
	playerIdx := c.PlayerID
	c.Game = nil
	c.PlayerID = 0

	select {
	case g.Actions <- game.Action{
		Type:      game.ActionDisconnect,
		PlayerIdx: playerIdx,
	}:
	default:
		c.sendError("Could not leave game. Try again.")
		c.Game = g
		c.PlayerID = playerIdx
	}
}

func (c *Client) sendError(message string) {
	msg := ErrorMsg{Type: "error", Message: message}
	data, _ := json.Marshal(msg)
	wsutil.SafeSend(c.Send, data)
}

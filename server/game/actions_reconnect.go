package game

import (
	"encoding/json"
	"time"

	"memory-game-server/wsutil"
)

func (g *Game) handleDisconnect(playerIdx int) {
	g.Finished = true
	opponentIdx := 1 - playerIdx
	if g.OnGameEnd != nil {
		g.OnGameEnd(g.ID, g.PlayerUserIDs[0], g.PlayerUserIDs[1], g.Players[0].Name, g.Players[1].Name, g.Players[0].Score, g.Players[1].Score, opponentIdx, "opponent_disconnected")
	}
	// Notify the opponent
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]string{"type": "opponent_disconnected"}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
}

func (g *Game) cancelReconnectionTimer() {
	if g.reconnectionTimerCancel != nil {
		close(g.reconnectionTimerCancel)
		g.reconnectionTimerCancel = nil
	}
	g.DisconnectedPlayerIdx = -1
}

func (g *Game) handlePlayerDisconnected(playerIdx int) {
	if g.DisconnectedPlayerIdx >= 0 {
		return
	}
	g.cancelTurnTimer()
	g.DisconnectedPlayerIdx = playerIdx
	timeoutSec := g.Config.ReconnectTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	g.ReconnectionDeadline = time.Now().Add(time.Duration(timeoutSec) * time.Second)
	opponentIdx := 1 - playerIdx
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]interface{}{
			"type":                        "opponent_reconnecting",
			"reconnectionDeadlineUnixMs": g.ReconnectionDeadline.UnixMilli(),
		}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
	g.reconnectionTimerCancel = make(chan struct{})
	cancel := g.reconnectionTimerCancel
	go func() {
		select {
		case <-time.After(time.Duration(timeoutSec) * time.Second):
			select {
			case g.Actions <- Action{Type: ActionReconnectionTimeout}:
			case <-g.Done:
			}
		case <-cancel:
		}
	}()
}

func (g *Game) handleReconnectionTimeout() {
	idx := g.DisconnectedPlayerIdx
	g.cancelReconnectionTimer()
	g.handleDisconnect(idx)
}

func (g *Game) handleRejoinCompleted(playerIdx int, newSend chan []byte) {
	g.cancelReconnectionTimer()
	if playerIdx >= 0 && playerIdx <= 1 && g.Players[playerIdx] != nil && newSend != nil {
		g.Players[playerIdx].Send = newSend
	}
	opponentIdx := 1 - playerIdx
	opponent := g.Players[opponentIdx]
	if opponent != nil && opponent.Send != nil {
		msg := map[string]string{"type": "opponent_reconnected"}
		data, _ := json.Marshal(msg)
		wsutil.SafeSend(opponent.Send, data)
	}
	g.startTurnTimer()
	g.broadcastState()
}

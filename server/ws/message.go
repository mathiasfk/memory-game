package ws

import "encoding/json"

// InboundEnvelope is the generic envelope for all client-to-server messages.
// The Type field is used for routing; Raw holds the full JSON payload.
type InboundEnvelope struct {
	Type string          `json:"type"`
	Raw  json.RawMessage `json:"-"`
}

// UnmarshalJSON implements custom unmarshaling to capture the raw payload.
func (e *InboundEnvelope) UnmarshalJSON(data []byte) error {
	// Unmarshal just the type field
	type typeOnly struct {
		Type string `json:"type"`
	}
	var t typeOnly
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	e.Type = t.Type
	e.Raw = json.RawMessage(data)
	return nil
}

// --- Client-to-Server message payloads ---

// SetNameMsg is sent by the client to declare a display name.
type SetNameMsg struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// FlipCardMsg is sent by the client to flip a card.
type FlipCardMsg struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
}

// UsePowerUpMsg is sent by the client to activate a power-up.
// CardIndex is optional; required for power-ups that target a card (e.g. Radar). Use -1 when not applicable.
type UsePowerUpMsg struct {
	Type      string `json:"type"`
	PowerUpID string `json:"powerUpId"`
	CardIndex int    `json:"cardIndex,omitempty"` // -1 when not used
}

// PlayAgainMsg is sent by the client to re-enter matchmaking.
type PlayAgainMsg struct {
	Type string `json:"type"`
}

// --- Server-to-Client messages ---

// ErrorMsg is sent when a client action is invalid.
type ErrorMsg struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// WaitingForMatchMsg confirms the player is in the matchmaking queue.
type WaitingForMatchMsg struct {
	Type string `json:"type"`
}

// MatchFoundMsg is sent when two players are paired.
type MatchFoundMsg struct {
	Type         string `json:"type"`
	OpponentName string `json:"opponentName"`
	BoardRows    int    `json:"boardRows"`
	BoardCols    int    `json:"boardCols"`
	YourTurn     bool   `json:"yourTurn"`
}

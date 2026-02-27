package game

// CardView is the client-facing representation of a card.
// PairID and Element are only included when the card is revealed or matched; hidden/removed cards never expose element (no leak).
type CardView struct {
	Index   int    `json:"index"`
	PairID  *int   `json:"pairId,omitempty"`
	State   string `json:"state"`
	Element string `json:"element,omitempty"`
}

// PlayerView is the client-facing representation of a player.
type PlayerView struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// PowerUpView is the client-facing representation of an available power-up (legacy; hand used instead).
type PowerUpView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
	CanAfford   bool   `json:"canAfford"`
}

// PowerUpInHand is one slot in the player's power-up hand sent to the client.
type PowerUpInHand struct {
	PowerUpID   string `json:"powerUpId"`
	Count       int    `json:"count"`
	UsableCount int    `json:"usableCount"`
}

// GameStateMsg is the full game state broadcast to a specific player.
type GameStateMsg struct {
	Type                 string          `json:"type"`
	Cards                []CardView      `json:"cards"`
	You                  PlayerView      `json:"you"`
	Opponent             PlayerView      `json:"opponent"`
	YourTurn             bool            `json:"yourTurn"`
	Hand                 []PowerUpInHand `json:"hand"`
	FlippedIndices       []int           `json:"flippedIndices"`
	Phase                string          `json:"phase"`
	TurnEndsAtUnixMs     int64           `json:"turnEndsAtUnixMs,omitempty"`
	TurnCountdownShowSec int             `json:"turnCountdownShowSec,omitempty"`
	// KnownIndices are card indices that have been revealed at some point (used when computing Unveiling highlight).
	KnownIndices []int `json:"knownIndices,omitempty"`
	// PairIDToPowerUp maps board pair IDs (0..3 for arcana pairs) to power-up IDs for this match; client uses for display.
	PairIDToPowerUp map[int]string `json:"pairIdToPowerUp,omitempty"`
	// HighlightIndices are card indices to highlight (Unveiling: never-revealed hidden; Elementals: tiles of chosen element). Current turn only.
	HighlightIndices []int `json:"highlightIndices,omitempty"`
}

// BuildCardViews constructs the client-facing card list.
// Hidden and removed cards do not expose pairId or element; only revealed/matched cards send them (no leak).
func BuildCardViews(board *Board) []CardView {
	views := make([]CardView, len(board.Cards))
	for i, card := range board.Cards {
		cv := CardView{
			Index: card.Index,
			State: card.State.String(),
		}
		if card.State == Revealed || card.State == Matched {
			pairID := card.PairID
			cv.PairID = &pairID
			if card.Element != "" {
				cv.Element = card.Element
			}
		}
		views[i] = cv
	}
	return views
}

// BuildPlayerView creates a PlayerView from a Player.
func BuildPlayerView(p *Player, currentRound int) PlayerView {
	return PlayerView{
		Name:  p.Name,
		Score: p.Score,
	}
}

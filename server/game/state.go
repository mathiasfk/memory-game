package game

// CardView is the client-facing representation of a card.
// PairID is only included when the card is revealed or matched.
type CardView struct {
	Index  int    `json:"index"`
	PairID *int   `json:"pairId,omitempty"`
	State  string `json:"state"`
}

// PlayerView is the client-facing representation of a player.
type PlayerView struct {
	Name        string `json:"name"`
	Score       int    `json:"score"`
	ComboStreak int    `json:"comboStreak"`

	// SecondChanceRoundsRemaining is how many rounds the Second Chance power-up is still active (0 = inactive).
	SecondChanceRoundsRemaining int `json:"secondChanceRoundsRemaining"`
}

// PowerUpView is the client-facing representation of an available power-up.
type PowerUpView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Cost        int    `json:"cost"`
	CanAfford   bool   `json:"canAfford"`
}

// GameStateMsg is the full game state broadcast to a specific player.
type GameStateMsg struct {
	Type             string        `json:"type"`
	Cards            []CardView    `json:"cards"`
	You              PlayerView    `json:"you"`
	Opponent         PlayerView    `json:"opponent"`
	YourTurn         bool          `json:"yourTurn"`
	AvailablePowerUps []PowerUpView `json:"availablePowerUps"`
	FlippedIndices   []int         `json:"flippedIndices"`
	Phase            string        `json:"phase"`
}

// BuildCardViews constructs the client-facing card list.
// Hidden cards do not expose their pairId.
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
		}
		views[i] = cv
	}
	return views
}

// BuildPlayerView creates a PlayerView from a Player. currentRound is used to compute SecondChanceRoundsRemaining.
func BuildPlayerView(p *Player, currentRound int) PlayerView {
	remaining := 0
	if p.SecondChanceActiveUntilRound > 0 && currentRound <= p.SecondChanceActiveUntilRound {
		remaining = p.SecondChanceActiveUntilRound - currentRound
	}
	return PlayerView{
		Name:                        p.Name,
		Score:                       p.Score,
		ComboStreak:                 p.ComboStreak,
		SecondChanceRoundsRemaining: remaining,
	}
}

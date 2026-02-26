package game

// Player represents a player in a game session.
type Player struct {
	Name        string
	Score       int
	ComboStreak int
	Send        chan []byte // reference to the client's send channel

	// Hand is the player's power-up hand: powerUpId -> count. Use is free; cards are gained by matching pairs.
	Hand map[string]int

	// UnveilingHighlightActive is true after the player uses Unveiling; cleared when turn ends or Chaos is used.
	UnveilingHighlightActive bool

	// LeechActive is true after the player uses Leech; points from matching this turn are subtracted from the opponent. Cleared when turn ends.
	LeechActive bool

	// BloodPactActive is true after the player uses Blood Pact; they must match 3 pairs in a row for +5, or lose 3 on first mismatch.
	BloodPactActive bool
	// BloodPactMatchesCount is the number of consecutive matches since activating Blood Pact.
	BloodPactMatchesCount int

	// ElementalHighlightIndices are card indices to highlight when the player used an elemental powerup (this turn only). Cleared when turn ends or Chaos is used.
	ElementalHighlightIndices []int
}

// NewPlayer creates a new Player with the given name and send channel.
func NewPlayer(name string, send chan []byte) *Player {
	return &Player{
		Name:        name,
		Score:       0,
		ComboStreak: 0,
		Send:        send,
		Hand:        make(map[string]int),
	}
}

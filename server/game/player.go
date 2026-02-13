package game

// Player represents a player in a game session.
type Player struct {
	Name        string
	Score       int
	ComboStreak int
	Send        chan []byte // reference to the client's send channel
}

// NewPlayer creates a new Player with the given name and send channel.
func NewPlayer(name string, send chan []byte) *Player {
	return &Player{
		Name:        name,
		Score:       0,
		ComboStreak: 0,
		Send:        send,
	}
}

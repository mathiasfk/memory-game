package powerup

import (
	"memory-game-server/game"
)

// NecromancyPowerUp returns all matched (collected) tiles back to the board as hidden, then shuffles
// their positions among all hidden slots. Tiles that were already hidden stay in place; only the
// pairIDs are redistributed.
type NecromancyPowerUp struct {
	CostValue int
}

func (n *NecromancyPowerUp) ID() string          { return "necromancy" }
func (n *NecromancyPowerUp) Name() string        { return "Necromancy" }
func (n *NecromancyPowerUp) Description() string { return "Returns all collected tiles back to the board in new random positions." }
func (n *NecromancyPowerUp) Cost() int           { return n.CostValue }

func (n *NecromancyPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player) error {
	// Unmatch all matched cards (set to Hidden)
	for i := range board.Cards {
		if board.Cards[i].State == game.Matched {
			board.Cards[i].State = game.Hidden
		}
	}
	// Shuffle pairIDs among all hidden positions
	game.ShuffleUnmatched(board)
	return nil
}

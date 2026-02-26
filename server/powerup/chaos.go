package powerup

import (
	"memory-game-server/game"
)

// ChaosPowerUp reshuffles all unmatched cards on the board.
// When applied, the game layer also clears KnownIndices and UnveilingHighlightActive (see game.handleUsePowerUp).
type ChaosPowerUp struct {
	CostValue int
}

func (c *ChaosPowerUp) ID() string   { return "chaos" }
func (c *ChaosPowerUp) Name() string { return "Chaos" }
func (c *ChaosPowerUp) Description() string {
	return "Reshuffles the positions of all cards that are not yet matched."
}
func (c *ChaosPowerUp) Cost() int   { return c.CostValue }
func (c *ChaosPowerUp) Rarity() int { return RarityUncommon }

func (c *ChaosPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	game.ShuffleUnmatched(board)
	return nil
}

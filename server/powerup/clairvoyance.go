package powerup

import (
	"memory-game-server/game"
)

// ClairvoyancePowerUp reveals a 3x3 region centered on the chosen card for a short duration, then hides those cards again.
// Activation and duration are applied in the game layer (handleUsePowerUp, handleHideClairvoyanceReveal).
type ClairvoyancePowerUp struct {
	CostValue      int
	RevealDuration int // seconds, for description only; actual duration comes from config
}

func (c *ClairvoyancePowerUp) ID() string   { return "clairvoyance" }
func (c *ClairvoyancePowerUp) Name() string { return "Clairvoyance" }
func (c *ClairvoyancePowerUp) Description() string {
	return "Reveals a 3x3 area around the tile you choose for a few seconds, then hides it again."
}
func (c *ClairvoyancePowerUp) Cost() int   { return c.CostValue }
func (c *ClairvoyancePowerUp) Rarity() int { return RarityUncommon }

func (c *ClairvoyancePowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// Effect is applied in game.handleUsePowerUp (reveal 3x3, schedule hide).
	return nil
}

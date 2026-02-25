package powerup

import (
	"memory-game-server/game"
)

// UnveilingPowerUp highlights (without revealing) all tiles that have never been revealed.
// Activation is applied in the game layer (Player.UnveilingHighlightActive). The effect
// lasts only for the current turn and is cleared when the turn ends (match, mismatch, or timeout).
// When Chaos is used, KnownIndices is cleared and UnveilingHighlightActive is reset for both players.
type UnveilingPowerUp struct {
	CostValue int
}

func (d *UnveilingPowerUp) ID() string          { return "unveiling" }
func (d *UnveilingPowerUp) Name() string        { return "Unveiling" }
func (d *UnveilingPowerUp) Description() string { return "Highlights all tiles that have never been revealed (this turn only)." }
func (d *UnveilingPowerUp) Cost() int           { return d.CostValue }

func (d *UnveilingPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// No-op; highlight is activated in game.handleUsePowerUp (UnveilingHighlightActive).
	return nil
}

package powerup

import (
	"memory-game-server/game"
)

// DiscernmentPowerUp highlights (without revealing) all tiles that have never been revealed.
// Activation is applied in the game layer (Player.DiscernmentHighlightActive). When Chaos is used,
// KnownIndices is cleared and DiscernmentHighlightActive is reset for both players.
type DiscernmentPowerUp struct {
	CostValue int
}

func (d *DiscernmentPowerUp) ID() string          { return "discernment" }
func (d *DiscernmentPowerUp) Name() string        { return "Discernment" }
func (d *DiscernmentPowerUp) Description() string { return "Highlights all tiles that have never been revealed." }
func (d *DiscernmentPowerUp) Cost() int           { return d.CostValue }

func (d *DiscernmentPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player) error {
	// No-op; highlight is activated in game.handleUsePowerUp (DiscernmentHighlightActive).
	return nil
}

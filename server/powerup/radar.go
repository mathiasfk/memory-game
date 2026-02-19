package powerup

import (
	"fmt"
	"memory-game-server/game"
)

// RadarPowerUp reveals a 3x3 region centered on the chosen card for a short duration, then hides those cards again.
// Activation and duration are applied in the game layer (handleUsePowerUp, handleHideRadarReveal).
type RadarPowerUp struct {
	CostValue      int
	RevealDuration int // seconds, for description only; actual duration comes from config
}

func (r *RadarPowerUp) ID() string   { return "radar" }
func (r *RadarPowerUp) Name() string { return "Radar" }
func (r *RadarPowerUp) Description() string {
	sec := r.RevealDuration
	if sec <= 0 {
		sec = 1
	}
	return fmt.Sprintf("Reveals a 3x3 area around the card you choose for %d second(s), then hides it again.", sec)
}
func (r *RadarPowerUp) Cost() int { return r.CostValue }

func (r *RadarPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player) error {
	// Effect is applied in game.handleUsePowerUp (reveal 3x3, schedule hide).
	return nil
}

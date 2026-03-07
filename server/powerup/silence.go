package powerup

import (
	"memory-game-server/game"
)

// SilencePowerUp makes the player pass their turn immediately without revealing a pair.
// Turn-pass logic is applied in the game layer (handleUsePowerUp).
type SilencePowerUp struct {
	CostValue int
}

func (s *SilencePowerUp) ID() string   { return "silence" }
func (s *SilencePowerUp) Name() string { return "Silence" }
func (s *SilencePowerUp) Description() string {
	return "Pass your turn immediately without revealing a pair."
}
func (s *SilencePowerUp) Cost() int   { return s.CostValue }
func (s *SilencePowerUp) Rarity() int { return RarityUncommon }

func (s *SilencePowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// Effect is applied in game.handleUsePowerUp (pass turn).
	return nil
}

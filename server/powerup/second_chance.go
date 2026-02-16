package powerup

import (
	"fmt"
	"memory-game-server/game"
)

// SecondChancePowerUp grants +1 point per mismatch while active. It stays active for a fixed number of rounds after use.
// Activation and duration are applied in the game layer (handleUsePowerUp, handleResolveMismatch).
type SecondChancePowerUp struct {
	CostValue       int
	DurationRounds  int
}

func (s *SecondChancePowerUp) ID() string   { return "second_chance" }
func (s *SecondChancePowerUp) Name() string { return "Second chance" }
func (s *SecondChancePowerUp) Description() string {
	n := s.DurationRounds
	if n <= 0 {
		n = 5
	}
	return fmt.Sprintf("+1 point per mismatch while active. Lasts %d rounds.", n)
}
func (s *SecondChancePowerUp) Cost() int { return s.CostValue }

func (s *SecondChancePowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player) error {
	// Effect is applied in game.handleUsePowerUp (set active until round) and game.handleResolveMismatch (+1 point).
	return nil
}

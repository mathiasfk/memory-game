package powerup

import (
	"memory-game-server/game"
)

// ShufflePowerUp reshuffles all unmatched cards on the board.
type ShufflePowerUp struct {
	CostValue int
}

func (s *ShufflePowerUp) ID() string          { return "shuffle" }
func (s *ShufflePowerUp) Name() string        { return "Shuffle" }
func (s *ShufflePowerUp) Description() string { return "Reshuffles the positions of all cards that are not yet matched." }
func (s *ShufflePowerUp) Cost() int           { return s.CostValue }

func (s *ShufflePowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player) error {
	game.ShuffleUnmatched(board)
	return nil
}

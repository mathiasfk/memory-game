package powerup

import (
	"memory-game-server/game"
)

// LeechPowerUp makes points earned this turn from matching be subtracted from the opponent.
// Activation and score logic are applied in the game layer (handleUsePowerUp, handleFlipCard).
type LeechPowerUp struct {
	CostValue int
}

func (l *LeechPowerUp) ID() string          { return "leech" }
func (l *LeechPowerUp) Name() string        { return "Leech" }
func (l *LeechPowerUp) Description() string { return "This turn, all points you earn from matching are subtracted from the opponent (until you miss a pair)." }
func (l *LeechPowerUp) Cost() int           { return l.CostValue }
func (l *LeechPowerUp) Rarity() int         { return RarityRare }

func (l *LeechPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// Effect is applied in game.handleUsePowerUp (LeechActive) and handleFlipCard (score drain).
	return nil
}

package powerup

import (
	"memory-game-server/game"
)

// OblivionPowerUp removes the chosen tile and its pair from the game. No one gains or loses points.
// Target selection and removal are applied in the game layer (handleUsePowerUp).
type OblivionPowerUp struct {
	CostValue int
}

func (o *OblivionPowerUp) ID() string          { return "oblivion" }
func (o *OblivionPowerUp) Name() string        { return "Oblivion" }
func (o *OblivionPowerUp) Description() string { return "Select a tile. It and its pair are removed from the game. No one gains or loses points." }
func (o *OblivionPowerUp) Cost() int           { return o.CostValue }
func (o *OblivionPowerUp) Rarity() int         { return RarityCommon }

func (o *OblivionPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// Effect is applied in game.handleUsePowerUp (remove target and pair).
	return nil
}

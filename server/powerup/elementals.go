package powerup

import (
	"memory-game-server/game"
)

// EarthElementalPowerUp highlights all tiles of the earth element (this turn only; no symbol reveal).
// Activation is applied in the game layer (Player.HighlightIndices).
type EarthElementalPowerUp struct {
	CostValue int
}

func (e *EarthElementalPowerUp) ID() string          { return "earth_elemental" }
func (e *EarthElementalPowerUp) Name() string      { return "Elemental da Terra" }
func (e *EarthElementalPowerUp) Description() string { return "Destaca todos os tiles do elemento Terra (apenas este turno), sem revelar o símbolo." }
func (e *EarthElementalPowerUp) Cost() int         { return e.CostValue }
func (e *EarthElementalPowerUp) Rarity() int       { return RarityCommon }

func (e *EarthElementalPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	return nil
}

// FireElementalPowerUp highlights all tiles of the fire element (this turn only; no symbol reveal).
type FireElementalPowerUp struct {
	CostValue int
}

func (e *FireElementalPowerUp) ID() string          { return "fire_elemental" }
func (e *FireElementalPowerUp) Name() string       { return "Elemental do Fogo" }
func (e *FireElementalPowerUp) Description() string { return "Destaca todos os tiles do elemento Fogo (apenas este turno), sem revelar o símbolo." }
func (e *FireElementalPowerUp) Cost() int          { return e.CostValue }
func (e *FireElementalPowerUp) Rarity() int        { return RarityCommon }

func (e *FireElementalPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	return nil
}

// WaterElementalPowerUp highlights all tiles of the water element (this turn only; no symbol reveal).
type WaterElementalPowerUp struct {
	CostValue int
}

func (e *WaterElementalPowerUp) ID() string          { return "water_elemental" }
func (e *WaterElementalPowerUp) Name() string        { return "Elemental da Água" }
func (e *WaterElementalPowerUp) Description() string { return "Destaca todos os tiles do elemento Água (apenas este turno), sem revelar o símbolo." }
func (e *WaterElementalPowerUp) Cost() int           { return e.CostValue }
func (e *WaterElementalPowerUp) Rarity() int          { return RarityCommon }

func (e *WaterElementalPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	return nil
}

// AirElementalPowerUp highlights all tiles of the air element (this turn only; no symbol reveal).
type AirElementalPowerUp struct {
	CostValue int
}

func (e *AirElementalPowerUp) ID() string          { return "air_elemental" }
func (e *AirElementalPowerUp) Name() string        { return "Elemental do Ar" }
func (e *AirElementalPowerUp) Description() string { return "Destaca todos os tiles do elemento Ar (apenas este turno), sem revelar o símbolo." }
func (e *AirElementalPowerUp) Cost() int           { return e.CostValue }
func (e *AirElementalPowerUp) Rarity() int          { return RarityCommon }

func (e *AirElementalPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	return nil
}

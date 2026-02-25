package powerup

import (
	"memory-game-server/game"
)

// BloodPactPowerUp: if the player matches 3 pairs in a row they gain +5; if they fail (mismatch or turn timeout) they lose 3 points.
// Logic is applied in the game layer (handleUsePowerUp, handleFlipCard, handleResolveMismatch, handleTurnTimeout).
type BloodPactPowerUp struct {
	CostValue int
}

func (b *BloodPactPowerUp) ID() string          { return "blood_pact" }
func (b *BloodPactPowerUp) Name() string        { return "Blood Pact" }
func (b *BloodPactPowerUp) Description() string { return "If you match 3 pairs in a row, you gain +5 points. If you fail (mismatch) before that, you lose 3 points." }
func (b *BloodPactPowerUp) Cost() int           { return b.CostValue }
func (b *BloodPactPowerUp) Rarity() int         { return RarityUncommon }

func (b *BloodPactPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	// Effect is applied in game loop (BloodPactActive, BloodPactMatchesCount).
	return nil
}

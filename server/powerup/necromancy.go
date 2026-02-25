package powerup

import (
	"memory-game-server/game"
)

// NecromancyPowerUp returns all matched (collected) tiles back to the board as hidden, then shuffles
// only those revived tiles; tiles that were already hidden stay in place. The Necromancy tile itself
// is not revived (excluded via ctx.SelfPairID) to prevent infinite reuse.
type NecromancyPowerUp struct {
	CostValue int
}

func (n *NecromancyPowerUp) ID() string          { return "necromancy" }
func (n *NecromancyPowerUp) Name() string        { return "Necromancy" }
func (n *NecromancyPowerUp) Description() string { return "Returns all collected tiles back to the board in new random positions." }
func (n *NecromancyPowerUp) Cost() int           { return n.CostValue }
func (n *NecromancyPowerUp) Rarity() int         { return RarityUncommon }

func (n *NecromancyPowerUp) Apply(board *game.Board, active *game.Player, opponent *game.Player, ctx *game.PowerUpContext) error {
	selfPairID := -1
	if ctx != nil && ctx.SelfPairID >= 0 {
		selfPairID = ctx.SelfPairID
	}
	var revivedIndices []int
	for i := range board.Cards {
		c := &board.Cards[i]
		if c.State != game.Matched {
			continue
		}
		if c.PairID == selfPairID {
			continue // do not revive the Necromancy tile itself
		}
		revivedIndices = append(revivedIndices, i)
		c.State = game.Hidden
	}
	game.ShufflePairIDsAmongIndices(board, revivedIndices)
	return nil
}

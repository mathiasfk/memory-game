package heuristic

import (
	"memory-game-server/game"
)

const (
	necromancyBonusFactor = 0.15
	necromancyRevivedCap  = 6
)

func init() {
	Register("necromancy", evNecromancy, nil)
}

// evNecromancy returns the expected value of using Necromancy when the AI is behind.
// Necromancy returns all matched tiles (except itself) to the board as hidden and shuffles them.
// We only consider it when behind on score and when there are matched pairs to revive.
// EV = evRevert (RandomMatchProb after revert) + bonus proportional to score gap and revived pairs.
func evNecromancy(state *game.GameStateMsg, memory map[int]int, hidden []int, P int) float64 {
	if state.You.Score >= state.Opponent.Score {
		return -1
	}

	matchedCount := 0
	var necromancyMatchedCount int
	necromancyPairID := -1
	if state.PairIDToPowerUp != nil {
		for pairID, id := range state.PairIDToPowerUp {
			if id == "necromancy" {
				necromancyPairID = pairID
				break
			}
		}
	}
	for i := range state.Cards {
		c := &state.Cards[i]
		if c.State != "matched" {
			continue
		}
		matchedCount++
		if necromancyPairID >= 0 && c.PairID != nil && *c.PairID == necromancyPairID {
			necromancyMatchedCount++
		}
	}

	revivedPairs := matchedCount / 2
	if necromancyPairID >= 0 && necromancyMatchedCount == 2 {
		revivedPairs--
	}
	if revivedPairs <= 0 {
		return -1
	}

	PAfter := P + revivedPairs
	evRevert := RandomMatchProb(PAfter)
	scoreGap := state.Opponent.Score - state.You.Score
	capRevived := revivedPairs
	if capRevived > necromancyRevivedCap {
		capRevived = necromancyRevivedCap
	}
	bonus := float64(scoreGap) * necromancyBonusFactor * float64(capRevived)
	return evRevert + bonus
}

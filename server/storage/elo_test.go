package storage

import (
	"testing"
)

func TestComputeEloUpdates_WinLoss(t *testing.T) {
	// Same rating (1000 vs 1000), player 0 wins -> player 0 gains, player 1 loses
	newR0, newR1 := computeEloUpdates(1000, 1000, 0)
	if newR0 <= 1000 {
		t.Errorf("winner (0) should gain: got R0=%d", newR0)
	}
	if newR1 >= 1000 {
		t.Errorf("loser (1) should lose: got R1=%d", newR1)
	}
	// Symmetric: player 1 wins
	newR0, newR1 = computeEloUpdates(1000, 1000, 1)
	if newR0 >= 1000 {
		t.Errorf("loser (0) should lose: got R0=%d", newR0)
	}
	if newR1 <= 1000 {
		t.Errorf("winner (1) should gain: got R1=%d", newR1)
	}
}

func TestComputeEloUpdates_Draw(t *testing.T) {
	// Same rating: draw -> both stay ~1000 (small change due to float)
	newR0, newR1 := computeEloUpdates(1000, 1000, -1)
	if newR0 < 990 || newR0 > 1010 {
		t.Errorf("draw at same rating: R0 should stay ~1000, got %d", newR0)
	}
	if newR1 < 990 || newR1 > 1010 {
		t.Errorf("draw at same rating: R1 should stay ~1000, got %d", newR1)
	}
}

func TestComputeEloUpdates_WeakerPlayerDrawsWithStronger(t *testing.T) {
	// Weaker (800) draws with stronger (1200): weaker should gain, stronger should lose
	r0Weak, r1Strong := 800, 1200
	newR0, newR1 := computeEloUpdates(r0Weak, r1Strong, -1)
	if newR0 <= r0Weak {
		t.Errorf("weaker player should gain on draw: had %d, got %d", r0Weak, newR0)
	}
	if newR1 >= r1Strong {
		t.Errorf("stronger player should lose on draw: had %d, got %d", r1Strong, newR1)
	}
}

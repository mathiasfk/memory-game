package powerup

import (
	"strings"
	"testing"

	"memory-game-server/game"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	s := &ShufflePowerUp{CostValue: 3}
	r.Register(s)

	def, ok := r.GetPowerUp("shuffle")
	if !ok {
		t.Fatal("expected to find 'shuffle' power-up in registry")
	}
	if def.ID != "shuffle" {
		t.Errorf("expected ID='shuffle', got %q", def.ID)
	}
	if def.Name != "Shuffle" {
		t.Errorf("expected Name='Shuffle', got %q", def.Name)
	}
	if def.Cost != 3 {
		t.Errorf("expected Cost=3, got %d", def.Cost)
	}
}

func TestRegistryGetNonExistent(t *testing.T) {
	r := NewRegistry()

	_, ok := r.GetPowerUp("nonexistent")
	if ok {
		t.Error("expected GetPowerUp to return false for nonexistent power-up")
	}
}

func TestRegistryAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&ShufflePowerUp{CostValue: 3})

	all := r.AllPowerUps()
	if len(all) != 1 {
		t.Fatalf("expected 1 power-up, got %d", len(all))
	}
	if all[0].ID != "shuffle" {
		t.Errorf("expected first power-up ID='shuffle', got %q", all[0].ID)
	}
}

func TestShufflePowerUpApply(t *testing.T) {
	s := &ShufflePowerUp{CostValue: 3}

	board := game.NewBoard(4, 4)

	// Record original pairIDs
	originalPairIDs := make([]int, len(board.Cards))
	for i, card := range board.Cards {
		originalPairIDs[i] = card.PairID
	}

	// Mark a couple as matched
	board.Cards[0].State = game.Matched
	board.Cards[1].State = game.Matched

	p1 := &game.Player{Name: "Alice", Score: 5}
	p2 := &game.Player{Name: "Bob", Score: 0}

	err := s.Apply(board, p1, p2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Matched cards should keep their pairID
	if board.Cards[0].PairID != originalPairIDs[0] {
		t.Error("matched card[0] pairID changed after shuffle")
	}
	if board.Cards[1].PairID != originalPairIDs[1] {
		t.Error("matched card[1] pairID changed after shuffle")
	}

	// All pair counts should still be exactly 2
	pairCount := make(map[int]int)
	for _, card := range board.Cards {
		pairCount[card.PairID]++
	}
	for pairID, count := range pairCount {
		if count != 2 {
			t.Errorf("pair %d has %d cards after shuffle, expected 2", pairID, count)
		}
	}
}

func TestShufflePowerUpMetadata(t *testing.T) {
	s := &ShufflePowerUp{CostValue: 5}

	if s.ID() != "shuffle" {
		t.Errorf("expected ID='shuffle', got %q", s.ID())
	}
	if s.Name() != "Shuffle" {
		t.Errorf("expected Name='Shuffle', got %q", s.Name())
	}
	if s.Description() == "" {
		t.Error("expected non-empty description")
	}
	if s.Cost() != 5 {
		t.Errorf("expected Cost=5, got %d", s.Cost())
	}
}

func TestSecondChancePowerUpMetadata(t *testing.T) {
	sc := &SecondChancePowerUp{CostValue: 2, DurationRounds: 5}

	if sc.ID() != "second_chance" {
		t.Errorf("expected ID='second_chance', got %q", sc.ID())
	}
	if sc.Name() != "Second chance" {
		t.Errorf("expected Name='Second chance', got %q", sc.Name())
	}
	if sc.Description() == "" {
		t.Error("expected non-empty description")
	}
	if !strings.Contains(sc.Description(), "5 rounds") {
		t.Errorf("expected description to mention 5 rounds, got %q", sc.Description())
	}
	if sc.Cost() != 2 {
		t.Errorf("expected Cost=2, got %d", sc.Cost())
	}
}

func TestSecondChancePowerUpApply(t *testing.T) {
	sc := &SecondChancePowerUp{CostValue: 2, DurationRounds: 5}
	board := game.NewBoard(2, 2)
	p1 := &game.Player{Name: "Alice"}
	p2 := &game.Player{Name: "Bob"}

	err := sc.Apply(board, p1, p2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Apply is a no-op; activation is handled in game layer
}

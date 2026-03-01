package matchmaking

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"memory-game-server/config"
	"memory-game-server/game"
	"memory-game-server/ws"
)

type mockPowerUpProvider struct{}

func (m *mockPowerUpProvider) GetPowerUp(id string) (game.PowerUpDef, bool) {
	return game.PowerUpDef{}, false
}

func (m *mockPowerUpProvider) AllPowerUps() []game.PowerUpDef {
	return nil
}

func (m *mockPowerUpProvider) PickArcanaForMatch(n int) []game.PowerUpDef {
	return nil
}

// mockClient creates a client-like struct for testing.
// Since Client depends on websocket.Conn and Hub, we need to work around this.
// We'll test the matchmaker's Enqueue behavior and pairing.

func TestMatchmakerPairsPlayers(t *testing.T) {
	cfg := &config.Config{
		BoardRows:          2,
		BoardCols:          2,
		RevealDurationMS:   100,
		MaxNameLength:      24,
		WSPort:             8080,
		AIPairTimeoutSec:   60,
		PowerUps:           config.PowerUpsConfig{Chaos: config.ChaosPowerUpConfig{Cost: 3}, Clairvoyance: config.ClairvoyancePowerUpConfig{}},
		AIProfiles:         []config.AIParams{{Name: "Mnemosyne", DelayMinMS: 100, DelayMaxMS: 500, UseBestMoveChance: 85, ArcanaRandomness: 0}},
	}

	pups := &mockPowerUpProvider{}
	mm := NewMatchmaker(cfg, pups, nil)
	go mm.Run(context.Background())

	// Create two mock clients with send channels
	send1 := make(chan []byte, 100)
	send2 := make(chan []byte, 100)

	c1 := &ws.Client{
		Send: send1,
		Name: "Alice",
	}
	c2 := &ws.Client{
		Send: send2,
		Name: "Bob",
	}

	// Enqueue both
	mm.Enqueue(c1)
	mm.Enqueue(c2)

	// Wait for pairing
	time.Sleep(200 * time.Millisecond)

	// Both clients should have received MatchFound messages
	checkMatchFound := func(ch chan []byte, expectedOpponent string) {
		select {
		case msg := <-ch:
			var mf ws.MatchFoundMsg
			if err := json.Unmarshal(msg, &mf); err != nil {
				t.Fatalf("failed to unmarshal MatchFound: %v", err)
			}
			if mf.Type != "match_found" {
				t.Errorf("expected type 'match_found', got %q", mf.Type)
			}
			if mf.OpponentName != expectedOpponent {
				t.Errorf("expected opponent name %q, got %q", expectedOpponent, mf.OpponentName)
			}
			if mf.BoardRows != cfg.BoardRows {
				t.Errorf("expected boardRows=%d, got %d", cfg.BoardRows, mf.BoardRows)
			}
			if mf.BoardCols != cfg.BoardCols {
				t.Errorf("expected boardCols=%d, got %d", cfg.BoardCols, mf.BoardCols)
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for MatchFound message")
		}
	}

	checkMatchFound(send1, "Bob")
	checkMatchFound(send2, "Alice")

	// Both clients should have a game assigned
	if c1.Game == nil {
		t.Error("client 1 should have a game assigned")
	}
	if c2.Game == nil {
		t.Error("client 2 should have a game assigned")
	}
	if c1.Game != c2.Game {
		t.Error("both clients should be in the same game")
	}
}

func TestMatchmakerPairsWithAIAfterTimeout(t *testing.T) {
	cfg := &config.Config{
		BoardRows:          2,
		BoardCols:          2,
		RevealDurationMS:   100,
		MaxNameLength:      24,
		WSPort:             8080,
		AIPairTimeoutSec:   0, // very short: 0s, so AI pairs almost immediately
		PowerUps:           config.PowerUpsConfig{Chaos: config.ChaosPowerUpConfig{Cost: 3}, Clairvoyance: config.ClairvoyancePowerUpConfig{}},
		AIProfiles:         []config.AIParams{{Name: "Mnemosyne", DelayMinMS: 10, DelayMaxMS: 50, UseBestMoveChance: 85, ArcanaRandomness: 0}},
	}

	pups := &mockPowerUpProvider{}
	mm := NewMatchmaker(cfg, pups, nil)
	go mm.Run(context.Background())

	send1 := make(chan []byte, 100)
	c1 := &ws.Client{Send: send1, Name: "Alice"}

	mm.Enqueue(c1)

	// Wait for AI pair (timeout is 0 so we get AI quickly)
	time.Sleep(200 * time.Millisecond)

	select {
	case msg := <-send1:
		var mf ws.MatchFoundMsg
		if err := json.Unmarshal(msg, &mf); err != nil {
			t.Fatalf("failed to unmarshal MatchFound: %v", err)
		}
		if mf.Type != "match_found" {
			t.Errorf("expected type match_found, got %q", mf.Type)
		}
		if mf.OpponentName != "Mnemosyne" {
			t.Errorf("expected opponent Mnemosyne, got %q", mf.OpponentName)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for MatchFound (AI)")
	}

	if c1.Game == nil {
		t.Error("client should have a game assigned (vs AI)")
	}
}

func TestMatchmakerLeaveQueue(t *testing.T) {
	cfg := &config.Config{
		BoardRows:          2,
		BoardCols:          2,
		RevealDurationMS:   100,
		MaxNameLength:      24,
		WSPort:             8080,
		AIPairTimeoutSec:   2, // give time to leave before AI pairs
		PowerUps:           config.PowerUpsConfig{Chaos: config.ChaosPowerUpConfig{Cost: 3}, Clairvoyance: config.ClairvoyancePowerUpConfig{}},
		AIProfiles:         []config.AIParams{{Name: "Mnemosyne", DelayMinMS: 10, DelayMaxMS: 50, UseBestMoveChance: 85, ArcanaRandomness: 0}},
	}

	pups := &mockPowerUpProvider{}
	mm := NewMatchmaker(cfg, pups, nil)
	go mm.Run(context.Background())

	send1 := make(chan []byte, 100)
	c1 := &ws.Client{Send: send1, Name: "Alice"}

	mm.Enqueue(c1)
	time.Sleep(50 * time.Millisecond)

	// Cancel before timeout: leave queue
	mm.LeaveQueue(c1)
	time.Sleep(100 * time.Millisecond)

	// c1 should not have a game (they left the queue)
	if c1.Game != nil {
		t.Error("client should not have a game after leaving queue")
	}

	// No match_found should arrive
	select {
	case msg := <-send1:
		var mf ws.MatchFoundMsg
		if err := json.Unmarshal(msg, &mf); err == nil && mf.Type == "match_found" {
			t.Error("client who left queue should not receive match_found")
		}
	case <-time.After(100 * time.Millisecond):
		// expected: no match
	}
}

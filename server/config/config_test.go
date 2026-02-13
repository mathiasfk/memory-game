package config

import (
	"os"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.BoardRows != 4 {
		t.Errorf("expected BoardRows=4, got %d", cfg.BoardRows)
	}
	if cfg.BoardCols != 4 {
		t.Errorf("expected BoardCols=4, got %d", cfg.BoardCols)
	}
	if cfg.ComboBasePoints != 1 {
		t.Errorf("expected ComboBasePoints=1, got %d", cfg.ComboBasePoints)
	}
	if cfg.RevealDurationMS != 1000 {
		t.Errorf("expected RevealDurationMS=1000, got %d", cfg.RevealDurationMS)
	}
	if cfg.PowerUpShuffleCost != 3 {
		t.Errorf("expected PowerUpShuffleCost=3, got %d", cfg.PowerUpShuffleCost)
	}
	if cfg.MaxNameLength != 24 {
		t.Errorf("expected MaxNameLength=24, got %d", cfg.MaxNameLength)
	}
	if cfg.WSPort != 8080 {
		t.Errorf("expected WSPort=8080, got %d", cfg.WSPort)
	}
	if cfg.MaxLatencyMS != 500 {
		t.Errorf("expected MaxLatencyMS=500, got %d", cfg.MaxLatencyMS)
	}
	if cfg.AIPairTimeoutSec != 60 {
		t.Errorf("expected AIPairTimeoutSec=60, got %d", cfg.AIPairTimeoutSec)
	}
	if cfg.AIName != "Mnemosyne" {
		t.Errorf("expected AIName=Mnemosyne, got %q", cfg.AIName)
	}
	if cfg.AIDelayMinMS != 800 {
		t.Errorf("expected AIDelayMinMS=800, got %d", cfg.AIDelayMinMS)
	}
	if cfg.AIDelayMaxMS != 2500 {
		t.Errorf("expected AIDelayMaxMS=2500, got %d", cfg.AIDelayMaxMS)
	}
	if cfg.AIUseKnownPairChance != 85 {
		t.Errorf("expected AIUseKnownPairChance=85, got %d", cfg.AIUseKnownPairChance)
	}
}

func TestLoadWithEnvOverrides(t *testing.T) {
	os.Setenv("BOARD_ROWS", "6")
	os.Setenv("BOARD_COLS", "6")
	os.Setenv("COMBO_BASE_POINTS", "2")
	os.Setenv("WS_PORT", "9090")
	defer func() {
		os.Unsetenv("BOARD_ROWS")
		os.Unsetenv("BOARD_COLS")
		os.Unsetenv("COMBO_BASE_POINTS")
		os.Unsetenv("WS_PORT")
	}()

	cfg := Load()

	if cfg.BoardRows != 6 {
		t.Errorf("expected BoardRows=6 after env override, got %d", cfg.BoardRows)
	}
	if cfg.BoardCols != 6 {
		t.Errorf("expected BoardCols=6 after env override, got %d", cfg.BoardCols)
	}
	if cfg.ComboBasePoints != 2 {
		t.Errorf("expected ComboBasePoints=2 after env override, got %d", cfg.ComboBasePoints)
	}
	if cfg.WSPort != 9090 {
		t.Errorf("expected WSPort=9090 after env override, got %d", cfg.WSPort)
	}
	// Non-overridden fields should remain default
	if cfg.RevealDurationMS != 1000 {
		t.Errorf("expected RevealDurationMS=1000 (default), got %d", cfg.RevealDurationMS)
	}
}

func TestLoadWithAIEnvOverrides(t *testing.T) {
	os.Setenv("AI_PAIR_TIMEOUT_SEC", "30")
	os.Setenv("AI_NAME", "TestBot")
	os.Setenv("AI_DELAY_MIN_MS", "500")
	os.Setenv("AI_DELAY_MAX_MS", "3000")
	os.Setenv("AI_USE_KNOWN_PAIR_CHANCE", "90")
	defer func() {
		os.Unsetenv("AI_PAIR_TIMEOUT_SEC")
		os.Unsetenv("AI_NAME")
		os.Unsetenv("AI_DELAY_MIN_MS")
		os.Unsetenv("AI_DELAY_MAX_MS")
		os.Unsetenv("AI_USE_KNOWN_PAIR_CHANCE")
	}()

	cfg := Load()

	if cfg.AIPairTimeoutSec != 30 {
		t.Errorf("expected AIPairTimeoutSec=30, got %d", cfg.AIPairTimeoutSec)
	}
	if cfg.AIName != "TestBot" {
		t.Errorf("expected AIName=TestBot, got %q", cfg.AIName)
	}
	if cfg.AIDelayMinMS != 500 {
		t.Errorf("expected AIDelayMinMS=500, got %d", cfg.AIDelayMinMS)
	}
	if cfg.AIDelayMaxMS != 3000 {
		t.Errorf("expected AIDelayMaxMS=3000, got %d", cfg.AIDelayMaxMS)
	}
	if cfg.AIUseKnownPairChance != 90 {
		t.Errorf("expected AIUseKnownPairChance=90, got %d", cfg.AIUseKnownPairChance)
	}
}

func TestLoadWithInvalidEnv(t *testing.T) {
	os.Setenv("BOARD_ROWS", "invalid")
	defer os.Unsetenv("BOARD_ROWS")

	cfg := Load()

	// Should fall back to default when env value is invalid
	if cfg.BoardRows != 4 {
		t.Errorf("expected BoardRows=4 (default) with invalid env, got %d", cfg.BoardRows)
	}
}

package config

import (
	"os"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	if cfg.BoardRows != 6 {
		t.Errorf("expected BoardRows=6, got %d", cfg.BoardRows)
	}
	if cfg.BoardCols != 6 {
		t.Errorf("expected BoardCols=6, got %d", cfg.BoardCols)
	}
	if cfg.ComboBasePoints != 1 {
		t.Errorf("expected ComboBasePoints=1, got %d", cfg.ComboBasePoints)
	}
	if cfg.RevealDurationMS != 1000 {
		t.Errorf("expected RevealDurationMS=1000, got %d", cfg.RevealDurationMS)
	}
	// Power-up Cost is deprecated (power-ups earned by matching pairs, use is free)
	if cfg.PowerUps.SecondChance.DurationRounds != 5 {
		t.Errorf("expected PowerUps.SecondChance.DurationRounds=5, got %d", cfg.PowerUps.SecondChance.DurationRounds)
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
	if cfg.AIPairTimeoutSec != 15 {
		t.Errorf("expected AIPairTimeoutSec=15, got %d", cfg.AIPairTimeoutSec)
	}
	if cfg.TurnLimitSec != 60 {
		t.Errorf("expected TurnLimitSec=60, got %d", cfg.TurnLimitSec)
	}
	if cfg.TurnCountdownShowSec != 30 {
		t.Errorf("expected TurnCountdownShowSec=30, got %d", cfg.TurnCountdownShowSec)
	}
	if len(cfg.AIProfiles) != 3 {
		t.Fatalf("expected 3 AI profiles, got %d", len(cfg.AIProfiles))
	}
	if cfg.AIProfiles[0].Name != "Mnemosyne" {
		t.Errorf("expected first AI name Mnemosyne, got %q", cfg.AIProfiles[0].Name)
	}
	if cfg.AIProfiles[0].DelayMinMS != 1000 || cfg.AIProfiles[0].DelayMaxMS != 2500 || cfg.AIProfiles[0].UseKnownPairChance != 90 || cfg.AIProfiles[0].ForgetChance != 1 {
		t.Errorf("expected Mnemosyne 1000/2500/90 ForgetChance=1, got %d/%d/%d ForgetChance=%d", cfg.AIProfiles[0].DelayMinMS, cfg.AIProfiles[0].DelayMaxMS, cfg.AIProfiles[0].UseKnownPairChance, cfg.AIProfiles[0].ForgetChance)
	}
	if cfg.AIProfiles[1].Name != "Calliope" {
		t.Errorf("expected second AI name Calliope, got %q", cfg.AIProfiles[1].Name)
	}
	if cfg.AIProfiles[1].DelayMinMS != 500 || cfg.AIProfiles[1].DelayMaxMS != 1100 || cfg.AIProfiles[1].UseKnownPairChance != 80 || cfg.AIProfiles[1].ForgetChance != 15 {
		t.Errorf("expected Calliope 500/1100/80 ForgetChance=15, got %d/%d/%d ForgetChance=%d", cfg.AIProfiles[1].DelayMinMS, cfg.AIProfiles[1].DelayMaxMS, cfg.AIProfiles[1].UseKnownPairChance, cfg.AIProfiles[1].ForgetChance)
	}
	if cfg.AIProfiles[2].Name != "Thalia" {
		t.Errorf("expected third AI name Thalia, got %q", cfg.AIProfiles[2].Name)
	}
	if cfg.AIProfiles[2].DelayMinMS != 500 || cfg.AIProfiles[2].DelayMaxMS != 2000 || cfg.AIProfiles[2].UseKnownPairChance != 85 || cfg.AIProfiles[2].ForgetChance != 30 {
		t.Errorf("expected Thalia 500/2000/85 ForgetChance=30, got %d/%d/%d ForgetChance=%d", cfg.AIProfiles[2].DelayMinMS, cfg.AIProfiles[2].DelayMaxMS, cfg.AIProfiles[2].UseKnownPairChance, cfg.AIProfiles[2].ForgetChance)
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
	if len(cfg.AIProfiles) == 0 {
		t.Fatal("expected at least one AI profile")
	}
	if cfg.AIProfiles[0].Name != "TestBot" {
		t.Errorf("expected first AI name TestBot, got %q", cfg.AIProfiles[0].Name)
	}
	if cfg.AIProfiles[0].DelayMinMS != 500 {
		t.Errorf("expected first AI DelayMinMS=500, got %d", cfg.AIProfiles[0].DelayMinMS)
	}
	if cfg.AIProfiles[0].DelayMaxMS != 3000 {
		t.Errorf("expected first AI DelayMaxMS=3000, got %d", cfg.AIProfiles[0].DelayMaxMS)
	}
	if cfg.AIProfiles[0].UseKnownPairChance != 90 {
		t.Errorf("expected first AI UseKnownPairChance=90, got %d", cfg.AIProfiles[0].UseKnownPairChance)
	}
}

func TestLoadWithInvalidEnv(t *testing.T) {
	os.Setenv("BOARD_ROWS", "invalid")
	defer os.Unsetenv("BOARD_ROWS")

	cfg := Load()

	// Should fall back to default when env value is invalid
	if cfg.BoardRows != 6 {
		t.Errorf("expected BoardRows=6 (default) with invalid env, got %d", cfg.BoardRows)
	}
}

func TestLoadWithPowerUpEnvOverrides(t *testing.T) {
	os.Setenv("POWERUP_SECOND_CHANCE_DURATION_ROUNDS", "7")
	defer os.Unsetenv("POWERUP_SECOND_CHANCE_DURATION_ROUNDS")

	cfg := Load()

	if cfg.PowerUps.SecondChance.DurationRounds != 7 {
		t.Errorf("expected PowerUps.SecondChance.DurationRounds=7 after env override, got %d", cfg.PowerUps.SecondChance.DurationRounds)
	}
}

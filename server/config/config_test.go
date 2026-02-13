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

func TestLoadWithInvalidEnv(t *testing.T) {
	os.Setenv("BOARD_ROWS", "invalid")
	defer os.Unsetenv("BOARD_ROWS")

	cfg := Load()

	// Should fall back to default when env value is invalid
	if cfg.BoardRows != 4 {
		t.Errorf("expected BoardRows=4 (default) with invalid env, got %d", cfg.BoardRows)
	}
}

package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

// Config holds all configurable game parameters.
type Config struct {
	BoardRows            int    `json:"board_rows"`
	BoardCols            int    `json:"board_cols"`
	ComboBasePoints      int    `json:"combo_base_points"`
	RevealDurationMS     int    `json:"reveal_duration_ms"`
	PowerUpShuffleCost   int    `json:"powerup_shuffle_cost"`
	MaxNameLength        int    `json:"max_name_length"`
	WSPort               int    `json:"ws_port"`
	MaxLatencyMS         int    `json:"max_latency_ms"`
	AIPairTimeoutSec     int    `json:"ai_pair_timeout_sec"`
	AIName               string `json:"ai_name"`
	AIDelayMinMS         int    `json:"ai_delay_min_ms"`
	AIDelayMaxMS         int    `json:"ai_delay_max_ms"`
	AIUseKnownPairChance int    `json:"ai_use_known_pair_chance"` // 0-100, probability to use a memorized pair when available
}

// Defaults returns a Config with all default values from the spec.
func Defaults() *Config {
	return &Config{
		BoardRows:            6,
		BoardCols:            6,
		ComboBasePoints:      1,
		RevealDurationMS:     1000,
		PowerUpShuffleCost:   3,
		MaxNameLength:        24,
		WSPort:               8080,
		MaxLatencyMS:         500,
		AIPairTimeoutSec:     30,
		AIName:               "Mnemosyne",
		AIDelayMinMS:         800,
		AIDelayMaxMS:         1500,
		AIUseKnownPairChance: 90,
	}
}

// Load reads configuration from an optional config.json file,
// then applies environment variable overrides. Fields not set
// in either source retain their default values.
func Load() *Config {
	cfg := Defaults()

	// Try to load from config.json
	if f, err := os.Open("config.json"); err == nil {
		defer f.Close()
		if err := json.NewDecoder(f).Decode(cfg); err != nil {
			log.Printf("Warning: failed to parse config.json: %v", err)
		}
	}

	// Environment variable overrides
	overrideInt(&cfg.BoardRows, "BOARD_ROWS")
	overrideInt(&cfg.BoardCols, "BOARD_COLS")
	overrideInt(&cfg.ComboBasePoints, "COMBO_BASE_POINTS")
	overrideInt(&cfg.RevealDurationMS, "REVEAL_DURATION_MS")
	overrideInt(&cfg.PowerUpShuffleCost, "POWERUP_SHUFFLE_COST")
	overrideInt(&cfg.MaxNameLength, "MAX_NAME_LENGTH")
	overrideInt(&cfg.WSPort, "WS_PORT")
	overrideInt(&cfg.MaxLatencyMS, "MAX_LATENCY_MS")
	overrideInt(&cfg.AIPairTimeoutSec, "AI_PAIR_TIMEOUT_SEC")
	overrideString(&cfg.AIName, "AI_NAME")
	overrideInt(&cfg.AIDelayMinMS, "AI_DELAY_MIN_MS")
	overrideInt(&cfg.AIDelayMaxMS, "AI_DELAY_MAX_MS")
	overrideInt(&cfg.AIUseKnownPairChance, "AI_USE_KNOWN_PAIR_CHANCE")

	return cfg
}

func overrideInt(field *int, envKey string) {
	if val := os.Getenv(envKey); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			*field = n
		} else {
			log.Printf("Warning: invalid value for %s: %q", envKey, val)
		}
	}
}

func overrideString(field *string, envKey string) {
	if val := os.Getenv(envKey); val != "" {
		*field = val
	}
}

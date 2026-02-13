package config

import (
	"encoding/json"
	"log"
	"os"
	"strconv"
)

// AIParams holds the parameters for one AI profile (name and behavior).
type AIParams struct {
	Name               string `json:"name"`
	DelayMinMS         int    `json:"delay_min_ms"`
	DelayMaxMS         int    `json:"delay_max_ms"`
	UseKnownPairChance int    `json:"use_known_pair_chance"` // 0-100, probability to use a memorized pair when available
}

// Config holds all configurable game parameters.
type Config struct {
	BoardRows          int `json:"board_rows"`
	BoardCols          int `json:"board_cols"`
	ComboBasePoints    int `json:"combo_base_points"`
	RevealDurationMS   int `json:"reveal_duration_ms"`
	PowerUpShuffleCost int `json:"powerup_shuffle_cost"`
	MaxNameLength      int `json:"max_name_length"`
	WSPort             int `json:"ws_port"`
	MaxLatencyMS       int `json:"max_latency_ms"`
	AIPairTimeoutSec   int `json:"ai_pair_timeout_sec"`

	// AIProfiles lists available AI opponents; one is chosen at random when pairing vs AI.
	AIProfiles []AIParams `json:"ai_profiles"`

	// Legacy single-AI fields (used when config.json has ai_name etc. but no ai_profiles)
	AIName               string `json:"ai_name"`
	AIDelayMinMS         int    `json:"ai_delay_min_ms"`
	AIDelayMaxMS         int    `json:"ai_delay_max_ms"`
	AIUseKnownPairChance int    `json:"ai_use_known_pair_chance"`
}

// Defaults returns a Config with all default values from the spec.
func Defaults() *Config {
	return &Config{
		BoardRows:          6,
		BoardCols:          6,
		ComboBasePoints:    1,
		RevealDurationMS:   1000,
		PowerUpShuffleCost: 2,
		MaxNameLength:      24,
		WSPort:             8080,
		MaxLatencyMS:       500,
		AIPairTimeoutSec:   15,
		AIProfiles: []AIParams{
			{Name: "Mnemosyne", DelayMinMS: 1000, DelayMaxMS: 2500, UseKnownPairChance: 90},
			{Name: "Calliope", DelayMinMS: 500, DelayMaxMS: 1100, UseKnownPairChance: 50},
			{Name: "Thalia", DelayMinMS: 500, DelayMaxMS: 2000, UseKnownPairChance: 15},
		},
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

	// If no ai_profiles in config, build one from legacy fields
	if len(cfg.AIProfiles) == 0 {
		name := cfg.AIName
		if name == "" {
			name = "Mnemosyne"
		}
		delayMin := cfg.AIDelayMinMS
		if delayMin == 0 {
			delayMin = 800
		}
		delayMax := cfg.AIDelayMaxMS
		if delayMax == 0 {
			delayMax = 1500
		}
		chance := cfg.AIUseKnownPairChance
		if chance == 0 {
			chance = 90
		}
		cfg.AIProfiles = []AIParams{{Name: name, DelayMinMS: delayMin, DelayMaxMS: delayMax, UseKnownPairChance: chance}}
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
	// Env overrides for first AI profile (backward compatibility)
	if len(cfg.AIProfiles) > 0 {
		overrideString(&cfg.AIProfiles[0].Name, "AI_NAME")
		overrideInt(&cfg.AIProfiles[0].DelayMinMS, "AI_DELAY_MIN_MS")
		overrideInt(&cfg.AIProfiles[0].DelayMaxMS, "AI_DELAY_MAX_MS")
		overrideInt(&cfg.AIProfiles[0].UseKnownPairChance, "AI_USE_KNOWN_PAIR_CHANCE")
	}

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

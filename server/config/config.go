package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// AIParams holds the parameters for one AI profile (name and behavior).
type AIParams struct {
	Name               string `json:"name"`
	DelayMinMS         int    `json:"delay_min_ms"`
	DelayMaxMS         int    `json:"delay_max_ms"`
	UseKnownPairChance int    `json:"use_known_pair_chance"` // 0-100, probability to use a memorized pair when available
	ForgetChance       int    `json:"forget_chance"`         // 0-100, probability to forget (delete from memory) a known card each turn
	ArcanaRandomness   int    `json:"arcana_randomness"`     // 0-100, probability to randomize arcana use decision (avoids robotic play)
}

// ChaosPowerUpConfig holds configuration for the Chaos power-up.
type ChaosPowerUpConfig struct {
	Cost int `json:"cost"`
}

// ClairvoyancePowerUpConfig holds configuration for the Clairvoyance power-up.
type ClairvoyancePowerUpConfig struct {
	Cost             int `json:"cost"`
	RevealDurationMS int `json:"reveal_duration_ms"`
}

// PowerUpsConfig holds per-power-up configuration sections.
type PowerUpsConfig struct {
	Chaos        ChaosPowerUpConfig        `json:"chaos"`
	Clairvoyance ClairvoyancePowerUpConfig `json:"clairvoyance"`
}

// TelemetryHistogramConfig holds bin settings for telemetry histograms (turn and pairs at card use).
type TelemetryHistogramConfig struct {
	TurnMax      int `json:"turn_max"`       // max turn for equal bins; last bin is "TurnMax+"
	TurnNumBins  int `json:"turn_num_bins"`  // e.g. 6 = 5 equal bins in [0,TurnMax) + 1 for TurnMax+
	PairsMax     int `json:"pairs_max"`      // max pairs for equal bins (e.g. 36)
	PairsNumBins int `json:"pairs_num_bins"` // e.g. 6 equal bins in [0,PairsMax]
}

// Config holds all configurable game parameters.
type Config struct {
	BoardRows        int    `json:"board_rows"`
	BoardCols        int    `json:"board_cols"`
	RevealDurationMS int    `json:"reveal_duration_ms"`
	MaxNameLength    int    `json:"max_name_length"`
	WSPort           int    `json:"ws_port"`
	NeonAuthBaseURL  string `json:"-"` // From NEON_AUTH_BASE_URL; not persisted in config.json
	DatabaseURL      string `json:"-"` // From DATABASE_URL; not persisted (override in production)
	MaxLatencyMS     int    `json:"max_latency_ms"`
	AIPairTimeoutSec int    `json:"ai_pair_timeout_sec"`

	// TurnLimitSec is the max time per turn in seconds; 0 = disabled.
	TurnLimitSec int `json:"turn_limit_sec"`
	// TurnCountdownShowSec is how many seconds before turn end to show the countdown.
	TurnCountdownShowSec int `json:"turn_countdown_show_sec"`
	// ReconnectTimeoutSec is how long to wait for a disconnected player to rejoin before ending the game.
	ReconnectTimeoutSec int `json:"reconnect_timeout_sec"`

	// PowerUps holds configuration for each power-up.
	PowerUps PowerUpsConfig `json:"powerups"`

	// AIProfiles lists available AI opponents; one is chosen at random when pairing vs AI.
	AIProfiles []AIParams `json:"ai_profiles"`

	// TelemetryHistogram defines histogram bins for "game stage at use" (turn and pairs already matched).
	TelemetryHistogram TelemetryHistogramConfig `json:"telemetry_histogram"`

	// LogLevel is the minimum log level: "debug", "info", "warn", "error". Default "info".
	LogLevel string `json:"log_level"`
}

// Defaults returns a Config with all default values from the spec.
func Defaults() *Config {
	return &Config{
		BoardRows:            6,
		BoardCols:            6,
		RevealDurationMS:     1000,
		MaxNameLength:        24,
		WSPort:               8080,
		MaxLatencyMS:         500,
		AIPairTimeoutSec:     15,
		TurnLimitSec:         60,
		TurnCountdownShowSec: 30,
		ReconnectTimeoutSec:  120,
		PowerUps: PowerUpsConfig{
			Chaos:        ChaosPowerUpConfig{},
			Clairvoyance: ClairvoyancePowerUpConfig{RevealDurationMS: 3000},
		},
		AIProfiles: []AIParams{
			{Name: "Mnemosyne", DelayMinMS: 1000, DelayMaxMS: 2000, UseKnownPairChance: 90, ForgetChance: 1, ArcanaRandomness: 10},
			{Name: "Calliope", DelayMinMS: 500, DelayMaxMS: 1100, UseKnownPairChance: 87, ForgetChance: 10, ArcanaRandomness: 20},
			{Name: "Thalia", DelayMinMS: 500, DelayMaxMS: 2000, UseKnownPairChance: 85, ForgetChance: 25, ArcanaRandomness: 25},
		},
		TelemetryHistogram: TelemetryHistogramConfig{
			TurnMax:      100,
			TurnNumBins:  6,
			PairsMax:     36,
			PairsNumBins: 6,
		},
		LogLevel: "info",
	}
}

// SlogLevel returns the slog.Level for the configured LogLevel string.
// Accepts "debug", "info", "warn", "error" (case-insensitive). Invalid values default to LevelInfo.
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Load reads configuration from an optional config file,
// then applies environment variable overrides. Fields not set
// in either source retain their default values.
// Config file path: CONFIG_PATH or CONFIG_FILE env, or "config.json" in the current directory.
func Load() *Config {
	cfg := Defaults()

	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = os.Getenv("CONFIG_FILE")
	}
	if configPath == "" {
		configPath = "config.json"
	}
	if f, err := os.Open(configPath); err == nil {
		if err := json.NewDecoder(f).Decode(cfg); err != nil {
			slog.Warn("failed to parse config file", "tag", "config", "path", configPath, "err", err)
		}
		f.Close()
	}

	// Environment variable overrides
	overrideInt(&cfg.BoardRows, "BOARD_ROWS")
	overrideInt(&cfg.BoardCols, "BOARD_COLS")
	overrideInt(&cfg.RevealDurationMS, "REVEAL_DURATION_MS")
	overrideInt(&cfg.PowerUps.Clairvoyance.RevealDurationMS, "POWERUP_CLAIRVOYANCE_REVEAL_MS")
	overrideInt(&cfg.MaxNameLength, "MAX_NAME_LENGTH")
	overrideInt(&cfg.WSPort, "WS_PORT")
	overrideInt(&cfg.MaxLatencyMS, "MAX_LATENCY_MS")
	overrideInt(&cfg.AIPairTimeoutSec, "AI_PAIR_TIMEOUT_SEC")
	overrideInt(&cfg.TurnLimitSec, "TURN_LIMIT_SEC")
	overrideInt(&cfg.TurnCountdownShowSec, "TURN_COUNTDOWN_SHOW_SEC")
	overrideInt(&cfg.ReconnectTimeoutSec, "RECONNECT_TIMEOUT_SEC")
	overrideString(&cfg.NeonAuthBaseURL, "NEON_AUTH_BASE_URL")
	overrideString(&cfg.DatabaseURL, "DATABASE_URL")
	if len(cfg.AIProfiles) > 0 {
		overrideString(&cfg.AIProfiles[0].Name, "AI_NAME")
		overrideInt(&cfg.AIProfiles[0].DelayMinMS, "AI_DELAY_MIN_MS")
		overrideInt(&cfg.AIProfiles[0].DelayMaxMS, "AI_DELAY_MAX_MS")
		overrideInt(&cfg.AIProfiles[0].UseKnownPairChance, "AI_USE_KNOWN_PAIR_CHANCE")
		overrideInt(&cfg.AIProfiles[0].ForgetChance, "AI_FORGET_CHANCE")
		overrideInt(&cfg.AIProfiles[0].ArcanaRandomness, "AI_ARCANA_RANDOMNESS")
	}
	overrideInt(&cfg.TelemetryHistogram.TurnMax, "TELEMETRY_TURN_MAX")
	overrideInt(&cfg.TelemetryHistogram.TurnNumBins, "TELEMETRY_TURN_NUM_BINS")
	overrideInt(&cfg.TelemetryHistogram.PairsMax, "TELEMETRY_PAIRS_MAX")
	overrideInt(&cfg.TelemetryHistogram.PairsNumBins, "TELEMETRY_PAIRS_NUM_BINS")
	overrideString(&cfg.LogLevel, "LOG_LEVEL")

	return cfg
}

func overrideInt(field *int, envKey string) {
	if val := os.Getenv(envKey); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			*field = n
		} else {
			slog.Warn("invalid config value", "tag", "config", "key", envKey, "value", val)
		}
	}
}

func overrideString(field *string, envKey string) {
	if val := os.Getenv(envKey); val != "" {
		*field = val
	}
}

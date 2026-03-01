package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	EloK           = 32
	InitialElo     = 1000
	aiUserIDPrefix = "ai:"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS game_history (
	id UUID PRIMARY KEY,
	played_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	player0_user_id TEXT NOT NULL,
	player1_user_id TEXT NOT NULL,
	player0_name TEXT NOT NULL,
	player1_name TEXT NOT NULL,
	player0_score INT NOT NULL,
	player1_score INT NOT NULL,
	winner_index SMALLINT,
	end_reason TEXT,
	player0_elo_before INT,
	player0_elo_after INT,
	player1_elo_before INT,
	player1_elo_after INT
);
CREATE INDEX IF NOT EXISTS idx_game_history_player0 ON game_history(player0_user_id);
CREATE INDEX IF NOT EXISTS idx_game_history_player1 ON game_history(player1_user_id);
CREATE TABLE IF NOT EXISTS player_ratings (
	user_id      TEXT PRIMARY KEY,
	display_name TEXT NOT NULL DEFAULT '',
	elo          INT  NOT NULL DEFAULT 1000,
	wins         INT  NOT NULL DEFAULT 0,
	losses       INT  NOT NULL DEFAULT 0,
	draws        INT  NOT NULL DEFAULT 0,
	updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_player_ratings_elo ON player_ratings(elo DESC);
CREATE TABLE IF NOT EXISTS match_arcana (
	match_id    UUID NOT NULL REFERENCES game_history(id),
	power_up_id TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_match_arcana_match_id ON match_arcana(match_id);
CREATE INDEX IF NOT EXISTS idx_match_arcana_power_up_id ON match_arcana(power_up_id);
CREATE TABLE IF NOT EXISTS turn (
	id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	match_id                UUID NOT NULL REFERENCES game_history(id),
	round                   INT NOT NULL,
	player_idx              SMALLINT NOT NULL,
	player_score_after_turn INT NOT NULL,
	opponent_score_after_turn INT NOT NULL,
	point_delta_player      INT NOT NULL,
	point_delta_opponent    INT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_turn_match_id ON turn(match_id);
CREATE INDEX IF NOT EXISTS idx_turn_match_round ON turn(match_id, round);
CREATE TABLE IF NOT EXISTS arcana_use (
	id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	match_id               UUID NOT NULL REFERENCES game_history(id),
	round                  INT NOT NULL,
	played_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
	player_idx             SMALLINT NOT NULL,
	power_up_id            TEXT NOT NULL,
	target_card_index      INT NOT NULL,
	player_score_before    INT NOT NULL,
	opponent_score_before  INT NOT NULL,
	pairs_matched_before   INT NOT NULL,
	point_delta_player     INT,
	point_delta_opponent   INT
);
CREATE INDEX IF NOT EXISTS idx_arcana_use_match_id ON arcana_use(match_id);
CREATE INDEX IF NOT EXISTS idx_arcana_use_power_up_id ON arcana_use(power_up_id);
CREATE INDEX IF NOT EXISTS idx_arcana_use_match_round ON arcana_use(match_id, round);
`

// alterGameHistoryAddEloColumns adds elo columns to game_history for existing DBs (no-op if already present).
const alterGameHistoryAddEloColumns = `
ALTER TABLE game_history ADD COLUMN IF NOT EXISTS player0_elo_before INT;
ALTER TABLE game_history ADD COLUMN IF NOT EXISTS player0_elo_after INT;
ALTER TABLE game_history ADD COLUMN IF NOT EXISTS player1_elo_before INT;
ALTER TABLE game_history ADD COLUMN IF NOT EXISTS player1_elo_after INT;
`

// alterGameHistoryDropGameID removes game_id column for existing DBs (no-op if already dropped).
const alterGameHistoryDropGameID = `
ALTER TABLE game_history DROP COLUMN IF EXISTS game_id;
`

// Store persists and retrieves game history.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore connects to Postgres and ensures the game_history table exists.
// If databaseURL is empty, NewStore returns (nil, nil) and no persistence occurs.
func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, nil
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, createTableSQL); err != nil {
		pool.Close()
		return nil, err
	}
	for _, q := range strings.Split(strings.TrimSpace(alterGameHistoryAddEloColumns), "\n") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := pool.Exec(ctx, q); err != nil {
			pool.Close()
			return nil, err
		}
	}
	for _, q := range strings.Split(strings.TrimSpace(alterGameHistoryDropGameID), "\n") {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, err := pool.Exec(ctx, q); err != nil {
			pool.Close()
			return nil, err
		}
	}
	slog.Info("connected to Postgres", "tag", "storage")
	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// computeEloUpdates returns new ratings (newR0, newR1) given current ratings and winnerIdx (0, 1, or -1 for draw).
func computeEloUpdates(r0, r1 int, winnerIdx int) (newR0, newR1 int) {
	var score0, score1 float64
	switch winnerIdx {
	case 0:
		score0, score1 = 1, 0
	case 1:
		score0, score1 = 0, 1
	default:
		score0, score1 = 0.5, 0.5
	}
	e0 := 1 / (1 + math.Pow(10, float64(r1-r0)/400))
	e1 := 1 - e0
	delta0 := EloK * (score0 - e0)
	delta1 := EloK * (score1 - e1)
	newR0 = r0 + int(math.Round(delta0))
	newR1 = r1 + int(math.Round(delta1))
	if newR0 < 0 {
		newR0 = 0
	}
	if newR1 < 0 {
		newR1 = 0
	}
	return newR0, newR1
}

// UpdateRatingsAfterGame updates ELO and W/L/D for both players after a completed game.
// Returns each player's elo before and after the game so the caller can store them in game_history.
func (s *Store) UpdateRatingsAfterGame(ctx context.Context, p0UserID, p1UserID, p0Name, p1Name string, winnerIdx int) (elo0Before, elo0After, elo1Before, elo1After int, err error) {
	if s == nil || s.pool == nil {
		return 0, 0, 0, 0, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	defer tx.Rollback(ctx)

	// Ensure both players have a row (default 1000 elo, 0 W/L/D)
	_, _ = tx.Exec(ctx, `INSERT INTO player_ratings (user_id, display_name, elo, wins, losses, draws) VALUES ($1, '', 1000, 0, 0, 0) ON CONFLICT (user_id) DO NOTHING`, p0UserID)
	_, _ = tx.Exec(ctx, `INSERT INTO player_ratings (user_id, display_name, elo, wins, losses, draws) VALUES ($1, '', 1000, 0, 0, 0) ON CONFLICT (user_id) DO NOTHING`, p1UserID)

	var r0, w0, l0, d0, r1, w1, l1, d1 int
	err = tx.QueryRow(ctx, `SELECT elo, wins, losses, draws FROM player_ratings WHERE user_id = $1`, p0UserID).Scan(&r0, &w0, &l0, &d0)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	err = tx.QueryRow(ctx, `SELECT elo, wins, losses, draws FROM player_ratings WHERE user_id = $1`, p1UserID).Scan(&r1, &w1, &l1, &d1)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	elo0Before, elo1Before = r0, r1
	newR0, newR1 := computeEloUpdates(r0, r1, winnerIdx)
	elo0After, elo1After = newR0, newR1

	switch winnerIdx {
	case 0:
		w0++
		l1++
	case 1:
		l0++
		w1++
	default:
		d0++
		d1++
	}

	_, err = tx.Exec(ctx, `UPDATE player_ratings SET display_name = $1, elo = $2, wins = $3, losses = $4, draws = $5, updated_at = now() WHERE user_id = $6`,
		p0Name, newR0, w0, l0, d0, p0UserID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	_, err = tx.Exec(ctx, `UPDATE player_ratings SET display_name = $1, elo = $2, wins = $3, losses = $4, draws = $5, updated_at = now() WHERE user_id = $6`,
		p1Name, newR1, w1, l1, d1, p1UserID)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, 0, 0, 0, err
	}
	return elo0Before, elo0After, elo1Before, elo1After, nil
}

// InsertGameResult records a finished game. matchID is the UUID of the match (used as game_history.id).
// winnerIndex is 0 or 1 (winner), or -1 for draw (stored as NULL).
// For end_reason "opponent_disconnected", winnerIndex is the player who stayed (winner); the abandoner is 1 - winnerIndex.
// Pass elo before/after for both "completed" and "opponent_disconnected"; pass nil only when ratings are not updated.
func (s *Store) InsertGameResult(ctx context.Context, matchID, player0UserID, player1UserID, player0Name, player1Name string, player0Score, player1Score int, winnerIndex int, endReason string, elo0Before, elo0After, elo1Before, elo1After *int) error {
	if s == nil || s.pool == nil {
		return nil
	}
	var winner *int
	if winnerIndex >= 0 && winnerIndex <= 1 {
		winner = &winnerIndex
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO game_history (id, player0_user_id, player1_user_id, player0_name, player1_name, player0_score, player1_score, winner_index, end_reason, player0_elo_before, player0_elo_after, player1_elo_before, player1_elo_after)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		matchID, player0UserID, player1UserID, player0Name, player1Name, player0Score, player1Score, winner, endReason, elo0Before, elo0After, elo1Before, elo1After)
	return err
}

// InsertMatchArcana inserts one row per arcana in the match (typically 6). Call after InsertGameResult for the same matchID.
func (s *Store) InsertMatchArcana(ctx context.Context, matchID string, powerUpIDs []string) error {
	if s == nil || s.pool == nil {
		return nil
	}
	for _, pid := range powerUpIDs {
		_, err := s.pool.Exec(ctx, `INSERT INTO match_arcana (match_id, power_up_id) VALUES ($1, $2)`, matchID, pid)
		if err != nil {
			return err
		}
	}
	return nil
}

// InsertTurn inserts a turn record for telemetry. Deltas are the score change for the player who had the turn and the opponent.
func (s *Store) InsertTurn(ctx context.Context, matchID string, round, playerIdx int, playerScoreAfter, opponentScoreAfter, deltaPlayer, deltaOpponent int) error {
	if s == nil || s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO turn (match_id, round, player_idx, player_score_after_turn, opponent_score_after_turn, point_delta_player, point_delta_opponent)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		matchID, round, playerIdx, playerScoreAfter, opponentScoreAfter, deltaPlayer, deltaOpponent)
	return err
}

// InsertArcanaUse records a power-up use. pointDeltaPlayer/pointDeltaOpponent are the score change
// from card use until end of turn (for balance telemetry).
func (s *Store) InsertArcanaUse(ctx context.Context, matchID string, round, playerIdx int, powerUpID string, targetCardIndex int, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore int, pointDeltaPlayer, pointDeltaOpponent int) error {
	if s == nil || s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO arcana_use (match_id, round, player_idx, power_up_id, target_card_index, player_score_before, opponent_score_before, pairs_matched_before, point_delta_player, point_delta_opponent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		matchID, round, playerIdx, powerUpID, targetCardIndex, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore, pointDeltaPlayer, pointDeltaOpponent)
	return err
}

// GameRecord is a single row returned for the history API.
// GameID is set to ID (match UUID) for client compatibility.
type GameRecord struct {
	ID               string  `json:"id"`
	PlayedAt         string  `json:"played_at"` // ISO8601
	GameID           string  `json:"game_id"`   // same as ID for backward compatibility
	Player0UserID    string  `json:"player0_user_id"`
	Player1UserID    string  `json:"player1_user_id"`
	Player0Name      string  `json:"player0_name"`
	Player1Name      string  `json:"player1_name"`
	Player0Score     int     `json:"player0_score"`
	Player1Score     int     `json:"player1_score"`
	WinnerIndex      *int    `json:"winner_index"` // 0 or 1 (winner), or null for draw. When end_reason is "opponent_disconnected", abandoner = 1 - winner_index.
	EndReason        string  `json:"end_reason"`
	YourIndex        *int    `json:"your_index"` // 0 or 1 for the requesting user; set by ListByUserID
	Player0EloBefore *int    `json:"player0_elo_before,omitempty"`
	Player0EloAfter  *int    `json:"player0_elo_after,omitempty"`
	Player1EloBefore *int    `json:"player1_elo_before,omitempty"`
	Player1EloAfter  *int    `json:"player1_elo_after,omitempty"`
}

// ListByUserID returns all games where the user participated, ordered by played_at DESC.
// Each record has your_index set to 0 or 1 so the client can show "You" vs opponent.
func (s *Store) ListByUserID(ctx context.Context, userID string) ([]GameRecord, error) {
	if s == nil || s.pool == nil {
		return []GameRecord{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, played_at, player0_user_id, player1_user_id, player0_name, player1_name, player0_score, player1_score, winner_index, COALESCE(end_reason,''),
			player0_elo_before, player0_elo_after, player1_elo_before, player1_elo_after
		FROM game_history
		WHERE player0_user_id = $1 OR player1_user_id = $1
		ORDER BY played_at DESC`,
		userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameRecord
	for rows.Next() {
		var r GameRecord
		var winnerIndex *int
		var playedAt time.Time
		var elo0Before, elo0After, elo1Before, elo1After *int
		if err := rows.Scan(&r.ID, &playedAt, &r.Player0UserID, &r.Player1UserID, &r.Player0Name, &r.Player1Name, &r.Player0Score, &r.Player1Score, &winnerIndex, &r.EndReason, &elo0Before, &elo0After, &elo1Before, &elo1After); err != nil {
			return nil, err
		}
		r.GameID = r.ID // backward compatibility for clients expecting game_id
		r.PlayedAt = playedAt.UTC().Format(time.RFC3339)
		r.WinnerIndex = winnerIndex
		r.Player0EloBefore = elo0Before
		r.Player0EloAfter = elo0After
		r.Player1EloBefore = elo1Before
		r.Player1EloAfter = elo1After
		yi := 0
		if r.Player1UserID == userID {
			yi = 1
		}
		r.YourIndex = &yi
		out = append(out, r)
	}
	return out, rows.Err()
}

// LeaderboardEntry is a single row for the leaderboard API.
type LeaderboardEntry struct {
	UserID        string `json:"user_id"`
	DisplayName   string `json:"display_name"`
	Elo           int    `json:"elo"`
	Wins          int    `json:"wins"`
	Losses        int    `json:"losses"`
	Draws         int    `json:"draws"`
	IsBot         bool   `json:"is_bot"`
	IsCurrentUser bool   `json:"is_current_user,omitempty"`
}

// ListLeaderboard returns entries ordered by elo DESC, with optional limit and offset.
func (s *Store) ListLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error) {
	if s == nil || s.pool == nil {
		return []LeaderboardEntry{}, nil
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(ctx, `
		SELECT user_id, display_name, elo, wins, losses, draws
		FROM player_ratings
		ORDER BY elo DESC
		LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LeaderboardEntry
	for rows.Next() {
		var e LeaderboardEntry
		if err := rows.Scan(&e.UserID, &e.DisplayName, &e.Elo, &e.Wins, &e.Losses, &e.Draws); err != nil {
			return nil, err
		}
		e.IsBot = strings.HasPrefix(e.UserID, aiUserIDPrefix)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetLeaderboardEntryByUserID returns one player's leaderboard entry by user_id, or (nil, nil) if not found.
func (s *Store) GetLeaderboardEntryByUserID(ctx context.Context, userID string) (*LeaderboardEntry, error) {
	if s == nil || s.pool == nil || userID == "" {
		return nil, nil
	}
	var e LeaderboardEntry
	err := s.pool.QueryRow(ctx, `
		SELECT user_id, display_name, elo, wins, losses, draws
		FROM player_ratings
		WHERE user_id = $1`,
		userID).Scan(&e.UserID, &e.DisplayName, &e.Elo, &e.Wins, &e.Losses, &e.Draws)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	e.IsBot = strings.HasPrefix(e.UserID, aiUserIDPrefix)
	return &e, nil
}

// GetUserRole returns the role for the user from neon_auth.user (e.g. "admin", "user"). Empty string if not found.
// Neon Auth manages the role in schema neon_auth, table "user", column role.
func (s *Store) GetUserRole(ctx context.Context, userID string) (string, error) {
	if s == nil || s.pool == nil || userID == "" {
		return "", nil
	}
	var role string
	// Table name "user" is reserved in PostgreSQL, so quote it.
	err := s.pool.QueryRow(ctx, `SELECT role FROM neon_auth."user" WHERE id = $1`, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

// TelemetryMetrics holds aggregated metrics for the admin telemetry dashboard.
type TelemetryMetrics struct {
	Players TelemetryPlayers  `json:"players"`
	Global  TelemetryGlobal   `json:"global"`
	ByCard  []TelemetryByCard `json:"by_card"`
	ByCombo []TelemetryByCombo `json:"by_combo"`
}

// TelemetryPlayers holds player-count and activity metrics.
type TelemetryPlayers struct {
	RegisteredCount   int `json:"registered_count"`
	ActiveLastWeek    int `json:"active_last_week"`
	TotalMatches      int `json:"total_matches"`
}

type TelemetryGlobal struct {
	TotalMatches            int      `json:"total_matches"`
	TotalTurns              int      `json:"total_turns"`
	AvgTurnsPerMatch        float64  `json:"avg_turns_per_match"`
	AvgNetPointSwingPerTurn float64  `json:"avg_net_point_swing_per_turn"`
	AvgNetPointSwingPerCard *float64 `json:"avg_net_point_swing_per_card,omitempty"`
	CardsPerTurnAvg         float64  `json:"cards_per_turn_avg"`
	CardsPerTurnMax         int      `json:"cards_per_turn_max"`
}

// TelemetryBinConfig defines histogram bin bounds for turn and pairs (used by GetTelemetryMetrics).
type TelemetryBinConfig struct {
	TurnMax      int
	TurnNumBins  int
	PairsMax     int
	PairsNumBins int
}

// TelemetryHistogramBucket is one bin in a histogram (label + count).
type TelemetryHistogramBucket struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

type TelemetryByCard struct {
	PowerUpID             string                    `json:"power_up_id"`
	TotalMatches          int                       `json:"total_matches"`
	WinsWithCard          int                       `json:"wins_with_card"`
	WinRatePct            float64                   `json:"win_rate_pct"`
	UseCount              int                       `json:"use_count"`
	AvgPointSwingPlayer   float64                   `json:"avg_point_swing_player"`
	AvgPointSwingOpponent float64                   `json:"avg_point_swing_opponent"`
	AvgPairsMatchedBefore float64                   `json:"avg_pairs_matched_before"`
	AvgTurnAtUse          float64                   `json:"avg_turn_at_use"`
	TurnHistogram         []TelemetryHistogramBucket `json:"turn_histogram"`
	PairsHistogram        []TelemetryHistogramBucket `json:"pairs_histogram"`
}

type TelemetryByCombo struct {
	ComboKey               string                    `json:"combo_key"`
	CardCount              int                       `json:"card_count"` // number of cards in the combo (2+)
	TotalMatches           int                       `json:"total_matches"`
	Wins                   int                       `json:"wins"`
	WinRatePct             float64                   `json:"win_rate_pct"`
	AvgPointSwingPlayer    float64                   `json:"avg_point_swing_player"`
	AvgPointSwingOpponent  float64                   `json:"avg_point_swing_opponent"`
	AvgTurnAtUse           float64                   `json:"avg_turn_at_use"`
	AvgPairsMatchedBefore  float64                   `json:"avg_pairs_matched_before"`
	TurnHistogram         []TelemetryHistogramBucket `json:"turn_histogram"`
	PairsHistogram        []TelemetryHistogramBucket `json:"pairs_histogram"`
}

// defaultTelemetryBinConfig is used when GetTelemetryMetrics is called with nil binConfig.
var defaultTelemetryBinConfig = TelemetryBinConfig{TurnMax: 100, TurnNumBins: 6, PairsMax: 36, PairsNumBins: 6}

// buildTurnHistogramLabels returns labels for turn bins: [0-step), [step-2*step), ..., "TurnMax+".
func buildTurnHistogramLabels(cfg TelemetryBinConfig) []string {
	if cfg.TurnNumBins < 2 || cfg.TurnMax <= 0 {
		return nil
	}
	step := cfg.TurnMax / (cfg.TurnNumBins - 1)
	labels := make([]string, 0, cfg.TurnNumBins)
	for i := 0; i < cfg.TurnNumBins-1; i++ {
		labels = append(labels, fmt.Sprintf("%d-%d", i*step, (i+1)*step))
	}
	labels = append(labels, fmt.Sprintf("%d+", cfg.TurnMax))
	return labels
}

// buildPairsHistogramLabels returns labels for pairs bins (equal width in [0, PairsMax]).
func buildPairsHistogramLabels(cfg TelemetryBinConfig) []string {
	if cfg.PairsNumBins <= 0 || cfg.PairsMax <= 0 {
		return nil
	}
	step := cfg.PairsMax / cfg.PairsNumBins
	if step < 1 {
		step = 1
	}
	labels := make([]string, 0, cfg.PairsNumBins)
	for i := 0; i < cfg.PairsNumBins; i++ {
		lo, hi := i*step, (i+1)*step
		if i == cfg.PairsNumBins-1 {
			hi = cfg.PairsMax
		}
		labels = append(labels, fmt.Sprintf("%d-%d", lo, hi))
	}
	return labels
}

// GetTelemetryMetrics returns aggregated metrics from game_history, match_arcana, turn, arcana_use.
func (s *Store) GetTelemetryMetrics(ctx context.Context, binConfig *TelemetryBinConfig) (*TelemetryMetrics, error) {
	if s == nil || s.pool == nil {
		return &TelemetryMetrics{}, nil
	}
	cfg := defaultTelemetryBinConfig
	if binConfig != nil {
		cfg = *binConfig
	}
	if cfg.TurnNumBins < 2 {
		cfg.TurnNumBins = 6
	}
	if cfg.TurnMax <= 0 {
		cfg.TurnMax = 100
	}
	if cfg.PairsNumBins <= 0 {
		cfg.PairsNumBins = 6
	}
	if cfg.PairsMax <= 0 {
		cfg.PairsMax = 36
	}
	out := &TelemetryMetrics{}

	// Players: registered count (players with a rating row)
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_ratings`).Scan(&out.Players.RegisteredCount); err != nil {
		return nil, err
	}
	// Players: active in last 7 days (distinct users who played a match)
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id) FROM (
			SELECT player0_user_id AS user_id FROM game_history WHERE played_at >= now() - interval '7 days'
			UNION
			SELECT player1_user_id FROM game_history WHERE played_at >= now() - interval '7 days'
		) t
	`).Scan(&out.Players.ActiveLastWeek); err != nil {
		return nil, err
	}
	// Global: total matches (also used for Players.TotalMatches)
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM game_history`).Scan(&out.Global.TotalMatches); err != nil {
		return nil, err
	}
	out.Players.TotalMatches = out.Global.TotalMatches

	// Global: total turns
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM turn`).Scan(&out.Global.TotalTurns); err != nil {
		return nil, err
	}
	// Global: avg turns per match
	if out.Global.TotalMatches > 0 {
		out.Global.AvgTurnsPerMatch = float64(out.Global.TotalTurns) / float64(out.Global.TotalMatches)
	}
	// Global: avg net point swing per turn (player gain + opponent loss = point_delta_player - point_delta_opponent)
	if out.Global.TotalTurns > 0 {
		if err := s.pool.QueryRow(ctx, `
			SELECT AVG((point_delta_player - point_delta_opponent))::float FROM turn
		`).Scan(&out.Global.AvgNetPointSwingPerTurn); err != nil {
			return nil, err
		}
	}
	// Global: avg net point swing per card (arcana uses only; only rows with deltas)
	var avgNetPerCard *float64
	if err := s.pool.QueryRow(ctx, `
		SELECT AVG((COALESCE(point_delta_player, 0) - COALESCE(point_delta_opponent, 0)))::float
		FROM arcana_use
		WHERE point_delta_player IS NOT NULL AND point_delta_opponent IS NOT NULL
	`).Scan(&avgNetPerCard); err != nil {
		return nil, err
	}
	out.Global.AvgNetPointSwingPerCard = avgNetPerCard
	// Global: cards per turn (count arcana_use per match_id, round)
	var cardsPerTurnAvg *float64
	var cardsPerTurnMax *int
	if err := s.pool.QueryRow(ctx, `
		SELECT AVG(cnt)::float, MAX(cnt)::int FROM (
			SELECT match_id, round, COUNT(*) AS cnt FROM arcana_use GROUP BY match_id, round
		) t
	`).Scan(&cardsPerTurnAvg, &cardsPerTurnMax); err != nil {
		return nil, err
	}
	if cardsPerTurnAvg != nil {
		out.Global.CardsPerTurnAvg = *cardsPerTurnAvg
	}
	if cardsPerTurnMax != nil {
		out.Global.CardsPerTurnMax = *cardsPerTurnMax
	}

	// By card: win rate and use stats per power_up_id
	// Win rate: matches where this power_up_id was in match_arcana and winner_index = player who had it (we consider "wins with card" as matches where the card was in the set and the match was won by either side; plan says "win rate when the card was in the game")
	// So: for each power_up_id, count matches that have this card in match_arcana; of those, count where winner_index is not null (we don't have "which player had the card" in match_arcana, so "wins with card" = matches where that card was in the set and someone won). Actually re-reading the plan: "Win rate por carta" = game_history JOIN match_arcana, winner_index + power_up_id. So it's: of all matches that included this card, what % were won (by either player). So total_matches = count of distinct match_id in match_arcana for this power_up_id; wins = count where that match has winner_index IS NOT NULL (a draw has winner_index null). So "wins with card" = number of matches (that have this card) that ended in a win (not draw).
	rows, err := s.pool.Query(ctx, `
		SELECT ma.power_up_id,
			COUNT(DISTINCT gh.id) AS total_matches,
			COUNT(DISTINCT gh.id) FILTER (WHERE gh.winner_index IS NOT NULL) AS wins_with_card
		FROM match_arcana ma
		JOIN game_history gh ON gh.id = ma.match_id
		GROUP BY ma.power_up_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cardWins := make(map[string]struct{ TotalMatches, WinsWithCard int })
	for rows.Next() {
		var powerUpID string
		var total, wins int
		if err := rows.Scan(&powerUpID, &total, &wins); err != nil {
			return nil, err
		}
		cardWins[powerUpID] = struct{ TotalMatches, WinsWithCard int }{total, wins}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Arcana use aggregates per power_up_id: use_count, avg point_delta, avg pairs_matched_before, avg_turn_at_use
	useRows, err := s.pool.Query(ctx, `
		SELECT power_up_id,
			COUNT(*) AS use_count,
			AVG(COALESCE(point_delta_player, 0))::float AS avg_delta_player,
			AVG(COALESCE(point_delta_opponent, 0))::float AS avg_delta_opponent,
			AVG(pairs_matched_before)::float AS avg_pairs_matched_before,
			AVG(round)::float AS avg_turn_at_use
		FROM arcana_use
		GROUP BY power_up_id
	`)
	if err != nil {
		return nil, err
	}
	defer useRows.Close()
	cardUse := make(map[string]struct {
		UseCount             int
		AvgDeltaPlayer       float64
		AvgDeltaOpponent     float64
		AvgPairsMatchedBefore float64
		AvgTurnAtUse         float64
	})
	for useRows.Next() {
		var pid string
		var useCount int
		var avgPlayer, avgOpp, avgPairs, avgTurn float64
		if err := useRows.Scan(&pid, &useCount, &avgPlayer, &avgOpp, &avgPairs, &avgTurn); err != nil {
			return nil, err
		}
		cardUse[pid] = struct {
			UseCount              int
			AvgDeltaPlayer        float64
			AvgDeltaOpponent      float64
			AvgPairsMatchedBefore float64
			AvgTurnAtUse          float64
		}{useCount, avgPlayer, avgOpp, avgPairs, avgTurn}
	}
	if err := useRows.Err(); err != nil {
		return nil, err
	}

	// Turn histogram: raw (power_up_id, round, cnt) then bin per card
	turnStep := cfg.TurnMax / (cfg.TurnNumBins - 1)
	if turnStep < 1 {
		turnStep = 1
	}
	turnLabels := buildTurnHistogramLabels(cfg)
	turnBinsByCard := make(map[string][]int)
	histRows, err := s.pool.Query(ctx, `
		SELECT power_up_id, round, COUNT(*) AS cnt
		FROM arcana_use
		GROUP BY power_up_id, round
		ORDER BY power_up_id, round
	`)
	if err != nil {
		return nil, err
	}
	for histRows.Next() {
		var pid string
		var round, cnt int
		if err := histRows.Scan(&pid, &round, &cnt); err != nil {
			histRows.Close()
			return nil, err
		}
		if turnBinsByCard[pid] == nil {
			turnBinsByCard[pid] = make([]int, len(turnLabels))
		}
		binIdx := cfg.TurnNumBins - 1
		if round < cfg.TurnMax {
			binIdx = round / turnStep
			if binIdx >= cfg.TurnNumBins-1 {
				binIdx = cfg.TurnNumBins - 2
			}
		}
		turnBinsByCard[pid][binIdx] += cnt
	}
	histRows.Close()
	if err := histRows.Err(); err != nil {
		return nil, err
	}

	// Pairs histogram: raw (power_up_id, pairs_matched_before) then bin per card
	pairsStep := cfg.PairsMax / cfg.PairsNumBins
	if pairsStep < 1 {
		pairsStep = 1
	}
	pairsLabels := buildPairsHistogramLabels(cfg)
	pairsBinsByCard := make(map[string][]int)
	pairsRows, err := s.pool.Query(ctx, `SELECT power_up_id, pairs_matched_before FROM arcana_use`)
	if err != nil {
		return nil, err
	}
	for pairsRows.Next() {
		var pid string
		var pairs int
		if err := pairsRows.Scan(&pid, &pairs); err != nil {
			pairsRows.Close()
			return nil, err
		}
		if pairsBinsByCard[pid] == nil {
			pairsBinsByCard[pid] = make([]int, len(pairsLabels))
		}
		binIdx := pairs / pairsStep
		if binIdx >= cfg.PairsNumBins {
			binIdx = cfg.PairsNumBins - 1
		}
		pairsBinsByCard[pid][binIdx]++
	}
	pairsRows.Close()
	if err := pairsRows.Err(); err != nil {
		return nil, err
	}

	// Build by_card list: all power_up_id that appear in match_arcana or arcana_use
	seenPowerUp := make(map[string]bool)
	for pid := range cardWins {
		seenPowerUp[pid] = true
	}
	for pid := range cardUse {
		seenPowerUp[pid] = true
	}
	for pid := range turnBinsByCard {
		seenPowerUp[pid] = true
	}
	for pid := range pairsBinsByCard {
		seenPowerUp[pid] = true
	}
	var powerUpIDs []string
	for pid := range seenPowerUp {
		powerUpIDs = append(powerUpIDs, pid)
	}
	sort.Strings(powerUpIDs) // deterministic order
	for _, pid := range powerUpIDs {
		w := cardWins[pid]
		u := cardUse[pid]
		winRatePct := 0.0
		if w.TotalMatches > 0 {
			winRatePct = 100.0 * float64(w.WinsWithCard) / float64(w.TotalMatches)
		}
		// Turn histogram with fixed labels (same order for all cards)
		turnHist := make([]TelemetryHistogramBucket, len(turnLabels))
		for i, label := range turnLabels {
			turnHist[i] = TelemetryHistogramBucket{Label: label, Count: 0}
			if bins := turnBinsByCard[pid]; i < len(bins) {
				turnHist[i].Count = bins[i]
			}
		}
		// Pairs histogram with fixed labels
		pairsHist := make([]TelemetryHistogramBucket, len(pairsLabels))
		for i, label := range pairsLabels {
			pairsHist[i] = TelemetryHistogramBucket{Label: label, Count: 0}
			if bins := pairsBinsByCard[pid]; i < len(bins) {
				pairsHist[i].Count = bins[i]
			}
		}
		out.ByCard = append(out.ByCard, TelemetryByCard{
			PowerUpID:             pid,
			TotalMatches:          w.TotalMatches,
			WinsWithCard:          w.WinsWithCard,
			WinRatePct:            winRatePct,
			UseCount:              u.UseCount,
			AvgPointSwingPlayer:   u.AvgDeltaPlayer,
			AvgPointSwingOpponent: u.AvgDeltaOpponent,
			AvgPairsMatchedBefore: u.AvgPairsMatchedBefore,
			AvgTurnAtUse:          u.AvgTurnAtUse,
			TurnHistogram:         turnHist,
			PairsHistogram:        pairsHist,
		})
	}

	// Combo "game stage at use": (combo_key, round, pairs_matched_before) per combo use for histograms.
	comboUseRows, err := s.pool.Query(ctx, `
		WITH turn_cards AS (
			SELECT match_id, round, player_idx, array_agg(DISTINCT power_up_id ORDER BY power_up_id) AS arr
			FROM arcana_use
			GROUP BY match_id, round, player_idx
		),
		turn_combos AS (
			SELECT match_id, round, player_idx,
				array_to_string(arr, ',') AS combo_key
			FROM turn_cards
			WHERE array_length(arr, 1) >= 2
		)
		SELECT tc.combo_key, tc.round, MIN(au.pairs_matched_before) AS pairs_matched_before
		FROM turn_combos tc
		JOIN arcana_use au ON au.match_id = tc.match_id AND au.round = tc.round AND au.player_idx = tc.player_idx
		GROUP BY tc.combo_key, tc.match_id, tc.round, tc.player_idx
	`)
	if err != nil {
		return nil, err
	}
	comboUseByKey := make(map[string][]struct{ round, pairs int })
	for comboUseRows.Next() {
		var comboKey string
		var round, pairs int
		if err := comboUseRows.Scan(&comboKey, &round, &pairs); err != nil {
			comboUseRows.Close()
			return nil, err
		}
		comboUseByKey[comboKey] = append(comboUseByKey[comboKey], struct{ round, pairs int }{round, pairs})
	}
	comboUseRows.Close()
	if err := comboUseRows.Err(); err != nil {
		return nil, err
	}

	// By combo: cards used together in the same turn (arcana_use grouped by match_id, round, player_idx).
	// Combo = sorted set of power_up_ids used in one turn; only combos with 2+ cards (synergy).
	// Wins = number of times that combo was used and the player who used it won the match.
	// Point swing = from first card of combo until end of turn (consistent with individual card metric).
	comboRows, err := s.pool.Query(ctx, `
		WITH turn_cards AS (
			SELECT match_id, round, player_idx, array_agg(DISTINCT power_up_id ORDER BY power_up_id) AS arr
			FROM arcana_use
			GROUP BY match_id, round, player_idx
		),
		turn_combos AS (
			SELECT match_id, round, player_idx,
				array_to_string(arr, ',') AS combo_key,
				array_length(arr, 1) AS card_count
			FROM turn_cards
			WHERE array_length(arr, 1) >= 2
		),
		turn_swing AS (
			SELECT tc.combo_key, tc.card_count, tc.match_id, tc.round, tc.player_idx,
				MIN(au.player_score_before) AS first_player_before,
				MIN(au.opponent_score_before) AS first_opponent_before
			FROM turn_combos tc
			JOIN arcana_use au ON au.match_id = tc.match_id AND au.round = tc.round AND au.player_idx = tc.player_idx
			GROUP BY tc.combo_key, tc.card_count, tc.match_id, tc.round, tc.player_idx
		)
		SELECT ts.combo_key, ts.card_count,
			COUNT(*) AS total_uses,
			COUNT(*) FILTER (WHERE gh.winner_index = ts.player_idx) AS wins,
			AVG(t.player_score_after_turn - ts.first_player_before)::float AS avg_point_swing_player,
			AVG(t.opponent_score_after_turn - ts.first_opponent_before)::float AS avg_point_swing_opponent
		FROM turn_swing ts
		JOIN turn t ON t.match_id = ts.match_id AND t.round = ts.round AND t.player_idx = ts.player_idx
		JOIN game_history gh ON gh.id = ts.match_id
		GROUP BY ts.combo_key, ts.card_count
		ORDER BY COUNT(*) DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer comboRows.Close()
	for comboRows.Next() {
		var comboKey string
		var cardCount, total, wins int
		var avgSwingPlayer, avgSwingOpponent float64
		if err := comboRows.Scan(&comboKey, &cardCount, &total, &wins, &avgSwingPlayer, &avgSwingOpponent); err != nil {
			return nil, err
		}
		winRatePct := 0.0
		if total > 0 {
			winRatePct = 100.0 * float64(wins) / float64(total)
		}
		uses := comboUseByKey[comboKey]
		var avgTurn, avgPairs float64
		comboTurnBins := make([]int, len(turnLabels))
		comboPairsBins := make([]int, len(pairsLabels))
		for _, u := range uses {
			avgTurn += float64(u.round)
			avgPairs += float64(u.pairs)
			binIdx := cfg.TurnNumBins - 1
			if u.round < cfg.TurnMax {
				binIdx = u.round / turnStep
				if binIdx >= cfg.TurnNumBins-1 {
					binIdx = cfg.TurnNumBins - 2
				}
			}
			comboTurnBins[binIdx]++
			pairsBinIdx := u.pairs / pairsStep
			if pairsBinIdx >= cfg.PairsNumBins {
				pairsBinIdx = cfg.PairsNumBins - 1
			}
			comboPairsBins[pairsBinIdx]++
		}
		if n := len(uses); n > 0 {
			avgTurn /= float64(n)
			avgPairs /= float64(n)
		}
		turnHist := make([]TelemetryHistogramBucket, len(turnLabels))
		for i, label := range turnLabels {
			turnHist[i] = TelemetryHistogramBucket{Label: label, Count: comboTurnBins[i]}
		}
		pairsHist := make([]TelemetryHistogramBucket, len(pairsLabels))
		for i, label := range pairsLabels {
			pairsHist[i] = TelemetryHistogramBucket{Label: label, Count: comboPairsBins[i]}
		}
		out.ByCombo = append(out.ByCombo, TelemetryByCombo{
			ComboKey:              comboKey,
			CardCount:             cardCount,
			TotalMatches:          total,
			Wins:                  wins,
			WinRatePct:            winRatePct,
			AvgPointSwingPlayer:   avgSwingPlayer,
			AvgPointSwingOpponent: avgSwingOpponent,
			AvgTurnAtUse:          avgTurn,
			AvgPairsMatchedBefore: avgPairs,
			TurnHistogram:         turnHist,
			PairsHistogram:        pairsHist,
		})
	}
	if err := comboRows.Err(); err != nil {
		return nil, err
	}
	if out.ByCombo == nil {
		out.ByCombo = []TelemetryByCombo{}
	}
	return out, nil
}

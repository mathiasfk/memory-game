package storage

import (
	"context"
	"errors"
	"log"
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
	log.Print("Game history storage: connected to Postgres")
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
// winnerIndex is 0 or 1, or -1 for draw (stored as NULL).
// For completed games pass elo before/after; for abandonos pass nil for all four.
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
	WinnerIndex      *int    `json:"winner_index"` // 0, 1, or null for draw
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

type TelemetryByCard struct {
	PowerUpID            string                 `json:"power_up_id"`
	TotalMatches         int                    `json:"total_matches"`
	WinsWithCard         int                    `json:"wins_with_card"`
	WinRatePct           float64                `json:"win_rate_pct"`
	UseCount             int                    `json:"use_count"`
	AvgPointSwingPlayer  float64                `json:"avg_point_swing_player"`
	AvgPointSwingOpponent float64                `json:"avg_point_swing_opponent"`
	AvgPairsMatchedBefore float64                `json:"avg_pairs_matched_before"`
	TurnHistogram        []TelemetryTurnBucket  `json:"turn_histogram"`
}

type TelemetryTurnBucket struct {
	Round int `json:"round"`
	Count int `json:"count"`
}

type TelemetryByCombo struct {
	ComboKey    string  `json:"combo_key"`
	TotalMatches int    `json:"total_matches"`
	Wins        int    `json:"wins"`
	WinRatePct  float64 `json:"win_rate_pct"`
}

// GetTelemetryMetrics returns aggregated metrics from game_history, match_arcana, turn, arcana_use.
func (s *Store) GetTelemetryMetrics(ctx context.Context) (*TelemetryMetrics, error) {
	if s == nil || s.pool == nil {
		return &TelemetryMetrics{}, nil
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

	// Arcana use aggregates per power_up_id: use_count, avg point_delta, avg pairs_matched_before, turn histogram
	useRows, err := s.pool.Query(ctx, `
		SELECT power_up_id,
			COUNT(*) AS use_count,
			AVG(COALESCE(point_delta_player, 0))::float AS avg_delta_player,
			AVG(COALESCE(point_delta_opponent, 0))::float AS avg_delta_opponent,
			AVG(pairs_matched_before)::float AS avg_pairs_matched_before
		FROM arcana_use
		GROUP BY power_up_id
	`)
	if err != nil {
		return nil, err
	}
	defer useRows.Close()
	cardUse := make(map[string]struct {
		UseCount             int
		AvgDeltaPlayer        float64
		AvgDeltaOpponent      float64
		AvgPairsMatchedBefore float64
	})
	for useRows.Next() {
		var pid string
		var useCount int
		var avgPlayer, avgOpp, avgPairs float64
		if err := useRows.Scan(&pid, &useCount, &avgPlayer, &avgOpp, &avgPairs); err != nil {
			return nil, err
		}
		cardUse[pid] = struct {
			UseCount             int
			AvgDeltaPlayer        float64
			AvgDeltaOpponent      float64
			AvgPairsMatchedBefore float64
		}{useCount, avgPlayer, avgOpp, avgPairs}
	}
	if err := useRows.Err(); err != nil {
		return nil, err
	}

	// Turn histogram per power_up_id: round -> count
	histRows, err := s.pool.Query(ctx, `
		SELECT power_up_id, round, COUNT(*) AS cnt
		FROM arcana_use
		GROUP BY power_up_id, round
		ORDER BY power_up_id, round
	`)
	if err != nil {
		return nil, err
	}
	defer histRows.Close()
	histogramByCard := make(map[string][]TelemetryTurnBucket)
	for histRows.Next() {
		var pid string
		var round, cnt int
		if err := histRows.Scan(&pid, &round, &cnt); err != nil {
			return nil, err
		}
		histogramByCard[pid] = append(histogramByCard[pid], TelemetryTurnBucket{Round: round, Count: cnt})
	}
	if err := histRows.Err(); err != nil {
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
	for pid := range histogramByCard {
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
		hist := histogramByCard[pid]
		if hist == nil {
			hist = []TelemetryTurnBucket{}
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
			TurnHistogram:         hist,
		})
	}

	// By combo: group matches by sorted set of 6 power_up_ids, then win rate
	comboRows, err := s.pool.Query(ctx, `
		WITH match_arcana_list AS (
			SELECT match_id, array_agg(power_up_id ORDER BY power_up_id) AS arr
			FROM match_arcana
			GROUP BY match_id
		),
		combo_keys AS (
			SELECT match_id, array_to_string(arr, ',') AS combo_key FROM match_arcana_list WHERE array_length(arr, 1) = 6
		)
		SELECT c.combo_key, COUNT(*), COUNT(*) FILTER (WHERE gh.winner_index IS NOT NULL)
		FROM combo_keys c
		JOIN game_history gh ON gh.id = c.match_id
		GROUP BY c.combo_key
		ORDER BY COUNT(*) DESC
		LIMIT 50
	`)
	if err != nil {
		return nil, err
	}
	defer comboRows.Close()
	for comboRows.Next() {
		var comboKey string
		var total, wins int
		if err := comboRows.Scan(&comboKey, &total, &wins); err != nil {
			return nil, err
		}
		winRatePct := 0.0
		if total > 0 {
			winRatePct = 100.0 * float64(wins) / float64(total)
		}
		out.ByCombo = append(out.ByCombo, TelemetryByCombo{
			ComboKey:     comboKey,
			TotalMatches: total,
			Wins:         wins,
			WinRatePct:   winRatePct,
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

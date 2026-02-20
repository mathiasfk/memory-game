package storage

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const createTableSQL = `
CREATE TABLE IF NOT EXISTS game_history (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	played_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	game_id TEXT,
	player0_user_id TEXT NOT NULL,
	player1_user_id TEXT NOT NULL,
	player0_name TEXT NOT NULL,
	player1_name TEXT NOT NULL,
	player0_score INT NOT NULL,
	player1_score INT NOT NULL,
	winner_index SMALLINT,
	end_reason TEXT
);
CREATE INDEX IF NOT EXISTS idx_game_history_player0 ON game_history(player0_user_id);
CREATE INDEX IF NOT EXISTS idx_game_history_player1 ON game_history(player1_user_id);
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
	log.Print("Game history storage: connected to Postgres")
	return &Store{pool: pool}, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// InsertGameResult records a finished game. winnerIndex is 0 or 1, or -1 for draw (stored as NULL).
func (s *Store) InsertGameResult(ctx context.Context, gameID, player0UserID, player1UserID, player0Name, player1Name string, player0Score, player1Score int, winnerIndex int, endReason string) error {
	if s == nil || s.pool == nil {
		return nil
	}
	var winner *int
	if winnerIndex >= 0 && winnerIndex <= 1 {
		winner = &winnerIndex
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO game_history (game_id, player0_user_id, player1_user_id, player0_name, player1_name, player0_score, player1_score, winner_index, end_reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		gameID, player0UserID, player1UserID, player0Name, player1Name, player0Score, player1Score, winner, endReason)
	return err
}

// GameRecord is a single row returned for the history API.
type GameRecord struct {
	ID            string  `json:"id"`
	PlayedAt      string  `json:"played_at"` // ISO8601
	GameID        string  `json:"game_id"`
	Player0UserID string  `json:"player0_user_id"`
	Player1UserID string  `json:"player1_user_id"`
	Player0Name   string  `json:"player0_name"`
	Player1Name   string  `json:"player1_name"`
	Player0Score  int     `json:"player0_score"`
	Player1Score  int     `json:"player1_score"`
	WinnerIndex   *int    `json:"winner_index"` // 0, 1, or null for draw
	EndReason     string  `json:"end_reason"`
	YourIndex     *int    `json:"your_index"` // 0 or 1 for the requesting user; set by ListByUserID
}

// ListByUserID returns all games where the user participated, ordered by played_at DESC.
// Each record has your_index set to 0 or 1 so the client can show "You" vs opponent.
func (s *Store) ListByUserID(ctx context.Context, userID string) ([]GameRecord, error) {
	if s == nil || s.pool == nil {
		return []GameRecord{}, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, played_at, COALESCE(game_id,''), player0_user_id, player1_user_id, player0_name, player1_name, player0_score, player1_score, winner_index, COALESCE(end_reason,'')
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
		if err := rows.Scan(&r.ID, &playedAt, &r.GameID, &r.Player0UserID, &r.Player1UserID, &r.Player0Name, &r.Player1Name, &r.Player0Score, &r.Player1Score, &winnerIndex, &r.EndReason); err != nil {
			return nil, err
		}
		r.PlayedAt = playedAt.UTC().Format(time.RFC3339)
		r.WinnerIndex = winnerIndex
		yi := 0
		if r.Player1UserID == userID {
			yi = 1
		}
		r.YourIndex = &yi
		out = append(out, r)
	}
	return out, rows.Err()
}

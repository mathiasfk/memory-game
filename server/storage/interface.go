package storage

import "context"

// HistoryStore abstracts persistence for game history, leaderboard, and telemetry.
// Implementations can be swapped for testing (mocks) or different backends (e.g. read replicas).
type HistoryStore interface {
	// Read
	ListByUserID(ctx context.Context, userID string) ([]GameRecord, error)
	ListLeaderboard(ctx context.Context, limit, offset int) ([]LeaderboardEntry, error)
	GetLeaderboardEntryByUserID(ctx context.Context, userID string) (*LeaderboardEntry, error)
	GetUserRole(ctx context.Context, userID string) (string, error)
	GetTelemetryMetrics(ctx context.Context) (*TelemetryMetrics, error)

	// Write
	InsertGameResult(ctx context.Context, matchID, player0UserID, player1UserID, player0Name, player1Name string, player0Score, player1Score int, winnerIndex int, endReason string, elo0Before, elo0After, elo1Before, elo1After *int) error
	UpdateRatingsAfterGame(ctx context.Context, p0UserID, p1UserID, p0Name, p1Name string, winnerIdx int) (elo0Before, elo0After, elo1Before, elo1After int, err error)
	InsertMatchArcana(ctx context.Context, matchID string, powerUpIDs []string) error
	InsertTurn(ctx context.Context, matchID string, round, playerIdx int, playerScoreAfter, opponentScoreAfter, deltaPlayer, deltaOpponent int) error
	InsertArcanaUse(ctx context.Context, matchID string, round, playerIdx int, powerUpID string, targetCardIndex int, playerScoreBefore, opponentScoreBefore, pairsMatchedBefore int, pointDeltaPlayer, pointDeltaOpponent int) error

	// Lifecycle
	Close()
}

// Ensure *Store implements HistoryStore at compile time.
var _ HistoryStore = (*Store)(nil)

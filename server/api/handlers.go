package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"memory-game-server/auth"
	"memory-game-server/config"
	"memory-game-server/storage"
)

const bearerPrefix = "Bearer "

// Handler holds dependencies for API handlers.
type Handler struct {
	Config               *config.Config
	HistoryStore         storage.HistoryStore
	FrontendErrorLogger  *slog.Logger
}

// NewHandler creates a new API handler with the given dependencies.
func NewHandler(cfg *config.Config, historyStore storage.HistoryStore, frontendErrorLogger *slog.Logger) *Handler {
	return &Handler{
		Config:              cfg,
		HistoryStore:        historyStore,
		FrontendErrorLogger: frontendErrorLogger,
	}
}

// CORS sets CORS headers on the response. Call before writing body.
func CORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// CORSWithPost sets CORS headers including POST. Use for endpoints that accept POST.
// When the request has an Origin header, that origin is reflected and credentials are
// allowed so that requests with credentials mode 'include' pass CORS.
func CORSWithPost(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// extractUserID validates the Authorization header and returns the user ID, or empty string on failure.
func (h *Handler) extractUserID(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return ""
	}
	token := strings.TrimSpace(authHeader[len(bearerPrefix):])
	claims, err := auth.ValidateNeonToken(h.Config.NeonAuthBaseURL, token)
	if err != nil {
		return ""
	}
	return auth.UserIDFromClaims(claims)
}

// HistoryResponse is the JSON structure for /api/history (paginated).
type HistoryResponse struct {
	Games   []storage.GameRecord `json:"games"`
	HasMore bool                 `json:"has_more"`
}

// History returns the game history for the authenticated user (paginated).
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	if CORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}

	limit := 10
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}
	offset := 0
	if s := r.URL.Query().Get("offset"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			offset = n
		}
	}

	resp := HistoryResponse{Games: []storage.GameRecord{}}
	if h.HistoryStore != nil {
		var err error
		resp.Games, resp.HasMore, err = h.HistoryStore.ListByUserIDPaginated(r.Context(), userID, limit, offset)
		if err != nil {
			slog.Error("ListByUserIDPaginated", "tag", "api", "err", err)
			http.Error(w, "failed to load history", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Encode history response", "tag", "api", "err", err)
	}
}

// LeaderboardResponse is the JSON structure for /api/leaderboard.
type LeaderboardResponse struct {
	Entries          []storage.LeaderboardEntry  `json:"entries"`
	CurrentUserEntry *storage.LeaderboardEntry  `json:"current_user_entry"`
}

// Leaderboard returns the global leaderboard with optional current user entry.
func (h *Handler) Leaderboard(w http.ResponseWriter, r *http.Request) {
	if CORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	entries := []storage.LeaderboardEntry{}
	if h.HistoryStore != nil {
		var err error
		entries, err = h.HistoryStore.ListLeaderboard(r.Context(), limit, offset)
		if err != nil {
			slog.Error("ListLeaderboard", "tag", "api", "err", err)
			http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
			return
		}
	}

	var currentUserEntry *storage.LeaderboardEntry
	authUserID := h.extractUserID(r)
	if authUserID != "" && h.HistoryStore != nil {
		cur, err := h.HistoryStore.GetLeaderboardEntryByUserID(r.Context(), authUserID)
		if err != nil {
			slog.Error("GetLeaderboardEntryByUserID", "tag", "api", "err", err)
		} else if cur != nil {
			inTop := false
			for i := range entries {
				if entries[i].UserID == authUserID {
					entries[i].IsCurrentUser = true
					inTop = true
					break
				}
			}
			if !inTop {
				cur.IsCurrentUser = true
				currentUserEntry = cur
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	resp := LeaderboardResponse{Entries: entries, CurrentUserEntry: currentUserEntry}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("Encode leaderboard response", "tag", "api", "err", err)
	}
}

// TelemetryMetrics returns aggregated telemetry metrics. Requires admin role (from neon_auth.user).
func (h *Handler) TelemetryMetrics(w http.ResponseWriter, r *http.Request) {
	if CORS(w, r) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := h.extractUserID(r)
	if userID == "" {
		http.Error(w, "authorization required", http.StatusUnauthorized)
		return
	}
	if h.HistoryStore == nil {
		http.Error(w, "telemetry not available", http.StatusServiceUnavailable)
		return
	}
	role, err := h.HistoryStore.GetUserRole(r.Context(), userID)
	if err != nil {
		slog.Error("GetUserRole", "tag", "api", "err", err)
		http.Error(w, "failed to verify role", http.StatusInternalServerError)
		return
	}
	if role != "admin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	matchType := r.URL.Query().Get("match_type")
	if matchType != "all" && matchType != "pvp" && matchType != "vs_ai" {
		matchType = "all"
	}
	timeRange := r.URL.Query().Get("time_range")
	if timeRange != "24h" && timeRange != "7d" && timeRange != "30d" {
		timeRange = "7d"
	}
	th := h.Config.TelemetryHistogram
	binConfig := &storage.TelemetryBinConfig{
		TurnMax:      th.TurnMax,
		TurnNumBins:  th.TurnNumBins,
		PairsMax:     th.PairsMax,
		PairsNumBins: th.PairsNumBins,
		MatchType:    matchType,
		TimeRange:    timeRange,
	}
	metrics, err := h.HistoryStore.GetTelemetryMetrics(r.Context(), binConfig)
	if err != nil {
		slog.Error("GetTelemetryMetrics", "tag", "api", "err", err)
		http.Error(w, "failed to load metrics", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		slog.Error("Encode telemetry response", "tag", "api", "err", err)
	}
}

// FrontendErrorPayload is the JSON body for POST /api/log/frontend-error.
type FrontendErrorPayload struct {
	Message        string `json:"message"`
	Stack          string `json:"stack,omitempty"`
	ComponentStack string `json:"componentStack,omitempty"`
	URL            string `json:"url,omitempty"`
	UserAgent      string `json:"userAgent,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
	UserID         string `json:"userId,omitempty"`
}

// FrontendError accepts POST with a JSON body and logs it with the frontend logger (error_source=frontend).
func (h *Handler) FrontendError(w http.ResponseWriter, r *http.Request) {
	if CORSWithPost(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload FrontendErrorPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if payload.Message == "" {
		http.Error(w, "message required", http.StatusBadRequest)
		return
	}

	h.FrontendErrorLogger.Error("frontend error",
		"tag", "api",
		"message", payload.Message,
		"stack", payload.Stack,
		"component_stack", payload.ComponentStack,
		"url", payload.URL,
		"user_agent", payload.UserAgent,
		"timestamp", payload.Timestamp,
		"user_id", payload.UserID,
	)

	w.WriteHeader(http.StatusNoContent)
}

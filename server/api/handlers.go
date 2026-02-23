package api

import (
	"encoding/json"
	"log"
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
	Config       *config.Config
	HistoryStore *storage.Store
}

// NewHandler creates a new API handler with the given dependencies.
func NewHandler(cfg *config.Config, historyStore *storage.Store) *Handler {
	return &Handler{
		Config:       cfg,
		HistoryStore: historyStore,
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

// History returns the game history for the authenticated user.
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

	list := []storage.GameRecord{}
	if h.HistoryStore != nil {
		var err error
		list, err = h.HistoryStore.ListByUserID(r.Context(), userID)
		if err != nil {
			log.Printf("ListByUserID: %v", err)
			http.Error(w, "failed to load history", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(list); err != nil {
		log.Printf("Encode history response: %v", err)
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
			log.Printf("ListLeaderboard: %v", err)
			http.Error(w, "failed to load leaderboard", http.StatusInternalServerError)
			return
		}
	}

	var currentUserEntry *storage.LeaderboardEntry
	authUserID := h.extractUserID(r)
	if authUserID != "" && h.HistoryStore != nil {
		cur, err := h.HistoryStore.GetLeaderboardEntryByUserID(r.Context(), authUserID)
		if err != nil {
			log.Printf("GetLeaderboardEntryByUserID: %v", err)
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
		log.Printf("Encode leaderboard response: %v", err)
	}
}

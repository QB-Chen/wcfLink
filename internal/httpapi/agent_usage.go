package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/agent"
)

func (s *Server) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	store := s.usageStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}

	period := strings.TrimSpace(r.URL.Query().Get("period"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))

	var since time.Time
	now := time.Now().UTC()
	switch period {
	case "monthly":
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	}

	if userID != "" {
		summary, err := store.GetUserUsageSince(r.Context(), userID, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, summary)
		return
	}

	items, err := store.GetAllUsageSince(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []agent.UsageSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "period": period, "since": since})
}

func (s *Server) usageStore() *agent.UsageStore {
	if s.agentInst == nil {
		return nil
	}
	return s.agentInst.UsageStore()
}

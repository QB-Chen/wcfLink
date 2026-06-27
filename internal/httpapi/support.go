package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/agent/support"
)

func (s *Server) supportStore() *support.Store {
	if s.agentInst == nil {
		return nil
	}
	return s.agentInst.SupportStore()
}

// --- Knowledge Base ---

func (s *Server) handleKBList(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	category := r.URL.Query().Get("category")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	articles, err := store.KBList(r.Context(), category, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": articles})
}

func (s *Server) handleKBGet(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/kb/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	article, err := store.KBGet(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "article not found"})
		return
	}
	writeJSON(w, http.StatusOK, article)
}

func (s *Server) handleKBCreate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	var art support.KBArticle
	if err := json.NewDecoder(r.Body).Decode(&art); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(art.Question) == "" || strings.TrimSpace(art.Answer) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "question and answer are required"})
		return
	}
	created, err := store.KBAdd(r.Context(), art)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleKBUpdate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/kb/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if err := store.KBUpdate(r.Context(), id, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKBDelete(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/kb/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	if err := store.KBDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleKBSearch(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "query parameter 'q' is required"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	articles, err := store.KBSearch(r.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": articles})
}

// --- Tickets ---

func (s *Server) handleTicketList(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	filters := make(map[string]string)
	for _, key := range []string{"status", "priority", "category", "customer_id", "assignee"} {
		if v := r.URL.Query().Get(key); v != "" {
			filters[key] = v
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tickets, err := store.TicketQuery(r.Context(), filters, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tickets})
}

func (s *Server) handleTicketGet(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/tickets/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	ticket, err := store.TicketGet(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "ticket not found"})
		return
	}
	writeJSON(w, http.StatusOK, ticket)
}

func (s *Server) handleTicketCreate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	var ticket support.Ticket
	if err := json.NewDecoder(r.Body).Decode(&ticket); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(ticket.Subject) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "subject is required"})
		return
	}
	created, err := store.TicketCreate(r.Context(), ticket)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleTicketUpdate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/tickets/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if err := store.TicketUpdate(r.Context(), id, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Orders ---

func (s *Server) handleOrderList(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	filters := make(map[string]string)
	for _, key := range []string{"customer_id", "status", "product"} {
		if v := r.URL.Query().Get(key); v != "" {
			filters[key] = v
		}
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	orders, err := store.OrderQuery(r.Context(), filters, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": orders})
}

func (s *Server) handleOrderGet(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/orders/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	order, err := store.OrderGet(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "order not found"})
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) handleOrderCreate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	var order support.Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(order.Product) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "product is required"})
		return
	}
	created, err := store.OrderCreate(r.Context(), order)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleOrderRefund(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	var req struct {
		Amount float64 `json:"amount"`
		Reason string  `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if req.Reason == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "reason is required"})
		return
	}
	if err := store.OrderRefund(r.Context(), id, req.Amount, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Support Profiles ---

func (s *Server) handleProfileList(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	profiles, err := store.ProfileList(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": profiles})
}

func (s *Server) handleProfileGet(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/profiles/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	profile, err := store.ProfileGet(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "profile not found"})
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) handleProfileCreate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	var profile support.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(profile.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name is required"})
		return
	}
	created, err := store.ProfileCreate(r.Context(), profile)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleProfileUpdate(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/profiles/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if err := store.ProfileUpdate(r.Context(), id, updates); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProfileDelete(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/support/profiles/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	if err := store.ProfileDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleProfileSetDefault(w http.ResponseWriter, r *http.Request) {
	store := s.supportStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "support module not enabled"})
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "id is required"})
		return
	}
	if err := store.ProfileSetDefault(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

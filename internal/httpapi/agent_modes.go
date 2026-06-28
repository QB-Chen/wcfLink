package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QB-Chen/wcfLink/internal/agent"
	"github.com/QB-Chen/wcfLink/internal/agent/modes"
)

func (s *Server) handleCustomModeList(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	items, err := store.ListModes(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []agent.CustomMode{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleCustomModeCreate(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	var cm agent.CustomMode
	if err := json.NewDecoder(r.Body).Decode(&cm); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	cm.Slug = strings.TrimSpace(cm.Slug)
	cm.Name = strings.TrimSpace(cm.Name)
	if cm.Slug == "" || cm.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "slug and name are required"})
		return
	}
	if _, builtin := modes.Get(cm.Slug); builtin {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "slug conflicts with built-in mode: " + cm.Slug})
		return
	}
	reservedCmds := map[string]bool{
		"reset": true, "mode": true, "help": true,
		"support-setup": true, "support-profiles": true, "support-use": true,
	}
	if reservedCmds[cm.Slug] {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "slug conflicts with reserved command: " + cm.Slug})
		return
	}
	created, err := store.CreateMode(r.Context(), cm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleCustomModeGet(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/modes/custom/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode id is required"})
		return
	}
	cm, err := store.GetMode(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "custom mode not found"})
		return
	}
	writeJSON(w, http.StatusOK, cm)
}

func (s *Server) handleCustomModeUpdate(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/modes/custom/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode id is required"})
		return
	}
	var cm agent.CustomMode
	if err := json.NewDecoder(r.Body).Decode(&cm); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	cm.ID = id
	if err := store.UpdateMode(r.Context(), cm); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleCustomModeDelete(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/modes/custom/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "mode id is required"})
		return
	}
	if err := store.DeleteMode(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// LLM Provider handlers

func (s *Server) handleLLMProviderList(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	items, err := store.ListProviders(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if items == nil {
		items = []agent.LLMProvider{}
	}
	// Redact API keys in list response.
	for i := range items {
		items[i].APIKey = redactKey(items[i].APIKey)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleLLMProviderCreate(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	var p agent.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.BaseURL) == "" || strings.TrimSpace(p.Model) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "name, base_url and model are required"})
		return
	}
	created, err := store.CreateProvider(r.Context(), p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	created.APIKey = redactKey(created.APIKey)
	writeJSON(w, http.StatusCreated, created)
}

func (s *Server) handleLLMProviderGet(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/llm-providers/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "provider id is required"})
		return
	}
	p, err := store.GetProvider(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "llm provider not found"})
		return
	}
	p.APIKey = redactKey(p.APIKey)
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleLLMProviderUpdate(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/llm-providers/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "provider id is required"})
		return
	}
	var p agent.LLMProvider
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	p.ID = id
	// Preserve existing API key if the submitted value is a redacted placeholder.
	if strings.Contains(p.APIKey, "***") {
		existing, err := store.GetProvider(r.Context(), id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "llm provider not found"})
			return
		}
		p.APIKey = existing.APIKey
	}
	if err := store.UpdateProvider(r.Context(), p); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if s.agentInst != nil {
		s.agentInst.InvalidateProviderCache(id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleLLMProviderDelete(w http.ResponseWriter, r *http.Request) {
	store := s.customModeStore()
	if store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/agent/llm-providers/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "provider id is required"})
		return
	}
	if err := store.DeleteProvider(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if s.agentInst != nil {
		s.agentInst.InvalidateProviderCache(id)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) customModeStore() *agent.CustomModeStore {
	if s.agentInst == nil {
		return nil
	}
	return s.agentInst.CustomModeStore()
}

func redactKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "***" + key[len(key)-4:]
}

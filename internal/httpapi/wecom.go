package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/lich0821/wcfLink/internal/model"
)

func (s *Server) handleWeComCallback(w http.ResponseWriter, r *http.Request) {
	if s.wecomHandler == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "wecom callback not configured"})
		return
	}
	w.Header().Del("Content-Type")
	s.wecomHandler.ServeHTTP(w, r)
}

func (s *Server) handleWeComAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.wecomSvc.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
}

func (s *Server) handleWeComAddAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CorpID         string `json:"corp_id"`
		CorpSecret     string `json:"corp_secret"`
		AgentID        int    `json:"agent_id"`
		CallbackToken  string `json:"callback_token"`
		CallbackAESKey string `json:"callback_aes_key"`
		AutoReply      bool   `json:"auto_reply"`
		WebhookURL     string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.CorpID) == "" || strings.TrimSpace(req.CorpSecret) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id and corp_secret are required"})
		return
	}
	account := model.WeComAccount{
		CorpID:         req.CorpID,
		CorpSecret:     req.CorpSecret,
		AgentID:        req.AgentID,
		CallbackToken:  req.CallbackToken,
		CallbackAESKey: req.CallbackAESKey,
		Enabled:        true,
		AutoReply:      req.AutoReply,
		WebhookURL:     req.WebhookURL,
	}
	if err := s.wecomSvc.AddAccount(r.Context(), account); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"restart_needed": true,
		"note":           "restart required for callback handler to pick up new account",
	})
}

func (s *Server) handleWeComRemoveAccount(w http.ResponseWriter, r *http.Request) {
	corpID := strings.TrimSpace(r.URL.Query().Get("corp_id"))
	agentIDStr := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if corpID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id is required"})
		return
	}
	agentID, _ := strconv.Atoi(agentIDStr)
	if err := s.wecomSvc.RemoveAccount(r.Context(), corpID, agentID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleWeComEvents(w http.ResponseWriter, r *http.Request) {
	afterID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after_id")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	events, err := s.wecomSvc.ListEvents(r.Context(), afterID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleWeComSendText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CorpID     string `json:"corp_id"`
		CorpSecret string `json:"corp_secret"`
		AgentID    int    `json:"agent_id"`
		ToUser     string `json:"to_user"`
		Text       string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.CorpID) == "" || strings.TrimSpace(req.CorpSecret) == "" ||
		strings.TrimSpace(req.ToUser) == "" || strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id, corp_secret, to_user and text are required"})
		return
	}
	if err := s.wecomSvc.SendText(r.Context(), req.CorpID, req.CorpSecret, req.AgentID, req.ToUser, req.Text); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleWeComGetUser(w http.ResponseWriter, r *http.Request) {
	corpID := strings.TrimSpace(r.URL.Query().Get("corp_id"))
	corpSecret := strings.TrimSpace(r.URL.Query().Get("corp_secret"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if corpID == "" || corpSecret == "" || userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id, corp_secret and user_id are required"})
		return
	}
	user, err := s.wecomSvc.GetUser(r.Context(), corpID, corpSecret, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) handleWeComListUsers(w http.ResponseWriter, r *http.Request) {
	corpID := strings.TrimSpace(r.URL.Query().Get("corp_id"))
	corpSecret := strings.TrimSpace(r.URL.Query().Get("corp_secret"))
	deptIDStr := strings.TrimSpace(r.URL.Query().Get("department_id"))
	if corpID == "" || corpSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id and corp_secret are required"})
		return
	}
	deptID, _ := strconv.Atoi(deptIDStr)
	if deptID == 0 {
		deptID = 1
	}
	users, err := s.wecomSvc.ListDepartmentUsers(r.Context(), corpID, corpSecret, deptID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

func (s *Server) handleWeComListDepartments(w http.ResponseWriter, r *http.Request) {
	corpID := strings.TrimSpace(r.URL.Query().Get("corp_id"))
	corpSecret := strings.TrimSpace(r.URL.Query().Get("corp_secret"))
	if corpID == "" || corpSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id and corp_secret are required"})
		return
	}
	departments, err := s.wecomSvc.ListDepartments(r.Context(), corpID, corpSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": departments})
}

func (s *Server) handleWeComGetGroupChat(w http.ResponseWriter, r *http.Request) {
	corpID := strings.TrimSpace(r.URL.Query().Get("corp_id"))
	corpSecret := strings.TrimSpace(r.URL.Query().Get("corp_secret"))
	chatID := strings.TrimSpace(r.URL.Query().Get("chat_id"))
	if corpID == "" || corpSecret == "" || chatID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "corp_id, corp_secret and chat_id are required"})
		return
	}
	chat, err := s.wecomSvc.GetGroupChat(r.Context(), corpID, corpSecret, chatID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, chat)
}

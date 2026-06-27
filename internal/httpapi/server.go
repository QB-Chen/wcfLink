package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/agent"
	"github.com/QB-Chen/wcfLink/internal/ilink"
	"github.com/QB-Chen/wcfLink/internal/model"
	"github.com/QB-Chen/wcfLink/internal/wecom"
	coreversion "github.com/QB-Chen/wcfLink/version"
)

type Service interface {
	StartLogin(ctx context.Context, baseURL string) (model.LoginSession, error)
	GetLoginSession(ctx context.Context, sessionID string) (model.LoginSession, error)
	GetLoginStatus(ctx context.Context, sessionID string) (model.LoginSession, error)
	ListAccounts(ctx context.Context) ([]model.Account, error)
	ListEvents(ctx context.Context, afterID int64, limit int) ([]model.Event, error)
	ListLogs(ctx context.Context, afterID int64, limit int) ([]model.LogEntry, error)
	GetSettings(ctx context.Context) (model.Settings, error)
	UpdateSettings(ctx context.Context, settings model.Settings) (model.Settings, error)
	SendText(ctx context.Context, accountID, toUserID, text, contextToken string) error
	SendMedia(ctx context.Context, accountID, toUserID, mediaType, filePath, text, contextToken string) error
	GetConfig(ctx context.Context, accountID, ilinkUserID, contextToken string) (ilink.GetConfigResponse, error)
	SendTyping(ctx context.Context, accountID, ilinkUserID, typingTicket string, status int) error
	NotifyStart(ctx context.Context, accountID string) (ilink.NotifyResponse, error)
	NotifyStop(ctx context.Context, accountID string) (ilink.NotifyResponse, error)
}

type WeComService interface {
	SendText(ctx context.Context, corpID, corpSecret string, agentID int, toUser, text string) error
	ListAccounts(ctx context.Context) ([]model.WeComAccount, error)
	ListEvents(ctx context.Context, afterID int64, limit int) ([]model.WeComEvent, error)
	AddAccount(ctx context.Context, account model.WeComAccount) error
	RemoveAccount(ctx context.Context, corpID string, agentID int) error
	GetUser(ctx context.Context, corpID, corpSecret, userID string) (wecom.UserInfo, error)
	ListDepartmentUsers(ctx context.Context, corpID, corpSecret string, departmentID int) ([]wecom.UserInfo, error)
	ListDepartments(ctx context.Context, corpID, corpSecret string) ([]wecom.DepartmentInfo, error)
	GetGroupChat(ctx context.Context, corpID, corpSecret, chatID string) (wecom.GroupChatInfo, error)
}

type WeComCallbackHandler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type Server struct {
	logger       *slog.Logger
	service      Service
	wecomSvc     WeComService
	wecomHandler WeComCallbackHandler
	agentInst    *agent.Agent
}

func NewServer(service Service, logger *slog.Logger, wecomSvc WeComService, wecomHandler WeComCallbackHandler, agentInst *agent.Agent) *Server {
	return &Server{
		logger:       logger,
		service:      service,
		wecomSvc:     wecomSvc,
		wecomHandler: wecomHandler,
		agentInst:    agentInst,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", s.handleLive)
	mux.HandleFunc("GET /health/ready", s.handleReady)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("POST /api/accounts/login/start", s.handleLoginStart)
	mux.HandleFunc("GET /api/accounts/login/status", s.handleLoginStatus)
	mux.HandleFunc("GET /api/accounts/login/qr", s.handleLoginQR)
	mux.HandleFunc("GET /api/accounts", s.handleAccounts)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("POST /api/settings", s.handleUpdateSettings)
	mux.HandleFunc("POST /api/messages/send-text", s.handleSendText)
	mux.HandleFunc("POST /api/messages/send-media", s.handleSendMedia)
	mux.HandleFunc("POST /api/bot/getconfig", s.handleGetConfig)
	mux.HandleFunc("POST /api/bot/sendtyping", s.handleSendTyping)
	mux.HandleFunc("POST /api/bot/notifystart", s.handleNotifyStart)
	mux.HandleFunc("POST /api/bot/notifystop", s.handleNotifyStop)

	mux.HandleFunc("GET /api/wecom/accounts", s.handleWeComAccounts)
	mux.HandleFunc("POST /api/wecom/accounts", s.handleWeComAddAccount)
	mux.HandleFunc("DELETE /api/wecom/accounts", s.handleWeComRemoveAccount)
	mux.HandleFunc("GET /api/wecom/events", s.handleWeComEvents)
	mux.HandleFunc("POST /api/wecom/messages/send-text", s.handleWeComSendText)
	mux.HandleFunc("GET /api/wecom/contacts/user", s.handleWeComGetUser)
	mux.HandleFunc("GET /api/wecom/contacts/users", s.handleWeComListUsers)
	mux.HandleFunc("GET /api/wecom/contacts/departments", s.handleWeComListDepartments)
	mux.HandleFunc("GET /api/wecom/contacts/groupchat", s.handleWeComGetGroupChat)
	if s.wecomHandler != nil {
		mux.HandleFunc("GET /api/wecom/callback", s.handleWeComCallback)
		mux.HandleFunc("POST /api/wecom/callback", s.handleWeComCallback)
	}

	mux.HandleFunc("GET /api/agent/status", s.handleAgentStatus)
	mux.HandleFunc("GET /api/agent/conversations", s.handleAgentListConversations)
	mux.HandleFunc("GET /api/agent/conversations/", s.handleAgentGetConversation)
	mux.HandleFunc("DELETE /api/agent/conversations/", s.handleAgentDeleteConversation)
	mux.HandleFunc("POST /api/agent/chat", s.handleAgentChat)

	return withJSONContentType(mux)
}

func (s *Server) handleAgentStatus(w http.ResponseWriter, _ *http.Request) {
	enabled := s.agentInst != nil
	writeJSON(w, http.StatusOK, map[string]any{
		"enabled": enabled,
	})
}

func (s *Server) handleAgentListConversations(w http.ResponseWriter, r *http.Request) {
	if s.agentInst == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	convs, err := s.agentInst.ConversationManager().ListConversations(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": convs})
}

func (s *Server) handleAgentGetConversation(w http.ResponseWriter, r *http.Request) {
	if s.agentInst == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	convID := strings.TrimPrefix(r.URL.Path, "/api/agent/conversations/")
	if convID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "conversation id is required"})
		return
	}
	mgr := s.agentInst.ConversationManager()
	conv, err := mgr.GetConversation(r.Context(), convID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "conversation not found"})
		return
	}
	msgs, err := mgr.GetMessages(r.Context(), convID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": conv,
		"messages":     msgs,
	})
}

func (s *Server) handleAgentDeleteConversation(w http.ResponseWriter, r *http.Request) {
	if s.agentInst == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	convID := strings.TrimPrefix(r.URL.Path, "/api/agent/conversations/")
	if convID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "conversation id is required"})
		return
	}
	if err := s.agentInst.ConversationManager().DeleteConversation(r.Context(), convID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAgentChat(w http.ResponseWriter, r *http.Request) {
	if s.agentInst == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "agent is not enabled"})
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
		Mode      string `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "message is required"})
		return
	}
	if req.SessionID == "" {
		req.SessionID = "http-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}

	session := agent.SessionKey{
		ChannelType: "http",
		UserID:      req.SessionID,
		GroupID:     "",
	}

	var replies []string
	mockSender := &httpChatSender{replies: &replies}

	tempAgent := agent.NewWithSender(s.agentInst, mockSender)
	if err := tempAgent.HandleMessage(r.Context(), session, req.Message); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	reply := strings.Join(replies, "\n")
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": req.SessionID,
		"reply":      reply,
	})
}

type httpChatSender struct {
	replies *[]string
}

func (s *httpChatSender) SendText(_ context.Context, _ agent.SessionKey, text string) error {
	*s.replies = append(*s.replies, text)
	return nil
}

func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"timestamp": time.Now().UTC(),
		"version":   coreversion.Current(),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"timestamp": time.Now().UTC(),
		"version":   coreversion.Current(),
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, coreversion.Current())
}

func (s *Server) handleLoginStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, http.ErrBodyNotAllowed) && err.Error() != "EOF" {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	session, err := s.service.StartLogin(r.Context(), strings.TrimSpace(req.BaseURL))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleLoginStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "session_id is required"})
		return
	}
	session, err := s.service.GetLoginStatus(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "login session not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleLoginQR(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "session_id is required", http.StatusBadRequest)
		return
	}
	session, err := s.service.GetLoginSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "login session not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if session.QRCodeURL == "" {
		http.Error(w, "qr code url is empty", http.StatusBadRequest)
		return
	}
	png, err := GenerateQRCodePNG(session.QRCodeURL)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.service.ListAccounts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	afterID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after_id")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	events, err := s.service.ListEvents(r.Context(), afterID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	afterID, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("after_id")), 10, 64)
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := s.service.ListLogs(r.Context(), afterID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.service.GetSettings(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings model.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(settings.ListenAddr) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "listen_addr is required"})
		return
	}
	out, err := s.service.UpdateSettings(r.Context(), settings)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":       out,
		"restart_needed": true,
	})
}

func (s *Server) handleSendText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID    string `json:"account_id"`
		ToUserID     string `json:"to_user_id"`
		Text         string `json:"text"`
		ContextToken string `json:"context_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.ToUserID) == "" || strings.TrimSpace(req.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id, to_user_id and text are required"})
		return
	}
	if err := s.service.SendText(r.Context(), req.AccountID, req.ToUserID, req.Text, req.ContextToken); err != nil {
		if isContextTokenMissingError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleSendMedia(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID    string `json:"account_id"`
		ToUserID     string `json:"to_user_id"`
		Type         string `json:"type"`
		FilePath     string `json:"file_path"`
		Text         string `json:"text"`
		ContextToken string `json:"context_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.ToUserID) == "" || strings.TrimSpace(req.FilePath) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id, to_user_id and file_path are required"})
		return
	}
	if err := s.service.SendMedia(r.Context(), req.AccountID, req.ToUserID, req.Type, req.FilePath, req.Text, req.ContextToken); err != nil {
		if isContextTokenMissingError(err) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID    string `json:"account_id"`
		ILinkUserID  string `json:"ilink_user_id"`
		ContextToken string `json:"context_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.ILinkUserID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id and ilink_user_id are required"})
		return
	}
	resp, err := s.service.GetConfig(r.Context(), req.AccountID, req.ILinkUserID, req.ContextToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleSendTyping(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID    string `json:"account_id"`
		ILinkUserID  string `json:"ilink_user_id"`
		TypingTicket string `json:"typing_ticket"`
		Status       int    `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" || strings.TrimSpace(req.ILinkUserID) == "" || strings.TrimSpace(req.TypingTicket) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id, ilink_user_id and typing_ticket are required"})
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	if err := s.service.SendTyping(r.Context(), req.AccountID, req.ILinkUserID, req.TypingTicket, req.Status); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleNotifyStart(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id is required"})
		return
	}
	resp, err := s.service.NotifyStart(r.Context(), req.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleNotifyStop(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string `json:"account_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid json body"})
		return
	}
	if strings.TrimSpace(req.AccountID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "account_id is required"})
		return
	}
	resp, err := s.service.NotifyStop(r.Context(), req.AccountID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func isContextTokenMissingError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "context token not found")
}

func withJSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{"error": err.Error()})
}

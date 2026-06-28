package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QB-Chen/wcfLink/internal/agent"
	"github.com/QB-Chen/wcfLink/internal/agent/support"
	"github.com/QB-Chen/wcfLink/internal/config"
	"github.com/QB-Chen/wcfLink/internal/httpapi"
	"github.com/QB-Chen/wcfLink/internal/ilink"
	"github.com/QB-Chen/wcfLink/internal/llm"
	"github.com/QB-Chen/wcfLink/internal/model"
	"github.com/QB-Chen/wcfLink/internal/netguard"
	"github.com/QB-Chen/wcfLink/internal/store"
	"github.com/QB-Chen/wcfLink/internal/wecom"
	"github.com/QB-Chen/wcfLink/internal/worker"
)

type App struct {
	cfg          config.Config
	logger       *slog.Logger
	store        *store.Store
	client       *ilink.Client
	pollers      *worker.PollerManager
	server       *http.Server
	runtime      *runtimeState
	svc          *service
	wecomSvc     *wecomService
	wecomHandler *wecom.CallbackHandler
	agentInst    *agent.Agent
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := validateListenSecurity(cfg.ListenAddr, cfg.APIToken); err != nil {
		return nil, err
	}

	st, err := store.New(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	client := ilink.NewClient(cfg.ChannelVersion, cfg.PollTimeout+10*time.Second)
	runtime := newRuntimeState(st, cfg)
	svc := &service{
		cfg:     cfg,
		logger:  logger,
		store:   st,
		client:  client,
		runtime: runtime,
	}
	pollers := worker.NewPollerManager(st, client, logger, svc.HandleInboundMessage)
	svc.pollers = pollers

	wecomClient := wecom.NewClient(cfg.WeComAPIBaseURL)
	wecomSvc := newWeComService(wecomServiceConfig{
		WeComWebhookURL: cfg.WeComWebhookURL,
	}, logger, st, wecomClient)

	var wecomHandler *wecom.CallbackHandler
	wecomAccounts := buildWeComAccounts(cfg, ctx, st)
	if len(wecomAccounts) > 0 {
		var err2 error
		wecomHandler, err2 = wecom.NewCallbackHandler(wecomAccounts, logger, wecomSvc.HandleInbound)
		if err2 != nil {
			return nil, fmt.Errorf("init wecom callback handler: %w", err2)
		}
	}

	var agentInst *agent.Agent
	if cfg.AgentEnabled && cfg.LLMAPIKey != "" {
		llmClient := llm.NewClient(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
		convMgr := agent.NewConversationManager(st.DB())
		if err := convMgr.Migrate(ctx); err != nil {
			return nil, fmt.Errorf("migrate conversation tables: %w", err)
		}

		sender := agent.NewMultiChannelSender(
			svc,
			wecomSvc,
			func(session agent.SessionKey) (agent.ILinkSessionInfo, error) {
				accounts, err := st.ListAccounts(ctx)
				if err != nil || len(accounts) == 0 {
					return agent.ILinkSessionInfo{}, fmt.Errorf("no ilink accounts available")
				}
				account := accounts[0]
				peerCtx, err := st.GetPeerContext(ctx, account.AccountID, session.UserID)
				if err != nil {
					return agent.ILinkSessionInfo{}, fmt.Errorf("no context token for user %s", session.UserID)
				}
				return agent.ILinkSessionInfo{
					AccountID:    account.AccountID,
					ContextToken: peerCtx.ContextToken,
				}, nil
			},
			func(session agent.SessionKey) (agent.WeComSessionInfo, error) {
				wecomAccounts, err := st.ListWeComAccounts(ctx)
				if err != nil || len(wecomAccounts) == 0 {
					return agent.WeComSessionInfo{}, fmt.Errorf("no wecom accounts available")
				}
				acct := wecomAccounts[0]
				return agent.WeComSessionInfo{
					CorpID:     acct.CorpID,
					CorpSecret: acct.CorpSecret,
					AgentID:    acct.AgentID,
				}, nil
			},
		)

		supportStore := support.NewStore(st.DB())
		if err := supportStore.Migrate(ctx); err != nil {
			return nil, fmt.Errorf("migrate support tables: %w", err)
		}

		temp := cfg.LLMTemperature
		agentInst = agent.New(llmClient, convMgr, sender, logger, agent.AgentConfig{
			DefaultMode:     cfg.AgentDefaultMode,
			MaxIterations:   cfg.AgentMaxIterations,
			SessionTTL:      cfg.AgentSessionTTL,
			Temperature:     &temp,
			MaxTokens:       cfg.LLMMaxTokens,
			FetchMaxContent: cfg.FetchMaxContent,
		}, supportStore)
		logger.Info("agent enabled", "mode", cfg.AgentDefaultMode, "model", cfg.LLMModel)
	}

	svc.agent = agentInst
	wecomSvc.agentInst = agentInst

	api := httpapi.NewServer(svc, logger, wecomSvc, wecomHandler, agentInst, cfg.APIToken)

	return &App{
		cfg:          cfg,
		logger:       logger,
		store:        st,
		client:       client,
		pollers:      pollers,
		runtime:      runtime,
		svc:          svc,
		wecomSvc:     wecomSvc,
		wecomHandler: wecomHandler,
		agentInst:    agentInst,
		server: &http.Server{
			Addr:              cfg.ListenAddr,
			Handler:           api.Handler(),
			ReadHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if err := a.StartBackground(ctx); err != nil {
		return err
	}

	<-ctx.Done()
	a.logger.Info("shutdown requested")
	return a.Shutdown()
}

func (a *App) StartBackground(ctx context.Context) error {
	if err := a.store.Ping(ctx); err != nil {
		return err
	}
	if err := a.pollers.StartEnabledAccounts(ctx); err != nil {
		return err
	}

	go func() {
		a.logger.Info("http server listening", "addr", a.cfg.ListenAddr)
		if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("http server stopped with error", "err", err)
		}
	}()
	return nil
}

func (a *App) Shutdown() error {
	a.pollers.StopAll()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = a.server.Shutdown(shutdownCtx)
	return a.store.Close()
}

func (a *App) StartLogin(ctx context.Context, baseURL string) (model.LoginSession, error) {
	return a.svc.StartLogin(ctx, baseURL)
}

func (a *App) GetLoginStatus(ctx context.Context, sessionID string) (model.LoginSession, error) {
	return a.svc.GetLoginStatus(ctx, sessionID)
}

func (a *App) GetLoginSession(ctx context.Context, sessionID string) (model.LoginSession, error) {
	return a.svc.GetLoginSession(ctx, sessionID)
}

func (a *App) ListAccounts(ctx context.Context) ([]model.Account, error) {
	return a.svc.ListAccounts(ctx)
}

func (a *App) ListEvents(ctx context.Context, afterID int64, limit int) ([]model.Event, error) {
	return a.svc.ListEvents(ctx, afterID, limit)
}

func (a *App) GetSettings(ctx context.Context) (model.Settings, error) {
	return a.svc.GetSettings(ctx)
}

func (a *App) UpdateSettings(ctx context.Context, settings model.Settings) (model.Settings, error) {
	return a.svc.UpdateSettings(ctx, settings)
}

func (a *App) SendText(ctx context.Context, accountID, toUserID, text, contextToken string) error {
	return a.svc.SendText(ctx, accountID, toUserID, text, contextToken)
}

func (a *App) SendMedia(ctx context.Context, accountID, toUserID, mediaType, filePath, text, contextToken string) error {
	return a.svc.SendMedia(ctx, accountID, toUserID, mediaType, filePath, text, contextToken)
}

func (a *App) LogoutAccount(ctx context.Context, accountID string) error {
	return a.svc.LogoutAccount(ctx, accountID)
}

func (a *App) GetConfig(ctx context.Context, accountID, ilinkUserID, contextToken string) (ilink.GetConfigResponse, error) {
	return a.svc.GetConfig(ctx, accountID, ilinkUserID, contextToken)
}

func (a *App) SendTyping(ctx context.Context, accountID, ilinkUserID, typingTicket string, status int) error {
	return a.svc.SendTyping(ctx, accountID, ilinkUserID, typingTicket, status)
}

func (a *App) NotifyStart(ctx context.Context, accountID string) (ilink.NotifyResponse, error) {
	return a.svc.NotifyStart(ctx, accountID)
}

func (a *App) NotifyStop(ctx context.Context, accountID string) (ilink.NotifyResponse, error) {
	return a.svc.NotifyStop(ctx, accountID)
}

func (a *App) WeComSendText(ctx context.Context, corpID, corpSecret string, agentID int, toUser, text string) error {
	return a.wecomSvc.SendText(ctx, corpID, corpSecret, agentID, toUser, text)
}

func (a *App) WeComSendMedia(ctx context.Context, corpID, corpSecret string, agentID int, toUser, mediaType, filePath string, fileData []byte) error {
	return a.wecomSvc.SendMedia(ctx, corpID, corpSecret, agentID, toUser, mediaType, filePath, fileData)
}

func (a *App) WeComListAccounts(ctx context.Context) ([]model.WeComAccount, error) {
	return a.wecomSvc.ListAccounts(ctx)
}

func (a *App) WeComListEvents(ctx context.Context, afterID int64, limit int) ([]model.WeComEvent, error) {
	return a.wecomSvc.ListEvents(ctx, afterID, limit)
}

func (a *App) WeComAddAccount(ctx context.Context, account model.WeComAccount) error {
	return a.wecomSvc.AddAccount(ctx, account)
}

func (a *App) WeComRemoveAccount(ctx context.Context, corpID string, agentID int) error {
	return a.wecomSvc.RemoveAccount(ctx, corpID, agentID)
}

func (a *App) WeComGetUser(ctx context.Context, corpID, corpSecret, userID string) (wecom.UserInfo, error) {
	return a.wecomSvc.GetUser(ctx, corpID, corpSecret, userID)
}

func (a *App) WeComListDepartmentUsers(ctx context.Context, corpID, corpSecret string, departmentID int) ([]wecom.UserInfo, error) {
	return a.wecomSvc.ListDepartmentUsers(ctx, corpID, corpSecret, departmentID)
}

func (a *App) WeComListDepartments(ctx context.Context, corpID, corpSecret string) ([]wecom.DepartmentInfo, error) {
	return a.wecomSvc.ListDepartments(ctx, corpID, corpSecret)
}

func (a *App) WeComGetGroupChat(ctx context.Context, corpID, corpSecret, chatID string) (wecom.GroupChatInfo, error) {
	return a.wecomSvc.GetGroupChat(ctx, corpID, corpSecret, chatID)
}

func buildWeComAccounts(cfg config.Config, ctx context.Context, st *store.Store) []wecom.AccountConfig {
	var accounts []wecom.AccountConfig

	if cfg.WeComCorpID != "" && cfg.WeComCorpSecret != "" {
		accounts = append(accounts, wecom.AccountConfig{
			CorpID:         cfg.WeComCorpID,
			CorpSecret:     cfg.WeComCorpSecret,
			AgentID:        cfg.WeComAgentID,
			CallbackToken:  cfg.WeComCallbackToken,
			CallbackAESKey: cfg.WeComCallbackAESKey,
		})
		_ = st.UpsertWeComAccount(ctx, model.WeComAccount{
			CorpID:         cfg.WeComCorpID,
			CorpSecret:     cfg.WeComCorpSecret,
			AgentID:        cfg.WeComAgentID,
			CallbackToken:  cfg.WeComCallbackToken,
			CallbackAESKey: cfg.WeComCallbackAESKey,
			Enabled:        true,
			AutoReply:      cfg.WeComAutoReply,
			WebhookURL:     cfg.WeComWebhookURL,
		})
	}

	stored, err := st.ListWeComAccounts(ctx)
	if err == nil {
		for _, s := range stored {
			if !s.Enabled {
				continue
			}
			if s.CorpID == cfg.WeComCorpID && s.AgentID == cfg.WeComAgentID {
				continue
			}
			accounts = append(accounts, wecom.AccountConfig{
				CorpID:         s.CorpID,
				CorpSecret:     s.CorpSecret,
				AgentID:        s.AgentID,
				CallbackToken:  s.CallbackToken,
				CallbackAESKey: s.CallbackAESKey,
			})
		}
	}

	return accounts
}

type service struct {
	cfg     config.Config
	logger  *slog.Logger
	store   *store.Store
	client  *ilink.Client
	pollers *worker.PollerManager
	runtime *runtimeState
	agent   *agent.Agent
}

func (s *service) StartLogin(ctx context.Context, baseURL string) (model.LoginSession, error) {
	if baseURL == "" {
		baseURL = s.cfg.DefaultBaseURL
	}
	qr, err := s.client.FetchQRCode(ctx, baseURL)
	if err != nil {
		return model.LoginSession{}, err
	}
	now := time.Now().UTC()
	session := model.LoginSession{
		SessionID: fmt.Sprintf("login_%d", now.UnixNano()),
		BaseURL:   baseURL,
		QRCode:    qr.QRCode,
		QRCodeURL: qr.QRCodeURL,
		Status:    "wait",
		StartedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateLoginSession(ctx, session); err != nil {
		return model.LoginSession{}, err
	}
	return session, nil
}

func (s *service) GetLoginStatus(ctx context.Context, sessionID string) (model.LoginSession, error) {
	session, err := s.store.GetLoginSession(ctx, sessionID)
	if err != nil {
		return model.LoginSession{}, err
	}
	if session.Status == "confirmed" {
		return session, nil
	}
	status, err := s.client.FetchQRCodeStatus(ctx, session.BaseURL, session.QRCode)
	if err != nil {
		_ = s.store.UpdateLoginSessionStatus(context.Background(), sessionID, session.Status, err.Error())
		return model.LoginSession{}, err
	}
	if status.Status == "" {
		return session, nil
	}
	if status.Status == "confirmed" {
		if err := s.store.CompleteLoginSession(ctx, sessionID, status); err != nil {
			return model.LoginSession{}, err
		}
		account, err := s.store.GetAccount(ctx, status.AccountID)
		if err == nil {
			s.pollers.StartAccount(context.Background(), account)
		}
		return s.store.GetLoginSession(ctx, sessionID)
	}
	if err := s.store.UpdateLoginSessionStatus(ctx, sessionID, status.Status, ""); err != nil {
		return model.LoginSession{}, err
	}
	return s.store.GetLoginSession(ctx, sessionID)
}

func (s *service) GetLoginSession(ctx context.Context, sessionID string) (model.LoginSession, error) {
	return s.store.GetLoginSession(ctx, sessionID)
}

func (s *service) ListAccounts(ctx context.Context) ([]model.Account, error) {
	return s.store.ListAccounts(ctx)
}

func (s *service) ListEvents(ctx context.Context, afterID int64, limit int) ([]model.Event, error) {
	return s.store.ListEvents(ctx, afterID, limit)
}

func (s *service) ListLogs(ctx context.Context, afterID int64, limit int) ([]model.LogEntry, error) {
	return s.store.ListLogs(ctx, afterID, limit)
}

func (s *service) GetSettings(ctx context.Context) (model.Settings, error) {
	_ = ctx
	return s.runtime.Settings(), nil
}

func (s *service) UpdateSettings(ctx context.Context, settings model.Settings) (model.Settings, error) {
	if err := validateListenSecurity(settings.ListenAddr, s.cfg.APIToken); err != nil {
		return model.Settings{}, err
	}
	if err := config.SaveFileSettings(s.cfg.SettingsPath, config.FileSettings{
		ListenAddr: settings.ListenAddr,
		WebhookURL: settings.WebhookURL,
	}); err != nil {
		return model.Settings{}, err
	}
	if err := s.runtime.UpdateSettings(ctx, settings); err != nil {
		return model.Settings{}, err
	}
	return s.runtime.Settings(), nil
}

func (s *service) LogoutAccount(ctx context.Context, accountID string) error {
	s.pollers.StopAccount(accountID)
	if err := s.store.DeleteAccount(ctx, accountID); err != nil {
		return err
	}
	_ = s.store.AddLog(context.Background(), "INFO", "account disconnected locally", "account", fmt.Sprintf(`{"account_id":%q}`, accountID))
	return nil
}

func (s *service) SendText(ctx context.Context, accountID, toUserID, text, contextToken string) error {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	contextToken, err = s.resolveContextToken(ctx, accountID, toUserID, contextToken)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contextToken) == "" {
		return errors.New("context token not found for this user; current text sending only supports replying to users who have already sent a message")
	}
	if err := s.client.SendTextMessage(ctx, account.BaseURL, account.Token, toUserID, text, contextToken); err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "outbound send failed", "message", fmt.Sprintf(`{"account_id":%q,"to_user_id":%q,"err":%q}`, accountID, toUserID, err.Error()))
		return err
	}
	raw := fmt.Sprintf(`{"to_user_id":%q,"text":%q,"context_token":%q}`, toUserID, text, contextToken)
	if err := s.store.CreateOutboundEvent(ctx, accountID, "text", toUserID, contextToken, text, "", "", "", raw); err != nil {
		return err
	}
	_ = s.store.AddLog(context.Background(), "INFO", "outbound text sent", "message", raw)
	return nil
}

func (s *service) SendMedia(ctx context.Context, accountID, toUserID, mediaType, filePath, text, contextToken string) error {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	contextToken, err = s.resolveContextToken(ctx, accountID, toUserID, contextToken)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contextToken) == "" {
		return errors.New("context token not found for this user; media sending only supports replying to users who have already sent a message")
	}
	filePath, err = validateLocalMediaPath(filePath, s.cfg.MediaSendRoot, s.cfg.MaxMediaBytes)
	if err != nil {
		return err
	}

	normalizedType, uploadType, err := normalizeMediaSendType(mediaType, filePath)
	if err != nil {
		return err
	}
	uploaded, err := s.client.UploadLocalMedia(ctx, s.cfg.CDNBaseURL, account.BaseURL, account.Token, toUserID, filePath, uploadType)
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "media upload failed", "message", fmt.Sprintf(`{"account_id":%q,"to_user_id":%q,"file_path":%q,"err":%q}`, accountID, toUserID, filePath, err.Error()))
		return err
	}

	fileName := filepath.Base(filePath)
	switch normalizedType {
	case "image":
		err = s.client.SendImageMessage(ctx, account.BaseURL, account.Token, toUserID, contextToken, text, uploaded)
	case "video":
		err = s.client.SendVideoMessage(ctx, account.BaseURL, account.Token, toUserID, contextToken, text, uploaded)
	case "file":
		err = s.client.SendFileMessage(ctx, account.BaseURL, account.Token, toUserID, contextToken, text, fileName, uploaded)
	case "voice":
		err = s.client.SendVoiceMessage(ctx, account.BaseURL, account.Token, toUserID, contextToken, text, detectVoiceEncodeType(filePath), uploaded)
	default:
		err = fmt.Errorf("unsupported media type %q", normalizedType)
	}
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "outbound media send failed", "message", fmt.Sprintf(`{"account_id":%q,"to_user_id":%q,"file_path":%q,"media_type":%q,"err":%q}`, accountID, toUserID, filePath, normalizedType, err.Error()))
		if isAudioFilePath(filePath) {
			return fmt.Errorf("%s发送失败：\n%s", strings.TrimPrefix(strings.ToLower(filepath.Ext(filePath)), "."), err.Error())
		}
		return err
	}

	mimeType := detectOutboundMIME(normalizedType, filePath)
	raw := fmt.Sprintf(`{"to_user_id":%q,"file_path":%q,"media_type":%q,"text":%q,"context_token":%q}`, toUserID, filePath, normalizedType, text, contextToken)
	if err := s.store.CreateOutboundEvent(ctx, accountID, normalizedType, toUserID, contextToken, text, filePath, fileName, mimeType, raw); err != nil {
		return err
	}
	_ = s.store.AddLog(context.Background(), "INFO", "outbound media sent", "message", raw)
	return nil
}

func (s *service) GetConfig(ctx context.Context, accountID, ilinkUserID, contextToken string) (ilink.GetConfigResponse, error) {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return ilink.GetConfigResponse{}, err
	}
	return s.client.GetConfig(ctx, account.BaseURL, account.Token, ilinkUserID, contextToken)
}

func (s *service) SendTyping(ctx context.Context, accountID, ilinkUserID, typingTicket string, status int) error {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return err
	}
	return s.client.SendTyping(ctx, account.BaseURL, account.Token, ilinkUserID, typingTicket, status)
}

func (s *service) NotifyStart(ctx context.Context, accountID string) (ilink.NotifyResponse, error) {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return ilink.NotifyResponse{}, err
	}
	return s.client.NotifyStart(ctx, account.BaseURL, account.Token)
}

func (s *service) NotifyStop(ctx context.Context, accountID string) (ilink.NotifyResponse, error) {
	account, err := s.store.GetAccount(ctx, accountID)
	if err != nil {
		return ilink.NotifyResponse{}, err
	}
	return s.client.NotifyStop(ctx, account.BaseURL, account.Token)
}

func (s *service) HandleInboundMessage(ctx context.Context, account model.Account, msg ilink.WeixinMessage) error {
	mediaPath, mediaFileName, mediaMimeType := "", "", ""
	if mediaItem, ok := firstInboundMediaItem(msg); ok {
		mediaBytes, suggestedFileName, mimeType, err := s.client.DownloadMessageMedia(ctx, s.cfg.CDNBaseURL, mediaItem)
		if err != nil {
			s.logger.Warn("download inbound media failed", "account_id", account.AccountID, "message_id", msg.MessageID, "err", err)
			_ = s.store.AddLog(context.Background(), "ERROR", "download inbound media failed", "media", fmt.Sprintf(`{"account_id":%q,"message_id":%d,"err":%q}`, account.AccountID, msg.MessageID, err.Error()))
		} else {
			mediaPath, mediaFileName, mediaMimeType, err = s.saveInboundMedia(account.AccountID, msg.MessageID, msg.FromUserID, suggestedFileName, mimeType, mediaBytes)
			if err != nil {
				s.logger.Warn("persist inbound media failed", "account_id", account.AccountID, "message_id", msg.MessageID, "err", err)
				_ = s.store.AddLog(context.Background(), "ERROR", "persist inbound media failed", "media", fmt.Sprintf(`{"account_id":%q,"message_id":%d,"err":%q}`, account.AccountID, msg.MessageID, err.Error()))
				mediaPath, mediaFileName, mediaMimeType = "", "", ""
			}
		}
	}

	if err := s.store.SaveInboundMessage(ctx, account.AccountID, msg, mediaPath, mediaFileName, mediaMimeType); err != nil {
		return err
	}

	payload, err := json.Marshal(map[string]any{
		"account_id":      account.AccountID,
		"base_url":        account.BaseURL,
		"event_type":      ilink.DetectEventType(msg),
		"body_text":       ilink.ExtractBodyText(msg),
		"from_user_id":    msg.FromUserID,
		"to_user_id":      msg.ToUserID,
		"group_id":        msg.GroupID,
		"session_id":      msg.SessionID,
		"message_id":      msg.MessageID,
		"context_token":   msg.ContextToken,
		"media_path":      mediaPath,
		"media_file_name": mediaFileName,
		"media_mime_type": mediaMimeType,
		"raw_message":     msg,
		"received_at":     time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	if s.agent != nil {
		bodyText := ilink.ExtractBodyText(msg)
		if bodyText != "" {
			session := agent.SessionKey{
				ChannelType: "ilink",
				UserID:      msg.FromUserID,
				GroupID:     msg.GroupID,
			}
			go func() {
				if err := s.agent.HandleMessage(context.Background(), session, bodyText); err != nil {
					s.logger.Error("agent handle ilink message failed", "err", err)
				}
			}()
			_ = s.store.AddLog(ctx, "INFO", "inbound message routed to agent", "agent", string(payload))
			return nil
		}
	}

	settings := s.runtime.Settings()
	if settings.WebhookURL == "" {
		text := "[non-text]"
		for _, item := range msg.ItemList {
			if item.Type == ilink.MessageItemTypeText && item.TextItem != nil && item.TextItem.Text != "" {
				text = item.TextItem.Text
				break
			}
			if item.Type == ilink.MessageItemTypeVoice && item.VoiceItem != nil && item.VoiceItem.Text != "" {
				text = item.VoiceItem.Text
				break
			}
		}
		return s.store.AddLog(ctx, "INFO", fmt.Sprintf("inbound message from %s: %s", msg.FromUserID, text), "inbound", string(payload))
	}

	go s.deliverWebhook(settings.WebhookURL, payload)
	return s.store.AddLog(ctx, "INFO", "inbound message queued for webhook", "webhook", string(payload))
}

func (s *service) resolveContextToken(ctx context.Context, accountID, toUserID, contextToken string) (string, error) {
	if strings.TrimSpace(contextToken) != "" {
		return contextToken, nil
	}
	peerCtx, err := s.store.GetPeerContext(ctx, accountID, toUserID)
	if err == nil {
		return peerCtx.ContextToken, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return "", err
}

func (s *service) saveInboundMedia(accountID string, messageID int64, fromUserID, fileName, mimeType string, data []byte) (string, string, string, error) {
	if err := os.MkdirAll(s.cfg.MediaDir, 0o700); err != nil {
		return "", "", "", err
	}
	now := time.Now()
	dir := filepath.Join(s.cfg.MediaDir, sanitizePathSegment(accountID), now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", "", err
	}

	safeName := sanitizeFileName(fileName)
	if safeName == "" {
		safeName = "media"
	}
	base := strings.TrimSuffix(safeName, filepath.Ext(safeName))
	ext := filepath.Ext(safeName)
	if ext == "" {
		ext = extensionForMIME(mimeType)
	}
	if ext == "" {
		ext = ".bin"
	}

	prefix := fmt.Sprintf("%d", messageID)
	if messageID == 0 {
		prefix = fmt.Sprintf("%d", now.UnixNano())
	}
	if fromUserID != "" {
		prefix += "_" + sanitizePathSegment(fromUserID)
	}
	finalName := prefix + "_" + base + ext
	fullPath := filepath.Join(dir, finalName)
	if err := os.WriteFile(fullPath, data, 0o600); err != nil {
		return "", "", "", err
	}
	return fullPath, finalName, mimeType, nil
}

func normalizeMediaSendType(mediaType, filePath string) (string, int, error) {
	value := strings.ToLower(strings.TrimSpace(mediaType))
	if value == "" {
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp":
			value = "image"
		case ".mp4", ".mov", ".m4v":
			value = "video"
		default:
			value = "file"
		}
	}
	switch value {
	case "image":
		return value, ilink.UploadMediaTypeImage, nil
	case "video":
		return value, ilink.UploadMediaTypeVideo, nil
	case "file":
		return value, ilink.UploadMediaTypeFile, nil
	case "voice":
		return value, ilink.UploadMediaTypeVoice, nil
	default:
		return "", 0, fmt.Errorf("unsupported media type %q", mediaType)
	}
}

func firstInboundMediaItem(msg ilink.WeixinMessage) (ilink.MessageItem, bool) {
	for _, item := range msg.ItemList {
		switch item.Type {
		case ilink.MessageItemTypeImage, ilink.MessageItemTypeVoice, ilink.MessageItemTypeFile, ilink.MessageItemTypeVideo:
			return item, true
		}
	}
	return ilink.MessageItem{}, false
}

func detectOutboundMIME(mediaType, filePath string) string {
	switch mediaType {
	case "image":
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".png":
			return "image/png"
		case ".gif":
			return "image/gif"
		default:
			return "image/jpeg"
		}
	case "video":
		return "video/mp4"
	case "voice":
		switch strings.ToLower(filepath.Ext(filePath)) {
		case ".amr":
			return "audio/amr"
		case ".mp3":
			return "audio/mpeg"
		case ".ogg":
			return "audio/ogg"
		default:
			return "audio/silk"
		}
	default:
		return "application/octet-stream"
	}
}

func sanitizePathSegment(value string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
		"@", "_",
	)
	out := strings.TrimSpace(replacer.Replace(value))
	if out == "" {
		return "unknown"
	}
	return out
}

func sanitizeFileName(value string) string {
	value = filepath.Base(strings.TrimSpace(value))
	if value == "." || value == "/" || value == "" {
		return ""
	}
	return sanitizePathSegment(value)
}

func extensionForMIME(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "video/mp4":
		return ".mp4"
	case "audio/silk":
		return ".silk"
	case "audio/amr":
		return ".amr"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func detectVoiceEncodeType(filePath string) int {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".amr":
		return 5
	case ".mp3":
		return 7
	case ".ogg":
		return 8
	default:
		return 6
	}
}

func isAudioFilePath(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".silk", ".amr", ".mp3", ".ogg", ".wav", ".m4a":
		return true
	default:
		return false
	}
}

func validateListenSecurity(listenAddr, apiToken string) error {
	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fmt.Errorf("invalid listen_addr %q: %w", listenAddr, err)
	}
	if isLoopbackHost(host) || strings.TrimSpace(apiToken) != "" {
		return nil
	}
	return errors.New("WCFLINK_API_TOKEN is required when listen_addr is not loopback")
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validateLocalMediaPath(filePath, root string, maxBytes int64) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", errors.New("media send root is required")
	}
	if maxBytes <= 0 {
		return "", errors.New("max media size must be positive")
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	rootPath, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	rootPath, err = filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", err
	}

	candidate := strings.TrimSpace(filePath)
	if candidate == "" {
		return "", errors.New("file_path is required")
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootPath, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	candidate, err = filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootPath, candidate)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file_path must be under media send root %s", rootPath)
	}

	info, err := os.Stat(candidate)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("file_path must point to a regular file")
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("media file exceeds max size %d bytes", maxBytes)
	}
	return candidate, nil
}

func (s *service) deliverWebhook(webhookURL string, payload []byte) {
	if err := netguard.ValidateOutboundURL(context.Background(), webhookURL); err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "webhook url rejected", "webhook", err.Error())
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "build webhook request failed", "webhook", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := netguard.NewHTTPClient(10 * time.Second).Do(req)
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "webhook delivery failed", "webhook", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = s.store.AddLog(context.Background(), "ERROR", fmt.Sprintf("webhook delivery failed with status %d", resp.StatusCode), "webhook", "")
		return
	}
	_ = s.store.AddLog(context.Background(), "INFO", "webhook delivered", "webhook", "")
}

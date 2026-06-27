package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/lich0821/wcfLink/internal/model"
	"github.com/lich0821/wcfLink/internal/store"
	"github.com/lich0821/wcfLink/internal/wecom"
)

type wecomService struct {
	cfg         wecomServiceConfig
	logger      *slog.Logger
	store       *store.Store
	wecomClient *wecom.Client
}

type wecomServiceConfig struct {
	WeComWebhookURL string
}

func newWeComService(cfg wecomServiceConfig, logger *slog.Logger, st *store.Store, wecomClient *wecom.Client) *wecomService {
	return &wecomService{
		cfg:         cfg,
		logger:      logger,
		store:       st,
		wecomClient: wecomClient,
	}
}

func (s *wecomService) HandleInbound(ctx context.Context, account wecom.AccountConfig, msg wecom.InboundMessage) {
	rawJSON, _ := json.Marshal(msg)
	bodyText := msg.Content
	if bodyText == "" {
		bodyText = formatInboundBody(msg)
	}

	_ = s.store.SaveWeComEvent(ctx, account.CorpID, account.AgentID,
		"inbound", msg.MsgType, msg.FromUserName, msg.ToUserName,
		msg.MsgID, bodyText, msg.MediaID, string(rawJSON))
	_ = s.store.TouchWeComAccountInbound(ctx, account.CorpID, account.AgentID)

	webhookURL := s.cfg.WeComWebhookURL
	stored, err := s.store.GetWeComAccount(ctx, account.CorpID, account.AgentID)
	if err == nil && webhookURL == "" && stored.WebhookURL != "" {
		webhookURL = stored.WebhookURL
	}

	autoReply := err == nil && stored.AutoReply
	if !autoReply {
		if webhookURL != "" {
			go s.deliverWeComWebhook(webhookURL, account, msg, rawJSON)
		}
		return
	}

	if msg.MsgType == "event" {
		return
	}

	if webhookURL != "" {
		go s.deliverWeComWebhookAndReply(ctx, webhookURL, account, msg, rawJSON)
		return
	}

	s.logger.Info("wecom auto-reply: no webhook configured, sending echo reply",
		"from", msg.FromUserName, "content", msg.Content)
	replyText := fmt.Sprintf("[wcfLink] 已收到消息: %s", bodyText)
	s.sendReply(ctx, account, msg.FromUserName, replyText)
}

func (s *wecomService) sendReply(ctx context.Context, account wecom.AccountConfig, toUser, text string) {
	accessToken, err := s.wecomClient.GetAccessToken(ctx, account.CorpID, account.CorpSecret)
	if err != nil {
		s.logger.Error("wecom auto-reply: get access token failed", "err", err)
		_ = s.store.UpdateWeComAccountError(ctx, account.CorpID, account.AgentID, err.Error())
		return
	}
	if err := s.wecomClient.SendTextMessage(ctx, accessToken, account.AgentID, toUser, text); err != nil {
		s.logger.Error("wecom auto-reply: send message failed", "err", err)
		_ = s.store.UpdateWeComAccountError(ctx, account.CorpID, account.AgentID, err.Error())
		return
	}
	rawJSON, _ := json.Marshal(map[string]any{
		"to_user": toUser,
		"text":    text,
	})
	_ = s.store.SaveWeComEvent(ctx, account.CorpID, account.AgentID,
		"outbound", "text", "", toUser, 0, text, "", string(rawJSON))
}

func (s *wecomService) deliverWeComWebhook(webhookURL string, account wecom.AccountConfig, msg wecom.InboundMessage, rawJSON []byte) {
	payload, _ := json.Marshal(map[string]any{
		"channel":      "wecom",
		"corp_id":      account.CorpID,
		"agent_id":     account.AgentID,
		"from_user":    msg.FromUserName,
		"to_user":      msg.ToUserName,
		"msg_type":     msg.MsgType,
		"content":      msg.Content,
		"msg_id":       msg.MsgID,
		"media_id":     msg.MediaID,
		"pic_url":      msg.PicURL,
		"recognition":  msg.Recognition,
		"event_type":   msg.EventType,
		"event_key":    msg.EventKey,
		"raw_message":  json.RawMessage(rawJSON),
		"received_at":  time.Now().UTC(),
	})
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "build wecom webhook request failed", "wecom_webhook", err.Error())
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = s.store.AddLog(context.Background(), "ERROR", "wecom webhook delivery failed", "wecom_webhook", err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = s.store.AddLog(context.Background(), "ERROR", fmt.Sprintf("wecom webhook delivery failed with status %d", resp.StatusCode), "wecom_webhook", "")
		return
	}
	_ = s.store.AddLog(context.Background(), "INFO", "wecom webhook delivered", "wecom_webhook", "")
}

func (s *wecomService) deliverWeComWebhookAndReply(ctx context.Context, webhookURL string, account wecom.AccountConfig, msg wecom.InboundMessage, rawJSON []byte) {
	payload, _ := json.Marshal(map[string]any{
		"channel":      "wecom",
		"corp_id":      account.CorpID,
		"agent_id":     account.AgentID,
		"from_user":    msg.FromUserName,
		"to_user":      msg.ToUserName,
		"msg_type":     msg.MsgType,
		"content":      msg.Content,
		"msg_id":       msg.MsgID,
		"media_id":     msg.MediaID,
		"pic_url":      msg.PicURL,
		"recognition":  msg.Recognition,
		"event_type":   msg.EventType,
		"event_key":    msg.EventKey,
		"raw_message":  json.RawMessage(rawJSON),
		"received_at":  time.Now().UTC(),
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(payload))
	if err != nil {
		s.logger.Error("wecom auto-reply: build webhook request failed", "err", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("wecom auto-reply: webhook request failed", "err", err)
		s.sendReply(ctx, account, msg.FromUserName, "[wcfLink] 处理请求失败，请稍后重试")
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Error("wecom auto-reply: webhook returned error", "status", resp.StatusCode)
		return
	}

	var webhookResp struct {
		Reply string `json:"reply"`
		Text  string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &webhookResp); err != nil {
		replyText := strings.TrimSpace(string(respBody))
		if replyText != "" {
			s.sendReply(ctx, account, msg.FromUserName, replyText)
		}
		return
	}

	replyText := webhookResp.Reply
	if replyText == "" {
		replyText = webhookResp.Text
	}
	if strings.TrimSpace(replyText) != "" {
		s.sendReply(ctx, account, msg.FromUserName, replyText)
	}
}

func (s *wecomService) SendText(ctx context.Context, corpID, corpSecret string, agentID int, toUser, text string) error {
	accessToken, err := s.wecomClient.GetAccessToken(ctx, corpID, corpSecret)
	if err != nil {
		return err
	}
	if err := s.wecomClient.SendTextMessage(ctx, accessToken, agentID, toUser, text); err != nil {
		return err
	}
	rawJSON, _ := json.Marshal(map[string]any{
		"to_user": toUser,
		"text":    text,
	})
	return s.store.SaveWeComEvent(ctx, corpID, agentID,
		"outbound", "text", "", toUser, 0, text, "", string(rawJSON))
}

func (s *wecomService) SendMedia(ctx context.Context, corpID, corpSecret string, agentID int, toUser, mediaType, filePath string, fileData []byte) error {
	accessToken, err := s.wecomClient.GetAccessToken(ctx, corpID, corpSecret)
	if err != nil {
		return err
	}
	mediaID, err := s.wecomClient.UploadMedia(ctx, accessToken, mediaType, filePath, fileData)
	if err != nil {
		return err
	}
	switch mediaType {
	case "image":
		err = s.wecomClient.SendImageMessage(ctx, accessToken, agentID, toUser, mediaID)
	case "voice":
		err = s.wecomClient.SendVoiceMessage(ctx, accessToken, agentID, toUser, mediaID)
	case "video":
		err = s.wecomClient.SendVideoMessage(ctx, accessToken, agentID, toUser, mediaID, "", "")
	case "file":
		err = s.wecomClient.SendFileMessage(ctx, accessToken, agentID, toUser, mediaID)
	default:
		err = fmt.Errorf("unsupported media type %q", mediaType)
	}
	if err != nil {
		return err
	}
	rawJSON, _ := json.Marshal(map[string]any{
		"to_user":    toUser,
		"media_type": mediaType,
		"media_id":   mediaID,
	})
	return s.store.SaveWeComEvent(ctx, corpID, agentID,
		"outbound", mediaType, "", toUser, 0, "", mediaID, string(rawJSON))
}

func (s *wecomService) ListAccounts(ctx context.Context) ([]model.WeComAccount, error) {
	return s.store.ListWeComAccounts(ctx)
}

func (s *wecomService) ListEvents(ctx context.Context, afterID int64, limit int) ([]model.WeComEvent, error) {
	return s.store.ListWeComEvents(ctx, afterID, limit)
}

func (s *wecomService) AddAccount(ctx context.Context, account model.WeComAccount) error {
	return s.store.UpsertWeComAccount(ctx, account)
}

func (s *wecomService) RemoveAccount(ctx context.Context, corpID string, agentID int) error {
	return s.store.DeleteWeComAccount(ctx, corpID, agentID)
}

func formatInboundBody(msg wecom.InboundMessage) string {
	switch msg.MsgType {
	case "text":
		return msg.Content
	case "image":
		return "[image]"
	case "voice":
		if msg.Recognition != "" {
			return "[voice] " + msg.Recognition
		}
		return "[voice]"
	case "video":
		return "[video]"
	case "file":
		return "[file]"
	case "event":
		return fmt.Sprintf("[event] %s", msg.EventType)
	default:
		return fmt.Sprintf("[%s]", msg.MsgType)
	}
}

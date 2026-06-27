package ilink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	httpClient     *http.Client
	channelVersion string
}

func NewClient(channelVersion string, timeout time.Duration) *Client {
	return &Client{
		httpClient:     &http.Client{Timeout: timeout},
		channelVersion: channelVersion,
	}
}

type QRCodeResponse struct {
	QRCode    string `json:"qrcode"`
	QRCodeURL string `json:"qrcode_img_content"`
}

type QRStatusResponse struct {
	Status      string `json:"status"`
	BotToken    string `json:"bot_token"`
	AccountID   string `json:"ilink_bot_id"`
	BaseURL     string `json:"baseurl"`
	ILinkUserID string `json:"ilink_user_id"`
}

type TextItem struct {
	Text string `json:"text,omitempty"`
}

type VoiceItem struct {
	Media      CDNMedia `json:"media,omitempty"`
	Text       string   `json:"text,omitempty"`
	EncodeType int      `json:"encode_type,omitempty"`
	Playtime   int      `json:"playtime,omitempty"`
	SampleRate int      `json:"sample_rate,omitempty"`
}

type FileItem struct {
	Media    CDNMedia `json:"media,omitempty"`
	FileName string `json:"file_name,omitempty"`
	MD5      string `json:"md5,omitempty"`
	Len      string `json:"len,omitempty"`
}

type CDNMedia struct {
	EncryptQueryParam string `json:"encrypt_query_param,omitempty"`
	AESKey            string `json:"aes_key,omitempty"`
	EncryptType       int    `json:"encrypt_type,omitempty"`
	FullURL           string `json:"full_url,omitempty"`
}

type ImageItem struct {
	Media       CDNMedia `json:"media,omitempty"`
	ThumbMedia  CDNMedia `json:"thumb_media,omitempty"`
	AESKey      string   `json:"aeskey,omitempty"`
	URL         string   `json:"url,omitempty"`
	MidSize     int      `json:"mid_size,omitempty"`
	ThumbSize   int      `json:"thumb_size,omitempty"`
	ThumbHeight int      `json:"thumb_height,omitempty"`
	ThumbWidth  int      `json:"thumb_width,omitempty"`
	HDSize      int      `json:"hd_size,omitempty"`
}

type VideoItem struct {
	Media       CDNMedia `json:"media,omitempty"`
	ThumbMedia  CDNMedia `json:"thumb_media,omitempty"`
	VideoSize   int      `json:"video_size,omitempty"`
	PlayLength  int      `json:"play_length,omitempty"`
	VideoMD5    string   `json:"video_md5,omitempty"`
	ThumbSize   int      `json:"thumb_size,omitempty"`
	ThumbHeight int      `json:"thumb_height,omitempty"`
	ThumbWidth  int      `json:"thumb_width,omitempty"`
}

type RefMessage struct {
	MessageItem *MessageItem `json:"message_item,omitempty"`
	Title       string       `json:"title,omitempty"`
}

type ToolCallStartItem struct {
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type ToolCallResultItem struct {
	ToolName   string `json:"tool_name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	Status     string `json:"status,omitempty"`
}

const (
	MessageItemTypeText           = 1
	MessageItemTypeImage          = 2
	MessageItemTypeVoice          = 3
	MessageItemTypeFile           = 4
	MessageItemTypeVideo          = 5
	MessageItemTypeToolCallStart  = 11
	MessageItemTypeToolCallResult = 12
)

type MessageItem struct {
	Type               int                  `json:"type,omitempty"`
	CreateTimeMS       int64                `json:"create_time_ms,omitempty"`
	UpdateTimeMS       int64                `json:"update_time_ms,omitempty"`
	IsCompleted        bool                 `json:"is_completed,omitempty"`
	MsgID              string               `json:"msg_id,omitempty"`
	RefMsg             *RefMessage          `json:"ref_msg,omitempty"`
	TextItem           *TextItem            `json:"text_item,omitempty"`
	VoiceItem          *VoiceItem           `json:"voice_item,omitempty"`
	FileItem           *FileItem            `json:"file_item,omitempty"`
	ImageItem          *ImageItem           `json:"image_item,omitempty"`
	VideoItem          *VideoItem           `json:"video_item,omitempty"`
	ToolCallStartItem  *ToolCallStartItem   `json:"tool_call_start_item,omitempty"`
	ToolCallResultItem *ToolCallResultItem  `json:"tool_call_result_item,omitempty"`
}

type WeixinMessage struct {
	Seq          int64         `json:"seq,omitempty"`
	MessageID    int64         `json:"message_id,omitempty"`
	FromUserID   string        `json:"from_user_id,omitempty"`
	ToUserID     string        `json:"to_user_id,omitempty"`
	ClientID     string        `json:"client_id,omitempty"`
	CreateTimeMS int64         `json:"create_time_ms,omitempty"`
	UpdateTimeMS int64         `json:"update_time_ms,omitempty"`
	DeleteTimeMS int64         `json:"delete_time_ms,omitempty"`
	SessionID    string        `json:"session_id,omitempty"`
	GroupID      string        `json:"group_id,omitempty"`
	MessageType  int           `json:"message_type,omitempty"`
	MessageState int           `json:"message_state,omitempty"`
	ItemList     []MessageItem `json:"item_list,omitempty"`
	ContextToken string        `json:"context_token,omitempty"`
	RunID        string        `json:"run_id,omitempty"`
}

type GetUpdatesResponse struct {
	Ret                int             `json:"ret,omitempty"`
	ErrCode            int             `json:"errcode,omitempty"`
	ErrMsg             string          `json:"errmsg,omitempty"`
	Messages           []WeixinMessage `json:"msgs,omitempty"`
	GetUpdatesBuf      string          `json:"get_updates_buf,omitempty"`
	LongPollingTimeout int             `json:"longpolling_timeout_ms,omitempty"`
}

type SendMessageResponse struct {
	Ret     int    `json:"ret,omitempty"`
	ErrCode int    `json:"errcode,omitempty"`
	ErrMsg  string `json:"errmsg,omitempty"`
}

type GetConfigResponse struct {
	Ret           int    `json:"ret,omitempty"`
	ErrMsg        string `json:"errmsg,omitempty"`
	TypingTicket  string `json:"typing_ticket,omitempty"`
}

type NotifyResponse struct {
	Ret    int    `json:"ret,omitempty"`
	ErrMsg string `json:"errmsg,omitempty"`
}

const (
	UploadMediaTypeImage = 1
	UploadMediaTypeVideo = 2
	UploadMediaTypeFile  = 3
	UploadMediaTypeVoice = 4

	TypingStatusTyping = 1
	TypingStatusCancel = 2
)

func (c *Client) FetchQRCode(ctx context.Context, baseURL string) (QRCodeResponse, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/ilink/bot/get_bot_qrcode?bot_type=3")
	if err != nil {
		return QRCodeResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return QRCodeResponse{}, err
	}
	var out QRCodeResponse
	if err := c.doJSON(req, "", nil, &out); err != nil {
		return QRCodeResponse{}, err
	}
	return out, nil
}

func (c *Client) FetchQRCodeStatus(ctx context.Context, baseURL, qrCode string) (QRStatusResponse, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/ilink/bot/get_qrcode_status?qrcode=" + url.QueryEscape(qrCode))
	if err != nil {
		return QRStatusResponse{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return QRStatusResponse{}, err
	}
	req.Header.Set("iLink-App-ClientVersion", "1")
	var out QRStatusResponse
	if err := c.doJSON(req, "", nil, &out); err != nil {
		return QRStatusResponse{}, err
	}
	return out, nil
}

func (c *Client) GetUpdates(ctx context.Context, baseURL, token, getUpdatesBuf string) (GetUpdatesResponse, error) {
	body := map[string]any{
		"get_updates_buf": getUpdatesBuf,
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	var out GetUpdatesResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/getupdates", token, body, &out); err != nil {
		return GetUpdatesResponse{}, err
	}
	return out, nil
}

func (c *Client) SendTextMessage(ctx context.Context, baseURL, token, toUserID, text, contextToken string) error {
	msg := map[string]any{
		"from_user_id":  "",
		"to_user_id":    toUserID,
		"client_id":     fmt.Sprintf("wcfLink-%d", time.Now().UnixNano()),
		"message_type":  2,
		"message_state": 2,
		"item_list": []map[string]any{
			{
				"type": 1,
				"text_item": map[string]any{
					"text": text,
				},
			},
		},
	}
	if strings.TrimSpace(contextToken) != "" {
		msg["context_token"] = contextToken
	}

	body := map[string]any{
		"msg": msg,
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	var out SendMessageResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/sendmessage", token, body, &out); err != nil {
		return err
	}
	if out.ErrCode != 0 || out.Ret != 0 {
		errText := out.ErrMsg
		if strings.TrimSpace(errText) == "" {
			errText = "sendmessage returned non-zero status"
		}
		return fmt.Errorf("%s (ret=%d errcode=%d)", errText, out.Ret, out.ErrCode)
	}
	return nil
}

func (c *Client) GetConfig(ctx context.Context, baseURL, token, ilinkUserID, contextToken string) (GetConfigResponse, error) {
	body := map[string]any{
		"ilink_user_id": ilinkUserID,
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	if strings.TrimSpace(contextToken) != "" {
		body["context_token"] = contextToken
	}
	var out GetConfigResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/getconfig", token, body, &out); err != nil {
		return GetConfigResponse{}, err
	}
	return out, nil
}

func (c *Client) SendTyping(ctx context.Context, baseURL, token, ilinkUserID, typingTicket string, status int) error {
	body := map[string]any{
		"ilink_user_id":  ilinkUserID,
		"typing_ticket":  typingTicket,
		"status":         status,
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	var out NotifyResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/sendtyping", token, body, &out); err != nil {
		return err
	}
	if out.Ret != 0 {
		errText := out.ErrMsg
		if strings.TrimSpace(errText) == "" {
			errText = "sendtyping returned non-zero status"
		}
		return fmt.Errorf("%s (ret=%d)", errText, out.Ret)
	}
	return nil
}

func (c *Client) NotifyStart(ctx context.Context, baseURL, token string) (NotifyResponse, error) {
	body := map[string]any{
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	var out NotifyResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/msg/notifystart", token, body, &out); err != nil {
		return NotifyResponse{}, err
	}
	return out, nil
}

func (c *Client) NotifyStop(ctx context.Context, baseURL, token string) (NotifyResponse, error) {
	body := map[string]any{
		"base_info": map[string]any{
			"channel_version": c.channelVersion,
		},
	}
	var out NotifyResponse
	if err := c.postJSON(ctx, strings.TrimRight(baseURL, "/")+"/ilink/bot/msg/notifystop", token, body, &out); err != nil {
		return NotifyResponse{}, err
	}
	return out, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint, token string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	return c.doJSON(req, token, payload, out)
}

func (c *Client) doJSON(req *http.Request, token string, payload []byte, out any) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("AuthorizationType", "ilink_bot_token")
	req.Header.Set("X-WECHAT-UIN", randomWechatUIN())
	if len(payload) > 0 {
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(payload)))
	}
	if strings.TrimSpace(token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ilink http %d: %s", resp.StatusCode, string(raw))
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func ExtractBodyText(msg WeixinMessage) string {
	for _, item := range msg.ItemList {
		switch item.Type {
		case MessageItemTypeText:
			if item.TextItem != nil {
				return item.TextItem.Text
			}
		case MessageItemTypeVoice:
			if item.VoiceItem != nil && item.VoiceItem.Text != "" {
				return item.VoiceItem.Text
			}
		case MessageItemTypeImage:
			return "[image]"
		case MessageItemTypeFile:
			if item.FileItem != nil && item.FileItem.FileName != "" {
				return "[file] " + item.FileItem.FileName
			}
			return "[file]"
		case MessageItemTypeVideo:
			return "[video]"
		case MessageItemTypeToolCallStart:
			if item.ToolCallStartItem != nil {
				return "[tool_call_start] " + item.ToolCallStartItem.ToolName
			}
			return "[tool_call_start]"
		case MessageItemTypeToolCallResult:
			if item.ToolCallResultItem != nil {
				return "[tool_call_result] " + item.ToolCallResultItem.ToolName + " " + item.ToolCallResultItem.Status
			}
			return "[tool_call_result]"
		}
	}
	return ""
}

func DetectEventType(msg WeixinMessage) string {
	for _, item := range msg.ItemList {
		switch item.Type {
		case MessageItemTypeText:
			return "text"
		case MessageItemTypeImage:
			return "image"
		case MessageItemTypeVoice:
			return "voice"
		case MessageItemTypeFile:
			return "file"
		case MessageItemTypeVideo:
			return "video"
		case MessageItemTypeToolCallStart:
			return "tool_call_start"
		case MessageItemTypeToolCallResult:
			return "tool_call_result"
		}
	}
	return "unknown"
}

func randomWechatUIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1<<32-1))
	if err != nil {
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	}
	return base64.StdEncoding.EncodeToString([]byte(n.String()))
}

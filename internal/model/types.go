package model

import "time"

type LoginSession struct {
	SessionID   string     `json:"session_id"`
	BaseURL     string     `json:"base_url"`
	QRCode      string     `json:"-"`
	QRCodeURL   string     `json:"qr_code_url"`
	Status      string     `json:"status"`
	AccountID   string     `json:"account_id,omitempty"`
	ILinkUserID string     `json:"ilink_user_id,omitempty"`
	BotToken    string     `json:"-"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Account struct {
	AccountID     string     `json:"account_id"`
	BaseURL       string     `json:"base_url"`
	Token         string     `json:"-"`
	ILinkUserID   string     `json:"ilink_user_id,omitempty"`
	Enabled       bool       `json:"enabled"`
	LoginStatus   string     `json:"login_status"`
	LastError     string     `json:"last_error,omitempty"`
	GetUpdatesBuf string     `json:"-"`
	LastPollAt    *time.Time `json:"last_poll_at,omitempty"`
	LastInboundAt *time.Time `json:"last_inbound_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type PeerContext struct {
	AccountID    string    `json:"account_id"`
	PeerUserID   string    `json:"peer_user_id"`
	ContextToken string    `json:"-"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Event struct {
	ID            int64     `json:"id"`
	AccountID     string    `json:"account_id"`
	Direction     string    `json:"direction"`
	EventType     string    `json:"event_type"`
	FromUserID    string    `json:"from_user_id,omitempty"`
	ToUserID      string    `json:"to_user_id,omitempty"`
	GroupID       string    `json:"group_id,omitempty"`
	MessageID     int64     `json:"message_id,omitempty"`
	ContextToken  string    `json:"-"`
	BodyText      string    `json:"body_text,omitempty"`
	MediaPath     string    `json:"media_path,omitempty"`
	MediaFileName string    `json:"media_file_name,omitempty"`
	MediaMimeType string    `json:"media_mime_type,omitempty"`
	RawJSON       string    `json:"-"`
	CreatedAt     time.Time `json:"created_at"`
}

type LogEntry struct {
	ID        int64     `json:"id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	MetaJSON  string    `json:"meta_json,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Settings struct {
	ListenAddr string `json:"listen_addr"`
	WebhookURL string `json:"webhook_url"`
}

type WeComAccount struct {
	ID             int64      `json:"id"`
	CorpID         string     `json:"corp_id"`
	CorpSecret     string     `json:"-"`
	AgentID        int        `json:"agent_id"`
	CallbackToken  string     `json:"-"`
	CallbackAESKey string     `json:"-"`
	Enabled        bool       `json:"enabled"`
	AutoReply      bool       `json:"auto_reply"`
	WebhookURL     string     `json:"webhook_url,omitempty"`
	LastError      string     `json:"last_error,omitempty"`
	LastInboundAt  *time.Time `json:"last_inbound_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type WeComEvent struct {
	ID        int64     `json:"id"`
	CorpID    string    `json:"corp_id"`
	AgentID   int       `json:"agent_id"`
	Direction string    `json:"direction"`
	EventType string    `json:"event_type"`
	FromUser  string    `json:"from_user,omitempty"`
	ToUser    string    `json:"to_user,omitempty"`
	MsgID     int64     `json:"msg_id,omitempty"`
	BodyText  string    `json:"body_text,omitempty"`
	MediaID   string    `json:"media_id,omitempty"`
	RawJSON   string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

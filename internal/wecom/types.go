package wecom

import "time"

type AccountConfig struct {
	CorpID         string `json:"corp_id"`
	CorpSecret     string `json:"-"`
	AgentID        int    `json:"agent_id"`
	CallbackToken  string `json:"-"`
	CallbackAESKey string `json:"-"`
}

type AccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type SendMessageRequest struct {
	ToUser  string `json:"touser,omitempty"`
	ToParty string `json:"toparty,omitempty"`
	ToTag   string `json:"totag,omitempty"`
	MsgType string `json:"msgtype"`
	AgentID int    `json:"agentid"`
	Text    *Text  `json:"text,omitempty"`
	Image   *Media `json:"image,omitempty"`
	Voice   *Media `json:"voice,omitempty"`
	Video   *Video `json:"video,omitempty"`
	File    *Media `json:"file,omitempty"`
}

type Text struct {
	Content string `json:"content"`
}

type Media struct {
	MediaID string `json:"media_id"`
}

type Video struct {
	MediaID     string `json:"media_id"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

type SendMessageResponse struct {
	ErrCode      int    `json:"errcode"`
	ErrMsg       string `json:"errmsg"`
	InvalidUser  string `json:"invaliduser,omitempty"`
	InvalidParty string `json:"invalidparty,omitempty"`
	InvalidTag   string `json:"invalidtag,omitempty"`
	MsgID        string `json:"msgid,omitempty"`
}

type UploadMediaResponse struct {
	ErrCode   int    `json:"errcode"`
	ErrMsg    string `json:"errmsg"`
	Type      string `json:"type"`
	MediaID   string `json:"media_id"`
	CreatedAt string `json:"created_at"`
}

type InboundMessage struct {
	ToUserName   string
	FromUserName string
	CreateTime   int64
	MsgType      string
	Content      string
	MsgID        int64
	AgentID      int
	PicURL       string
	MediaID      string
	Format       string
	Recognition  string
	ThumbMediaID string
	EventType    string
	EventKey     string
}

type AutoReplyConfig struct {
	Enabled    bool   `json:"enabled"`
	WebhookURL string `json:"webhook_url"`
}

type WeComAccount struct {
	ID             int64     `json:"id"`
	CorpID         string    `json:"corp_id"`
	AgentID        int       `json:"agent_id"`
	Enabled        bool      `json:"enabled"`
	LastError      string    `json:"last_error,omitempty"`
	LastInboundAt  *time.Time `json:"last_inbound_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

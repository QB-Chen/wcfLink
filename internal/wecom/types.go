package wecom

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

type UserInfo struct {
	UserID     string `json:"userid"`
	Name       string `json:"name"`
	Department []int  `json:"department"`
	Position   string `json:"position,omitempty"`
	Mobile     string `json:"mobile,omitempty"`
	Gender     string `json:"gender,omitempty"`
	Email      string `json:"email,omitempty"`
	BizMail    string `json:"biz_mail,omitempty"`
	Avatar     string `json:"avatar,omitempty"`
	Status     int    `json:"status"`
	Alias      string `json:"alias,omitempty"`
}

type GetUserResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	UserInfo
}

type DepartmentUserListResponse struct {
	ErrCode  int        `json:"errcode"`
	ErrMsg   string     `json:"errmsg"`
	UserList []UserInfo `json:"userlist"`
}

type DepartmentInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ParentID int    `json:"parentid"`
	Order    int    `json:"order"`
}

type DepartmentListResponse struct {
	ErrCode    int              `json:"errcode"`
	ErrMsg     string           `json:"errmsg"`
	Department []DepartmentInfo `json:"department"`
}

type GroupChatInfo struct {
	ChatID   string            `json:"chatid"`
	Name     string            `json:"name"`
	Owner    string            `json:"owner"`
	UserList []GroupChatMember `json:"userlist"`
}

type GroupChatMember struct {
	UserID string `json:"userid"`
}

type GetGroupChatResponse struct {
	ErrCode  int           `json:"errcode"`
	ErrMsg   string        `json:"errmsg"`
	ChatInfo GroupChatInfo `json:"chat_info"`
}

type CreateGroupChatRequest struct {
	Name     string   `json:"name"`
	Owner    string   `json:"owner"`
	UserList []string `json:"userlist"`
	ChatID   string   `json:"chatid,omitempty"`
}

type CreateGroupChatResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
	ChatID  string `json:"chatid"`
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

package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultAPIBaseURL = "https://qyapi.weixin.qq.com"

type Client struct {
	httpClient *http.Client
	apiBaseURL string

	mu         sync.Mutex
	tokenCache map[string]*tokenEntry
}

type tokenEntry struct {
	token     string
	expiresAt time.Time
}

func NewClient(apiBaseURL string) *Client {
	if strings.TrimSpace(apiBaseURL) == "" {
		apiBaseURL = defaultAPIBaseURL
	}
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
		tokenCache: make(map[string]*tokenEntry),
	}
}

func (c *Client) GetAccessToken(ctx context.Context, corpID, corpSecret string) (string, error) {
	cacheKey := corpID + ":" + corpSecret
	c.mu.Lock()
	if entry, ok := c.tokenCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		token := entry.token
		c.mu.Unlock()
		return token, nil
	}
	c.mu.Unlock()

	u := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		c.apiBaseURL,
		url.QueryEscape(corpID),
		url.QueryEscape(corpSecret),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result AccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("gettoken failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}

	c.mu.Lock()
	c.tokenCache[cacheKey] = &tokenEntry{
		token:     result.AccessToken,
		expiresAt: time.Now().Add(time.Duration(result.ExpiresIn-120) * time.Second),
	}
	c.mu.Unlock()

	return result.AccessToken, nil
}

func (c *Client) SendTextMessage(ctx context.Context, accessToken string, agentID int, toUser, content string) error {
	msg := SendMessageRequest{
		ToUser:  toUser,
		MsgType: "text",
		AgentID: agentID,
		Text:    &Text{Content: content},
	}
	return c.sendMessage(ctx, accessToken, msg)
}

func (c *Client) SendImageMessage(ctx context.Context, accessToken string, agentID int, toUser, mediaID string) error {
	msg := SendMessageRequest{
		ToUser:  toUser,
		MsgType: "image",
		AgentID: agentID,
		Image:   &Media{MediaID: mediaID},
	}
	return c.sendMessage(ctx, accessToken, msg)
}

func (c *Client) SendVoiceMessage(ctx context.Context, accessToken string, agentID int, toUser, mediaID string) error {
	msg := SendMessageRequest{
		ToUser:  toUser,
		MsgType: "voice",
		AgentID: agentID,
		Voice:   &Media{MediaID: mediaID},
	}
	return c.sendMessage(ctx, accessToken, msg)
}

func (c *Client) SendVideoMessage(ctx context.Context, accessToken string, agentID int, toUser, mediaID, title, description string) error {
	msg := SendMessageRequest{
		ToUser:  toUser,
		MsgType: "video",
		AgentID: agentID,
		Video: &Video{
			MediaID:     mediaID,
			Title:       title,
			Description: description,
		},
	}
	return c.sendMessage(ctx, accessToken, msg)
}

func (c *Client) SendFileMessage(ctx context.Context, accessToken string, agentID int, toUser, mediaID string) error {
	msg := SendMessageRequest{
		ToUser:  toUser,
		MsgType: "file",
		AgentID: agentID,
		File:    &Media{MediaID: mediaID},
	}
	return c.sendMessage(ctx, accessToken, msg)
}

func (c *Client) sendMessage(ctx context.Context, accessToken string, msg SendMessageRequest) error {
	u := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s",
		c.apiBaseURL, url.QueryEscape(accessToken))
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result SendMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode send response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("send message failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

func (c *Client) UploadMedia(ctx context.Context, accessToken, mediaType, fileName string, data []byte) (string, error) {
	u := fmt.Sprintf("%s/cgi-bin/media/upload?access_token=%s&type=%s",
		c.apiBaseURL, url.QueryEscape(accessToken), url.QueryEscape(mediaType))

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("media", fileName)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result UploadMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("upload media failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.MediaID, nil
}

func (c *Client) GetUser(ctx context.Context, accessToken, userID string) (UserInfo, error) {
	u := fmt.Sprintf("%s/cgi-bin/user/get?access_token=%s&userid=%s",
		c.apiBaseURL, url.QueryEscape(accessToken), url.QueryEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return UserInfo{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()
	var result GetUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return UserInfo{}, fmt.Errorf("decode user response: %w", err)
	}
	if result.ErrCode != 0 {
		return UserInfo{}, fmt.Errorf("get user failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.UserInfo, nil
}

func (c *Client) ListDepartmentUsers(ctx context.Context, accessToken string, departmentID int) ([]UserInfo, error) {
	u := fmt.Sprintf("%s/cgi-bin/user/list?access_token=%s&department_id=%d",
		c.apiBaseURL, url.QueryEscape(accessToken), departmentID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result DepartmentUserListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode user list response: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("list users failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.UserList, nil
}

func (c *Client) ListDepartments(ctx context.Context, accessToken string) ([]DepartmentInfo, error) {
	u := fmt.Sprintf("%s/cgi-bin/department/list?access_token=%s",
		c.apiBaseURL, url.QueryEscape(accessToken))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result DepartmentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode department response: %w", err)
	}
	if result.ErrCode != 0 {
		return nil, fmt.Errorf("list departments failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.Department, nil
}

func (c *Client) GetGroupChat(ctx context.Context, accessToken, chatID string) (GroupChatInfo, error) {
	u := fmt.Sprintf("%s/cgi-bin/appchat/get?access_token=%s&chatid=%s",
		c.apiBaseURL, url.QueryEscape(accessToken), url.QueryEscape(chatID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return GroupChatInfo{}, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return GroupChatInfo{}, err
	}
	defer resp.Body.Close()
	var result GetGroupChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return GroupChatInfo{}, fmt.Errorf("decode group chat response: %w", err)
	}
	if result.ErrCode != 0 {
		return GroupChatInfo{}, fmt.Errorf("get group chat failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.ChatInfo, nil
}

func (c *Client) CreateGroupChat(ctx context.Context, accessToken string, chatReq CreateGroupChatRequest) (string, error) {
	u := fmt.Sprintf("%s/cgi-bin/appchat/create?access_token=%s",
		c.apiBaseURL, url.QueryEscape(accessToken))
	payload, err := json.Marshal(chatReq)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result CreateGroupChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode create group chat response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("create group chat failed: errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	return result.ChatID, nil
}

func (c *Client) DownloadMedia(ctx context.Context, accessToken, mediaID string) ([]byte, string, error) {
	u := fmt.Sprintf("%s/cgi-bin/media/get?access_token=%s&media_id=%s",
		c.apiBaseURL, url.QueryEscape(accessToken), url.QueryEscape(mediaID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		raw, _ := io.ReadAll(resp.Body)
		return nil, "", fmt.Errorf("download media failed: %s", string(raw))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, contentType, nil
}

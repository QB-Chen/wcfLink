package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QB-Chen/wcfLink/internal/llm"
	"github.com/google/uuid"
)

type SessionKey struct {
	ChannelType string // "ilink" or "wecom"
	UserID      string
	GroupID     string
}

func (k SessionKey) String() string {
	return k.ChannelType + ":" + k.UserID + ":" + k.GroupID
}

type Conversation struct {
	ID          string
	ChannelType string
	UserID      string
	GroupID     string
	Mode        string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ConversationManager struct {
	db    *sql.DB
	locks sync.Map // map[string]*sync.Mutex for per-session locks
}

func NewConversationManager(db *sql.DB) *ConversationManager {
	return &ConversationManager{db: db}
}

func (m *ConversationManager) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS conversations (
			id            TEXT PRIMARY KEY,
			channel_type  TEXT NOT NULL,
			user_id       TEXT NOT NULL,
			group_id      TEXT NOT NULL DEFAULT '',
			mode          TEXT NOT NULL DEFAULT 'icemark',
			created_at    TIMESTAMP NOT NULL,
			updated_at    TIMESTAMP NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_conv_session_key
		 ON conversations(channel_type, user_id, group_id);`,
		`CREATE TABLE IF NOT EXISTS conversation_messages (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			role            TEXT NOT NULL,
			content         TEXT NOT NULL DEFAULT '',
			tool_calls      TEXT NOT NULL DEFAULT '',
			tool_call_id    TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_conv_msg_conv_id
		 ON conversation_messages(conversation_id);`,
		`CREATE TABLE IF NOT EXISTS tool_call_logs (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			tool_name       TEXT NOT NULL,
			arguments       TEXT NOT NULL DEFAULT '',
			result          TEXT NOT NULL DEFAULT '',
			duration_ms     INTEGER NOT NULL DEFAULT 0,
			error           TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMP NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := m.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("conversation migrate: %w", err)
		}
	}
	return nil
}

func (m *ConversationManager) GetSessionLock(key SessionKey) *sync.Mutex {
	v, _ := m.locks.LoadOrStore(key.String(), &sync.Mutex{})
	return v.(*sync.Mutex)
}

func (m *ConversationManager) GetOrCreate(ctx context.Context, key SessionKey, defaultMode string) (Conversation, error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT id, channel_type, user_id, group_id, mode, created_at, updated_at
		 FROM conversations
		 WHERE channel_type = ? AND user_id = ? AND group_id = ?`,
		key.ChannelType, key.UserID, key.GroupID)

	var conv Conversation
	err := row.Scan(&conv.ID, &conv.ChannelType, &conv.UserID, &conv.GroupID, &conv.Mode, &conv.CreatedAt, &conv.UpdatedAt)
	if err == nil {
		return conv, nil
	}
	if err != sql.ErrNoRows {
		return Conversation{}, err
	}

	now := time.Now().UTC()
	conv = Conversation{
		ID:          uuid.New().String(),
		ChannelType: key.ChannelType,
		UserID:      key.UserID,
		GroupID:     key.GroupID,
		Mode:        defaultMode,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err = m.db.ExecContext(ctx,
		`INSERT INTO conversations (id, channel_type, user_id, group_id, mode, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		conv.ID, conv.ChannelType, conv.UserID, conv.GroupID, conv.Mode, now, now)
	if err != nil {
		return Conversation{}, err
	}
	return conv, nil
}

func (m *ConversationManager) UpdateMode(ctx context.Context, convID, mode string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE conversations SET mode = ?, updated_at = ? WHERE id = ?`,
		mode, time.Now().UTC(), convID)
	return err
}

func (m *ConversationManager) TouchUpdatedAt(ctx context.Context, convID string) error {
	_, err := m.db.ExecContext(ctx,
		`UPDATE conversations SET updated_at = ? WHERE id = ?`,
		time.Now().UTC(), convID)
	return err
}

func (m *ConversationManager) AddMessage(ctx context.Context, convID string, msg llm.Message) error {
	toolCallsJSON := ""
	if len(msg.ToolCalls) > 0 {
		data, err := json.Marshal(msg.ToolCalls)
		if err != nil {
			return err
		}
		toolCallsJSON = string(data)
	}
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO conversation_messages (conversation_id, role, content, tool_calls, tool_call_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		convID, string(msg.Role), msg.Content, toolCallsJSON, msg.ToolCallID, time.Now().UTC())
	return err
}

func (m *ConversationManager) GetMessages(ctx context.Context, convID string) ([]llm.Message, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT role, content, tool_calls, tool_call_id
		 FROM conversation_messages
		 WHERE conversation_id = ?
		 ORDER BY id ASC`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []llm.Message
	for rows.Next() {
		var role, content, toolCallsJSON, toolCallID string
		if err := rows.Scan(&role, &content, &toolCallsJSON, &toolCallID); err != nil {
			return nil, err
		}
		msg := llm.Message{
			Role:       llm.Role(role),
			Content:    content,
			ToolCallID: toolCallID,
		}
		if toolCallsJSON != "" {
			var tcs []llm.ToolCall
			if err := json.Unmarshal([]byte(toolCallsJSON), &tcs); err == nil {
				msg.ToolCalls = tcs
			}
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func (m *ConversationManager) ClearMessages(ctx context.Context, convID string) error {
	_, err := m.db.ExecContext(ctx,
		`DELETE FROM conversation_messages WHERE conversation_id = ?`, convID)
	return err
}

func (m *ConversationManager) DeleteConversation(ctx context.Context, convID string) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM conversation_messages WHERE conversation_id = ?`, convID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tool_call_logs WHERE conversation_id = ?`, convID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, convID); err != nil {
		return err
	}
	return tx.Commit()
}

func (m *ConversationManager) ListConversations(ctx context.Context) ([]Conversation, error) {
	rows, err := m.db.QueryContext(ctx,
		`SELECT id, channel_type, user_id, group_id, mode, created_at, updated_at
		 FROM conversations
		 ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Conversation
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.ChannelType, &c.UserID, &c.GroupID, &c.Mode, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (m *ConversationManager) GetConversation(ctx context.Context, convID string) (Conversation, error) {
	row := m.db.QueryRowContext(ctx,
		`SELECT id, channel_type, user_id, group_id, mode, created_at, updated_at
		 FROM conversations
		 WHERE id = ?`, convID)
	var c Conversation
	err := row.Scan(&c.ID, &c.ChannelType, &c.UserID, &c.GroupID, &c.Mode, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (m *ConversationManager) LogToolCall(ctx context.Context, convID, toolName, arguments, result, errText string, durationMs int64) error {
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO tool_call_logs (conversation_id, tool_name, arguments, result, duration_ms, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		convID, toolName, arguments, truncateString(result, 10000), durationMs, errText, time.Now().UTC())
	return err
}

func (m *ConversationManager) CleanExpired(ctx context.Context, ttl time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-ttl)
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM conversations WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) == 0 {
		return 0, nil
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM conversation_messages WHERE conversation_id IN (%s)`, placeholders), args...); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM tool_call_logs WHERE conversation_id IN (%s)`, placeholders), args...); err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM conversations WHERE id IN (%s)`, placeholders), args...); err != nil {
		return 0, err
	}

	return int64(len(ids)), tx.Commit()
}

func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

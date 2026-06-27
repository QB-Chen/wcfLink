package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/lich0821/wcfLink/internal/ilink"
	"github.com/lich0821/wcfLink/internal/model"
)

type Store struct {
	db *sql.DB
}

func New(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) CreateLoginSession(ctx context.Context, session model.LoginSession) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO login_sessions (
  session_id, base_url, qr_code, qr_code_url, status, error, started_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.SessionID, session.BaseURL, session.QRCode, session.QRCodeURL, session.Status,
		session.Error, session.StartedAt.UTC(), session.UpdatedAt.UTC(),
	)
	return err
}

func (s *Store) GetLoginSession(ctx context.Context, sessionID string) (model.LoginSession, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT session_id, base_url, qr_code, qr_code_url, status, account_id, ilink_user_id, bot_token,
       error, started_at, updated_at, completed_at
FROM login_sessions
WHERE session_id = ?`, sessionID)
	var session model.LoginSession
	var completedAt sql.NullTime
	err := row.Scan(
		&session.SessionID, &session.BaseURL, &session.QRCode, &session.QRCodeURL, &session.Status,
		&session.AccountID, &session.ILinkUserID, &session.BotToken, &session.Error,
		&session.StartedAt, &session.UpdatedAt, &completedAt,
	)
	if err != nil {
		return model.LoginSession{}, err
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}
	return session, nil
}

func (s *Store) UpdateLoginSessionStatus(ctx context.Context, sessionID, status, errorText string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE login_sessions
SET status = ?, error = ?, updated_at = ?
WHERE session_id = ?`, status, errorText, time.Now().UTC(), sessionID)
	return err
}

func (s *Store) CompleteLoginSession(ctx context.Context, sessionID string, status ilink.QRStatusResponse) error {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
UPDATE login_sessions
SET status = ?, account_id = ?, ilink_user_id = ?, bot_token = ?, base_url = ?, updated_at = ?, completed_at = ?
WHERE session_id = ?`,
		status.Status, status.AccountID, status.ILinkUserID, status.BotToken, status.BaseURL, now, now, sessionID,
	)
	if err != nil {
		return err
	}

	baseURL := status.BaseURL
	if baseURL == "" {
		var fallback string
		if err := tx.QueryRowContext(ctx, `SELECT base_url FROM login_sessions WHERE session_id = ?`, sessionID).Scan(&fallback); err == nil {
			baseURL = fallback
		}
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO accounts (
  account_id, base_url, token, ilink_user_id, enabled, login_status, created_at, updated_at
) VALUES (?, ?, ?, ?, 1, 'connected', ?, ?)
ON CONFLICT(account_id) DO UPDATE SET
  base_url = excluded.base_url,
  token = excluded.token,
  ilink_user_id = excluded.ilink_user_id,
  enabled = 1,
  login_status = 'connected',
  last_error = '',
  updated_at = excluded.updated_at`,
		status.AccountID, baseURL, status.BotToken, status.ILinkUserID, now, now,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) ListAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT account_id, base_url, token, ilink_user_id, enabled, login_status, last_error,
       get_updates_buf, last_poll_at, last_inbound_at, created_at, updated_at
FROM accounts
ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Account
	for rows.Next() {
		var item model.Account
		var enabled int
		var lastPollAt sql.NullTime
		var lastInboundAt sql.NullTime
		if err := rows.Scan(
			&item.AccountID, &item.BaseURL, &item.Token, &item.ILinkUserID, &enabled, &item.LoginStatus,
			&item.LastError, &item.GetUpdatesBuf, &lastPollAt, &lastInboundAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		if lastPollAt.Valid {
			item.LastPollAt = &lastPollAt.Time
		}
		if lastInboundAt.Valid {
			item.LastInboundAt = &lastInboundAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetAccount(ctx context.Context, accountID string) (model.Account, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT account_id, base_url, token, ilink_user_id, enabled, login_status, last_error,
       get_updates_buf, last_poll_at, last_inbound_at, created_at, updated_at
FROM accounts
WHERE account_id = ?`, accountID)
	var item model.Account
	var enabled int
	var lastPollAt sql.NullTime
	var lastInboundAt sql.NullTime
	err := row.Scan(
		&item.AccountID, &item.BaseURL, &item.Token, &item.ILinkUserID, &enabled, &item.LoginStatus,
		&item.LastError, &item.GetUpdatesBuf, &lastPollAt, &lastInboundAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return model.Account{}, err
	}
	item.Enabled = enabled == 1
	if lastPollAt.Valid {
		item.LastPollAt = &lastPollAt.Time
	}
	if lastInboundAt.Valid {
		item.LastInboundAt = &lastInboundAt.Time
	}
	return item, nil
}

func (s *Store) DeleteAccount(ctx context.Context, accountID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statements := []string{
		`DELETE FROM accounts WHERE account_id = ?`,
		`DELETE FROM peer_contexts WHERE account_id = ?`,
		`DELETE FROM login_sessions WHERE account_id = ?`,
	}
	for _, stmt := range statements {
		if _, err := tx.ExecContext(ctx, stmt, accountID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) UpdateAccountPollState(ctx context.Context, accountID, getUpdatesBuf, loginStatus, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET get_updates_buf = ?, login_status = ?, last_error = ?, last_poll_at = ?, updated_at = ?
WHERE account_id = ?`, getUpdatesBuf, loginStatus, lastError, time.Now().UTC(), time.Now().UTC(), accountID)
	return err
}

func (s *Store) TouchAccountInbound(ctx context.Context, accountID string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE accounts
SET last_inbound_at = ?, updated_at = ?
WHERE account_id = ?`, now, now, accountID)
	return err
}

func (s *Store) SaveInboundMessage(ctx context.Context, accountID string, msg ilink.WeixinMessage, mediaPath, mediaFileName, mediaMimeType string) error {
	raw, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	bodyText := ilink.ExtractBodyText(msg)
	now := time.Now().UTC()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
INSERT OR IGNORE INTO events (
  account_id, direction, event_type, from_user_id, to_user_id, group_id, message_id, context_token, body_text, media_path, media_file_name, media_mime_type, raw_json, created_at
) VALUES (?, 'inbound', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, ilink.DetectEventType(msg), msg.FromUserID, msg.ToUserID, msg.GroupID, msg.MessageID, msg.ContextToken, bodyText, mediaPath, mediaFileName, mediaMimeType, string(raw), now,
	)
	if err != nil {
		return err
	}

	if stringsNotEmpty(msg.FromUserID, msg.ContextToken) {
		_, err = tx.ExecContext(ctx, `
INSERT INTO peer_contexts (account_id, peer_user_id, context_token, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, peer_user_id) DO UPDATE SET
  context_token = excluded.context_token,
  updated_at = excluded.updated_at`, accountID, msg.FromUserID, msg.ContextToken, now)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `
UPDATE accounts
SET last_inbound_at = ?, updated_at = ?, last_error = '', login_status = 'connected'
WHERE account_id = ?`, now, now, accountID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) GetPeerContext(ctx context.Context, accountID, peerUserID string) (model.PeerContext, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT account_id, peer_user_id, context_token, updated_at
FROM peer_contexts
WHERE account_id = ? AND peer_user_id = ?`, accountID, peerUserID)
	var item model.PeerContext
	if err := row.Scan(&item.AccountID, &item.PeerUserID, &item.ContextToken, &item.UpdatedAt); err != nil {
		return model.PeerContext{}, err
	}
	return item, nil
}

func (s *Store) CreateOutboundEvent(ctx context.Context, accountID, eventType, toUserID, contextToken, bodyText, mediaPath, mediaFileName, mediaMimeType, rawJSON string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO events (
  account_id, direction, event_type, to_user_id, context_token, body_text, media_path, media_file_name, media_mime_type, raw_json, created_at
) VALUES (?, 'outbound', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		accountID, eventType, toUserID, contextToken, bodyText, mediaPath, mediaFileName, mediaMimeType, rawJSON, time.Now().UTC(),
	)
	return err
}

func (s *Store) AddLog(ctx context.Context, level, message, source, metaJSON string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO logs (level, message, source, meta_json, created_at)
VALUES (?, ?, ?, ?, ?)`, level, message, source, metaJSON, time.Now().UTC())
	return err
}

func (s *Store) ListLogs(ctx context.Context, afterID int64, limit int) ([]model.LogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, level, message, source, meta_json, created_at
FROM logs
WHERE id > ?
ORDER BY id ASC
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.LogEntry
	for rows.Next() {
		var item model.LogEntry
		if err := rows.Scan(&item.ID, &item.Level, &item.Message, &item.Source, &item.MetaJSON, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListEvents(ctx context.Context, afterID int64, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, account_id, direction, event_type, from_user_id, to_user_id, group_id, message_id, context_token, body_text, media_path, media_file_name, media_mime_type, raw_json, created_at
FROM events
WHERE id > ?
ORDER BY id ASC
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.Event
	for rows.Next() {
		var item model.Event
		if err := rows.Scan(
			&item.ID, &item.AccountID, &item.Direction, &item.EventType, &item.FromUserID, &item.ToUserID, &item.GroupID,
			&item.MessageID, &item.ContextToken, &item.BodyText, &item.MediaPath, &item.MediaFileName, &item.MediaMimeType, &item.RawJSON, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`CREATE TABLE IF NOT EXISTS login_sessions (
			session_id TEXT PRIMARY KEY,
			base_url TEXT NOT NULL,
			qr_code TEXT NOT NULL,
			qr_code_url TEXT NOT NULL,
			status TEXT NOT NULL,
			account_id TEXT NOT NULL DEFAULT '',
			ilink_user_id TEXT NOT NULL DEFAULT '',
			bot_token TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			started_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			completed_at TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS accounts (
			account_id TEXT PRIMARY KEY,
			base_url TEXT NOT NULL,
			token TEXT NOT NULL,
			ilink_user_id TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			login_status TEXT NOT NULL DEFAULT 'pending',
			last_error TEXT NOT NULL DEFAULT '',
			get_updates_buf TEXT NOT NULL DEFAULT '',
			last_poll_at TIMESTAMP,
			last_inbound_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS peer_contexts (
			account_id TEXT NOT NULL,
			peer_user_id TEXT NOT NULL,
			context_token TEXT NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			PRIMARY KEY (account_id, peer_user_id)
		);`,
		`CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id TEXT NOT NULL,
			direction TEXT NOT NULL,
			event_type TEXT NOT NULL,
			from_user_id TEXT NOT NULL DEFAULT '',
			to_user_id TEXT NOT NULL DEFAULT '',
			group_id TEXT NOT NULL DEFAULT '',
			message_id INTEGER NOT NULL DEFAULT 0,
			context_token TEXT NOT NULL DEFAULT '',
			body_text TEXT NOT NULL DEFAULT '',
			media_path TEXT NOT NULL DEFAULT '',
			media_file_name TEXT NOT NULL DEFAULT '',
			media_mime_type TEXT NOT NULL DEFAULT '',
			raw_json TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_events_account_message_inbound
		 ON events(account_id, direction, message_id)
		 WHERE direction = 'inbound' AND message_id != 0;`,
		`CREATE TABLE IF NOT EXISTS logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			source TEXT NOT NULL,
			meta_json TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS wecom_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			corp_id TEXT NOT NULL,
			corp_secret TEXT NOT NULL DEFAULT '',
			agent_id INTEGER NOT NULL DEFAULT 0,
			callback_token TEXT NOT NULL DEFAULT '',
			callback_aes_key TEXT NOT NULL DEFAULT '',
			enabled INTEGER NOT NULL DEFAULT 1,
			auto_reply INTEGER NOT NULL DEFAULT 0,
			webhook_url TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			last_inbound_at TIMESTAMP,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			UNIQUE(corp_id, agent_id)
		);`,
		`CREATE TABLE IF NOT EXISTS wecom_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			corp_id TEXT NOT NULL,
			agent_id INTEGER NOT NULL DEFAULT 0,
			direction TEXT NOT NULL,
			event_type TEXT NOT NULL,
			from_user TEXT NOT NULL DEFAULT '',
			to_user TEXT NOT NULL DEFAULT '',
			msg_id INTEGER NOT NULL DEFAULT 0,
			body_text TEXT NOT NULL DEFAULT '',
			media_id TEXT NOT NULL DEFAULT '',
			raw_json TEXT NOT NULL,
			created_at TIMESTAMP NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_wecom_events_msg_inbound
		 ON wecom_events(corp_id, agent_id, direction, msg_id)
		 WHERE direction = 'inbound' AND msg_id != 0;`,
	}

	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	for _, stmt := range []string{
		`ALTER TABLE events ADD COLUMN media_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN media_file_name TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN media_mime_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE events ADD COLUMN group_id TEXT NOT NULL DEFAULT ''`,
	} {
		if err := s.execMigrationCompat(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) execMigrationCompat(ctx context.Context, stmt string) error {
	if _, err := s.db.ExecContext(ctx, stmt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

func stringsNotEmpty(values ...string) bool {
	for _, value := range values {
		if value == "" {
			return false
		}
	}
	return true
}

func (s *Store) UpsertWeComAccount(ctx context.Context, account model.WeComAccount) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO wecom_accounts (
  corp_id, corp_secret, agent_id, callback_token, callback_aes_key,
  enabled, auto_reply, webhook_url, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?, ?)
ON CONFLICT(corp_id, agent_id) DO UPDATE SET
  corp_secret = excluded.corp_secret,
  callback_token = excluded.callback_token,
  callback_aes_key = excluded.callback_aes_key,
  enabled = excluded.enabled,
  auto_reply = excluded.auto_reply,
  webhook_url = excluded.webhook_url,
  updated_at = excluded.updated_at`,
		account.CorpID, account.CorpSecret, account.AgentID,
		account.CallbackToken, account.CallbackAESKey,
		boolToInt(account.Enabled), boolToInt(account.AutoReply),
		account.WebhookURL, now, now,
	)
	return err
}

func (s *Store) ListWeComAccounts(ctx context.Context) ([]model.WeComAccount, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, corp_id, corp_secret, agent_id, callback_token, callback_aes_key,
       enabled, auto_reply, webhook_url, last_error, last_inbound_at, created_at, updated_at
FROM wecom_accounts
ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.WeComAccount
	for rows.Next() {
		var item model.WeComAccount
		var enabled, autoReply int
		var lastInboundAt sql.NullTime
		if err := rows.Scan(
			&item.ID, &item.CorpID, &item.CorpSecret, &item.AgentID,
			&item.CallbackToken, &item.CallbackAESKey,
			&enabled, &autoReply, &item.WebhookURL,
			&item.LastError, &lastInboundAt, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		item.AutoReply = autoReply == 1
		if lastInboundAt.Valid {
			item.LastInboundAt = &lastInboundAt.Time
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetWeComAccount(ctx context.Context, corpID string, agentID int) (model.WeComAccount, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, corp_id, corp_secret, agent_id, callback_token, callback_aes_key,
       enabled, auto_reply, webhook_url, last_error, last_inbound_at, created_at, updated_at
FROM wecom_accounts
WHERE corp_id = ? AND agent_id = ?`, corpID, agentID)
	var item model.WeComAccount
	var enabled, autoReply int
	var lastInboundAt sql.NullTime
	err := row.Scan(
		&item.ID, &item.CorpID, &item.CorpSecret, &item.AgentID,
		&item.CallbackToken, &item.CallbackAESKey,
		&enabled, &autoReply, &item.WebhookURL,
		&item.LastError, &lastInboundAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return model.WeComAccount{}, err
	}
	item.Enabled = enabled == 1
	item.AutoReply = autoReply == 1
	if lastInboundAt.Valid {
		item.LastInboundAt = &lastInboundAt.Time
	}
	return item, nil
}

func (s *Store) DeleteWeComAccount(ctx context.Context, corpID string, agentID int) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM wecom_accounts WHERE corp_id = ? AND agent_id = ?`, corpID, agentID)
	return err
}

func (s *Store) TouchWeComAccountInbound(ctx context.Context, corpID string, agentID int) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE wecom_accounts SET last_inbound_at = ?, last_error = '', updated_at = ?
WHERE corp_id = ? AND agent_id = ?`, now, now, corpID, agentID)
	return err
}

func (s *Store) UpdateWeComAccountError(ctx context.Context, corpID string, agentID int, lastError string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `
UPDATE wecom_accounts SET last_error = ?, updated_at = ?
WHERE corp_id = ? AND agent_id = ?`, lastError, now, corpID, agentID)
	return err
}

func (s *Store) SaveWeComEvent(ctx context.Context, corpID string, agentID int, direction, eventType, fromUser, toUser string, msgID int64, bodyText, mediaID, rawJSON string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO wecom_events (
  corp_id, agent_id, direction, event_type, from_user, to_user, msg_id, body_text, media_id, raw_json, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		corpID, agentID, direction, eventType, fromUser, toUser, msgID, bodyText, mediaID, rawJSON, time.Now().UTC(),
	)
	return err
}

func (s *Store) ListWeComEvents(ctx context.Context, afterID int64, limit int) ([]model.WeComEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, corp_id, agent_id, direction, event_type, from_user, to_user, msg_id, body_text, media_id, raw_json, created_at
FROM wecom_events
WHERE id > ?
ORDER BY id ASC
LIMIT ?`, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []model.WeComEvent
	for rows.Next() {
		var item model.WeComEvent
		if err := rows.Scan(
			&item.ID, &item.CorpID, &item.AgentID, &item.Direction, &item.EventType,
			&item.FromUser, &item.ToUser, &item.MsgID, &item.BodyText, &item.MediaID, &item.RawJSON, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

var ErrNotFound = errors.New("not found")

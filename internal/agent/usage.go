package agent

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TokenUsageRecord struct {
	ID               int64     `json:"id"`
	ConversationID   string    `json:"conversation_id"`
	UserID           string    `json:"user_id"`
	ChannelType      string    `json:"channel_type"`
	Mode             string    `json:"mode"`
	Model            string    `json:"model"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	TotalTokens      int       `json:"total_tokens"`
	CreatedAt        time.Time `json:"created_at"`
}

type UsageSummary struct {
	UserID          string `json:"user_id"`
	TotalPrompt     int64  `json:"total_prompt_tokens"`
	TotalCompletion int64  `json:"total_completion_tokens"`
	TotalTokens     int64  `json:"total_tokens"`
	RequestCount    int64  `json:"request_count"`
}

type UsageStore struct {
	db *sql.DB
}

func NewUsageStore(db *sql.DB) *UsageStore {
	return &UsageStore{db: db}
}

func (s *UsageStore) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS token_usage (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id   TEXT NOT NULL,
			user_id           TEXT NOT NULL,
			channel_type      TEXT NOT NULL,
			mode              TEXT NOT NULL DEFAULT '',
			model             TEXT NOT NULL DEFAULT '',
			prompt_tokens     INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens      INTEGER NOT NULL DEFAULT 0,
			created_at        TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_user
		 ON token_usage(user_id, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_token_usage_conv
		 ON token_usage(conversation_id);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("token_usage migrate: %w", err)
		}
	}
	return nil
}

func (s *UsageStore) Record(ctx context.Context, rec TokenUsageRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO token_usage (conversation_id, user_id, channel_type, mode, model, prompt_tokens, completion_tokens, total_tokens, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ConversationID, rec.UserID, rec.ChannelType, rec.Mode, rec.Model,
		rec.PromptTokens, rec.CompletionTokens, rec.TotalTokens, rec.CreatedAt)
	return err
}

func (s *UsageStore) GetUserUsageSince(ctx context.Context, userID string, since time.Time) (UsageSummary, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0), COUNT(*)
		 FROM token_usage WHERE user_id = ? AND created_at >= ?`, userID, since)
	var us UsageSummary
	us.UserID = userID
	err := row.Scan(&us.TotalPrompt, &us.TotalCompletion, &us.TotalTokens, &us.RequestCount)
	return us, err
}

func (s *UsageStore) GetUserDailyUsage(ctx context.Context, userID string) (UsageSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	return s.GetUserUsageSince(ctx, userID, start)
}

func (s *UsageStore) GetUserMonthlyUsage(ctx context.Context, userID string) (UsageSummary, error) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.GetUserUsageSince(ctx, userID, start)
}

func (s *UsageStore) GetAllUsageSince(ctx context.Context, since time.Time) ([]UsageSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT user_id, COALESCE(SUM(prompt_tokens),0), COALESCE(SUM(completion_tokens),0), COALESCE(SUM(total_tokens),0), COUNT(*)
		 FROM token_usage WHERE created_at >= ?
		 GROUP BY user_id ORDER BY SUM(total_tokens) DESC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []UsageSummary
	for rows.Next() {
		var us UsageSummary
		if err := rows.Scan(&us.UserID, &us.TotalPrompt, &us.TotalCompletion, &us.TotalTokens, &us.RequestCount); err != nil {
			return nil, err
		}
		items = append(items, us)
	}
	return items, rows.Err()
}

func (s *UsageStore) CheckLimit(ctx context.Context, userID string, dailyLimit, monthlyLimit int64) (bool, string) {
	if dailyLimit > 0 {
		daily, err := s.GetUserDailyUsage(ctx, userID)
		if err == nil && daily.TotalTokens >= dailyLimit {
			return false, fmt.Sprintf("今日 token 用量已达上限（%d/%d），请明天再试。", daily.TotalTokens, dailyLimit)
		}
	}
	if monthlyLimit > 0 {
		monthly, err := s.GetUserMonthlyUsage(ctx, userID)
		if err == nil && monthly.TotalTokens >= monthlyLimit {
			return false, fmt.Sprintf("本月 token 用量已达上限（%d/%d），请下月再试。", monthly.TotalTokens, monthlyLimit)
		}
	}
	return true, ""
}

func (s *UsageStore) CleanOldRecords(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan)
	result, err := s.db.ExecContext(ctx, `DELETE FROM token_usage WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

package agent

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CustomMode struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	Name           string    `json:"name"`
	SystemPrompt   string    `json:"system_prompt"`
	AvailableTools string    `json:"available_tools"` // comma-separated
	WelcomeMessage string    `json:"welcome_message"`
	LLMProviderID  string    `json:"llm_provider_id,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (cm CustomMode) ToolList() []string {
	if cm.AvailableTools == "" {
		return []string{"web_search", "url_content_fetch"}
	}
	parts := strings.Split(cm.AvailableTools, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

type LLMProvider struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	BaseURL     string   `json:"base_url"`
	APIKey      string   `json:"api_key,omitempty"`
	Model       string   `json:"model"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CustomModeStore struct {
	db *sql.DB
}

func NewCustomModeStore(db *sql.DB) *CustomModeStore {
	return &CustomModeStore{db: db}
}

func (s *CustomModeStore) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS custom_modes (
			id              TEXT PRIMARY KEY,
			slug            TEXT NOT NULL UNIQUE,
			name            TEXT NOT NULL,
			system_prompt   TEXT NOT NULL DEFAULT '',
			available_tools TEXT NOT NULL DEFAULT '',
			welcome_message TEXT NOT NULL DEFAULT '',
			llm_provider_id TEXT NOT NULL DEFAULT '',
			created_at      TIMESTAMP NOT NULL,
			updated_at      TIMESTAMP NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS llm_providers (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL,
			base_url    TEXT NOT NULL,
			api_key     TEXT NOT NULL DEFAULT '',
			model       TEXT NOT NULL,
			temperature REAL,
			max_tokens  INTEGER NOT NULL DEFAULT 0,
			created_at  TIMESTAMP NOT NULL,
			updated_at  TIMESTAMP NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("custom_mode migrate: %w", err)
		}
	}
	return nil
}

// Custom Mode CRUD

func (s *CustomModeStore) CreateMode(ctx context.Context, cm CustomMode) (CustomMode, error) {
	now := time.Now().UTC()
	cm.ID = uuid.New().String()
	cm.CreatedAt = now
	cm.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO custom_modes (id, slug, name, system_prompt, available_tools, welcome_message, llm_provider_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cm.ID, cm.Slug, cm.Name, cm.SystemPrompt, cm.AvailableTools, cm.WelcomeMessage, cm.LLMProviderID, now, now)
	if err != nil {
		return CustomMode{}, fmt.Errorf("create custom mode: %w", err)
	}
	return cm, nil
}

func (s *CustomModeStore) GetMode(ctx context.Context, id string) (CustomMode, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, system_prompt, available_tools, welcome_message, llm_provider_id, created_at, updated_at
		 FROM custom_modes WHERE id = ?`, id)
	return scanCustomMode(row)
}

func (s *CustomModeStore) GetModeBySlug(ctx context.Context, slug string) (CustomMode, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, slug, name, system_prompt, available_tools, welcome_message, llm_provider_id, created_at, updated_at
		 FROM custom_modes WHERE slug = ?`, slug)
	return scanCustomMode(row)
}

func (s *CustomModeStore) ListModes(ctx context.Context) ([]CustomMode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, slug, name, system_prompt, available_tools, welcome_message, llm_provider_id, created_at, updated_at
		 FROM custom_modes ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CustomMode
	for rows.Next() {
		var cm CustomMode
		if err := rows.Scan(&cm.ID, &cm.Slug, &cm.Name, &cm.SystemPrompt, &cm.AvailableTools, &cm.WelcomeMessage, &cm.LLMProviderID, &cm.CreatedAt, &cm.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, cm)
	}
	return items, rows.Err()
}

func (s *CustomModeStore) UpdateMode(ctx context.Context, cm CustomMode) error {
	cm.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE custom_modes SET name=?, system_prompt=?, available_tools=?, welcome_message=?, llm_provider_id=?, updated_at=? WHERE id=?`,
		cm.Name, cm.SystemPrompt, cm.AvailableTools, cm.WelcomeMessage, cm.LLMProviderID, cm.UpdatedAt, cm.ID)
	return err
}

func (s *CustomModeStore) DeleteMode(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM custom_modes WHERE id = ?`, id)
	return err
}

// LLM Provider CRUD

func (s *CustomModeStore) CreateProvider(ctx context.Context, p LLMProvider) (LLMProvider, error) {
	now := time.Now().UTC()
	p.ID = uuid.New().String()
	p.CreatedAt = now
	p.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO llm_providers (id, name, base_url, api_key, model, temperature, max_tokens, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.BaseURL, p.APIKey, p.Model, p.Temperature, p.MaxTokens, now, now)
	if err != nil {
		return LLMProvider{}, fmt.Errorf("create llm provider: %w", err)
	}
	return p, nil
}

func (s *CustomModeStore) GetProvider(ctx context.Context, id string) (LLMProvider, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, base_url, api_key, model, temperature, max_tokens, created_at, updated_at
		 FROM llm_providers WHERE id = ?`, id)
	return scanLLMProvider(row)
}

func (s *CustomModeStore) ListProviders(ctx context.Context) ([]LLMProvider, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, base_url, api_key, model, temperature, max_tokens, created_at, updated_at
		 FROM llm_providers ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LLMProvider
	for rows.Next() {
		p, err := scanLLMProviderRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (s *CustomModeStore) UpdateProvider(ctx context.Context, p LLMProvider) error {
	p.UpdatedAt = time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE llm_providers SET name=?, base_url=?, api_key=?, model=?, temperature=?, max_tokens=?, updated_at=? WHERE id=?`,
		p.Name, p.BaseURL, p.APIKey, p.Model, p.Temperature, p.MaxTokens, p.UpdatedAt, p.ID)
	return err
}

func (s *CustomModeStore) DeleteProvider(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM llm_providers WHERE id = ?`, id)
	return err
}

func scanCustomMode(row *sql.Row) (CustomMode, error) {
	var cm CustomMode
	err := row.Scan(&cm.ID, &cm.Slug, &cm.Name, &cm.SystemPrompt, &cm.AvailableTools, &cm.WelcomeMessage, &cm.LLMProviderID, &cm.CreatedAt, &cm.UpdatedAt)
	return cm, err
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanLLMProvider(row *sql.Row) (LLMProvider, error) {
	var p LLMProvider
	var temp sql.NullFloat64
	err := row.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.Model, &temp, &p.MaxTokens, &p.CreatedAt, &p.UpdatedAt)
	if temp.Valid {
		p.Temperature = &temp.Float64
	}
	return p, err
}

func scanLLMProviderRow(rows *sql.Rows) (LLMProvider, error) {
	var p LLMProvider
	var temp sql.NullFloat64
	err := rows.Scan(&p.ID, &p.Name, &p.BaseURL, &p.APIKey, &p.Model, &temp, &p.MaxTokens, &p.CreatedAt, &p.UpdatedAt)
	if temp.Valid {
		p.Temperature = &temp.Float64
	}
	return p, err
}

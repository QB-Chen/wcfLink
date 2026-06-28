package support

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS kb_articles (
			id         TEXT PRIMARY KEY,
			category   TEXT NOT NULL DEFAULT '',
			question   TEXT NOT NULL,
			answer     TEXT NOT NULL,
			tags       TEXT NOT NULL DEFAULT '',
			priority   INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_kb_category ON kb_articles(category);`,

		`CREATE TABLE IF NOT EXISTS tickets (
			id          TEXT PRIMARY KEY,
			customer_id TEXT NOT NULL DEFAULT '',
			channel     TEXT NOT NULL DEFAULT '',
			subject     TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status      TEXT NOT NULL DEFAULT 'open',
			priority    TEXT NOT NULL DEFAULT 'medium',
			category    TEXT NOT NULL DEFAULT '',
			assignee    TEXT NOT NULL DEFAULT '',
			notes       TEXT NOT NULL DEFAULT '',
			created_at  TIMESTAMP NOT NULL,
			updated_at  TIMESTAMP NOT NULL,
			closed_at   TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_ticket_status ON tickets(status);`,
		`CREATE INDEX IF NOT EXISTS idx_ticket_customer ON tickets(customer_id);`,

		`CREATE TABLE IF NOT EXISTS orders (
			id           TEXT PRIMARY KEY,
			customer_id  TEXT NOT NULL DEFAULT '',
			product      TEXT NOT NULL DEFAULT '',
			amount       REAL NOT NULL DEFAULT 0,
			currency     TEXT NOT NULL DEFAULT 'CNY',
			status       TEXT NOT NULL DEFAULT 'pending',
			payment_method TEXT NOT NULL DEFAULT '',
			notes        TEXT NOT NULL DEFAULT '',
			created_at   TIMESTAMP NOT NULL,
			updated_at   TIMESTAMP NOT NULL,
			refund_amount REAL NOT NULL DEFAULT 0,
			refund_reason TEXT NOT NULL DEFAULT '',
			refunded_at  TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_order_customer ON orders(customer_id);`,
		`CREATE INDEX IF NOT EXISTS idx_order_status ON orders(status);`,

		`CREATE TABLE IF NOT EXISTS support_profiles (
			id           TEXT PRIMARY KEY,
			name         TEXT NOT NULL UNIQUE,
			company_name TEXT NOT NULL DEFAULT '',
			industry     TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			greeting     TEXT NOT NULL DEFAULT '',
			escalation_threshold REAL NOT NULL DEFAULT 500,
			refund_policy TEXT NOT NULL DEFAULT '',
			business_hours TEXT NOT NULL DEFAULT '',
			extra_config TEXT NOT NULL DEFAULT '',
			is_default   INTEGER NOT NULL DEFAULT 0,
			created_at   TIMESTAMP NOT NULL,
			updated_at   TIMESTAMP NOT NULL
		);`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("support migrate: %w", err)
		}
	}
	return nil
}

// --- Knowledge Base ---

type KBArticle struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Tags      string    `json:"tags"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *Store) KBAdd(ctx context.Context, art KBArticle) (KBArticle, error) {
	now := time.Now().UTC()
	art.ID = uuid.New().String()
	art.CreatedAt = now
	art.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kb_articles (id, category, question, answer, tags, priority, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		art.ID, art.Category, art.Question, art.Answer, art.Tags, art.Priority, now, now)
	return art, err
}

func (s *Store) KBSearch(ctx context.Context, query string, category string, limit int) ([]KBArticle, error) {
	if limit <= 0 {
		limit = 10
	}
	q := "%" + escapeLike(query) + "%"
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, category, question, answer, tags, priority, created_at, updated_at
			 FROM kb_articles
			 WHERE category = ? AND (question LIKE ? OR answer LIKE ? OR tags LIKE ?)
			 ORDER BY priority DESC, updated_at DESC
			 LIMIT ?`,
			category, q, q, q, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, category, question, answer, tags, priority, created_at, updated_at
			 FROM kb_articles
			 WHERE question LIKE ? OR answer LIKE ? OR tags LIKE ? OR category LIKE ?
			 ORDER BY priority DESC, updated_at DESC
			 LIMIT ?`,
			q, q, q, q, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKBArticles(rows)
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

func (s *Store) KBList(ctx context.Context, category string, limit int) ([]KBArticle, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, category, question, answer, tags, priority, created_at, updated_at
			 FROM kb_articles WHERE category = ? ORDER BY priority DESC, updated_at DESC LIMIT ?`,
			category, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, category, question, answer, tags, priority, created_at, updated_at
			 FROM kb_articles ORDER BY priority DESC, updated_at DESC LIMIT ?`,
			limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanKBArticles(rows)
}

func (s *Store) KBGet(ctx context.Context, id string) (KBArticle, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, category, question, answer, tags, priority, created_at, updated_at
		 FROM kb_articles WHERE id = ?`, id)
	var a KBArticle
	err := row.Scan(&a.ID, &a.Category, &a.Question, &a.Answer, &a.Tags, &a.Priority, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (s *Store) KBUpdate(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []interface{}
	for k, v := range updates {
		switch k {
		case "category", "question", "answer", "tags":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		case "priority":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC(), id)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE kb_articles SET %s WHERE id = ?`, strings.Join(setClauses, ", ")), args...)
	return err
}

func (s *Store) KBDelete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM kb_articles WHERE id = ?`, id)
	return err
}

func scanKBArticles(rows *sql.Rows) ([]KBArticle, error) {
	var items []KBArticle
	for rows.Next() {
		var a KBArticle
		if err := rows.Scan(&a.ID, &a.Category, &a.Question, &a.Answer, &a.Tags, &a.Priority, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

// --- Tickets ---

type Ticket struct {
	ID          string     `json:"id"`
	CustomerID  string     `json:"customer_id"`
	Channel     string     `json:"channel"`
	Subject     string     `json:"subject"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	Category    string     `json:"category"`
	Assignee    string     `json:"assignee"`
	Notes       string     `json:"notes"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
}

func (s *Store) TicketCreate(ctx context.Context, t Ticket) (Ticket, error) {
	now := time.Now().UTC()
	t.ID = uuid.New().String()
	if t.Status == "" {
		t.Status = "open"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	t.CreatedAt = now
	t.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tickets (id, customer_id, channel, subject, description, status, priority, category, assignee, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.CustomerID, t.Channel, t.Subject, t.Description, t.Status, t.Priority, t.Category, t.Assignee, t.Notes, now, now)
	return t, err
}

func (s *Store) TicketGet(ctx context.Context, id string) (Ticket, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, channel, subject, description, status, priority, category, assignee, notes, created_at, updated_at, closed_at
		 FROM tickets WHERE id = ?`, id)
	return scanTicket(row)
}

func (s *Store) TicketQuery(ctx context.Context, filters map[string]string, limit int) ([]Ticket, error) {
	if limit <= 0 {
		limit = 20
	}
	var wheres []string
	var args []interface{}
	for k, v := range filters {
		switch k {
		case "status", "priority", "category", "customer_id", "assignee", "channel":
			wheres = append(wheres, k+" = ?")
			args = append(args, v)
		}
	}
	query := `SELECT id, customer_id, channel, subject, description, status, priority, category, assignee, notes, created_at, updated_at, closed_at FROM tickets`
	if len(wheres) > 0 {
		query += " WHERE " + strings.Join(wheres, " AND ")
	}
	query += " ORDER BY updated_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTickets(rows)
}

func (s *Store) TicketUpdate(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []interface{}
	for k, v := range updates {
		switch k {
		case "status", "priority", "category", "assignee", "notes", "subject", "description":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}
	if status, ok := updates["status"]; ok && status == "closed" {
		setClauses = append(setClauses, "closed_at = ?")
		args = append(args, time.Now().UTC())
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC(), id)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE tickets SET %s WHERE id = ?`, strings.Join(setClauses, ", ")), args...)
	return err
}

func scanTicket(row *sql.Row) (Ticket, error) {
	var t Ticket
	err := row.Scan(&t.ID, &t.CustomerID, &t.Channel, &t.Subject, &t.Description,
		&t.Status, &t.Priority, &t.Category, &t.Assignee, &t.Notes,
		&t.CreatedAt, &t.UpdatedAt, &t.ClosedAt)
	return t, err
}

func scanTickets(rows *sql.Rows) ([]Ticket, error) {
	var items []Ticket
	for rows.Next() {
		var t Ticket
		if err := rows.Scan(&t.ID, &t.CustomerID, &t.Channel, &t.Subject, &t.Description,
			&t.Status, &t.Priority, &t.Category, &t.Assignee, &t.Notes,
			&t.CreatedAt, &t.UpdatedAt, &t.ClosedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// --- Orders ---

type Order struct {
	ID            string     `json:"id"`
	CustomerID    string     `json:"customer_id"`
	Product       string     `json:"product"`
	Amount        float64    `json:"amount"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	PaymentMethod string     `json:"payment_method"`
	Notes         string     `json:"notes"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	RefundAmount  float64    `json:"refund_amount"`
	RefundReason  string     `json:"refund_reason"`
	RefundedAt    *time.Time `json:"refunded_at,omitempty"`
}

func (s *Store) OrderCreate(ctx context.Context, o Order) (Order, error) {
	now := time.Now().UTC()
	o.ID = uuid.New().String()
	if o.Status == "" {
		o.Status = "pending"
	}
	if o.Currency == "" {
		o.Currency = "CNY"
	}
	o.CreatedAt = now
	o.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO orders (id, customer_id, product, amount, currency, status, payment_method, notes, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.CustomerID, o.Product, o.Amount, o.Currency, o.Status, o.PaymentMethod, o.Notes, now, now)
	return o, err
}

func (s *Store) OrderGet(ctx context.Context, id string) (Order, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, customer_id, product, amount, currency, status, payment_method, notes, created_at, updated_at, refund_amount, refund_reason, refunded_at
		 FROM orders WHERE id = ?`, id)
	return scanOrder(row)
}

func (s *Store) OrderQuery(ctx context.Context, filters map[string]string, limit int) ([]Order, error) {
	if limit <= 0 {
		limit = 20
	}
	var wheres []string
	var args []interface{}
	for k, v := range filters {
		switch k {
		case "customer_id", "status", "product":
			wheres = append(wheres, k+" = ?")
			args = append(args, v)
		}
	}
	query := `SELECT id, customer_id, product, amount, currency, status, payment_method, notes, created_at, updated_at, refund_amount, refund_reason, refunded_at FROM orders`
	if len(wheres) > 0 {
		query += " WHERE " + strings.Join(wheres, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (s *Store) OrderRefund(ctx context.Context, id string, amount float64, reason string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE orders SET status = 'refunded', refund_amount = ?, refund_reason = ?, refunded_at = ?, updated_at = ? WHERE id = ?`,
		amount, reason, now, now, id)
	return err
}

func scanOrder(row *sql.Row) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.CustomerID, &o.Product, &o.Amount, &o.Currency,
		&o.Status, &o.PaymentMethod, &o.Notes,
		&o.CreatedAt, &o.UpdatedAt, &o.RefundAmount, &o.RefundReason, &o.RefundedAt)
	return o, err
}

func scanOrders(rows *sql.Rows) ([]Order, error) {
	var items []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.Product, &o.Amount, &o.Currency,
			&o.Status, &o.PaymentMethod, &o.Notes,
			&o.CreatedAt, &o.UpdatedAt, &o.RefundAmount, &o.RefundReason, &o.RefundedAt); err != nil {
			return nil, err
		}
		items = append(items, o)
	}
	return items, rows.Err()
}

// --- Support Profiles ---

type Profile struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	CompanyName         string    `json:"company_name"`
	Industry            string    `json:"industry"`
	SystemPrompt        string    `json:"system_prompt"`
	Greeting            string    `json:"greeting"`
	EscalationThreshold float64   `json:"escalation_threshold"`
	RefundPolicy        string    `json:"refund_policy"`
	BusinessHours       string    `json:"business_hours"`
	ExtraConfig         string    `json:"extra_config"`
	IsDefault           bool      `json:"is_default"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (s *Store) ProfileCreate(ctx context.Context, p Profile) (Profile, error) {
	now := time.Now().UTC()
	p.ID = uuid.New().String()
	p.CreatedAt = now
	p.UpdatedAt = now
	isDefault := 0
	if p.IsDefault {
		isDefault = 1
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Profile{}, err
	}
	defer tx.Rollback()

	if p.IsDefault {
		if _, err := tx.ExecContext(ctx, `UPDATE support_profiles SET is_default = 0`); err != nil {
			return Profile{}, err
		}
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO support_profiles (id, name, company_name, industry, system_prompt, greeting, escalation_threshold, refund_policy, business_hours, extra_config, is_default, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.CompanyName, p.Industry, p.SystemPrompt, p.Greeting,
		p.EscalationThreshold, p.RefundPolicy, p.BusinessHours, p.ExtraConfig, isDefault, now, now)
	if err != nil {
		return Profile{}, err
	}
	if err := tx.Commit(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

func (s *Store) ProfileGet(ctx context.Context, id string) (Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, company_name, industry, system_prompt, greeting, escalation_threshold, refund_policy, business_hours, extra_config, is_default, created_at, updated_at
		 FROM support_profiles WHERE id = ?`, id)
	return scanProfile(row)
}

func (s *Store) ProfileGetByName(ctx context.Context, name string) (Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, company_name, industry, system_prompt, greeting, escalation_threshold, refund_policy, business_hours, extra_config, is_default, created_at, updated_at
		 FROM support_profiles WHERE name = ?`, name)
	return scanProfile(row)
}

func (s *Store) ProfileGetDefault(ctx context.Context) (Profile, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, company_name, industry, system_prompt, greeting, escalation_threshold, refund_policy, business_hours, extra_config, is_default, created_at, updated_at
		 FROM support_profiles WHERE is_default = 1 LIMIT 1`)
	return scanProfile(row)
}

func (s *Store) ProfileList(ctx context.Context) ([]Profile, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, company_name, industry, system_prompt, greeting, escalation_threshold, refund_policy, business_hours, extra_config, is_default, created_at, updated_at
		 FROM support_profiles ORDER BY is_default DESC, updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProfiles(rows)
}

func (s *Store) ProfileSetDefault(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE support_profiles SET is_default = 0`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE support_profiles SET is_default = 1, updated_at = ? WHERE id = ?`, time.Now().UTC(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ProfileDelete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM support_profiles WHERE id = ?`, id)
	return err
}

func (s *Store) ProfileUpdate(ctx context.Context, id string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}
	var setClauses []string
	var args []interface{}
	for k, v := range updates {
		switch k {
		case "name", "company_name", "industry", "system_prompt", "greeting", "refund_policy", "business_hours", "extra_config":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		case "escalation_threshold":
			setClauses = append(setClauses, k+" = ?")
			args = append(args, v)
		}
	}
	if len(setClauses) == 0 {
		return nil
	}
	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC(), id)
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE support_profiles SET %s WHERE id = ?`, strings.Join(setClauses, ", ")), args...)
	return err
}

func scanProfile(row *sql.Row) (Profile, error) {
	var p Profile
	var isDefault int
	err := row.Scan(&p.ID, &p.Name, &p.CompanyName, &p.Industry, &p.SystemPrompt, &p.Greeting,
		&p.EscalationThreshold, &p.RefundPolicy, &p.BusinessHours, &p.ExtraConfig,
		&isDefault, &p.CreatedAt, &p.UpdatedAt)
	p.IsDefault = isDefault == 1
	return p, err
}

func scanProfiles(rows *sql.Rows) ([]Profile, error) {
	var items []Profile
	for rows.Next() {
		var p Profile
		var isDefault int
		if err := rows.Scan(&p.ID, &p.Name, &p.CompanyName, &p.Industry, &p.SystemPrompt, &p.Greeting,
			&p.EscalationThreshold, &p.RefundPolicy, &p.BusinessHours, &p.ExtraConfig,
			&isDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		p.IsDefault = isDefault == 1
		items = append(items, p)
	}
	return items, rows.Err()
}

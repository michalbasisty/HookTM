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
)

const (
	// DefaultLimit is the default number of results to return.
	DefaultLimit = 20
	// MaxLimit is the maximum number of results allowed.
	MaxLimit = 500
	// BusyTimeoutMs is the SQLite busy timeout in milliseconds.
	BusyTimeoutMs = 5000
)

type Store struct {
	path string
	db   *sql.DB
}

func Open(path string) (*Store, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("empty db path")
	}
	var dsn string
	if path == ":memory:" {
		// WAL doesn't make sense for in-memory DB; use MEMORY journal.
		dsn = fmt.Sprintf("file::memory:?cache=shared&_pragma=foreign_keys(ON)&_pragma=journal_mode(MEMORY)&_pragma=busy_timeout(%d)", BusyTimeoutMs)
	} else {
		if err := ensureDir(filepath.Dir(path)); err != nil {
			return nil, err
		}
		dsn = fmt.Sprintf("file:%s?_pragma=foreign_keys(ON)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(%d)", path, BusyTimeoutMs)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // keep it simple & WAL-friendly for MVP
	s := &Store{path: path, db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string { return s.path }

type Webhook struct {
	ID        string `json:"id"`
	CreatedAt int64  `json:"created_at"`

	Method  string              `json:"method"`
	Path    string              `json:"path"`
	Query   string              `json:"query,omitempty"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`

	Provider  string `json:"provider,omitempty"`
	EventType string `json:"event_type,omitempty"`
	Signature string `json:"signature,omitempty"`

	StatusCode *int  `json:"status_code,omitempty"`
	ResponseMS int64 `json:"response_ms"`

	BodyText string `json:"body_text,omitempty"`
}

type WebhookSummary struct {
	ID         string `json:"id"`
	CreatedAt  int64  `json:"created_at"`
	Method     string `json:"method"`
	Path       string `json:"path"`
	Provider   string `json:"provider,omitempty"`
	EventType  string `json:"event_type,omitempty"`
	StatusCode *int   `json:"status_code,omitempty"`
	ResponseMS int64  `json:"response_ms"`
}

type InsertParams struct {
	ID        string
	CreatedAt int64

	Method  string
	Path    string
	Query   string
	Headers map[string][]string
	Body    []byte

	Provider  string
	EventType string
	Signature string

	StatusCode *int
	ResponseMS int64
	BodyText   string
}

func (s *Store) InsertWebhook(ctx context.Context, p InsertParams) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("missing id")
	}
	if p.CreatedAt == 0 {
		p.CreatedAt = time.Now().UnixMilli()
	}
	if strings.TrimSpace(p.Method) == "" || strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("missing method/path")
	}
	hb, err := json.Marshal(p.Headers)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO webhooks (
  id, created_at,
  method, path, query, headers, body,
  provider, event_type, signature,
  status_code, response_ms,
  body_text
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, p.ID, p.CreatedAt, p.Method, p.Path, nullIfEmpty(p.Query), string(hb), p.Body,
		nullIfEmpty(p.Provider), nullIfEmpty(p.EventType), nullIfEmpty(p.Signature),
		p.StatusCode, p.ResponseMS, nullIfEmpty(p.BodyText),
	)
	return err
}

type ListFilter struct {
	Limit      int
	Provider   string
	StatusCode *int
	From       *time.Time // Inclusive start date
	To         *time.Time // Inclusive end date
}

func (s *Store) ListSummaries(ctx context.Context, f ListFilter) ([]WebhookSummary, error) {
	limit := f.Limit
	if limit <= 0 || limit > MaxLimit {
		limit = DefaultLimit
	}
	var (
		wheres []string
		args   []any
	)
	if strings.TrimSpace(f.Provider) != "" {
		wheres = append(wheres, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.StatusCode != nil {
		wheres = append(wheres, "status_code = ?")
		args = append(args, *f.StatusCode)
	}
	if f.From != nil {
		wheres = append(wheres, "created_at >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if f.To != nil {
		wheres = append(wheres, "created_at <= ?")
		args = append(args, f.To.UnixMilli())
	}
	whereSQL := ""
	if len(wheres) > 0 {
		whereSQL = "WHERE " + strings.Join(wheres, " AND ")
	}

	q := fmt.Sprintf(`
SELECT id, created_at, method, path, provider, event_type, status_code, response_ms
FROM webhooks
%s
ORDER BY created_at DESC
LIMIT ?
`, whereSQL)
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WebhookSummary
	for rows.Next() {
		var r WebhookSummary
		var prov, ev sql.NullString
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.Method, &r.Path, &prov, &ev, &r.StatusCode, &r.ResponseMS); err != nil {
			return nil, err
		}
		r.Provider = prov.String
		r.EventType = ev.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetWebhook(ctx context.Context, id string) (Webhook, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Webhook{}, fmt.Errorf("empty id")
	}
	var (
		hJSON string
		wh    Webhook
		qry   sql.NullString
		prov  sql.NullString
		ev    sql.NullString
		sig   sql.NullString
		bt    sql.NullString
	)
	err := s.db.QueryRowContext(ctx, `
SELECT
  id, created_at,
  method, path, query, headers, body,
  provider, event_type, signature,
  status_code, response_ms,
  body_text
FROM webhooks
WHERE id = ?
`, id).Scan(
		&wh.ID, &wh.CreatedAt,
		&wh.Method, &wh.Path, &qry, &hJSON, &wh.Body,
		&prov, &ev, &sig,
		&wh.StatusCode, &wh.ResponseMS,
		&bt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Webhook{}, fmt.Errorf("not found: %s", id)
	}
	if err != nil {
		return Webhook{}, err
	}
	wh.Query = qry.String
	wh.Provider = prov.String
	wh.EventType = ev.String
	wh.Signature = sig.String
	wh.BodyText = bt.String
	if err := json.Unmarshal([]byte(hJSON), &wh.Headers); err != nil {
		// Don't fail hard on corrupt headers; keep usable.
		wh.Headers = map[string][]string{"_error": {err.Error()}}
	}
	return wh, nil
}

func (s *Store) SearchSummaries(ctx context.Context, query string, limit int) ([]WebhookSummary, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	if limit <= 0 || limit > MaxLimit {
		limit = DefaultLimit
	}
	// Sanitize FTS5 query to prevent injection of special operators.
	// Wrap each word in double quotes to treat as literal text.
	query = sanitizeFTSQuery(query)
	rows, err := s.db.QueryContext(ctx, `
SELECT w.id, w.created_at, w.method, w.path, w.provider, w.event_type, w.status_code, w.response_ms
FROM webhooks_fts f
JOIN webhooks w ON w.rowid = f.rowid
WHERE webhooks_fts MATCH ?
ORDER BY w.created_at DESC
LIMIT ?
`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WebhookSummary
	for rows.Next() {
		var r WebhookSummary
		var prov, ev sql.NullString
		if err := rows.Scan(&r.ID, &r.CreatedAt, &r.Method, &r.Path, &prov, &ev, &r.StatusCode, &r.ResponseMS); err != nil {
			return nil, err
		}
		r.Provider = prov.String
		r.EventType = ev.String
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) DeleteWebhook(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("empty id")
	}
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("not found: %s", id)
	}
	return nil
}

type DeleteFilter struct {
	OlderThan  time.Duration
	Provider   string
	StatusCode *int
}

func (s *Store) DeleteByFilter(ctx context.Context, f DeleteFilter) (int64, error) {
	var (
		wheres []string
		args   []any
	)
	if f.OlderThan > 0 {
		cutoff := time.Now().Add(-f.OlderThan).UnixMilli()
		wheres = append(wheres, "created_at < ?")
		args = append(args, cutoff)
	}
	if strings.TrimSpace(f.Provider) != "" {
		wheres = append(wheres, "provider = ?")
		args = append(args, f.Provider)
	}
	if f.StatusCode != nil {
		wheres = append(wheres, "status_code = ?")
		args = append(args, *f.StatusCode)
	}
	if len(wheres) == 0 {
		return 0, fmt.Errorf("at least one filter required for bulk delete")
	}
	whereSQL := "WHERE " + strings.Join(wheres, " AND ")
	res, err := s.db.ExecContext(ctx, `DELETE FROM webhooks `+whereSQL, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func ensureDir(path string) error {
	if path == "." || path == "" {
		return nil
	}
	return os.MkdirAll(path, 0o755)
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// sanitizeFTSQuery escapes FTS5 special characters and wraps terms in quotes.
// FTS5 special chars: " * - ^ : OR AND NOT NEAR
// We escape double quotes and wrap each word in quotes for literal matching.
func sanitizeFTSQuery(q string) string {
	// Escape existing double quotes
	q = strings.ReplaceAll(q, `"`, `""`)
	// Wrap the entire query in quotes to treat as a phrase/literal
	return `"` + q + `"`
}

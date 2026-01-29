package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS webhooks (
    id           TEXT PRIMARY KEY,
    created_at   INTEGER NOT NULL,

    method       TEXT NOT NULL,
    path         TEXT NOT NULL,
    query        TEXT,
    headers      TEXT NOT NULL,
    body         BLOB,

    provider     TEXT,
    event_type   TEXT,
    signature    TEXT,

    status_code  INTEGER,
    response_ms  INTEGER,

    body_text    TEXT
);

CREATE INDEX IF NOT EXISTS idx_webhooks_created ON webhooks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_webhooks_provider ON webhooks(provider);
CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhooks(status_code);

CREATE VIRTUAL TABLE IF NOT EXISTS webhooks_fts USING fts5(
    body_text,
    content='webhooks',
    content_rowid='rowid'
);

-- FTS5 external content triggers
CREATE TRIGGER IF NOT EXISTS webhooks_ai AFTER INSERT ON webhooks BEGIN
  INSERT INTO webhooks_fts(rowid, body_text) VALUES (new.rowid, new.body_text);
END;
CREATE TRIGGER IF NOT EXISTS webhooks_ad AFTER DELETE ON webhooks BEGIN
  INSERT INTO webhooks_fts(webhooks_fts, rowid, body_text) VALUES('delete', old.rowid, old.body_text);
END;
CREATE TRIGGER IF NOT EXISTS webhooks_au AFTER UPDATE ON webhooks BEGIN
  INSERT INTO webhooks_fts(webhooks_fts, rowid, body_text) VALUES('delete', old.rowid, old.body_text);
  INSERT INTO webhooks_fts(rowid, body_text) VALUES (new.rowid, new.body_text);
END;
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	if err := ping(ctx, s.db); err != nil {
		return err
	}
	return nil
}

func ping(ctx context.Context, db *sql.DB) error {
	return db.PingContext(ctx)
}

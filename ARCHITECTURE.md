# Architecture

This document describes the internal architecture of HookTM.

## Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                         HookTM CLI                               │
├─────────────────────────────────────────────────────────────────┤
│  cmd/hooktm/main.go                                             │
│  └── Parses args, initializes app, runs commands                │
├─────────────────────────────────────────────────────────────────┤
│                      internal/cli/                               │
│  ┌─────────┬─────────┬─────────┬─────────┬─────────┬─────────┐ │
│  │ listen  │  list   │  show   │ replay  │   ui    │ codegen │ │
│  └────┬────┴────┬────┴────┬────┴────┬────┴────┬────┴────┬────┘ │
├───────┼─────────┼─────────┼─────────┼─────────┼─────────┼───────┤
│       ▼         ▼         ▼         ▼         ▼         ▼       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │  proxy  │ │  store  │ │  store  │ │ replay  │ │ codegen │   │
│  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘   │
│       │           │           │           │           │         │
│       ▼           ▼           ▼           ▼           ▼         │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                    SQLite (WAL mode)                     │   │
│  │                   ~/.hooktm/hooks.db                     │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Package Overview

### `cmd/hooktm`

Entry point. Minimal code that:
- Creates the CLI app
- Normalizes arguments (allows flags after positional args)
- Runs the selected command

### `internal/cli`

CLI command implementations using [urfave/cli](https://github.com/urfave/cli).

| File | Purpose |
|------|---------|
| `app.go` | App definition, global flags |
| `listen.go` | Start proxy server |
| `list.go` | List webhooks |
| `show.go` | Show webhook details |
| `replay.go` | Replay webhooks |
| `ui.go` | Launch TUI |
| `codegen.go` | Generate validation code |
| `common.go` | Shared utilities (store opening, etc.) |
| `normalize_args.go` | Argument normalization |

### `internal/proxy`

HTTP reverse proxy that captures webhooks.

**Key components:**
- `RecorderProxy` - HTTP handler that:
  1. Reads request body (with size limit)
  2. Detects provider
  3. Forwards to target (if configured)
  4. Records to database

**Request flow:**
```
Client → RecorderProxy → Forward Target
              │
              ▼
           SQLite
```

### `internal/store`

SQLite database operations.

**Schema:**
```sql
webhooks (
    id           TEXT PRIMARY KEY,  -- Nano ID
    created_at   INTEGER,           -- Unix ms
    method       TEXT,
    path         TEXT,
    query        TEXT,
    headers      TEXT,              -- JSON
    body         BLOB,
    provider     TEXT,
    event_type   TEXT,
    signature    TEXT,
    status_code  INTEGER,
    response_ms  INTEGER,
    body_text    TEXT               -- For FTS
)

webhooks_fts (FTS5 virtual table for full-text search)
```

**Key operations:**
- `InsertWebhook` - Store captured webhook
- `ListSummaries` - List with filters
- `GetWebhook` - Get full details by ID
- `SearchSummaries` - FTS5 full-text search

### `internal/replay`

Webhook replay engine.

**Features:**
- Reconstructs original request
- Applies JSON merge patches (RFC7396)
- Supports dry-run mode
- Preserves original headers

### `internal/codegen`

Generates signature validation code from captured webhooks.

**Supported languages:**
- Go
- TypeScript
- Python
- PHP
- Ruby

**Provider-specific templates:**
- Stripe (uses official SDK)
- GitHub (HMAC-SHA256)
- Generic HMAC fallback

### `internal/tui`

Terminal UI using [Bubble Tea](https://github.com/charmbracelet/bubbletea).

**Components:**
- List panel (left) - Webhook list
- Detail panel (right) - Selected webhook
- Search input
- Keyboard navigation

### `internal/provider`

Webhook provider detection.

**Detection logic:**
```go
Stripe-Signature header    → stripe
X-GitHub-Event header      → github
Otherwise                  → unknown (with signature extraction)
```

### `internal/config`

YAML configuration loading.

**Config file:** `~/.hooktm/config.yaml`

```yaml
forward: localhost:3000
port: 8080
db: ~/.hooktm/hooks.db
lang: go
```

### `internal/urlutil`

Shared URL utilities.

- `SingleJoiningSlash` - Join URL paths correctly

## Data Flow

### Webhook Capture

```
1. HTTP request arrives at proxy port
2. RecorderProxy.ServeHTTP():
   a. Read body (limited to 10MB)
   b. Generate nano ID
   c. Detect provider from headers
   d. Forward to target (if configured)
   e. Record to SQLite
3. Response returned to client
```

### Webhook Replay

```
1. User runs: hooktm replay <id>
2. Load webhook from SQLite
3. Apply JSON patch (if provided)
4. Reconstruct HTTP request
5. Send to target
6. Report result
```

### Full-Text Search

```
1. On insert: body_text extracted and indexed via FTS5 trigger
2. On search: Query sanitized, FTS5 MATCH executed
3. Results joined with main table for full data
```

## Design Decisions

### SQLite with WAL

- Single-file database, no server needed
- WAL mode for concurrent reads during writes
- MaxOpenConns(1) for simplicity

### Nano IDs

- URL-safe, compact identifiers
- No sequential guessing
- Good for CLI copy-paste

### FTS5 External Content

- Avoids data duplication
- Triggers keep index in sync
- Query sanitization prevents injection

### Record-Only Mode

- Useful for testing without running backend
- Returns 200 OK, still records webhook
- Forward target is optional

## Error Handling

### Proxy Errors

- Body read errors → 400 Bad Request
- Body too large → 413 Request Entity Too Large
- Forward failure → 502 Bad Gateway (still recorded)
- Database errors → Logged, request continues

### Store Errors

- Wrapped with context: `fmt.Errorf("migrate: %w", err)`
- Missing records: `not found: <id>`
- Invalid input: Validation errors returned

## Security Considerations

### Request Size Limit

10 MB limit prevents memory exhaustion attacks.

### FTS Query Sanitization

User input wrapped in quotes to prevent FTS5 operator injection.

### Sensitive Data

- All headers stored (including auth tokens)
- Consider adding `--strip-headers` for replay
- Database should be treated as sensitive

## Performance

### Bottlenecks

- SQLite writes (mitigated by WAL)
- Large webhook bodies
- FTS indexing

### Optimizations

- Single connection (WAL-friendly)
- Prepared statements via database/sql
- Body text capped at 200KB for FTS

## Testing Strategy

### Unit Tests

- `store/store_test.go` - Database operations
- `provider/detect_test.go` - Provider detection
- `replay/engine_test.go` - Replay logic

### Integration Tests

- Manual testing with real webhooks
- Stripe CLI forwarding

### Missing Coverage

- Proxy handler (needs HTTP test server)
- CLI commands (needs integration harness)
- TUI (difficult to test)

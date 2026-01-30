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
│  │ listen  │  list   │  show   │ replay  │  delete │   ui    │ │
│  │         │         │         │  code   │         │  codegen│ │
│  └────┬────┴────┬────┴────┬────┴────┬────┴────┬────┴────┬────┘ │
├───────┼─────────┼─────────┼─────────┼─────────┼─────────┼───────┤
│       ▼         ▼         ▼         ▼         ▼         ▼       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐   │
│  │  proxy  │ │  store  │ │  store  │ │ replay  │ │ codegen │   │
│  │         │ │         │ │         │ │         │ │         │   │
│  │┌───────┐│ │┌───────┐│ │┌───────┐│ │┌───────┐│ │┌───────┐│   │
│  ││logger ││ ││config ││ ││search ││ ││config ││ ││provider││   │
│  │└───────┘│ │└───────┘│ │└───────┘│ │└───────┘│ │└───────┘│   │
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
| `listen.go` | Start proxy server with logging options |
| `list.go` | List webhooks with date filtering |
| `show.go` | Show webhook details |
| `replay.go` | Replay webhooks with CI exit codes |
| `delete.go` | Delete webhooks by filter |
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
  5. Structured logging with correlation IDs

**Request flow:**
```
Client → RecorderProxy → Forward Target
              │
              ├──► Logger (structured)
              │
              ▼
           SQLite
```

**Context Support:**
- Checks for context cancellation on each request
- Returns 503 Service Unavailable if request cancelled
- Respects timeouts from parent context

### `internal/store`

SQLite database operations with context support.

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
- `InsertWebhook` - Store captured webhook (context-aware)
- `ListSummaries` - List with filters (context-aware)
- `GetWebhook` - Get full details by ID (context-aware)
- `SearchSummaries` - FTS5 full-text search (context-aware)
- `DeleteWebhook` - Delete by ID (context-aware)
- `DeleteByFilter` - Bulk delete (context-aware)

**Context Support:**
- All operations accept `context.Context`
- Respects cancellation and timeouts
- Uses `OpenContext()` for cancellable database opening

### `internal/replay`

Webhook replay engine with context support.

**Features:**
- Reconstructs original request
- Applies JSON merge patches (RFC7396)
- Supports dry-run mode
- Preserves original headers
- Context-aware HTTP requests
- Respects cancellation during body draining

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
- Replay integration

### `internal/provider`

Webhook provider detection.

**Detection logic:**
```go
Stripe-Signature header    → stripe
X-GitHub-Event header      → github
Otherwise                  → unknown (with signature extraction)
```

### `internal/config`

YAML configuration loading and validation.

**Config file:** `~/.hooktm/config.yaml`

```yaml
forward: localhost:3000
port: 8080
db: ~/.hooktm/hooks.db
lang: go
```

**Validation:**
- `port`: 1-65535
- `forward`: Valid URL or host:port
- `db`: No path traversal (`..`)
- `lang`: Supported language code

**Functions:**
- `Load()` - Load config from file
- `LoadAndValidate()` - Load and validate
- `Validate()` - Validate config values
- `SetDefaults()` - Apply default values

### `internal/logger`

Structured logging with multiple output formats.

**Features:**
- Log levels: DEBUG, INFO, WARN, ERROR
- Formats: text, JSON
- Structured fields with chaining
- Correlation ID support via context
- Thread-safe operations

**Usage:**
```go
log := logger.New(logger.Config{
    Level:  logger.InfoLevel,
    Format: "json",
})
log.WithField("provider", "stripe").Info("webhook captured")
```

### `internal/urlutil`

Shared URL utilities.

- `SingleJoiningSlash` - Join URL paths correctly
- `ParseURL` - Parse URLs with host:port shorthand support
- `MustParseURL` - Parse with panic on error (for tests)

## Data Flow

### Webhook Capture

```
1. HTTP request arrives at proxy port
2. RecorderProxy.ServeHTTP():
   a. Check context cancellation
   b. Read body (limited to 10MB)
   c. Generate nano ID
   d. Detect provider from headers
   e. Forward to target (if configured)
   f. Record to SQLite (with context)
3. Response returned to client
```

### Webhook Replay

```
1. User runs: hooktm replay <id>
2. Load webhook from SQLite (with context)
3. Apply JSON patch (if provided)
4. Reconstruct HTTP request (with context)
5. Send to target (respects cancellation)
6. Drain response body (with context check)
7. Report result
```

### Full-Text Search

```
1. On insert: body_text extracted and indexed via FTS5 trigger
2. On search: Query sanitized, FTS5 MATCH executed
3. Results joined with main table for full data
```

### Context Propagation

```
CLI Command ──► Store Operation
     │                │
     ▼                ▼
  Context ◄───── Context Timeout
     │                │
     ▼                ▼
  Proxy Handler ◄── Cancellation Check
     │
     ▼
  Replay Engine ◄── HTTP Request Context
```

## Design Decisions

### SQLite with WAL

- Single-file database, no server needed
- WAL mode for concurrent reads during writes
- MaxOpenConns(1) for simplicity
- Context-aware operations for cancellation

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

### Structured Logging

- Interface-based design for testability
- Multiple output formats (text/JSON)
- Log levels for different environments
- Correlation IDs for request tracing

### Context Cancellation

- All long-running operations accept context
- Graceful shutdown on SIGINT/SIGTERM
- Request cancellation returns 503
- Database operations respect timeouts

## Error Handling

### Proxy Errors

- Body read errors → 400 Bad Request
- Body too large → 413 Request Entity Too Large
- Context cancelled → 503 Service Unavailable
- Forward failure → 502 Bad Gateway (still recorded)
- Database errors → Logged, request continues

### Store Errors

- Wrapped with context: `fmt.Errorf("migrate: %w", err)`
- Missing records: `not found: <id>`
- Invalid input: Validation errors returned
- Context cancelled: Operation aborted

### Validation Errors

- `ValidationError` type with field name
- Descriptive error messages
- `IsValidationError()` helper function

## Security Considerations

### Request Size Limit

10 MB limit prevents memory exhaustion attacks.

### FTS Query Sanitization

User input wrapped in quotes to prevent FTS5 operator injection.

### Config Validation

- Port range validation (1-65535)
- URL format validation
- Path traversal prevention (`..`)
- Language code whitelist

### Sensitive Data

- All headers stored (including auth tokens)
- Database should be treated as sensitive
- Consider file permissions on hooks.db

## Performance

### Benchmarks (AMD Ryzen 9 3900X)

| Operation | Performance |
|-----------|-------------|
| Insert webhook | ~140μs |
| Get webhook | ~25μs |
| List webhooks (1000) | ~80μs |
| Search webhooks (1000) | ~1.2ms |
| Concurrent operations | 10,000+ ops/sec |

### Bottlenecks

- SQLite writes (mitigated by WAL)
- Large webhook bodies
- FTS indexing

### Optimizations

- Single connection (WAL-friendly)
- Prepared statements via database/sql
- Body text capped at 200KB for FTS
- Context-aware cancellation

## Testing Strategy

### Unit Tests

| Package | Coverage | Focus |
|---------|----------|-------|
| `store` | 80%+ | Database operations, context handling |
| `provider` | 79% | Provider detection |
| `replay` | 96%+ | Replay logic, JSON patching |
| `config` | 91%+ | Loading, validation |
| `logger` | 83%+ | Logging, formatting |
| `urlutil` | 93%+ | URL parsing |
| `proxy` | 89%+ | HTTP handling, logging |
| `tui` | 84%+ | UI components |

### Integration Tests

- `tests/integration_test.go` - Full flow testing
- Webhook capture → store → replay
- Concurrent operations
- Provider detection
- Error scenarios

### Benchmarks

- `store/store_bench_test.go` - Performance baselines
- Insert, get, list, search, delete operations
- Concurrent load testing

### Running Tests

```bash
# All tests
go test ./...

# With coverage
go test -cover ./...

# Benchmarks
go test -bench=. ./internal/store

# Race detection
go test -race ./...
```

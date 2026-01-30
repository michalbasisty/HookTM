# HookTM

**Local-first webhook development companion**

Capture, browse, replay, and generate signature validation code — **no accounts, no cloud**.

## Features

- **Capture**: Proxy that records all incoming webhooks to SQLite
- **Browse**: Terminal UI for exploring captured webhooks
- **Replay**: Re-send webhooks with optional JSON patching
- **Codegen**: Generate signature validation code (Go, TypeScript, Python, PHP, Ruby)
- **Search**: Full-text search across webhook bodies
- **Provider Detection**: Auto-detects Stripe, GitHub, and other webhook providers
- **Structured Logging**: Configurable log levels and JSON output
- **Graceful Shutdown**: Context-aware cancellation and timeouts

## Installation

```bash
go build ./cmd/hooktm
```

Requires Go 1.22 or later.

## Quick Start

### 1. Start the Proxy

```bash
# Record-only mode (just capture webhooks)
./hooktm listen 8080

# Forward mode (capture + forward to your app)
./hooktm listen 8080 --forward localhost:3000

# With structured logging (JSON format)
./hooktm listen 8080 --log-format json --log-level info
```

### 2. Point Your Webhooks

Configure your webhook provider (Stripe, GitHub, etc.) to send to:
```
http://localhost:8080/your/webhook/path
```

### 3. Browse Captured Webhooks

```bash
# Terminal UI
./hooktm ui

# Command line
./hooktm list
./hooktm show <id>
```

## Commands

### `listen` - Start Proxy

```bash
./hooktm listen <port> [--forward <target>]

# Examples
./hooktm listen 8080                          # Record-only mode
./hooktm listen 8080 --forward localhost:3000 # Forward to app
./hooktm listen 8080 --forward http://app:3000/api

# With logging options
./hooktm listen 8080 --log-level debug --log-format text
./hooktm listen 8080 --log-level warn --log-format json
```

**Options:**
- `--forward <target>` - Forward requests to this URL
- `--log-level <level>` - Log level: debug, info, warn, error (default: info)
- `--log-format <format>` - Log format: text, json (default: text)

### `list` - List Webhooks

```bash
./hooktm list [options]

Options:
  --limit <n>       Max rows (default: 20, max: 500)
  --provider <name> Filter by provider (stripe, github, unknown)
  --status <code>   Filter by response status code
  --search <query>  Full-text search in body
  --from <date>     Start date (e.g., 2024-01-15 or 7d for last 7 days)
  --to <date>       End date
  --json            Output as JSON
```

### `show` - View Webhook Details

```bash
./hooktm show <id> [--format json|raw]
```

### `replay` - Replay Webhooks

```bash
./hooktm replay <id> [options]
./hooktm replay --last <n> [options]

Options:
  --to <url>        Override replay target
  --patch <json>    Apply RFC7396 JSON merge patch
  --dry-run         Print without sending
  --json            Output as JSON
  --ci              CI mode: return non-zero exit code on failure

# Examples
./hooktm replay abc123 --to localhost:3000
./hooktm replay abc123 --patch '{"amount": 5000}'
./hooktm replay --last 5 --to localhost:3000
./hooktm replay abc123 --ci --json
```

**Exit Codes (with --ci):**
- `0` - Success (2xx response)
- `1` - Connection error
- `2` - HTTP error (4xx/5xx)
- `3` - Other error

### `delete` - Delete Webhooks

```bash
./hooktm delete <id>                    # Delete by ID
./hooktm delete --older-than 7d         # Delete older than 7 days
./hooktm delete --provider stripe       # Delete all Stripe webhooks
./hooktm delete --status 500            # Delete failed webhooks
```

### `codegen` - Generate Validation Code

```bash
./hooktm codegen <id> --lang <go|ts|python|php|ruby>

# Example
./hooktm codegen abc123 --lang go > webhook/validate.go
```

### `ui` - Interactive Terminal UI

```bash
./hooktm ui
```

**Keybindings:**
- `j/k` or `↑/↓` - Navigate
- `r` - Replay selected webhook
- `/` - Search
- `Enter` - Apply search
- `q` - Quit

## Configuration

### Config File (Optional)

Create `~/.hooktm/config.yaml`:

```yaml
# Default forward target for replay
forward: localhost:3000

# Default port for the listen command
port: 8080

# Database file path
db: ~/.hooktm/hooks.db

# Default language for code generation
lang: go
```

**Validation:**
- `port` must be between 1 and 65535
- `forward` must be a valid URL or host:port
- `db` path cannot contain `..` (path traversal)
- `lang` must be one of: go, ts, python, php, ruby (and aliases)

### Environment Variables

```bash
HOOKTM_DB=/path/to/hooks.db
HOOKTM_CONFIG=/path/to/config.yaml
```

### Command Line Flags

```bash
./hooktm --db /path/to/hooks.db <command>
./hooktm --config /path/to/config.yaml <command>
```

## Structured Logging

HookTM supports structured logging with configurable levels and formats.

### Log Levels

- `debug` - Detailed debugging information
- `info` - General operational information (default)
- `warn` - Warning messages
- `error` - Error messages

### Log Formats

**Text format:**
```
2026-01-30T15:04:05.123Z [INFO] webhook captured method=POST path=/webhooks provider=stripe
2026-01-30T15:04:05.234Z [WARN] forward failed error="connection refused" target=http://localhost:3000
```

**JSON format:**
```json
{"timestamp":"2026-01-30T15:04:05.123456789Z","level":"INFO","message":"webhook captured","fields":{"method":"POST","path":"/webhooks","provider":"stripe"}}
```

### Configuration

```bash
# Default text format
./hooktm listen 8080

# JSON format for production logging
./hooktm listen 8080 --log-format json

# Debug level for troubleshooting
./hooktm listen 8080 --log-level debug
```

## Storage

Default database location: `~/.hooktm/hooks.db`

The database uses SQLite with WAL mode for performance. Webhooks are stored with:
- Full request headers and body
- Provider detection (Stripe, GitHub, etc.)
- Event type extraction
- Response status and latency
- Full-text search index (FTS5)

### Limits

- Max request body: 10 MB
- Body text indexed for search: 200 KB
- Max list results: 500

## Provider Detection

HookTM auto-detects webhook providers:

| Provider | Detection Method |
|----------|-----------------|
| Stripe | `Stripe-Signature` header |
| GitHub | `X-GitHub-Event` header |
| Slack | `X-Slack-Signature` header |
| Shopify | `X-Shopify-Hmac-SHA256` header |
| Twilio | `X-Twilio-Signature` header |

## Examples

### Local Development with Stripe

```bash
# Terminal 1: Start your app
npm run dev  # Running on localhost:3000

# Terminal 2: Start HookTM proxy
./hooktm listen 8080 --forward localhost:3000

# Terminal 3: Use Stripe CLI to forward webhooks
stripe listen --forward-to localhost:8080/webhooks/stripe
```

### Debugging a Failed Webhook

```bash
# Find the webhook
./hooktm list --status 500 --limit 10

# View details
./hooktm show abc123 --format raw

# Replay with modifications
./hooktm replay abc123 --patch '{"data":{"object":{"amount":1000}}}'
```

### Generate Validation Code

```bash
# Capture a real webhook first, then generate code
./hooktm codegen abc123 --lang ts > src/lib/stripe-webhook.ts
```

### Batch Operations

```bash
# Replay last 10 webhooks
./hooktm replay --last 10 --to localhost:3000

# Delete old webhooks
./hooktm delete --older-than 30d

# Search and replay
./hooktm list --search "payment_intent" --json | jq -r '.[].id' | xargs -I {} ./hooktm replay {} --to localhost:3000
```

## Performance

Based on benchmarks (AMD Ryzen 9 3900X):

| Operation | Performance |
|-----------|-------------|
| Insert webhook | ~140μs |
| Get webhook | ~25μs |
| List webhooks (1000) | ~80μs |
| Search webhooks (1000) | ~1.2ms |
| Concurrent operations | Handles 10,000+ ops/sec |

## Graceful Shutdown

HookTM supports graceful shutdown with context cancellation:
- In-flight requests are allowed to complete
- Database operations are cancelled appropriately
- HTTP connections are properly closed

Press `Ctrl+C` to initiate graceful shutdown.

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./internal/store
```

### Project Structure

```
.
├── cmd/hooktm/        # CLI entry point
├── internal/
│   ├── cli/           # CLI commands
│   ├── config/        # Configuration loading and validation
│   ├── logger/        # Structured logging
│   ├── proxy/         # HTTP proxy and recorder
│   ├── replay/        # Webhook replay engine
│   ├── store/         # SQLite database operations
│   ├── tui/           # Terminal UI (Bubble Tea)
│   └── urlutil/       # URL utilities
└── tests/             # Integration tests
```

## License

MIT

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

## Installation

```bash
go build ./cmd/hooktm
```

## Quick Start

### 1. Start the Proxy

```bash
# Record-only mode (just capture webhooks)
./hooktm listen 8080

# Forward mode (capture + forward to your app)
./hooktm listen 8080 --forward localhost:3000
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
```

### `list` - List Webhooks

```bash
./hooktm list [options]

Options:
  --limit <n>       Max rows (default: 20, max: 500)
  --provider <name> Filter by provider (stripe, github, unknown)
  --status <code>   Filter by response status code
  --search <query>  Full-text search in body
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

# Examples
./hooktm replay abc123 --to localhost:3000
./hooktm replay abc123 --patch '{"amount": 5000}'
./hooktm replay --last 5 --to localhost:3000
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
- `q` - Quit

## Configuration

### Config File (Optional)

Create `~/.hooktm/config.yaml`:

```yaml
forward: localhost:3000
port: 8080
db: ~/.hooktm/hooks.db
lang: go
```

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

## Storage

Default database location: `~/.hooktm/hooks.db`

The database uses SQLite with WAL mode for performance. Webhooks are stored with:
- Full request headers and body
- Provider detection (Stripe, GitHub, etc.)
- Event type extraction
- Response status and latency

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

## Limits

- Max request body: 10 MB
- Body text indexed for search: 200 KB
- Max list results: 500

## License

MIT

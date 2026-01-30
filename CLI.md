# HookTM CLI Reference

HookTM is a local-first webhook development tool for capturing, inspecting, and replaying webhooks.

## Global Flags

| Flag | Env Var | Description |
|------|---------|-------------|
| `--db` | `HOOKTM_DB` | Database file path (default: `~/.hooktm/hooks.db`) |
| `--config` | `HOOKTM_CONFIG` | Config file path |
| `--help` | - | Show help |
| `--version` | - | Show version |

---

## Commands

### `listen` - Start webhook capture server

Start a server that captures incoming webhooks and stores them in the database.

```bash
hooktm listen <port> [flags]
```

**Flags:**
- `--forward` - Forward requests to a URL (e.g., `localhost:3000`)
- `--log-level` - Log level: `debug`, `info`, `warn`, `error` (default: `info`)
- `--log-format` - Log format: `text`, `json` (default: `text`)

**Examples:**
```bash
# Record only
hooktm listen 8080

# Proxy to local development server
hooktm listen 8080 --forward localhost:3000

# Proxy to full URL
hooktm listen 8080 --forward http://api.example.com/webhook

# With debug logging
hooktm listen 8080 --log-level debug

# With JSON logging for production
hooktm listen 8080 --log-format json

# Combined options
hooktm listen 8080 --forward localhost:3000 --log-level warn --log-format json
```

---

### `list` - List captured webhooks

Display captured webhooks with optional filtering.

```bash
hooktm list [flags]
```

**Flags:**
- `--limit` - Maximum results (default: 20)
- `--provider` - Filter by provider (stripe, github, etc.)
- `--status` - Filter by HTTP status code
- `--search` - Search in webhook body text
- `--from` - Start date/time
- `--to` - End date/time
- `--json` - Output as JSON

**Date Formats:**
- `YYYY-MM-DD` - Date only (e.g., `2024-01-15`)
- ISO 8601 - Full timestamp (e.g., `2024-01-15T10:30:00Z`)
- Relative - `1d`, `7d`, `30d`, `1h` (e.g., `--from 7d` for last 7 days)

**Examples:**
```bash
# Show recent 20 webhooks
hooktm list

# Show more results
hooktm list --limit 50

# Filter by provider
hooktm list --provider stripe

# Last 7 days
hooktm list --from 7d

# Date range
hooktm list --from 2024-01-01 --to 2024-01-31

# Search body text
hooktm list --search "payment"

# JSON output
hooktm list --json

# Combined filters
hooktm list --provider stripe --status 200 --from 7d --json
```

---

### `show` - Show webhook details

Display full details of a captured webhook.

```bash
hooktm show <id> [flags]
```

**Flags:**
- `--format` - Output format: `json` (default) or `raw`

**Examples:**
```bash
# JSON output (default)
hooktm show abc123

# Raw text output
hooktm show abc123 --format raw
```

---

### `replay` - Replay webhooks

Replay captured webhooks to a target URL.

```bash
hooktm replay [id] [flags]
```

**Flags:**
- `--to` - Target URL to replay to
- `--patch` - JSON merge patch to apply (RFC 7396)
- `--last` - Replay last N webhooks (newest first)
- `--dry-run` - Show what would be sent without sending
- `--json` - Output as JSON
- `--ci` - CI mode: return non-zero exit code on failure

**Exit Codes (with --ci):**
- `0` - Success (2xx response)
- `1` - Connection error
- `2` - HTTP error (4xx/5xx)
- `3` - Other error

**Examples:**
```bash
# Replay single webhook
hooktm replay abc123 --to localhost:3000

# Replay to full URL
hooktm replay abc123 --to http://api.example.com/webhook

# Dry run
hooktm replay abc123 --to localhost:3000 --dry-run

# Replay last 5 webhooks
hooktm replay --last 5 --to localhost:3000

# CI mode with JSON output
hooktm replay abc123 --to localhost:3000 --ci --json

# Apply JSON patch
hooktm replay abc123 --to localhost:3000 --patch '{"status":"test"}'
```

---

### `codegen` - Generate validation code

Generate signature validation code from a captured webhook.

```bash
hooktm codegen <id> --lang <language>
```

**Flags:**
- `--lang` - Language: `go`, `ts`, `python`, `php`, `ruby` (required)

**Supported Languages:**
- `go`, `golang` - Go
- `ts`, `typescript` - TypeScript
- `py`, `python` - Python
- `php` - PHP
- `rb`, `ruby` - Ruby

**Examples:**
```bash
hooktm codegen abc123 --lang go
hooktm codegen abc123 --lang python
hooktm codegen abc123 --lang typescript
```

---

### `delete` - Delete webhooks

Delete webhooks by ID or by filter criteria.

```bash
hooktm delete [id] [flags]
```

**Flags:**
- `--older-than` - Delete webhooks older than duration (e.g., `7d`, `30d`)
- `--provider` - Delete by provider name
- `--status` - Delete by HTTP status code
- `--yes` - Skip confirmation prompt

**Examples:**
```bash
# Delete by ID
hooktm delete abc123

# Delete older than 7 days
hooktm delete --older-than 7d

# Delete all Stripe webhooks
hooktm delete --provider stripe

# Delete failed webhooks
hooktm delete --status 500

# Skip confirmation
hooktm delete --older-than 30d --yes
```

---

### `ui` - Open interactive UI

Launch the interactive terminal UI for browsing webhooks.

```bash
hooktm ui
```

**Navigation:**
- `↑/↓` or `j/k` - Move up/down
- `Enter` - View details
- `/` - Search
- `q` - Quit

---

## Quick Start

1. **Start capturing webhooks:**
   ```bash
   hooktm listen 8080 --forward localhost:3000
   ```

2. **List captured webhooks:**
   ```bash
   hooktm list
   ```

3. **View webhook details:**
   ```bash
   hooktm show <id>
   ```

4. **Replay a webhook:**
   ```bash
   hooktm replay <id> --to localhost:3000
   ```

5. **Open interactive UI:**
   ```bash
   hooktm ui
   ```

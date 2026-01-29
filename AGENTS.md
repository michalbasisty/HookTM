# AGENTS.md

This file provides guidance to WARP (warp.dev) when working with code in this repository.

## Key development commands

All commands are intended to be run from the repository root.

### Build

- Build the `hooktm` CLI binary:

```bash path=null start=null
go build ./cmd/hooktm
```

### Tests

The project is Go 1.22+ and follows a strong TDD culture (see `AGENTIC.md`). Prefer writing or updating tests first.

- Run all tests:

```bash path=null start=null
go test ./...
```

- Run all tests with coverage:

```bash path=null start=null
go test -cover ./...
```

- Run tests for a single package (example: store):

```bash path=null start=null
go test ./internal/store
```

- Run tests for a single package with verbose output (example: provider):

```bash path=null start=null
go test -v ./internal/provider
```

- Run a specific test by name (pattern match):

```bash path=null start=null
go test -v -run TestProviderDetection ./internal/provider
```

- Generate a coverage report and open it in a browser:

```bash path=null start=null
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Formatting and basic linting

Use standard Go tooling; there is no separate custom linter config in this repo.

- Format code:

```bash path=null start=null
go fmt ./...
```

- Go vet (static checks):

```bash path=null start=null
go vet ./...
```

## High-level architecture

HookTM is a Go CLI application for capturing, browsing, replaying, and generating code for webhooks. The core binary is built from `cmd/hooktm`, and most logic lives under `internal/`.

### Top-level layout

- `cmd/hooktm/`: Minimal entrypoint; sets up the CLI app, normalizes arguments, and dispatches to commands.
- `internal/cli/`: CLI commands (listen, list, show, replay, ui, codegen). These are thin wrappers that parse flags/args, validate input, call domain packages, and handle user-facing output & exit codes.
- `internal/proxy/`: HTTP reverse proxy that terminates incoming webhook requests, optionally forwards them, and records them to SQLite.
- `internal/store/`: All database concerns (SQLite + FTS5), including schema, migrations, queries, and full-text search.
- `internal/replay/`: Webhook replay engine that reconstructs and optionally patches requests before sending them to a target.
- `internal/codegen/`: Code generation logic and templates for producing signature verification code in multiple languages.
- `internal/tui/`: Bubble Tea–based terminal UI for exploring captured webhooks.
- `internal/provider/`: Webhook provider detection, mapping HTTP headers/body to provider identifiers and signatures.
- `internal/config/`: Configuration loading/merging from YAML files and environment variables.
- `internal/urlutil/`: Shared URL/path utilities.

The README and `ARCHITECTURE.md` contain more end-user and design details; prefer to keep them as the single source of truth and update them when changing behavior.

### Data and control flow

At a high level, the flow through the system looks like this:

1. **CLI entry**: `cmd/hooktm/main.go` constructs the CLI app (using `internal/cli`) and dispatches to the selected command (`listen`, `list`, `show`, `replay`, `ui`, `codegen`, etc.).
2. **Commands → domain packages**: Each `internal/cli/*.go` command delegates to domain packages instead of embedding logic directly in the command file.
   - `listen` → `internal/proxy` (starts the HTTP proxy/recorder) and `internal/store` (for DB access).
   - `list`/`show` → `internal/store` (listing summaries, fetching full webhook records) and formatting results for CLI.
   - `replay` → `internal/replay` (reconstructs and sends HTTP requests) with data loaded from `internal/store`.
   - `codegen` → `internal/codegen` (renders provider-specific templates from captured webhook data).
   - `ui` → `internal/tui` (Bubble Tea program that internally talks to `internal/store` and other packages as needed).
3. **Persistence**: All webhook data is stored in a single SQLite database (WAL mode) managed by `internal/store`.
   - Primary table `webhooks` stores headers, body, request/response metadata, provider info, and derived fields.
   - An FTS5 virtual table is used for full-text search over body text, kept in sync via triggers.
4. **Replay and codegen**:
   - `internal/replay` reads webhook records from `internal/store`, applies RFC 7396 JSON merge patches when requested, and issues HTTP requests to the chosen target.
   - `internal/codegen` consumes stored webhook data plus provider detection to produce validation snippets in Go, TypeScript, Python, PHP, and Ruby (including provider-specific behaviors like Stripe and GitHub).
5. **TUI**: `internal/tui` provides a two-pane terminal UI (list + detail with search), built on Bubble Tea, that uses `internal/store` for data access.

### Package responsibilities and boundaries

These boundaries are important when adding or modifying functionality:

- **CLI (`internal/cli`)**
  - Responsible for user interaction at the command line: parsing flags/args, printing help and errors, setting exit codes.
  - Should not contain business logic; instead, call into `store`, `proxy`, `replay`, `codegen`, `tui`, etc.

- **Proxy (`internal/proxy`)**
  - Owns HTTP request handling for incoming webhooks.
  - Enforces request size limits (e.g., 10 MB) and orchestrates provider detection and DB writes.
  - Forwards requests to user applications when configured, but business rules and persistence remain in domain packages.

- **Store (`internal/store`)**
  - Central place for all DB access; no other package issues raw SQL.
  - Handles schema migrations (see `migrate.go`), inserts, queries, and FTS integration.
  - Should not know about HTTP or CLI-specific concerns.

- **Domain packages (`internal/replay`, `internal/codegen`, `internal/provider`)**
  - Encapsulate core webhook logic.
  - Prefer pure, testable functions with minimal side effects.
  - Organized so they can be used from both CLI and TUI.

- **Config & utilities (`internal/config`, `internal/urlutil`)**
  - Provide configuration loading/merging and shared helpers; avoid growing them into grab-bag utility packages.

### Import and layering rules (from `AGENTIC.md`)

Respect the layering constraints to avoid tangled dependencies:

- CLI code may import any internal package it needs.
- `internal/store` should only import the standard library and low-level helpers like `internal/urlutil`.
- Domain packages should avoid depending on CLI or proxy code; keep them focused on business logic.
- Avoid circular imports and avoid having lower-level layers (`store`, `provider`, etc.) import higher-level ones (`cli`, `tui`, `proxy`).

## Testing and quality expectations

The project has explicit expectations for tests and quality (see `AGENTIC.md` and `CONTRIBUTING.md`). When modifying behavior or adding features:

- Prefer TDD: add or update tests in `*_test.go` next to the implementation before changing code.
- Use table-driven tests and clear naming (`Test<Thing>_<Scenario>`).
- Keep tests independent of global state; use interfaces and in-memory SQLite where appropriate for DB-backed tests.
- For DB schema changes, update migrations in `internal/store/migrate.go` and exercise them via tests.
- For provider detection or codegen changes, update the corresponding tests in `internal/provider` and `internal/codegen`.

Approximate coverage targets for key areas (from `AGENTIC.md`):

- `internal/store`: ~80%+
- `internal/provider`: ~90%+
- `internal/replay`: ~80%+
- `internal/codegen`: ~70%+
- `internal/config`: ~70%+
- `internal/cli`: ~60%+ (commands are harder to test; prefer focused tests on behavior and exit codes).

When adding user-facing behavior (new commands, provider templates, or configuration options), ensure that README/CHANGELOG/ROADMAP are updated in tandem so that they remain the source of truth for users.

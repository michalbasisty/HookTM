# Agentic Development Guide for HookTM

This document provides guidelines for AI agents (and humans) working on the HookTM codebase. It establishes conventions, workflows, and quality standards.

---

## Core Principles

1. **Test-Driven Development (TDD)** - Write tests first, then implementation
2. **Minimal Changes** - Make the smallest change that achieves the goal
3. **Readability Over Cleverness** - Code is read more than written
4. **Explicit Error Handling** - No silent failures
5. **Documentation as Code** - Keep docs in sync with code

---

## Test-Driven Development (TDD) Workflow

### The TDD Cycle

```
RED → GREEN → REFACTOR → REPEAT
```

### 1. RED: Write a Failing Test

Before writing any implementation:

```go
// internal/store/store_test.go
func TestWebhookFilteringByDateRange(t *testing.T) {
    // Arrange
    db := setupTestDB(t)
    defer cleanup(db)
    
    // Insert test data with different dates
    // ...
    
    // Act
    results, err := db.ListSummaries(store.Filter{
        From: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
        To:   time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC),
    })
    
    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(results) != expectedCount {
        t.Errorf("expected %d results, got %d", expectedCount, len(results))
    }
}
```

**Rules for tests:**
- Test file naming: `*_test.go` next to the implementation
- Test function naming: `Test<Component>_<Scenario>` or `Test<Function>_<Condition>`
- Use table-driven tests for multiple cases
- Always clean up resources (defer cleanup)
- Use `t.Fatalf` for unexpected errors, `t.Errorf` for assertion failures

### 2. GREEN: Make It Pass

Write the minimal implementation to pass the test:

```go
// Minimal implementation
func (s *Store) ListSummaries(filter Filter) ([]Summary, error) {
    // Just enough to pass the test
    query := "SELECT * FROM webhooks WHERE created_at >= ? AND created_at <= ?"
    // ... implementation
}
```

### 3. REFACTOR: Clean Up

Once passing, refactor both test and implementation:

- Remove duplication
- Improve naming
- Optimize if needed (with benchmarks)
- Ensure edge cases are covered

### 4. Test Coverage Requirements

| Component | Minimum Coverage |
|-----------|------------------|
| `internal/store` | 80% |
| `internal/provider` | 90% |
| `internal/replay` | 80% |
| `internal/codegen` | 70% |
| `internal/config` | 70% |
| `internal/cli` | 60% (commands are harder to test) |

**Run tests before committing:**

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/store

# Verbose output for debugging
go test -v ./internal/store
```

---

## Code Organization Rules

### Package Structure

```
internal/
├── cli/        # CLI commands - thin wrappers, delegate to packages
├── proxy/      # HTTP proxy - request handling, forwarding
├── store/      # Database - all SQL/SQLite operations
├── replay/     # Replay engine - HTTP request reconstruction
├── codegen/    # Code generation - templates, rendering
├── tui/        # Terminal UI - bubble tea components
├── provider/   # Provider detection - header/body analysis
├── config/     # Configuration - YAML, env vars
└── urlutil/    # Utilities - URL helpers
```

### Package Responsibilities

**CLI (`internal/cli`):**
- Parse flags and arguments
- Validate inputs
- Call domain packages
- Handle output formatting
- Return appropriate exit codes

**Proxy (`internal/proxy`):**
- Handle HTTP requests
- Forward to targets
- Record to database
- NO business logic

**Store (`internal/store`):**
- ALL database operations
- Schema migrations
- NO HTTP handling
- NO business logic

**Domain packages (replay, codegen, provider):**
- Pure functions when possible
- Testable without database
- No side effects

### Import Rules

Allowed imports:

```go
// CLI can import everything
import (
    "hooktm/internal/store"
    "hooktm/internal/proxy"
    // etc.
)

// Store can only import standard library + urlutil
import (
    "database/sql"
    "hooktm/internal/urlutil"
)

// Domain packages only import standard library + other domain packages
```

**Forbidden:**
- Circular imports
- CLI importing TUI directly (use interface)
- Store importing proxy or CLI

---

## Coding Standards

### Go Style

1. **Format with gofmt:**
   ```bash
   gofmt -w internal/
   ```

2. **Function size:** Keep under 50 lines, preferably under 30

3. **Naming:**
   - `CamelCase` for exported
   - `camelCase` for unexported
   - `ALL_CAPS` for constants
   - No underscores in Go names (except in tests)

4. **Error handling:**

```go
// Good - wrap with context
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Good - check and return early
result, err := fetchData()
if err != nil {
    return nil, err
}

// Bad - swallowing error
doSomething() // ignoring error!

// Bad - generic messages
if err != nil {
    return fmt.Errorf("error") // no context!
}
```

5. **Logging:**

```go
// Use log package with prefix
log.Printf("[hooktm] processing webhook: %s", id)

// For errors in CLI commands
return fmt.Errorf("[hooktm] failed to list webhooks: %w", err)
```

### Comments

```go
// Package store provides database operations for webhook storage.
package store

// Webhook represents a captured webhook request.
type Webhook struct {
    // ID is a unique nano ID for the webhook.
    ID string
}

// InsertWebhook stores a webhook in the database.
// Returns an error if the database write fails.
func (s *Store) InsertWebhook(w *Webhook) error
```

---

## Database Guidelines

### Schema Changes

1. Always update `internal/store/migrate.go`
2. Use sequential migration numbers
3. Make migrations idempotent
4. Test migrations on fresh and existing databases

```go
// Example migration
{
    Version: 2,
    Name:    "add_webhook_fts",
    SQL: `
        CREATE VIRTUAL TABLE IF NOT EXISTS webhooks_fts ...
    `,
},
```

### Query Guidelines

- Use prepared statements via `database/sql`
- Parameterize ALL user input
- Use transactions for multi-step operations
- Index columns used in WHERE clauses

### Testing with Database

```go
func setupTestDB(t *testing.T) *Store {
    t.Helper()
    
    // Use in-memory database
    db, err := sql.Open("sqlite3", ":memory:")
    if err != nil {
        t.Fatalf("failed to open test db: %v", err)
    }
    
    store := &Store{db: db}
    if err := store.Migrate(); err != nil {
        t.Fatalf("failed to migrate: %v", err)
    }
    
    return store
}
```

---

## Adding New Features

### Checklist

- [ ] Write tests first (TDD)
- [ ] Implement feature
- [ ] All tests pass
- [ ] Code is formatted (`gofmt`)
- [ ] No linting errors (`go vet ./...`)
- [ ] Documentation updated (if public API)
- [ ] CHANGELOG.md updated
- [ ] ROADMAP.md updated (if applicable)

### Adding a Provider Template

1. Add detection in `internal/provider/detect.go`
2. Add tests in `internal/provider/detect_test.go`
3. Add codegen template in `internal/codegen/render.go`
4. Add tests in `internal/codegen/render_test.go`
5. Update README.md provider table
6. Update CHANGELOG.md

### Adding a CLI Command

1. Create `internal/cli/<command>.go`
2. Add command to `internal/cli/app.go`
3. Update `internal/cli/normalize_args.go` if flags need special handling
4. Add tests in `internal/cli/<command>_test.go`
5. Update README.md
6. Update CHANGELOG.md

---

## Error Messages

### User-Facing Errors (CLI)

Be helpful and actionable:

```go
// Good
return fmt.Errorf("webhook not found: %s (run 'hooktm list' to see available)", id)

// Good
return fmt.Errorf("database locked - another instance of hooktm may be running")

// Bad
return fmt.Errorf("error") // too vague

// Bad
return err // no context
```

### Internal Errors

Wrap for debugging:

```go
return fmt.Errorf("store.ListSummaries failed: %w", err)
```

---

## Performance Considerations

### Database

- Use WAL mode (already configured)
- Limit concurrent connections (`MaxOpenConns(1)` for SQLite)
- Cap body text for FTS (200KB max)

### HTTP

- Body size limit: 10MB
- Timeouts on all HTTP clients
- Close response bodies

### Memory

- Stream large responses when possible
- Don't load entire database into memory
- Use pagination for list commands

---

## Testing Best Practices

### Unit Tests

```go
func TestProviderDetection(t *testing.T) {
    tests := []struct {
        name     string
        headers  http.Header
        body     []byte
        wantProv string
        wantSig  string
    }{
        {
            name:     "stripe signature",
            headers:  http.Header{"Stripe-Signature": []string{"v1=abc"}},
            wantProv: "stripe",
            wantSig:  "v1=abc",
        },
        // ...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            prov, _, sig := Detect(tt.headers, tt.body)
            if prov != tt.wantProv {
                t.Errorf("provider = %q, want %q", prov, tt.wantProv)
            }
            if sig != tt.wantSig {
                t.Errorf("signature = %q, want %q", sig, tt.wantSig)
            }
        })
    }
}
```

### Integration Tests

```go
func TestReplayIntegration(t *testing.T) {
    // Start test server
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Verify request
        w.WriteHeader(http.StatusOK)
    }))
    defer ts.Close()
    
    // Replay webhook
    engine := replay.New(nil)
    err := engine.Replay(context.Background(), webhook, ts.URL)
    
    // Assert
    if err != nil {
        t.Fatalf("replay failed: %v", err)
    }
}
```

### Mocking

Use interfaces for testability:

```go
type WebhookStore interface {
    InsertWebhook(w *Webhook) error
    GetWebhook(id string) (*Webhook, error)
    ListSummaries(filter Filter) ([]Summary, error)
}
```

---

## Git Workflow

### Commits

Format: `<type>: <description>`

Types:
- `feat:` - New feature
- `fix:` - Bug fix
- `test:` - Test-only changes
- `refactor:` - Code restructuring
- `docs:` - Documentation
- `chore:` - Maintenance

Examples:
```
feat: add webhook filtering by date range
fix: handle nil body in replay engine
test: add tests for provider detection
refactor: extract common db operations
docs: update README with new commands
```

### Branches

- `main` - Production-ready
- `feature/<name>` - New features
- `fix/<name>` - Bug fixes
- `refactor/<name>` - Refactoring

---

## Review Checklist (Self-Review Before Submitting)

- [ ] Does it solve the problem completely?
- [ ] Are there tests covering the new code?
- [ ] Do all tests pass?
- [ ] Is the code formatted?
- [ ] Are errors handled properly?
- [ ] Is the code documented?
- [ ] Are there any TODOs that should be addressed?
- [ ] Is the CHANGELOG updated?

---

## Common Pitfalls to Avoid

1. **Don't ignore errors** - Always handle errors explicitly
2. **Don't use global state** - Pass dependencies as parameters
3. **Don't break existing tests** - Fix or update tests when changing behavior
4. **Don't mix concerns** - Keep CLI, business logic, and storage separate
5. **Don't forget cleanup** - Close files, connections, temp resources
6. **Don't panic** - Return errors, don't panic in library code

---

## Quick Reference

```bash
# Run all tests
go test ./...

# Run with race detection
go test -race ./...

# Format code
go fmt ./...

# Vet code
go vet ./...

# Build
go build ./cmd/hooktm

# Run specific test
go test -v -run TestProviderDetection ./internal/provider

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

---

## Questions?

When in doubt:
1. Check existing code in the same package
2. Look at tests for examples
3. Follow TDD: write the test first
4. Keep it simple and minimal

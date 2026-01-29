# Contributing to HookTM

Thank you for your interest in contributing to HookTM!

## Development Setup

### Prerequisites

- Go 1.22 or later
- Git

### Getting Started

```bash
# Clone the repository
git clone https://github.com/yourusername/hooktm.git
cd hooktm

# Build
go build ./cmd/hooktm

# Run tests
go test ./...
```

## Project Structure

```
hooktm/
├── cmd/hooktm/          # CLI entry point
├── internal/
│   ├── cli/             # CLI commands (listen, list, show, replay, ui, codegen)
│   ├── proxy/           # HTTP reverse proxy and recorder
│   ├── store/           # SQLite database operations
│   ├── replay/          # Webhook replay engine
│   ├── codegen/         # Code generation templates
│   ├── tui/             # Terminal UI (bubbletea)
│   ├── provider/        # Webhook provider detection
│   ├── config/          # YAML configuration
│   └── urlutil/         # Shared URL utilities
├── go.mod
├── go.sum
└── README.md
```

## Making Changes

### 1. Create a Branch

```bash
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes

- Follow existing code style
- Add tests for new functionality
- Update documentation if needed

### 3. Run Tests

```bash
go test ./...
go build ./cmd/hooktm
```

### 4. Commit Your Changes

Write clear, concise commit messages:

```bash
git commit -m "Add support for Shopify webhook detection"
```

### 5. Submit a Pull Request

Push your branch and open a PR with:
- Clear description of changes
- Any related issues
- Screenshots for UI changes

## Code Guidelines

### Go Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Keep functions focused and small
- Handle errors explicitly (don't ignore with `_`)

### Error Handling

```go
// Good
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Avoid
doSomething() // ignoring error
```

### Logging

Use the `log` package with `[hooktm]` prefix:

```go
log.Printf("[hooktm] processing webhook: %s", id)
```

## Adding a New Provider

1. Edit `internal/provider/detect.go`
2. Add detection logic in `Detect()` function
3. Add tests in `internal/provider/detect_test.go`
4. Update README.md provider table

Example:

```go
// In Detect()
if sig := h.Get("X-NewProvider-Signature"); sig != "" {
    return "newprovider", extractEventType(body), sig
}
```

## Adding a New Command

1. Create `internal/cli/newcmd.go`
2. Add command in `internal/cli/app.go`
3. Update `internal/cli/normalize_args.go` if needed
4. Add documentation to README.md

## Testing

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/store

# With coverage
go test -cover ./...

# Verbose
go test -v ./...
```

### Writing Tests

Place tests in `*_test.go` files next to the code:

```go
func TestSomething(t *testing.T) {
    // Arrange
    input := "test"

    // Act
    result := DoSomething(input)

    // Assert
    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}
```

## Reporting Issues

When reporting bugs, include:

1. HookTM version (`go version`)
2. Operating system
3. Steps to reproduce
4. Expected vs actual behavior
5. Relevant logs or error messages

## Feature Requests

Open an issue with:

1. Clear description of the feature
2. Use case / why it's needed
3. Proposed implementation (optional)

## Questions?

Open a discussion or issue — we're happy to help!

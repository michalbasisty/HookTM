# Changelog

All notable changes to HookTM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Core Features
- **Structured Logging**: New logger package with configurable levels and formats
  - Log levels: `debug`, `info`, `warn`, `error`
  - Output formats: `text` (human-readable), `json` (machine-parseable)
  - `--log-level` and `--log-format` flags on `listen` command
  - Correlation ID support for request tracing
- **Configuration Validation**: Automatic validation of config file values
  - Port range validation (1-65535)
  - URL format validation for forward targets
  - Path traversal prevention for database paths
  - Language code validation
  - `IsValidationError()` helper for error handling
- **Context Cancellation**: Full context support throughout the application
  - Graceful shutdown on SIGINT/SIGTERM
  - Request cancellation returns 503 Service Unavailable
  - Database operations respect context timeouts
  - HTTP requests are context-aware
- **Performance Benchmarks**: Comprehensive benchmark suite
  - Benchmarks for all store operations
  - Concurrent operation testing
  - Performance baselines established

#### CLI Improvements
- `replay` command now supports `--ci` flag for CI/automation mode
  - Exit code 0: Success (2xx response)
  - Exit code 1: Connection error (network/DNS/timeout)
  - Exit code 2: HTTP error (4xx/5xx response)
  - Exit code 3: Other error (not found, invalid input, etc.)
- `list` command now supports `--from` and `--to` flags for date range filtering
  - Supports ISO 8601 format: `2024-01-15T10:30:00Z`
  - Supports date only: `2024-01-15` (uses start/end of day)
  - Supports relative duration: `1d`, `7d`, `1h` (e.g., `hooktm list --from 7d`)
- `delete` command to delete webhooks by ID or filter (--older-than, --provider, --status)
- `OpenContext()` method for context-aware database opening

#### Testing
- **Integration Tests**: Complete integration test suite
  - Full flow: capture → store → replay
  - Concurrent webhook handling
  - Provider detection verification
  - Error scenario testing
- **Unit Test Coverage**: Significantly improved coverage
  - `replay`: 96%+ coverage
  - `config`: 91%+ coverage
  - `proxy`: 88%+ coverage
  - `store`: 80%+ coverage
  - `tui`: 84%+ coverage
  - `logger`: 83%+ coverage
  - `urlutil`: 93%+ coverage

#### Documentation
- Updated README with new features and performance data
- Updated ARCHITECTURE with context flow diagrams
- Updated CLI reference with new flags

### Security
- Request body size limit (10 MB) to prevent memory exhaustion
- FTS5 query sanitization to prevent injection
- Path traversal prevention in database paths (`..` detection)
- Config validation prevents invalid URLs and ports

### Performance
- SQLite WAL mode for concurrent reads during writes
- Single connection pool for simplicity
- Prepared statements via database/sql
- Body text capped at 200KB for FTS

### Developer Experience
- Comprehensive test suite with `go test ./...`
- Benchmark suite with `go test -bench=. ./internal/store`
- Example configuration via `config.ExampleConfig()`
- Better error messages with field-level validation

## [0.2.0] - 2026-01-30

### Added
- HTTP proxy for capturing webhooks
- SQLite storage with WAL mode
- Full-text search across webhook bodies
- Terminal UI for browsing webhooks
- Webhook replay with JSON merge patch support (RFC 7396)
- Code generation for signature validation (Go, TypeScript, Python, PHP, Ruby)
- Provider detection for Stripe, GitHub, Slack, Shopify, Twilio
- Record-only mode (no forward target required)
- Configuration via YAML file and environment variables
- Graceful shutdown with signal handling
- CI mode for replay command with exit codes

---

## Version History

### Versioning Scheme

- **MAJOR**: Breaking changes to CLI or config format
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

### Upgrade Notes

#### Upgrading to 0.2.0

No breaking changes. New features are backward compatible:
- Existing config files continue to work
- New logging flags are optional
- Context cancellation is automatic

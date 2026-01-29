# Changelog

All notable changes to HookTM will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- `list` command now supports `--from` and `--to` flags for date range filtering
  - Supports ISO 8601 format: `2024-01-15T10:30:00Z`
  - Supports date only: `2024-01-15` (uses start/end of day)
  - Supports relative duration: `1d`, `7d`, `1h` (e.g., `hooktm list --from 7d`)
- `delete` command to delete webhooks by ID or filter (--older-than, --provider, --status)
- Initial release of HookTM
- HTTP proxy for capturing webhooks
- SQLite storage with WAL mode
- Full-text search across webhook bodies
- Terminal UI for browsing webhooks
- Webhook replay with JSON merge patch support
- Code generation for signature validation (Go, TypeScript, Python, PHP, Ruby)
- Provider detection for Stripe, GitHub, Slack, Shopify, Twilio
- Record-only mode (no forward target required)
- Configuration via YAML file and environment variables

### Security
- Request body size limit (10 MB) to prevent memory exhaustion
- FTS5 query sanitization to prevent injection
- Proper error handling and logging

## [0.1.0] - 2024-XX-XX

### Added
- Initial public release

---

## Version History

### Versioning Scheme

- **MAJOR**: Breaking changes to CLI or config format
- **MINOR**: New features, backward compatible
- **PATCH**: Bug fixes, backward compatible

### Upgrade Notes

#### Upgrading to 0.1.0

No special steps required for initial release.

# Development Roadmap

## Phase 1: MVP (Complete)

**Goal:** Working proxy + recorder + replay + TUI

| Week | Deliverable | Status |
|------|-------------|--------|
| 1 | HTTP proxy with forwarding, SQLite storage | Done |
| 2 | Provider detection, CLI list/show commands | Done |
| 3 | TUI with browse/search/replay | Done |
| 4 | Codegen templates, docs | Done |

**Ship: v0.1.0** — *"It captures and replays webhooks"*

---

## Phase 2: Polish

**Goal:** Production ready for daily use

- [ ] Full-text search in TUI
- [ ] Payload diff view (compare two webhooks)
- [ ] More provider templates (10+)
  - [ ] Shopify
  - [ ] Twilio
  - [ ] Slack
  - [ ] Discord
  - [ ] Linear
  - [ ] Paddle
  - [ ] Lemon Squeezy
  - [ ] SendGrid
  - [ ] Mailgun
  - [ ] PagerDuty
- [ ] Export to Postman/Insomnia collections
- [ ] CI mode (`hooktm replay --ci` with exit codes)
- [ ] Shell completions (bash, zsh, fish)
- [ ] Homebrew formula
- [x] Delete command (cleanup old webhooks)
- [ ] Webhook filtering by date range

**Ship: v0.2.0** — *"Production ready for daily use"*

---

## Phase 3: Network

**Goal:** Full ngrok replacement

- [ ] SSH-based tunnel (`hooktm tunnel --ssh user@vps`)
- [ ] Self-hosted relay server (optional)
- [ ] Secure webhook sharing (encrypted links)
- [ ] Custom domains support
- [ ] TLS termination

**Ship: v0.3.0** — *"Full ngrok replacement"*

---

## Phase 4: Monetization

**Goal:** Sustainable development

- [ ] Team workspaces (shared webhook history)
- [ ] Webhook analytics dashboard
- [ ] Priority support tier
- [ ] Cloud-hosted relay option
- [ ] Webhook alerting (Slack/Discord/email)

---

## Current Status

**Version:** 0.1.0-dev
**Phase:** 1 (MVP) - Complete
**Next:** Phase 2 - Polish

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to help with roadmap items.

Priority areas where contributions are welcome:
1. Provider templates (codegen)
2. Shell completions
3. Export formats
4. Documentation

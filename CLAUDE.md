# SignPost

A Docker-based local SMTP relay that DKIM-signs outgoing mail and relays through a configurable smarthost.

## Project Overview

- **What:** Local SMTP server with DKIM signing, web admin UI, DNS validation tools
- **Why:** Local services get email rejected due to DKIM/SPF/DMARC requirements
- **Stack:** Maddy (mail server) + Go (backend API) + React/Vite/Tailwind/shadcn (frontend) + SQLite
- **Container:** Single Docker container with s6-overlay managing Maddy + Go web app
- **Owner:** drose, domain drcs.ca on Cloudflare DNS, Gmail-hosted email
- **Prod server:** TrueNAS at `192.168.1.2` (aka `truenas.drcs.ca`), SSH as root, Dockge for GUI mgmt
- **Repo:** github.com/drose12/signpost (public)

## Key Files

- `docs/superpowers/specs/2026-03-29-signpost-design.md` — **full design spec** (architecture, API surface, DB schema, all features, implementation phases)
- `templates/maddy.conf.tmpl` — Go template that generates Maddy's config
- `internal/db/` — SQLite schema, migrations, domain/relay/settings/maillog queries
- `internal/config/` — Maddy config generator (reads DB → renders template → writes file)
- `internal/api/` — REST API handlers (chi router, basic auth)
- `internal/crypto/` — AES-256-GCM encryption for relay credentials (HKDF-SHA256 key derivation)
- `internal/dkim/` — DKIM key generation (RSA 2048, PKCS#8 PEM) and DNS record builders
- `cmd/signpost/main.go` — entrypoint (initializes DB, generates config, starts HTTP server)
- `web/` — React frontend (not yet built, Phase 2)
- `rootfs/` — s6-overlay service definitions for container process management
- `Dockerfile` — multi-stage build (Go builder → foxcpp/maddy:0.9.2 + s6-overlay)

## Development

```bash
# Go is at /usr/local/go/bin (add to PATH if needed)
export PATH=$PATH:/usr/local/go/bin

# Run all tests (51 tests across 4 packages)
CGO_ENABLED=1 go test -race ./internal/...

# Run specific package tests
go test -v ./internal/db/
go test -v ./internal/api/
go test -v ./internal/dkim/
go test -v ./internal/config/

# Build Docker image
docker build -t signpost:dev .

# Run locally with Docker (dev mode)
docker compose -f docker-compose.dev.yml up --build

# Quick smoke test against running container
curl http://localhost:8080/api/v1/healthz
curl -u admin:yourpass http://localhost:8080/api/v1/domains
```

## Architecture Decisions

- Single container (s6-overlay) over two-container docker-compose — simpler for users
- Maddy over Postfix+OpenDKIM — unified config, modern, active project (5.9k stars)
- SQLite over PostgreSQL — zero-config, single file, appropriate scale
- Config generation (Go templates → maddy.conf) — web app owns config, never edit manually
- Web app starts first via s6-rc dependency, generates maddy.conf, then Maddy starts
- Relay credentials encrypted with AES-256-GCM, key derived from SIGNPOST_SECRET_KEY via HKDF-SHA256
- Configurable relay method per domain: Gmail, ISP, direct, custom SMTP
- Configurable auth: network trust on port 25, SMTP AUTH on port 587

## Environments

- **Dev:** Local Win 11 WSL Ubuntu + Docker Desktop, IP `192.168.1.19` (self-signed TLS, network trust, `docker-compose.dev.yml`, healthz at `localhost:8081`). Proxied via Nginx Proxy Manager with LE cert → `http://192.168.1.19:8081`
- **Prod:** TrueNAS at `192.168.1.2` (aka `truenas.drcs.ca`), SSH as `root@truenas.drcs.ca`. Container runs as standalone `docker run` (not compose), image `ghcr.io/drose12/signpost:latest`. Dockge available for GUI management. Proxied via Nginx Proxy Manager with LE cert → `http://192.168.1.2:8080`.

## Implementation Status

### Phase 1 — MVP Core (mostly complete)
- [x] 1.1: Project scaffolding (Go module, git, directories, .gitignore, .env.example, CI/CD configs)
- [x] 1.2: Database layer — 12 tests (SQLite, WAL, migrations, domain/relay/settings/maillog CRUD)
- [x] 1.3: Maddy config generation — 13 tests (Go templates, backup/rollback, real template rendering)
- [x] 1.4: DKIM key management — 9 tests (RSA 2048, PKCS#8, DNS TXT record generation)
- [x] 1.5: Docker container — verified (s6-overlay, healthz working, API accessible)
- [x] 1.6: REST API core — 17 tests (domains CRUD, DKIM gen, DNS records, relay config, settings, logs, test send stub, basic auth)
- [ ] **1.7: Integration tests** — Testcontainers (container startup, SMTP send, DKIM verification, relay to mock SMTP). Must also serve as the upgrade regression gate — if these pass, an upgrade is safe.
- [x] 1.8: CI/CD + README (GitHub Actions ci.yml + release.yml, Dependabot, README, CHANGELOG)

### Phase 2 — Web UI (complete)
- [x] 2.1: Backend prerequisites (SPA handler, embed.FS, extend status with Maddy TCP check, implement test send via SMTP)
- [x] 2.2: Frontend scaffolding (Vite 8 + React 19 + TypeScript + Tailwind v4 + shadcn/ui)
- [x] 2.3: API client + TypeScript types (7 interfaces, fetch wrapper with basic auth)
- [x] 2.4: Layout shell (dark sidebar, React Router v7, theme toggle, login dialog)
- [x] 2.5: Dashboard page (status cards, recent activity table, empty state)
- [x] 2.6: Domains page — list + DNS records tab (add domain dialog, copy-to-clipboard)
- [x] 2.7: Domains page — relay config, DKIM keys, settings tabs (full CRUD)
- [x] 2.8: Mail log viewer (filterable, load-more pagination)
- [x] 2.9: Setup wizard (vertical checklist, 5-step flow, first-run detection)
- [x] 2.10: Dockerfile update (3-stage: Node 20 + Go 1.24 + Maddy)
- [x] 2.11: Frontend tests — 5 Vitest tests (API client + wizard rendering)
- [x] 2.12: Final integration verification (Docker build, SPA served, all APIs working)

### Phase 3 — TLS & Security (in progress)
- [x] 3.1: Let's Encrypt ACME DNS-01 via Cloudflare, SMTPS port 465, TLS management UI
- [x] 3.2: TLS — self-signed cert generation at startup, TLS management card on Dashboard
- [ ] 3.3: Security audit page
- [x] 3.4: SMTP user management — CRUD UI, bcrypt hashing (bcrypt: prefix for Maddy), port 587 control
- [ ] 3.5: Security tests
- [x] AES-256-GCM credential encryption (internal/crypto package, 10 tests)

### Phase 4 — Polish & Release (mostly complete)
- [x] 4.1: Backup/restore
- [ ] 4.2: Multi-domain support
- [x] 4.3: Documentation (README with setup guide, DNS config, relay setup, troubleshooting, full API reference)
- [x] 4.4: Release automation (tag → ghcr.io → GitHub Release)
- [ ] 4.5: Production hardening (version pinning strategy, upgrade-test CI job, Dependabot → integration test gate)

## Known Issues / TODOs for Next Session

1. **ISP RBL block** — Home IP `206.45.58.220` was blacklisted by MTS. Direct delivery works again as of v0.7.0 testing. May recur with high SMTP volume.
2. **Integration tests** — Phase 1.7 not started. Need Testcontainers.
3. **Config reload race** — Rapid API calls can cause double SIGHUP. s6 handles it but could debounce.
4. **Let's Encrypt ACME** — Phase 3.1, not started. Self-signed certs work for now.

## Current Version

v0.11.0 — Let's Encrypt ACME, SMTPS port 465, TLS management UI

## Deployment Process

**DO NOT auto-deploy after every change.** Wait for the user to say "deploy", then:

### Pre-deploy (both environments)
1. Run all Go tests: `CGO_ENABLED=1 go test -race ./internal/...`
2. Run frontend tests: `cd web && npx vitest run`
3. Run frontend build: `cd web && npm run build`
4. If tests/build pass: bump version in `cmd/signpost/main.go`
5. Add release notes to `CHANGELOG.md` (Keep a Changelog format)
6. Commit, tag, push with `--tags`
7. Update GitHub Release notes: `gh release edit <tag> --notes "..."`

### Dev (local WSL Docker Desktop)
1. `docker compose -f docker-compose.dev.yml up --build -d`
2. Verify: `curl http://localhost:8081/api/v1/healthz`

### Prod (TrueNAS — truenas.drcs.ca / 192.168.1.2)
1. Wait for GitHub Actions Release workflow to finish: `gh run watch <run-id>`
2. Pull new image: `ssh root@truenas.drcs.ca "docker pull ghcr.io/drose12/signpost:latest"`
3. Recreate container (**docker restart is not enough** — it reuses the old image):
   ```
   ssh root@truenas.drcs.ca "docker stop signpost && docker rm signpost && docker run -d --name signpost --restart unless-stopped -p 25:25 -p 587:587 -p 8080:8080 -v signpost_signpost-data:/data/signpost -e SIGNPOST_ADMIN_USER=admin -e SIGNPOST_ADMIN_PASS=dBbVvLAcHu3dAaa9FEnc -e SIGNPOST_ENV=prod -e SIGNPOST_DOMAIN=drcs.ca -e SIGNPOST_SECRET_KEY=Q4mZ8tN1xK7pR2vH9cL5yW3dS6jF0bTu ghcr.io/drose12/signpost:latest"
   ```
4. Verify: `ssh root@truenas.drcs.ca "curl -s http://localhost:8080/api/v1/healthz"`

Only commit code when changes are ready. Do not rebuild the container on every file change.

## How to Pick Up

1. Read this file
2. Run `CGO_ENABLED=1 go test -race ./internal/...` to verify Go tests (100+ tests across 6 packages)
3. Run `cd web && npx vitest run` to verify frontend tests (5 tests)
6. **Phase 2 Web UI is complete.** Next priorities:
   - Phase 1.7: Integration tests (Testcontainers)
   - Phase 3: TLS & Security (ACME, security audit page)

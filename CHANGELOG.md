# Changelog

All notable changes to SignPost will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.6.0] - 2026-04-02

### Added
- About page with version info, tech stack versions, and links
- Release Notes page rendering CHANGELOG.md (version clickable in sidebar)
- SMTP health check in Dockerfile HEALTHCHECK (checks both HTTP API and SMTP port)
- Upgrade procedure documentation (`docs/UPGRADE.md`)
- Security procedures documentation (`docs/SECURITY-PROCEDURES.md`)
- Security policy (`SECURITY.md`)
- CodeQL workflow for Go + TypeScript static analysis
- GHCR image publishing via GitHub Actions on tag
- Theme toggle moved to sidebar header (next to logo)

### Fixed
- Node.js bumped from 20 to 22 LTS in Dockerfile
- golang.org/x/crypto upgraded to v0.45.0 (2 CVE fixes)
- go-sqlite3 upgraded to v1.14.38
- GitHub Actions dependencies updated to latest versions

## [0.5.0] - 2026-03-31

### Added
- Full system backup/restore on Dashboard (all domains, SMTP users, settings)
- SMTP user export/import
- Clear mail log button
- Domain config export/import (JSON with DKIM keys and relay passwords)
- DKIM key export/import (PEM files)

### Fixed
- Foreign key constraint on domain delete (manually delete dependents)
- msmtpd hidden from listeners when not active
- Favicon and page title updated to SignPost branding

## [0.4.0] - 2026-03-31

### Added
- Comprehensive README with Docker Hub setup, compose examples, full API reference
- DNS TTL column (replaces generic "24-48 hours" warning)
- Egress hostname field for dynamic DNS SPF
- Broken SPF `include:` detection (catches permerror-causing entries)
- Public IP auto-detection for direct delivery SPF recommendations

### Changed
- Domains page redesigned: cards instead of tabs
- Relay config shows sub-cards per method with active/configured state
- Multi-method relay persistence (all methods saved in DB, one active)
- LOGIN auth warning updated to reflect msmtpd proxy

## [0.3.0] - 2026-03-31

### Added
- AES-256-GCM encryption for relay credentials at rest
- Self-signed TLS certificate auto-generation for STARTTLS
- SMTP user management (CRUD, bcrypt hashing, port 587 control)
- Port 25/587 enable/disable toggles on Dashboard
- msmtpd + msmtp relay proxy for LOGIN-auth ISP servers
- Go-based DKIM signing for LOGIN relay bypass
- Relay connection test with LOGIN auth fallback
- Password show/hide eye toggle on relay config and SMTP users
- Copy-to-clipboard buttons throughout UI

### Fixed
- Maddy `bcrypt:` hash prefix (not `{bcrypt}`)
- WAL checkpoint after SMTP user changes for Maddy auth
- Docker DNS cache bypass (queries Cloudflare 1.1.1.1 directly)
- Quoted relay auth credentials in Maddy config (spaces in app passwords)
- Self-signed TLS for STARTTLS on port 587

## [0.2.0] - 2026-03-31

### Added
- DNS awareness with live lookups (SPF, DKIM, DMARC comparison)
- SPF merge logic and broken include detection
- Setup Wizard restructured: 6 steps (Domain → Method → Relay → DKIM → DNS → Test)
- Dashboard: status cards, listeners, send test email, TLS management
- Domains page with DNS Records, Relay Config, DKIM Keys, Settings tabs
- Mail Log page with filtering and load-more pagination
- Dark/light mode toggle
- Login dialog with basic auth

## [0.1.0] - 2026-03-29

### Added
- Initial release
- Go backend with REST API (chi router, basic auth)
- SQLite database with WAL mode and schema migrations
- Maddy config template generation from database state
- DKIM key generation (RSA 2048, PKCS#8 PEM)
- Docker container with s6-overlay (Maddy + SignPost Web)
- React + TypeScript + Tailwind CSS + shadcn/ui frontend
- 51 unit tests across 4 packages

# Changelog

All notable changes to SignPost will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- About page with version info, tech stack, and links
- Release Notes page that renders CHANGELOG.md with card-based layout
- Version number in sidebar is now clickable (links to Release Notes)
- About link in sidebar navigation
- Backend `GET /api/v1/changelog` endpoint serving raw changelog content
- SMTP port health check in Dockerfile HEALTHCHECK (checks both HTTP API and SMTP)
- Upgrade procedure documentation (`docs/UPGRADE.md`)

## [0.1.0] - 2026-03-29

### Added
- Project scaffolding with Go module and Docker setup
- SQLite database layer with WAL mode and schema migrations
- Domain CRUD operations via REST API
- DKIM key generation (RSA 2048, PKCS#8 PEM) and DNS record builder
- Maddy config template generation from database state
- Config backup and rollback support
- REST API with basic auth (chi router)
- Health check endpoint (`/api/v1/healthz`)
- DNS record helpers (SPF, DKIM, DMARC generation)
- DNS validation and checking against live DNS
- Relay configuration (Gmail, ISP, direct, custom SMTP)
- Relay connection testing
- Mail log tracking with filtering
- SMTP user management for port 587 submission
- TLS self-signed certificate generation
- System backup and restore (JSON export/import)
- Domain config export and import
- Public IP detection for DNS configuration
- Docker container with s6-overlay (Maddy + SignPost Web)
- Docker Compose files for dev and prod environments
- GitHub Actions CI/CD pipelines (ci.yml + release.yml)
- Dependabot configuration for Go, npm, Docker, and GitHub Actions
- Web UI with React, TypeScript, Tailwind CSS, and shadcn/ui
- Dashboard with status overview, test email, and backup/restore
- Domain management with DNS records, DKIM, relay config tabs
- Mail log viewer with filtering and pagination
- SMTP user management page
- Setup wizard with 5-step guided flow
- Dark mode support
- 51 unit tests across 4 packages

# Changelog

All notable changes to SignPost will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Project scaffolding with Go module and Docker setup
- SQLite database layer with schema migrations
- Domain CRUD operations via REST API
- DKIM key generation (RSA 2048) and DNS record builder
- Maddy config template generation from database state
- Config backup and rollback support
- REST API with basic auth (chi router)
- Health check endpoint (`/api/v1/healthz`)
- DNS record helper (SPF, DKIM, DMARC generation)
- Relay configuration (Gmail, ISP, direct, custom)
- Mail log tracking
- Docker container with s6-overlay (Maddy + SignPost Web)
- Docker Compose files for dev and prod
- GitHub Actions CI/CD pipelines
- Dependabot configuration for Go, npm, Docker, and GitHub Actions
- 47 unit tests across 4 packages

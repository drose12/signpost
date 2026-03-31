# SignPost

A Docker-based SMTP relay that properly DKIM-signs outgoing emails and guides you through correct DNS configuration.

## What It Does

SignPost sits between your local services and the internet, ensuring every outgoing email is properly authenticated:

- **Accepts email** from local services via SMTP (port 25) or authenticated submission (port 587)
- **Signs with DKIM** using per-domain RSA keys
- **Relays** through configurable smarthosts (Gmail, ISP, or direct delivery)
- **Web admin UI** for domain management, DNS validation, and testing
- **Guided setup** with built-in explanations and DNS record generation

## Quick Start

1. Clone and create your environment file:

```bash
git clone https://github.com/drose-drcs/signpost.git
cd signpost
cp .env.example .env
# Edit .env with your domain and credentials
```

2. Start with Docker Compose:

```bash
# Development
docker compose -f docker-compose.dev.yml up --build

# Production
docker compose -f docker-compose.prod.yml up -d
```

3. Open the web UI at `http://localhost:8080` and follow the setup wizard.

## Configuration

All configuration is done through environment variables and the web admin UI.

### Required Environment Variables

| Variable | Description |
|----------|-------------|
| `SIGNPOST_DOMAIN` | Your email domain (e.g., `drcs.ca`) |
| `SIGNPOST_SECRET_KEY` | Master encryption key (min 32 chars) |
| `SIGNPOST_ADMIN_PASS` | Web UI admin password |

### Optional Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SIGNPOST_ENV` | `prod` | Environment (`dev` or `prod`) |
| `SIGNPOST_HOSTNAME` | `mail.$DOMAIN` | SMTP hostname |
| `SIGNPOST_ADMIN_USER` | `admin` | Web UI username |
| `SIGNPOST_WEB_PORT` | `8080` | Web UI port |
| `SIGNPOST_SMTP_PORT` | `25` | SMTP port |
| `SIGNPOST_SUBMISSION_PORT` | `587` | Submission port |
| `SIGNPOST_LOG_LEVEL` | `info` | Log level |
| `SIGNPOST_ACME_EMAIL` | — | Let's Encrypt email |
| `SIGNPOST_CF_API_TOKEN` | — | Cloudflare API token (for LE DNS-01) |

## Architecture

SignPost runs as a single Docker container with two processes managed by s6-overlay:

- **Maddy** — all-in-one mail server (SMTP, DKIM, SPF/DMARC)
- **SignPost Web** — Go API + React admin UI

```
Local Service → SMTP :25 or :587
  → Maddy (DKIM sign)
  → Relay (Gmail/ISP/direct)
  → Recipient
```

## API

The REST API is available at `/api/v1/`. Authentication is via HTTP Basic Auth.

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/healthz` | GET | Health check (no auth) |
| `/api/v1/status` | GET | Dashboard data |
| `/api/v1/domains` | GET/POST | List/create domains |
| `/api/v1/domains/:id` | GET/DELETE | Get/delete domain |
| `/api/v1/domains/:id/dkim/generate` | POST | Generate DKIM key |
| `/api/v1/domains/:id/dns` | GET | Required DNS records |
| `/api/v1/domains/:id/relay` | GET/PUT | Relay configuration |
| `/api/v1/settings` | GET/PUT | Global settings |
| `/api/v1/logs` | GET | Mail logs |
| `/api/v1/test/send` | POST | Send test email |

## Development

### Prerequisites

- Go 1.24+
- Node.js 22+ (for frontend, Phase 2)
- Docker

### Running Tests

```bash
# All Go tests
go test -race ./...

# Specific package
go test -v ./internal/db/
go test -v ./internal/api/
go test -v ./internal/dkim/
go test -v ./internal/config/
```

### Building

```bash
# Docker image
docker build -t signpost:dev .

# Go binary only
go build -o signpost ./cmd/signpost/
```

## License

MIT

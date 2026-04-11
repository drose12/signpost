# SignPost

**A Docker-based SMTP relay that DKIM-signs outgoing emails, relays through your preferred smarthost, and guides you through DNS configuration.**

SignPost sits between your local services (NAS, home automation, monitoring, Proxmox, TrueNAS, Synology, Unraid) and the internet, ensuring every outgoing email passes SPF, DKIM, and DMARC checks. No more emails landing in spam.

- **Web Admin UI** -- Dashboard, domain management, DNS validation, setup wizard
- **Let's Encrypt TLS** -- ACME DNS-01 via Cloudflare, configurable from the Dashboard
- **DKIM Signing** -- Per-domain RSA-2048 keys, one-click generation
- **DNS Awareness** -- Live SPF/DKIM/DMARC checks with fix suggestions
- **Multiple Relay Methods** -- Gmail, ISP, direct delivery, or custom SMTP
- **Three SMTP Ports** -- 25 (plaintext), 587 (STARTTLS), 465 (implicit TLS/SSL)
- **Authenticated Submission** -- Ports 587/465 with per-user credentials
- **Real-time Mail Logging** -- Live log capture from Maddy with search, filtering, queue visibility
- **Credential Encryption** -- AES-256-GCM at rest for relay passwords and API tokens
- **Single Container** -- Maddy + Go API + React UI, managed by s6-overlay

![SignPost Login](docs/images/screenshot.png)

## Quick Start

### 1. Pull the image

```bash
docker pull ghcr.io/drose12/signpost:latest
```

### 2. Create a docker-compose.yml

```yaml
services:
  signpost:
    image: ghcr.io/drose12/signpost:latest
    ports:
      - "25:25"       # SMTP (local services, no auth)
      - "465:465"     # SMTPS (implicit TLS + auth)
      - "587:587"     # Submission (STARTTLS + auth)
      - "8080:8080"   # Web UI
    volumes:
      - signpost-data:/data/signpost
    environment:
      - SIGNPOST_DOMAIN=example.com
      - SIGNPOST_SECRET_KEY=change-me-to-something-at-least-32-chars-long
      - SIGNPOST_ADMIN_PASS=your-secure-admin-password
    restart: unless-stopped

volumes:
  signpost-data:
```

### 3. Start it

```bash
docker compose up -d
```

### 4. Open the Web UI

Navigate to `http://your-server:8080`. Log in with username `admin` and the password you set in `SIGNPOST_ADMIN_PASS`. The setup wizard will walk you through:

1. Adding your domain
2. Generating DKIM keys
3. Configuring DNS records
4. Choosing a relay method
5. Sending a test email

---

## Docker Compose Examples

### Standard

```yaml
services:
  signpost:
    image: ghcr.io/drose12/signpost:latest
    ports:
      - "25:25"         # SMTP (local services, no auth)
      - "465:465"       # SMTPS (implicit TLS + auth)
      - "587:587"       # Submission (STARTTLS + auth)
      - "8080:8080"     # Web UI
    volumes:
      - signpost-data:/data/signpost
    environment:
      - SIGNPOST_DOMAIN=example.com
      - SIGNPOST_SECRET_KEY=change-me-to-something-at-least-32-chars-long
      - SIGNPOST_ADMIN_PASS=your-secure-admin-password
    restart: unless-stopped

volumes:
  signpost-data:
```

### Behind a Reverse Proxy (Nginx Proxy Manager, Traefik, etc.)

Bind the web UI to localhost only -- your reverse proxy handles HTTPS for the UI:

```yaml
services:
  signpost:
    image: ghcr.io/drose12/signpost:latest
    ports:
      - "25:25"
      - "465:465"
      - "587:587"
      - "127.0.0.1:8080:8080"   # Web UI on localhost only
    volumes:
      - signpost-data:/data/signpost
    environment:
      - SIGNPOST_DOMAIN=drcs.ca
      - SIGNPOST_HOSTNAME=mail.drcs.ca      # Used for SMTP EHLO and TLS cert
      - SIGNPOST_SECRET_KEY=your-very-long-secret-key-at-least-32-characters
      - SIGNPOST_ADMIN_PASS=strong-admin-password
    restart: unless-stopped

volumes:
  signpost-data:
```

> **Dockge / TrueNAS:** Use a host path (`/mnt/pool/apps/signpost:/data/signpost`) instead of a named volume if your orchestrator prefers it.

---

## Configuration Reference

### Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `SIGNPOST_DOMAIN` | Yes | -- | Your email domain (e.g., `drcs.ca`) |
| `SIGNPOST_SECRET_KEY` | Yes | -- | Encryption key for relay credentials (min 32 chars) |
| `SIGNPOST_ADMIN_PASS` | Yes | -- | Web UI admin password |
| `SIGNPOST_ADMIN_USER` | No | `admin` | Web UI admin username |
| `SIGNPOST_ENV` | No | `prod` | Environment: `dev` or `prod` |
| `SIGNPOST_HOSTNAME` | No | `mail.$DOMAIN` | SMTP hostname used in EHLO and TLS certs |
| `SIGNPOST_WEB_PORT` | No | `8080` | Internal web UI port |
| `SIGNPOST_SMTP_PORT` | No | `25` | Internal SMTP port |
| `SIGNPOST_SUBMISSION_PORT` | No | `587` | Internal submission port |
| `SIGNPOST_LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |

### Port Mapping

| Port | Protocol | TLS | Auth | Purpose |
|---|---|---|---|---|
| 25 | SMTP | None | Network trust | Local services (NAS, printers, etc.) |
| 465 | SMTPS | Implicit TLS | SMTP AUTH | Clients with "SSL" checkbox (UDM Pro, etc.) |
| 587 | Submission | STARTTLS | SMTP AUTH | Standard authenticated submission |
| 8080 | HTTP | -- | Basic Auth | Web admin UI + REST API |

> **Important:** Port 25 relies on network trust -- only expose to your local/Docker network. Ports 465 and 587 require SMTP user credentials. Enable ports 465/587 and SMTPS from the Dashboard.

---

## Features

### Web Admin UI

A React-based admin interface with:

- **Dashboard** -- Service status, listener health, SMTP port toggles (25/465/587), TLS management with cert details
- **Domain Management** -- Add/remove domains, per-domain DKIM keys and relay config
- **DNS Records** -- View required records with copy-to-clipboard, live validation
- **Setup Wizard** -- Step-by-step first-run configuration
- **Mail Log** -- Real-time log with search, date filtering, status badges, queue visibility
- **SMTP Users** -- Manage submission credentials for ports 587/465
- **TLS Management** -- Self-signed or Let's Encrypt (ACME DNS-01), cert details, renewal
- **Backup/Restore** -- Full system backup including domains, DKIM keys, relay configs, SMTP users, TLS config
- **Dark Mode** -- System-aware theme toggle

### DKIM Signing

- RSA-2048 keys generated per domain
- PKCS#8 PEM format, stored in `/data/signpost/dkim_keys/`
- Export/import private keys (backup, migration between instances)
- DNS TXT record value generated automatically
- Selector configurable per domain (default: `signpost`)

### DNS Awareness

The DNS check feature (`/api/v1/domains/{id}/dns/check`) performs live lookups against Cloudflare DNS (1.1.1.1) and reports:

- **SPF** -- Checks for your SPF record, detects missing mechanisms for your relay method, identifies broken `include:` entries (hosts with no SPF record that cause permerror), suggests merged SPF records
- **DKIM** -- Compares published TXT record against your generated public key
- **DMARC** -- Verifies `_dmarc.` record exists
- **TTL Display** -- Shows current TTL values via raw DNS queries

### Relay Methods

SignPost supports four relay methods, configurable per domain:

| Method | Description | Auth | Use Case |
|---|---|---|---|
| **Gmail** | Relay through `smtp.gmail.com:587` | PLAIN (app password) | Gmail/Google Workspace users |
| **ISP** | Relay through your ISP's SMTP server | LOGIN or PLAIN (auto-detected) | ISP-provided email accounts |
| **Direct** | Deliver directly via MX lookup | None | Servers with clean IP reputation |
| **Custom** | Any SMTP server | PLAIN or LOGIN | Self-hosted relay, Mailgun, etc. |

**How relay testing works:** The web UI's "Test Connection" button establishes a real SMTP session to the relay, tries PLAIN auth first, then falls back to LOGIN. The detected auth method is persisted so subsequent sends use the correct mechanism.

**LOGIN auth support:** Many ISP mail servers only support the LOGIN SASL mechanism, which Go's `net/smtp` does not natively support. SignPost includes a custom LOGIN auth implementation for direct relay, and uses msmtpd as a local SMTP proxy for Maddy's relay (since Maddy only speaks PLAIN).

### SMTP User Management

- Create/delete users for port 587 authenticated submission
- Passwords hashed with bcrypt (Maddy-compatible `bcrypt:` prefix)
- Per-user enable/disable toggle
- Credentials stored in SQLite, referenced by Maddy's `auth.pass_table`

### TLS

- **Let's Encrypt** -- ACME DNS-01 via Cloudflare, configured from the Dashboard TLS card
- **Self-signed certificates** -- auto-generated at startup as default, switchable from the Dashboard
- Certificate details displayed on Dashboard: issuer, subject, SANs, expiry, days remaining
- Cloudflare API token stored encrypted in DB (AES-256-GCM), included in backup/restore
- Maddy handles certificate renewal automatically (30 days before expiry)
- Renew Now button for manual renewal
- Configurable mail hostname (`SIGNPOST_HOSTNAME` env var or via Dashboard)

### Credential Encryption

Relay passwords are encrypted at rest using AES-256-GCM:
- Key derived from `SIGNPOST_SECRET_KEY` via HKDF-SHA256
- Each password stored with its own random nonce
- Graceful migration: pre-encryption plaintext passwords are handled transparently

### Domain Backup/Restore

Export a domain's full configuration as a JSON file:
- Domain settings, DKIM selector
- DKIM private key (PEM)
- All relay configurations (with decrypted passwords)

Import on another instance to replicate the setup.

---

## Architecture

### Container Components

SignPost runs as a single Docker container with four processes managed by [s6-overlay](https://github.com/just-containers/s6-overlay):

| Process | Role |
|---|---|
| **SignPost Web** | Go HTTP server -- REST API + embedded React SPA |
| **Maddy** | Mail server -- SMTP listener, DKIM signing, PLAIN auth relay |
| **msmtpd** | Local SMTP proxy for LOGIN auth relays (only runs when needed) |
| **s6-overlay** | Process supervisor, dependency ordering, signal management |

Startup order: s6 starts SignPost Web first, which initializes the database and generates `maddy.conf`. Then Maddy starts using the generated config. msmtpd starts only if a LOGIN auth relay is configured.

### Mail Flow

**Gmail / PLAIN auth relay:**

```
Local Service --> SMTP :25 --> Maddy (DKIM sign) --> smtp.gmail.com:587 (PLAIN auth) --> Recipient
```

**ISP / LOGIN auth relay:**

```
Local Service --> SMTP :25 --> Maddy (DKIM sign) --> msmtpd :2500 --> msmtp (LOGIN auth) --> ISP SMTP --> Recipient
```

**Direct delivery:**

```
Local Service --> SMTP :25 --> Maddy (DKIM sign) --> MX lookup --> Recipient server
```

**Authenticated submission (port 587 STARTTLS / port 465 implicit TLS):**

```
Remote Client --> Submission :587/:465 (SMTP AUTH + TLS) --> Maddy (DKIM sign) --> Relay/Direct --> Recipient
```

### How DKIM Signing Works

1. Local service sends email to SignPost on port 25 (or 587/465 with auth)
2. Maddy matches the sender domain to a configured domain
3. Maddy signs the message with the domain's RSA-2048 private key
4. The signed message includes a `DKIM-Signature` header
5. The receiving server looks up the public key via DNS TXT record (`signpost._domainkey.example.com`)
6. If the signature validates, the email passes DKIM checks

### How LOGIN Auth Relay Works

Maddy only supports PLAIN SASL authentication for upstream relays. Many ISP mail servers require LOGIN auth. SignPost bridges this gap:

1. Maddy relays to msmtpd on `127.0.0.1:2500` (no auth, localhost only)
2. msmtpd invokes msmtp with the ISP's SMTP config
3. msmtp authenticates to the ISP using LOGIN auth
4. The ISP delivers the email

This is transparent -- the web UI configures it automatically when LOGIN auth is detected during relay testing.

---

## DNS Setup Guide

After adding your domain and generating DKIM keys, you need to create three DNS records. The web UI shows the exact values and can check if they are configured correctly.

### SPF Record

SPF tells receiving servers which hosts are authorized to send email for your domain.

**For Gmail relay:**
```
Type: TXT
Name: example.com
Value: v=spf1 include:_spf.google.com ~all
```

**For ISP relay (e.g., mail.isp.com):**
```
Type: TXT
Name: example.com
Value: v=spf1 include:mail.isp.com ~all
```

If the ISP host does not publish an SPF record, SignPost detects this and suggests `ip4:` instead of `include:`:
```
Value: v=spf1 ip4:203.0.113.10 ~all
```

**For direct delivery:**
```
Type: TXT
Name: example.com
Value: v=spf1 a:mail.example.com ~all
```

**Merging with existing SPF:** If you already have an SPF record (e.g., from Google Workspace), SignPost shows how to merge the mechanisms. You can only have one SPF record per domain.

### DKIM Record

After generating keys in the web UI, add the TXT record it provides:

```
Type: TXT
Name: signpost._domainkey.example.com
Value: v=DKIM1; k=rsa; p=MIIBIjANBgkqh...  (your public key)
```

The selector defaults to `signpost` but is configurable per domain.

### DMARC Record

DMARC ties SPF and DKIM together and tells receivers what to do with failures:

```
Type: TXT
Name: _dmarc.example.com
Value: v=DMARC1; p=quarantine; rua=mailto:postmaster@example.com
```

### How the DNS Check Feature Helps

Click "Check DNS" in the web UI to:
- See current vs. recommended values side by side
- Get status for each record: OK, Missing, Needs Update, Conflict
- Detect broken `include:` entries that cause SPF permerror
- View TTL values for each record
- Copy recommended values to clipboard

---

## Troubleshooting

### Common Issues

**"Sender domain not configured in SignPost"**
Your local service is sending from a domain that is not added in SignPost. Add the domain in the web UI.

**Test email fails with "connection refused"**
Maddy may not have started yet. Check the dashboard -- the Maddy status should show "running". If not, check container logs: `docker compose logs signpost`.

**DKIM check fails on receiver**
1. Verify the DKIM DNS record is published: check with the DNS validation feature
2. Ensure the record value matches exactly (no extra spaces or truncation)
3. Some DNS providers split long TXT records -- this is normal and handled automatically

**SPF permerror**
Usually caused by a broken `include:` pointing to a host with no SPF record. The DNS check feature detects this and suggests removal.

**Gmail relay: emails to same domain silently dropped**
If using `smtp.gmail.com:587` as your relay, Gmail silently drops emails where both sender and recipient are on the same Google Workspace domain (e.g., `user@example.com` to `other@example.com`). This happens when the envelope sender doesn't match the authenticated relay user. **Fix:** Enable "Use relay credentials as envelope sender" in the relay config. This rewrites the MAIL FROM to match the authenticated user, which satisfies Gmail. The From header recipients see is not affected. Google Workspace paid plans can alternatively use `smtp-relay.google.com` which doesn't have this limitation.

### Checking Logs

```bash
# Container logs (all processes)
docker compose logs signpost

# Follow logs in real time
docker compose logs -f signpost

# Maddy-specific logs
docker compose exec signpost cat /data/signpost/logs/maddy.log

# Web UI mail log
# Navigate to the Mail Log page in the web UI, or:
curl -u admin:yourpass http://localhost:8080/api/v1/logs
```

### SPF/DKIM/DMARC Debugging

Use these external tools to verify your email authentication:

1. **Send a test email** from the SignPost web UI to a Gmail address
2. In Gmail, click the three dots on the message and "Show original"
3. Look for `SPF: PASS`, `DKIM: PASS`, `DMARC: PASS` in the headers

External verification tools:
- [mail-tester.com](https://www.mail-tester.com) -- comprehensive email score
- [MXToolbox](https://mxtoolbox.com/SuperTool.aspx) -- DNS record lookup
- [DKIM Validator](https://dkimvalidator.com) -- send a test email for analysis

---

## API Reference

All endpoints except `/api/v1/healthz` require HTTP Basic Auth. The default username is `admin`.

### Health & Status

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/healthz` | No | Health check (DB integrity) |
| GET | `/api/v1/status` | Yes | Dashboard data (version, domain count, TLS, Maddy status, listeners) |

<details>
<summary>GET /api/v1/healthz</summary>

```bash
curl http://localhost:8080/api/v1/healthz
```

```json
{"status": "healthy", "db": "ok"}
```
</details>

<details>
<summary>GET /api/v1/status</summary>

```bash
curl -u admin:pass http://localhost:8080/api/v1/status
```

```json
{
  "version": "v0.11.1",
  "domain_count": 1,
  "tls_mode": "acme",
  "schema_version": 10,
  "maddy_status": "running",
  "listeners": [
    {"name": "SMTP", "bind": "0.0.0.0:25", "status": "running"},
    {"name": "Submission (STARTTLS)", "bind": "0.0.0.0:587", "status": "running"},
    {"name": "SMTPS (implicit TLS)", "bind": "0.0.0.0:465", "status": "running"},
    {"name": "HTTP API", "bind": "0.0.0.0:8080", "status": "running"}
  ]
}
```
</details>

### Domains

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/domains` | Yes | List all domains |
| POST | `/api/v1/domains` | Yes | Create a domain |
| GET | `/api/v1/domains/{id}` | Yes | Get a domain |
| DELETE | `/api/v1/domains/{id}` | Yes | Delete a domain |

<details>
<summary>POST /api/v1/domains</summary>

```bash
curl -u admin:pass -X POST http://localhost:8080/api/v1/domains \
  -H "Content-Type: application/json" \
  -d '{"name": "example.com", "selector": "signpost"}'
```

```json
{
  "id": 1,
  "name": "example.com",
  "dkim_selector": "signpost",
  "active": true,
  "created_at": "2026-03-29T12:00:00Z"
}
```
</details>

### DKIM

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/domains/{id}/dkim/generate` | Yes | Generate new DKIM key pair |
| GET | `/api/v1/domains/{id}/dkim/export` | Yes | Download private key PEM |
| POST | `/api/v1/domains/{id}/dkim/import` | Yes | Upload private key PEM |

<details>
<summary>POST /api/v1/domains/{id}/dkim/generate</summary>

```bash
curl -u admin:pass -X POST http://localhost:8080/api/v1/domains/1/dkim/generate
```

```json
{
  "dns_record_name": "signpost._domainkey.example.com",
  "dns_record_value": "v=DKIM1; k=rsa; p=MIIBIjANBgkqh...",
  "selector": "signpost",
  "key_path": "/data/signpost/dkim_keys/example.com.key"
}
```
</details>

### DNS

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/domains/{id}/dns` | Yes | Get required DNS records |
| GET | `/api/v1/domains/{id}/dns/check` | Yes | Live DNS validation |

<details>
<summary>GET /api/v1/domains/{id}/dns/check</summary>

```bash
curl -u admin:pass http://localhost:8080/api/v1/domains/1/dns/check
```

```json
{
  "records": [
    {
      "type": "TXT",
      "name": "example.com",
      "purpose": "spf",
      "current": "v=spf1 include:_spf.google.com ~all",
      "recommended": "v=spf1 include:_spf.google.com ~all",
      "status": "ok",
      "message": "Existing SPF already includes your relay's sending servers",
      "ttl": 300
    },
    {
      "type": "TXT",
      "name": "signpost._domainkey.example.com",
      "purpose": "dkim",
      "current": "v=DKIM1; k=rsa; p=MIIBIjANBgkqh...",
      "recommended": "v=DKIM1; k=rsa; p=MIIBIjANBgkqh...",
      "status": "ok",
      "message": "DKIM record matches",
      "ttl": 300
    },
    {
      "type": "TXT",
      "name": "_dmarc.example.com",
      "purpose": "dmarc",
      "current": "v=DMARC1; p=quarantine; rua=mailto:postmaster@example.com",
      "recommended": "v=DMARC1; p=quarantine; rua=mailto:postmaster@example.com",
      "status": "ok",
      "message": "DMARC record exists",
      "ttl": 3600
    }
  ]
}
```
</details>

### Relay Configuration

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/domains/{id}/relay` | Yes | Get active relay config |
| GET | `/api/v1/domains/{id}/relay/all` | Yes | Get all relay configs (all methods) |
| PUT | `/api/v1/domains/{id}/relay` | Yes | Create/update relay config |
| PUT | `/api/v1/domains/{id}/relay/{method}/activate` | Yes | Switch active relay method |
| POST | `/api/v1/domains/{id}/relay/test` | Yes | Test relay connectivity + auth |

<details>
<summary>PUT /api/v1/domains/{id}/relay -- Gmail relay</summary>

```bash
curl -u admin:pass -X PUT http://localhost:8080/api/v1/domains/1/relay \
  -H "Content-Type: application/json" \
  -d '{
    "method": "gmail",
    "host": "smtp.gmail.com",
    "port": 587,
    "username": "you@gmail.com",
    "password": "your-app-password",
    "starttls": true
  }'
```

```json
{"status": "updated"}
```
</details>

<details>
<summary>POST /api/v1/domains/{id}/relay/test</summary>

```bash
curl -u admin:pass -X POST http://localhost:8080/api/v1/domains/1/relay/test
```

```json
{
  "status": "ok",
  "message": "Connected and authenticated to smtp.gmail.com:587 (PLAIN auth)",
  "auth_method": "plain"
}
```
</details>

### Domain Export/Import

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/domains/{id}/export` | Yes | Export domain config as JSON |
| POST | `/api/v1/domains/import` | Yes | Import domain config from JSON |

<details>
<summary>GET /api/v1/domains/{id}/export</summary>

```bash
curl -u admin:pass http://localhost:8080/api/v1/domains/1/export -o drcs.ca-signpost-config.json
```

```json
{
  "signpost_version": "v0.4.0",
  "exported_at": "2026-03-29T12:00:00Z",
  "domain": {
    "name": "drcs.ca",
    "dkim_selector": "signpost"
  },
  "dkim_key": "-----BEGIN PRIVATE KEY-----\n...",
  "relay_configs": [
    {
      "method": "gmail",
      "host": "smtp.gmail.com",
      "port": 587,
      "username": "user@gmail.com",
      "password": "decrypted-app-password",
      "starttls": true,
      "auth_method": "plain",
      "active": true
    }
  ]
}
```
</details>

### SMTP Users

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/smtp-users` | Yes | List SMTP users |
| POST | `/api/v1/smtp-users` | Yes | Create SMTP user |
| DELETE | `/api/v1/smtp-users/{id}` | Yes | Delete SMTP user |
| PUT | `/api/v1/smtp-users/{id}/password` | Yes | Change password |
| PUT | `/api/v1/smtp-users/{id}/active` | Yes | Enable/disable user |

<details>
<summary>POST /api/v1/smtp-users</summary>

```bash
curl -u admin:pass -X POST http://localhost:8080/api/v1/smtp-users \
  -H "Content-Type: application/json" \
  -d '{"username": "nas@drcs.ca", "password": "smtp-password"}'
```

```json
{
  "id": 1,
  "username": "nas@drcs.ca",
  "active": true,
  "created_at": "2026-03-29T12:00:00Z"
}
```
</details>

### Settings

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/settings` | Yes | Get all settings |
| PUT | `/api/v1/settings` | Yes | Update settings |

<details>
<summary>PUT /api/v1/settings</summary>

```bash
curl -u admin:pass -X PUT http://localhost:8080/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"smtp_enabled": "true", "submission_enabled": "true"}'
```

```json
{"status": "updated"}
```
</details>

### TLS

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/tls` | Yes | Get TLS config, cert details (issuer, SANs, expiry, days remaining) |
| PUT | `/api/v1/tls` | Yes | Update TLS mode, ACME email, CF token, hostname |
| POST | `/api/v1/tls/generate-selfsigned` | Yes | Regenerate self-signed certificate |

### Network

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/network/public-ip` | Yes | Detect server's public IP |

### Mail Logs

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| GET | `/api/v1/logs` | Yes | Paginated mail log |

Query parameters: `limit` (1-200, default 50), `offset` (default 0), `status` (filter by `sent` or `failed`).

<details>
<summary>GET /api/v1/logs</summary>

```bash
curl -u admin:pass "http://localhost:8080/api/v1/logs?limit=10&status=failed"
```

```json
[
  {
    "id": 42,
    "from_addr": "alerts@drcs.ca",
    "to_addr": "admin@gmail.com",
    "domain_id": 1,
    "subject": "Disk Usage Alert",
    "status": "failed",
    "relay_host": "smtp.gmail.com",
    "error": "550 5.7.1 sender rejected",
    "dkim_signed": true,
    "created_at": "2026-03-29T11:30:00Z"
  }
]
```
</details>

### Test Email

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/test/send` | Yes | Send a test email |

<details>
<summary>POST /api/v1/test/send</summary>

```bash
curl -u admin:pass -X POST http://localhost:8080/api/v1/test/send \
  -H "Content-Type: application/json" \
  -d '{"from": "test@drcs.ca", "to": "you@gmail.com", "subject": "SignPost Test"}'
```

```json
{
  "status": "sent",
  "message": "Test email sent from test@drcs.ca to you@gmail.com via Maddy"
}
```
</details>

---

## Development

### Prerequisites

- Go 1.24+
- Node.js 20+
- Docker
- SQLite (CGO required for go-sqlite3)

### Running Tests

```bash
# Go is at /usr/local/go/bin (add to PATH if needed)
export PATH=$PATH:/usr/local/go/bin

# All Go tests (139+ tests across 8 packages)
CGO_ENABLED=1 go test -race ./internal/...

# Specific package
go test -v ./internal/db/
go test -v ./internal/api/
go test -v ./internal/config/
go test -v ./internal/crypto/
go test -v ./internal/dkim/
go test -v ./internal/logtail/
go test -v ./internal/queue/
go test -v ./internal/tls/

# Frontend tests
cd web && npx vitest run
```

### Building from Source

```bash
# Build frontend
cd web && npm ci && npm run build && cd ..

# Build Go binary
CGO_ENABLED=1 go build -o signpost ./cmd/signpost/

# Build Docker image
docker build -t signpost:dev .

# Run in dev mode
docker compose -f docker-compose.dev.yml up --build
```

### Project Structure

```
signpost/
  cmd/signpost/           # Go entrypoint
  internal/
    api/                  # REST API handlers (chi router)
    config/               # Maddy config generator (Go templates)
    crypto/               # AES-256-GCM credential encryption
    db/                   # SQLite database, migrations, queries
    dkim/                 # RSA key generation, DNS record builders
    logtail/              # Real-time Maddy log parser and event mapper
    queue/                # Maddy queue scanner with thread-safe caching
    tls/                  # Self-signed certificate generation
  web/                    # React frontend (Vite + TypeScript + Tailwind + shadcn/ui)
  templates/              # Maddy config template (maddy.conf.tmpl)
  rootfs/                 # s6-overlay service definitions
  Dockerfile              # 3-stage build (Node + Go + Maddy)
  docker-compose.yml      # Default compose
  docker-compose.dev.yml  # Dev compose (bind mounts, debug logging)
  docker-compose.prod.yml # Prod compose (named volumes, localhost binding)
```

---

## License

MIT

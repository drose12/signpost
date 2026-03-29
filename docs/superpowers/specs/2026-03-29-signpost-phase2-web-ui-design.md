# SignPost Phase 2 — Web UI Design

**Date:** 2026-03-29
**Status:** Approved
**Author:** drose + Claude

## Overview

A React single-page application that provides a browser-based admin interface for SignPost. Users configure domains, DKIM keys, DNS records, relay settings, and send test emails — all through the UI instead of API calls.

## Tech Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Framework | React 19 + TypeScript | Modern, typed, large ecosystem |
| Build tool | Vite | Fast dev server, optimized production builds |
| CSS | Tailwind CSS v4 | Utility-first, dark mode via `dark:` classes |
| Components | shadcn/ui | Copy-paste components built on Radix, fully customizable |
| Icons | Lucide React | Clean icon set, tree-shakeable, used by shadcn |
| Routing | React Router v7 | Standard SPA routing |
| HTTP client | fetch wrapper | Thin wrapper with basic auth + JSON handling |
| Serving | Go `embed.FS` | Built SPA embedded in Go binary, served by chi router |

## Layout

### Structure

```
┌──────────────────────────────────────────────────────┐
│ ┌──────────┐ ┌─────────────────────────────────────┐ │
│ │           │ │                                     │ │
│ │  Sidebar  │ │          Content Area               │ │
│ │  (dark)   │ │          (light/dark)               │ │
│ │           │ │                                     │ │
│ │  Logo     │ │  Page header                        │ │
│ │  ──────── │ │  ──────────────────                 │ │
│ │  Dashboard│ │                                     │ │
│ │  Domains  │ │  Page content                       │ │
│ │  Mail Log │ │                                     │ │
│ │  Wizard   │ │                                     │ │
│ │           │ │                                     │ │
│ │           │ │                                     │ │
│ │  ──────── │ │                                     │ │
│ │  ☀/☾ v0.1│ │                                     │ │
│ └──────────┘ └─────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

- **Sidebar:** Dark background (`slate-900`), always visible. Logo + nav items + theme toggle at bottom with version.
- **Content area:** Light background (`slate-50`) in light mode, dark (`slate-900`) in dark mode.
- **Theme toggle:** Sun/moon icon at sidebar bottom. Persisted to `localStorage`. Light mode default.
- **Active nav item:** Highlighted background with left accent border.

### Navigation

| Item | Route | Icon |
|------|-------|------|
| Dashboard | `/` | `LayoutDashboard` |
| Domains | `/domains` | `Globe` |
| Mail Log | `/logs` | `Mail` |
| Setup Wizard | `/wizard` | `Wand2` |

Four pages total. Relay configuration lives as a tab on the Domains page (per-domain), not a separate nav item.

## Pages

### 1. Dashboard (`/`)

At-a-glance system status.

**Components:**
- **Status cards row** (3 cards):
  - Domains: count of active domains, primary domain name
  - TLS: current mode (self-signed/manual/acme)
  - Schema: current DB version
- **Recent activity table:** Last 10 mail log entries showing `from_addr`, `to_addr`, status (`sent`/`failed`/`deferred`), `timestamp`. "View all" link to Mail Log page.

**API calls:**
- `GET /api/v1/status` — returns `domain_count`, `tls_mode`, `tls_cert_expiry`, `schema_version`
- `GET /api/v1/logs?limit=10` — recent activity (separate call)

**Note:** The `recent_logs` field will be removed from the `/api/v1/status` response (it's redundant with the `/logs` endpoint). The status endpoint does not yet include Maddy process status or uptime — these will be added as a backend prerequisite (see Backend Prerequisites section). The migration seeds a default TLS config row, so `tls_mode` will always have a value.

### 2. Domains (`/domains`)

The primary management page. Select a domain, then use tabs to manage it.

**Layout:**
- Top: domain list + "Add Domain" button
- Selected domain highlighted
- Below: tabbed detail view for selected domain

**Tabs:**

#### DNS Records tab
- Table: Type, Name, Value, Copy button
- Status column shows "—" (unknown) by default. DNS validation is out of scope for Phase 2; the "Validate DNS" button will be shown but disabled with a tooltip: "Coming soon".
- Info banner explaining DNS propagation
- **API:** `GET /api/v1/domains/{id}/dns`

#### Relay Config tab
- Method selector: Gmail SMTP, ISP relay, Direct delivery, Custom SMTP
- Conditional fields based on method:
  - Gmail: host pre-filled (`smtp.gmail.com:587`), username, app password, STARTTLS on
  - Custom: host, port, username, password, STARTTLS toggle
  - Direct: no fields (just explanation text)
- Save button + "Test Connection" button (disabled, Phase 3)
- **API:** `GET /api/v1/domains/{id}/relay`, `PUT /api/v1/domains/{id}/relay`

#### DKIM Keys tab
- Current key info: selector, key path, last updated (`updated_at` from domain object, which updates when DKIM is generated)
- "Regenerate Keys" button with confirmation dialog (destructive — invalidates existing DNS record)
- Public key display (DNS TXT record value) with copy button
- **API:** `POST /api/v1/domains/{id}/dkim/generate` (returns `dns_record_name`, `dns_record_value`, `selector`, `key_path`). Domain's `updated_at` serves as the "generated date". Domain data is already loaded from the parent Domains page.

#### Settings tab
- Delete domain button with confirmation dialog
- **API:** `GET /api/v1/domains/{id}`, `DELETE /api/v1/domains/{id}`

**Note:** Domain active/inactive toggle requires a `PUT /api/v1/domains/{id}` endpoint that does not exist yet. This is deferred — the Settings tab will only show delete for Phase 2. The toggle will be added when the backend endpoint is implemented.

**Add Domain flow:**
- Modal dialog: domain name input + DKIM selector input (defaults to "signpost")
- On submit: creates domain via API, auto-selects it, prompts to run Setup Wizard
- **API:** `POST /api/v1/domains`

### 3. Mail Log (`/logs`)

Paginated, filterable log viewer.

**Components:**
- Filter bar: status dropdown (all/sent/failed/deferred)
- Table: `timestamp`, `from_addr`, `to_addr`, `status`, details (`error` field shown on hover/expand)
- "Load more" button at bottom (not page numbers — the API doesn't return a total count). Fetches next batch by incrementing offset.

**API:** `GET /api/v1/logs?limit=50&offset=0&status=...`

### 4. Setup Wizard (`/wizard`)

Guided vertical checklist for configuring a domain end-to-end. Re-runnable — accessible anytime from the sidebar to add new domains.

**Flow (5 steps):**

```
1. Add Domain        → domain name + selector input, creates domain
2. Generate DKIM     → one-click, auto-generates RSA-2048 key pair
3. DNS Records       → shows SPF/DKIM/DMARC records with copy buttons,
                       "Verify DNS" button (disabled, Phase 3)
4. Configure Relay   → method selector + credentials (same form as Relay tab)
5. Send Test Email   → to-address input, from-address auto-populated from wizard's
                       domain (e.g., test@drcs.ca). POST /api/v1/test/send with
                       subject and body using sensible defaults.
```

**Layout:** Vertical timeline on the left. All 5 steps visible. Current step expands to show its form/content inline. Completed steps show green checkmarks with summary text. Pending steps are grayed out but visible.

**Behavior:**
- First visit with no domains: wizard auto-opens at step 1
- Re-run: "Add another domain" triggers wizard, pre-fills nothing
- Steps can be revisited by clicking on them (not strictly linear)
- Each step auto-advances on completion but user can go back
- Step 3 (DNS) has a "Skip for now" option since DNS propagation takes time
- Step 5 (Test) shows success/failure result inline

**API calls:** Same as Domains page — the wizard is a guided wrapper around the same endpoints.

## API Integration

### Fetch Wrapper

A thin `api.ts` module that handles:
- Base URL: relative to current origin (`/api/v1/...`)
- Basic auth: credentials stored in memory after login prompt
- JSON encode/decode
- Error handling: parse error responses, surface messages to UI

```typescript
// Usage pattern
const domains = await api.get<Domain[]>('/domains');
const domain = await api.post<Domain>('/domains', { name: 'drcs.ca', selector: 'signpost' });
```

### Auth Flow

On first load, prompt for admin username/password. Store in memory (not localStorage — cleared on tab close). All API calls include the `Authorization: Basic ...` header.

Simple modal login form — no session tokens or cookies needed since the backend uses basic auth.

**Error handling:** If any API call returns 401, re-show the login dialog.

## SPA Serving

The built React app (`web/dist/`) is embedded into the Go binary using `embed.FS`. Since Go embed paths are relative to the source file, the embed directive lives in `web/embed.go`:

```go
// web/embed.go
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
```

Then `cmd/signpost/main.go` imports it:

```go
import "github.com/drose-drcs/signpost/web"
// Pass web.DistFS to the API server
```

The FS is passed to `api.NewServer` which mounts the SPA fallback handler on the chi router:

```go
// NewServer signature updated to accept the web FS
func NewServer(database *db.DB, configGen *config.Generator, keysDir, adminUser, adminPass string, webFS fs.FS) *Server

// In router setup — SPA fallback after API routes:
r.Handle("/*", spaHandler(webFS))
```

The SPA handler serves static files from `webFS`, falling back to `index.html` for any path that doesn't match a file (client-side routing). API routes (`/api/v1/*`) take precedence since they're registered first.

**Dev mode:** Vite dev server on port 5173 with proxy to Go backend on 8080. Pass `nil` for webFS in dev mode; the proxy handles it.

**Note:** `web/dist/` must exist at build time for the embed to succeed. The Dockerfile's Node.js stage builds it before the Go stage. For local Go builds without the frontend, create a placeholder: `mkdir -p web/dist && touch web/dist/.gitkeep`.

## Dark Mode

- Tailwind `dark:` variant with `class` strategy (not `media`)
- Toggle adds/removes `dark` class on `<html>` element
- Preference persisted to `localStorage` key `signpost-theme`
- Default: light mode
- Sidebar always dark regardless of theme (it's slate-900 in both modes)

## UI States

### Loading
- Pages show a centered spinner while API calls are in flight
- Individual components (domain list, mail log table) show skeleton placeholders

### Errors
- API errors display as a toast notification (top-right, auto-dismiss after 5s)
- Network failures show a persistent banner: "Cannot reach SignPost API"
- 401 responses re-trigger the login dialog

### Empty States
- **Dashboard** (no domains): "No domains configured. Run the Setup Wizard to get started." with link to `/wizard`
- **Domains** (no domains): Same empty state with "Add Domain" button prominent
- **Mail Log** (no entries): "No mail log entries yet. Send a test email to see activity here."
- **Wizard**: Always has content (step 1 form)

## File Structure

```
web/
├── embed.go                 # Go embed directive for web/dist/
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── tailwind.config.ts
├── components.json          # shadcn config
├── src/
│   ├── main.tsx             # React entry point
│   ├── App.tsx              # Router + layout shell
│   ├── api.ts               # Fetch wrapper with basic auth
│   ├── theme.ts             # Dark mode toggle logic
│   ├── components/
│   │   ├── ui/              # shadcn components (button, card, dialog, table, tabs, input, badge, etc.)
│   │   ├── Layout.tsx       # Sidebar + content area shell
│   │   ├── Sidebar.tsx      # Navigation sidebar
│   │   ├── LoginDialog.tsx  # Basic auth login prompt
│   │   └── StatusBadge.tsx  # Reusable status indicator
│   ├── pages/
│   │   ├── Dashboard.tsx
│   │   ├── Domains.tsx      # Domain list + tabbed detail
│   │   ├── MailLog.tsx
│   │   └── Wizard.tsx       # Setup wizard with vertical checklist
│   └── types.ts             # TypeScript types matching API responses
```

## Component Inventory (shadcn/ui)

Components to install:

- `button` — actions, form submits
- `card` — dashboard status cards, domain cards
- `dialog` — add domain modal, delete confirmation, login
- `input` — form fields
- `label` — form labels
- `select` — relay method dropdown, log filters
- `table` — DNS records, mail log
- `tabs` — domain detail tabs
- `badge` — status indicators (active, sent, failed)
- `separator` — visual dividers
- `tooltip` — copy button feedback, disabled button explanations
- `switch` — toggles (future use)
- `alert` — info banners (DNS propagation note)
- `sonner` (toast) — success/error notifications

## Testing Strategy

- **Component tests:** Vitest + React Testing Library for key interactions (wizard step progression, domain CRUD, copy-to-clipboard)
- **No E2E browser tests for MVP** — manual testing against the running container is sufficient for Phase 2
- **API mock:** MSW (Mock Service Worker) for component tests that need API responses

## Build & Deploy

**Development:**
```bash
cd web && npm install && npm run dev    # Vite dev server on :5173
# Proxy /api/* to Go backend on :8080
```

**Production build:**
```bash
cd web && npm run build                 # Output to web/dist/
# Go binary embeds web/dist/ via embed.FS
```

**Docker:** The current Dockerfile has 2 stages (Go builder + Final). Phase 2 requires updating it to 3 stages:

```
Stage 1: Node (node:20-alpine) — npm ci + npm run build → web/dist/
Stage 2: Go (golang:1.24-alpine) — COPY --from=node web/dist/ web/dist/, go build with embed
Stage 3: Final (foxcpp/maddy:0.9.2) — s6-overlay + signpost binary
```

This is a required Dockerfile change for Phase 2.

## Backend Prerequisites

The following backend changes are needed before or during Phase 2 frontend work:

1. **Extend `GET /api/v1/status`** — Add Maddy process status (check if port 25 is listening via TCP dial) and uptime. Remove `recent_logs` from response (fetched separately via `/logs`).
2. **Implement `handleTestSend`** — Currently returns a stub. Needs actual SMTP send via localhost:25 so the wizard's step 5 works. Must accept a `from` address (or infer from domain).
3. **Update `NewServer` signature** — Accept `fs.FS` parameter for serving the embedded SPA.
4. **Add SPA fallback handler** — Serve `web/dist/` files from `web.DistFS`, fall back to `index.html` for client-side routes.
5. **Create `web/embed.go`** — Package `web` with `//go:embed all:dist` directive exposing `DistFS`.

Deferred to later phases:
- `PUT /api/v1/domains/{id}` — needed for active/inactive toggle
- DNS validation endpoint — needed for "Validate DNS" button
- Relay test connection endpoint — needed for "Test Connection" button

## Scope Boundaries

**In scope (Phase 2):**
- Dashboard, Domains (with tabs), Mail Log, Setup Wizard
- Dark/light mode toggle
- Basic auth login
- All existing API endpoints wired up
- SPA embedded in Go binary
- Dockerfile updated for Node.js build stage

**Out of scope (Phase 3+):**
- TLS/certificate management page
- SMTP user management page
- Security audit page
- Backup/restore page
- DNS validation backend (UI shows button disabled)
- Test connection button for relay (UI shows button disabled)
- Real-time log streaming (polling on manual refresh is fine for MVP)
- Domain active/inactive toggle (needs PUT endpoint)

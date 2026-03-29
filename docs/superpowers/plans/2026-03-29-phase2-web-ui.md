# Phase 2 Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a React SPA admin interface for SignPost — dashboard, domain management with tabs, mail log viewer, and setup wizard — served from the Go binary via embed.FS.

**Architecture:** Vite + React + TypeScript frontend in `web/`, built to `web/dist/`, embedded into Go binary via `web/embed.go`. Go backend serves API on `/api/v1/*` and SPA fallback on `/*`. Dark sidebar layout with light/dark mode toggle.

**Tech Stack:** React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui, Lucide React, React Router v7

**Spec:** `docs/superpowers/specs/2026-03-29-signpost-phase2-web-ui-design.md`

---

## File Map

### New Files (Frontend)

```
web/
├── embed.go                    # Go embed directive for dist/
├── index.html                  # Vite HTML entry
├── package.json                # npm deps
├── tsconfig.json               # TypeScript config
├── tsconfig.app.json           # App-specific TS config
├── tsconfig.node.json          # Node-specific TS config (vite config)
├── vite.config.ts              # Vite + proxy config
├── components.json             # shadcn/ui config
├── src/
│   ├── main.tsx                # React entry point
│   ├── App.tsx                 # Router + layout shell
│   ├── api.ts                  # Fetch wrapper with basic auth
│   ├── theme.ts                # Dark mode toggle + localStorage
│   ├── types.ts                # TS types matching Go API responses
│   ├── index.css               # Tailwind imports + base styles
│   ├── lib/
│   │   └── utils.ts            # shadcn cn() helper
│   ├── components/
│   │   ├── ui/                 # shadcn components (generated)
│   │   ├── Layout.tsx          # Sidebar + content area shell
│   │   ├── Sidebar.tsx         # Navigation sidebar
│   │   ├── LoginDialog.tsx     # Basic auth login prompt
│   │   └── StatusBadge.tsx     # Reusable status indicator
│   └── pages/
│       ├── Dashboard.tsx       # Status cards + recent activity
│       ├── Domains.tsx         # Domain list + tabbed detail
│       ├── MailLog.tsx         # Filterable log viewer
│       └── Wizard.tsx          # Setup wizard vertical checklist
```

### Modified Files (Backend)

```
cmd/signpost/main.go            # Import web.DistFS, pass to NewServer
internal/api/server.go           # Add webFS param, SPA handler
internal/api/handlers.go         # Extend handleStatus, implement handleTestSend
Dockerfile                       # Add Node.js build stage
.dockerignore                    # Add web/node_modules
```

---

## Task 1: Backend Prerequisites

**Files:**
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/server_test.go`
- Create: `web/embed.go`
- Create: `web/dist/.gitkeep`
- Modify: `cmd/signpost/main.go`

### 1a: Create web/embed.go and placeholder dist

- [ ] **Step 1: Create the embed package**

```go
// web/embed.go
package web

import "embed"

//go:embed all:dist
var DistFS embed.FS
```

- [ ] **Step 2: Create placeholder dist so embed compiles**

```bash
mkdir -p web/dist
touch web/dist/.gitkeep
# Force-add since web/dist/ may be gitignored
git add -f web/dist/.gitkeep
```

- [ ] **Step 3: Verify Go build still works**

Run: `CGO_ENABLED=1 go build ./...`
Expected: compiles with no errors

### 1b: Add SPA handler and update NewServer

- [ ] **Step 4: Update Server struct and NewServer to accept webFS**

In `internal/api/server.go`, add `io/fs` import and update:

```go
import (
    "io/fs"
    // ... existing imports
)

type Server struct {
    db        *db.DB
    configGen *config.Generator
    keysDir   string
    router    chi.Router
    adminUser string
    adminPass string
    webFS     fs.FS  // embedded frontend, nil in dev
}

func NewServer(database *db.DB, configGen *config.Generator, keysDir, adminUser, adminPass string, webFS fs.FS) *Server {
    s := &Server{
        db:        database,
        configGen: configGen,
        keysDir:   keysDir,
        adminUser: adminUser,
        adminPass: adminPass,
        webFS:     webFS,
    }
    s.router = s.buildRouter()
    return s
}
```

- [ ] **Step 5: Add SPA fallback handler to buildRouter**

At the end of `buildRouter()`, after the API routes group:

```go
// Serve frontend SPA (after API routes)
if s.webFS != nil {
    s.router.Handle("/*", s.spaHandler())
}
```

Add the `spaHandler` method:

```go
func (s *Server) spaHandler() http.Handler {
    // Strip the "dist" prefix from the embedded FS
    subFS, err := fs.Sub(s.webFS, "dist")
    if err != nil {
        log.Fatalf("Failed to create sub filesystem: %v", err)
    }
    fileServer := http.FileServer(http.FS(subFS))

    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Try to serve the static file
        path := r.URL.Path
        if path == "/" {
            path = "/index.html"
        }
        // Check if file exists in embedded FS
        f, err := subFS.Open(strings.TrimPrefix(path, "/"))
        if err == nil {
            f.Close()
            fileServer.ServeHTTP(w, r)
            return
        }
        // File not found — serve index.html for client-side routing
        r.URL.Path = "/"
        fileServer.ServeHTTP(w, r)
    })
}
```

Add `"strings"` to the imports.

- [ ] **Step 6: Update main.go to pass webFS**

```go
import (
    // ... existing imports
    "github.com/drose-drcs/signpost/web"
)

// In main(), update NewServer call:
srv := api.NewServer(database, configGen, keysDir, adminUser, adminPass, web.DistFS)
```

- [ ] **Step 7: Fix test calls to NewServer**

In `internal/api/server_test.go`, update all `NewServer` calls to pass `nil` as the last argument:

```go
srv := NewServer(database, gen, keysDir, "admin", "admin", nil)
```

- [ ] **Step 8: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/...`
Expected: all tests pass

- [ ] **Step 9: Commit**

```bash
git add web/embed.go web/dist/.gitkeep internal/api/server.go internal/api/handlers.go internal/api/server_test.go cmd/signpost/main.go
git commit -m "feat: add SPA serving infrastructure and web embed package"
```

### 1c: Extend handleStatus and implement handleTestSend

- [ ] **Step 10: Update handleStatus to check Maddy and remove recent_logs**

In `internal/api/handlers.go`, replace `handleStatus`:

```go
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
    domains, err := s.db.ListDomains()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    tlsConfig, err := s.db.GetTLSConfig()
    if err != nil {
        writeError(w, http.StatusInternalServerError, err.Error())
        return
    }

    version, _ := s.db.SchemaVersion()

    // Check if Maddy is listening on SMTP port
    maddyStatus := "stopped"
    smtpPort := envOrDefault("SIGNPOST_SMTP_PORT", "25")
    conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", smtpPort), 500*time.Millisecond)
    if dialErr == nil {
        conn.Close()
        maddyStatus = "running"
    }

    writeJSON(w, http.StatusOK, map[string]interface{}{
        "domain_count":    len(domains),
        "tls_mode":        tlsConfig.Mode,
        "tls_cert_expiry": tlsConfig.CertExpiry,
        "schema_version":  version,
        "maddy_status":    maddyStatus,
    })
}
```

Add `"net"` and `"time"` to imports.

- [ ] **Step 11: Implement handleTestSend with actual SMTP**

Replace the stub in `internal/api/handlers.go`:

```go
func (s *Server) handleTestSend(w http.ResponseWriter, r *http.Request) {
    var req struct {
        From    string `json:"from"`
        To      string `json:"to"`
        Subject string `json:"subject"`
        Body    string `json:"body"`
    }
    if err := decodeJSON(r, &req); err != nil {
        writeError(w, http.StatusBadRequest, "invalid request body")
        return
    }
    if req.To == "" {
        writeError(w, http.StatusBadRequest, "to address is required")
        return
    }
    if req.From == "" {
        writeError(w, http.StatusBadRequest, "from address is required")
        return
    }
    if req.Subject == "" {
        req.Subject = "SignPost Test Email"
    }
    if req.Body == "" {
        req.Body = "This is a test email from SignPost.\nIf you received this, your mail relay is working correctly."
    }

    // Send via local SMTP (Maddy on port 25)
    msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@signpost>\r\n\r\n%s",
        req.From, req.To, req.Subject,
        time.Now().Format(time.RFC1123Z),
        fmt.Sprintf("%d", time.Now().UnixNano()),
        req.Body,
    )

    smtpPort := envOrDefault("SIGNPOST_SMTP_PORT", "25")
    addr := net.JoinHostPort("127.0.0.1", smtpPort)

    err := smtp.SendMail(addr, nil, req.From, []string{req.To}, []byte(msg))
    if err != nil {
        errStr := err.Error()
        s.db.LogMail(req.From, req.To, nil, req.Subject, "failed", nil, &errStr, false)
        writeJSON(w, http.StatusOK, map[string]string{
            "status": "failed",
            "error":  errStr,
        })
        return
    }

    s.db.LogMail(req.From, req.To, nil, req.Subject, "sent", nil, nil, true)
    writeJSON(w, http.StatusOK, map[string]string{
        "status":  "sent",
        "message": fmt.Sprintf("Test email sent from %s to %s", req.From, req.To),
    })
}
```

Add `"net"`, `"net/smtp"`, and `"time"` to imports. Also add the `envOrDefault` helper if not already present in handlers.go (it exists in main.go — either move to a shared package or duplicate):

```go
func envOrDefault(key, def string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return def
}
```

Add `"os"` to imports.

- [ ] **Step 12: Update existing test for handleTestSend**

In `internal/api/server_test.go`, update the test send test to include `"from"` in the request body:
```go
body := `{"from":"test@drcs.ca","to":"test@example.com"}`
```

Also add a test for missing `from` field returning 400.

The `handleStatus` test may need updating since the response shape changed (removed `recent_logs`, added `maddy_status`). Update assertions to match the new fields. The `maddy_status` will be `"stopped"` in tests since no Maddy is running — that's expected.

- [ ] **Step 13: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/...`
Expected: all tests pass

- [ ] **Step 14: Commit**

```bash
git add internal/api/handlers.go internal/api/server_test.go
git commit -m "feat: extend status endpoint with Maddy check, implement test send"
```

---

## Task 2: Frontend Scaffolding

**Files:**
- Create: `web/index.html`, `web/package.json`, `web/tsconfig.json`, `web/tsconfig.app.json`, `web/tsconfig.node.json`, `web/vite.config.ts`, `web/components.json`, `web/src/main.tsx`, `web/src/index.css`, `web/src/lib/utils.ts`

- [ ] **Step 1: Initialize Vite React TypeScript project**

```bash
cd web
npm create vite@latest . -- --template react-ts
```

If it complains the directory isn't empty, remove the existing empty `src/` dirs first:
```bash
rm -rf src/ web/src/
```

- [ ] **Step 2: Install dependencies**

```bash
cd web
npm install react-router-dom@7 lucide-react sonner
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 3: Configure Vite with Tailwind and API proxy**

Replace `web/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
```

- [ ] **Step 4: Set up Tailwind CSS**

Replace `web/src/index.css`:

```css
@import "tailwindcss";
```

- [ ] **Step 5: Initialize shadcn/ui**

```bash
cd web
npx shadcn@latest init
```

Select: New York style, Slate base color, CSS variables yes. If it prompts for tsconfig path, use `tsconfig.app.json`.

- [ ] **Step 6: Install shadcn components**

```bash
cd web
npx shadcn@latest add button card dialog input label select table tabs badge separator tooltip switch alert sonner
```

- [ ] **Step 7: Create minimal App.tsx for verification**

```tsx
// web/src/App.tsx
export default function App() {
  return <div className="p-8"><h1 className="text-2xl font-bold">SignPost</h1></div>
}
```

- [ ] **Step 8: Verify dev server starts**

```bash
cd web && npm run dev
```

Open http://localhost:5173 — should show "SignPost" heading with Tailwind styling.

- [ ] **Step 9: Verify production build**

```bash
cd web && npm run build
```

Expected: `web/dist/` populated with `index.html` + JS/CSS assets.

- [ ] **Step 10: Commit**

```bash
git add web/
git commit -m "feat: scaffold Vite + React + TypeScript + Tailwind + shadcn frontend"
```

---

## Task 3: API Client + TypeScript Types

**Files:**
- Create: `web/src/types.ts`
- Create: `web/src/api.ts`

- [ ] **Step 1: Define TypeScript types matching Go models**

```typescript
// web/src/types.ts

export interface Domain {
  id: number;
  name: string;
  dkim_selector: string;
  dkim_key_path?: string;
  dkim_public_dns?: string;
  dkim_created_at?: string;
  spf_record?: string;
  dmarc_record?: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export interface RelayConfig {
  id: number;
  domain_id: number;
  method: string; // gmail, isp, direct, custom
  host?: string;
  port: number;
  username?: string;
  starttls: boolean;
  created_at: string;
  updated_at: string;
}

export interface MailLogEntry {
  id: number;
  timestamp: string;
  from_addr: string;
  to_addr: string;
  domain_id?: number;
  subject?: string;
  status: string; // sent, failed, deferred
  relay_host?: string;
  error?: string;
  dkim_signed: boolean;
}

export interface DNSRecord {
  type: string;
  name: string;
  value: string;
  description: string;
}

export interface StatusResponse {
  domain_count: number;
  tls_mode: string;
  tls_cert_expiry?: string;
  schema_version: number;
  maddy_status: string;
}

export interface DKIMGenerateResponse {
  dns_record_name: string;
  dns_record_value: string;
  selector: string;
  key_path: string;
}

export interface TestSendResponse {
  status: string;
  message?: string;
  error?: string;
}
```

- [ ] **Step 2: Create API client with basic auth**

```typescript
// web/src/api.ts

let credentials: { username: string; password: string } | null = null;

export function setCredentials(username: string, password: string) {
  credentials = { username, password };
}

export function clearCredentials() {
  credentials = null;
}

export function hasCredentials(): boolean {
  return credentials !== null;
}

class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (credentials) {
    headers['Authorization'] = 'Basic ' + btoa(`${credentials.username}:${credentials.password}`);
  }

  const res = await fetch(`/api/v1${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401) {
    clearCredentials();
    throw new ApiError(401, 'Unauthorized');
  }

  const data = await res.json();

  if (!res.ok) {
    throw new ApiError(res.status, data.error || 'Unknown error');
  }

  return data as T;
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, body),
  del: <T>(path: string) => request<T>('DELETE', path),
};
```

- [ ] **Step 3: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add TypeScript types and API client with basic auth"
```

---

## Task 4: Layout Shell — Sidebar, Routing, Theme, Login

**Files:**
- Create: `web/src/theme.ts`
- Create: `web/src/components/Sidebar.tsx`
- Create: `web/src/components/Layout.tsx`
- Create: `web/src/components/LoginDialog.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create theme toggle logic**

```typescript
// web/src/theme.ts

export function getTheme(): 'light' | 'dark' {
  return (localStorage.getItem('signpost-theme') as 'light' | 'dark') || 'light';
}

export function setTheme(theme: 'light' | 'dark') {
  localStorage.setItem('signpost-theme', theme);
  if (theme === 'dark') {
    document.documentElement.classList.add('dark');
  } else {
    document.documentElement.classList.remove('dark');
  }
}

export function initTheme() {
  setTheme(getTheme());
}
```

- [ ] **Step 2: Create Sidebar component**

`web/src/components/Sidebar.tsx` — dark sidebar with nav items (Dashboard, Domains, Mail Log, Setup Wizard), active state highlighting, theme toggle at bottom. Uses `NavLink` from react-router-dom and Lucide icons (`LayoutDashboard`, `Globe`, `Mail`, `Wand2`, `Sun`, `Moon`).

Key structure:
```tsx
// Sidebar with:
// - Logo area: "✉ SignPost" in sky-blue
// - Nav items with NavLink, active = slate-700 bg + left blue border
// - Theme toggle at bottom with Sun/Moon icon
// - Responsive: fixed width 200px, slate-900 bg
```

- [ ] **Step 3: Create Layout component**

`web/src/components/Layout.tsx` — flexbox with Sidebar on left, Outlet on right. Content area is `slate-50` (light) / `slate-900` (dark).

```tsx
// Layout.tsx
import { Outlet } from 'react-router-dom';
import { Sidebar } from './Sidebar';

export function Layout() {
  return (
    <div className="flex h-screen">
      <Sidebar />
      <main className="flex-1 overflow-auto bg-slate-50 dark:bg-slate-900 p-6">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 4: Create LoginDialog component**

`web/src/components/LoginDialog.tsx` — modal dialog (uses shadcn Dialog) with username/password inputs. On submit, calls `setCredentials()` and attempts `api.get('/status')` to verify. Shows error on 401.

- [ ] **Step 5: Wire up App.tsx with routing**

```tsx
// web/src/App.tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Toaster } from 'sonner';
import { Layout } from './components/Layout';
import { LoginDialog } from './components/LoginDialog';
import { Dashboard } from './pages/Dashboard';
import { Domains } from './pages/Domains';
import { MailLog } from './pages/MailLog';
import { Wizard } from './pages/Wizard';
import { hasCredentials } from './api';
import { initTheme } from './theme';

export default function App() {
  const [loggedIn, setLoggedIn] = useState(hasCredentials());

  useEffect(() => { initTheme(); }, []);

  if (!loggedIn) {
    return <LoginDialog onLogin={() => setLoggedIn(true)} />;
  }

  return (
    <BrowserRouter>
      <Toaster position="top-right" />
      <Routes>
        <Route element={<Layout />}>
          <Route path="/" element={<Dashboard />} />
          <Route path="/domains" element={<Domains />} />
          <Route path="/logs" element={<MailLog />} />
          <Route path="/wizard" element={<Wizard />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
```

- [ ] **Step 6: Create placeholder page components**

Create minimal placeholder components for each page that just render a heading:

```tsx
// web/src/pages/Dashboard.tsx
export function Dashboard() {
  return <h1 className="text-2xl font-semibold">Dashboard</h1>;
}
```

Same pattern for `Domains.tsx`, `MailLog.tsx`, `Wizard.tsx`.

- [ ] **Step 7: Update main.tsx entry**

```tsx
// web/src/main.tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import App from './App';
import './index.css';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

- [ ] **Step 8: Test — verify layout renders in dev**

```bash
cd web && npm run dev
```

Open http://localhost:5173 — should show login dialog, then sidebar + dashboard placeholder after login.

- [ ] **Step 9: Commit**

```bash
git add web/src/
git commit -m "feat: add layout shell with sidebar, routing, theme toggle, login dialog"
```

---

## Task 5: Dashboard Page

**Files:**
- Modify: `web/src/pages/Dashboard.tsx`
- Create: `web/src/components/StatusBadge.tsx`

- [ ] **Step 1: Create StatusBadge component**

Small reusable component that shows a colored dot + text for status values:
- `running` / `sent` → green
- `stopped` / `failed` → red
- `deferred` → amber

- [ ] **Step 2: Implement Dashboard page**

Fetches `GET /api/v1/status` and `GET /api/v1/logs?limit=10` on mount. Renders:

1. **Page header:** "Dashboard"
2. **Status cards row** (3 cards using shadcn Card):
   - Maddy Status: StatusBadge + "Running"/"Stopped"
   - Domains: count from status
   - TLS: mode from status
3. **Recent Activity** (shadcn Table):
   - Columns: Time, From, To, Status
   - Rows from logs response
   - "View all" link to `/logs`
4. **Empty state:** If domain_count === 0, show prompt to run Setup Wizard

- [ ] **Step 3: Verify against running container**

Start the container (`docker compose -f docker-compose.dev.yml up -d`), then run the Vite dev server. Dashboard should load real data from the API.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Dashboard.tsx web/src/components/StatusBadge.tsx
git commit -m "feat: implement Dashboard page with status cards and recent activity"
```

---

## Task 6: Domains Page — List + DNS Records Tab

**Files:**
- Modify: `web/src/pages/Domains.tsx`

- [ ] **Step 1: Implement domain list with selection**

Top section: list of domains fetched from `GET /api/v1/domains`. Each domain shows name, active badge, DKIM selector. Clicking selects it (highlighted). "Add Domain" button opens a Dialog with name + selector inputs, calls `POST /api/v1/domains`.

- [ ] **Step 2: Implement tabbed detail view**

Below the domain list, when a domain is selected, show shadcn Tabs: DNS Records, Relay Config, DKIM Keys, Settings. Default to DNS Records tab.

- [ ] **Step 3: Implement DNS Records tab**

Fetches `GET /api/v1/domains/{id}/dns`. Renders a Table with columns: Type (badge), Name, Value (monospace, truncated with tooltip for long values), Copy button.

Copy button uses `navigator.clipboard.writeText()` with a sonner toast on success.

"Validate DNS" button shown but disabled with tooltip "Coming soon".

Info alert banner about DNS propagation.

- [ ] **Step 4: Verify against running container**

Create a domain + generate DKIM via the API, then check that the Domains page shows it correctly with DNS records and working copy buttons.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/Domains.tsx
git commit -m "feat: implement Domains page with domain list and DNS records tab"
```

---

## Task 7: Domains Page — Relay, DKIM, Settings Tabs

**Files:**
- Modify: `web/src/pages/Domains.tsx` (or extract tab components)

- [ ] **Step 1: Implement Relay Config tab**

Method selector (Select component): Gmail SMTP, ISP Relay, Direct, Custom.

Conditional fields:
- Gmail: host input (default `smtp.gmail.com`), port (default 587), username, password, STARTTLS toggle (default on)
- Custom: same fields, no defaults
- Direct: text explanation, no fields

Save button calls `PUT /api/v1/domains/{id}/relay`. Loads existing config on tab open via `GET /api/v1/domains/{id}/relay`.

"Test Connection" button shown disabled with tooltip "Coming soon".

- [ ] **Step 2: Implement DKIM Keys tab**

Shows current DKIM info from the domain object: selector, key path, updated_at as "generated date".

"Generate DKIM Keys" button (or "Regenerate" if keys exist). If keys exist, show confirmation dialog warning that this invalidates the existing DNS record.

On generate, calls `POST /api/v1/domains/{id}/dkim/generate`. Shows the returned public key DNS value with copy button.

- [ ] **Step 3: Implement Settings tab**

Delete domain button with confirmation Dialog. On confirm, calls `DELETE /api/v1/domains/{id}`, removes from list, deselects.

- [ ] **Step 4: Test full domain management flow**

Via the UI: add domain → generate DKIM → view DNS records → configure Gmail relay → save. Verify all API calls succeed and UI updates.

- [ ] **Step 5: Commit**

```bash
git add web/src/pages/Domains.tsx
git commit -m "feat: implement Relay Config, DKIM Keys, and Settings tabs on Domains page"
```

---

## Task 8: Mail Log Page

**Files:**
- Modify: `web/src/pages/MailLog.tsx`

- [ ] **Step 1: Implement filterable log table**

Page header: "Mail Log"

Filter bar: status Select (All, Sent, Failed, Deferred).

Table (shadcn Table): Timestamp (formatted), From, To, Status (StatusBadge), Error (shown in expandable row or tooltip if present).

Fetches `GET /api/v1/logs?limit=50&offset=0&status=...`. Re-fetches when filter changes.

"Load more" button at bottom — increments offset by 50 and appends results. Hidden when fewer than 50 results returned (end of data).

- [ ] **Step 2: Handle empty state**

If no entries: "No mail log entries yet. Send a test email to see activity here."

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/MailLog.tsx
git commit -m "feat: implement Mail Log page with filtering and load-more pagination"
```

---

## Task 9: Setup Wizard

**Files:**
- Modify: `web/src/pages/Wizard.tsx`

- [ ] **Step 1: Implement wizard state machine**

State: `currentStep` (1-5), `completedSteps` set, `domainId` (set after step 1), `domainName`.

Steps:
1. Add Domain — name + selector inputs, POST /api/v1/domains
2. Generate DKIM — one-click button, POST /api/v1/domains/{id}/dkim/generate
3. DNS Records — shows records with copy buttons (same as DNS tab), "Skip for now" button
4. Configure Relay — same form as Relay Config tab, PUT /api/v1/domains/{id}/relay
5. Send Test Email — to-address input, from auto-populated as `test@{domainName}`, POST /api/v1/test/send

- [ ] **Step 2: Implement vertical timeline layout**

Left side: vertical timeline with step circles (green check for completed, blue for current, gray for pending) connected by lines.

Current step expands to show its form/content inline. Completed steps show summary text. Pending steps show title only, grayed out.

Clicking a completed step re-expands it (non-linear navigation).

- [ ] **Step 3: Implement each step's form**

Step 1: Domain name input + selector input (default "signpost") + Create button
Step 2: "Generate DKIM Keys" button + success message with selector info
Step 3: DNS records table (reuse DNS records display logic from Domains page) + "Skip for now" + "Next" buttons
Step 4: Relay method selector + credential fields (reuse relay form logic) + Save button
Step 5: To-address input + "Send Test" button + result display (sent/failed with details)

- [ ] **Step 4: Handle first-run detection**

On mount, check if any domains exist via `GET /api/v1/domains`. If empty array, auto-start at step 1. Otherwise show a "Set up a new domain" intro with "Start" button.

- [ ] **Step 5: Test the full wizard flow**

Run through: create domain → generate DKIM → view DNS → configure relay → send test. Verify each step works and auto-advances.

- [ ] **Step 6: Commit**

```bash
git add web/src/pages/Wizard.tsx
git commit -m "feat: implement Setup Wizard with vertical checklist and 5-step flow"
```

---

## Task 10: Dockerfile Update

**Files:**
- Modify: `Dockerfile`
- Modify: `.dockerignore`

- [ ] **Step 1: Add Node.js build stage to Dockerfile**

Update `Dockerfile` to 3 stages:

```dockerfile
###############################################################################
# Stage 1: Build the frontend
###############################################################################
FROM node:20-alpine AS frontend

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

###############################################################################
# Stage 2: Build the SignPost Go binary
###############################################################################
FROM golang:1.24-alpine AS builder

ARG VERSION=dev

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Copy built frontend into the Go build context for embed
COPY --from=frontend /build/web/dist web/dist/
RUN CGO_ENABLED=1 go build -ldflags "-X main.version=${VERSION}" -o signpost ./cmd/signpost/

###############################################################################
# Stage 3: Final image based on Maddy
###############################################################################
FROM foxcpp/maddy:0.9.2
# ... (rest unchanged)
```

- [ ] **Step 2: Update .dockerignore**

Add `web/node_modules` and `web/dist` (built in container, not from host):

```
web/node_modules/
web/dist/
```

- [ ] **Step 3: Test Docker build**

```bash
docker build -t signpost:dev .
```

Expected: builds successfully with all 3 stages.

- [ ] **Step 4: Test full container**

```bash
docker compose -f docker-compose.dev.yml down
sudo rm -rf data/ && mkdir data
docker compose -f docker-compose.dev.yml up --build -d
sleep 8
curl http://localhost:8080/api/v1/healthz
curl -s http://localhost:8080/ | head -5
```

Expected: healthz returns OK, root returns the React SPA HTML.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .dockerignore
git commit -m "feat: add Node.js build stage to Dockerfile for frontend embed"
```

---

## Task 11: Frontend Tests

**Files:**
- Create: `web/src/__tests__/api.test.ts`
- Create: `web/src/__tests__/Wizard.test.tsx`

- [ ] **Step 1: Set up Vitest**

```bash
cd web
npm install -D vitest @testing-library/react @testing-library/jest-dom jsdom msw
```

Add to `web/vite.config.ts`:

```typescript
// Add to defineConfig:
test: {
  globals: true,
  environment: 'jsdom',
  setupFiles: ['./src/__tests__/setup.ts'],
},
```

Create `web/src/__tests__/setup.ts`:
```typescript
import '@testing-library/jest-dom';
```

- [ ] **Step 2: Write API client tests**

Test `api.ts`: verify auth header is set, 401 clears credentials, errors are thrown with correct status.

- [ ] **Step 3: Write Wizard step progression test**

Test that the wizard renders all 5 steps, current step is expanded, completed steps show checkmarks.

- [ ] **Step 4: Run tests**

```bash
cd web && npx vitest run
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/__tests__/ web/vite.config.ts
git commit -m "feat: add frontend tests for API client and wizard"
```

---

## Task 12: Final Integration Verification

- [ ] **Step 1: Run full Go test suite**

```bash
CGO_ENABLED=1 go test -race ./internal/...
```

Expected: all tests pass.

- [ ] **Step 2: Run frontend tests**

```bash
cd web && npx vitest run
```

Expected: all tests pass.

- [ ] **Step 3: Full Docker build + smoke test**

```bash
docker compose -f docker-compose.dev.yml down
sudo rm -rf data/ && mkdir data
docker compose -f docker-compose.dev.yml up --build -d
sleep 10
# Verify health
curl http://localhost:8080/api/v1/healthz
# Verify SPA loads
curl -s http://localhost:8080/ | grep -o '<title>.*</title>'
# Verify API still works
curl -s -u admin:admin http://localhost:8080/api/v1/domains
# Verify SMTP
echo "QUIT" | nc -w 3 localhost 25
```

Expected: all checks pass — health OK, SPA HTML returned, API works, SMTP responds.

- [ ] **Step 4: Update CLAUDE.md**

Update the test count, implementation status (mark Phase 2 tasks complete), and "How to Pick Up" section.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for Phase 2 completion"
```

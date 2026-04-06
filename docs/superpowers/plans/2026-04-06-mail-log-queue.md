# Mail Log & Queue Visibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Capture all mail flowing through Maddy in real-time, wrap relay targets in queues for retry safety, and provide queue visibility in the web UI.

**Architecture:** A log tailer goroutine parses Maddy's structured stderr (captured to file by s6-log) and writes events to SQLite. A queue scanner reads Maddy's `.meta` files every 30s. The Maddy config template wraps all relay targets in `target.queue` blocks for retry safety. The frontend gets enhanced search/filters and a queue tab.

**Tech Stack:** Go 1.24, SQLite (go-sqlite3), chi router, React 19, TypeScript, Tailwind v4, shadcn/ui, s6-overlay, Maddy 0.9.2

**Spec:** `docs/superpowers/specs/2026-04-06-mail-log-queue-design.md`

---

## File Structure

### New Files
| File | Responsibility |
|---|---|
| `internal/logtail/tailer.go` | Tails Maddy log file, parses structured events, writes to DB |
| `internal/logtail/tailer_test.go` | Tests for log line parsing and event correlation |
| `internal/queue/scanner.go` | Scans Maddy queue directories, reads `.meta` files |
| `internal/queue/scanner_test.go` | Tests for `.meta` file parsing |
| `rootfs/etc/s6-overlay/s6-rc.d/maddy/log/run` | s6 log service capturing Maddy stderr to file (conventional s6 logging) |

### Modified Files
| File | Changes |
|---|---|
| `internal/db/migrations.go` | Migration 7: add msg_id, source_ip, source_port, attempt_count, direction columns |
| `internal/db/models.go` | Add new fields to MailLogEntry struct |
| `internal/db/maillog.go` | Add LogMailEvent(), enhance ListMailLog() with search/date filters |
| `internal/api/handlers.go` | Enhance handleGetLogs(), add handleGetQueue() |
| `internal/api/server.go` | Register queue route, accept tailer/scanner dependencies |
| `cmd/signpost/main.go` | Start tailer and scanner goroutines with context |
| `templates/maddy.conf.tmpl` | Add `log stderr_ts`, wrap relay targets in queues |
| `web/src/types.ts` | Add new fields to MailLogEntry, add QueueItem types |
| `web/src/pages/MailLog.tsx` | Enhanced UI: search, date filters, expandable rows, queue tab |
| `web/src/api.ts` | Add queue endpoint helper |
| `Dockerfile` | Add maddy log directory to mkdir |

---

### Task 1: Database schema migration and model updates

**Files:**
- Modify: `internal/db/migrations.go`
- Modify: `internal/db/models.go`
- Modify: `internal/db/db_test.go`

- [ ] **Step 1: Add migration 7 to migrations.go**

Add after the last migration in the `migrations` slice:

```go
// Migration 7: Add fields for Maddy log tailer and enhanced mail tracking.
`ALTER TABLE mail_log ADD COLUMN msg_id TEXT;
 ALTER TABLE mail_log ADD COLUMN source_ip TEXT;
 ALTER TABLE mail_log ADD COLUMN source_port TEXT;
 ALTER TABLE mail_log ADD COLUMN attempt_count INTEGER DEFAULT 0;
 ALTER TABLE mail_log ADD COLUMN direction TEXT DEFAULT 'outbound';
 CREATE UNIQUE INDEX idx_mail_log_msg_id ON mail_log(msg_id) WHERE msg_id IS NOT NULL;`,
```

- [ ] **Step 2: Update MailLogEntry struct in models.go**

Add new fields after `DKIMSigned`:

```go
type MailLogEntry struct {
    ID           int64     `json:"id"`
    Timestamp    time.Time `json:"timestamp"`
    FromAddr     string    `json:"from_addr"`
    ToAddr       string    `json:"to_addr"`
    DomainID     *int64    `json:"domain_id,omitempty"`
    Subject      *string   `json:"subject,omitempty"`
    Status       string    `json:"status"`
    RelayHost    *string   `json:"relay_host,omitempty"`
    Error        *string   `json:"error,omitempty"`
    DKIMSigned   bool      `json:"dkim_signed"`
    MsgID        *string   `json:"msg_id,omitempty"`
    SourceIP     *string   `json:"source_ip,omitempty"`
    SourcePort   *string   `json:"source_port,omitempty"`
    AttemptCount int       `json:"attempt_count"`
    Direction    string    `json:"direction"`
}
```

- [ ] **Step 3: Update schema version checks in db_test.go and server_test.go**

Change `version != 6` to `version != 7` in both `TestOpen` and `TestOpenIdempotent` in `db_test.go`, and the status test in `server_test.go`.

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/db/`
Expected: PASS (migration applies, struct compiles)

- [ ] **Step 5: Commit**

```bash
git add internal/db/migrations.go internal/db/models.go internal/db/db_test.go internal/api/server_test.go
git commit -m "feat: add mail log schema migration for tailer fields"
```

---

### Task 2: Enhanced mail log DB queries

**Files:**
- Modify: `internal/db/maillog.go`
- Test: `internal/db/db_test.go`

- [ ] **Step 1: Write tests for LogMailEvent and enhanced ListMailLog**

In `db_test.go`, add:

```go
func TestLogMailEvent(t *testing.T) {
    db := testDB(t)

    // Insert new event
    err := db.LogMailEvent("abc123", "csb@drcs.ca", "d@drcs.ca", "accepted", nil, nil, "172.21.0.1", "587", false)
    if err != nil {
        t.Fatalf("LogMailEvent insert: %v", err)
    }

    // Verify it was created
    entries, _ := db.ListMailLog(MailLogFilter{Limit: 10})
    if len(entries) != 1 {
        t.Fatalf("expected 1 entry, got %d", len(entries))
    }
    if entries[0].MsgID == nil || *entries[0].MsgID != "abc123" {
        t.Error("expected msg_id abc123")
    }
    if entries[0].Status != "accepted" {
        t.Errorf("expected status accepted, got %s", entries[0].Status)
    }

    // Update existing event (delivery)
    relayHost := "smtp.gmail.com"
    err = db.LogMailEvent("abc123", "csb@drcs.ca", "d@drcs.ca", "sent", &relayHost, nil, "", "", true)
    if err != nil {
        t.Fatalf("LogMailEvent update: %v", err)
    }

    entries, _ = db.ListMailLog(MailLogFilter{Limit: 10})
    if len(entries) != 1 {
        t.Fatalf("expected 1 entry after update, got %d", len(entries))
    }
    if entries[0].Status != "sent" {
        t.Errorf("expected status sent after update, got %s", entries[0].Status)
    }
}

func TestListMailLogSearch(t *testing.T) {
    db := testDB(t)

    db.LogMailEvent("msg1", "alice@drcs.ca", "bob@example.com", "sent", nil, nil, "", "", false)
    db.LogMailEvent("msg2", "csb@drcs.ca", "d@drcs.ca", "failed", nil, nil, "", "", false)

    search := "alice"
    entries, _ := db.ListMailLog(MailLogFilter{Search: &search, Limit: 10})
    if len(entries) != 1 {
        t.Errorf("expected 1 result for search 'alice', got %d", len(entries))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `CGO_ENABLED=1 go test -race -run "TestLogMailEvent|TestListMailLogSearch" ./internal/db/`
Expected: FAIL (functions don't exist yet)

- [ ] **Step 3: Update MailLogFilter struct**

In `maillog.go`, update the filter:

```go
type MailLogFilter struct {
    Status   *string
    DomainID *int64
    Search   *string
    FromDate *string // ISO 8601
    ToDate   *string // ISO 8601
    Limit    int
    Offset   int
}
```

- [ ] **Step 4: Implement LogMailEvent**

In `maillog.go`, add:

```go
// LogMailEvent creates or updates a mail log entry based on msg_id.
// Used by the Maddy log tailer for real-time event capture.
func (db *DB) LogMailEvent(msgID, fromAddr, toAddr, status string, relayHost, sendErr *string, sourceIP, sourcePort string, dkimSigned bool) error {
    _, err := db.Exec(`INSERT INTO mail_log (msg_id, from_addr, to_addr, status, relay_host, error, source_ip, source_port, dkim_signed, direction)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'outbound')
        ON CONFLICT(msg_id) DO UPDATE SET
            from_addr = COALESCE(NULLIF(excluded.from_addr, ''), from_addr),
            to_addr = COALESCE(NULLIF(excluded.to_addr, ''), to_addr),
            status = excluded.status,
            relay_host = COALESCE(excluded.relay_host, relay_host),
            error = COALESCE(excluded.error, error),
            source_ip = COALESCE(NULLIF(excluded.source_ip, ''), source_ip),
            source_port = COALESCE(NULLIF(excluded.source_port, ''), source_port),
            dkim_signed = CASE WHEN excluded.dkim_signed THEN 1 ELSE dkim_signed END,
            attempt_count = attempt_count + CASE WHEN excluded.status IN ('deferred','failed') THEN 1 ELSE 0 END`,
        msgID, fromAddr, toAddr, status, relayHost, sendErr, sourceIP, sourcePort, dkimSigned)
    if err != nil {
        return fmt.Errorf("logging mail event: %w", err)
    }
    return nil
}
```

- [ ] **Step 5: Enhance ListMailLog with search and date filters**

Update `ListMailLog` in `maillog.go` to handle the new filter fields. Add `WHERE` clauses for:
- `search`: `AND (from_addr LIKE ? OR to_addr LIKE ? OR msg_id LIKE ? OR error LIKE ?)`
- `fromDate`: `AND timestamp >= ?`
- `toDate`: `AND timestamp <= ?`

Update the Scan call to include the new fields: `MsgID`, `SourceIP`, `SourcePort`, `AttemptCount`, `Direction`.

- [ ] **Step 7: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/db/`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add internal/db/maillog.go internal/db/migrations.go internal/db/db_test.go
git commit -m "feat: add LogMailEvent upsert and enhanced log filters"
```

---

### Task 3: Maddy log parser (internal/logtail)

**Files:**
- Create: `internal/logtail/tailer.go`
- Create: `internal/logtail/tailer_test.go`

- [ ] **Step 1: Write tests for log line parsing**

Create `internal/logtail/tailer_test.go`:

```go
package logtail

import "testing"

func TestParseLogLine(t *testing.T) {
    tests := []struct {
        name   string
        line   string
        module string
        msg    string
        fields map[string]string
    }{
        {
            name:   "incoming message with timestamp",
            line:   "2026-04-06T14:18:55.123Z submission: incoming message\t{\"msg_id\":\"abc123\",\"sender\":\"csb@drcs.ca\",\"src_ip\":\"172.21.0.1:58098\",\"username\":\"csb\"}",
            module: "submission",
            msg:    "incoming message",
            fields: map[string]string{"msg_id": "abc123", "sender": "csb@drcs.ca", "src_ip": "172.21.0.1:58098"},
        },
        {
            name:   "incoming message without timestamp",
            line:   "submission: incoming message\t{\"msg_id\":\"abc123\",\"sender\":\"csb@drcs.ca\",\"src_ip\":\"172.21.0.1:58098\"}",
            module: "submission",
            msg:    "incoming message",
            fields: map[string]string{"msg_id": "abc123", "sender": "csb@drcs.ca"},
        },
        {
            name:   "delivered",
            line:   "queue: delivered\t{\"attempt\":1,\"msg_id\":\"abc123\",\"rcpt\":\"d@drcs.ca\"}",
            module: "queue",
            msg:    "delivered",
            fields: map[string]string{"msg_id": "abc123", "rcpt": "d@drcs.ca"},
        },
        {
            name:   "RCPT ok",
            line:   "submission: RCPT ok\t{\"msg_id\":\"abc123\",\"rcpt\":\"d@drcs.ca\"}",
            module: "submission",
            msg:    "RCPT ok",
            fields: map[string]string{"msg_id": "abc123", "rcpt": "d@drcs.ca"},
        },
        {
            name:   "RCPT error",
            line:   "submission: RCPT error\t{\"msg_id\":\"abc123\",\"rcpt\":\"d@drcs.ca\",\"smtp_msg\":\"Sender domain not configured\"}",
            module: "submission",
            msg:    "RCPT error",
            fields: map[string]string{"msg_id": "abc123", "rcpt": "d@drcs.ca", "smtp_msg": "Sender domain not configured"},
        },
        {
            name:   "delivery attempt failed",
            line:   "queue: delivery attempt failed\t{\"msg_id\":\"abc123\",\"rcpt\":\"d@drcs.ca\",\"reason\":\"dial tcp: connection refused\"}",
            module: "queue",
            msg:    "delivery attempt failed",
            fields: map[string]string{"msg_id": "abc123", "reason": "dial tcp: connection refused"},
        },
        {
            name:   "auth failed",
            line:   "submission/sasl: authentication failed\t{\"reason\":\"unknown credentials\",\"src_ip\":\"172.21.0.1:36450\",\"username\":\"csb\"}",
            module: "submission/sasl",
            msg:    "authentication failed",
            fields: map[string]string{"username": "csb", "src_ip": "172.21.0.1:36450"},
        },
        {
            name:   "no json context",
            line:   "server started\t{\"version\":\"0.9.2\"}",
            module: "",
            msg:    "server started",
            fields: map[string]string{"version": "0.9.2"},
        },
        {
            name:   "plain text no tab",
            line:   "loading configuration...",
            module: "",
            msg:    "loading configuration...",
            fields: nil,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            evt := ParseLogLine(tt.line)
            if evt.Module != tt.module {
                t.Errorf("module: got %q, want %q", evt.Module, tt.module)
            }
            if evt.Message != tt.msg {
                t.Errorf("message: got %q, want %q", evt.Message, tt.msg)
            }
            for k, v := range tt.fields {
                if evt.Fields[k] != v {
                    t.Errorf("field %s: got %q, want %q", k, evt.Fields[k], v)
                }
            }
        })
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `CGO_ENABLED=1 go test -race ./internal/logtail/`
Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Implement ParseLogLine**

Create `internal/logtail/tailer.go`:

```go
package logtail

import (
    "encoding/json"
    "strings"
)

// LogEvent represents a parsed Maddy log line.
type LogEvent struct {
    Timestamp string
    Module    string
    Message   string
    Fields    map[string]string
}

// ParseLogLine parses a Maddy structured log line.
// Format: [TIMESTAMP ]MODULE: MESSAGE\t{JSON}
// Or:     [TIMESTAMP ]MESSAGE\t{JSON}
// Or:     plain text (no tab)
func ParseLogLine(line string) LogEvent {
    var evt LogEvent

    // Strip optional ISO 8601 timestamp prefix
    if len(line) > 24 && line[4] == '-' && line[10] == 'T' {
        evt.Timestamp = line[:24]
        line = strings.TrimLeft(line[24:], " ")
    }

    // Split on tab to separate message from JSON context
    parts := strings.SplitN(line, "\t", 2)
    prefix := parts[0]

    // Parse JSON context if present
    if len(parts) == 2 {
        var raw map[string]interface{}
        if err := json.Unmarshal([]byte(parts[1]), &raw); err == nil {
            evt.Fields = make(map[string]string, len(raw))
            for k, v := range raw {
                switch val := v.(type) {
                case string:
                    evt.Fields[k] = val
                case float64:
                    if val == float64(int(val)) {
                        evt.Fields[k] = fmt.Sprintf("%d", int(val))
                    } else {
                        evt.Fields[k] = fmt.Sprintf("%v", val)
                    }
                default:
                    evt.Fields[k] = fmt.Sprintf("%v", val)
                }
            }
        }
    }

    // Parse module and message from prefix
    // Format: "MODULE: MESSAGE" or just "MESSAGE"
    if idx := strings.Index(prefix, ": "); idx != -1 {
        evt.Module = prefix[:idx]
        evt.Message = prefix[idx+2:]
    } else {
        evt.Message = prefix
    }

    return evt
}
```

Add `"fmt"` to imports.

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/logtail/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/logtail/
git commit -m "feat: add Maddy log line parser with tests"
```

---

### Task 4: Log tailer goroutine

**Files:**
- Modify: `internal/logtail/tailer.go`
- Modify: `internal/logtail/tailer_test.go`

- [ ] **Step 1: Write test for event processing logic**

Add to `tailer_test.go`:

```go
func TestProcessEvent(t *testing.T) {
    // Test that incoming message creates the right DB call args
    evt := LogEvent{
        Module:  "submission",
        Message: "incoming message",
        Fields:  map[string]string{"msg_id": "abc123", "sender": "csb@drcs.ca", "src_ip": "172.21.0.1:58098"},
    }

    args := EventToMailLog(evt)
    if args == nil {
        t.Fatal("expected non-nil args for incoming message")
    }
    if args.MsgID != "abc123" {
        t.Errorf("msg_id: got %q, want abc123", args.MsgID)
    }
    if args.FromAddr != "csb@drcs.ca" {
        t.Errorf("from: got %q, want csb@drcs.ca", args.FromAddr)
    }
    if args.Status != "accepted" {
        t.Errorf("status: got %q, want accepted", args.Status)
    }
    if args.SourcePort != "587" {
        t.Errorf("source_port: got %q, want 587", args.SourcePort)
    }
}

func TestProcessEventDelivered(t *testing.T) {
    evt := LogEvent{
        Module:  "queue",
        Message: "delivered",
        Fields:  map[string]string{"msg_id": "abc123", "rcpt": "d@drcs.ca", "attempt": "1"},
    }
    args := EventToMailLog(evt)
    if args == nil {
        t.Fatal("expected non-nil args")
    }
    if args.Status != "sent" {
        t.Errorf("status: got %q, want sent", args.Status)
    }
}

func TestProcessEventIgnored(t *testing.T) {
    evt := LogEvent{
        Module:  "",
        Message: "loading configuration...",
    }
    args := EventToMailLog(evt)
    if args != nil {
        t.Error("expected nil args for non-mail event")
    }
}
```

- [ ] **Step 2: Implement EventToMailLog and MailLogArgs**

Add to `tailer.go`:

```go
// MailLogArgs holds the parsed data for a mail log DB write.
type MailLogArgs struct {
    MsgID      string
    FromAddr   string
    ToAddr     string
    Status     string
    RelayHost  *string
    Error      *string
    SourceIP   string
    SourcePort string
    DKIMSigned bool
}

// EventToMailLog converts a parsed log event into mail log arguments.
// Returns nil if the event is not a mail-related event.
func EventToMailLog(evt LogEvent) *MailLogArgs {
    msgID := evt.Fields["msg_id"]
    if msgID == "" {
        return nil
    }

    switch evt.Message {
    case "incoming message":
        sourceIP := evt.Fields["src_ip"]
        // Strip port from src_ip (e.g., "172.21.0.1:58098" -> "172.21.0.1")
        if idx := strings.LastIndex(sourceIP, ":"); idx != -1 {
            sourceIP = sourceIP[:idx]
        }
        port := "25"
        if evt.Module == "submission" {
            port = "587"
        }
        return &MailLogArgs{
            MsgID:      msgID,
            FromAddr:   evt.Fields["sender"],
            ToAddr:     evt.Fields["rcpt"], // may be empty, filled by RCPT ok
            Status:     "accepted",
            SourceIP:   sourceIP,
            SourcePort: port,
        }

    case "RCPT ok":
        return &MailLogArgs{
            MsgID:  msgID,
            ToAddr: evt.Fields["rcpt"],
            Status: "accepted",
        }

    case "delivered":
        return &MailLogArgs{
            MsgID:  msgID,
            ToAddr: evt.Fields["rcpt"],
            Status: "sent",
        }

    case "delivery attempt failed":
        reason := evt.Fields["reason"]
        return &MailLogArgs{
            MsgID:  msgID,
            Status: "deferred",
            Error:  &reason,
        }

    case "not delivered, permanent error":
        reason := evt.Fields["reason"]
        return &MailLogArgs{
            MsgID:  msgID,
            Status: "failed",
            Error:  &reason,
        }

    case "RCPT error":
        errMsg := evt.Fields["smtp_msg"]
        return &MailLogArgs{
            MsgID:  msgID,
            ToAddr: evt.Fields["rcpt"],
            Status: "rejected",
            Error:  &errMsg,
        }

    default:
        return nil
    }
}
```

Also handle `authentication failed` events separately in the tailer (these have no `msg_id`). Add to `EventToMailLog` before the `msgID == ""` check:

```go
// Handle auth failures (no msg_id — use username+timestamp as key)
if evt.Message == "authentication failed" {
    username := evt.Fields["username"]
    srcIP := evt.Fields["src_ip"]
    if idx := strings.LastIndex(srcIP, ":"); idx != -1 {
        srcIP = srcIP[:idx]
    }
    reason := evt.Fields["reason"]
    return &MailLogArgs{
        MsgID:      fmt.Sprintf("auth-%s-%d", username, time.Now().UnixNano()),
        FromAddr:   username,
        Status:     "rejected",
        Error:      &reason,
        SourceIP:   srcIP,
        SourcePort: "587",
    }
}
```

**Deferred from spec (acceptable for MVP):**
- LRU deduplication cache (10k entries) — the tailer seeks to end on startup, so re-processing is unlikely in normal operation
- Position persistence to `tailer.pos` — same reason; events between crash and restart may be missed but not duplicated
```

- [ ] **Step 3: Implement the Tailer struct and Run method**

Add to `tailer.go`:

```go
import (
    "bufio"
    "context"
    "io"
    "log"
    "os"
    "time"
)

// DBWriter is the interface the tailer uses to write log events.
type DBWriter interface {
    LogMailEvent(msgID, fromAddr, toAddr, status string, relayHost, sendErr *string, sourceIP, sourcePort string, dkimSigned bool) error
}

// Tailer watches a Maddy log file and writes parsed events to the database.
type Tailer struct {
    logPath string
    db      DBWriter
}

// NewTailer creates a new log tailer.
func NewTailer(logPath string, db DBWriter) *Tailer {
    return &Tailer{logPath: logPath, db: db}
}

// Run starts the tailer. Blocks until context is cancelled.
func (t *Tailer) Run(ctx context.Context) {
    for {
        if err := t.tailFile(ctx); err != nil {
            if ctx.Err() != nil {
                return
            }
            log.Printf("Log tailer error: %v, retrying in 2s", err)
        }
        select {
        case <-ctx.Done():
            return
        case <-time.After(2 * time.Second):
        }
    }
}

func (t *Tailer) tailFile(ctx context.Context) error {
    f, err := os.Open(t.logPath)
    if err != nil {
        return err
    }
    defer f.Close()

    // Seek to end — only process new lines
    f.Seek(0, io.SeekEnd)

    reader := bufio.NewReader(f)
    for {
        select {
        case <-ctx.Done():
            return nil
        default:
        }

        line, err := reader.ReadString('\n')
        if err != nil {
            // EOF — wait for more data
            time.Sleep(200 * time.Millisecond)
            continue
        }

        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }

        evt := ParseLogLine(line)
        args := EventToMailLog(evt)
        if args == nil {
            continue
        }

        if err := t.db.LogMailEvent(
            args.MsgID, args.FromAddr, args.ToAddr, args.Status,
            args.RelayHost, args.Error, args.SourceIP, args.SourcePort,
            args.DKIMSigned,
        ); err != nil {
            log.Printf("Failed to log mail event: %v", err)
        }
    }
}
```

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/logtail/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/logtail/
git commit -m "feat: add log tailer goroutine with event processing"
```

---

### Task 5: Queue scanner

**Files:**
- Create: `internal/queue/scanner.go`
- Create: `internal/queue/scanner_test.go`

- [ ] **Step 1: Write tests for .meta file parsing**

Create `internal/queue/scanner_test.go`:

```go
package queue

import (
    "os"
    "path/filepath"
    "testing"
)

func TestParseMetaFile(t *testing.T) {
    content := `{"MsgMeta":{},"From":"csb@drcs.ca","To":["d@drcs.ca"],"FailedRcpts":[],"TemporaryFailedRcpts":["d@drcs.ca"],"RcptErrs":{"d@drcs.ca":"dial tcp: connection refused"},"TriesCount":{"d@drcs.ca":3},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:45:00Z"}`

    dir := t.TempDir()
    metaPath := filepath.Join(dir, "abc123.meta")
    os.WriteFile(metaPath, []byte(content), 0644)

    item, err := parseMetaFile(metaPath)
    if err != nil {
        t.Fatalf("parseMetaFile: %v", err)
    }
    if item.From != "csb@drcs.ca" {
        t.Errorf("from: got %q", item.From)
    }
    if len(item.Recipients) != 1 {
        t.Fatalf("expected 1 recipient, got %d", len(item.Recipients))
    }
    if item.Recipients[0].Attempts != 3 {
        t.Errorf("attempts: got %d", item.Recipients[0].Attempts)
    }
    if item.Recipients[0].LastError != "dial tcp: connection refused" {
        t.Errorf("error: got %q", item.Recipients[0].LastError)
    }
}

func TestScanDirectory(t *testing.T) {
    dir := t.TempDir()

    // Create two fake queue entries
    os.WriteFile(filepath.Join(dir, "msg1.meta"),
        []byte(`{"From":"a@drcs.ca","To":["b@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":[],"RcptErrs":{},"TriesCount":{},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:00:00Z"}`), 0644)
    os.WriteFile(filepath.Join(dir, "msg2.meta"),
        []byte(`{"From":"c@drcs.ca","To":["d@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":["d@example.com"],"RcptErrs":{"d@example.com":"timeout"},"TriesCount":{"d@example.com":2},"FirstAttempt":"2026-04-06T13:00:00Z","LastAttempt":"2026-04-06T14:30:00Z"}`), 0644)
    // Non-meta file should be ignored
    os.WriteFile(filepath.Join(dir, "msg1.header"), []byte("header"), 0644)

    items, err := ScanDirectory(dir, "test_queue")
    if err != nil {
        t.Fatalf("ScanDirectory: %v", err)
    }
    if len(items) != 2 {
        t.Errorf("expected 2 items, got %d", len(items))
    }
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `CGO_ENABLED=1 go test -race ./internal/queue/`
Expected: FAIL (package doesn't exist)

- [ ] **Step 3: Implement scanner**

Create `internal/queue/scanner.go`:

```go
package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"
    "sync"
    "time"
)

// QueueItem represents a message in Maddy's delivery queue.
type QueueItem struct {
    MsgID      string           `json:"msg_id"`
    From       string           `json:"from"`
    Recipients []QueueRecipient `json:"recipients"`
    QueuedAt   string           `json:"queued_at"`
    QueueName  string           `json:"queue_name"`
}

// QueueRecipient represents a recipient's delivery status in the queue.
type QueueRecipient struct {
    Address     string `json:"address"`
    Attempts    int    `json:"attempts"`
    LastAttempt string `json:"last_attempt,omitempty"`
    LastError   string `json:"last_error,omitempty"`
    Status      string `json:"status"` // "pending" or "retrying"
}

// maddyMeta matches Maddy's QueueMetadata JSON structure.
type maddyMeta struct {
    From                  string            `json:"From"`
    To                    []string          `json:"To"`
    FailedRcpts           []string          `json:"FailedRcpts"`
    TemporaryFailedRcpts  []string          `json:"TemporaryFailedRcpts"`
    RcptErrs              map[string]string `json:"RcptErrs"`
    TriesCount            map[string]int    `json:"TriesCount"`
    FirstAttempt          string            `json:"FirstAttempt"`
    LastAttempt           string            `json:"LastAttempt"`
}

func parseMetaFile(path string) (*QueueItem, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var meta maddyMeta
    if err := json.Unmarshal(data, &meta); err != nil {
        return nil, err
    }

    msgID := strings.TrimSuffix(filepath.Base(path), ".meta")

    var recipients []QueueRecipient
    allRcpts := make(map[string]bool)
    for _, r := range meta.To {
        allRcpts[r] = true
    }
    for _, r := range meta.TemporaryFailedRcpts {
        allRcpts[r] = true
    }

    for addr := range allRcpts {
        rcpt := QueueRecipient{
            Address:   addr,
            Attempts:  meta.TriesCount[addr],
            LastError: meta.RcptErrs[addr],
            Status:    "pending",
        }
        if rcpt.Attempts > 0 {
            rcpt.Status = "retrying"
            rcpt.LastAttempt = meta.LastAttempt
        }
        recipients = append(recipients, rcpt)
    }

    return &QueueItem{
        MsgID:      msgID,
        From:       meta.From,
        Recipients: recipients,
        QueuedAt:   meta.FirstAttempt,
    }, nil
}

// ScanDirectory reads all .meta files in a directory.
func ScanDirectory(dir, queueName string) ([]QueueItem, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }

    var items []QueueItem
    for _, e := range entries {
        if !strings.HasSuffix(e.Name(), ".meta") {
            continue
        }
        item, err := parseMetaFile(filepath.Join(dir, e.Name()))
        if err != nil {
            continue // skip corrupt meta files
        }
        item.QueueName = queueName
        items = append(items, *item)
    }
    return items, nil
}

// Scanner periodically scans Maddy queue directories.
type Scanner struct {
    stateDir string
    mu       sync.RWMutex
    items    []QueueItem
}

// NewScanner creates a new queue scanner.
func NewScanner(stateDir string) *Scanner {
    return &Scanner{stateDir: stateDir}
}

// Run starts periodic scanning. Blocks until context is cancelled.
func (s *Scanner) Run(ctx context.Context) {
    s.scan() // initial scan
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.scan()
        }
    }
}

func (s *Scanner) scan() {
    var all []QueueItem

    // Discover queue directories dynamically
    entries, err := os.ReadDir(s.stateDir)
    if err != nil {
        return
    }
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        dir := filepath.Join(s.stateDir, e.Name())
        // Check if directory contains .meta files
        items, err := ScanDirectory(dir, e.Name())
        if err != nil {
            continue
        }
        all = append(all, items...)
    }

    s.mu.Lock()
    s.items = all
    s.mu.Unlock()
}

// Items returns the current queue snapshot.
func (s *Scanner) Items() []QueueItem {
    s.mu.RLock()
    defer s.mu.RUnlock()
    result := make([]QueueItem, len(s.items))
    copy(result, s.items)
    return result
}
```

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/queue/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/queue/
git commit -m "feat: add Maddy queue directory scanner"
```

---

### Task 6: Maddy config template changes

**Files:**
- Modify: `templates/maddy.conf.tmpl`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Add `log stderr_ts` to template**

At the top of `maddy.conf.tmpl`, after the `hostname` line (line 8), add:

```
log stderr_ts
```

If Maddy 0.9.2 doesn't support this (verify at deploy time), the tailer gracefully handles lines without timestamps.

- [ ] **Step 2: Wrap relay targets in queues**

Change the relay target section (lines 39-60) from:

```
target.smtp relay_{{$i}} {
```

To:

```
target.smtp relay_target_{{$i}} {
    ...
}
target.queue relay_{{$i}} {
    target &relay_target_{{$i}}
    autogenerated_msg_domain {{$.PrimaryDomain}}
    bounce {
        default_destination {
            reject 550 5.0.0 "Refusing to send DSN to non-local address"
        }
    }
}
```

Both the msmtp proxy path and the direct relay path get the same queue wrapping. The `source` blocks continue referencing `&relay_{{$i}}` unchanged.

- [ ] **Step 3: Update config tests**

Update tests in `config_test.go` that check for `relay_0` to expect `relay_target_0` for the SMTP target and `relay_0` for the queue wrapper.

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/config/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add templates/maddy.conf.tmpl internal/config/config_test.go
git commit -m "feat: wrap relay targets in queues, add log stderr_ts"
```

---

### Task 7: s6 log service for Maddy

**Files:**
- Create: `rootfs/etc/s6-overlay/s6-rc.d/maddy/log/run`
- Modify: `rootfs/etc/s6-overlay/s6-rc.d/maddy/run`
- Modify: `Dockerfile`

- [ ] **Step 1: Create s6 log service using conventional `log/` subdirectory**

Create `rootfs/etc/s6-overlay/s6-rc.d/maddy/log/run`:

```bash
#!/command/execlineb -P
s6-log -b -- n10 s1000000 /data/signpost/logs/maddy/
```

This follows s6's convention: a `log/` subdirectory under the service automatically pipes the service's stdout to the log program.

- [ ] **Step 2: Update Maddy run script to merge stderr into stdout**

Update `rootfs/etc/s6-overlay/s6-rc.d/maddy/run`:

```bash
#!/command/with-contenv sh
sleep 3
exec /bin/maddy -config /data/signpost/maddy.conf run 2>&1
```

The `2>&1` ensures Maddy's structured log output (which goes to stderr) is captured by the s6 log service.

- [ ] **Step 3: Update Dockerfile mkdir**

Change the mkdir line in Dockerfile to include the maddy log directory:

```dockerfile
RUN mkdir -p /data/signpost/dkim_keys /data/signpost/tls /data/signpost/logs /data/signpost/logs/maddy /data/signpost/backups
```

- [ ] **Step 4: Commit**

```bash
git add rootfs/ Dockerfile
git commit -m "feat: add s6 log service for Maddy stderr capture"
```

---

### Task 8: API endpoints — enhanced logs and queue

**Files:**
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/server_test.go`

- [ ] **Step 1: Update handleGetLogs for new filter params**

In `handlers.go`, enhance `handleGetLogs` to parse `search`, `from`, `to` query params and pass them to `ListMailLog`:

```go
if s := r.URL.Query().Get("search"); s != "" {
    filter.Search = &s
}
if f := r.URL.Query().Get("from"); f != "" {
    filter.FromDate = &f
}
if t := r.URL.Query().Get("to"); t != "" {
    filter.ToDate = &t
}
```

- [ ] **Step 2: Add handleGetQueue handler**

```go
func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
    items := s.queueScanner.Items()
    writeJSON(w, http.StatusOK, map[string]interface{}{
        "items": items,
        "count": len(items),
    })
}
```

- [ ] **Step 3: Update server.go**

Add `queueScanner *queue.Scanner` field to the Server struct. Register the new route:

```go
r.Get("/api/v1/queue", s.handleGetQueue)
```

Update `NewServer` to accept the scanner dependency. Add `queueScanner *queue.Scanner` as a field on the `Server` struct. Update the `NewServer` function signature to accept it. Update `testServer()` in `server_test.go` to pass `queue.NewScanner(t.TempDir())` for the new parameter.

- [ ] **Step 4: Run tests**

Run: `CGO_ENABLED=1 go test -race ./internal/api/`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat: add queue endpoint, enhance log filters"
```

---

### Task 9: Wire tailer and scanner into main.go

**Files:**
- Modify: `cmd/signpost/main.go`

- [ ] **Step 1: Start tailer and scanner with context**

After config generation, before HTTP server start:

```go
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// Start Maddy log tailer
logPath := filepath.Join(dataDir, "logs", "maddy", "current")
tailer := logtail.NewTailer(logPath, database)
go tailer.Run(ctx)

// Start queue scanner
stateDir := filepath.Join(dataDir, "maddy_state")
queueScanner := queue.NewScanner(stateDir)
go queueScanner.Run(ctx)
```

Pass `queueScanner` to `NewServer`.

- [ ] **Step 2: Run Go tests**

Run: `CGO_ENABLED=1 go test -race ./internal/...`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add cmd/signpost/main.go
git commit -m "feat: wire log tailer and queue scanner into startup"
```

---

### Task 10: Frontend — TypeScript types and API

**Files:**
- Modify: `web/src/types.ts`
- Modify: `web/src/api.ts`

- [ ] **Step 1: Update MailLogEntry interface**

```typescript
export interface MailLogEntry {
  id: number;
  timestamp: string;
  from_addr: string;
  to_addr: string;
  domain_id?: number;
  subject?: string;
  status: string; // accepted, sent, failed, deferred, rejected
  relay_host?: string;
  error?: string;
  dkim_signed: boolean;
  msg_id?: string;
  source_ip?: string;
  source_port?: string;
  attempt_count: number;
  direction: string;
}
```

- [ ] **Step 2: Add queue types**

```typescript
export interface QueueRecipient {
  address: string;
  attempts: number;
  last_attempt?: string;
  last_error?: string;
  status: string;
}

export interface QueueItem {
  msg_id: string;
  from: string;
  recipients: QueueRecipient[];
  queued_at: string;
  queue_name: string;
}

export interface QueueResponse {
  items: QueueItem[];
  count: number;
}
```

- [ ] **Step 3: Commit**

```bash
git add web/src/types.ts web/src/api.ts
git commit -m "feat: add queue types and update mail log interface"
```

---

### Task 11: Frontend — enhanced Mail Log page

**Files:**
- Modify: `web/src/pages/MailLog.tsx`

- [ ] **Step 1: Add tab bar (Mail Log / Queue)**

Add a tab state at the top of the component. Render either the log view or queue view based on active tab.

- [ ] **Step 2: Add search box and date filters**

Above the table, add:
- Search input with 300ms debounce
- Date range inputs (from/to)
- Status filter dropdown with new values: All, Accepted, Sent, Failed, Deferred, Rejected
- Clear filters button

Wire the filters into the API call query params.

- [ ] **Step 3: Update table columns**

Replace existing table with: Time | From | To | Status | Relay | DKIM | expand arrow.

Status badges:
- `accepted` → blue
- `sent` → green
- `deferred` → yellow/amber
- `failed` → red
- `rejected` → red outline/variant

DKIM column: checkmark or X icon.
Relay column: show relay_host or "direct".

- [ ] **Step 4: Add expandable row detail**

Clicking a row toggles an inline detail panel below it showing: msg_id, source IP, source port (25=SMTP / 587=Submission), attempt count, full error text. Handle null values gracefully (test sends won't have msg_id, source_ip, etc.).

- [ ] **Step 5: Build queue tab**

Queue tab fetches `GET /api/v1/queue` on mount and every 30 seconds.

Table columns: Queued At | From | To | Attempts | Last Error | Queue

Empty state: green checkmark icon + "Queue is empty — all messages delivered."
Non-empty: yellow banner at top "N messages waiting for delivery."

- [ ] **Step 6: Run frontend tests**

Run: `cd web && npx vitest run`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/src/pages/MailLog.tsx
git commit -m "feat: enhanced mail log UI with search, filters, queue tab"
```

---

### Task 12: Integration test and deploy

**Files:**
- All

- [ ] **Step 1: Run all Go tests**

Run: `CGO_ENABLED=1 go test -race ./internal/...`
Expected: ALL PASS

- [ ] **Step 2: Run frontend tests**

Run: `cd web && npx vitest run`
Expected: PASS

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: Success

- [ ] **Step 4: Deploy to dev**

Run: `docker compose -f docker-compose.dev.yml up --build -d`
Verify: `curl http://localhost:8081/api/v1/healthz`

- [ ] **Step 5: Verify Maddy log capture**

Check that `/data/signpost/logs/maddy/current` exists:
```bash
docker exec signpost-signpost-1 ls -la /data/signpost/logs/maddy/
```

- [ ] **Step 6: Send test email and verify log appears**

Send a test email via the UI or CLI, then check:
```bash
curl -u admin:admin http://localhost:8081/api/v1/logs?limit=5
```
Verify the entry has `msg_id`, `source_ip`, `source_port` populated.

- [ ] **Step 7: Check queue endpoint**

```bash
curl -u admin:admin http://localhost:8081/api/v1/queue
```
Expected: `{"items":[],"count":0}` (empty queue)

- [ ] **Step 8: Verify relay queue wrapping**

```bash
docker exec signpost-signpost-1 grep -A5 "relay_target_0\|target.queue relay_0" /data/signpost/maddy.conf
```
Expected: `relay_target_0` as the SMTP target, `relay_0` as the queue wrapping it.

- [ ] **Step 9: Commit any fixes, update CHANGELOG**

Update `CHANGELOG.md` with the new features for the next release.

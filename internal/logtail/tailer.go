package logtail

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogEvent represents a parsed Maddy log line.
type LogEvent struct {
	Timestamp string
	Module    string
	Message   string
	Fields    map[string]string
}

// MailLogArgs holds the arguments for a DB mail log write.
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

// DBWriter is the interface for writing mail log events to the database.
type DBWriter interface {
	LogMailEvent(msgID, fromAddr, toAddr, status string, relayHost, sendErr *string, sourceIP, sourcePort string, dkimSigned bool) error
}

// Tailer watches a Maddy log file and writes parsed mail events to the database.
type Tailer struct {
	logPath string
	db      DBWriter
}

// NewTailer creates a new Tailer that watches the given log file and writes events to db.
func NewTailer(logPath string, db DBWriter) *Tailer {
	return &Tailer{
		logPath: logPath,
		db:      db,
	}
}

// Run blocks and continuously tails the log file, parsing events and writing
// them to the database. It retries on file errors and respects context cancellation.
func (t *Tailer) Run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		err := t.tail(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("logtail: error tailing %s: %v, retrying in 2s", t.logPath, err)
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}
}

// tail opens the log file, seeks to end, and reads new lines until an error occurs.
func (t *Tailer) tail(ctx context.Context) error {
	f, err := os.Open(t.logPath)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	// Seek to end — only process new lines.
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("seeking to end: %w", err)
	}

	reader := bufio.NewReader(f)

	for {
		if ctx.Err() != nil {
			return nil
		}

		line, err := reader.ReadString('\n')
		if err == io.EOF {
			// Process any partial line content before sleeping.
			if line != "" {
				t.processLine(line)
			}
			select {
			case <-time.After(200 * time.Millisecond):
			case <-ctx.Done():
				return nil
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("reading line: %w", err)
		}

		t.processLine(line)
	}
}

// processLine parses a log line and writes it to the database if it's a mail event.
func (t *Tailer) processLine(line string) {
	evt := ParseLogLine(line)
	args := EventToMailLog(evt)
	if args == nil {
		return
	}

	if err := t.db.LogMailEvent(
		args.MsgID, args.FromAddr, args.ToAddr, args.Status,
		args.RelayHost, args.Error,
		args.SourceIP, args.SourcePort, args.DKIMSigned,
	); err != nil {
		log.Printf("logtail: db write error: %v", err)
	}
}

// ParseLogLine parses a Maddy structured log line into a LogEvent.
//
// Format: [TIMESTAMP ]MODULE: MESSAGE\t{JSON}
//
// The timestamp is an optional 24-char ISO 8601 prefix (e.g. 2026-04-06T14:18:55.123Z).
// The tab separates the message prefix from the JSON context fields.
// Some lines have no module (just a message) and some have no JSON.
func ParseLogLine(line string) LogEvent {
	line = strings.TrimRight(line, "\n\r")

	var evt LogEvent

	// Check for ISO 8601 timestamp prefix (24 chars like "2026-04-06T14:18:55.123Z").
	// Must start with a digit and have 'T' at position 10 and 'Z' at position 23.
	if len(line) >= 25 && line[0] >= '0' && line[0] <= '9' &&
		line[10] == 'T' && line[23] == 'Z' && line[24] == ' ' {
		evt.Timestamp = line[:24]
		line = line[25:]
	}

	// Split on tab to separate message prefix from JSON context.
	var jsonPart string
	if idx := strings.Index(line, "\t"); idx >= 0 {
		jsonPart = line[idx+1:]
		line = line[:idx]
	}

	// Split message prefix on ": " to extract module and message.
	if idx := strings.Index(line, ": "); idx >= 0 {
		evt.Module = line[:idx]
		evt.Message = line[idx+2:]
	} else {
		evt.Message = line
	}

	// Parse JSON fields if present.
	if jsonPart != "" {
		evt.Fields = parseJSONFields(jsonPart)
	}

	return evt
}

// parseJSONFields parses a JSON object string into a map of string values.
// All values (strings, numbers, bools) are converted to their string representation.
func parseJSONFields(raw string) map[string]string {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}

	result := make(map[string]string, len(m))
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result
}

// EventToMailLog converts a LogEvent into MailLogArgs for the DB write.
// Returns nil for non-mail events.
func EventToMailLog(evt LogEvent) *MailLogArgs {
	// Handle authentication failures (no msg_id).
	if evt.Message == "authentication failed" {
		username := evt.Fields["username"]
		sourceIP, _ := splitHostPort(evt.Fields["src_ip"])
		reason := evt.Fields["reason"]
		var errPtr *string
		if reason != "" {
			errPtr = &reason
		}
		return &MailLogArgs{
			MsgID:      fmt.Sprintf("auth-%s-%d", username, time.Now().UnixNano()),
			FromAddr:   username,
			Status:     "rejected",
			Error:      errPtr,
			SourceIP:   sourceIP,
			SourcePort: "587",
		}
	}

	msgID := evt.Fields["msg_id"]
	if msgID == "" {
		return nil
	}

	switch evt.Message {
	case "incoming message":
		sourceIP, _ := splitHostPort(evt.Fields["src_ip"])
		sourcePort := "25"
		if evt.Module == "submission" {
			sourcePort = "587"
		}
		return &MailLogArgs{
			MsgID:      msgID,
			FromAddr:   evt.Fields["sender"],
			Status:     "accepted",
			SourceIP:   sourceIP,
			SourcePort: sourcePort,
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
		var errPtr *string
		if reason != "" {
			errPtr = &reason
		}
		return &MailLogArgs{
			MsgID:  msgID,
			Status: "deferred",
			Error:  errPtr,
		}

	case "not delivered, permanent error":
		reason := evt.Fields["reason"]
		var errPtr *string
		if reason != "" {
			errPtr = &reason
		}
		return &MailLogArgs{
			MsgID:  msgID,
			Status: "failed",
			Error:  errPtr,
		}

	case "RCPT error":
		smtpMsg := evt.Fields["smtp_msg"]
		var errPtr *string
		if smtpMsg != "" {
			errPtr = &smtpMsg
		}
		return &MailLogArgs{
			MsgID:  msgID,
			ToAddr: evt.Fields["rcpt"],
			Status: "rejected",
			Error:  errPtr,
		}

	default:
		return nil
	}
}

// splitHostPort splits a "host:port" string. If there's no colon, returns the
// input as host and empty port.
func splitHostPort(addr string) (host, port string) {
	if addr == "" {
		return "", ""
	}
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return addr, ""
	}
	return addr[:idx], addr[idx+1:]
}

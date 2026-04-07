package queue

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMetaFile(t *testing.T) {
	content := `{"From":"csb@drcs.ca","To":["d@drcs.ca"],"FailedRcpts":[],"TemporaryFailedRcpts":["d@drcs.ca"],"RcptErrs":{"d@drcs.ca":"dial tcp: connection refused"},"TriesCount":{"d@drcs.ca":3},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:45:00Z"}`

	dir := t.TempDir()
	metaPath := filepath.Join(dir, "abc123.meta")
	os.WriteFile(metaPath, []byte(content), 0644)

	item, err := parseMetaFile(metaPath)
	if err != nil {
		t.Fatalf("parseMetaFile: %v", err)
	}
	if item.MsgID != "abc123" {
		t.Errorf("msg_id: got %q, want abc123", item.MsgID)
	}
	if item.From != "csb@drcs.ca" {
		t.Errorf("from: got %q", item.From)
	}
	if len(item.Recipients) != 1 {
		t.Fatalf("expected 1 recipient, got %d", len(item.Recipients))
	}
	if item.Recipients[0].Address != "d@drcs.ca" {
		t.Errorf("recipient: got %q", item.Recipients[0].Address)
	}
	if item.Recipients[0].Attempts != 3 {
		t.Errorf("attempts: got %d, want 3", item.Recipients[0].Attempts)
	}
	if item.Recipients[0].LastError != "dial tcp: connection refused" {
		t.Errorf("error: got %q", item.Recipients[0].LastError)
	}
	if item.Recipients[0].Status != "retrying" {
		t.Errorf("status: got %q, want retrying", item.Recipients[0].Status)
	}
	if item.QueuedAt != "2026-04-06T14:00:00Z" {
		t.Errorf("queued_at: got %q", item.QueuedAt)
	}
}

func TestParseMetaFilePending(t *testing.T) {
	content := `{"From":"a@drcs.ca","To":["b@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":[],"RcptErrs":{},"TriesCount":{},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:00:00Z"}`

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "fresh.meta"), []byte(content), 0644)

	item, err := parseMetaFile(filepath.Join(dir, "fresh.meta"))
	if err != nil {
		t.Fatal(err)
	}
	if item.Recipients[0].Status != "pending" {
		t.Errorf("expected pending status, got %q", item.Recipients[0].Status)
	}
}

func TestScanDirectory(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "msg1.meta"),
		[]byte(`{"From":"a@drcs.ca","To":["b@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":[],"RcptErrs":{},"TriesCount":{},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:00:00Z"}`), 0644)
	os.WriteFile(filepath.Join(dir, "msg2.meta"),
		[]byte(`{"From":"c@drcs.ca","To":["d@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":["d@example.com"],"RcptErrs":{"d@example.com":"timeout"},"TriesCount":{"d@example.com":2},"FirstAttempt":"2026-04-06T13:00:00Z","LastAttempt":"2026-04-06T14:30:00Z"}`), 0644)
	// Non-meta files should be ignored
	os.WriteFile(filepath.Join(dir, "msg1.header"), []byte("header"), 0644)
	os.WriteFile(filepath.Join(dir, "msg1.body"), []byte("body"), 0644)

	items, err := ScanDirectory(dir, "test_queue")
	if err != nil {
		t.Fatalf("ScanDirectory: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if item.QueueName != "test_queue" {
			t.Errorf("expected queue name test_queue, got %q", item.QueueName)
		}
	}
}

func TestScanDirectoryMissing(t *testing.T) {
	items, err := ScanDirectory("/nonexistent/path", "test")
	if err != nil {
		t.Errorf("expected nil error for missing dir, got %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items for missing dir")
	}
}

func TestScanDirectoryCorruptMeta(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "bad.meta"), []byte("not json"), 0644)
	os.WriteFile(filepath.Join(dir, "good.meta"),
		[]byte(`{"From":"a@drcs.ca","To":["b@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":[],"RcptErrs":{},"TriesCount":{},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:00:00Z"}`), 0644)

	items, err := ScanDirectory(dir, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item (corrupt skipped), got %d", len(items))
	}
}

func TestScannerItems(t *testing.T) {
	dir := t.TempDir()
	queueDir := filepath.Join(dir, "remote_queue")
	os.MkdirAll(queueDir, 0755)
	os.WriteFile(filepath.Join(queueDir, "msg1.meta"),
		[]byte(`{"From":"a@drcs.ca","To":["b@example.com"],"FailedRcpts":[],"TemporaryFailedRcpts":[],"RcptErrs":{},"TriesCount":{},"FirstAttempt":"2026-04-06T14:00:00Z","LastAttempt":"2026-04-06T14:00:00Z"}`), 0644)

	scanner := NewScanner(dir)
	scanner.scan()

	items := scanner.Items()
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
	if len(items) > 0 && items[0].QueueName != "remote_queue" {
		t.Errorf("expected queue name remote_queue, got %q", items[0].QueueName)
	}
}

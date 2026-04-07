package queue

import (
	"context"
	"encoding/json"
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
	From                 string            `json:"From"`
	To                   []string          `json:"To"`
	FailedRcpts          []string          `json:"FailedRcpts"`
	TemporaryFailedRcpts []string          `json:"TemporaryFailedRcpts"`
	RcptErrs             map[string]string `json:"RcptErrs"`
	TriesCount           map[string]int    `json:"TriesCount"`
	FirstAttempt         string            `json:"FirstAttempt"`
	LastAttempt          string            `json:"LastAttempt"`
}

// parseMetaFile reads and parses a single Maddy queue .meta file.
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

	// Collect all unique recipients from To and TemporaryFailedRcpts
	allRcpts := make(map[string]bool)
	for _, r := range meta.To {
		allRcpts[r] = true
	}
	for _, r := range meta.TemporaryFailedRcpts {
		allRcpts[r] = true
	}

	var recipients []QueueRecipient
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

// scan discovers queue directories dynamically and reads .meta files.
func (s *Scanner) scan() {
	var all []QueueItem

	entries, err := os.ReadDir(s.stateDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(s.stateDir, e.Name())
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

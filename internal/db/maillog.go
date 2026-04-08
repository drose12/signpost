package db

import (
	"database/sql"
	"fmt"
	"time"
)

// LogMail records a mail send event.
func (db *DB) LogMail(fromAddr, toAddr string, domainID *int64, subject, status string, relayHost *string, sendErr *string, dkimSigned bool) error {
	_, err := db.Exec(`INSERT INTO mail_log (from_addr, to_addr, domain_id, subject, status, relay_host, error, dkim_signed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		fromAddr, toAddr, domainID, subject, status, relayHost, sendErr, dkimSigned)
	if err != nil {
		return fmt.Errorf("logging mail: %w", err)
	}
	return nil
}

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

// MailLogFilter holds filtering options for mail log queries.
type MailLogFilter struct {
	Status   *string
	DomainID *int64
	Search   *string // LIKE search across from_addr, to_addr, msg_id, error
	FromDate *string // ISO 8601 start date
	ToDate   *string // ISO 8601 end date
	Limit    int
	Offset   int
}

// ListMailLog returns mail log entries matching the filter.
func (db *DB) ListMailLog(filter MailLogFilter) ([]MailLogEntry, error) {
	query := `SELECT id, timestamp, from_addr, to_addr, domain_id, subject, status,
		relay_host, error, dkim_signed, msg_id, source_ip, source_port, attempt_count, direction
		FROM mail_log WHERE 1=1`
	var args []interface{}

	if filter.Status != nil {
		query += ` AND status = ?`
		args = append(args, *filter.Status)
	}
	if filter.DomainID != nil {
		query += ` AND domain_id = ?`
		args = append(args, *filter.DomainID)
	}
	if filter.Search != nil {
		query += ` AND (from_addr LIKE ? OR to_addr LIKE ? OR msg_id LIKE ? OR error LIKE ?)`
		wildcard := "%" + *filter.Search + "%"
		args = append(args, wildcard, wildcard, wildcard, wildcard)
	}
	if filter.FromDate != nil {
		query += ` AND timestamp >= ?`
		args = append(args, *filter.FromDate)
	}
	if filter.ToDate != nil {
		query += ` AND timestamp <= ?`
		args = append(args, *filter.ToDate)
	}

	query += ` ORDER BY timestamp DESC`

	if filter.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, filter.Limit)
	} else {
		query += ` LIMIT 100`
	}
	if filter.Offset > 0 {
		query += ` OFFSET ?`
		args = append(args, filter.Offset)
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing mail log: %w", err)
	}
	defer rows.Close()

	var entries []MailLogEntry
	for rows.Next() {
		var e MailLogEntry
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.FromAddr, &e.ToAddr, &e.DomainID,
			&e.Subject, &e.Status, &e.RelayHost, &e.Error, &e.DKIMSigned,
			&e.MsgID, &e.SourceIP, &e.SourcePort, &e.AttemptCount, &e.Direction); err != nil {
			return nil, fmt.Errorf("scanning mail log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// LookupRelayHost returns the active relay host for a sender domain.
// Returns empty string if domain uses direct delivery or domain is not found.
func (db *DB) LookupRelayHost(senderDomain string) string {
	var host sql.NullString
	err := db.QueryRow(`SELECT rc.host FROM relay_configs rc
		JOIN domains d ON d.id = rc.domain_id
		WHERE d.name = ? AND rc.active = 1 AND rc.method != 'direct'`, senderDomain).Scan(&host)
	if err != nil || !host.Valid {
		return ""
	}
	return host.String
}

// HasDKIM returns true if the domain has a DKIM key configured.
func (db *DB) HasDKIM(senderDomain string) bool {
	var keyPath sql.NullString
	err := db.QueryRow(`SELECT dkim_key_path FROM domains WHERE name = ? AND active = 1`, senderDomain).Scan(&keyPath)
	return err == nil && keyPath.Valid && keyPath.String != ""
}

// PruneMailLog deletes log entries older than the given duration.
func (db *DB) PruneMailLog(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.Exec(`DELETE FROM mail_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("pruning mail log: %w", err)
	}
	return result.RowsAffected()
}

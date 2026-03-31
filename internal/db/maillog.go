package db

import (
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

// MailLogFilter holds filtering options for mail log queries.
type MailLogFilter struct {
	Status   *string
	DomainID *int64
	Limit    int
	Offset   int
}

// ListMailLog returns mail log entries matching the filter.
func (db *DB) ListMailLog(filter MailLogFilter) ([]MailLogEntry, error) {
	query := `SELECT id, timestamp, from_addr, to_addr, domain_id, subject, status,
		relay_host, error, dkim_signed FROM mail_log WHERE 1=1`
	var args []interface{}

	if filter.Status != nil {
		query += ` AND status = ?`
		args = append(args, *filter.Status)
	}
	if filter.DomainID != nil {
		query += ` AND domain_id = ?`
		args = append(args, *filter.DomainID)
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
			&e.Subject, &e.Status, &e.RelayHost, &e.Error, &e.DKIMSigned); err != nil {
			return nil, fmt.Errorf("scanning mail log entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
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

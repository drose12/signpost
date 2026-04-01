package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ListDomains returns all configured domains.
func (db *DB) ListDomains() ([]Domain, error) {
	rows, err := db.Query(`SELECT id, name, dkim_selector, dkim_key_path, dkim_public_dns,
		dkim_created_at, spf_record, dmarc_record, active, created_at, updated_at
		FROM domains ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}
	defer rows.Close()

	var domains []Domain
	for rows.Next() {
		var d Domain
		if err := rows.Scan(&d.ID, &d.Name, &d.DKIMSelector, &d.DKIMKeyPath,
			&d.DKIMPublicDNS, &d.DKIMCreatedAt, &d.SPFRecord, &d.DMARCRecord,
			&d.Active, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning domain: %w", err)
		}
		domains = append(domains, d)
	}
	return domains, rows.Err()
}

// GetDomain returns a single domain by ID.
func (db *DB) GetDomain(id int64) (*Domain, error) {
	var d Domain
	err := db.QueryRow(`SELECT id, name, dkim_selector, dkim_key_path, dkim_public_dns,
		dkim_created_at, spf_record, dmarc_record, active, created_at, updated_at
		FROM domains WHERE id = ?`, id).Scan(
		&d.ID, &d.Name, &d.DKIMSelector, &d.DKIMKeyPath, &d.DKIMPublicDNS,
		&d.DKIMCreatedAt, &d.SPFRecord, &d.DMARCRecord, &d.Active,
		&d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting domain %d: %w", id, err)
	}
	return &d, nil
}

// GetDomainByName returns a domain by its name.
func (db *DB) GetDomainByName(name string) (*Domain, error) {
	var d Domain
	err := db.QueryRow(`SELECT id, name, dkim_selector, dkim_key_path, dkim_public_dns,
		dkim_created_at, spf_record, dmarc_record, active, created_at, updated_at
		FROM domains WHERE name = ?`, name).Scan(
		&d.ID, &d.Name, &d.DKIMSelector, &d.DKIMKeyPath, &d.DKIMPublicDNS,
		&d.DKIMCreatedAt, &d.SPFRecord, &d.DMARCRecord, &d.Active,
		&d.CreatedAt, &d.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting domain %q: %w", name, err)
	}
	return &d, nil
}

// CreateDomain inserts a new domain and returns it with the generated ID.
func (db *DB) CreateDomain(name, selector string) (*Domain, error) {
	result, err := db.Exec(`INSERT INTO domains (name, dkim_selector) VALUES (?, ?)`, name, selector)
	if err != nil {
		return nil, fmt.Errorf("creating domain %q: %w", name, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting domain ID: %w", err)
	}
	return db.GetDomain(id)
}

// UpdateDomainDKIM updates the DKIM key information for a domain.
func (db *DB) UpdateDomainDKIM(id int64, keyPath, publicDNS string) error {
	now := time.Now()
	_, err := db.Exec(`UPDATE domains SET dkim_key_path = ?, dkim_public_dns = ?,
		dkim_created_at = ?, updated_at = ? WHERE id = ?`,
		keyPath, publicDNS, now, now, id)
	if err != nil {
		return fmt.Errorf("updating DKIM for domain %d: %w", id, err)
	}
	return nil
}

// UpdateDomainDNSRecords updates the recommended SPF and DMARC records.
func (db *DB) UpdateDomainDNSRecords(id int64, spf, dmarc string) error {
	_, err := db.Exec(`UPDATE domains SET spf_record = ?, dmarc_record = ?,
		updated_at = CURRENT_TIMESTAMP WHERE id = ?`, spf, dmarc, id)
	if err != nil {
		return fmt.Errorf("updating DNS records for domain %d: %w", id, err)
	}
	return nil
}

// DeleteDomain removes a domain by ID.
func (db *DB) DeleteDomain(id int64) error {
	// Delete dependent records first (CASCADE may not work if foreign_keys pragma
	// isn't active on all connections, e.g., Maddy's connection)
	db.Exec(`DELETE FROM relay_configs WHERE domain_id = ?`, id)
	db.Exec(`DELETE FROM smtp_user_domains WHERE domain_id = ?`, id)
	db.Exec(`UPDATE mail_log SET domain_id = NULL WHERE domain_id = ?`, id)

	result, err := db.Exec(`DELETE FROM domains WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting domain %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("domain %d not found", id)
	}
	return nil
}

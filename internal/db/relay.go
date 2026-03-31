package db

import (
	"database/sql"
	"fmt"
)

// GetRelayConfig returns the relay configuration for a domain.
func (db *DB) GetRelayConfig(domainID int64) (*RelayConfig, error) {
	var rc RelayConfig
	err := db.QueryRow(`SELECT id, domain_id, method, host, port, username,
		password_enc, password_nonce, starttls, auth_method, created_at, updated_at
		FROM relay_configs WHERE domain_id = ?`, domainID).Scan(
		&rc.ID, &rc.DomainID, &rc.Method, &rc.Host, &rc.Port, &rc.Username,
		&rc.PasswordEnc, &rc.PasswordNonce, &rc.StartTLS, &rc.AuthMethod,
		&rc.CreatedAt, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting relay config for domain %d: %w", domainID, err)
	}
	return &rc, nil
}

// UpsertRelayConfig creates or updates the relay configuration for a domain.
func (db *DB) UpsertRelayConfig(domainID int64, method string, host *string, port int, username, passwordEnc, passwordNonce *string, starttls bool) error {
	_, err := db.Exec(`INSERT INTO relay_configs (domain_id, method, host, port, username, password_enc, password_nonce, starttls)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(domain_id) DO UPDATE SET
			method = excluded.method,
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			password_enc = excluded.password_enc,
			password_nonce = excluded.password_nonce,
			starttls = excluded.starttls,
			updated_at = CURRENT_TIMESTAMP`,
		domainID, method, host, port, username, passwordEnc, passwordNonce, starttls)
	if err != nil {
		return fmt.Errorf("upserting relay config for domain %d: %w", domainID, err)
	}
	return nil
}

// UpdateRelayAuthMethod updates the auth_method for a domain's relay config.
func (db *DB) UpdateRelayAuthMethod(domainID int64, authMethod string) error {
	result, err := db.Exec(`UPDATE relay_configs SET auth_method = ?, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ?`,
		authMethod, domainID)
	if err != nil {
		return fmt.Errorf("updating auth_method for domain %d: %w", domainID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no relay config found for domain %d", domainID)
	}
	return nil
}

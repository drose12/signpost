package db

import (
	"database/sql"
	"fmt"
)

// GetRelayConfig returns the ACTIVE relay configuration for a domain.
// Returns nil if no active config exists.
func (db *DB) GetRelayConfig(domainID int64) (*RelayConfig, error) {
	var rc RelayConfig
	err := db.QueryRow(`SELECT id, domain_id, method, host, port, username,
		password_enc, password_nonce, starttls, auth_method, active, created_at, updated_at
		FROM relay_configs WHERE domain_id = ? AND active = 1`, domainID).Scan(
		&rc.ID, &rc.DomainID, &rc.Method, &rc.Host, &rc.Port, &rc.Username,
		&rc.PasswordEnc, &rc.PasswordNonce, &rc.StartTLS, &rc.AuthMethod,
		&rc.Active, &rc.CreatedAt, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting active relay config for domain %d: %w", domainID, err)
	}
	return &rc, nil
}

// GetRelayConfigByMethod returns the relay config for a specific method on a domain.
func (db *DB) GetRelayConfigByMethod(domainID int64, method string) (*RelayConfig, error) {
	var rc RelayConfig
	err := db.QueryRow(`SELECT id, domain_id, method, host, port, username,
		password_enc, password_nonce, starttls, auth_method, active, created_at, updated_at
		FROM relay_configs WHERE domain_id = ? AND method = ?`, domainID, method).Scan(
		&rc.ID, &rc.DomainID, &rc.Method, &rc.Host, &rc.Port, &rc.Username,
		&rc.PasswordEnc, &rc.PasswordNonce, &rc.StartTLS, &rc.AuthMethod,
		&rc.Active, &rc.CreatedAt, &rc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting relay config for domain %d method %s: %w", domainID, method, err)
	}
	return &rc, nil
}

// ListRelayConfigs returns ALL relay configurations for a domain (all methods).
func (db *DB) ListRelayConfigs(domainID int64) ([]RelayConfig, error) {
	rows, err := db.Query(`SELECT id, domain_id, method, host, port, username,
		password_enc, password_nonce, starttls, auth_method, active, created_at, updated_at
		FROM relay_configs WHERE domain_id = ? ORDER BY method`, domainID)
	if err != nil {
		return nil, fmt.Errorf("listing relay configs for domain %d: %w", domainID, err)
	}
	defer rows.Close()

	var configs []RelayConfig
	for rows.Next() {
		var rc RelayConfig
		if err := rows.Scan(&rc.ID, &rc.DomainID, &rc.Method, &rc.Host, &rc.Port,
			&rc.Username, &rc.PasswordEnc, &rc.PasswordNonce, &rc.StartTLS,
			&rc.AuthMethod, &rc.Active, &rc.CreatedAt, &rc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning relay config: %w", err)
		}
		configs = append(configs, rc)
	}
	return configs, rows.Err()
}

// UpsertRelayConfig creates or updates the relay configuration for a specific method on a domain.
// If active is true, all other methods for this domain are deactivated first.
func (db *DB) UpsertRelayConfig(domainID int64, method string, host *string, port int, username, passwordEnc, passwordNonce *string, starttls bool, active bool) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// If setting this config as active, deactivate all others for this domain
	if active {
		if _, err := tx.Exec(`UPDATE relay_configs SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ?`, domainID); err != nil {
			return fmt.Errorf("deactivating other relay configs: %w", err)
		}
	}

	_, err = tx.Exec(`INSERT INTO relay_configs (domain_id, method, host, port, username, password_enc, password_nonce, starttls, active)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(domain_id, method) DO UPDATE SET
			host = excluded.host,
			port = excluded.port,
			username = excluded.username,
			password_enc = excluded.password_enc,
			password_nonce = excluded.password_nonce,
			starttls = excluded.starttls,
			active = excluded.active,
			updated_at = CURRENT_TIMESTAMP`,
		domainID, method, host, port, username, passwordEnc, passwordNonce, starttls, active)
	if err != nil {
		return fmt.Errorf("upserting relay config for domain %d method %s: %w", domainID, method, err)
	}

	return tx.Commit()
}

// ActivateRelayConfig sets a specific method as active and deactivates all others for the domain.
func (db *DB) ActivateRelayConfig(domainID int64, method string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Deactivate all methods for this domain
	if _, err := tx.Exec(`UPDATE relay_configs SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ?`, domainID); err != nil {
		return fmt.Errorf("deactivating relay configs: %w", err)
	}

	// Activate the specified method
	result, err := tx.Exec(`UPDATE relay_configs SET active = 1, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ? AND method = ?`, domainID, method)
	if err != nil {
		return fmt.Errorf("activating relay config: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no relay config found for domain %d method %s", domainID, method)
	}

	return tx.Commit()
}

// DeactivateRelayConfig sets a specific method to inactive.
func (db *DB) DeactivateRelayConfig(domainID int64, method string) error {
	result, err := db.Exec(`UPDATE relay_configs SET active = 0, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ? AND method = ?`, domainID, method)
	if err != nil {
		return fmt.Errorf("deactivating relay config: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no relay config found for domain %d method %s", domainID, method)
	}
	return nil
}

// UpdateRelayAuthMethod updates the auth_method for a domain's relay config matching a specific method.
// If method is empty, it updates the active config.
func (db *DB) UpdateRelayAuthMethod(domainID int64, authMethod string) error {
	result, err := db.Exec(`UPDATE relay_configs SET auth_method = ?, updated_at = CURRENT_TIMESTAMP WHERE domain_id = ? AND active = 1`,
		authMethod, domainID)
	if err != nil {
		return fmt.Errorf("updating auth_method for domain %d: %w", domainID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("no active relay config found for domain %d", domainID)
	}
	return nil
}

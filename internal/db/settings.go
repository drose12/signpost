package db

import (
	"database/sql"
	"fmt"
)

// GetSetting retrieves a setting value by key. Returns empty string if not found.
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("getting setting %q: %w", key, err)
	}
	return value, nil
}

// SetSetting creates or updates a setting.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	if err != nil {
		return fmt.Errorf("setting %q: %w", key, err)
	}
	return nil
}

// GetAllSettings returns all settings as a map.
func (db *DB) GetAllSettings() (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("listing settings: %w", err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scanning setting: %w", err)
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

// GetTLSConfig returns the singleton TLS configuration.
func (db *DB) GetTLSConfig() (*TLSConfig, error) {
	var tc TLSConfig
	err := db.QueryRow(`SELECT mode, acme_email, acme_provider, cert_path,
		key_path, cert_expiry, updated_at FROM tls_config WHERE id = 1`).Scan(
		&tc.Mode, &tc.ACMEEmail, &tc.ACMEProvider, &tc.CertPath,
		&tc.KeyPath, &tc.CertExpiry, &tc.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting TLS config: %w", err)
	}
	return &tc, nil
}

// UpdateTLSConfig updates the TLS configuration.
func (db *DB) UpdateTLSConfig(mode string, acmeEmail, acmeProvider, certPath, keyPath *string) error {
	_, err := db.Exec(`UPDATE tls_config SET mode = ?, acme_email = ?, acme_provider = ?,
		cert_path = ?, key_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		mode, acmeEmail, acmeProvider, certPath, keyPath)
	if err != nil {
		return fmt.Errorf("updating TLS config: %w", err)
	}
	return nil
}

// UpdateTLSCertPaths updates only the cert and key paths in the TLS configuration.
// This is used by the self-signed cert generator at startup.
func (db *DB) UpdateTLSCertPaths(certPath, keyPath string) error {
	_, err := db.Exec(`UPDATE tls_config SET cert_path = ?, key_path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
		certPath, keyPath)
	if err != nil {
		return fmt.Errorf("updating TLS cert paths: %w", err)
	}
	return nil
}

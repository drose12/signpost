package db

import (
	"database/sql"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword generates a bcrypt hash from a plaintext password.
// Maddy's auth.pass_table expects the {bcrypt} tag prefix.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hashing password: %w", err)
	}
	return "bcrypt:" + string(hash), nil
}

// ListSMTPUsers returns all SMTP users.
func (db *DB) ListSMTPUsers() ([]SMTPUser, error) {
	rows, err := db.Query(`SELECT id, username, password_hash, password_enc, password_nonce, active, created_at, updated_at
		FROM smtp_users ORDER BY username`)
	if err != nil {
		return nil, fmt.Errorf("listing SMTP users: %w", err)
	}
	defer rows.Close()

	var users []SMTPUser
	for rows.Next() {
		var u SMTPUser
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.PasswordEnc, &u.PasswordNonce, &u.Active, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning SMTP user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetSMTPUser returns a single SMTP user by ID.
func (db *DB) GetSMTPUser(id int64) (*SMTPUser, error) {
	var u SMTPUser
	err := db.QueryRow(`SELECT id, username, password_hash, password_enc, password_nonce, active, created_at, updated_at
		FROM smtp_users WHERE id = ?`, id).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.PasswordEnc, &u.PasswordNonce, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting SMTP user %d: %w", id, err)
	}
	return &u, nil
}

// CreateSMTPUser creates a new SMTP user with a bcrypt-hashed password and optional encrypted password for display.
func (db *DB) CreateSMTPUser(username, passwordHash string, passwordEnc, passwordNonce *string) (*SMTPUser, error) {
	result, err := db.Exec(`INSERT INTO smtp_users (username, password_hash, password_enc, password_nonce) VALUES (?, ?, ?, ?)`,
		username, passwordHash, passwordEnc, passwordNonce)
	if err != nil {
		return nil, fmt.Errorf("creating SMTP user %q: %w", username, err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("getting SMTP user ID: %w", err)
	}
	return db.GetSMTPUser(id)
}

// DeleteSMTPUser removes an SMTP user by ID.
func (db *DB) DeleteSMTPUser(id int64) error {
	result, err := db.Exec(`DELETE FROM smtp_users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting SMTP user %d: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking delete result: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("SMTP user %d not found", id)
	}
	return nil
}

// ToggleSMTPUserActive toggles the active state of an SMTP user.
func (db *DB) ToggleSMTPUserActive(id int64, active bool) error {
	now := time.Now()
	result, err := db.Exec(`UPDATE smtp_users SET active = ?, updated_at = ? WHERE id = ?`,
		active, now, id)
	if err != nil {
		return fmt.Errorf("toggling SMTP user %d active: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update result: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("SMTP user %d not found", id)
	}
	return nil
}

// UpdateSMTPUserPassword updates the password hash and encrypted password for an SMTP user.
func (db *DB) UpdateSMTPUserPassword(id int64, passwordHash string, passwordEnc, passwordNonce *string) error {
	now := time.Now()
	result, err := db.Exec(`UPDATE smtp_users SET password_hash = ?, password_enc = ?, password_nonce = ?, updated_at = ? WHERE id = ?`,
		passwordHash, passwordEnc, passwordNonce, now, id)
	if err != nil {
		return fmt.Errorf("updating SMTP user %d password: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking update result: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("SMTP user %d not found", id)
	}
	return nil
}

// CountSMTPUsers returns the number of active SMTP users.
func (db *DB) CountSMTPUsers() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM smtp_users WHERE active = 1`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting SMTP users: %w", err)
	}
	return count, nil
}

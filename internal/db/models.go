package db

import "time"

// Domain represents a configured email domain.
type Domain struct {
	ID             int64      `json:"id"`
	Name           string     `json:"name"`
	DKIMSelector   string     `json:"dkim_selector"`
	DKIMKeyPath    *string    `json:"dkim_key_path,omitempty"`
	DKIMPublicDNS  *string    `json:"dkim_public_dns,omitempty"`
	DKIMCreatedAt  *time.Time `json:"dkim_created_at,omitempty"`
	SPFRecord      *string    `json:"spf_record,omitempty"`
	DMARCRecord    *string    `json:"dmarc_record,omitempty"`
	Active         bool       `json:"active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// RelayConfig holds the outbound relay settings for a domain.
type RelayConfig struct {
	ID            int64     `json:"id"`
	DomainID      int64     `json:"domain_id"`
	Method        string    `json:"method"` // gmail, isp, direct, custom
	Host          *string   `json:"host,omitempty"`
	Port          int       `json:"port"`
	Username      *string   `json:"username,omitempty"`
	PasswordEnc   *string   `json:"-"` // never exposed in JSON
	PasswordNonce *string   `json:"-"`
	StartTLS       bool      `json:"starttls"`
	AuthMethod     string    `json:"auth_method"` // plain (default, Maddy relay) or login (Go direct relay)
	Active         bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// SMTPUser represents a user authorized to send via port 587.
type SMTPUser struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	PasswordHash  string    `json:"-"`              // bcrypt hash for Maddy auth
	PasswordEnc   *string   `json:"-"`              // AES-GCM encrypted for display
	PasswordNonce *string   `json:"-"`              // nonce for decryption
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TLSConfig holds the TLS/certificate configuration.
type TLSConfig struct {
	Mode         string     `json:"mode"` // acme, manual, self-signed
	ACMEEmail    *string    `json:"acme_email,omitempty"`
	ACMEProvider *string    `json:"acme_provider,omitempty"`
	CertPath     *string    `json:"cert_path,omitempty"`
	KeyPath      *string    `json:"key_path,omitempty"`
	CertExpiry   *time.Time `json:"cert_expiry,omitempty"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// MailLogEntry represents a single mail send event.
type MailLogEntry struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	FromAddr   string    `json:"from_addr"`
	ToAddr     string    `json:"to_addr"`
	DomainID   *int64    `json:"domain_id,omitempty"`
	Subject    *string   `json:"subject,omitempty"`
	Status     string    `json:"status"` // sent, failed, deferred
	RelayHost  *string   `json:"relay_host,omitempty"`
	Error      *string   `json:"error,omitempty"`
	DKIMSigned bool      `json:"dkim_signed"`
}

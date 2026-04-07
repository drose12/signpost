package db

// migrations is an ordered list of SQL migrations.
// Each migration is applied exactly once, tracked by the schema_migrations table.
// Migrations are forward-only — no rollbacks.
var migrations = []string{
	// Migration 1: Initial schema
	`CREATE TABLE domains (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		name          TEXT NOT NULL UNIQUE,
		dkim_selector TEXT NOT NULL DEFAULT 'signpost',
		dkim_key_path TEXT,
		dkim_public_dns TEXT,
		dkim_created_at DATETIME,
		spf_record    TEXT,
		dmarc_record  TEXT,
		active        BOOLEAN NOT NULL DEFAULT 1,
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE relay_configs (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_id      INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
		method         TEXT NOT NULL DEFAULT 'direct',
		host           TEXT,
		port           INTEGER DEFAULT 587,
		username       TEXT,
		password_enc   TEXT,
		password_nonce TEXT,
		starttls       BOOLEAN NOT NULL DEFAULT 1,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(domain_id)
	);

	CREATE TABLE smtp_users (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		username      TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		active        BOOLEAN NOT NULL DEFAULT 1,
		created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE smtp_user_domains (
		smtp_user_id INTEGER NOT NULL REFERENCES smtp_users(id) ON DELETE CASCADE,
		domain_id    INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
		PRIMARY KEY (smtp_user_id, domain_id)
	);

	CREATE TABLE tls_config (
		id            INTEGER PRIMARY KEY CHECK (id = 1),
		mode          TEXT NOT NULL DEFAULT 'self-signed',
		acme_email    TEXT,
		acme_provider TEXT DEFAULT 'cloudflare',
		cert_path     TEXT,
		key_path      TEXT,
		cert_expiry   DATETIME,
		updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE settings (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE TABLE mail_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		from_addr   TEXT NOT NULL,
		to_addr     TEXT NOT NULL,
		domain_id   INTEGER REFERENCES domains(id),
		subject     TEXT,
		status      TEXT NOT NULL,
		relay_host  TEXT,
		error       TEXT,
		dkim_signed BOOLEAN DEFAULT 0
	);

	CREATE TABLE schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Default settings
	INSERT INTO settings (key, value) VALUES ('network_trust_enabled', 'true');
	INSERT INTO settings (key, value) VALUES ('network_trust_cidrs', '172.16.0.0/12,127.0.0.1/8');
	INSERT INTO settings (key, value) VALUES ('smtp_port', '25');
	INSERT INTO settings (key, value) VALUES ('submission_port', '587');
	INSERT INTO settings (key, value) VALUES ('web_port', '8080');
	INSERT INTO settings (key, value) VALUES ('log_retention_days', '30');

	-- Default TLS config (self-signed)
	INSERT INTO tls_config (id, mode) VALUES (1, 'self-signed');

	-- Indexes for common queries
	CREATE INDEX idx_mail_log_timestamp ON mail_log(timestamp);
	CREATE INDEX idx_mail_log_status ON mail_log(status);
	CREATE INDEX idx_mail_log_domain_id ON mail_log(domain_id);
	CREATE INDEX idx_domains_name ON domains(name);`,

	// Migration 2: Add auth_method column to relay_configs
	`ALTER TABLE relay_configs ADD COLUMN auth_method TEXT NOT NULL DEFAULT 'plain';`,

	// Migration 3: Add port enable/disable settings
	`INSERT OR IGNORE INTO settings (key, value) VALUES ('smtp_enabled', 'true');
	 INSERT OR IGNORE INTO settings (key, value) VALUES ('submission_enabled', 'false');`,

	// Migration 4: Add encrypted password columns to smtp_users for display
	`ALTER TABLE smtp_users ADD COLUMN password_enc TEXT;
	 ALTER TABLE smtp_users ADD COLUMN password_nonce TEXT;`,

	// Migration 5: Add egress host setting for direct delivery SPF
	`INSERT OR IGNORE INTO settings (key, value) VALUES ('egress_host', '');`,

	// Migration 6: Support multiple relay configs per domain (one per method).
	// Drops UNIQUE(domain_id) in favor of UNIQUE(domain_id, method).
	// Adds 'active' column so only one method is active at a time per domain.
	`CREATE TABLE relay_configs_new (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_id      INTEGER NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
		method         TEXT NOT NULL DEFAULT 'direct',
		host           TEXT,
		port           INTEGER DEFAULT 587,
		username       TEXT,
		password_enc   TEXT,
		password_nonce TEXT,
		starttls       BOOLEAN NOT NULL DEFAULT 1,
		auth_method    TEXT NOT NULL DEFAULT 'plain',
		active         BOOLEAN NOT NULL DEFAULT 0,
		created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(domain_id, method)
	);

	INSERT INTO relay_configs_new (id, domain_id, method, host, port, username, password_enc, password_nonce, starttls, auth_method, active, created_at, updated_at)
	SELECT id, domain_id, method, host, port, username, password_enc, password_nonce, starttls, auth_method, 1, created_at, updated_at FROM relay_configs;

	DROP TABLE relay_configs;
	ALTER TABLE relay_configs_new RENAME TO relay_configs;`,

	// Migration 7: schema version bump (columns added in migration 8).
	// Originally attempted multi-statement ALTER TABLEs here, but go-sqlite3
	// silently ignores all but the first statement. Replaced with no-op;
	// the real work happens in migration 8.
	`SELECT 1;`,

	// Migration 8: Add fields for Maddy log tailer and enhanced mail tracking.
	// Each ALTER TABLE must be a separate Exec call, so we use a table rebuild approach.
	`CREATE TABLE mail_log_new (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		from_addr     TEXT NOT NULL,
		to_addr       TEXT NOT NULL,
		domain_id     INTEGER REFERENCES domains(id),
		subject       TEXT,
		status        TEXT NOT NULL,
		relay_host    TEXT,
		error         TEXT,
		dkim_signed   BOOLEAN DEFAULT 0,
		msg_id        TEXT,
		source_ip     TEXT,
		source_port   TEXT,
		attempt_count INTEGER DEFAULT 0,
		direction     TEXT DEFAULT 'outbound'
	);
	INSERT INTO mail_log_new (id, timestamp, from_addr, to_addr, domain_id, subject, status, relay_host, error, dkim_signed)
		SELECT id, timestamp, from_addr, to_addr, domain_id, subject, status, relay_host, error, dkim_signed FROM mail_log;
	DROP TABLE mail_log;
	ALTER TABLE mail_log_new RENAME TO mail_log;
	CREATE INDEX idx_mail_log_timestamp ON mail_log(timestamp);
	CREATE INDEX idx_mail_log_status ON mail_log(status);
	CREATE INDEX idx_mail_log_domain_id ON mail_log(domain_id);
	CREATE UNIQUE INDEX idx_mail_log_msg_id ON mail_log(msg_id);`,
}

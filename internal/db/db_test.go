package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// testDB creates a temporary database for testing and returns it with a cleanup function.
func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db file not created: %v", err)
	}

	// Verify schema version
	version, err := db.SchemaVersion()
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version != 2 {
		t.Errorf("expected schema version 2, got %d", version)
	}
}

func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Open twice to verify migrations don't re-apply
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer db2.Close()

	version, _ := db2.SchemaVersion()
	if version != 2 {
		t.Errorf("expected schema version 2 after reopening, got %d", version)
	}
}

func TestCheckIntegrity(t *testing.T) {
	db := testDB(t)
	if err := db.CheckIntegrity(); err != nil {
		t.Fatalf("CheckIntegrity: %v", err)
	}
}

func TestDefaultSettings(t *testing.T) {
	db := testDB(t)

	val, err := db.GetSetting("network_trust_enabled")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "true" {
		t.Errorf("expected network_trust_enabled='true', got %q", val)
	}

	val, err = db.GetSetting("network_trust_cidrs")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "172.16.0.0/12,127.0.0.1/8" {
		t.Errorf("unexpected CIDRs: %q", val)
	}
}

func TestDefaultTLSConfig(t *testing.T) {
	db := testDB(t)

	tc, err := db.GetTLSConfig()
	if err != nil {
		t.Fatalf("GetTLSConfig: %v", err)
	}
	if tc == nil {
		t.Fatal("expected default TLS config, got nil")
	}
	if tc.Mode != "self-signed" {
		t.Errorf("expected mode 'self-signed', got %q", tc.Mode)
	}
}

func TestDomainCRUD(t *testing.T) {
	db := testDB(t)

	// Create
	domain, err := db.CreateDomain("drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}
	if domain.Name != "drcs.ca" {
		t.Errorf("expected name 'drcs.ca', got %q", domain.Name)
	}
	if domain.DKIMSelector != "signpost" {
		t.Errorf("expected selector 'signpost', got %q", domain.DKIMSelector)
	}
	if !domain.Active {
		t.Error("expected domain to be active")
	}

	// Get by ID
	got, err := db.GetDomain(domain.ID)
	if err != nil {
		t.Fatalf("GetDomain: %v", err)
	}
	if got == nil || got.Name != "drcs.ca" {
		t.Errorf("GetDomain returned unexpected result: %+v", got)
	}

	// Get by name
	got, err = db.GetDomainByName("drcs.ca")
	if err != nil {
		t.Fatalf("GetDomainByName: %v", err)
	}
	if got == nil || got.ID != domain.ID {
		t.Errorf("GetDomainByName returned unexpected result: %+v", got)
	}

	// List
	domains, err := db.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 1 {
		t.Errorf("expected 1 domain, got %d", len(domains))
	}

	// Update DKIM
	err = db.UpdateDomainDKIM(domain.ID, "/data/signpost/dkim_keys/drcs.ca.key", "v=DKIM1; k=rsa; p=MIIBIjAN...")
	if err != nil {
		t.Fatalf("UpdateDomainDKIM: %v", err)
	}
	got, _ = db.GetDomain(domain.ID)
	if got.DKIMKeyPath == nil || *got.DKIMKeyPath != "/data/signpost/dkim_keys/drcs.ca.key" {
		t.Error("DKIM key path not updated")
	}

	// Update DNS records
	err = db.UpdateDomainDNSRecords(domain.ID, "v=spf1 mx ~all", "v=DMARC1; p=quarantine")
	if err != nil {
		t.Fatalf("UpdateDomainDNSRecords: %v", err)
	}

	// Delete
	err = db.DeleteDomain(domain.ID)
	if err != nil {
		t.Fatalf("DeleteDomain: %v", err)
	}
	got, _ = db.GetDomain(domain.ID)
	if got != nil {
		t.Error("expected domain to be deleted")
	}
}

func TestDuplicateDomain(t *testing.T) {
	db := testDB(t)

	_, err := db.CreateDomain("drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("first CreateDomain: %v", err)
	}

	_, err = db.CreateDomain("drcs.ca", "signpost")
	if err == nil {
		t.Error("expected error creating duplicate domain")
	}
}

func TestDeleteNonexistentDomain(t *testing.T) {
	db := testDB(t)
	err := db.DeleteDomain(999)
	if err == nil {
		t.Error("expected error deleting nonexistent domain")
	}
}

func TestRelayConfig(t *testing.T) {
	db := testDB(t)

	domain, err := db.CreateDomain("drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}

	// No relay config initially
	rc, err := db.GetRelayConfig(domain.ID)
	if err != nil {
		t.Fatalf("GetRelayConfig: %v", err)
	}
	if rc != nil {
		t.Error("expected no relay config initially")
	}

	// Create relay config
	host := "smtp.gmail.com"
	username := "user@drcs.ca"
	passEnc := "encrypted-password"
	passNonce := "nonce-value"
	err = db.UpsertRelayConfig(domain.ID, "gmail", &host, 587, &username, &passEnc, &passNonce, true)
	if err != nil {
		t.Fatalf("UpsertRelayConfig: %v", err)
	}

	rc, err = db.GetRelayConfig(domain.ID)
	if err != nil {
		t.Fatalf("GetRelayConfig after upsert: %v", err)
	}
	if rc == nil {
		t.Fatal("expected relay config after upsert")
	}
	if rc.Method != "gmail" {
		t.Errorf("expected method 'gmail', got %q", rc.Method)
	}
	if rc.Host == nil || *rc.Host != "smtp.gmail.com" {
		t.Error("unexpected host")
	}

	// Update relay config
	newHost := "smtp.bellmts.net"
	err = db.UpsertRelayConfig(domain.ID, "isp", &newHost, 25, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("UpsertRelayConfig update: %v", err)
	}

	rc, _ = db.GetRelayConfig(domain.ID)
	if rc.Method != "isp" {
		t.Errorf("expected method 'isp' after update, got %q", rc.Method)
	}
}

func TestRelayConfigCascadeDelete(t *testing.T) {
	db := testDB(t)

	domain, _ := db.CreateDomain("drcs.ca", "signpost")
	host := "smtp.gmail.com"
	db.UpsertRelayConfig(domain.ID, "gmail", &host, 587, nil, nil, nil, true)

	// Deleting the domain should cascade to relay config
	db.DeleteDomain(domain.ID)

	rc, err := db.GetRelayConfig(domain.ID)
	if err != nil {
		t.Fatalf("GetRelayConfig after cascade: %v", err)
	}
	if rc != nil {
		t.Error("expected relay config to be cascade deleted")
	}
}

func TestSettings(t *testing.T) {
	db := testDB(t)

	// Get nonexistent setting
	val, err := db.GetSetting("nonexistent")
	if err != nil {
		t.Fatalf("GetSetting: %v", err)
	}
	if val != "" {
		t.Errorf("expected empty string for nonexistent setting, got %q", val)
	}

	// Set and get
	err = db.SetSetting("test_key", "test_value")
	if err != nil {
		t.Fatalf("SetSetting: %v", err)
	}
	val, _ = db.GetSetting("test_key")
	if val != "test_value" {
		t.Errorf("expected 'test_value', got %q", val)
	}

	// Update existing
	err = db.SetSetting("test_key", "new_value")
	if err != nil {
		t.Fatalf("SetSetting update: %v", err)
	}
	val, _ = db.GetSetting("test_key")
	if val != "new_value" {
		t.Errorf("expected 'new_value', got %q", val)
	}

	// Get all
	all, err := db.GetAllSettings()
	if err != nil {
		t.Fatalf("GetAllSettings: %v", err)
	}
	if len(all) < 7 { // 6 defaults + 1 test
		t.Errorf("expected at least 7 settings, got %d", len(all))
	}
}

func TestMailLog(t *testing.T) {
	db := testDB(t)

	domain, _ := db.CreateDomain("drcs.ca", "signpost")

	// Log some mail
	err := db.LogMail("sender@drcs.ca", "recipient@gmail.com", &domain.ID, "Test Subject", "sent", nil, nil, true)
	if err != nil {
		t.Fatalf("LogMail: %v", err)
	}

	errMsg := "connection refused"
	err = db.LogMail("sender@drcs.ca", "other@gmail.com", &domain.ID, "Failed", "failed", nil, &errMsg, false)
	if err != nil {
		t.Fatalf("LogMail failed: %v", err)
	}

	// List all
	entries, err := db.ListMailLog(MailLogFilter{Limit: 10})
	if err != nil {
		t.Fatalf("ListMailLog: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(entries))
	}

	// Filter by status
	status := "sent"
	entries, err = db.ListMailLog(MailLogFilter{Status: &status, Limit: 10})
	if err != nil {
		t.Fatalf("ListMailLog filtered: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 'sent' entry, got %d", len(entries))
	}

	// Prune old entries (none should be pruned since they're fresh)
	pruned, err := db.PruneMailLog(24 * time.Hour)
	if err != nil {
		t.Fatalf("PruneMailLog: %v", err)
	}
	if pruned != 0 {
		t.Errorf("expected 0 pruned, got %d", pruned)
	}
}

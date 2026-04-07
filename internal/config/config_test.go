package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drose-drcs/signpost/internal/db"
)

func testSetup(t *testing.T) (*db.DB, *Generator) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Find template relative to the project root
	tmplPath := filepath.Join(dir, "maddy.conf.tmpl")
	// Copy the template to temp dir for testing
	tmplContent := `# Test Maddy Config
hostname {{.Hostname}}
smtp_port {{.SMTPPort}}
tls_mode {{.TLS.Mode}}
{{- range .Domains}}
domain {{.Name}} selector={{.DKIMSelector}} active={{.Active}}
{{- if .DKIMKeyPath}} dkim_key={{.DKIMKeyPath}}{{end}}
{{- if .Relay}} relay={{.Relay.Method}} host={{.Relay.Host}} port={{.Relay.Port}}{{end}}
{{- end}}
network_trust={{.NetworkTrustEnabled}} cidrs={{.NetworkTrustCIDRs}}
has_relay={{.HasRelayDomains}}
smtp_users={{.SMTPUsers}}`
	os.WriteFile(tmplPath, []byte(tmplContent), 0644)

	outputPath := filepath.Join(dir, "maddy.conf")
	gen := NewGenerator(tmplPath, outputPath, dir)

	os.Setenv("SIGNPOST_DOMAIN", "drcs.ca")
	os.Setenv("SIGNPOST_HOSTNAME", "mail.drcs.ca")
	t.Cleanup(func() {
		os.Unsetenv("SIGNPOST_DOMAIN")
		os.Unsetenv("SIGNPOST_HOSTNAME")
	})

	return database, gen
}

func noopDecrypt(enc, nonce string) (string, error) {
	return "decrypted-" + enc, nil
}

func TestGenerateEmptyDB(t *testing.T) {
	database, gen := testSetup(t)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(content, "hostname mail.drcs.ca") {
		t.Error("expected hostname in output")
	}
	if !strings.Contains(content, "tls_mode self-signed") {
		t.Error("expected self-signed TLS mode")
	}
	if !strings.Contains(content, "network_trust=true") {
		t.Error("expected network trust enabled")
	}
	if !strings.Contains(content, "has_relay=false") {
		t.Error("expected no relay domains")
	}
	if !strings.Contains(content, "smtp_users=false") {
		t.Error("expected no SMTP users")
	}
}

func TestGenerateWithDomain(t *testing.T) {
	database, gen := testSetup(t)

	_, err := database.CreateDomain("drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("CreateDomain: %v", err)
	}

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(content, "domain drcs.ca selector=signpost active=true") {
		t.Errorf("expected domain in output, got:\n%s", content)
	}
}

func TestGenerateWithDKIM(t *testing.T) {
	database, gen := testSetup(t)

	domain, _ := database.CreateDomain("drcs.ca", "signpost")
	database.UpdateDomainDKIM(domain.ID, "/data/signpost/dkim_keys/drcs.ca.key", "v=DKIM1; k=rsa; p=AAAA")

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(content, "dkim_key=/data/signpost/dkim_keys/drcs.ca.key") {
		t.Errorf("expected DKIM key path in output, got:\n%s", content)
	}
}

func TestGenerateWithRelay(t *testing.T) {
	database, gen := testSetup(t)

	domain, _ := database.CreateDomain("drcs.ca", "signpost")
	host := "smtp.gmail.com"
	user := "user@drcs.ca"
	passEnc := "encpass"
	passNonce := "nonce"
	database.UpsertRelayConfig(domain.ID, "gmail", &host, 587, &user, &passEnc, &passNonce, true, true)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if !strings.Contains(content, "relay=gmail host=smtp.gmail.com port=587") {
		t.Errorf("expected relay config in output, got:\n%s", content)
	}
	if !strings.Contains(content, "has_relay=true") {
		t.Error("expected has_relay=true")
	}
}

func TestGenerateDirectDelivery(t *testing.T) {
	database, gen := testSetup(t)

	domain, _ := database.CreateDomain("drcs.ca", "signpost")
	database.UpsertRelayConfig(domain.ID, "direct", nil, 25, nil, nil, nil, false, true)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Direct delivery should not show up as a relay
	if strings.Contains(content, "relay=direct") {
		t.Error("direct delivery should not appear as relay in template")
	}
	if !strings.Contains(content, "has_relay=false") {
		t.Error("expected has_relay=false for direct delivery")
	}
}

func TestWriteConfigCreatesFile(t *testing.T) {
	database, gen := testSetup(t)

	err := gen.WriteConfig(database, noopDecrypt)
	if err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	if _, err := os.Stat(gen.outputPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	content, _ := os.ReadFile(gen.outputPath)
	if !strings.Contains(string(content), "hostname mail.drcs.ca") {
		t.Error("config file has wrong content")
	}
}

func TestWriteConfigCreatesBackup(t *testing.T) {
	database, gen := testSetup(t)

	// Write initial config
	gen.WriteConfig(database, noopDecrypt)

	// Write again — should create .bak
	gen.WriteConfig(database, noopDecrypt)

	backupPath := gen.outputPath + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatal("backup file not created")
	}
}

func TestRollbackConfig(t *testing.T) {
	database, gen := testSetup(t)

	// Write initial config
	gen.WriteConfig(database, noopDecrypt)
	original, _ := os.ReadFile(gen.outputPath)

	// Create a domain and regenerate
	database.CreateDomain("drcs.ca", "signpost")
	gen.WriteConfig(database, noopDecrypt)

	// Rollback
	err := gen.RollbackConfig()
	if err != nil {
		t.Fatalf("RollbackConfig: %v", err)
	}

	restored, _ := os.ReadFile(gen.outputPath)
	if string(restored) != string(original) {
		t.Error("rollback did not restore original config")
	}
}

func TestFormatNetworkCIDRs(t *testing.T) {
	result := FormatNetworkCIDRs("172.16.0.0/12, 127.0.0.1/8")
	if result != "172.16.0.0/12 127.0.0.1/8" {
		t.Errorf("unexpected format: %q", result)
	}
}

// realTemplateSetup creates a generator that uses the actual maddy.conf.tmpl
func realTemplateSetup(t *testing.T) (*db.DB, *Generator) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Use the real template from project root
	tmplPath := filepath.Join("..", "..", "templates", "maddy.conf.tmpl")
	outputPath := filepath.Join(dir, "maddy.conf")
	gen := NewGenerator(tmplPath, outputPath, dir)

	os.Setenv("SIGNPOST_DOMAIN", "drcs.ca")
	os.Setenv("SIGNPOST_HOSTNAME", "mail.drcs.ca")
	t.Cleanup(func() {
		os.Unsetenv("SIGNPOST_DOMAIN")
		os.Unsetenv("SIGNPOST_HOSTNAME")
	})

	return database, gen
}

func TestRealTemplateEmptyDB(t *testing.T) {
	database, gen := realTemplateSetup(t)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate with real template: %v", err)
	}

	// Check global settings
	if !strings.Contains(content, "hostname mail.drcs.ca") {
		t.Error("expected hostname")
	}
	if !strings.Contains(content, "tls off") {
		t.Errorf("expected 'tls off' for self-signed mode, got:\n%s", content)
	}
	// Should have smtp and submission endpoints
	if !strings.Contains(content, "smtp tcp://0.0.0.0:25") {
		t.Error("expected smtp endpoint")
	}
	// Submission endpoint only appears when SMTP users are configured
	if strings.Contains(content, "submission tcp://0.0.0.0:587") {
		t.Error("submission endpoint should not appear without SMTP users")
	}
	// Should reject unknown senders
	if !strings.Contains(content, "Sender domain not configured") {
		t.Error("expected default rejection for unconfigured domains")
	}
}

func TestRealTemplateWithDomainAndDKIM(t *testing.T) {
	database, gen := realTemplateSetup(t)

	domain, _ := database.CreateDomain("drcs.ca", "signpost")
	database.UpdateDomainDKIM(domain.ID, "/data/signpost/dkim_keys/drcs.ca_signpost.key", "v=DKIM1; k=rsa; p=AAAA")

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should have inline DKIM config in modify block
	if !strings.Contains(content, "domains drcs.ca") {
		t.Error("expected domain in DKIM config")
	}
	if !strings.Contains(content, "selector signpost") {
		t.Error("expected selector in DKIM config")
	}
	if !strings.Contains(content, "key_path /data/signpost/dkim_keys/drcs.ca_signpost.key") {
		t.Error("expected key_path in DKIM config")
	}
	// Should have source routing for the domain
	if !strings.Contains(content, "source drcs.ca") {
		t.Error("expected source routing for domain")
	}
	// Direct delivery (no relay configured)
	if !strings.Contains(content, "deliver_to &remote_queue") {
		t.Error("expected direct delivery via remote_queue")
	}
}

func TestRealTemplateWithRelay(t *testing.T) {
	database, gen := realTemplateSetup(t)

	domain, _ := database.CreateDomain("drcs.ca", "signpost")
	database.UpdateDomainDKIM(domain.ID, "/data/signpost/dkim_keys/drcs.ca_signpost.key", "v=DKIM1; k=rsa; p=AAAA")

	host := "smtp.gmail.com"
	user := "user@drcs.ca"
	passEnc := "encpass"
	passNonce := "nonce"
	database.UpsertRelayConfig(domain.ID, "gmail", &host, 587, &user, &passEnc, &passNonce, true, true)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Should have relay SMTP target (renamed to relay_target_0)
	if !strings.Contains(content, "target.smtp relay_target_0") {
		t.Errorf("expected relay SMTP target definition, got:\n%s", content)
	}
	if !strings.Contains(content, "targets tcp://smtp.gmail.com:587") {
		t.Error("expected relay targets directive")
	}
	if !strings.Contains(content, "starttls yes") {
		t.Error("expected starttls in relay config")
	}
	if !strings.Contains(content, `auth plain "user@drcs.ca" "decrypted-encpass"`) {
		t.Error("expected auth credentials in relay config")
	}
	// Should have queue wrapper around relay target
	if !strings.Contains(content, "target.queue relay_0") {
		t.Errorf("expected relay queue wrapper, got:\n%s", content)
	}
	// Pipeline should deliver to relay queue and have inline DKIM
	if !strings.Contains(content, "deliver_to &relay_0") {
		t.Error("expected deliver_to relay reference in pipeline")
	}
	if !strings.Contains(content, "selector signpost") {
		t.Error("expected inline DKIM selector")
	}
}

func TestRealTemplateMultipleDomains(t *testing.T) {
	database, gen := realTemplateSetup(t)

	d1, _ := database.CreateDomain("drcs.ca", "signpost")
	database.UpdateDomainDKIM(d1.ID, "/data/signpost/dkim_keys/drcs.ca_signpost.key", "v=DKIM1; k=rsa; p=AAAA")

	d2, _ := database.CreateDomain("example.com", "mail")
	database.UpdateDomainDKIM(d2.ID, "/data/signpost/dkim_keys/example.com_mail.key", "v=DKIM1; k=rsa; p=BBBB")

	// Relay for first domain, direct for second
	host := "smtp.gmail.com"
	user := "user@drcs.ca"
	passEnc := "encpass"
	passNonce := "nonce"
	database.UpsertRelayConfig(d1.ID, "gmail", &host, 587, &user, &passEnc, &passNonce, true, true)

	content, err := gen.Generate(database, noopDecrypt)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Both domains should have inline DKIM config
	if !strings.Contains(content, "selector signpost") {
		t.Error("expected first DKIM selector")
	}
	if !strings.Contains(content, "selector mail") {
		t.Error("expected second DKIM selector")
	}
	// Both source routes
	if !strings.Contains(content, "source drcs.ca") {
		t.Error("expected source routing for drcs.ca")
	}
	if !strings.Contains(content, "source example.com") {
		t.Error("expected source routing for example.com")
	}
	// First domain relays, second delivers directly
	if !strings.Contains(content, "deliver_to &relay_0") {
		t.Error("expected relay delivery for first domain")
	}
	if !strings.Contains(content, "deliver_to &remote_queue") {
		t.Error("expected direct delivery for second domain")
	}
}

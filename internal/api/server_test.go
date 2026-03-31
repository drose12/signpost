package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/drose-drcs/signpost/internal/config"
	"github.com/drose-drcs/signpost/internal/db"
)

func testServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	gen := config.NewGenerator(
		filepath.Join(dir, "maddy.conf.tmpl"),
		filepath.Join(dir, "maddy.conf"),
		dir,
	)

	keysDir := filepath.Join(dir, "dkim_keys")
	srv := NewServer(database, gen, keysDir, "admin", "testpass", "test-secret-key-minimum-32-characters-long", nil)
	return srv, database
}

func doRequest(t *testing.T, srv *Server, method, path string, body interface{}, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}

	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if auth {
		req.SetBasicAuth("admin", "testpass")
	}

	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	return rr
}

func TestHealthz(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "GET", "/api/v1/healthz", nil, false)
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Errorf("expected healthy, got %q", resp["status"])
	}
}

func TestHealthzNoAuth(t *testing.T) {
	srv, _ := testServer(t)

	// Health check should work without auth
	rr := doRequest(t, srv, "GET", "/api/v1/healthz", nil, false)
	if rr.Code != http.StatusOK {
		t.Errorf("healthz should not require auth, got %d", rr.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "GET", "/api/v1/domains", nil, false)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestWrongCredentials(t *testing.T) {
	srv, _ := testServer(t)

	req := httptest.NewRequest("GET", "/api/v1/domains", nil)
	req.SetBasicAuth("admin", "wrongpass")
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong password, got %d", rr.Code)
	}
}

func TestDomainCRUD(t *testing.T) {
	srv, _ := testServer(t)

	// List — empty initially
	rr := doRequest(t, srv, "GET", "/api/v1/domains", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("list domains: %d %s", rr.Code, rr.Body.String())
	}
	var domains []db.Domain
	json.Unmarshal(rr.Body.Bytes(), &domains)
	if len(domains) != 0 {
		t.Errorf("expected 0 domains, got %d", len(domains))
	}

	// Create
	rr = doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{
		"name": "drcs.ca",
	}, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create domain: %d %s", rr.Code, rr.Body.String())
	}
	var created db.Domain
	json.Unmarshal(rr.Body.Bytes(), &created)
	if created.Name != "drcs.ca" {
		t.Errorf("expected name 'drcs.ca', got %q", created.Name)
	}
	if created.DKIMSelector != "signpost" {
		t.Errorf("expected default selector 'signpost', got %q", created.DKIMSelector)
	}

	// Get by ID
	rr = doRequest(t, srv, "GET", "/api/v1/domains/1", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("get domain: %d %s", rr.Code, rr.Body.String())
	}

	// Create duplicate — should fail
	rr = doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{
		"name": "drcs.ca",
	}, true)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", rr.Code)
	}

	// Delete
	rr = doRequest(t, srv, "DELETE", "/api/v1/domains/1", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete domain: %d %s", rr.Code, rr.Body.String())
	}

	// Get deleted — should 404
	rr = doRequest(t, srv, "GET", "/api/v1/domains/1", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for deleted domain, got %d", rr.Code)
	}
}

func TestCreateDomainValidation(t *testing.T) {
	srv, _ := testServer(t)

	// Missing name
	rr := doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing name, got %d", rr.Code)
	}
}

func TestGenerateDKIM(t *testing.T) {
	srv, _ := testServer(t)

	// Create a domain first
	doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{"name": "drcs.ca"}, true)

	// Generate DKIM key
	rr := doRequest(t, srv, "POST", "/api/v1/domains/1/dkim/generate", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("generate DKIM: %d %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)

	if resp["dns_record_name"] != "signpost._domainkey.drcs.ca" {
		t.Errorf("unexpected DNS record name: %q", resp["dns_record_name"])
	}
	if resp["selector"] != "signpost" {
		t.Errorf("unexpected selector: %q", resp["selector"])
	}
}

func TestGenerateDKIMNotFound(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "POST", "/api/v1/domains/999/dkim/generate", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGetDNSRecords(t *testing.T) {
	srv, _ := testServer(t)

	doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{"name": "drcs.ca"}, true)

	rr := doRequest(t, srv, "GET", "/api/v1/domains/1/dns", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("get DNS records: %d %s", rr.Code, rr.Body.String())
	}

	var records []map[string]string
	json.Unmarshal(rr.Body.Bytes(), &records)

	// Should have SPF and DMARC at minimum (no DKIM until key is generated)
	if len(records) < 2 {
		t.Errorf("expected at least 2 DNS records, got %d", len(records))
	}
}

func TestGetDNSRecordsWithDKIM(t *testing.T) {
	srv, _ := testServer(t)

	doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{"name": "drcs.ca"}, true)
	doRequest(t, srv, "POST", "/api/v1/domains/1/dkim/generate", nil, true)

	rr := doRequest(t, srv, "GET", "/api/v1/domains/1/dns", nil, true)
	var records []map[string]string
	json.Unmarshal(rr.Body.Bytes(), &records)

	// Should have SPF, DMARC, and DKIM
	if len(records) != 3 {
		t.Errorf("expected 3 DNS records (SPF + DMARC + DKIM), got %d", len(records))
	}
}

func TestRelayConfig(t *testing.T) {
	srv, _ := testServer(t)

	doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{"name": "drcs.ca"}, true)

	// Get — should return direct by default
	rr := doRequest(t, srv, "GET", "/api/v1/domains/1/relay", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("get relay: %d %s", rr.Code, rr.Body.String())
	}

	// Update
	host := "smtp.gmail.com"
	rr = doRequest(t, srv, "PUT", "/api/v1/domains/1/relay", map[string]interface{}{
		"method":   "gmail",
		"host":     host,
		"port":     587,
		"starttls": true,
	}, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("update relay: %d %s", rr.Code, rr.Body.String())
	}

	// Verify update
	rr = doRequest(t, srv, "GET", "/api/v1/domains/1/relay", nil, true)
	var rc db.RelayConfig
	json.Unmarshal(rr.Body.Bytes(), &rc)
	if rc.Method != "gmail" {
		t.Errorf("expected method 'gmail', got %q", rc.Method)
	}
}

func TestSettings(t *testing.T) {
	srv, _ := testServer(t)

	// Get all
	rr := doRequest(t, srv, "GET", "/api/v1/settings", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("get settings: %d %s", rr.Code, rr.Body.String())
	}

	// Update
	rr = doRequest(t, srv, "PUT", "/api/v1/settings", map[string]string{
		"network_trust_cidrs": "10.0.0.0/8",
	}, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("update settings: %d %s", rr.Code, rr.Body.String())
	}
}

func TestGetLogs(t *testing.T) {
	srv, database := testServer(t)

	// Log some entries
	database.LogMail("test@drcs.ca", "dest@gmail.com", nil, "Test", "sent", nil, nil, true)

	rr := doRequest(t, srv, "GET", "/api/v1/logs", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("get logs: %d %s", rr.Code, rr.Body.String())
	}

	var entries []db.MailLogEntry
	json.Unmarshal(rr.Body.Bytes(), &entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 log entry, got %d", len(entries))
	}
}

func TestGetLogsWithFilter(t *testing.T) {
	srv, database := testServer(t)

	database.LogMail("a@drcs.ca", "b@gmail.com", nil, "Test1", "sent", nil, nil, true)
	errMsg := "refused"
	database.LogMail("c@drcs.ca", "d@gmail.com", nil, "Test2", "failed", nil, &errMsg, false)

	rr := doRequest(t, srv, "GET", "/api/v1/logs?status=sent&limit=10", nil, true)
	var entries []db.MailLogEntry
	json.Unmarshal(rr.Body.Bytes(), &entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 'sent' entry, got %d", len(entries))
	}
}

func TestStatus(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "GET", "/api/v1/status", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: %d %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp["schema_version"].(float64) != 3 {
		t.Errorf("unexpected schema version: %v", resp["schema_version"])
	}
	// maddy_status should be present; value depends on whether port 25 is in use
	maddyStatus, ok := resp["maddy_status"].(string)
	if !ok {
		t.Errorf("expected maddy_status field in status response")
	}
	if maddyStatus != "stopped" && maddyStatus != "running" {
		t.Errorf("expected maddy_status 'stopped' or 'running', got %q", maddyStatus)
	}
	// recent_logs should no longer be present
	if _, ok := resp["recent_logs"]; ok {
		t.Errorf("recent_logs should not be present in status response")
	}
}

func TestTestSend(t *testing.T) {
	srv, _ := testServer(t)

	// When Maddy is not running, test send should return 200 with status "failed"
	rr := doRequest(t, srv, "POST", "/api/v1/test/send", map[string]string{
		"from": "test@drcs.ca",
		"to":   "test@example.com",
	}, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("test send: %d %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	// Status will be "failed" since no Maddy is running; that is expected
	if resp["status"] != "failed" && resp["status"] != "sent" {
		t.Errorf("expected status 'failed' or 'sent', got %q", resp["status"])
	}
}

func TestTestSendValidation(t *testing.T) {
	srv, _ := testServer(t)

	// Missing both from and to
	rr := doRequest(t, srv, "POST", "/api/v1/test/send", map[string]string{}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing 'to', got %d", rr.Code)
	}
}

func TestTestSendMissingFrom(t *testing.T) {
	srv, _ := testServer(t)

	// Missing from — should return 400
	rr := doRequest(t, srv, "POST", "/api/v1/test/send", map[string]string{
		"to": "test@example.com",
	}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing 'from', got %d", rr.Code)
	}
}

func TestDNSCheck(t *testing.T) {
	srv, _ := testServer(t)

	// Create a domain
	doRequest(t, srv, "POST", "/api/v1/domains", map[string]string{"name": "example.com"}, true)

	// Call the DNS check endpoint
	rr := doRequest(t, srv, "GET", "/api/v1/domains/1/dns/check", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("dns check: %d %s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Records []dnsCheckRecord `json:"records"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Should have 3 records: SPF, DKIM, DMARC
	if len(resp.Records) != 3 {
		t.Errorf("expected 3 records, got %d", len(resp.Records))
	}

	// Verify record structure
	purposes := map[string]bool{}
	for _, rec := range resp.Records {
		purposes[rec.Purpose] = true
		if rec.Type != "TXT" {
			t.Errorf("expected type 'TXT', got %q", rec.Type)
		}
		if rec.Name == "" {
			t.Error("expected non-empty name")
		}
		if rec.Status == "" {
			t.Error("expected non-empty status")
		}
		// Status should be one of the valid values
		switch rec.Status {
		case "ok", "missing", "update", "conflict":
			// valid
		default:
			t.Errorf("unexpected status %q for %s", rec.Status, rec.Purpose)
		}
	}

	for _, purpose := range []string{"spf", "dkim", "dmarc"} {
		if !purposes[purpose] {
			t.Errorf("missing expected purpose %q", purpose)
		}
	}

	// DKIM should say "Generate DKIM keys first" since we haven't generated any
	for _, rec := range resp.Records {
		if rec.Purpose == "dkim" && rec.Status == "missing" {
			if rec.Message != "Generate DKIM keys first" {
				t.Errorf("expected DKIM message about generating keys, got %q", rec.Message)
			}
		}
	}
}

func TestDNSCheckNotFound(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "GET", "/api/v1/domains/999/dns/check", nil, true)
	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestDNSCheckNoAuth(t *testing.T) {
	srv, _ := testServer(t)

	rr := doRequest(t, srv, "GET", "/api/v1/domains/1/dns/check", nil, false)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 without auth, got %d", rr.Code)
	}
}

func TestSMTPUserCRUD(t *testing.T) {
	srv, _ := testServer(t)

	// List — empty initially
	rr := doRequest(t, srv, "GET", "/api/v1/smtp-users", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("list smtp users: %d %s", rr.Code, rr.Body.String())
	}
	var users []db.SMTPUser
	json.Unmarshal(rr.Body.Bytes(), &users)
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	// Create
	rr = doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
		"password": "securepass123",
	}, true)
	if rr.Code != http.StatusCreated {
		t.Fatalf("create smtp user: %d %s", rr.Code, rr.Body.String())
	}
	var created db.SMTPUser
	json.Unmarshal(rr.Body.Bytes(), &created)
	if created.Username != "sender@drcs.ca" {
		t.Errorf("expected username 'sender@drcs.ca', got %q", created.Username)
	}

	// List — should have 1 user
	rr = doRequest(t, srv, "GET", "/api/v1/smtp-users", nil, true)
	json.Unmarshal(rr.Body.Bytes(), &users)
	if len(users) != 1 {
		t.Errorf("expected 1 user, got %d", len(users))
	}

	// Reset password
	rr = doRequest(t, srv, "PUT", "/api/v1/smtp-users/1/password", map[string]string{
		"password": "newpassword123",
	}, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("reset password: %d %s", rr.Code, rr.Body.String())
	}

	// Delete
	rr = doRequest(t, srv, "DELETE", "/api/v1/smtp-users/1", nil, true)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete smtp user: %d %s", rr.Code, rr.Body.String())
	}

	// List — should be empty again
	rr = doRequest(t, srv, "GET", "/api/v1/smtp-users", nil, true)
	json.Unmarshal(rr.Body.Bytes(), &users)
	if len(users) != 0 {
		t.Errorf("expected 0 users after delete, got %d", len(users))
	}
}

func TestSMTPUserDuplicate(t *testing.T) {
	srv, _ := testServer(t)

	doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
		"password": "securepass123",
	}, true)

	rr := doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
		"password": "otherpass1234",
	}, true)
	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate, got %d", rr.Code)
	}
}

func TestSMTPUserValidation(t *testing.T) {
	srv, _ := testServer(t)

	// Missing username
	rr := doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"password": "securepass123",
	}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing username, got %d", rr.Code)
	}

	// Missing password
	rr = doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
	}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing password, got %d", rr.Code)
	}

	// Short password
	rr = doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
		"password": "short",
	}, true)
	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for short password, got %d", rr.Code)
	}
}

func TestSMTPUserAutoSubmissionToggle(t *testing.T) {
	srv, database := testServer(t)

	// submission_enabled should start as 'false'
	val, _ := database.GetSetting("submission_enabled")
	if val != "false" {
		t.Errorf("expected submission_enabled='false' initially, got %q", val)
	}

	// Create first user — should auto-enable submission
	doRequest(t, srv, "POST", "/api/v1/smtp-users", map[string]string{
		"username": "sender@drcs.ca",
		"password": "securepass123",
	}, true)

	val, _ = database.GetSetting("submission_enabled")
	if val != "true" {
		t.Errorf("expected submission_enabled='true' after first user, got %q", val)
	}

	// Delete last user — should auto-disable submission
	doRequest(t, srv, "DELETE", "/api/v1/smtp-users/1", nil, true)

	val, _ = database.GetSetting("submission_enabled")
	if val != "false" {
		t.Errorf("expected submission_enabled='false' after deleting last user, got %q", val)
	}
}

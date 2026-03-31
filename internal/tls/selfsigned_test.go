package tls

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSelfSignedCert(t *testing.T) {
	dir := t.TempDir()

	certPath, keyPath, err := EnsureSelfSignedCert(dir, "mail.drcs.ca")
	if err != nil {
		t.Fatalf("EnsureSelfSignedCert: %v", err)
	}

	// Verify paths
	expectedCert := filepath.Join(dir, "tls", "selfsigned.crt")
	expectedKey := filepath.Join(dir, "tls", "selfsigned.key")
	if certPath != expectedCert {
		t.Errorf("certPath = %q, want %q", certPath, expectedCert)
	}
	if keyPath != expectedKey {
		t.Errorf("keyPath = %q, want %q", keyPath, expectedKey)
	}

	// Verify files exist
	if _, err := os.Stat(certPath); err != nil {
		t.Fatalf("cert file not created: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Fatalf("key file not created: %v", err)
	}

	// Verify key file permissions (owner read/write only)
	info, _ := os.Stat(keyPath)
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("key file permissions = %o, want 0600", perm)
	}

	// Verify the cert and key are valid TLS pair
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadX509KeyPair: %v", err)
	}
}

func TestEnsureSelfSignedCertSANs(t *testing.T) {
	dir := t.TempDir()

	certPath, _, err := EnsureSelfSignedCert(dir, "mail.example.com")
	if err != nil {
		t.Fatalf("EnsureSelfSignedCert: %v", err)
	}

	// Parse the certificate and check SANs
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("reading cert: %v", err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		t.Fatal("failed to decode cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parsing certificate: %v", err)
	}

	// Check DNS SANs
	wantDNS := map[string]bool{"mail.example.com": false, "localhost": false}
	for _, name := range cert.DNSNames {
		if _, ok := wantDNS[name]; ok {
			wantDNS[name] = true
		}
	}
	for name, found := range wantDNS {
		if !found {
			t.Errorf("missing DNS SAN: %s", name)
		}
	}

	// Check subject
	if cert.Subject.CommonName != "mail.example.com" {
		t.Errorf("CN = %q, want %q", cert.Subject.CommonName, "mail.example.com")
	}
	if len(cert.Subject.Organization) == 0 || cert.Subject.Organization[0] != "SignPost" {
		t.Errorf("Organization = %v, want [SignPost]", cert.Subject.Organization)
	}

	// Check IP SANs
	hasLoopback := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "127.0.0.1" || ip.String() == "::1" {
			hasLoopback = true
		}
	}
	if !hasLoopback {
		t.Error("missing loopback IP SAN")
	}
}

func TestEnsureSelfSignedCertReusesExisting(t *testing.T) {
	dir := t.TempDir()

	// Generate first time
	certPath1, keyPath1, err := EnsureSelfSignedCert(dir, "mail.drcs.ca")
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	// Read the cert content
	cert1, _ := os.ReadFile(certPath1)
	key1, _ := os.ReadFile(keyPath1)

	// Call again — should reuse
	certPath2, keyPath2, err := EnsureSelfSignedCert(dir, "mail.drcs.ca")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if certPath2 != certPath1 {
		t.Error("cert path changed on second call")
	}
	if keyPath2 != keyPath1 {
		t.Error("key path changed on second call")
	}

	cert2, _ := os.ReadFile(certPath2)
	key2, _ := os.ReadFile(keyPath2)

	if string(cert1) != string(cert2) {
		t.Error("cert content changed — should have been reused")
	}
	if string(key1) != string(key2) {
		t.Error("key content changed — should have been reused")
	}
}

func TestEnsureSelfSignedCertRegeneratesIfPartial(t *testing.T) {
	dir := t.TempDir()

	// Create only the cert file (simulate partial state)
	tlsDir := filepath.Join(dir, "tls")
	os.MkdirAll(tlsDir, 0755)
	os.WriteFile(filepath.Join(tlsDir, "selfsigned.crt"), []byte("old"), 0644)

	// Should regenerate since key is missing
	certPath, keyPath, err := EnsureSelfSignedCert(dir, "mail.drcs.ca")
	if err != nil {
		t.Fatalf("EnsureSelfSignedCert: %v", err)
	}

	// Both should now be valid
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatalf("LoadX509KeyPair after regeneration: %v", err)
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// Non-existent file
	if fileExists(filepath.Join(dir, "nope")) {
		t.Error("fileExists returned true for non-existent file")
	}

	// Directory should not count
	if fileExists(dir) {
		t.Error("fileExists returned true for directory")
	}

	// Existing file
	f := filepath.Join(dir, "exists")
	os.WriteFile(f, []byte("hi"), 0644)
	if !fileExists(f) {
		t.Error("fileExists returned false for existing file")
	}
}

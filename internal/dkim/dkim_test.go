package dkim

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateKey(t *testing.T) {
	dir := t.TempDir()

	kp, err := GenerateKey(dir, "drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	// Verify key pair fields
	if kp.Domain != "drcs.ca" {
		t.Errorf("expected domain 'drcs.ca', got %q", kp.Domain)
	}
	if kp.Selector != "signpost" {
		t.Errorf("expected selector 'signpost', got %q", kp.Selector)
	}
	if !strings.HasPrefix(kp.PublicKeyDNS, "v=DKIM1; k=rsa; p=") {
		t.Errorf("unexpected DNS record format: %q", kp.PublicKeyDNS)
	}

	// Verify private key file exists and is valid PEM
	keyData, err := os.ReadFile(kp.PrivateKeyPath)
	if err != nil {
		t.Fatalf("reading key file: %v", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		t.Fatal("failed to decode PEM from key file")
	}
	if block.Type != "PRIVATE KEY" {
		t.Errorf("expected PEM type 'PRIVATE KEY', got %q", block.Type)
	}

	// Verify it's a valid PKCS#8 key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parsing PKCS#8 key: %v", err)
	}
	if key == nil {
		t.Fatal("parsed key is nil")
	}

	// Verify file permissions
	info, _ := os.Stat(kp.PrivateKeyPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected key file perms 0600, got %o", info.Mode().Perm())
	}
}

func TestGenerateKeyCreatesDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "keys")

	_, err := GenerateKey(dir, "example.com", "sel1")
	if err != nil {
		t.Fatalf("GenerateKey with nested dir: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("keys directory not created: %v", err)
	}
}

func TestLoadPublicKeyDNS(t *testing.T) {
	dir := t.TempDir()

	kp, err := GenerateKey(dir, "drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	// Load the public key from the generated file
	dns, err := LoadPublicKeyDNS(kp.PrivateKeyPath)
	if err != nil {
		t.Fatalf("LoadPublicKeyDNS: %v", err)
	}

	// Should match what GenerateKey returned
	if dns != kp.PublicKeyDNS {
		t.Errorf("LoadPublicKeyDNS returned different DNS record:\ngenerated: %s\nloaded:    %s", kp.PublicKeyDNS, dns)
	}
}

func TestLoadPublicKeyDNSInvalidFile(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.key")
	os.WriteFile(badFile, []byte("not a pem file"), 0600)

	_, err := LoadPublicKeyDNS(badFile)
	if err == nil {
		t.Error("expected error for invalid key file")
	}
}

func TestLoadPublicKeyDNSMissing(t *testing.T) {
	_, err := LoadPublicKeyDNS("/nonexistent/path.key")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestDNSRecordName(t *testing.T) {
	name := DNSRecordName("signpost", "drcs.ca")
	if name != "signpost._domainkey.drcs.ca" {
		t.Errorf("unexpected DNS record name: %q", name)
	}
}

func TestRecommendedSPF(t *testing.T) {
	spf := RecommendedSPF("mail.drcs.ca")
	if !strings.HasPrefix(spf, "v=spf1") {
		t.Errorf("unexpected SPF: %q", spf)
	}
	if !strings.Contains(spf, "mail.drcs.ca") {
		t.Errorf("SPF should reference hostname: %q", spf)
	}
}

func TestRecommendedDMARC(t *testing.T) {
	dmarc := RecommendedDMARC("drcs.ca")
	if !strings.HasPrefix(dmarc, "v=DMARC1") {
		t.Errorf("unexpected DMARC: %q", dmarc)
	}
	if !strings.Contains(dmarc, "drcs.ca") {
		t.Errorf("DMARC should reference domain: %q", dmarc)
	}
}

func TestDMARCRecordName(t *testing.T) {
	name := DMARCRecordName("drcs.ca")
	if name != "_dmarc.drcs.ca" {
		t.Errorf("unexpected DMARC record name: %q", name)
	}
}

func TestSignMessage(t *testing.T) {
	dir := t.TempDir()

	// Generate a key pair to sign with
	kp, err := GenerateKey(dir, "drcs.ca", "signpost")
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	msg := []byte("From: sender@drcs.ca\r\nTo: recipient@example.com\r\nSubject: Test\r\nDate: Mon, 29 Mar 2026 12:00:00 +0000\r\nMessage-ID: <test@signpost>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nHello, world!\r\n")

	signed, err := SignMessage(msg, "drcs.ca", "signpost", kp.PrivateKeyPath)
	if err != nil {
		t.Fatalf("SignMessage: %v", err)
	}

	// The signed message should contain a DKIM-Signature header
	if !strings.Contains(string(signed), "DKIM-Signature:") {
		t.Error("signed message does not contain DKIM-Signature header")
	}

	// The original message content should still be present
	if !strings.Contains(string(signed), "Hello, world!") {
		t.Error("signed message does not contain original body")
	}

	// Should be longer than the original (DKIM header added)
	if len(signed) <= len(msg) {
		t.Errorf("signed message (%d bytes) should be longer than original (%d bytes)", len(signed), len(msg))
	}
}

func TestSignMessageInvalidKeyPath(t *testing.T) {
	_, err := SignMessage([]byte("From: a@b.com\r\n\r\nbody"), "b.com", "sel", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error for missing key file")
	}
}

func TestSignMessageInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.key")
	os.WriteFile(badFile, []byte("not a pem file"), 0600)

	_, err := SignMessage([]byte("From: a@b.com\r\n\r\nbody"), "b.com", "sel", badFile)
	if err == nil {
		t.Error("expected error for invalid PEM")
	}
}

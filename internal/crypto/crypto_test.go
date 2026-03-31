package crypto

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	key, err := DeriveKey("test-secret-key-minimum-32-characters-long")
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(key))
	}
}

func TestDeriveKeyDeterministic(t *testing.T) {
	key1, err := DeriveKey("test-secret-key-minimum-32-characters-long")
	if err != nil {
		t.Fatalf("DeriveKey 1 failed: %v", err)
	}
	key2, err := DeriveKey("test-secret-key-minimum-32-characters-long")
	if err != nil {
		t.Fatalf("DeriveKey 2 failed: %v", err)
	}
	if string(key1) != string(key2) {
		t.Error("DeriveKey should be deterministic for the same input")
	}
}

func TestDeriveKeyShortKey(t *testing.T) {
	_, err := DeriveKey("short")
	if err == nil {
		t.Error("expected error for short key")
	}
	if !strings.Contains(err.Error(), "at least 32 characters") {
		t.Errorf("expected error about key length, got: %v", err)
	}
}

func TestDeriveKeyDifferentInputs(t *testing.T) {
	key1, _ := DeriveKey("test-secret-key-minimum-32-characters-long")
	key2, _ := DeriveKey("different-secret-key-minimum-32-characters")
	if string(key1) == string(key2) {
		t.Error("different inputs should produce different keys")
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	key, err := DeriveKey("test-secret-key-minimum-32-characters-long")
	if err != nil {
		t.Fatalf("DeriveKey failed: %v", err)
	}

	plaintext := "my-secret-password-123!"
	ciphertext, nonce, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Ciphertext should be base64-encoded and different from plaintext
	if ciphertext == plaintext {
		t.Error("ciphertext should not equal plaintext")
	}
	if _, err := base64.StdEncoding.DecodeString(ciphertext); err != nil {
		t.Errorf("ciphertext should be valid base64: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(nonce); err != nil {
		t.Errorf("nonce should be valid base64: %v", err)
	}

	// Decrypt
	decrypted, err := Decrypt(key, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key, _ := DeriveKey("test-secret-key-minimum-32-characters-long")
	plaintext := "same-password"

	ct1, n1, _ := Encrypt(key, plaintext)
	ct2, n2, _ := Encrypt(key, plaintext)

	// Different nonces should produce different ciphertexts
	if n1 == n2 {
		t.Error("nonces should be unique (random)")
	}
	if ct1 == ct2 {
		t.Error("ciphertexts should differ due to random nonce")
	}

	// Both should decrypt to the same plaintext
	d1, _ := Decrypt(key, ct1, n1)
	d2, _ := Decrypt(key, ct2, n2)
	if d1 != plaintext || d2 != plaintext {
		t.Error("both ciphertexts should decrypt to the same plaintext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	key1, _ := DeriveKey("test-secret-key-minimum-32-characters-long")
	key2, _ := DeriveKey("another-secret-key-at-least-32-characters")

	ciphertext, nonce, err := Encrypt(key1, "secret-data")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = Decrypt(key2, ciphertext, nonce)
	if err == nil {
		t.Error("expected decryption to fail with wrong key")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("expected decryption failure error, got: %v", err)
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key, _ := DeriveKey("test-secret-key-minimum-32-characters-long")

	ciphertext, nonce, err := Encrypt(key, "secret-data")
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Tamper with the ciphertext
	ctBytes, _ := base64.StdEncoding.DecodeString(ciphertext)
	ctBytes[0] ^= 0xff // flip bits
	tampered := base64.StdEncoding.EncodeToString(ctBytes)

	_, err = Decrypt(key, tampered, nonce)
	if err == nil {
		t.Error("expected decryption to fail with tampered ciphertext")
	}
}

func TestDecryptInvalidBase64(t *testing.T) {
	key, _ := DeriveKey("test-secret-key-minimum-32-characters-long")

	// Invalid base64 ciphertext
	_, err := Decrypt(key, "not-valid-base64!!!", "dGVzdA==")
	if err == nil {
		t.Error("expected error for invalid base64 ciphertext")
	}

	// Invalid base64 nonce
	_, err = Decrypt(key, "dGVzdA==", "not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64 nonce")
	}
}

func TestEncryptEmptyString(t *testing.T) {
	key, _ := DeriveKey("test-secret-key-minimum-32-characters-long")

	ciphertext, nonce, err := Encrypt(key, "")
	if err != nil {
		t.Fatalf("Encrypt failed for empty string: %v", err)
	}

	decrypted, err := Decrypt(key, ciphertext, nonce)
	if err != nil {
		t.Fatalf("Decrypt failed for empty string: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}

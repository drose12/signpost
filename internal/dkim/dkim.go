package dkim

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// DefaultKeySize is the RSA key size for DKIM keys.
	DefaultKeySize = 2048
)

// KeyPair holds a generated DKIM key pair and its DNS record.
type KeyPair struct {
	PrivateKeyPath string
	PublicKeyDNS   string // The full DNS TXT record value
	Selector       string
	Domain         string
}

// GenerateKey generates a new RSA DKIM key pair for the given domain and selector.
// It writes the private key to keysDir/<domain>.key and returns the key pair info.
func GenerateKey(keysDir, domain, selector string) (*KeyPair, error) {
	if err := os.MkdirAll(keysDir, 0750); err != nil {
		return nil, fmt.Errorf("creating keys directory: %w", err)
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, DefaultKeySize)
	if err != nil {
		return nil, fmt.Errorf("generating RSA key: %w", err)
	}

	// Write private key in PKCS#8 PEM format (what Maddy expects)
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling private key: %w", err)
	}

	keyPath := filepath.Join(keysDir, domain+".key")
	keyFile, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("creating key file: %w", err)
	}
	defer keyFile.Close()

	if err := pem.Encode(keyFile, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}); err != nil {
		return nil, fmt.Errorf("writing private key: %w", err)
	}

	// Generate DNS TXT record value from public key
	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("marshaling public key: %w", err)
	}
	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)

	dnsRecord := fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubB64)

	return &KeyPair{
		PrivateKeyPath: keyPath,
		PublicKeyDNS:   dnsRecord,
		Selector:       selector,
		Domain:         domain,
	}, nil
}

// DNSRecordName returns the DNS name where the DKIM TXT record should be published.
// Format: <selector>._domainkey.<domain>
func DNSRecordName(selector, domain string) string {
	return fmt.Sprintf("%s._domainkey.%s", selector, domain)
}

// RecommendedSPF returns a recommended SPF record for the domain.
func RecommendedSPF(hostname string) string {
	return fmt.Sprintf("v=spf1 mx a:%s ~all", hostname)
}

// RecommendedDMARC returns a recommended DMARC record for the domain.
func RecommendedDMARC(domain string) string {
	return fmt.Sprintf("v=DMARC1; p=quarantine; ruf=mailto:postmaster@%s; fo=1", domain)
}

// DMARCRecordName returns the DNS name for the DMARC record.
func DMARCRecordName(domain string) string {
	return fmt.Sprintf("_dmarc.%s", domain)
}

// ValidateAndExtractPublicKey validates a PEM-encoded private key and returns the DKIM DNS record value.
func ValidateAndExtractPublicKey(pemData []byte) (string, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return "", fmt.Errorf("no PEM block found")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS1 format too
		rsaKey, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err2 != nil {
			return "", fmt.Errorf("failed to parse key (tried PKCS8 and PKCS1): %v", err)
		}
		key = rsaKey
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("key is not RSA")
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshaling public key: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	return fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubB64), nil
}

// LoadPublicKeyDNS reads a private key file and returns the DKIM DNS record value.
func LoadPublicKeyDNS(keyPath string) (string, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return "", fmt.Errorf("reading key file: %w", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parsing private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("key is not RSA")
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return "", fmt.Errorf("marshaling public key: %w", err)
	}

	pubB64 := base64.StdEncoding.EncodeToString(pubBytes)
	return fmt.Sprintf("v=DKIM1; k=rsa; p=%s", pubB64), nil
}

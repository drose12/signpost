package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// EnsureSelfSignedCert creates a self-signed TLS certificate if one doesn't
// already exist. The certificate is valid for 10 years and includes the given
// hostname plus "localhost" as SANs. Returns the cert and key file paths.
func EnsureSelfSignedCert(dataDir, hostname string) (certPath, keyPath string, err error) {
	certPath = filepath.Join(dataDir, "tls", "selfsigned.crt")
	keyPath = filepath.Join(dataDir, "tls", "selfsigned.key")

	// If both files already exist, reuse them
	if fileExists(certPath) && fileExists(keyPath) {
		return certPath, keyPath, nil
	}

	// Generate RSA 2048-bit key
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generating RSA key: %w", err)
	}

	// Serial number
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return "", "", fmt.Errorf("generating serial number: %w", err)
	}

	// Certificate template
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"SignPost"},
			CommonName:   hostname,
		},
		NotBefore: time.Now().Add(-1 * time.Hour), // 1 hour grace for clock skew
		NotAfter:  time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		DNSNames:              []string{hostname, "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("creating certificate: %w", err)
	}

	// Ensure tls directory exists
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return "", "", fmt.Errorf("creating tls directory: %w", err)
	}

	// Write cert PEM
	certFile, err := os.OpenFile(certPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return "", "", fmt.Errorf("creating cert file: %w", err)
	}
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		certFile.Close()
		return "", "", fmt.Errorf("writing cert PEM: %w", err)
	}
	certFile.Close()

	// Write key PEM (restricted permissions)
	keyFile, err := os.OpenFile(keyPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", "", fmt.Errorf("creating key file: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}); err != nil {
		keyFile.Close()
		return "", "", fmt.Errorf("writing key PEM: %w", err)
	}
	keyFile.Close()

	return certPath, keyPath, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

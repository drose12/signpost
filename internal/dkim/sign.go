package dkim

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/emersion/go-msgauth/dkim"
)

// SignMessage DKIM-signs a raw email message using the private key at keyPath.
// It returns the signed message with the DKIM-Signature header prepended.
func SignMessage(message []byte, domain, selector, keyPath string) ([]byte, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("reading DKIM private key: %w", err)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", keyPath)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parsing PKCS#8 private key: %w", err)
	}

	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA")
	}

	options := &dkim.SignOptions{
		Domain:                 domain,
		Selector:               selector,
		Signer:                 rsaKey,
		HeaderCanonicalization: dkim.CanonicalizationRelaxed,
		BodyCanonicalization:   dkim.CanonicalizationRelaxed,
		HeaderKeys:             []string{"From", "To", "Subject", "Date", "Message-ID", "MIME-Version", "Content-Type"},
	}

	var signed bytes.Buffer
	if err := dkim.Sign(&signed, bytes.NewReader(message), options); err != nil {
		return nil, fmt.Errorf("DKIM signing: %w", err)
	}

	return signed.Bytes(), nil
}

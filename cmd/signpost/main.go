package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/drose-drcs/signpost/internal/api"
	"github.com/drose-drcs/signpost/internal/config"
	"github.com/drose-drcs/signpost/internal/crypto"
	"github.com/drose-drcs/signpost/internal/db"
	selfsigned "github.com/drose-drcs/signpost/internal/tls"
	"github.com/drose-drcs/signpost/web"
)

var version = "v0.2.0"

func main() {
	fmt.Println("SignPost - DKIM-signing SMTP relay")
	fmt.Printf("Version: %s\n", version)

	if len(os.Args) > 1 && os.Args[1] == "test" {
		runHealthCheck()
		return
	}

	dataDir := envOrDefault("SIGNPOST_DATA_DIR", "/data/signpost")
	dbPath := dataDir + "/signpost.db"
	confOutput := dataDir + "/maddy.conf"
	keysDir := dataDir + "/dkim_keys"
	tmplPath := envOrDefault("SIGNPOST_TEMPLATE_PATH", "/app/templates/maddy.conf.tmpl")

	// Initialize database
	log.Println("Initializing database...")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := database.CheckIntegrity(); err != nil {
		log.Printf("WARNING: Database integrity check failed: %v", err)
	}

	// Initialize config generator
	configGen := config.NewGenerator(tmplPath, confOutput, dataDir)

	// Generate self-signed TLS certificate if needed
	tlsConfig, err := database.GetTLSConfig()
	if err != nil {
		log.Printf("WARNING: Failed to get TLS config: %v", err)
	}
	if tlsConfig != nil && (tlsConfig.Mode == "self-signed" || tlsConfig.Mode == "") {
		hostname := envOrDefault("SIGNPOST_DOMAIN", "localhost")
		certPath, keyPath, certErr := selfsigned.EnsureSelfSignedCert(dataDir, "mail."+hostname)
		if certErr != nil {
			log.Printf("WARNING: Failed to generate self-signed cert: %v", certErr)
		} else {
			log.Printf("Using self-signed TLS certificate: %s", certPath)
			if updateErr := database.UpdateTLSCertPaths(certPath, keyPath); updateErr != nil {
				log.Printf("WARNING: Failed to update TLS cert paths in DB: %v", updateErr)
			}
		}
	}

	// Set up decryption for relay passwords
	secretKey := envOrDefault("SIGNPOST_SECRET_KEY", "")
	var decryptFn func(string, string) (string, error)
	if secretKey != "" {
		encKey, err := crypto.DeriveKey(secretKey)
		if err != nil {
			log.Fatalf("Failed to derive encryption key: %v", err)
		}
		decryptFn = func(enc, nonce string) (string, error) {
			// Graceful migration: old placeholder nonces mean plaintext
			plaintext, err := crypto.Decrypt(encKey, enc, nonce)
			if err != nil && nonce == "placeholder-nonce" {
				return enc, nil
			}
			return plaintext, err
		}
	} else {
		// No secret key — assume plaintext passwords (migration path)
		decryptFn = func(enc, _ string) (string, error) { return enc, nil }
	}

	// Generate initial Maddy config
	log.Println("Generating Maddy configuration...")
	if err := configGen.WriteConfig(database, decryptFn); err != nil {
		log.Printf("WARNING: Failed to generate initial config: %v", err)
	}

	// Get admin credentials
	adminUser := envOrDefault("SIGNPOST_ADMIN_USER", "admin")
	adminPass := os.Getenv("SIGNPOST_ADMIN_PASS")
	if adminPass == "" {
		log.Fatal("SIGNPOST_ADMIN_PASS environment variable is required")
	}

	// Start API server
	webPort := envOrDefault("SIGNPOST_WEB_PORT", "8080")
	hostname := "mail." + envOrDefault("SIGNPOST_DOMAIN", "localhost")
	srv := api.NewServer(database, configGen, keysDir, adminUser, adminPass, secretKey, dataDir, hostname, version, web.DistFS)

	log.Printf("Starting web server on :%s", webPort)
	if err := http.ListenAndServe(":"+webPort, srv.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runHealthCheck() {
	webPort := envOrDefault("SIGNPOST_WEB_PORT", "8080")
	resp, err := http.Get(fmt.Sprintf("http://localhost:%s/api/v1/healthz", webPort))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Health check failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Health check returned status %d\n", resp.StatusCode)
		os.Exit(1)
	}

	fmt.Println("Health check passed")
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

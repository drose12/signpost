package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/drose-drcs/signpost/internal/api"
	"github.com/drose-drcs/signpost/internal/config"
	"github.com/drose-drcs/signpost/internal/db"
	"github.com/drose-drcs/signpost/web"
)

var version = "dev"

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

	// Generate initial Maddy config
	log.Println("Generating Maddy configuration...")
	if err := configGen.WriteConfig(database, nil); err != nil {
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
	srv := api.NewServer(database, configGen, keysDir, adminUser, adminPass, web.DistFS)

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

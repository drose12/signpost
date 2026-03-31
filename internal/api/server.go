package api

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/drose-drcs/signpost/internal/config"
	"github.com/drose-drcs/signpost/internal/crypto"
	"github.com/drose-drcs/signpost/internal/db"
)

// Server is the SignPost REST API server.
type Server struct {
	db        *db.DB
	configGen *config.Generator
	keysDir   string
	router    chi.Router
	adminUser string
	adminPass string
	encKey    []byte // AES-256 key derived from SIGNPOST_SECRET_KEY
	webFS     fs.FS  // embedded frontend, nil in dev
}

// NewServer creates a new API server.
func NewServer(database *db.DB, configGen *config.Generator, keysDir, adminUser, adminPass, secretKey string, webFS fs.FS) *Server {
	var encKey []byte
	if secretKey != "" {
		var err error
		encKey, err = crypto.DeriveKey(secretKey)
		if err != nil {
			log.Fatalf("Failed to derive encryption key: %v", err)
		}
	}
	s := &Server{
		db:        database,
		configGen: configGen,
		keysDir:   keysDir,
		adminUser: adminUser,
		adminPass: adminPass,
		encKey:    encKey,
		webFS:     webFS,
	}
	s.router = s.buildRouter()
	return s
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	// Health check — no auth required
	r.Get("/api/v1/healthz", s.handleHealthz)

	// All other API routes require basic auth
	r.Group(func(r chi.Router) {
		r.Use(s.basicAuth)

		r.Get("/api/v1/status", s.handleStatus)

		// Domains
		r.Get("/api/v1/domains", s.handleListDomains)
		r.Post("/api/v1/domains", s.handleCreateDomain)
		r.Get("/api/v1/domains/{id}", s.handleGetDomain)
		r.Delete("/api/v1/domains/{id}", s.handleDeleteDomain)
		r.Post("/api/v1/domains/{id}/dkim/generate", s.handleGenerateDKIM)
		r.Get("/api/v1/domains/{id}/dns", s.handleGetDNSRecords)
		r.Get("/api/v1/domains/{id}/dns/check", s.handleDNSCheck)

		// Relay configuration
		r.Get("/api/v1/domains/{id}/relay", s.handleGetRelay)
		r.Put("/api/v1/domains/{id}/relay", s.handleUpdateRelay)
		r.Post("/api/v1/domains/{id}/relay/test", s.handleRelayTest)

		// Settings
		r.Get("/api/v1/settings", s.handleGetSettings)
		r.Put("/api/v1/settings", s.handleUpdateSettings)

		// Mail logs
		r.Get("/api/v1/logs", s.handleGetLogs)

		// Test
		r.Post("/api/v1/test/send", s.handleTestSend)
	})

	// Serve frontend SPA (after API routes)
	if s.webFS != nil {
		r.Handle("/*", s.spaHandler())
	}

	return r
}

func (s *Server) spaHandler() http.Handler {
	// Strip the "dist" prefix from the embedded FS
	subFS, err := fs.Sub(s.webFS, "dist")
	if err != nil {
		log.Fatalf("Failed to create sub filesystem: %v", err)
	}
	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the static file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}
		// Check if file exists in embedded FS
		f, err := subFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// File not found — serve index.html for client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

func (s *Server) basicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != s.adminUser || pass != s.adminPass {
			w.Header().Set("WWW-Authenticate", `Basic realm="SignPost"`)
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// decryptRelayPassword decrypts an encrypted relay password.
// If no encryption key is configured, it assumes plaintext (migration path).
// If decryption fails and the nonce is "placeholder-nonce", it treats the
// value as plaintext (graceful migration from pre-encryption data).
func (s *Server) decryptRelayPassword(enc, nonce string) (string, error) {
	if s.encKey == nil {
		return enc, nil
	}
	plaintext, err := crypto.Decrypt(s.encKey, enc, nonce)
	if err != nil {
		// Fallback: if nonce is the old placeholder, treat as plaintext
		if nonce == "placeholder-nonce" {
			return enc, nil
		}
		return "", err
	}
	return plaintext, nil
}

// regenerateConfig regenerates the Maddy config and signals Maddy to reload.
func (s *Server) regenerateConfig() {
	if err := s.configGen.WriteConfig(s.db, s.decryptRelayPassword); err != nil {
		log.Printf("ERROR: Failed to regenerate config: %v", err)
		return
	}
	// Signal Maddy to reload — pkill sends SIGHUP to all matching processes.
	// In the container, this is the s6-managed Maddy process.
	if err := exec.Command("pkill", "-HUP", "maddy").Run(); err != nil {
		log.Printf("WARNING: Failed to signal Maddy reload (may not be running): %v", err)
	} else {
		log.Println("Maddy config regenerated and reload signaled")
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

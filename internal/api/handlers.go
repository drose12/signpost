package api

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/drose-drcs/signpost/internal/crypto"
	"github.com/drose-drcs/signpost/internal/db"
	"github.com/drose-drcs/signpost/internal/dkim"
	selfsigned "github.com/drose-drcs/signpost/internal/tls"
)

// handleHealthz is the lightweight health check endpoint.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if err := s.db.CheckIntegrity(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"db":     "ok",
	})
}

// handleStatus returns dashboard data.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	domains, err := s.db.ListDomains()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	tlsConfig, err := s.db.GetTLSConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	version, _ := s.db.SchemaVersion()

	// Check if Maddy is listening on SMTP port
	maddyStatus := "stopped"
	smtpPort := envOrDefault("SIGNPOST_SMTP_PORT", "25")
	conn, dialErr := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", smtpPort), 500*time.Millisecond)
	if dialErr == nil {
		conn.Close()
		maddyStatus = "running"
	}

	httpPort := envOrDefault("SIGNPOST_HTTP_PORT", "8080")
	submissionPort := envOrDefault("SIGNPOST_SUBMISSION_PORT", "587")

	// Check if msmtpd is needed and running
	msmtpdConn, msmtpdErr := net.DialTimeout("tcp", "127.0.0.1:2500", 500*time.Millisecond)
	msmtpdRunning := msmtpdErr == nil
	if msmtpdRunning {
		msmtpdConn.Close()
	}

	listeners := []map[string]string{
		{"name": "SMTP", "bind": "0.0.0.0:" + smtpPort, "status": maddyStatus},
		{"name": "Submission", "bind": "0.0.0.0:" + submissionPort, "status": maddyStatus},
		{"name": "HTTP API", "bind": "0.0.0.0:" + httpPort, "status": "running"},
	}
	// Only show msmtpd if it's actually running (LOGIN relay active)
	if msmtpdRunning {
		listeners = append(listeners, map[string]string{"name": "msmtpd (LOGIN relay)", "bind": "127.0.0.1:2500", "status": "running"})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":         s.version,
		"domain_count":    len(domains),
		"tls_mode":        tlsConfig.Mode,
		"tls_cert_expiry": tlsConfig.CertExpiry,
		"schema_version":  version,
		"maddy_status":    maddyStatus,
		"listeners":       listeners,
	})
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// handleListDomains returns all configured domains.
func (s *Server) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := s.db.ListDomains()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domains == nil {
		domains = []db.Domain{}
	}
	writeJSON(w, http.StatusOK, domains)
}

// handleCreateDomain adds a new domain.
func (s *Server) handleCreateDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Selector string `json:"selector"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Selector == "" {
		req.Selector = "signpost"
	}

	domain, err := s.db.CreateDomain(req.Name, req.Selector)
	if err != nil {
		writeError(w, http.StatusConflict, fmt.Sprintf("failed to create domain: %v", err))
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusCreated, domain)
}

// handleGetDomain returns a single domain by ID.
func (s *Server) handleGetDomain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	writeJSON(w, http.StatusOK, domain)
}

// handleDeleteDomain removes a domain.
func (s *Server) handleDeleteDomain(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	if err := s.db.DeleteDomain(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleGenerateDKIM generates a new DKIM key pair for a domain.
func (s *Server) handleGenerateDKIM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	kp, err := dkim.GenerateKey(s.keysDir, domain.Name, domain.DKIMSelector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to generate DKIM key: %v", err))
		return
	}

	if err := s.db.UpdateDomainDKIM(id, kp.PrivateKeyPath, kp.PublicKeyDNS); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{
		"dns_record_name":  dkim.DNSRecordName(kp.Selector, kp.Domain),
		"dns_record_value": kp.PublicKeyDNS,
		"selector":         kp.Selector,
		"key_path":         kp.PrivateKeyPath,
	})
}

// handleExportDKIM downloads the private key PEM file for a domain.
func (s *Server) handleExportDKIM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if domain.DKIMKeyPath == nil || *domain.DKIMKeyPath == "" {
		writeError(w, http.StatusNotFound, "no DKIM key generated for this domain")
		return
	}

	keyPEM, err := os.ReadFile(*domain.DKIMKeyPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read key file: %v", err))
		return
	}

	filename := domain.Name + "-dkim-private.pem"
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(keyPEM)
}

// handleImportDKIM imports a DKIM private key PEM file for a domain.
func (s *Server) handleImportDKIM(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// Accept either raw PEM or JSON {"pem": "..."}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var pemData []byte
	// Try JSON first
	var jsonReq struct {
		PEM string `json:"pem"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonReq); err == nil && jsonReq.PEM != "" {
		pemData = []byte(jsonReq.PEM)
	} else {
		// Treat as raw PEM
		pemData = bodyBytes
	}

	// Validate it's a valid PEM private key and extract public key DNS value
	publicKeyDNS, err := dkim.ValidateAndExtractPublicKey(pemData)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid DKIM key: %v", err))
		return
	}

	// Write the key to disk
	keyPath := s.keysDir + "/" + domain.Name + ".key"
	if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to write key file: %v", err))
		return
	}

	// Update the domain record
	if err := s.db.UpdateDomainDKIM(id, keyPath, publicKeyDNS); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{
		"status":           "imported",
		"dns_record_name":  dkim.DNSRecordName(domain.DKIMSelector, domain.Name),
		"dns_record_value": publicKeyDNS,
		"key_path":         keyPath,
	})
}

// handleGetDNSRecords returns the required DNS records for a domain.
func (s *Server) handleGetDNSRecords(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	hostname := "mail." + domain.Name
	records := []map[string]string{
		{
			"type":        "TXT",
			"name":        domain.Name,
			"value":       dkim.RecommendedSPF(hostname),
			"description": "SPF record - authorizes your mail server to send email for this domain",
		},
		{
			"type":        "TXT",
			"name":        dkim.DMARCRecordName(domain.Name),
			"value":       dkim.RecommendedDMARC(domain.Name),
			"description": "DMARC record - tells receivers how to handle emails that fail SPF/DKIM checks",
		},
	}

	if domain.DKIMPublicDNS != nil {
		records = append(records, map[string]string{
			"type":        "TXT",
			"name":        dkim.DNSRecordName(domain.DKIMSelector, domain.Name),
			"value":       *domain.DKIMPublicDNS,
			"description": "DKIM record - publishes your public key for email signature verification",
		})
	}

	writeJSON(w, http.StatusOK, records)
}

// handleGetRelay returns the ACTIVE relay config for a domain.
func (s *Server) handleGetRelay(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	rc, err := s.db.GetRelayConfig(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rc == nil {
		writeJSON(w, http.StatusOK, map[string]string{"method": "direct"})
		return
	}

	writeJSON(w, http.StatusOK, s.relayConfigResponse(rc))
}

// handleGetAllRelayConfigs returns ALL relay configs for a domain (all methods).
func (s *Server) handleGetAllRelayConfigs(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	configs, err := s.db.ListRelayConfigs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]map[string]interface{}, 0, len(configs))
	for i := range configs {
		result = append(result, s.relayConfigResponse(&configs[i]))
	}
	writeJSON(w, http.StatusOK, result)
}

// relayConfigResponse builds a JSON-friendly response map for a relay config,
// including decrypted password.
func (s *Server) relayConfigResponse(rc *db.RelayConfig) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          rc.ID,
		"domain_id":   rc.DomainID,
		"method":      rc.Method,
		"host":        rc.Host,
		"port":        rc.Port,
		"username":    rc.Username,
		"starttls":    rc.StartTLS,
		"auth_method": rc.AuthMethod,
		"active":      rc.Active,
		"created_at":  rc.CreatedAt,
		"updated_at":  rc.UpdatedAt,
	}
	if rc.PasswordEnc != nil && rc.PasswordNonce != nil {
		pw, err := s.decryptRelayPassword(*rc.PasswordEnc, *rc.PasswordNonce)
		if err == nil {
			resp["password"] = pw
		}
	}
	return resp
}

// handleUpdateRelay updates relay config for a domain.
// Upserts by (domain_id, method). If active is true (or omitted for backwards compat),
// all other methods for this domain are deactivated.
func (s *Server) handleUpdateRelay(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	var req struct {
		Method   string  `json:"method"`
		Host     *string `json:"host"`
		Port     int     `json:"port"`
		Username *string `json:"username"`
		Password *string `json:"password"`
		StartTLS bool    `json:"starttls"`
		Active   *bool   `json:"active"` // nil = true for backwards compat
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Method == "" {
		req.Method = "direct"
	}
	if req.Port == 0 {
		req.Port = 587
	}

	// Default active to true for backwards compatibility
	active := true
	if req.Active != nil {
		active = *req.Active
	}

	var passEnc, passNonce *string
	if req.Password != nil && *req.Password != "" {
		if s.encKey == nil {
			writeError(w, http.StatusInternalServerError, "encryption key not configured (set SIGNPOST_SECRET_KEY)")
			return
		}
		enc, nonce, err := crypto.Encrypt(s.encKey, *req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		passEnc = &enc
		passNonce = &nonce
	}

	if err := s.db.UpsertRelayConfig(id, req.Method, req.Host, req.Port, req.Username, passEnc, passNonce, req.StartTLS, active); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleActivateRelay activates a specific relay method for a domain.
func (s *Server) handleActivateRelay(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	method := chi.URLParam(r, "method")
	if method == "" {
		writeError(w, http.StatusBadRequest, "method is required")
		return
	}

	if err := s.db.ActivateRelayConfig(id, method); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "activated", "method": method})
}

// handleGetSettings returns all settings.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.db.GetAllSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

// handleUpdateSettings updates one or more settings.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for key, value := range req {
		if err := s.db.SetSetting(key, value); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleGetLogs returns paginated mail log entries.
func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	filter := db.MailLogFilter{
		Limit:  50,
		Offset: 0,
	}

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			filter.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	if v := r.URL.Query().Get("status"); v != "" {
		filter.Status = &v
	}

	entries, err := s.db.ListMailLog(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if entries == nil {
		entries = []db.MailLogEntry{}
	}

	writeJSON(w, http.StatusOK, entries)
}

// loginAuth implements smtp.Auth for the LOGIN mechanism.
// Many ISP mail servers only support LOGIN, not PLAIN.
type loginAuth struct {
	username, password string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := string(fromServer)
	if prompt == "Username:" || prompt == "VXNlcm5hbWU6" {
		return []byte(a.username), nil
	}
	if prompt == "Password:" || prompt == "UGFzc3dvcmQ6" {
		return []byte(a.password), nil
	}
	return nil, fmt.Errorf("unexpected LOGIN prompt: %s", prompt)
}

// handleRelayTest tests connectivity to the configured relay for a domain.
func (s *Server) handleRelayTest(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	rc, err := s.db.GetRelayConfig(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if rc == nil || rc.Method == "direct" {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "Direct delivery configured — no relay to test",
		})
		return
	}

	if rc.Host == nil || *rc.Host == "" {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  "No relay host configured",
		})
		return
	}

	addr := net.JoinHostPort(*rc.Host, strconv.Itoa(rc.Port))

	// Step 1: TCP connectivity test
	conn, dialErr := net.DialTimeout("tcp", addr, 5*time.Second)
	if dialErr != nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  fmt.Sprintf("Cannot connect to %s: %s", addr, dialErr.Error()),
		})
		return
	}
	conn.Close()

	// Step 2: If credentials are configured, test SMTP AUTH
	if rc.Username != nil && *rc.Username != "" && rc.PasswordEnc != nil && *rc.PasswordEnc != "" {
		nonce := ""
		if rc.PasswordNonce != nil {
			nonce = *rc.PasswordNonce
		}
		password, err := s.decryptRelayPassword(*rc.PasswordEnc, nonce)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  fmt.Sprintf("Failed to decrypt relay password: %s", err.Error()),
			})
			return
		}

		c, smtpErr := smtp.Dial(addr)
		if smtpErr != nil {
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  fmt.Sprintf("SMTP connection failed: %s", smtpErr.Error()),
			})
			return
		}
		defer c.Close()

		if err := c.Hello("localhost"); err != nil {
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  fmt.Sprintf("EHLO failed: %s", err.Error()),
			})
			return
		}

		// Upgrade to TLS if STARTTLS is enabled
		if rc.StartTLS {
			ok, _ := c.Extension("STARTTLS")
			if ok {
				tlsConfig := &tls.Config{ServerName: *rc.Host}
				if err := c.StartTLS(tlsConfig); err != nil {
					writeJSON(w, http.StatusOK, map[string]string{
						"status": "failed",
						"error":  fmt.Sprintf("STARTTLS failed: %s", err.Error()),
					})
					return
				}
			}
		}

		// Try PLAIN auth first, fall back to LOGIN if it fails
		plainAuth := smtp.PlainAuth("", *rc.Username, password, *rc.Host)
		authErr := c.Auth(plainAuth)
		if authErr != nil {
			// PLAIN failed — try LOGIN (many ISP servers only support LOGIN)
			c.Quit()
			c2, err2 := smtp.Dial(addr)
			if err2 != nil {
				writeJSON(w, http.StatusOK, map[string]string{
					"status": "failed",
					"error":  fmt.Sprintf("PLAIN auth failed: %s (LOGIN retry connect failed)", authErr.Error()),
				})
				return
			}
			defer c2.Close()
			c2.Hello("localhost")
			if rc.StartTLS {
				if ok, _ := c2.Extension("STARTTLS"); ok {
					c2.StartTLS(&tls.Config{ServerName: *rc.Host})
				}
			}
			loginA := &loginAuth{username: *rc.Username, password: password}
			if err2 := c2.Auth(loginA); err2 != nil {
				writeJSON(w, http.StatusOK, map[string]string{
					"status": "failed",
					"error":  fmt.Sprintf("Authentication failed (tried PLAIN and LOGIN): PLAIN: %s / LOGIN: %s", authErr.Error(), err2.Error()),
				})
				return
			}
			c2.Quit()
			// Persist detected auth method
			if err := s.db.UpdateRelayAuthMethod(id, "login"); err != nil {
				// Log but don't fail the test — connectivity succeeded
				fmt.Printf("WARNING: failed to persist auth_method=login for domain %d: %v\n", id, err)
			}
			go s.regenerateConfig()
			writeJSON(w, http.StatusOK, map[string]string{
				"status":      "ok",
				"message":     fmt.Sprintf("Connected and authenticated to %s (LOGIN auth)", addr),
				"auth_method": "login",
			})
			return
		}

		c.Quit()
		// Persist detected auth method
		if err := s.db.UpdateRelayAuthMethod(id, "plain"); err != nil {
			fmt.Printf("WARNING: failed to persist auth_method=plain for domain %d: %v\n", id, err)
		}
		go s.regenerateConfig()
		writeJSON(w, http.StatusOK, map[string]string{
			"status":      "ok",
			"message":     fmt.Sprintf("Connected and authenticated to %s (PLAIN auth)", addr),
			"auth_method": "plain",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Connected to %s (no credentials to test)", addr),
	})
}

// handleTestSend sends a test email. For LOGIN auth relays, it DKIM-signs in Go
// and relays directly. For PLAIN auth relays (e.g., Gmail), it sends through Maddy.
func (s *Server) handleTestSend(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.To == "" {
		writeError(w, http.StatusBadRequest, "to address is required")
		return
	}
	if req.From == "" {
		writeError(w, http.StatusBadRequest, "from address is required")
		return
	}
	if req.Subject == "" {
		req.Subject = "SignPost Test Email"
	}
	if req.Body == "" {
		req.Body = "This is a test email from SignPost.\nIf you received this, your mail relay is working correctly."
	}

	// Build the raw message
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@signpost>\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		req.From, req.To, req.Subject,
		time.Now().Format(time.RFC1123Z),
		fmt.Sprintf("%d", time.Now().UnixNano()),
		req.Body,
	)

	// Extract the sender domain to look up relay config
	fromParts := strings.SplitN(req.From, "@", 2)
	var domain *db.Domain
	var relay *db.RelayConfig
	if len(fromParts) == 2 {
		domain, _ = s.db.GetDomainByName(fromParts[1])
		if domain != nil {
			relay, _ = s.db.GetRelayConfig(domain.ID)
		}
	}

	// Route based on auth_method
	if relay != nil && relay.AuthMethod == "login" && relay.Host != nil && *relay.Host != "" {
		// LOGIN relay: DKIM sign in Go, then relay directly
		s.sendViaLoginRelay(w, req.From, req.To, req.Subject, []byte(msg), domain, relay)
		return
	}

	// Default: send via local SMTP (Maddy handles DKIM + relay)
	// Use manual connection to skip TLS verification for self-signed certs on localhost
	smtpPort := envOrDefault("SIGNPOST_SMTP_PORT", "25")
	addr := net.JoinHostPort("127.0.0.1", smtpPort)

	c, dialErr := smtp.Dial(addr)
	if dialErr != nil {
		errStr := dialErr.Error()
		s.db.LogMail(req.From, req.To, nil, req.Subject, "failed", nil, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{"status": "failed", "error": errStr})
		return
	}
	defer c.Close()
	c.Hello("localhost")
	if ok, _ := c.Extension("STARTTLS"); ok {
		c.StartTLS(&tls.Config{InsecureSkipVerify: true})
	}
	c.Mail(req.From)
	c.Rcpt(req.To)
	wc, _ := c.Data()
	wc.Write([]byte(msg))
	err := wc.Close()
	c.Quit()
	if err != nil {
		errStr := err.Error()
		var domainID *int64
		if domain != nil {
			domainID = &domain.ID
		}
		s.db.LogMail(req.From, req.To, domainID, req.Subject, "failed", nil, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	var domainID *int64
	if domain != nil {
		domainID = &domain.ID
	}
	s.db.LogMail(req.From, req.To, domainID, req.Subject, "sent", nil, nil, true)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "sent",
		"message": fmt.Sprintf("Test email sent from %s to %s via Maddy", req.From, req.To),
	})
}

// sendViaLoginRelay DKIM-signs a message in Go and relays directly using LOGIN auth.
func (s *Server) sendViaLoginRelay(w http.ResponseWriter, from, to, subject string, rawMsg []byte, domain *db.Domain, relay *db.RelayConfig) {
	relayHost := *relay.Host
	relayAddr := net.JoinHostPort(relayHost, strconv.Itoa(relay.Port))

	// Step 1: DKIM sign if the domain has a key
	signedMsg := rawMsg
	dkimSigned := false
	if domain.DKIMKeyPath != nil && *domain.DKIMKeyPath != "" {
		var err error
		signedMsg, err = dkim.SignMessage(rawMsg, domain.Name, domain.DKIMSelector, *domain.DKIMKeyPath)
		if err != nil {
			errStr := fmt.Sprintf("DKIM signing failed: %v", err)
			s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  errStr,
			})
			return
		}
		dkimSigned = true
	}

	// Step 2: Connect to relay
	c, err := smtp.Dial(relayAddr)
	if err != nil {
		errStr := fmt.Sprintf("Failed to connect to %s: %v", relayAddr, err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}
	defer c.Close()

	if err := c.Hello("localhost"); err != nil {
		errStr := fmt.Sprintf("EHLO failed: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	// Step 3: STARTTLS if configured
	if relay.StartTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: relayHost}
			if err := c.StartTLS(tlsConfig); err != nil {
				errStr := fmt.Sprintf("STARTTLS failed: %v", err)
				s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
				writeJSON(w, http.StatusOK, map[string]string{
					"status": "failed",
					"error":  errStr,
				})
				return
			}
		}
	}

	// Step 4: AUTH LOGIN
	if relay.Username != nil && *relay.Username != "" && relay.PasswordEnc != nil && *relay.PasswordEnc != "" {
		nonce := ""
		if relay.PasswordNonce != nil {
			nonce = *relay.PasswordNonce
		}
		password, err := s.decryptRelayPassword(*relay.PasswordEnc, nonce)
		if err != nil {
			errStr := fmt.Sprintf("Failed to decrypt relay password: %v", err)
			s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  errStr,
			})
			return
		}
		auth := &loginAuth{username: *relay.Username, password: password}
		if err := c.Auth(auth); err != nil {
			errStr := fmt.Sprintf("LOGIN auth failed: %v", err)
			s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  errStr,
			})
			return
		}
	}

	// Step 5: MAIL FROM, RCPT TO, DATA
	if err := c.Mail(from); err != nil {
		errStr := fmt.Sprintf("MAIL FROM failed: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	if err := c.Rcpt(to); err != nil {
		errStr := fmt.Sprintf("RCPT TO failed: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	wc, err := c.Data()
	if err != nil {
		errStr := fmt.Sprintf("DATA failed: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	if _, err := wc.Write(signedMsg); err != nil {
		wc.Close()
		errStr := fmt.Sprintf("Failed to write message data: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	if err := wc.Close(); err != nil {
		errStr := fmt.Sprintf("Failed to complete DATA: %v", err)
		s.db.LogMail(from, to, &domain.ID, subject, "failed", &relayHost, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	c.Quit()

	s.db.LogMail(from, to, &domain.ID, subject, "sent", &relayHost, nil, dkimSigned)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "sent",
		"message": fmt.Sprintf("Test email sent from %s to %s via %s (LOGIN auth, Go DKIM)", from, to, relayAddr),
	})
}

// handlePublicIP returns the server's public IP address.
func (s *Server) handlePublicIP(w http.ResponseWriter, r *http.Request) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ifconfig.co")
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to detect public IP: %v", err))
		return
	}
	defer resp.Body.Close()

	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))

	if ip == "" {
		writeError(w, http.StatusBadGateway, "Empty response from IP detection service")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"ip": ip})
}

// handleGetTLS returns the current TLS configuration.
func (s *Server) handleGetTLS(w http.ResponseWriter, r *http.Request) {
	tlsConfig, err := s.db.GetTLSConfig()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := map[string]interface{}{
		"mode":      tlsConfig.Mode,
		"cert_path": tlsConfig.CertPath,
		"key_path":  tlsConfig.KeyPath,
	}
	if tlsConfig.CertExpiry != nil {
		resp["cert_expiry"] = tlsConfig.CertExpiry
	}

	// Check if cert file exists and get info
	if tlsConfig.CertPath != nil && *tlsConfig.CertPath != "" {
		if _, err := os.Stat(*tlsConfig.CertPath); err == nil {
			resp["cert_exists"] = true
		} else {
			resp["cert_exists"] = false
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleGenerateSelfSigned generates (or regenerates) a self-signed TLS certificate.
func (s *Server) handleGenerateSelfSigned(w http.ResponseWriter, r *http.Request) {
	if s.dataDir == "" {
		writeError(w, http.StatusInternalServerError, "data directory not configured")
		return
	}

	hostname := s.hostname
	if hostname == "" {
		hostname = "localhost"
	}

	// Delete existing certs to force regeneration
	os.Remove(s.dataDir + "/tls/selfsigned.crt")
	os.Remove(s.dataDir + "/tls/selfsigned.key")

	certPath, keyPath, err := selfsigned.EnsureSelfSignedCert(s.dataDir, hostname)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to generate certificate: %v", err))
		return
	}

	// Update DB
	if err := s.db.UpdateTLSCertPaths(certPath, keyPath); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update TLS config: %v", err))
		return
	}

	// Regenerate Maddy config and reload
	go s.regenerateConfig()

	writeJSON(w, http.StatusOK, map[string]string{
		"status":    "ok",
		"message":   fmt.Sprintf("Self-signed certificate generated for %s", hostname),
		"cert_path": certPath,
		"key_path":  keyPath,
	})
}

// ---------------------------------------------------------------------------
// Domain Config Export / Import
// ---------------------------------------------------------------------------

// domainExportPayload is the JSON structure for a full domain config export.
type domainExportPayload struct {
	SignpostVersion string              `json:"signpost_version"`
	ExportedAt      string              `json:"exported_at"`
	Domain          domainExportDomain  `json:"domain"`
	DKIMKey         string              `json:"dkim_key,omitempty"`
	RelayConfigs    []relayExportConfig `json:"relay_configs"`
}

type domainExportDomain struct {
	Name         string `json:"name"`
	DKIMSelector string `json:"dkim_selector"`
}

type relayExportConfig struct {
	Method     string  `json:"method"`
	Host       *string `json:"host,omitempty"`
	Port       int     `json:"port"`
	Username   *string `json:"username,omitempty"`
	Password   string  `json:"password,omitempty"`
	StartTLS   bool    `json:"starttls"`
	AuthMethod string  `json:"auth_method,omitempty"`
	Active     bool    `json:"active"`
}

// handleExportDomainConfig exports a domain's entire configuration as a JSON file download.
func (s *Server) handleExportDomainConfig(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid domain ID")
		return
	}

	domain, err := s.db.GetDomain(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}

	// Read DKIM private key if it exists
	var dkimKeyPEM string
	if domain.DKIMKeyPath != nil && *domain.DKIMKeyPath != "" {
		keyBytes, err := os.ReadFile(*domain.DKIMKeyPath)
		if err == nil {
			dkimKeyPEM = string(keyBytes)
		}
	}

	// Get all relay configs for this domain
	relayConfigs, err := s.db.ListRelayConfigs(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	exportRelays := make([]relayExportConfig, 0, len(relayConfigs))
	for _, rc := range relayConfigs {
		var password string
		if rc.PasswordEnc != nil && rc.PasswordNonce != nil {
			pw, err := s.decryptRelayPassword(*rc.PasswordEnc, *rc.PasswordNonce)
			if err == nil {
				password = pw
			}
		}
		exportRelays = append(exportRelays, relayExportConfig{
			Method:     rc.Method,
			Host:       rc.Host,
			Port:       rc.Port,
			Username:   rc.Username,
			Password:   password,
			StartTLS:   rc.StartTLS,
			AuthMethod: rc.AuthMethod,
			Active:     rc.Active,
		})
	}

	payload := domainExportPayload{
		SignpostVersion: s.version,
		ExportedAt:      time.Now().UTC().Format(time.RFC3339),
		Domain: domainExportDomain{
			Name:         domain.Name,
			DKIMSelector: domain.DKIMSelector,
		},
		DKIMKey:      dkimKeyPEM,
		RelayConfigs: exportRelays,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encode export data")
		return
	}

	filename := domain.Name + "-signpost-config.json"
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

// handleImportDomainConfig imports a domain config from a JSON file.
// Creates or updates the domain, writes DKIM key, creates/upserts relay configs.
func (s *Server) handleImportDomainConfig(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1MB limit
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var payload domainExportPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	if payload.Domain.Name == "" {
		writeError(w, http.StatusBadRequest, "domain name is required")
		return
	}

	selector := payload.Domain.DKIMSelector
	if selector == "" {
		selector = "signpost"
	}

	// Create or update domain
	domain, err := s.db.GetDomainByName(payload.Domain.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if domain == nil {
		domain, err = s.db.CreateDomain(payload.Domain.Name, selector)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create domain: %v", err))
			return
		}
	}

	// Write DKIM key if provided
	if payload.DKIMKey != "" {
		pemData := []byte(payload.DKIMKey)
		publicKeyDNS, err := dkim.ValidateAndExtractPublicKey(pemData)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid DKIM key: %v", err))
			return
		}

		if err := os.MkdirAll(s.keysDir, 0750); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create keys directory: %v", err))
			return
		}

		keyPath := s.keysDir + "/" + domain.Name + ".key"
		if err := os.WriteFile(keyPath, pemData, 0600); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to write DKIM key: %v", err))
			return
		}

		if err := s.db.UpdateDomainDKIM(domain.ID, keyPath, publicKeyDNS); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Upsert relay configs
	for _, rc := range payload.RelayConfigs {
		var passEnc, passNonce *string
		if rc.Password != "" {
			if s.encKey == nil {
				writeError(w, http.StatusInternalServerError, "encryption key not configured (set SIGNPOST_SECRET_KEY)")
				return
			}
			enc, nonce, err := crypto.Encrypt(s.encKey, rc.Password)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to encrypt relay password")
				return
			}
			passEnc = &enc
			passNonce = &nonce
		}

		if err := s.db.UpsertRelayConfig(
			domain.ID, rc.Method, rc.Host, rc.Port,
			rc.Username, passEnc, passNonce,
			rc.StartTLS, rc.Active,
		); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to upsert relay config: %v", err))
			return
		}
	}

	// Checkpoint and regenerate config
	s.db.Checkpoint()
	go s.regenerateConfig()

	// Re-read domain to get updated fields
	domain, err = s.db.GetDomain(domain.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, domain)
}

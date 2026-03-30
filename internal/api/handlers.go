package api

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/drose-drcs/signpost/internal/db"
	"github.com/drose-drcs/signpost/internal/dkim"
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

	listeners := []map[string]string{
		{"name": "SMTP", "bind": "0.0.0.0:" + smtpPort, "status": maddyStatus},
		{"name": "Submission", "bind": "0.0.0.0:" + submissionPort, "status": maddyStatus},
		{"name": "HTTP API", "bind": "0.0.0.0:" + httpPort, "status": "running"},
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
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

// handleGetRelay returns relay config for a domain.
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

	writeJSON(w, http.StatusOK, rc)
}

// handleUpdateRelay updates relay config for a domain.
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

	// TODO: Encrypt password before storing
	// For now, store plaintext (will be replaced with AES-256-GCM in Phase 3)
	var passEnc, passNonce *string
	if req.Password != nil {
		passEnc = req.Password
		n := "placeholder-nonce"
		passNonce = &n
	}

	if err := s.db.UpsertRelayConfig(id, req.Method, req.Host, req.Port, req.Username, passEnc, passNonce, req.StartTLS); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	go s.regenerateConfig()
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
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
		// TODO: Replace with real decryption when AES-256-GCM is implemented (Phase 3)
		// For now, password_enc is stored as plaintext
		password := *rc.PasswordEnc

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

		auth := smtp.PlainAuth("", *rc.Username, password, *rc.Host)
		if err := c.Auth(auth); err != nil {
			writeJSON(w, http.StatusOK, map[string]string{
				"status": "failed",
				"error":  fmt.Sprintf("Authentication failed: %s", err.Error()),
			})
			return
		}

		c.Quit()
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": fmt.Sprintf("Connected and authenticated to %s", addr),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": fmt.Sprintf("Connected to %s (no credentials to test)", addr),
	})
}

// handleTestSend sends a test email via local SMTP (Maddy).
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

	// Send via local SMTP (Maddy on port 25)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@signpost>\r\n\r\n%s",
		req.From, req.To, req.Subject,
		time.Now().Format(time.RFC1123Z),
		fmt.Sprintf("%d", time.Now().UnixNano()),
		req.Body,
	)

	smtpPort := envOrDefault("SIGNPOST_SMTP_PORT", "25")
	addr := net.JoinHostPort("127.0.0.1", smtpPort)

	err := smtp.SendMail(addr, nil, req.From, []string{req.To}, []byte(msg))
	if err != nil {
		errStr := err.Error()
		s.db.LogMail(req.From, req.To, nil, req.Subject, "failed", nil, &errStr, false)
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "failed",
			"error":  errStr,
		})
		return
	}

	s.db.LogMail(req.From, req.To, nil, req.Subject, "sent", nil, nil, true)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "sent",
		"message": fmt.Sprintf("Test email sent from %s to %s", req.From, req.To),
	})
}

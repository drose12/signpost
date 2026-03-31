package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/drose-drcs/signpost/internal/db"
)

// TemplateData holds all the data needed to render the Maddy config template.
type TemplateData struct {
	GeneratedAt         string
	Hostname            string
	PrimaryDomain       string
	SMTPPort            string
	SubmissionPort      string
	NetworkTrustEnabled bool
	NetworkTrustCIDRs   string
	TLS                 TLSData
	Domains             []DomainData
	SMTPUsers           bool
	HasRelayDomains     bool
	NeedsDirectDelivery bool
}

// TLSData holds TLS-related config for the template.
type TLSData struct {
	Mode         string
	CertPath     string
	KeyPath      string
	ACMEEmail    string
	ACMEProvider string
}

// DomainData holds per-domain config for the template.
type DomainData struct {
	Name         string
	DKIMSelector string
	DKIMKeyPath  string
	Active       bool
	Relay        *RelayData
}

// RelayData holds relay config for a domain.
type RelayData struct {
	Method     string
	Host       string
	Port       int
	Username   string
	Password   string
	StartTLS   bool
	AuthMethod string // "plain" or "login"
}

// Generator creates Maddy configuration files from database state.
type Generator struct {
	templatePath string
	outputPath   string
	dataDir      string
}

// NewGenerator creates a new config generator.
func NewGenerator(templatePath, outputPath, dataDir string) *Generator {
	return &Generator{
		templatePath: templatePath,
		outputPath:   outputPath,
		dataDir:      dataDir,
	}
}

// Generate reads the current state from the database and renders the Maddy config.
// It returns the generated config content.
func (g *Generator) Generate(database *db.DB, decryptPassword func(enc, nonce string) (string, error)) (string, error) {
	data, err := g.buildTemplateData(database, decryptPassword)
	if err != nil {
		return "", fmt.Errorf("building template data: %w", err)
	}

	tmplContent, err := os.ReadFile(g.templatePath)
	if err != nil {
		return "", fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("maddy.conf").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// WriteConfig generates the config and writes it to the output path.
// It preserves the previous config as a .bak file.
func (g *Generator) WriteConfig(database *db.DB, decryptPassword func(enc, nonce string) (string, error)) error {
	content, err := g.Generate(database, decryptPassword)
	if err != nil {
		return err
	}

	// Backup existing config
	if _, statErr := os.Stat(g.outputPath); statErr == nil {
		backupPath := g.outputPath + ".bak"
		existing, readErr := os.ReadFile(g.outputPath)
		if readErr == nil {
			os.WriteFile(backupPath, existing, 0640)
		}
	}

	dir := filepath.Dir(g.outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	if err := os.WriteFile(g.outputPath, []byte(content), 0640); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// RollbackConfig restores the previous config from the .bak file.
func (g *Generator) RollbackConfig() error {
	backupPath := g.outputPath + ".bak"
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("reading backup config: %w", err)
	}
	if err := os.WriteFile(g.outputPath, backup, 0640); err != nil {
		return fmt.Errorf("restoring backup config: %w", err)
	}
	return nil
}

func (g *Generator) buildTemplateData(database *db.DB, decryptPassword func(enc, nonce string) (string, error)) (*TemplateData, error) {
	settings, err := database.GetAllSettings()
	if err != nil {
		return nil, fmt.Errorf("getting settings: %w", err)
	}

	domains, err := database.ListDomains()
	if err != nil {
		return nil, fmt.Errorf("listing domains: %w", err)
	}

	tlsConfig, err := database.GetTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("getting TLS config: %w", err)
	}

	hostname := envOrDefault("SIGNPOST_HOSTNAME", "")
	primaryDomain := envOrDefault("SIGNPOST_DOMAIN", "")
	if hostname == "" && primaryDomain != "" {
		hostname = "mail." + primaryDomain
	}
	if primaryDomain == "" && len(domains) > 0 {
		primaryDomain = domains[0].Name
	}

	data := &TemplateData{
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		Hostname:           hostname,
		PrimaryDomain:      primaryDomain,
		SMTPPort:           getOr(settings, "smtp_port", "25"),
		SubmissionPort:     getOr(settings, "submission_port", "587"),
		NetworkTrustEnabled: getOr(settings, "network_trust_enabled", "true") == "true",
		NetworkTrustCIDRs:  getOr(settings, "network_trust_cidrs", "172.16.0.0/12,127.0.0.1/8"),
	}

	// TLS
	if tlsConfig != nil {
		data.TLS = TLSData{Mode: tlsConfig.Mode}
		if tlsConfig.CertPath != nil {
			data.TLS.CertPath = *tlsConfig.CertPath
		}
		if tlsConfig.KeyPath != nil {
			data.TLS.KeyPath = *tlsConfig.KeyPath
		}
		if tlsConfig.ACMEEmail != nil {
			data.TLS.ACMEEmail = *tlsConfig.ACMEEmail
		}
		if tlsConfig.ACMEProvider != nil {
			data.TLS.ACMEProvider = *tlsConfig.ACMEProvider
		}
	}

	// Domains with relay configs
	hasRelayDomains := false
	needsDirectDelivery := false
	for _, d := range domains {
		dd := DomainData{
			Name:         d.Name,
			DKIMSelector: d.DKIMSelector,
			Active:       d.Active,
		}
		if d.DKIMKeyPath != nil {
			dd.DKIMKeyPath = *d.DKIMKeyPath
		}

		rc, err := database.GetRelayConfig(d.ID)
		if err != nil {
			return nil, fmt.Errorf("getting relay config for %s: %w", d.Name, err)
		}
		if rc != nil && rc.Method != "direct" {
			rd := &RelayData{
				Method:     rc.Method,
				Port:       rc.Port,
				StartTLS:   rc.StartTLS,
				AuthMethod: rc.AuthMethod,
			}
			if rc.Host != nil {
				rd.Host = *rc.Host
			}
			if rc.Username != nil {
				rd.Username = *rc.Username
			}
			if rc.PasswordEnc != nil && rc.PasswordNonce != nil && decryptPassword != nil {
				pw, err := decryptPassword(*rc.PasswordEnc, *rc.PasswordNonce)
				if err != nil {
					return nil, fmt.Errorf("decrypting relay password for %s: %w", d.Name, err)
				}
				rd.Password = pw
			}

			// LOGIN auth relays are handled by Go directly, not Maddy.
			// In Maddy config, treat them like direct delivery domains.
			if rc.AuthMethod == "login" {
				dd.Relay = nil // no Maddy relay target
				if d.Active {
					needsDirectDelivery = true
				}
			} else {
				dd.Relay = rd
				hasRelayDomains = true
			}
		} else if d.Active {
			needsDirectDelivery = true
		}

		data.Domains = append(data.Domains, dd)
	}
	data.HasRelayDomains = hasRelayDomains
	data.NeedsDirectDelivery = needsDirectDelivery

	// Check if any SMTP users exist
	var userCount int
	err = database.QueryRow(`SELECT COUNT(*) FROM smtp_users WHERE active = 1`).Scan(&userCount)
	if err != nil {
		return nil, fmt.Errorf("counting SMTP users: %w", err)
	}
	data.SMTPUsers = userCount > 0

	return data, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getOr(m map[string]string, key, def string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return def
}

// FormatNetworkCIDRs converts a comma-separated CIDR string to the format Maddy expects.
func FormatNetworkCIDRs(cidrs string) string {
	parts := strings.Split(cidrs, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return strings.Join(parts, " ")
}

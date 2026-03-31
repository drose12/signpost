package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/drose-drcs/signpost/internal/db"
	"github.com/drose-drcs/signpost/internal/dkim"
)

// dnsCheckRecord represents a single DNS record comparison between current and recommended.
type dnsCheckRecord struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	Purpose     string  `json:"purpose"`
	Current     *string `json:"current"`
	Recommended string  `json:"recommended"`
	Status      string  `json:"status"`  // ok, missing, update, conflict
	Message     string  `json:"message"`
}

// dnsLookupFunc abstracts DNS TXT lookups for testing.
type dnsLookupFunc func(name string) ([]string, error)

// defaultLookupTXT uses a custom resolver to bypass Docker's DNS cache.
// Queries Cloudflare (1.1.1.1) and Google (8.8.8.8) DNS directly.
func defaultLookupTXT(name string) ([]string, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", "1.1.1.1:53")
		},
	}
	return resolver.LookupTXT(context.Background(), name)
}

// handleDNSCheck performs live DNS lookups and compares against recommended records.
func (s *Server) handleDNSCheck(w http.ResponseWriter, r *http.Request) {
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

	// Load relay config (may be nil if none configured)
	relay, err := s.db.GetRelayConfig(domain.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	records := performDNSCheck(domain.Name, domain.DKIMSelector, domain.DKIMPublicDNS, relay, defaultLookupTXT)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": records,
	})
}

// performDNSCheck runs the actual DNS comparison logic. It accepts a lookup function
// so tests can inject a fake resolver.
func performDNSCheck(domainName, dkimSelector string, dkimPublicDNS *string, relay *db.RelayConfig, lookupTXT dnsLookupFunc) []dnsCheckRecord {
	var records []dnsCheckRecord

	// SPF check
	records = append(records, checkSPF(domainName, relay, lookupTXT))

	// DKIM check
	if dkimPublicDNS != nil {
		records = append(records, checkDKIM(domainName, dkimSelector, *dkimPublicDNS, lookupTXT))
	} else {
		records = append(records, dnsCheckRecord{
			Type:        "TXT",
			Name:        dkim.DNSRecordName(dkimSelector, domainName),
			Purpose:     "dkim",
			Recommended: "",
			Status:      "missing",
			Message:     "Generate DKIM keys first",
		})
	}

	// DMARC check
	records = append(records, checkDMARC(domainName, lookupTXT))

	return records
}

// checkSPF checks the SPF record for a domain.
func checkSPF(domainName string, relay *db.RelayConfig, lookupTXT dnsLookupFunc) dnsCheckRecord {
	hostname := "mail." + domainName
	recommended := dkim.RecommendedSPF(hostname)

	// Determine what SPF mechanism we need based on relay config
	neededMechanism := spfMechanismForRelay(relay)

	txts, err := lookupTXT(domainName)
	if err != nil || len(txts) == 0 {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        domainName,
			Purpose:     "spf",
			Recommended: recommended,
			Status:      "missing",
			Message:     "No SPF record found. Add the recommended record to authorize your mail server.",
		}
	}

	// Find existing SPF record
	var existingSPF string
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=spf1") {
			existingSPF = txt
			break
		}
	}

	if existingSPF == "" {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        domainName,
			Purpose:     "spf",
			Recommended: recommended,
			Status:      "missing",
			Message:     "No SPF record found among TXT records. Add the recommended record.",
		}
	}

	current := existingSPF

	// Check if the needed mechanism is already present
	if neededMechanism != "" && strings.Contains(existingSPF, neededMechanism) {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        domainName,
			Purpose:     "spf",
			Current:     &current,
			Recommended: recommended,
			Status:      "ok",
			Message:     "Existing SPF already includes your relay's sending servers",
		}
	}

	if neededMechanism == "" {
		// No specific mechanism needed (direct delivery or no relay)
		// Just check that a basic SPF exists
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        domainName,
			Purpose:     "spf",
			Current:     &current,
			Recommended: recommended,
			Status:      "ok",
			Message:     "SPF record exists",
		}
	}

	// SPF exists but doesn't contain the needed mechanism — suggest merge
	merged := mergeSPF(existingSPF, neededMechanism)
	return dnsCheckRecord{
		Type:        "TXT",
		Name:        domainName,
		Purpose:     "spf",
		Current:     &current,
		Recommended: merged,
		Status:      "update",
		Message:     fmt.Sprintf("SPF record exists but does not include %s. Update to the recommended value.", neededMechanism),
	}
}

// checkDKIM checks the DKIM TXT record for a domain.
func checkDKIM(domainName, selector, expectedDNS string, lookupTXT dnsLookupFunc) dnsCheckRecord {
	recordName := dkim.DNSRecordName(selector, domainName)

	txts, err := lookupTXT(recordName)
	if err != nil || len(txts) == 0 {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        recordName,
			Purpose:     "dkim",
			Recommended: expectedDNS,
			Status:      "missing",
			Message:     "DKIM record not found. Add the recommended TXT record to enable email signing verification.",
		}
	}

	// Concatenate all TXT strings (DNS splits long records into chunks)
	current := strings.Join(txts, "")

	// Normalize for comparison: strip spaces
	normalizedCurrent := strings.ReplaceAll(current, " ", "")
	normalizedExpected := strings.ReplaceAll(expectedDNS, " ", "")

	if normalizedCurrent == normalizedExpected {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        recordName,
			Purpose:     "dkim",
			Current:     &current,
			Recommended: expectedDNS,
			Status:      "ok",
			Message:     "DKIM record matches",
		}
	}

	return dnsCheckRecord{
		Type:        "TXT",
		Name:        recordName,
		Purpose:     "dkim",
		Current:     &current,
		Recommended: expectedDNS,
		Status:      "conflict",
		Message:     "DKIM record exists but does not match the generated key. Update to the recommended value.",
	}
}

// checkDMARC checks the DMARC TXT record for a domain.
func checkDMARC(domainName string, lookupTXT dnsLookupFunc) dnsCheckRecord {
	recordName := dkim.DMARCRecordName(domainName)
	recommended := dkim.RecommendedDMARC(domainName)

	txts, err := lookupTXT(recordName)
	if err != nil || len(txts) == 0 {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        recordName,
			Purpose:     "dmarc",
			Recommended: recommended,
			Status:      "missing",
			Message:     "No DMARC record found. Add the recommended record to define your email authentication policy.",
		}
	}

	// Find DMARC record
	var existingDMARC string
	for _, txt := range txts {
		if strings.HasPrefix(txt, "v=DMARC1") {
			existingDMARC = txt
			break
		}
	}

	if existingDMARC == "" {
		return dnsCheckRecord{
			Type:        "TXT",
			Name:        recordName,
			Purpose:     "dmarc",
			Recommended: recommended,
			Status:      "missing",
			Message:     "No DMARC record found among TXT records. Add the recommended record.",
		}
	}

	return dnsCheckRecord{
		Type:        "TXT",
		Name:        recordName,
		Purpose:     "dmarc",
		Current:     &existingDMARC,
		Recommended: recommended,
		Status:      "ok",
		Message:     "DMARC record exists",
	}
}

// spfMechanismForRelay returns the SPF mechanism needed for the configured relay.
// For ISP/custom relays, it checks whether the host publishes an SPF record.
// If not, it resolves the host to an IP and returns ip4: instead of include:.
func spfMechanismForRelay(relay *db.RelayConfig) string {
	if relay == nil {
		return ""
	}
	switch relay.Method {
	case "gmail":
		return "include:_spf.google.com"
	case "custom", "isp":
		if relay.Host == nil || *relay.Host == "" {
			return ""
		}
		host := *relay.Host
		// Check if the host has an SPF record (include: requires one)
		txts, err := defaultLookupTXT(host)
		if err == nil {
			for _, txt := range txts {
				if strings.HasPrefix(txt, "v=spf1") {
					return "include:" + host
				}
			}
		}
		// No SPF record on host — resolve to IP and use ip4:
		ips, err := net.LookupHost(host)
		if err == nil && len(ips) > 0 {
			return "ip4:" + ips[0]
		}
		// Fallback to include: even though it may not work
		return "include:" + host
	default:
		return ""
	}
}

// mergeSPF inserts a mechanism into an existing SPF record before the ~all or -all qualifier.
func mergeSPF(existing, mechanism string) string {
	// Find ~all or -all at the end
	for _, suffix := range []string{" ~all", " -all", " ?all", " +all"} {
		if strings.HasSuffix(existing, suffix) {
			return strings.TrimSuffix(existing, suffix) + " " + mechanism + suffix
		}
	}
	// No all qualifier found — append mechanism
	return existing + " " + mechanism
}

package api

import (
	"context"
	"encoding/binary"
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
	TTL         *int    `json:"ttl,omitempty"` // TTL in seconds, if detected
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

// lookupTXTTTL returns the TTL (in seconds) for TXT records at the given name.
// Uses a raw DNS query to Cloudflare 1.1.1.1. Returns 0 if lookup fails.
func lookupTXTTTL(name string) int {
	// Build a minimal DNS query for TXT records
	// DNS header (12 bytes) + question section
	txID := uint16(0x1234)
	flags := uint16(0x0100) // standard query, recursion desired
	qdCount := uint16(1)

	var query []byte
	// Header
	query = binary.BigEndian.AppendUint16(query, txID)
	query = binary.BigEndian.AppendUint16(query, flags)
	query = binary.BigEndian.AppendUint16(query, qdCount)
	query = binary.BigEndian.AppendUint16(query, 0) // ancount
	query = binary.BigEndian.AppendUint16(query, 0) // nscount
	query = binary.BigEndian.AppendUint16(query, 0) // arcount

	// Question: encode domain name
	for _, label := range strings.Split(name, ".") {
		query = append(query, byte(len(label)))
		query = append(query, []byte(label)...)
	}
	query = append(query, 0) // root label
	query = binary.BigEndian.AppendUint16(query, 16) // TXT type
	query = binary.BigEndian.AppendUint16(query, 1)  // IN class

	// Send UDP query
	conn, err := net.DialTimeout("udp", "1.1.1.1:53", 3*time.Second)
	if err != nil {
		return 0
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.Write(query); err != nil {
		return 0
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil || n < 12 {
		return 0
	}

	// Parse answer count from header
	anCount := binary.BigEndian.Uint16(buf[6:8])
	if anCount == 0 {
		return 0
	}

	// Skip header (12 bytes) and question section
	offset := 12
	// Skip question name
	for offset < n {
		if buf[offset] == 0 {
			offset++ // root label
			break
		}
		if buf[offset]&0xC0 == 0xC0 {
			offset += 2 // pointer
			break
		}
		offset += int(buf[offset]) + 1
	}
	offset += 4 // skip qtype + qclass

	// Parse first answer to get TTL
	if offset >= n {
		return 0
	}
	// Skip answer name (may be pointer)
	if buf[offset]&0xC0 == 0xC0 {
		offset += 2
	} else {
		for offset < n && buf[offset] != 0 {
			offset += int(buf[offset]) + 1
		}
		offset++ // root
	}
	if offset+10 > n {
		return 0
	}
	// offset now at: type(2) + class(2) + TTL(4) + rdlength(2)
	ttl := binary.BigEndian.Uint32(buf[offset+4 : offset+8])
	return int(ttl)
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

	// Get egress host setting for direct delivery SPF
	egressHost, _ := s.db.GetSetting("egress_host")

	records := performDNSCheck(domain.Name, domain.DKIMSelector, domain.DKIMPublicDNS, relay, egressHost, defaultLookupTXT)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"records": records,
	})
}

// performDNSCheck runs the actual DNS comparison logic. It accepts a lookup function
// so tests can inject a fake resolver.
func performDNSCheck(domainName, dkimSelector string, dkimPublicDNS *string, relay *db.RelayConfig, egressHost string, lookupTXT dnsLookupFunc) []dnsCheckRecord {
	var records []dnsCheckRecord

	// SPF check
	records = append(records, checkSPF(domainName, relay, egressHost, lookupTXT))

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

	// Populate TTLs for each record
	for i := range records {
		if records[i].Current != nil {
			ttl := lookupTXTTTL(records[i].Name)
			if ttl > 0 {
				records[i].TTL = &ttl
			}
		}
	}

	return records
}

// checkSPF checks the SPF record for a domain.
func checkSPF(domainName string, relay *db.RelayConfig, egressHost string, lookupTXT dnsLookupFunc) dnsCheckRecord {
	hostname := "mail." + domainName
	recommended := dkim.RecommendedSPF(hostname)

	// Determine what SPF mechanism we need based on relay config
	neededMechanism := spfMechanismForRelay(relay, egressHost)

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

	// Check for broken include: entries (hosts with no SPF record)
	brokenIncludes := findBrokenIncludes(existingSPF, lookupTXT)

	// Check if the needed mechanism is already present
	mechanismPresent := neededMechanism != "" && strings.Contains(existingSPF, neededMechanism)

	if mechanismPresent && len(brokenIncludes) == 0 {
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

	if neededMechanism == "" && len(brokenIncludes) == 0 {
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

	// Build the recommended SPF by fixing issues
	fixedSPF := existingSPF
	var issues []string

	// Remove broken includes
	for _, broken := range brokenIncludes {
		fixedSPF = strings.Replace(fixedSPF, " include:"+broken, "", 1)
		issues = append(issues, fmt.Sprintf("remove include:%s (no SPF record, causes permerror)", broken))
	}

	// Add missing mechanism
	if neededMechanism != "" && !strings.Contains(fixedSPF, neededMechanism) {
		fixedSPF = mergeSPF(fixedSPF, neededMechanism)
		issues = append(issues, fmt.Sprintf("add %s", neededMechanism))
	}

	// Clean up any double spaces from removals
	for strings.Contains(fixedSPF, "  ") {
		fixedSPF = strings.Replace(fixedSPF, "  ", " ", -1)
	}

	return dnsCheckRecord{
		Type:        "TXT",
		Name:        domainName,
		Purpose:     "spf",
		Current:     &current,
		Recommended: fixedSPF,
		Status:      "update",
		Message:     "SPF needs updating: " + strings.Join(issues, "; "),
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
// egressHost is the user-configured FQDN or empty string.
func spfMechanismForRelay(relay *db.RelayConfig, egressHost string) string {
	if relay == nil || relay.Method == "direct" || relay.Method == "" {
		return egressMechanism(egressHost)
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
		return egressMechanism(egressHost)
	}
}

// egressMechanism returns the SPF mechanism for direct delivery.
// Uses the configured egress FQDN if set, otherwise detects public IP.
func egressMechanism(egressHost string) string {
	if egressHost != "" {
		return "a:" + egressHost
	}
	// Try to detect public IP
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://ifconfig.co")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))
	if ip != "" {
		return "ip4:" + ip
	}
	return ""
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

// findBrokenIncludes checks each include: in an SPF record and returns any that
// don't have a valid SPF record (which causes permerror on SPF evaluation).
func findBrokenIncludes(spf string, lookupTXT dnsLookupFunc) []string {
	var broken []string
	parts := strings.Fields(spf)
	for _, part := range parts {
		if !strings.HasPrefix(part, "include:") {
			continue
		}
		host := strings.TrimPrefix(part, "include:")
		txts, err := lookupTXT(host)
		if err != nil {
			broken = append(broken, host)
			continue
		}
		hasSPF := false
		for _, txt := range txts {
			if strings.HasPrefix(txt, "v=spf1") {
				hasSPF = true
				break
			}
		}
		if !hasSPF {
			broken = append(broken, host)
		}
	}
	return broken
}

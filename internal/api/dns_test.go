package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/drose-drcs/signpost/internal/db"
)

// fakeLookupTXT returns a mock DNS resolver that uses a map of name → records.
func fakeLookupTXT(records map[string][]string) dnsLookupFunc {
	return func(name string) ([]string, error) {
		if recs, ok := records[name]; ok {
			return recs, nil
		}
		return nil, fmt.Errorf("no such host")
	}
}

func strPtr(s string) *string {
	return &s
}

func TestPerformDNSCheck_NoRecords(t *testing.T) {
	// All lookups fail — everything should be missing
	lookup := fakeLookupTXT(map[string][]string{})

	records := performDNSCheck("example.com", "signpost", nil, nil, "", lookup)

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// SPF
	if records[0].Purpose != "spf" {
		t.Errorf("expected first record purpose 'spf', got %q", records[0].Purpose)
	}
	if records[0].Status != "missing" {
		t.Errorf("expected SPF status 'missing', got %q", records[0].Status)
	}

	// DKIM — no key generated
	if records[1].Purpose != "dkim" {
		t.Errorf("expected second record purpose 'dkim', got %q", records[1].Purpose)
	}
	if records[1].Status != "missing" {
		t.Errorf("expected DKIM status 'missing', got %q", records[1].Status)
	}
	if records[1].Message != "Generate DKIM keys first" {
		t.Errorf("expected DKIM message about generating keys, got %q", records[1].Message)
	}

	// DMARC
	if records[2].Purpose != "dmarc" {
		t.Errorf("expected third record purpose 'dmarc', got %q", records[2].Purpose)
	}
	if records[2].Status != "missing" {
		t.Errorf("expected DMARC status 'missing', got %q", records[2].Status)
	}
}

func TestPerformDNSCheck_AllPresent(t *testing.T) {
	dkimPub := "v=DKIM1; k=rsa; p=MIIBIjANBg..."
	host := "smtp.gmail.com"
	relay := &db.RelayConfig{Method: "gmail", Host: &host}
	lookup := fakeLookupTXT(map[string][]string{
		"example.com":                        {"v=spf1 include:_spf.google.com ~all"},
		"_spf.google.com":                    {"v=spf1 ip4:209.85.128.0/17 ~all"},
		"signpost._domainkey.example.com":    {dkimPub},
		"_dmarc.example.com":                 {"v=DMARC1; p=quarantine; ruf=mailto:postmaster@example.com; fo=1"},
	})

	records := performDNSCheck("example.com", "signpost", &dkimPub, relay, "", lookup)

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	for _, rec := range records {
		if rec.Status != "ok" {
			t.Errorf("expected status 'ok' for %s, got %q: %s", rec.Purpose, rec.Status, rec.Message)
		}
		if rec.Current == nil {
			t.Errorf("expected non-nil current for %s", rec.Purpose)
		}
	}
}

func TestPerformDNSCheck_SPFUpdateNeeded(t *testing.T) {
	// Domain has gmail relay but SPF doesn't include Google
	host := "smtp.gmail.com"
	relay := &db.RelayConfig{
		Method: "gmail",
		Host:   &host,
	}

	lookup := fakeLookupTXT(map[string][]string{
		"example.com":        {"v=spf1 mx ~all"},
		"_dmarc.example.com": {"v=DMARC1; p=none"},
	})

	records := performDNSCheck("example.com", "signpost", nil, relay, "", lookup)

	spf := records[0]
	if spf.Status != "update" {
		t.Errorf("expected SPF status 'update', got %q", spf.Status)
	}
	if spf.Recommended != "v=spf1 mx include:_spf.google.com ~all" {
		t.Errorf("unexpected merged SPF: %q", spf.Recommended)
	}
}

func TestPerformDNSCheck_SPFAlreadyIncludesGmail(t *testing.T) {
	host := "smtp.gmail.com"
	relay := &db.RelayConfig{
		Method: "gmail",
		Host:   &host,
	}

	lookup := fakeLookupTXT(map[string][]string{
		"example.com":        {"v=spf1 include:_spf.google.com ~all"},
		"_spf.google.com":    {"v=spf1 ip4:209.85.128.0/17 ~all"},
		"_dmarc.example.com": {"v=DMARC1; p=none"},
	})

	records := performDNSCheck("example.com", "signpost", nil, relay, "", lookup)

	spf := records[0]
	if spf.Status != "ok" {
		t.Errorf("expected SPF status 'ok', got %q", spf.Status)
	}
}

func TestPerformDNSCheck_DKIMConflict(t *testing.T) {
	expectedDKIM := "v=DKIM1; k=rsa; p=NEWKEY..."
	lookup := fakeLookupTXT(map[string][]string{
		"example.com":                     {"v=spf1 ~all"},
		"signpost._domainkey.example.com": {"v=DKIM1; k=rsa; p=OLDKEY..."},
		"_dmarc.example.com":              {"v=DMARC1; p=none"},
	})

	records := performDNSCheck("example.com", "signpost", &expectedDKIM, nil, "", lookup)

	dkimRec := records[1]
	if dkimRec.Status != "conflict" {
		t.Errorf("expected DKIM status 'conflict', got %q", dkimRec.Status)
	}
}

func TestPerformDNSCheck_CustomRelay(t *testing.T) {
	host := "smtp.bellmts.net"
	relay := &db.RelayConfig{
		Method: "isp",
		Host:   &host,
	}

	lookup := fakeLookupTXT(map[string][]string{
		"example.com":        {"v=spf1 mx -all"},
		"_dmarc.example.com": {"v=DMARC1; p=reject"},
	})

	records := performDNSCheck("example.com", "signpost", nil, relay, "", lookup)

	spf := records[0]
	if spf.Status != "update" {
		t.Errorf("expected SPF status 'update', got %q", spf.Status)
	}
	if spf.Recommended != "v=spf1 mx include:smtp.bellmts.net -all" {
		t.Errorf("unexpected merged SPF: %q", spf.Recommended)
	}
}

func TestMergeSPF(t *testing.T) {
	tests := []struct {
		existing  string
		mechanism string
		want      string
	}{
		{
			existing:  "v=spf1 mx ~all",
			mechanism: "include:_spf.google.com",
			want:      "v=spf1 mx include:_spf.google.com ~all",
		},
		{
			existing:  "v=spf1 include:existing.com -all",
			mechanism: "include:new.com",
			want:      "v=spf1 include:existing.com include:new.com -all",
		},
		{
			existing:  "v=spf1 mx",
			mechanism: "include:relay.example.com",
			want:      "v=spf1 mx include:relay.example.com",
		},
		{
			existing:  "v=spf1 ?all",
			mechanism: "include:test.com",
			want:      "v=spf1 include:test.com ?all",
		},
	}

	for _, tt := range tests {
		got := mergeSPF(tt.existing, tt.mechanism)
		if got != tt.want {
			t.Errorf("mergeSPF(%q, %q) = %q, want %q", tt.existing, tt.mechanism, got, tt.want)
		}
	}
}

func TestSPFMechanismForRelay(t *testing.T) {
	tests := []struct {
		name       string
		relay      *db.RelayConfig
		egressHost string
		wantExact  string // exact match, or empty to skip exact check
		wantPrefix string // prefix match, or empty to skip
	}{
		{"nil relay no egress", nil, "", "", "ip4:"}, // auto-detects public IP
		{"nil relay with egress", nil, "myhost.dyndns.org", "a:myhost.dyndns.org", ""},
		{"direct no egress", &db.RelayConfig{Method: "direct"}, "", "", "ip4:"},
		{"direct with egress", &db.RelayConfig{Method: "direct"}, "home.example.com", "a:home.example.com", ""},
		{"gmail", &db.RelayConfig{Method: "gmail"}, "", "include:_spf.google.com", ""},
		{"isp with host", &db.RelayConfig{Method: "isp", Host: strPtr("smtp.bellmts.net")}, "", "include:smtp.bellmts.net", ""},
		{"custom with host", &db.RelayConfig{Method: "custom", Host: strPtr("relay.example.com")}, "", "include:relay.example.com", ""},
		{"isp no host", &db.RelayConfig{Method: "isp"}, "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := spfMechanismForRelay(tt.relay, tt.egressHost)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("spfMechanismForRelay(%s) = %q, want %q", tt.name, got, tt.wantExact)
			}
			if tt.wantPrefix != "" && !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("spfMechanismForRelay(%s) = %q, want prefix %q", tt.name, got, tt.wantPrefix)
			}
		})
	}
}

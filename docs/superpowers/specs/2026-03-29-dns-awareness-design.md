# DNS Awareness Feature

**Date:** 2026-03-29
**Status:** Approved
**Author:** drose + Claude

## Overview

Add DNS lookup capability so SignPost can show users what DNS records currently exist for their domain alongside what's recommended. This prevents users from accidentally overwriting existing records (especially SPF) and provides clear guidance on what needs to be added, merged, or left alone.

## Backend

### New endpoint: `GET /api/v1/domains/{id}/dns/check`

Performs live DNS lookups using Go's `net.LookupTXT` for the domain's SPF, DKIM, and DMARC record names. Returns current vs recommended comparison with merge analysis.

**Response shape:**

```json
{
  "records": [
    {
      "type": "TXT",
      "name": "drcs.ca",
      "purpose": "spf",
      "current": "v=spf1 include:_spf.google.com ~all",
      "recommended": "v=spf1 include:_spf.google.com ~all",
      "status": "ok",
      "message": "Existing SPF already includes your relay's sending servers"
    },
    {
      "type": "TXT",
      "name": "signpost._domainkey.drcs.ca",
      "purpose": "dkim",
      "current": null,
      "recommended": "v=DKIM1; k=rsa; p=MIIBIjAN...",
      "status": "missing",
      "message": "DKIM record needs to be added"
    },
    {
      "type": "TXT",
      "name": "_dmarc.drcs.ca",
      "purpose": "dmarc",
      "current": "v=DMARC1; p=none; rua=mailto:dmarc@drcs.ca",
      "recommended": "v=DMARC1; p=none; rua=mailto:dmarc@drcs.ca",
      "status": "ok",
      "message": "DMARC policy already configured"
    }
  ]
}
```

### Status values

- `ok` — no action needed, existing record is sufficient
- `missing` — record does not exist, needs to be added
- `update` — record exists but needs modification (e.g., SPF merge)
- `conflict` — existing record contradicts what SignPost needs (e.g., different DKIM key for same selector)

### SPF merge logic

1. Look up existing TXT records for the domain, find the one starting with `v=spf1`
2. Determine what SPF mechanism SignPost needs based on the domain's relay config:
   - Gmail relay → check for `include:_spf.google.com`
   - Custom relay → check for relay host's IP or include
   - Direct delivery → check for server's IP (`ip4:`)
   - No relay configured → recommend generic SPF with `~all`
3. If the existing SPF already includes the needed mechanism → status `ok`, message explains why
4. If the existing SPF exists but doesn't include the mechanism → status `update`, recommended field shows the merged record (insert new mechanism before the `~all`/`-all` terminator)
5. If no SPF record exists → status `missing`, recommended shows full new record

### DKIM lookup

- Query TXT records for `{selector}._domainkey.{domain}`
- If not found → status `missing`, recommended shows the public key DNS value
- If found and matches current key → status `ok`
- If found but different key → status `conflict`, message warns about mismatch

### DMARC lookup

- Query TXT records for `_dmarc.{domain}`
- If not found → status `missing`, recommended shows starter policy (`v=DMARC1; p=none;`)
- If found → status `ok`, message confirms policy exists
- No merge logic needed — DMARC is user policy, SignPost doesn't dictate it

### Implementation details

- Uses Go stdlib `net.LookupTXT` — no external DNS library needed
- DNS lookups have a natural timeout (Go's resolver default) — no explicit timeout needed
- The endpoint requires the domain to have DKIM keys generated (otherwise there's nothing to recommend for DKIM)
- Relay config is loaded from DB to inform SPF analysis
- This endpoint does NOT require authentication bypass — same basic auth as all other endpoints

## Frontend

### DNS Records tab (Domains page) and Wizard step 3

Both locations replace the current static DNS records table with a richer comparison view.

**Table columns:** Record (purpose badge), Current Value, Recommended Value, Status (badge), Action (copy button)

**Status badges:**
- `ok` → green badge, "No change needed" in recommended column
- `missing` → amber badge, recommended value shown with copy button
- `update` → amber badge, recommended shows merged value with copy button, current shown for reference
- `conflict` → red badge, warning message, recommended shown with copy button

**Additional UI:**
- "Check DNS" button to trigger/refresh the lookup (replaces the disabled "Validate DNS" button)
- Loading state while DNS lookup is in progress (can take 1-2 seconds)
- Info message explaining each status
- Copy button only on rows that need action (`missing`, `update`, `conflict`)

**Wizard step 3 specifics:**
- Auto-triggers DNS check on step entry
- "Skip for now" still available (DNS propagation takes time)
- After user says they've updated DNS, "Re-check" button to verify

### TypeScript types

```typescript
interface DNSCheckRecord {
  type: string;
  name: string;
  purpose: string; // spf, dkim, dmarc
  current: string | null;
  recommended: string;
  status: 'ok' | 'missing' | 'update' | 'conflict';
  message: string;
}

interface DNSCheckResponse {
  records: DNSCheckRecord[];
}
```

## Scope

**In scope:**
- DNS lookup via Go stdlib
- SPF merge logic (detect existing, merge if needed)
- DKIM existence check
- DMARC existence check
- Current vs recommended UI in both DNS tab and Wizard
- Copy buttons for actionable records only

**Out of scope:**
- Writing DNS records (Cloudflare API integration — future phase)
- Monitoring/alerting on DNS changes
- DNS propagation checking (polling until records appear)
- MX record checking

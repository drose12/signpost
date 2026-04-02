# SignPost Security Procedures

## Overview

This document defines the security checkup procedures for SignPost. Follow these before every release, monthly for maintenance, and immediately when security advisories are published.

## Automated Security (Always Running)

These run automatically via GitHub:

| Tool | What it does | Frequency |
|------|-------------|-----------|
| **Dependabot alerts** | Scans Go and npm dependencies for known CVEs | On every push + daily |
| **Dependabot security updates** | Auto-creates PRs for vulnerable dependencies | When CVE is found |
| **CodeQL (Go)** | Static analysis for SQL injection, command injection, path traversal, etc. | On push to main + weekly |
| **CodeQL (TypeScript)** | Static analysis for XSS, prototype pollution, insecure patterns | On push to main + weekly |

**Check results at:** https://github.com/drose12/signpost/security

## Manual Security Checkup Procedure

### When to run
- Before every version release
- Monthly maintenance
- After adding new dependencies
- After changes to auth, crypto, or API handlers

### Step 1: Dependency Audit

```bash
# Go dependencies — check for known vulnerabilities
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# npm dependencies
cd web && npm audit

# Check Dependabot alerts
gh api repos/drose12/signpost/dependabot/alerts --jq '.[] | {severity: .security_advisory.severity, package: .security_vulnerability.package.name, summary: .security_advisory.summary}'
```

**Action:** Update any vulnerable packages. Run full test suite after updating.

### Step 2: Secret Scanning (Manual)

GitHub secret scanning isn't available on free private repos. Run this locally:

```bash
# Check for hardcoded secrets in current code
grep -rn "password\|secret\|token\|apikey" --include="*.go" --include="*.ts" --include="*.tsx" --include="*.yml" | grep -v "_test\.\|test_\|Test\|example\|Example\|placeholder\|TODO\|FIXME\|node_modules"

# Check for real credentials (customize patterns for your setup)
grep -rn "vhjo\|dArN\|CDE#" --include="*.go" --include="*.ts" --include="*.tsx" --include="*.md" --include="*.yml"

# Check git history for secrets
git log --all -p | grep -c "BEGIN PRIVATE KEY\|password.*=.*['\"]"

# Check for .env files tracked
git ls-files | grep -i "\.env$"

# Check for key/cert files tracked
git ls-files | grep -i "\.pem\|\.key\|\.crt"
```

**Action:** If secrets found, rotate them immediately and use `git filter-repo` to scrub history.

### Step 3: Code Review Checklist

Review these areas for security issues:

**Authentication & Authorization:**
- [ ] All API endpoints (except `/healthz`) require basic auth
- [ ] Admin password is not logged or exposed in API responses
- [ ] SMTP user passwords are bcrypt-hashed (with `bcrypt:` prefix for Maddy)
- [ ] Relay passwords are AES-256-GCM encrypted at rest

**Input Validation:**
- [ ] All user input is validated before use (domain names, email addresses, ports)
- [ ] SQL queries use parameterized statements (no string concatenation)
- [ ] File paths are validated (no path traversal in DKIM key import)
- [ ] Request body size is limited (e.g., DKIM import uses `MaxBytesReader`)

**Cryptography:**
- [ ] Encryption key derived via HKDF-SHA256 (not raw key)
- [ ] Unique nonce generated per encryption operation
- [ ] Passwords hashed with bcrypt at default cost
- [ ] Self-signed TLS cert uses RSA-2048 minimum

**Information Exposure:**
- [ ] Password fields use `json:"-"` in Go models
- [ ] Error messages don't leak internal paths or stack traces
- [ ] SMTP banner doesn't expose software version
- [ ] API responses don't include sensitive fields unintentionally

**Frontend:**
- [ ] Auth credentials stored in memory only (not localStorage)
- [ ] 401 responses clear credentials and re-prompt login
- [ ] No user input rendered as raw HTML (React handles this)
- [ ] API calls use proper Content-Type headers

### Step 4: Dependency Freshness

```bash
# Check Go module versions
go list -m -u all 2>/dev/null | grep '\[' | head -20

# Check npm outdated
cd web && npm outdated

# Check Docker base image versions
grep "^FROM" Dockerfile
```

**Action:** Update dependencies that have security-relevant patches. Non-security updates can wait for a scheduled maintenance window.

### Step 5: Docker Image Scan

```bash
# Build the image
docker build -t signpost:scan .

# Scan with Trivy (install: brew install trivy / apt install trivy)
trivy image signpost:scan

# Or use Docker Scout (if available)
docker scout cves signpost:scan
```

**Action:** Address critical and high severity CVEs. Medium/low can be tracked as issues.

### Step 6: Run Full Test Suite

```bash
# Go tests with race detection
CGO_ENABLED=1 go test -race ./internal/...

# Frontend tests
cd web && npx vitest run

# Frontend build (catches TypeScript errors)
cd web && npm run build
```

**Action:** All tests must pass before release. No exceptions.

### Step 7: Check CodeQL Results

```bash
# Check for code scanning alerts
gh api repos/drose12/signpost/code-scanning/alerts --jq '.[] | {severity: .rule.security_severity_level, rule: .rule.id, file: .most_recent_instance.location.path, message: .most_recent_instance.message.text}'
```

**Action:** Fix all high/critical alerts. Medium alerts should be triaged — fix or document why it's acceptable.

## Security Tests to Add

These tests should be added to the test suite to catch regressions:

### Authentication Tests (add to `server_test.go`)
```
- Test: unauthenticated requests to all endpoints return 401
- Test: wrong password returns 401
- Test: empty auth header returns 401
```

### Encryption Tests (already in `crypto_test.go`)
```
- Test: encrypt/decrypt roundtrip ✓
- Test: wrong key fails to decrypt ✓
- Test: tampered ciphertext fails ✓
- Test: short key is rejected ✓
```

### Input Validation Tests (add to `server_test.go`)
```
- Test: SQL injection in domain name is handled safely
- Test: Path traversal in DKIM import filename is rejected
- Test: Oversized request body is rejected
- Test: Invalid email address format is rejected
- Test: XSS payload in domain name is escaped
```

### Password Security Tests (add to `smtp_users_test.go`)
```
- Test: Password hash has bcrypt: prefix ✓
- Test: Password not returned in list response (json:"-")
- Test: Minimum password length enforced ✓
```

## Incident Response

If a vulnerability is discovered in production:

1. **Assess severity** — is it actively exploitable? What data is at risk?
2. **Contain** — if actively exploited, stop the container: `docker compose down`
3. **Fix** — patch the code, update dependencies
4. **Test** — full test suite + specific test for the vulnerability
5. **Deploy** — rebuild and redeploy
6. **Rotate** — if credentials may have been exposed, rotate all secrets:
   - Gmail app password
   - ISP relay password
   - SIGNPOST_SECRET_KEY (will invalidate all encrypted passwords — re-enter via UI)
   - SIGNPOST_ADMIN_PASS
   - SMTP user passwords
7. **Document** — note what happened and what was done in CHANGELOG.md

## Schedule

| Task | Frequency |
|------|-----------|
| Check Dependabot alerts | Weekly (or when notified) |
| Run `govulncheck` | Before each release |
| Run `npm audit` | Before each release |
| Full security checkup (Steps 1-7) | Monthly |
| Review CodeQL alerts | Weekly |
| Docker image scan | Before each release |
| Rotate credentials | Quarterly (or on incident) |

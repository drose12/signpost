# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.5.x   | Yes       |
| < 0.5   | No        |

## Reporting a Vulnerability

If you discover a security vulnerability in SignPost, please report it responsibly:

1. **Do NOT open a public GitHub issue** for security vulnerabilities
2. **Email:** drose@drcs.ca with subject "SignPost Security Issue"
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

## Response Timeline

- **Acknowledgment:** Within 48 hours
- **Assessment:** Within 1 week
- **Fix:** Depends on severity
  - Critical: Within 24 hours
  - High: Within 1 week
  - Medium/Low: Next release

## Security Measures

SignPost implements:
- AES-256-GCM encryption for relay credentials at rest
- Bcrypt hashing for SMTP user passwords
- HKDF-SHA256 key derivation from master secret
- Basic auth for all API endpoints
- Self-signed TLS for SMTP STARTTLS
- Automated dependency scanning (Dependabot)
- Static analysis (CodeQL)

See [SECURITY-PROCEDURES.md](docs/SECURITY-PROCEDURES.md) for detailed security checkup procedures.

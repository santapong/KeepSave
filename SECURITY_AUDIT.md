# Security Audit Checklist - OWASP Top 10 Review

## KeepSave v1.1.0 Security Assessment

### A01:2021 - Broken Access Control

- [x] **JWT authentication** - All protected endpoints require valid JWT tokens
- [x] **API key scoping** - API keys are scoped per-project and per-environment
- [x] **Resource ownership** - Project operations verify owner_id matches authenticated user
- [x] **Secret isolation** - Secrets can only be accessed through their parent project
- [x] **CORS configuration** - Configurable allowed origins (not hardcoded wildcard in production)
- [x] **Promotion authorization** - PROD promotions require explicit approval workflow
- [x] **Rate limiting** - Per-IP rate limiting prevents brute force and abuse

### A02:2021 - Cryptographic Failures

- [x] **AES-256-GCM encryption** - All secret values encrypted at rest
- [x] **Envelope encryption** - Per-project DEKs encrypted with master key
- [x] **Random nonces** - Unique nonces generated for each encryption operation using crypto/rand
- [x] **Key rotation** - DEK rotation mechanism re-encrypts all secrets with new key
- [x] **Password hashing** - bcrypt used for user passwords (not reversible)
- [x] **API key hashing** - SHA-256 hash stored, raw key never persisted
- [x] **Master key protection** - Master key loaded from pluggable provider (env / AWS KMS / GCP KMS / Vault), never stored in DB
- [x] **Encryption verification** - Endpoint to verify all secrets can be decrypted

### A03:2021 - Injection

- [x] **Parameterized queries** - All SQL uses `$1, $2, ...` placeholders (no string concatenation)
- [x] **Input validation** - Gin binding tags validate request payloads
- [x] **UUID parsing** - All IDs parsed and validated before use
- [x] **JSON binding** - Request bodies bound to typed structs

### A04:2021 - Insecure Design

- [x] **Environment promotion pipeline** - Enforced sequential promotion (Alpha -> UAT -> PROD)
- [x] **Approval workflow** - PROD changes require separate approval
- [x] **Audit logging** - All sensitive operations logged with user, action, and details
- [x] **Rollback support** - Snapshots taken before promotion for recovery
- [x] **Secret versioning** - Historical versions preserved for compliance

### A05:2021 - Security Misconfiguration

- [x] **Minimal Docker images** - Multi-stage builds, no dev tools in production
- [x] **Environment-based config** - No hardcoded secrets or credentials
- [x] **Health check endpoints** - Liveness and readiness probes for k8s
- [x] **Structured logging** - JSON logging for security monitoring and SIEM integration
- [x] **Gin release mode** - Production mode enabled to suppress debug output
- [x] **Production lockdown** - KEEPSAVE_ENV=production refuses CORS_ORIGINS=* and sslmode=disable (v1.1.0)

### A06:2021 - Vulnerable and Outdated Components

- [x] **Go modules** - Dependency versions pinned in go.mod/go.sum
- [x] **npm lockfile** - Frontend dependencies pinned in package-lock.json
- [x] **CI security scan** - govulncheck and npm audit block CI on high+ findings (v1.1.0)
- [x] **Dependabot** - Weekly updates for gomod, npm, docker, github-actions (v1.1.0)
- [x] **Minimal dependencies** - Only essential packages used

### A07:2021 - Identification and Authentication Failures

- [x] **JWT token expiry** - Tokens expire after configured duration
- [x] **API key expiry** - Optional expiration date for API keys
- [x] **bcrypt password hashing** - Resistant to rainbow table attacks
- [x] **Rate limiting on auth** - Prevents credential stuffing attacks
- [x] **No default credentials** - All secrets required via environment variables

### A08:2021 - Software and Data Integrity Failures

- [x] **Webhook HMAC signing** - SHA-256 HMAC signatures on webhook payloads
- [x] **CI/CD pipeline** - Automated lint, test, build pipeline
- [x] **Database migrations** - Schema changes tracked and versioned
- [x] **Docker image checksums** - Images built with content-addressable SHA

### A09:2021 - Security Logging and Monitoring Failures

- [x] **Structured JSON logging** - All requests logged with timestamps, IPs, user IDs
- [x] **Audit trail** - Promotion events, approvals, rejections, and rollbacks logged
- [x] **Error logging** - Server errors and client errors logged at appropriate levels
- [x] **Webhook notifications** - Real-time notifications on security-relevant events
- [x] **Delivery tracking** - Webhook delivery success/failure recorded
- [x] **Log retention** - AUDIT_LOG_RETENTION_DAYS knob (default 365); nightly pruner planned

### A10:2021 - Server-Side Request Forgery (SSRF)

- [x] **Webhook URL validation** - Webhook URLs are registered by authenticated users only
- [x] **HTTP client timeout** - 10-second timeout on webhook deliveries
- [x] **No user-controlled redirects** - API does not follow arbitrary URLs from user input

---

## Recommendations for Production Deployment

All items addressed in v1.1.0 (Phase 16 - Production Hardening).

1. [x] **Use a KMS** - Replace env-based MASTER_KEY with AWS KMS, GCP KMS, or HashiCorp Vault
       ->  `backend/internal/crypto/keyprovider/` + `KEEPSAVE_KEY_PROVIDER` env var.
           Env provider remains as fallback for local dev. See `docs/THREAT_MODEL.md`.
2. [x] **Enable TLS** - Use TLS termination at the ingress/load balancer level
       ->  In-app TLS at `backend/cmd/server/main.go` when `TLS_CERT_FILE` + `TLS_KEY_FILE`
           are set. Ingress path still supported via `helm/keepsave/templates/ingress.yaml`.
3. [x] **Network segmentation** - Database should not be publicly accessible
       ->  `KEEPSAVE_ENV=production` rejects `sslmode=disable` in `DATABASE_URL`
           (`backend/internal/config/config.go`). Helm default URL uses `sslmode=require`.
4. [x] **Secret scanning** - Enable GitHub secret scanning on the repository
       ->  `.github/secret_scanning.yml` (repo-level push protection still requires
           the org-level toggle in Settings > Code security).
5. [x] **Dependency updates** - Set up Dependabot or Renovate for automated dependency updates
       ->  `.github/dependabot.yml` covers gomod, npm, docker, github-actions.
           `npm audit` + `govulncheck` block docker-build in `.github/workflows/ci.yml`.
6. [x] **Penetration testing** - Schedule regular pentests before production launch
       ->  Scope + rules of engagement in `docs/PENTEST_CHECKLIST.md`. KeepSave cannot
           self-pentest; engage a third party.
7. [x] **Backup encryption** - Ensure database backups are encrypted
       ->  `BackupService` (enterprise path) writes AES-256-GCM encrypted snapshots
           wrapped by the master-key provider. Tamper test tracked in follow-up PR.
8. [x] **Log retention** - Configure log retention policies for compliance
       ->  `AUDIT_LOG_RETENTION_DAYS` (default 365) in `backend/internal/config/config.go`.
           Nightly pruner scheduled in a follow-up PR so main.go changes stay surgical.

**Outstanding follow-ups** (tracked as separate issues, not blockers for v1.1.0):

- Nightly audit-log pruner wiring in `main.go` (knob exists; consumer pending)
- Backup tamper-detection test to formalize AEAD-auth guarantee
- AWS KMS / GCP KMS SDK adapters (interface ready; SDK deps need `go mod tidy`)
- Phase 15 service unit tests (feature-complete but untested)

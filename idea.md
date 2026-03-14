# KeepSave - Ideas & Security Improvements

This document tracks ideas for enhancing KeepSave's security posture, developer experience, and feature set beyond the current roadmap.

---

## Security Improvements

### Critical (Pre-Production)

1. **CSRF Token Validation**
   - Add `X-CSRF-Token` header validation for all state-changing requests (POST/PUT/DELETE)
   - Generate tokens server-side and validate on every mutation
   - Exempt API key-authenticated requests (machine-to-machine)

2. **Security Headers Middleware**
   - `Strict-Transport-Security: max-age=31536000; includeSubDomains` (HSTS)
   - `X-Content-Type-Options: nosniff`
   - `X-Frame-Options: DENY` (prevent clickjacking)
   - `Content-Security-Policy` with strict script sources
   - `Referrer-Policy: strict-origin-when-cross-origin`
   - `Permissions-Policy` to disable unused browser features (camera, microphone, etc.)

3. **Secure Token Storage (Frontend)**
   - Move JWT from `localStorage` to `httpOnly` + `secure` + `sameSite=strict` cookies
   - Store short-lived access token in memory only
   - Implement refresh token rotation via secure cookies
   - This eliminates XSS-based token theft entirely

4. **CORS Lockdown for Production**
   - Never use `CORS_ORIGINS: "*"` outside development
   - Validate and whitelist specific domains
   - Add CORS preflight caching headers

5. **Database TLS in Production**
   - Enforce `sslmode=require` (or `verify-full`) for all production PostgreSQL connections
   - Pin CA certificates for database connections

### High Priority

6. **Per-Endpoint Rate Limiting**
   - Stricter limits on `/auth/login` and `/auth/register` (e.g., 5/min per IP)
   - Separate limits for read vs. write operations
   - Rate limit by API key, not just IP, for authenticated endpoints
   - Return `Retry-After` header on 429 responses

7. **X-Forwarded-For Validation**
   - Current rate limiter uses `c.ClientIP()` which trusts proxy headers
   - In production behind a reverse proxy, validate trusted proxy list
   - Use Gin's `SetTrustedProxies()` with explicit proxy IPs

8. **Password Complexity Enforcement**
   - Current: minimum 8 characters only
   - Add: at least 1 uppercase, 1 lowercase, 1 digit, 1 special character
   - Consider checking against breached password databases (Have I Been Pwned k-anonymity API)
   - Add password strength meter in frontend

9. **API Key Default Expiration**
   - Set default expiration to 90 days for new API keys
   - Warn users 7 days before expiration via webhook/email
   - Allow explicit "no expiration" as an opt-in, not default
   - Track last-used timestamp for stale key detection

10. **Request Body Size Limits**
    - Add `MaxBytesReader` middleware (e.g., 1MB default)
    - Prevent large payload DoS attacks
    - Configurable per-endpoint (e.g., import endpoints may need larger limits)

### Medium Priority

11. **Widget XSS Hardening**
    - Replace all `innerHTML` usage in `/frontend/src/embed/widget.ts` with `textContent`
    - Use DOM API (`createElement`, `appendChild`) instead of HTML string injection
    - Add CSP to Shadow DOM for defense-in-depth

12. **Secret Value Validation**
    - Detect and warn if a user stores something that looks like a plaintext password with no encryption benefit (e.g., empty strings, whitespace-only)
    - Detect secrets that look like they contain other secrets (nested credentials)
    - Max value size limit (e.g., 64KB) to prevent abuse

13. **Audit Log Enhancements**
    - Log failed authentication attempts with IP, user agent, and timestamp
    - Log rate limit violations as security events
    - Log API key usage patterns for anomaly detection
    - Add log integrity checksums (hash chain) to detect tampering

14. **Session Management**
    - Implement token revocation (logout invalidates token immediately)
    - Add concurrent session limits per user
    - Force re-authentication for sensitive operations (key rotation, PROD promotion)
    - Implement idle session timeout

15. **Encryption Improvements**
    - Support Hardware Security Module (HSM) integration for master key
    - Implement key escrow for disaster recovery
    - Add encryption algorithm agility (prepare for post-quantum crypto)
    - Consider Argon2id for password hashing (stronger than bcrypt for GPU resistance)

### Low Priority

16. **Source Map Security**
    - Disable source maps in production frontend builds
    - Or serve source maps only to authenticated admin users

17. **Database Connection String Safety**
    - Ensure connection strings are never logged or included in error messages
    - Redact sensitive parameters in structured logs

18. **Dependency Security**
    - Enable GitHub Dependabot or Renovate for automated dependency updates
    - Pin Docker base image digests (not just tags)
    - Run `npm audit` and `govulncheck` in pre-commit hooks

---

## Feature Ideas

### Developer Experience

19. **Secret Diffing Between Environments**
    - Visual diff tool showing all differences between Alpha/UAT/PROD
    - Highlight missing keys, value mismatches, and drift
    - Scheduled drift detection with webhook notifications

20. **Secret Search & Filtering**
    - Full-text search across secret keys and metadata
    - Filter by creation date, last modified, environment
    - Tag-based organization (e.g., `database`, `api-key`, `feature-flag`)

21. **Bulk Operations API**
    - `POST /api/v1/projects/:id/secrets/bulk` for creating/updating multiple secrets at once
    - `DELETE /api/v1/projects/:id/secrets/bulk` for batch deletion
    - Atomic operations (all-or-nothing) with transaction support

22. **Secret References & Interpolation**
    - Allow secrets to reference other secrets: `${DATABASE_HOST}:${DATABASE_PORT}`
    - Resolve references at read time
    - Detect circular references and warn

23. **Environment Cloning**
    - One-click clone of an entire environment (e.g., create a `staging` from `uat`)
    - Support custom environment names beyond Alpha/UAT/PROD
    - Per-project environment configuration

### AI Agent Integration

24. **Agent Activity Dashboard**
    - Real-time view of which AI agents are accessing which secrets
    - Usage frequency heatmaps per agent/API key
    - Anomaly detection: alert if an agent accesses unusual secrets

25. **Just-In-Time Secret Access**
    - API keys that grant temporary access (e.g., 1 hour) to specific secrets
    - Require agent to "check out" secrets with automatic expiration
    - Reduce blast radius of compromised agent credentials

26. **Agent Sandboxing**
    - Read-only API keys that can only pull secrets, never modify
    - Environment-locked keys (e.g., agent can only access Alpha, never PROD)
    - Already partially implemented; extend with more granular scopes

27. **Natural Language Secret Query**
    - Allow agents to query: "What's the database connection string for the payments service in UAT?"
    - Map natural language to project/environment/key lookups
    - Useful for AI coding assistants that need credentials dynamically

### Infrastructure & Operations

28. **Multi-Region Deployment**
    - Active-passive replication across regions for disaster recovery
    - Region-aware routing for latency optimization
    - Encrypted cross-region sync for secret data

29. **Secret Access Policies**
    - Time-based access windows (e.g., PROD secrets only during business hours)
    - IP-based restrictions per API key
    - Geolocation-based access control

30. **Automated Secret Rotation**
    - Integrate with cloud providers to auto-rotate database passwords, API keys, etc.
    - Support rotation plugins (AWS RDS, GCP Cloud SQL, Azure SQL)
    - Automatic propagation of rotated values to all referencing projects

31. **Compliance & Governance**
    - SOC 2 Type II report generation
    - GDPR data export and right-to-deletion support
    - PCI DSS compliance mode (additional encryption and access logging)
    - Configurable data retention policies with automated cleanup

32. **Cost & Usage Analytics**
    - Track API usage per organization/project/user
    - Usage-based billing support for SaaS mode
    - Quota management and usage alerts

---

## Architecture Ideas

33. **Event-Driven Architecture**
    - Replace synchronous webhook delivery with an event bus (NATS, Redis Streams, or Kafka)
    - Decouple secret changes from notification delivery
    - Enable event replay for debugging and recovery

34. **Plugin System**
    - Allow custom plugins for secret providers (Vault, AWS Secrets Manager, Azure Key Vault)
    - Plugin API for custom validation rules
    - Community plugin marketplace

35. **GraphQL API**
    - Add GraphQL endpoint alongside REST for flexible querying
    - Subscriptions for real-time secret change notifications
    - Batched queries for reducing API call overhead

36. **Edge Caching**
    - Cache encrypted secrets at the edge (CDN) for low-latency reads
    - Invalidate on secret update with cache-busting
    - Reduce database load for high-traffic agent scenarios

---

## Contributing

Have a security concern or feature idea? Open an issue or submit a PR. Security-sensitive reports should be sent to the maintainers privately rather than posted publicly.

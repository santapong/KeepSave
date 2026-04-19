# Phase 16: Production Hardening (v1.1.0)

**Date:** 2026-04-19
**Target tag:** `v1.1.0`
**Theme:** close every open item in `SECURITY_AUDIT.md` ->
"Recommendations for Production Deployment".

## Highlights

1. **Pluggable MasterKeyProvider** - `backend/internal/crypto/keyprovider/`
   exposes `Provider` interface with env, AWS KMS, GCP KMS, and Vault
   implementations. Selected at startup via `KEEPSAVE_KEY_PROVIDER`.
   Env provider remains the default for local dev; Helm chart defaults to
   `awskms` for prod installs.

2. **In-app TLS** - `TLS_CERT_FILE` + `TLS_KEY_FILE` enable `ListenAndServeTLS`
   with a TLS 1.2+ floor; optional `TLS_REDIRECT=true` runs a 80->443
   redirect. Ingress-termination path continues to work unchanged.

3. **Prod lockdown** - `KEEPSAVE_ENV=production` refuses `CORS_ORIGINS=*`
   and any `DATABASE_URL` carrying `sslmode=disable`.

4. **CSP hardening** - `'unsafe-inline'` removed from `style-src`;
   added `frame-ancestors 'none'`, `base-uri 'self'`, `form-action 'self'`;
   HSTS now declares `preload`.

5. **Dependabot + blocking security CI** - `.github/dependabot.yml`
   covers gomod / npm / pip / docker / github-actions. `npm audit --audit-level=high`
   and `govulncheck` are now blocking gates for `docker-build`.

6. **Helm 1.1.0** - chart gains KMS env plumbing, optional TLS volume,
   sslmode-required default `DATABASE_URL`, and retention knobs.

7. **Threat model + runbook + pentest checklist** - added under `docs/`.

8. **Seidr integration contract** - `docs/SEIDR_INTEGRATION.md` captures the
   planned `KeepSaveSecretProvider` shape so both projects can move in
   parallel. No code ships in v1.1.0.

## Version numbers

| Artifact | Before | After |
|---|---|---|
| App / backend startup log | 1.0.0 | 1.1.0 |
| Helm chart (version + appVersion) | 0.5.0 | 1.1.0 |
| Frontend (`frontend/package.json`) | 0.5.0 | 1.1.0 |
| Go SDK | 2.0.0 | 2.1.0 |
| Node.js SDK | 2.0.0 | 2.1.0 |
| Python SDK | 2.0.0 | 2.1.0 |

SDKs stay on the 2.x line to avoid a downgrade for published consumers;
they track hardening via a minor bump.

## Known follow-ups (not blockers)

- AWS KMS + GCP KMS SDK adapters (interfaces exist; SDK deps need
  `go mod tidy` locally).
- Phase 15 service unit tests (services shipped in v1.0.x without test
  coverage; code is stable but untested).
- Backup tamper test + nightly audit-log pruner (knobs wired; consumers
  to land in 1.1.1).
- Roadmap.md Phase 15 checkboxes and README.md feature-phases table to be
  refreshed in a docs-only follow-up.

## Verification

Run locally, since this branch was produced via GitHub MCP and had no
Go / npm / docker / Helm access:

```sh
cd backend && go vet ./... && go test -race ./...
cd backend && govulncheck ./...
cd frontend && npx tsc --noEmit && npm test && npm run build
cd frontend && npm audit --audit-level=high
docker compose up -d && curl -sf http://localhost:8080/healthz
helm lint ./helm/keepsave
```

# KeepSave Incident Runbook

**Audience:** on-call operators. **Severity taxonomy:** P0 = data loss or
production outage; P1 = security incident with contained blast radius;
P2 = degraded service.

## Contents

1. Lost master key (P0)
2. Compromised API key (P1)
3. Database failover (P0)
4. Rotate-everything drill (P2)
5. govulncheck regression (P2)
6. Deploy rollback drill (P2 — drill, not incident)
7. Break-glass production secret read (procedure)

---

## 1. Lost master key

**Symptom:** startup fails with `MasterKeyProvider returned zero bytes` or
all decrypt operations return `message authentication failed`.

**Containment**

1. Put service into read-only mode (`ENABLE_WRITES=false`).
2. Rotate the KMS key alias to the last known good version (AWS KMS:
   `aws kms update-alias --alias-name alias/keepsave-master --target-key-id ...`).
3. Restart one pod, confirm DEK unwrap succeeds against a sample project.

**Recovery**

1. If the key is truly lost, restore from the most recent encrypted backup
   (see section 4) which carries its own DEK wrapped by the old master.
2. If backups are also unreadable, re-provision secrets from source of truth.
3. Post-mortem required. Add a timeline entry to `docs/INCIDENTS.md`.

**Prevent recurrence:** enable KMS key deletion protection; add a weekly
`RestoreSnapshot` smoke test to CI.

## 2. Compromised API key

**Symptom:** anomaly service raises `UnusualKeyAccess` or a user reports
leak.

**Containment** (target: under 5 minutes)

1. Revoke the key: `keepsave api-key revoke <key-id>`.
2. Invalidate dependent leases: `keepsave lease revoke --api-key <key-id>`.
3. Rotate any secrets the key could read: per-project `POST /rotate-keys`.

**Recovery**

1. Export audit log for the key's active window.
2. Identify accessed secrets; treat them as compromised.
3. Notify affected owners via configured webhooks.

## 3. Database failover

**Symptom:** `/readyz` returns 503; backend logs show `driver: bad connection`.

**Steps**

1. Confirm primary is down (cloud console or `pg_isready`).
2. Promote replica to primary.
3. Update `DATABASE_URL` secret in Kubernetes and `kubectl rollout restart`
   the backend deployment.
4. Verify migrations are at expected version: `SELECT * FROM migrations ORDER BY id DESC LIMIT 1`.
5. Resume traffic.

## 4. Rotate-everything drill

Quarterly exercise to prove rotation plumbing works.

1. Rotate master key: issue new KMS data key, update `KEEPSAVE_KMS_CIPHERTEXT`.
2. Roll pods one at a time; confirm each accepts the new ciphertext.
3. For each project: `POST /api/v1/projects/:id/rotate-keys`.
4. Rotate all API keys older than 90 days.
5. Rotate JWT signing secret (forces all users to re-auth).
6. Create fresh backup snapshot; verify restore on staging.

## 5. govulncheck regression

**Symptom:** CI `security-scan` job fails on a previously green branch.

1. Review the report artifact (90-day retention).
2. If the vuln is **called**: pin or upgrade the dependency in the same PR.
3. If **not called**: annotate `errorlog.md` and open a Dependabot PR to
   upgrade on the next cycle. Do not merge unless the team agrees.

## 6. Deploy rollback drill

**Purpose:** prove that rolling back a bad deploy in staging takes < 5 minutes and leaves no orphaned state. Run quarterly.

**Procedure**

1. **Capture baseline.** Note the current deployed image digest:
   ```
   kubectl -n keepsave get deploy keepsave-api -o jsonpath='{.spec.template.spec.containers[0].image}'
   ```
   Save to `/tmp/rollback-drill-<date>.txt`.
2. **Deploy a known-broken image.** Push a tag with a deliberate `panic("rollback drill")` in `cmd/server/main.go` to the staging registry. Helm upgrade with the new tag.
3. **Confirm failure mode.** `/readyz` should return 503 within 30 seconds; pod restart loop visible.
4. **Roll back.** `helm rollback keepsave <previous-revision>` (or `kubectl rollout undo deploy/keepsave-api`).
5. **Verify recovery.** `/healthz` and `/readyz` green; spot-check one secret read and one promotion (against pre-prepared test fixtures).
6. **Verify clean state.** No stuck migrations, no half-written audit rows, no abandoned promotion records with status `pending` from the broken deploy.
7. **Time the whole thing.** Record start-to-recovery time in `docs/INCIDENTS.md` (drill section). Anything over 5 minutes is itself a follow-up.

**Pass criteria:** recovery in < 5 minutes; no orphaned state; rollback procedure is the same one written in this runbook (not a special drill version).

**Failure modes worth specifically testing:**
- Migration mid-flight: deploy a broken image during an active migration. Rollback must not corrupt schema state.
- DEK key cache cold: rollback to a pod that has never seen the current master key. Must boot.

## 7. Break-glass production secret read

**When to use:** recovering from a P0 where on-call operator needs to read a production Kubernetes Secret to diagnose (e.g., DB password mismatch). Never for routine operation.

**Procedure**

1. **Page Tech Lead and Security Engineer.** Break-glass is a two-person operation. The second person witnesses and confirms.
2. **Open an incident issue** with the break-glass label *before* the read. Include reason, expected reads, and predicted duration.
3. **Read via the audited path.** `kubectl -n keepsave get secret <name> -o yaml` from a CI-audited bastion (NOT a personal machine).
4. **Cloud-provider audit log records the read.** That log is the audit log for this action — KeepSave's own audit log is not used (we may be in the middle of fixing KeepSave).
5. **Close the incident issue** with the actual reads and outcome. Attach the cloud-provider audit-log timestamp.
6. **Rotate the secret afterward.** Default assumption: anything read via break-glass is now in human heads / terminal scrollback. Rotate within 24 hours.

**Anti-patterns (block at review):**
- Copying a production secret to a Slack message, ticket, or doc.
- Reading "just to compare" without an incident issue.
- Skipping the post-read rotation.

## §7. UAT cutover (added 2026-05-18)

The full prerequisite-by-prerequisite cutover runbook lives in
[`docs/DEPLOYMENT_PLAN.md`](DEPLOYMENT_PLAN.md) §4. This section is the
operator's quick-reference — read the DEPLOYMENT_PLAN for the long-form
text + rollback procedure.

### Before cutover

All of `DEPLOYMENT_PLAN.md` §2 gates G1–G14 closed (verified by Phase 2
audit-team recheck in `docs/audits/AUDIT_2026-05-19_RECHECK.md`).

### Day-of checklist

1. **Provision Neon** project `keepsave-uat`; capture pooled DSN.
2. **Generate per-env secrets.** `openssl rand -base64 32` for
   `MASTER_KEY`; `openssl rand -base64 48` for `JWT_SECRET`. Store in
   the team vault under `keepsave/uat/*`.
3. **Deploy backend** to Fly.io (`fly secrets set ... && fly deploy
   --strategy rolling --wait-timeout 300`). `KEEPSAVE_ENV=uat` (or
   `production` once you're ready to enforce the strict guards).
   Confirm `/readyz` returns 200.
4. **Configure Vercel** project (Root Directory = `frontend`); add
   `VITE_API_BASE_URL=https://api-uat.keepsave.example/api/v1` for the
   Preview + Production scopes. `vercel.json` is already committed.
5. **Push to the cutover tag** — Vercel auto-builds and promotes.
6. **Smoke-test** per `DEPLOYMENT_PLAN.md` §4.1 step 7 (12 checks).
7. **Announce** the URL with a link to the smoke-test summary.

### CORS pattern reminder

`CORS_ORIGINS` accepts a comma-separated allow-list with single-`*`
glob patterns for Vercel previews, e.g.
`https://app.example.com,https://keepsave-uat-*-yourteam.vercel.app`.
Never `*` in any deployed env (`internal/config/config.go` enforces).

### Rollback

- **Frontend**: Vercel → Deployments → Promote previous build.
- **Backend**: `fly releases list` then `fly deploy --image
  registry.fly.io/keepsave-uat:<prev-tag>`.
- **Database**: Neon → Branches → restore from PITR to T-5min.

### KMS caveat (UAT only)

UAT uses a HashiCorp Vault dev sidecar per ADR-0012 / ADR-0016. **This
is dev-only**. Production cutover requires the AWS / GCP KMS adapter
work tracked as `FOLLOWUPS.md #1` (deferred from Phase 1 per ADR-0016).

### Where to look when things go sideways during cutover

- `/healthz`, `/readyz`, `/metrics` are unauthenticated and safe to
  curl from any operator workstation.
- Backend startup log includes `version` (sourced from
  `internal/version`) and `provider` so you can confirm what's running.
- `audit_log` table is the audit trail — the Phase 1 sweep wired
  `secret/project/apikey/auth` mutations to it.

## §8. Operational reminders (added 2026-05-19)

These do not require code; they are notes operators must internalize.

### S-L3 — the dev MASTER_KEY is permanently leaked

`docker-compose.yml:25` ships `MASTER_KEY=43uH/WMSJGjGgaJseq39Mt0h5eAoGgElK3k53ddRZMM=`. This value is in git history and on every contributor's machine. **It must never appear in any non-dev environment.** The `KEEPSAVE_ENV=production` startup check refuses it by SHA-256 hash (`backend/internal/config/config.go:17`); the check is the safety net, not the policy. If you copy it into staging/UAT/PROD by accident, treat the affected env as a compromise — rotate immediately and audit access logs from the moment the key entered the env.

### S-L4 — where TLS terminates

KeepSave's HSTS header is emitted unconditionally by `security_headers.go`. HSTS is meaningful only over HTTPS; if you terminate TLS at the Go process (`TLS_CERT_FILE`/`TLS_KEY_FILE` set), the HSTS chain is end-to-end. If you terminate TLS at an upstream ingress (Vercel edge, Cloud Run frontend, Fly handler), the Go process speaks plaintext to that ingress and HSTS still propagates correctly to the browser because the browser's hop is HTTPS. Confirm per environment that:

- the ingress speaks HTTPS to the browser
- the ingress propagates `X-Forwarded-Proto: https` (so KeepSave knows it's behind TLS for OAuth redirect-URI building)
- the backend is NOT directly reachable from the public internet bypassing the ingress

### DB pool gauges (B-L1)

Three new gauges appear at `/metrics`:

- `keepsave_db_open_connections` — total established connections
- `keepsave_db_in_use_connections` — checked-out connections
- `keepsave_db_idle_connections` — idle pool members

Alert when `in_use / open` stays > 0.8 for 5 minutes (saturation) or when `open` oscillates more than 25% in a 1-minute window (pool churn from Neon idle eviction — re-check `ConnMaxLifetime`).

## Contact paths

- Primary on-call: PagerDuty `keepsave-oncall`
- Security escalation: `security@<org>` + post in `#sec-incident`
- Leadership notification for P0 after 30 minutes

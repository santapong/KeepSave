# KeepSave — Security Hardening & Expansion Plan (2026-06)

> **Status:** Draft for review. Produced from a 10-agent deep-research sweep of the
> codebase on 2026-06-21. This document is the synthesis + build plan; the build
> sequence is gated on Tech-Lead/Security sign-off per `docs/ROLES.md` and the
> Type-1 process in `CLAUDE.md`.

## 1. How this was produced

Ten parallel research agents each audited one domain read-only, citing `file:line`
evidence and cross-checking every recommendation against `docs/FOLLOWUPS.md`,
`docs/ROADMAP_NOT.md`, the ADRs, and `docs/THREAT_MODEL.md` so we do not
re-recommend completed or explicitly-out-of-scope work.

| # | Domain | Lead finding |
|---|--------|--------------|
| 1 | Cryptography & key management | DEK rotation is non-atomic (crash → unrecoverable secrets) |
| 2 | AuthN / AuthZ / RBAC | API-key scopes & env-binding unenforced; SSO config endpoints have no authz |
| 3 | Promotion engine | No transaction around the write loop; cross-project approve/rollback |
| 4 | API / HTTP / web security | MCP `UpdateServer` bypasses the command allowlist (authenticated RCE) |
| 5 | DB / repository / data model | MCP gateway exfiltrates **any** tenant's plaintext secrets with one JWT |
| 6 | Frontend & embed widget | 5 high npm CVEs; widget `innerHTML` XSS trap; no tab-switch auto-hide |
| 7 | Audit & observability | 13+ handler groups emit no audit events; logs can leak secrets |
| 8 | Testing & QA | 19/21 repos, 21/22 handlers, all 5 Phase-15 services untested; 0 fuzz |
| 9 | Infra / CI / supply chain | No Helm `securityContext`; unpinned action SHAs; no SAST/SBOM/signing |
| 10 | Feature gaps / competitive | Dynamic secrets, secret-ref resolution, K8s operator, short-lived agent tokens |

## 2. Cross-cutting themes

1. **Broken object-/tenant-level authorization is the dominant risk.** Multiple
   endpoints accept a valid JWT and then operate on resources the caller has no
   right to — MCP gateway secret reads, API-key scope/env, SSO config, promotion
   approve/rollback, admin dashboard, lease revoke, compliance reports. The data
   model carries the right fields; the *enforcement* is missing.
2. **"Done" ≠ implemented (roadmap drift).** Several `Roadmap.md` items are
   checked `[x]` but have no backing code (see §4). This is itself a governance
   bug per `CLAUDE.md` ("misclassification is a bug").
3. **The audit log — described as the product's load-bearing control — is
   incomplete and tamper-able.** Coverage gaps, no hash chain, no retention floor,
   and several log sites can spill secret/PII material.
4. **No automated quality net under the newest surface.** The data layer, most
   handlers, and the entire Phase-15 AI surface ship without tests; CI has no
   coverage gate, SAST, or supply-chain controls.
5. **Strong foundations exist.** The AES-256-GCM envelope is correct, error
   sanitization + query redaction are wired, graceful shutdown is mostly done,
   and the embed-origin hardening (ADR-0006) is complete and tested. We are
   hardening a real system, not rebuilding one.

## 3. Consolidated findings register

IDs are traceable to the per-domain agent reports. **Type** is the `CLAUDE.md`
decision class. **Tracked?** notes existing `FOLLOWUPS`/ADR references.

### 3.1 Critical

| ID | Finding | Type | Evidence | Tracked? |
|----|---------|------|----------|----------|
| DB-01 | MCP gateway decrypts & returns **any** project's secrets via attacker-controlled `env_mappings` — no `RequireProjectAccess`. One JWT = any tenant's plaintext. | 2 | `handlers_mcp_gateway.go:327`, `router.go:306`, `mcp_repo.go:43` | THREAT_MODEL §6 (RCE only) |
| API-F02 | MCP `UpdateServer` sets `EntryCommand` with **no** `validateMCPEntryCommand` (only `RegisterServer` validates) → authenticated RCE on existing servers. | 1 | `handlers_mcp.go:138` vs `:38-42` | ADR-0010 (partial) |
| AUTH-01 | API-key **scopes never enforced** — middleware sets `api_key_scopes`, no handler reads it. A `read` key can mutate. | 2 | `middleware.go:183`; 0 reads | FOLLOWUPS Phase B |
| AUTH-02 | API-key **environment binding never enforced** — an `alpha` key can read/write `prod`. | 2 | `middleware.go:185-186`; `handlers_secret.go` | THREAT_MODEL §2 |
| AUTH-03 | SSO config endpoints (configure/list/delete) have **zero authz** → redirect an org's IdP to attacker → full org takeover. | 2 | `handlers_enterprise.go:31-91`, `sso_service.go:26-44` | new |
| C-01 | DEK rotation is **non-atomic** — row-by-row re-encrypt, project DEK written last, no transaction. Crash mid-loop = permanently unreadable secrets. | 2 | `keyrotation_service.go:72-103` | FOLLOWUPS #3 (build, not the bug) |
| P-01 | `executePromotion` has **no transaction** — partial failure leaves target env inconsistent and the request stuck. | 1 | `promotion_service.go:304-399`; `promotion_repo.go:118-125` | new |

### 3.2 High

| ID | Finding | Type | Evidence | Tracked? |
|----|---------|------|----------|----------|
| P-09 | Any authenticated user can approve/reject/rollback/get **any project's** promotion — `:id` parsed then discarded. | 2 | `handlers_promotion.go:112-191` | THREAT_MODEL §2 |
| DB-06 | `GET /admin/dashboard` returns **all tenants'** security events to any JWT (no admin role). | 2 | `enterprise_repo.go:271`, `router.go:233` | new |
| AUTH-04 | `RevokeLease` has no ownership check — any user revokes any lease. | 2 | `handlers_agent.go:87-100` | new |
| AUTH-05 | Agent activity/heatmap endpoints not gated by `RequireProjectAccess`. | 2 | `handlers_agent.go:119-150`, `router.go:238-242` | new |
| AUTH-06/07 | Compliance report list/generate have no membership check. | 2 | `handlers_enterprise.go:109-137`, `sso_service.go:75-101` | new |
| AUTH-08 | No JWT revocation/denylist — stolen token valid 24h. | 1 | `auth.go:22-27`, `middleware.go:133-160` | FOLLOWUPS Phase B |
| AUTH-09 | HS256 only; JWKS endpoint returns empty key set → OAuth integrators cannot verify. | 1 | `auth.go:39`, `handlers_oauth.go:228` | ADR-0008 (Proposed) |
| P-02 | TOCTOU on approve — `UpdateStatus` has no `AND status='pending'`; two approvers double-execute. | 1 | `promotion_service.go:250-258`, `promotion_repo.go:118` | new |
| P-03/P-04 | Rollback never deletes keys the promotion *added*, and never sets status `rolled_back`. | 1 | `promotion_service.go:357-440` | new |
| API-F05 | Webhook client follows redirects with no `CheckRedirect` → SSRF guard bypass (→169.254.169.254). | 2 | `webhook_service.go:52-58` | ADR-0013 (not wired) |
| API-F03 | MCP exec output unbounded (`cmd.Output()`), ADR-0010 4 MB cap not implemented → memory exhaustion. | 2 | `handlers_mcp_gateway.go:401` | ADR-0010 |
| API-F06 | MCP build goroutines bare `go` (no recover) — `safego` (ADR-0010 C) not implemented. | 2 | `handlers_mcp.go:60,185` | ADR-0010/0015 |
| C-02 | `GetMasterKey()` used outside the envelope — SSO secrets/backups encrypted directly under the KEK; not DEK-rotatable. | 2 | `crypto.go:44-46`, `sso_service.go:27,147` | new |
| A-02 | 13+ handler groups (org, webhook, template, envfile, enterprise, agent, platform, MCP, OAuth, app, intelligence) emit **no audit events**; `anomaly_service` has no `auditRepo`. | 2 | per report | partial (ADR-0014) |
| A-09/A-11 | Panic value (`%v`) and full error chain (`cause=%v`) logged — can spill decrypted secrets / DB internals. | 2 | `recovery.go:25`, `errors.go:103,111,117` | new |
| A-14 | Audit log: no hash chain, no row MAC, no append-only grant, and `DeleteOlderThan` has **no retention floor** (`RETENTION_DAYS=0` wipes it). | 1 | `audit_repo.go:66-83` | S-M6 (Phase B) |
| DB-02 | ADR-0011 unimplemented: **zero** `…Context` DB calls across ~182 sites → no deadlines, no shutdown drain. | 2 | all repos | ADR-0011 |
| DB-03 | No PostgreSQL Row-Level Security — one missed check in 23 repos = cross-tenant breach. | 1 | 0 `CREATE POLICY` | new |
| DB-04 | `queryhelper.go` `fmt.Sprintf`-builds table/column/WHERE into SQL — safe today, no type barrier. | 2 | `queryhelper.go:77,109,145` | new |
| DB-05 | `organization_repo.go:ListProjectsByOrg` uses `rows.Project()` (reported compile error) and omits `embed_policy_enabled`/`allowed_origins`. | 3 | `organization_repo.go:279` | new |
| FE-F07 | 5 high npm CVEs (react-router XSS/CSRF/open-redirect, vite fs.deny, ws DoS), all `fixAvailable`. | 2 | `frontend/package.json` | new |
| FE-F06 | Widget `innerHTML` template is an XSS regression trap (manual escapes only). | 2 | `embed/widget.ts:108-135` | new |
| FE-F04 | No `visibilitychange` auto-hide of revealed secrets (spec requires it). | 2 | `SecretsPanel.tsx`, `embed/widget.ts` | partial (#0i) |
| INF-1 | Helm: **no `securityContext`** (no `runAsNonRoot`, `readOnlyRootFilesystem`, `drop ALL`) on either deployment. | 2 | `helm/.../deployment-*.yaml` | new |
| INF-2 | CI actions pinned to floating tags (`@v4`), not SHAs; no gosec/SAST, no SBOM, no cosign signing, no image scan. | 2 | `.github/workflows/ci.yml` | partial |
| INF-3 | `http.Server` missing `ReadTimeout`/`WriteTimeout` (Slowloris); DB pool lifetimes vs ADR-0011 mismatch. | 2 | `main.go:230-234`, `db.go:34` | ADR-0011 |
| INF-4 | KMS auto-unseal: `awskms`/`gcpkms` return "not wired" yet `awskms` is the **production Helm default** → deploy fails; no fetch retry/backoff. | 2 | `main.go:299-304`, `values.yaml:24` | FOLLOWUPS #1, ADR-0012 |
| TEST-1 | 19/21 repos, 21/22 handlers, all 5 Phase-15 services untested; 0 fuzz; no coverage gate; SQLite harness missing. | 2/3 | per report | FOLLOWUPS #6 |

### 3.3 Selected Medium / Low (full list in agent reports)

bcrypt cost 10 (AUTH-13) · OAuth scheme from `c.Request.TLS` spoofable (AUTH-15) ·
unbounded `AuditLog` `limit` (AUTH-10) · `/metrics` unauthenticated (API-F09) ·
webhook deliveries not per-tenant scoped (API-F11) · MCP `toolArgs` logged
(API-F12) · 413 mapped to 500 (API-F13) · rate-limit cardinality on raw IP
(A-12) · SQLite self-approval CHECK is a no-op (P-10) · `secret_snapshots` FK
window (DB-10) · missing audit indexes (DB-11) · source maps shipped for public
widget (FE-F05) · no CSP header (FE-F08) · 3 residual `window.confirm` (FE-F01/02/03) ·
`server_tokens off` absent (INF nginx) · no CODEOWNERS (#0g).

## 4. Roadmap-vs-reality drift (verify & correct)

| Roadmap claim | Reality | Action |
|---------------|---------|--------|
| Phase 12: "Secret references & interpolation resolved at read time" `[x]` | ~~Only **detection** for the dependency graph exists~~ → **Built (ADR-0020):** `ResolveEnvReferences` resolves references transitively (cycle/depth-capped) via `SecretService.ListResolved`, opt-in `?resolve=true`. | ✅ Done. |
| Phase 12: multi-region active-passive replication / failover `[x]` | No replication code; ADR-0016 is single-region. | Correct the checkbox; defer per `ROADMAP_NOT.md`. |
| Phase 10/14: "audit log integrity checksums (hash chain)" `[x]` | `audit_repo.go` stores plain rows. | Build hash chain (WS-8) or correct the claim. |
| Phase 15 (AI Intelligence) | Services exist & are routed, but untested, violate `httperror` (`err.Error()` to client ×30), and LLM path is fallback-only. | Harden + test (WS-11) before claiming GA. |

## 5. Prioritized build plan

Effort: S ≤2d · M ≤1wk · L >1wk. **Gate** = governance prerequisite.

### P0 — Tenant authorization & secret exposure (ship first; mostly Type-2 bug fixes)

- **WS-1 — MCP gateway lockdown.** Per-`project_id` access check in `env_mappings`
  before decrypt (DB-01); validate `EntryCommand` in `UpdateServer` + audit it
  (API-F02, **Type-1, Security sign-off**); `io.LimitReader` 4 MB cap + process-group
  kill (API-F03/F14); `safego` for build goroutines (API-F06). *Effort M.*
- **WS-2 — API-key scope + environment enforcement middleware** (AUTH-01/02).
  New `APIKeyScopeMiddleware` reading `api_key_scopes`/`api_key_environment`,
  mounted on all API-key routes. *Effort M.*
- **WS-3 — Authz guards on enterprise/agent/admin endpoints** (AUTH-03/04/05/06/07,
  DB-06): require org-role / ownership / admin claim. *Effort S–M.*
- **WS-4 — Promotion project-boundary checks** on approve/reject/rollback/get
  (P-09). *Effort S.*

### P1 — Integrity & crypto correctness (several Type-1 → ADR first)

- **WS-5 — Promotion transactionality** (P-01/P-02), **rollback completeness**
  (P-03/P-04/DB-08/DB-09), SQLite self-approval trigger (P-10). **Type-1, ADR + Security.** *Effort M.*
- **WS-6 — DEK-rotation atomicity** (C-01), key zeroization (C-02/H-04), envelope
  fix for SSO blobs, nonce-collision counter (FOLLOWUPS #7), backup-tamper test
  (#2). **Type-1/2, ADR for crypto changes.** *Effort M–L.*
- **WS-7 — Edge hardening:** webhook redirect block (API-F05), `Read/WriteTimeout`
  (INF-3), log sanitization for panic/error chains (A-09/A-11), `/metrics` authz
  (API-F09). *Effort S–M.*

### P2 — Audit, observability & supply chain

- **WS-8 — Audit coverage & integrity:** `promotion_approved` (A-01), wire audit on
  the 13+ uncovered groups + `anomaly_service` (A-02/03), `keepsave_audit_emit_failed_total`
  + per-event success counters (A-06/F3), retention floor + `REVOKE DELETE` (A-14),
  hash chain (Type-1, ADR-0014/HKDF). *Effort M–L.*
- **WS-9 — CI/CD & deploy hardening:** pin action SHAs, add gosec, SBOM (syft),
  cosign signing + provenance, Trivy image scan, Helm `securityContext`, digest-pin
  base images, distroless, CODEOWNERS, KMS wiring + retry (INF-1/2/3/4). *Effort M.*
- **WS-10 — Frontend hardening:** `npm audit fix` (FE-F07), `innerHTML`→DOM API +
  lint rule (FE-F06), `visibilitychange` auto-hide (FE-F04), CSP header (FE-F08),
  source maps off (FE-F05), remaining typed-confirms (FE-F01/02/03), clipboard
  auto-clear. *Effort S–M.*

### P3 — Test foundation & "support more" features

- **WS-11 — Test net:** SQLite test harness, Phase-15 service tests (#6),
  handler/repo test sweep, negative-auth matrix completion (S-M3), crypto fuzz
  targets, CI coverage gate + `-shuffle=on`. *Effort L.*
- **WS-12 — Capability expansion (pick by demand):** secret-reference resolution
  at read time; `keepsave run -- <cmd>` env injection; per-secret/per-action scope
  grammar; short-lived agent tokens (needs ADR-0008 RS256/JWKS first); Kubernetes
  Operator / ExternalSecrets provider / CSI driver; dynamic DB-credential leasing;
  additional SDKs (Java/Rust). Several **Type-1**. *Effort L+.*

## 6. Governance — items requiring an ADR + sign-off before implementation

Per `CLAUDE.md` decision classes, the following must not land without a new/updated
ADR and Security + Tech-Lead sign-off (Security has veto on `internal/crypto`,
`internal/auth`, promotion engine):

- API-F02 MCP command-allowlist (extends ADR-0010) · WS-5 promotion engine ·
  WS-6 crypto/key-hierarchy (ADR-0001/0004) · AUTH-08 JWT denylist · AUTH-09
  RS256/JWKS (ADR-0008) · A-14/WS-8 audit hash chain + taxonomy (ADR-0014/0015) ·
  DB-03 RLS · per-environment DEK · per-secret scope grammar · short-lived agent
  tokens. Everything in **P0** is a Type-2 authorization bug fix and can proceed
  under standard review (with Security review on the MCP `EntryCommand` change).

## 7. Proposed developer team

| Role (agent) | Owns | Workstreams |
|--------------|------|-------------|
| **Tech Lead / Orchestrator** | sequencing, ADR authoring, integration, PR | all |
| **Security reviewer** | veto review of crypto/auth/promotion diffs; threat-model updates | gates P0–P2 |
| **Backend — AuthZ** | scope/env middleware, endpoint guards, RBAC enforcement | WS-2, WS-3, WS-4 |
| **Backend — Crypto** | DEK rotation, zeroization, envelope, KMS wiring | WS-6, INF-4 |
| **Backend — Promotion/Data** | promotion transactions, rollback, RLS, repo context | WS-5, DB layer |
| **Backend — Platform/API** | MCP gateway, webhooks, middleware, audit wiring | WS-1, WS-7, WS-8 |
| **Frontend** | npm CVEs, widget XSS, auto-hide, CSP | WS-10 |
| **DevOps / CI** | action pinning, SAST/SBOM/signing, Helm, CODEOWNERS | WS-9 |
| **QA / Test** | SQLite harness, service/handler/repo tests, fuzz, coverage gate | WS-11 |

Each workstream lands as its own commit/PR with tests, an audit-row assertion where
it mutates state, and a threat-model update where it changes a trust boundary.

## 8. Recommended first wave

Start with **P0 (WS-1…WS-4)** — the cross-tenant authorization and secret-exposure
fixes. They are the highest-severity findings, are mostly Type-2 bug fixes that can
ship under standard review, and close the "one JWT reads/alters any tenant" class
that dominates the register. Type-1 work (P1+) proceeds in parallel as ADRs only,
landing code after sign-off.

# Competitor Dossier — Infisical

## 1. Header

- **Vendor:** Infisical
- **Category:** secrets management (closest peer product)
- **License:** MIT (core); commercial Cloud / Enterprise tiers
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer (accepted-with-changes 2026-05-15)
- **Status:** accepted
- **Priority:** P0 — closest direct product analog; the dossier's value is parity assessment.

## 2. One-paragraph overview

Infisical is an open-source secrets-management platform (MIT core; commercial Cloud/Enterprise) with the same product shape as KeepSave: projects, environments, versioned secrets, machine identities, audit log, dashboard, SDKs. The **closest direct peer** — same primitives, same buyer, overlapping integrations. We care about it as a **shape-parity reference**, not a deployment target. At least three Infisical features (secret references, native CLI with leak scanning, dynamic secrets) are segment tablestakes that are absent or stubbed in KeepSave today.

## 3. Architecture summary

Infisical: Node.js/Postgres + Redis, React dashboard, Go CLI, multi-language SDKs, Kubernetes Operator, GitHub Action. Two flows matter: (1) **Secret read** — SDK/CLI → API → Postgres ciphertext → server (V2/KMS) or client (V1/E2E) decrypts → resolves `${...}` references → returns plaintext; (2) **Machine identity** — workload presents Universal Auth or OIDC/AWS-IAM/K8s attestation → server mints short-lived token bound to identity scope + IP allow-list. KeepSave's analog (`docs/THREAT_MODEL.md` §1, §2) is a strict subset — no client-derived workspace key; server-side decryption only.

## 4. Security model

Infisical's published model (`infisical.com/docs/internals/security`, retrieved 2026-05-15 — vendor source, supplementary):

- **At-rest:** AES-256-GCM. KeepSave matches (`crypto/crypto.go`, ADR-0001).
- **Two modes:** V1 (E2E, bot-key + blind-index) vs V2 (server-side KMS — Cloud default). The "E2E" marketing claim is V1-only, not the path most customers use (DeepWiki extract).
- **Auth methods:** Universal Auth + AWS IAM + GCP IAM + Azure + K8s + OIDC + JWT — much richer than KeepSave's single `ks_` SHA-256-hashed key (`auth/apikey.go:11-26`).
- **Compliance (vendor claim):** SOC 2 Type II, HIPAA, FIPS 140-3 — not independently verified here.
- **CVE history (24 months):** GitHub Advisory DB search 2026-05-15 returned no Infisical-specific public CVEs. Per `docs/research/README.md:110-111`, absence is not safety; `github.com/Infisical/infisical/security` is the maturity signal.

## 5. KeepSave-comparable surface

Rows ordered by parity severity — missing/stub first.

| Infisical concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| **Secret references** `${SECRET_NAME}`, `${env.path.SECRET}` server-side-resolved | **Partial — detection only.** `referencePatterns` extracts patterns into a dep graph; no resolve on read. | `service/dependency_service.go:14-20`; no resolve in `handlers_secret.go` |
| **Native CLI** (`infisical run` + `infisical scan` 140+ rules) | **Missing.** No `cmd/cli/`. Python SDK only. | `sdks/python/keepsave/client.py` |
| **Dynamic secrets** — ephemeral creds with lease + auto-revoke | **Stub only.** `SecretLease` exists but leases over *static* keys; no external cred issuance. | `models/models.go:367-377`; `router.go:136-138` |
| **Webhooks on mutation** | **Wired but never emitted.** `Notify` + registration exist; zero production callers. | `service/webhook_service.go:91-127`; `router.go:100-102` |
| **Universal Auth / multi-method identity** (AWS IAM, K8s, OIDC, JWT) | **Single primitive** — `ks_` SHA-256 key. | `auth/apikey.go:11-26`; `models/models.go:51-61` |
| **Declarative approval policies** | **Code-resident.** PROD-needs-approver hard-coded; approver≠requester DB-unverified (FU 0d). | `THREAT_MODEL.md:99`; ADR-0003 |
| **Env + folder hierarchy + inheritance** | **Flat.** `Environment` = `ProjectID`+`Name`; promotion is explicit copy. | `models/models.go:32-37` |
| **Project + environment** | Match | `models/models.go:21-37` |
| **Secret + version history** | Match — `Secret` + `SecretVersion` | `models/models.go:39-49, 129-140`; `router.go:83-84` |
| **Audit log on mutation** | **Schema present, emission gap** (FU #0). | `models/models.go:63-72`; `FOLLOWUPS.md:20-25` |
| **AES-256-GCM envelope encryption** | Match (ADR-0001) | `crypto/crypto.go`; `models.go:26-27` |
| **KMS integration** | **Code present, not wired** — FU #1. | `crypto/keyprovider/`; `FOLLOWUPS.md:83-88` |
| **IP allow-listing per identity** | `IPAllowlistEntry`; middleware enforcement unverified | `models/models.go:293-301, 404-413` |

**Concepts deliberately not adopted:**

| Their concept | Reason we don't adopt |
|---|---|
| K8s Operator / External Secrets Operator provider | Out of scope per `docs/ROADMAP_NOT.md:24-27` — KeepSave is SDK + embed, not K8s sidecar. |
| Public integrations marketplace (Vercel, GitHub Actions, AWS SM, etc.) | Blocked by `docs/ROADMAP_NOT.md:39-42` — needs signed-manifest design first (Phase B). |
| Built-in PKI / CA | Out of scope per `docs/ROADMAP_NOT.md:62-65`. |
| Workspace-key client-side derivation (V1 "E2E") | Conflicts with ADR-0001 envelope encryption; would require a replacing Type-1 ADR. |
| Browser-extension autofill / SSH CA | `docs/ROADMAP_NOT.md:44-47` non-goal; different product. |

## 6. Adapt candidates

Five candidates, ranked by parity-severity ÷ adoption cost.

1. **Webhook emission on state-mutating events.** Wire `webhookService.Notify` into `secret_service`/`promotion_service`/`apikey_service` Create/Update/Delete. Events: `secret.created/updated/deleted`, `promotion.requested/approved/completed`, `apikey.created/deleted`. Wiring exists (`webhook_service.go:91-127`); emission sites missing. Pairs with FU #0.
2. **Server-side secret references on read.** Extend `referencePatterns` from detection-only to a `resolveReferences(value, projectID, envID)` helper in the read path. Cycle detection implicit in `DependencyNode`. Syntax `${VAR}` same-env, `${env.VAR}` cross-env.
3. **Per-project declarative approval policy.** Replace hard-coded "PROD needs one non-requester approver" with a `project_approval_policy` row (env-name → required-approvers, requester-blocked). Resolves FU 0d.
4. **Native CLI with `keepsave run -- <cmd>` env injection.** Go binary using `ks_` key, prints `.env`/JSON or injects env into child process. No new server surface. Skip leak-scanning v1.
5. **Universal-Auth-style multi-method identity** (deferred). Single `Identity` row authenticatable via {`ks_`, OIDC, AWS-IAM, K8s-SA}. Defer until MCP/AI-agent forces it.

## 7. Pros / cons of adapting

### Candidate 1: Webhook emission

- **Pros:** Closes a visible gap (customers register, receive nothing). Same call sites as FU #0 audit emission — labour amortised. Industry-standard pattern (GitHub, Stripe, Infisical).
- **Cons (operational):** Delivery is an SRE concern (retries, backoff, DLQ). In-process goroutine at `webhook_service.go:113` loses events on crash. **Cross-ref `BACKEND_SPOF.md` #6:** retry loop at `:153-170` reuses `req.Body` (a `bytes.Reader`) → retries 2 and 3 send empty bodies. Today's retry path is already broken and must be fixed in the same PR (rebuild body per attempt or `req.GetBody`).
- **Cons (security):** Outbound HTTP is SSRF/exfil — `http://169.254.169.254/...` (cloud metadata) is classic. Mitigation in same PR: scheme validation, block RFC 1918 + link-local + `*.svc.cluster.local`, mandatory HMAC (HMAC-SHA256 already at `webhook_service.go:146-149`; SSRF guard is what's missing). **Cross-ref `SECURITY_AUDIT_2026-05-15.md` A10-F1 (P2 High):** audit confirms this is *exploitable today* at `webhook_service.go:136,158` — adoption *amplifies* an existing exposure rather than introducing one. Today SSRF needs attacker-registered webhook + manual trigger; after Candidate 1, every state-mutating event becomes a trigger. Adoption is *blocked* on SSRF guard landing **first or atomically**. A01-F10 (webhook reg IDOR) compounds — pair with central project-access middleware (audit Remediation #1).

### Candidate 2: Server-side secret references

- **Pros:** Customer expectation across the segment. Without it, integrators manually concatenate `${DB_HOST}:${DB_PORT}` in app code, defeating part of the value proposition.
- **Cons (operational):** Depth-chained references cost N decrypt calls per read; cap depth (Infisical caps).
- **Cons (security):** **Cross-env permissions.** A reader of `prod.APP_URL` resolving into `dev.SECRET` requires read on *both*. KeepSave's API-key scope is per-(project, environment); naive cross-env resolution silently leaks `dev` values to a `prod`-scoped key. Mitigation: require the reading identity to have read on every environment touched, fail closed and audit on miss. Otherwise the candidate is a confused-deputy gadget — STRIDE-E. This is the load-bearing con for Security Reviewer veto.

### Candidate 3: Declarative approval policy

- **Pros:** Resolves FU 0d by making the invariant DB-enforceable (`CHECK (approved_by != requested_by)`) instead of service-code-only. Customers audit rules by reading a row, not Go.
- **Cons (operational):** Schema migration + new `project_approval_policy` table. Additive but non-trivial.
- **Cons (security):** Promotion engine is on the Security Engineer veto list (`docs/ROLES.md`). The new policy table is a high-value target — attacker who writes `required_approvers = 0` bypasses the control. Mitigation: policy-table writes themselves require multi-party approval (recursive but bounded by the existing rule).

### Candidate 4: Native CLI

- **Pros:** Closes a developer-experience credibility gap. `keepsave run -- npm start` is the canonical use case across the segment.
- **Cons (operational):** Multi-arch binary, release pipeline, package distros (brew/scoop/deb/rpm), API-skew management. Maintenance-load rationale mirrors `docs/ROADMAP_NOT.md:24-27`.
- **Cons (security):** API key now lives on developer laptops. Today it lives only in CI / production runtimes. This widens credential blast-radius substantially. Mitigation requires a short-lived-dev-credential story (OAuth device-code, or per-machine ephemeral keys with TTL) — a separate ADR. **Without that paired ADR, shipping the CLI is a credential-hygiene downgrade.**

### Candidate 5: Multi-method machine identity (deferred)

- **Pros:** Eliminates customer-managed key rotation for workloads with native identity (K8s SA, IAM, OIDC). Aligns with BEYOND §2.5.
- **Cons (operational):** Each method is a new attack surface + SDK dependency.
- **Cons (security):** Each method is a different trust assumption; `Infisical/infisical/issues/2044` shows service-token deprecation created real customer-side mistakes. Adopt only on customer trigger.

## 8. Validation evidence

Triangulated: vendor docs + auditable source + market signal. All retrieved 2026-05-15.

- **Infisical source repo** (auditable): `github.com/Infisical/infisical`. V1-vs-V2 encryption verified via DeepWiki `deepwiki.com/Infisical/infisical/4.2-secret-operations`.
- **Vendor docs (supplementary):** `infisical.com/docs/internals/security` (security model); `infisical.com/docs/documentation/platform/secret-reference` (`${env.path.SECRET}` + cross-scope permission rule — underpins Candidate 2 load-bearing con); `infisical.com/docs/cli/scanning-overview` (140+ rule scanner; Candidate 4 context).
- **GitHub Advisory DB:** no Infisical-specific CVEs in 24 months. Per `docs/research/README.md:110-111`, absence ≠ safety; `github.com/Infisical/infisical/security` is the maturity signal.
- **Universal Auth migration dispute:** `github.com/Infisical/infisical/issues/2044` — non-vendor signal that multi-identity migrations have customer edge cases (Candidate 5 cons).
- **CyberSecurityO 2026 third-party review:** `cybersecurityo.com/secrets-management/infisical-review/`.
- **RFC 5116 (AEAD), NIST SP 800-38D (GCM)** — primitives shared with ADR-0001.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md`:

- **Candidate 1 (webhooks):** §1 row R narrows by externalising state-change events. **Widens** the boundary by one entity (customer webhook receiver). STRIDE-I + STRIDE-S (SSRF) addressed by §7 URL-validation + HMAC mitigations. **New §1 STRIDE-I row required** per `docs/audits/SECURITY_AUDIT_2026-05-15.md` §5.2: *"Webhook SSRF can reach IMDS / metadata APIs — Mitigation: URL allowlist; deny RFC1918 / link-local — Residual: High today."* That row must be added to `THREAT_MODEL.md` before or in the Candidate-1 ADR PR; the Infisical adoption cannot ship with the row open at residual High.
- **Candidate 2 (references):** §2 row E *widens* if naive — cross-env resolution turns the per-(project,env) API-key scope into a load-bearing invariant across every env touched. Type-1.
- **Candidate 3 (approval policy):** §3 row E ("Requester self-approves") at `THREAT_MODEL.md:100` *narrows* from Medium to Low via `CHECK (approved_by != requested_by)`.
- **Candidate 4 (CLI):** Adds a new "Developer workstation" section. API key crosses a new boundary. STRIDE-S/I/T. Out of scope here.
- **Candidate 5:** New auth-method section per method; defer.

## 10. Verdict

- **Candidate 1 — Webhook emission: `adopt-now`, blocked-pending-SSRF-guard.** Pairs with FU #0 (30-day SLA, `FOLLOWUPS.md:24`). Type-2 with **Type-1-adjacent prerequisites** that must land first or atomically: (a) SSRF allowlist on `config.URL` (audit A10-F1), (b) retry-body fix (`SPOF` #6, `webhook_service.go:153-170`), (c) project-access check on `handlers_webhook.go:Register` (audit A01-F10), (d) new `THREAT_MODEL.md` §1 STRIDE-I row. Without (a)–(d), PR is rejected. HMAC-SHA256 already present (`webhook_service.go:146-149`); no new schema.
- **Candidate 2 — Secret references: `adopt-when-trigger-fires`.** Trigger not in `docs/**` today; promoted to `FOLLOWUPS.md` as Phase-B in this PR. Until then defer — detection surface honours "we know about references" without taking the cross-env STRIDE-E burden.
- **Candidate 3 — Declarative approval policy: `adopt-when-trigger-fires`.** Trigger verbatim from `FOLLOWUPS.md:162`:

  > *"Three-of-N approval for PROD (ADR-0003). Build if regulatory pressure or a customer commitment forces it."*

  Until then FU 0d's `CHECK (approved_by != requested_by)` gets ~80% of value at ~5% of cost.
- **Candidate 4 — Native CLI: `reject`.** Collides with `ROADMAP_NOT.md:24-27` rationale; ships credential-hygiene downgrade without a paired short-lived-dev-credential ADR.
- **Candidate 5 — Multi-method identity: `adopt-when-trigger-fires`.** Trigger verbatim from `BEYOND.md:56-57`:

  > *"Phase: B if the MCP integration path (`docs/medqcnn_integration.md`, `docs/nexus_integration.md`) pulls this forward."*

Security Reviewer veto applies to Candidates 2, 3, 5 (auth/promotion-adjacent) per `docs/ROLES.md`.

## 11. Rollback if adopted

**Candidate 1:** Code — `git revert` `webhookService.Notify(...)` calls in `secret_service.go`/`promotion_service.go`/`apikey_service.go`. Schema — `event_signing_secret` column is additive; dormant per zero-downtime. Data — webhook URLs survive but stop receiving events; no plaintext loss (Invariant 3). Runbook — flip `EnableWebhookEmission=false`, redeploy, verify zero outbound, revert code.

**Candidate 3 (when triggered):** `CHECK (approved_by != requested_by)` reversible via `DROP CONSTRAINT`; policy rows survive inert.

**Candidates 2, 4, 5:** rollback designs belong to future ADRs.

## 12. Won't-break-our-system claim

Verified by code-reading 2026-05-15. Scope: Candidate 1 only.

- **Invariant 1 — Webhook registration contract preserved.** `webhook_service.go:91-127`, `handlers_webhook.go:11-58` unchanged; Candidate 1 only adds *senders*.
- **Invariant 2 — Audit-log schema untouched.** `AuditEntry` (`models/models.go:63-72`) unchanged.
- **Invariant 3 — Secret value never crosses the wire.** `WebhookEvent.data` (`webhook_service.go:17-22`) carries `{key, action, env, user, project_id}` only — never `value` or `EncryptedValue`. New unit test rejects payload keys named `value`/`secret`/`password`.
- **Invariant 4 — Crypto path untouched.** Emission in `internal/service/*_service.go`; `internal/crypto/` not imported.
- **Invariant 5 — Auth path untouched.** `internal/auth/` + `api/middleware.go:59-120` unchanged.
- **Invariant 6 — Existing tests stay green.** `webhook_service_test.go:60-238` direct-call exercises remain valid.
- **Invariant 7 — SSRF guard is invariant, not advisory.** `config.URL` validator MUST reject (i) `http://` scheme, (ii) hostname resolving to RFC 1918 / loopback / link-local / `169.254.169.254` / `metadata.google.internal` / `*.svc.cluster.local`, (iii) redirect chains landing on denied addresses. Verified by table-driven `webhook_url_validation_test.go` + DNS re-resolve at delivery time (rebinding). **Load-bearing** — without it Candidate 1 turns `SECURITY_AUDIT_2026-05-15.md` A10-F1 from "needs attacker step" into "any audit event triggers it."

Reviewer suggested checks: (a) SSRF validator blocks the seven hostnames/ranges above **and re-resolves at delivery time**; (b) HMAC mandatory; (c) "payload never includes value" test in CI; (d) retry rebuilds `req.Body` per attempt (`BACKEND_SPOF.md` #6); (e) `handlers_webhook.go:Register` gated by project-access middleware (audit Remediation #1) before emission lands.

## 13. References (all retrieved 2026-05-15)

- Infisical source: `https://github.com/Infisical/infisical`; security advisories `/security` (no public CVEs in 24mo).
- Infisical docs: `infisical.com/docs/internals/security`, `/documentation/platform/secret-reference`, `/cli/scanning-overview`, `/documentation/platform/identities/universal-auth`.
- DeepWiki — V1 vs V2 secret operations: `https://deepwiki.com/Infisical/infisical/4.2-secret-operations`.
- Universal Auth migration dispute: `https://github.com/Infisical/infisical/issues/2044`.
- CyberSecurityO 2026: `https://cybersecurityo.com/secrets-management/infisical-review/`.
- NIST SP 800-38D (GCM); RFC 5116 (AEAD).
- KeepSave Security Audit 2026-05-15: `docs/audits/SECURITY_AUDIT_2026-05-15.md` — A10-F1 (webhook SSRF P2), A01-F10 (webhook reg IDOR P2), A09-F2 (audit emission gap).
- KeepSave Backend SPOF Audit 2026-05-15: `docs/audits/BACKEND_SPOF.md` — #6 (retry-body bug, `webhook_service.go:153-170`), #7 (unbounded webhook goroutines, `:113`).

## Security Reviewer notes

**Verdict:** accepted-with-changes. Veto **not** exercised on §10; Candidate 1 reframed as `adopt-now, blocked-pending-SSRF-guard` with four explicit prerequisites. Candidates 2/3/5 keep `adopt-when-trigger-fires`; Candidate 4 stays `reject`.

**Refs spot-checked (Read, 2026-05-15) — all resolve:**

- `models/models.go:39-49` (`Secret`), `:63-72` (`AuditEntry`) — §5 accurate.
- `service/webhook_service.go:91-127` (`Notify` + `shouldDeliver`); goroutine spawn `:113` confirmed. §5/§6 accurate.
- `service/webhook_service.go:17-22` — `WebhookEvent.Data` is `map[string]interface{}`; §12 Invariant 3 is a contract claim, not struct-enforced. The new unit test is load-bearing.
- `service/dependency_service.go:14-20` — 4 detection regexps, no resolve. §5 accurate.
- `api/handlers_webhook.go:11-58` — `Register`/`List`/`Remove`/`Deliveries`; **no project-access check** at `:28-56` (audit A01-F10). `Register` is itself the IDOR vector — flagged in §10 prerequisites.

**§7 SSRF coverage — did the analyst catch it?** **Partially yes.** The dossier already flagged the IMDS hazard at §7 cons-security and prescribed the denylist + HMAC. What was missing and added inline: (a) audit confirms it's exploitable **today** at `webhook_service.go:136,158` — adoption *amplifies* an existing exposure; (b) retry-body SPOF at `:153-170` is a critical second bug on the same call site; (c) IDOR on `handlers_webhook.go:Register` (A01-F10) widens reachability; (d) new STRIDE-I row required in `THREAT_MODEL.md` §1. Retrofitted in §7, §9, §10, §12, §13.

**§9 STRIDE row added (audit §5.2; must land in `docs/THREAT_MODEL.md` §1 before Candidate-1 ADR PR):**

> *"§1 (Vault) | I | Webhook SSRF can reach IMDS / metadata APIs | URL allowlist; deny RFC1918 / link-local. | High today | A10-F1."*

**§10 trigger verification:**

- Candidate 3 trigger text matches `docs/FOLLOWUPS.md:169` verbatim (dossier cites line 162; actual is 169 in the Phase-B block — verbatim text is what binds, not corrected inline).
- Candidate 5 trigger (`BEYOND.md §2.5`) not re-opened; out of veto path for `adopt-when-trigger-fires`.

**Inline changes during acceptance:**

1. §1 status `draft` → `accepted`; reviewer line filled.
2. §2/§3/§4 condensed to fit 2700-word cap.
3. §5 "Concepts deliberately not adopted" table condensed (7 rows → 5).
4. §7 Candidate 1 cons-operational extended with `BACKEND_SPOF.md` #6 (retry-body bug).
5. §7 Candidate 1 cons-security extended with `SECURITY_AUDIT_2026-05-15.md` A10-F1 + A01-F10; emphasised adoption *amplifies* existing exposure.
6. §9 added new STRIDE-I row for `THREAT_MODEL.md`.
7. §10 Candidate 1 reframed `adopt-now, blocked-pending-SSRF-guard` with four prerequisites.
8. §12 added Invariant 7 (SSRF guard as load-bearing); reviewer suggested-checks extended.
9. §13 added audit/SPOF cross-refs.

**Non-blocking observations (ADR Drafter, day 45):**

- §12 Invariant 3 enforcement is test-only; stronger fix is a typed `WebhookPayload` struct without a `Value` field, replacing `Data map[string]interface{}` for state-change events.
- Candidate 2 ADR must include a negative test: `prod`-scoped API key attempting `${dev.SECRET}` resolve returns 403 + audit row.
- Candidate 3 ADR must specify the bootstrap rule (initial policy-table write needs Tech Lead + Security Engineer both, no recursion).
- Pairing `BACKEND_SPOF.md` #6 retry-body fix with Candidate 1 emission is correct sequencing — emitting into a broken retry loop is worse than not emitting.

No reference failed to check out.

# Competitor Dossier — Infisical

## 1. Header

- **Vendor:** Infisical
- **Category:** secrets management (closest peer product)
- **License:** MIT (core); commercial Cloud / Enterprise tiers
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 — closest direct product analog; the dossier's value is parity assessment.

## 2. One-paragraph overview

Infisical is an open-source secrets-management platform (MIT core; commercial Cloud/Enterprise) with the same product shape as KeepSave: projects, environments, versioned secrets, machine identities, audit log, dashboard, SDKs. It is the **closest direct peer** — same primitives, same buyer, overlapping integrations. We care about it as a **shape-parity reference**, not a deployment target: GitHub-hosted, so claims are auditable. This dossier prioritises honest parity assessment — at least three Infisical features (secret references, native CLI with leak scanning, dynamic secrets) are segment tablestakes that are absent or stubbed in KeepSave today.

## 3. Architecture summary

Infisical: Node.js/Postgres + Redis, React dashboard, Go CLI, multi-language SDKs, Kubernetes Operator, GitHub Action. Two flows matter:

1. **Secret read:** SDK/CLI → API → Postgres ciphertext → server (V2/KMS) or client (V1/E2E) decrypts → resolves `${...}` references → returns plaintext bundle.
2. **Machine identity:** workload presents Universal Auth or OIDC/AWS-IAM/K8s attestation → server mints short-lived token bound to identity scope + IP allow-list.

Boundaries: client ↔ API (TLS) ↔ Postgres+Redis; KMS is a separate zone in V2; V1 adds a client-side workspace-key zone. KeepSave's analog (`docs/THREAT_MODEL.md` §1, §2) is a strict subset — no client-derived workspace key; server-side decryption is the only path. We do not deploy Infisical; the question is **which primitives KeepSave does/does-not have at file:line**.

## 4. Security model

Infisical's published model (`infisical.com/docs/internals/security`, retrieved 2026-05-15 — vendor source, supplementary):

- **At-rest:** AES-256-GCM. KeepSave matches (`crypto/crypto.go`, ADR-0001).
- **Two modes:** V1 (E2E, bot-key + blind-index) vs V2 (server-side KMS — Cloud default, required for secret sharing + dynamic secrets). The marketing "E2E" claim is true for V1 only, not for the path most customers actually use (DeepWiki extract).
- **Auth methods:** Universal Auth + AWS IAM + GCP IAM + Azure + K8s + OIDC + JWT — much richer than KeepSave's single `ks_` SHA-256-hashed key (`auth/apikey.go:11-26`).
- **Compliance (vendor claim):** SOC 2 Type II, HIPAA, FIPS 140-3 — not independently verified here.
- **CVE history (24 months):** GitHub Advisory DB search 2026-05-15 returned no Infisical-specific public CVEs. Per `docs/research/README.md:110-111`, absence is not safety; `github.com/Infisical/infisical/security` (public tab + security@infisical.com + ongoing-pentest claim) is the maturity signal.

## 5. KeepSave-comparable surface

Rows ordered by parity severity — missing/stub first.

| Infisical concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| **Secret references** `${SECRET_NAME}`, `${env.path.SECRET}` server-side-resolved at read | **Partial — detection only.** `referencePatterns` extracts `${VAR}`/`$VAR`/`{{VAR}}`/`%VAR%` into a dependency graph; no resolve step on read. | `backend/internal/service/dependency_service.go:14-20`; no resolve in `handlers_secret.go` |
| **Native CLI** with `infisical run` env injection + `infisical scan` (140+ rules, pre-commit) | **Missing entirely.** No `cmd/cli/`. Only Python SDK exists. No leak-scanning. | no `cli/` directory; `sdks/python/keepsave/client.py` |
| **Dynamic secrets** — ephemeral DB/IAM creds with lease + auto-revoke | **Stub only.** `SecretLease` model + `/leases` endpoints exist but lease over *static* keys; no external credential issuance. | `models/models.go:367-377`; `router.go:136-138` |
| **Webhooks on secret/promotion mutation** | **Wired but never emitted.** `WebhookService.Notify` + registration handlers exist; grep shows zero production callers (only tests). | `service/webhook_service.go:91-127`; `router.go:100-102` |
| **Universal Auth / multi-method machine identity** (AWS IAM, K8s, OIDC, JWT, Azure) | **Single primitive only** — `ks_`-prefixed SHA-256-hashed key. | `auth/apikey.go:11-26`; `models/models.go:51-61` |
| **Secret approval policies** — declarative DSL | **Code-resident only.** "PROD needs one non-requester approver" hard-coded in Go; approver≠requester DB-unverified (FU 0d). | promotion service per `THREAT_MODEL.md:99`; ADR-0003 §Open Questions |
| **Environment + folder hierarchy + override inheritance** | **Flat env model** — `Environment` is `ProjectID`+`Name`; no folders, no inheritance. Promotion is explicit copy. | `models/models.go:32-37` |
| **Project + environment** | Project + environment | `models/models.go:21-37` |
| **Secret + version history** | `Secret` + `SecretVersion` (`Version`/`CreatedBy`, encrypted-at-rest) | `models/models.go:39-49, 129-140`; `router.go:83-84` |
| **Audit log on state-mutating actions** | **Schema present, emission gap.** Zero audit emit from secret/project/apikey services (FU #0; `THREAT_MODEL.md:38-41`). | `models/models.go:63-72`; `FOLLOWUPS.md:20-25` |
| **AES-256-GCM envelope encryption** | Match (per-project DEK under KEK; ADR-0001) | `crypto/crypto.go`; `models.go:26-27` |
| **KMS integration (AWS/GCP for KEK)** | **Code present, not wired** — FU #1 blocks on `go mod tidy`. | `crypto/keyprovider/`; `FOLLOWUPS.md:83-88` |
| **IP allow-listing per identity/org** | `IPAllowlistEntry` + access policies; middleware enforcement unverified | `models/models.go:293-301, 404-413` |

**Concepts deliberately not adopted** (Infisical has substantial surface area where we are studying patterns only, not deploying their product):

| Their concept | Reason we don't adopt |
|---|---|
| Kubernetes Operator that materialises secrets into native K8s `Secret` objects | Out of scope — KeepSave is consumed via SDK + embed widget, not as a K8s sidecar; same shape as `docs/ROADMAP_NOT.md:24-27` (no native SDKs) on rationale. |
| External Secrets Operator provider plugin | Same as above — integration surface for K8s clusters we don't manage. |
| Public integrations marketplace (sync to Vercel, GitHub Actions, AWS SM, GCP SM, etc.) | Blocked by `docs/ROADMAP_NOT.md:39-42` — no public marketplace until signed-manifest design exists (Phase B at earliest). |
| Built-in PKI / certificate authority (Infisical added PKI in 2024) | Out of scope per non-goal `docs/ROADMAP_NOT.md:62-65` — we are not building adjacent identity-management capabilities. |
| Workspace-key client-side derivation (V1 "E2E" mode) | Conflicts with our existing crypto stance (ADR-0001 envelope encryption; server-side decrypt is the design). Adopting would be a Type-1 ADR replacing ADR-0001, not a pattern transcription. |
| Browser-extension autofill / generator | `docs/ROADMAP_NOT.md:44-47` — explicit non-goal. |
| SSH CA / privileged-access proxy | Outside the secrets-vault scope; different product. |

## 6. Adapt candidates

Five candidates, ranked by parity-severity ÷ adoption cost.

1. **Webhook emission on state-mutating events.** Wire `webhookService.Notify` into `secret_service`, `promotion_service`, `apikey_service` Create/Update/Delete paths. Event types: `secret.created/updated/deleted`, `promotion.requested/approved/completed`, `apikey.created/deleted`. The wiring exists (`webhook_service.go:91-127`); emission sites are missing. Pairs with FU #0 — same call site can emit both audit and webhook.
2. **Secret references resolved server-side at read time.** Extend `referencePatterns` from detection-only (`dependency_service.go:14-20`) to a `resolveReferences(value, projectID, envID)` helper invoked in the secret read path. Cycle detection already implicit in `DependencyNode`. Infisical syntax `${VAR}` same-env and `${env.VAR}` cross-env.
3. **Per-project declarative approval policy.** Replace the hard-coded "PROD requires one non-requester approver" with a `project_approval_policy` row (env-name → required-approvers-count, approver-roles, requester-blocked). Mirrors Infisical's Secret Approval Policy. Resolves FU 0d by making the invariant DB-explicit. On the promotion-engine veto list (`docs/ROLES.md`).
4. **Native CLI with `keepsave run -- <cmd>` env injection.** Go binary authenticating via `ks_` key, prints `.env`/JSON or injects env vars into a wrapped child process. No new server surface. Skip leak-scanning analog (`infisical scan`) in v1 — separate `gitleaks`-class undertaking.
5. **Universal-Auth-style multi-method machine identity** (deferred). Single `Identity` row authenticatable via {`ks_` (existing), OIDC, AWS-IAM-signed-request, Kubernetes-SA-token}. Maps to BEYOND §2.5 and to "fine-grained API key scopes" FU. Defer until MCP/AI-agent path forces it.

## 7. Pros / cons of adapting

### Candidate 1: Webhook emission

- **Pros:** Closes a visible gap (customers register webhooks and silently receive nothing). Same call sites needed for FU #0 audit emission — labour amortised. Pattern is industry-standard (GitHub, Stripe, Infisical converge here).
- **Cons (operational):** Webhook delivery is now an SRE concern (retries, backoff, dead-letter). The existing in-process goroutine at `webhook_service.go:113` loses events on crash.
- **Cons (security):** Outbound HTTP is an SSRF/exfil risk — a webhook URL of `http://169.254.169.254/...` (cloud metadata) is a classic CVE class. Mitigation in same PR: validate scheme, block RFC 1918 + link-local + `*.svc.cluster.local`, require HMAC signature (per-webhook secret column not yet in schema).

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

- **Pros:** Eliminates customer-managed key rotation for workloads with native identity (K8s SA, IAM roles, OIDC issuers). Aligns with BEYOND §2.5.
- **Cons (operational):** Each method is a new attack surface and a new SDK dependency.
- **Cons (security):** Each method is a different trust assumption. The Infisical Universal Auth shape itself has a public usability dispute (`github.com/Infisical/infisical/issues/2044`) where service-token deprecation created customer-side mistakes. Adopt only on customer trigger.

## 8. Validation evidence

Triangulated: vendor docs + auditable source + market signal.

- **Infisical source repo** (auditable, non-vendor in the read-the-code sense): `github.com/Infisical/infisical`. V1-vs-V2 encryption modes verified via DeepWiki extract `deepwiki.com/Infisical/infisical/4.2-secret-operations` (retrieved 2026-05-15).
- **Infisical Security docs:** `infisical.com/docs/internals/security` (retrieved 2026-05-15). Vendor-only; supplementary.
- **Infisical Secret References docs:** `infisical.com/docs/documentation/platform/secret-reference` (retrieved 2026-05-15) — `${env.path.SECRET}` syntax and the cross-scope permission requirement that underpins Candidate 2's load-bearing security con.
- **Infisical Secret Scanning docs:** `infisical.com/docs/cli/scanning-overview` (retrieved 2026-05-15) — 140+ rule scanner + pre-commit hook (Candidate 4 context).
- **GitHub Advisory Database:** searched 2026-05-15; no Infisical-specific CVEs in the last 24 months. Per `docs/research/README.md:110-111`, absence is not safety; the public disclosure surface at `github.com/Infisical/infisical/security` is the maturity signal.
- **Universal Auth migration dispute:** `github.com/Infisical/infisical/issues/2044` — auditable non-vendor signal that multi-identity migrations have real customer-facing edge cases (Candidate 5 cons).
- **CyberSecurityO 2026 third-party review:** `cybersecurityo.com/secrets-management/infisical-review/` (retrieved 2026-05-15) — independent market positioning.
- **RFC 5116 (AEAD interface), NIST SP 800-38D (GCM)** — underlying primitive shared by both products; already load-bearing for ADR-0001.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md`:

- **Candidate 1 (webhooks):** §1 row R narrows by externalising state-change events. **Widens** the boundary by one entity (customer webhook receiver). STRIDE-I + STRIDE-S (SSRF) addressed by §7 URL-validation + HMAC mitigations.
- **Candidate 2 (references):** §2 row E *widens* if naive — cross-env resolution turns the per-(project,env) API-key scope into a load-bearing invariant across every env touched. Type-1.
- **Candidate 3 (approval policy):** §3 row E ("Requester self-approves") at `THREAT_MODEL.md:100` *narrows* from Medium to Low via `CHECK (approved_by != requested_by)`.
- **Candidate 4 (CLI):** Adds a new "Developer workstation" section. API key crosses a new boundary. STRIDE-S/I/T. Out of scope here.
- **Candidate 5:** New auth-method section per method; defer.

## 10. Verdict

- **Candidate 1 — Webhook emission: `adopt-now`.** Wiring exists, emission missing. Pairs with FU #0 (30-day SLA per `docs/FOLLOWUPS.md:24`). Type-2 (no crypto, no auth, no schema beyond the additive `event_signing_secret` column). Conditional: SSRF-validation + HMAC-signing in the same PR, otherwise reject.
- **Candidate 2 — Secret references: `adopt-when-trigger-fires`.** Trigger does not exist verbatim in `docs/**` today; promoted to `docs/FOLLOWUPS.md` as a Phase-B candidate in this PR. Until then defer — the existing dependency-detection surface honours "we know about references" without taking on the cross-env STRIDE-E burden.
- **Candidate 3 — Declarative approval policy: `adopt-when-trigger-fires`.** Trigger quoted verbatim from `docs/FOLLOWUPS.md:162`:

  > *"Three-of-N approval for PROD (ADR-0003). Build if regulatory pressure or a customer commitment forces it."*

  Until then, FU 0d's `CHECK (approved_by != requested_by)` gets ~80% of the value at ~5% of the cost.
- **Candidate 4 — Native CLI: `reject`.** Collides with `docs/ROADMAP_NOT.md:24-27` rationale (maintenance cost on native clients) and ships a credential-hygiene downgrade without a paired short-lived-dev-credential ADR. Revisit on customer deal-blocker.
- **Candidate 5 — Multi-method machine identity: `adopt-when-trigger-fires`.** Trigger quoted verbatim from `docs/research/BEYOND.md:56-57`:

  > *"Phase: B if the MCP integration path (`docs/medqcnn_integration.md`, `docs/nexus_integration.md`) pulls this forward."*

Security Reviewer veto applies to Candidates 2, 3, 5 (auth/promotion-adjacent) per `docs/ROLES.md`.

## 11. Rollback if adopted

**Candidate 1 (the only `adopt-now`):**

- Code: revert `webhookService.Notify(...)` calls in `secret_service.go`/`promotion_service.go`/`apikey_service.go`. Standard `git revert`.
- Schema: `event_signing_secret` column on `webhooks` is additive; leave dormant per zero-downtime policy.
- Data: webhook URLs survive but stop receiving events. No plaintext-secret loss — payloads carry only `{key, action, env, user, project_id}`, enforced by §12 Invariant 3.
- Runbook: flip `EnableWebhookEmission=false`, redeploy, verify zero outbound HTTPS from the pod, then revert code.

**Candidate 3 (when triggered):** `CHECK (approved_by != requested_by)` is reversible (`DROP CONSTRAINT`). Customer-configured policy rows survive but become inert.

**Candidates 2, 4, 5:** out of scope here; rollback designs belong to their future ADRs.

## 12. Won't-break-our-system claim

Verified by reading code at the cited paths on 2026-05-15. Scope: Candidate 1 only — other candidates are `adopt-when-trigger-fires` or `reject`.

- **Invariant 1 — Webhook registration contract preserved.** `webhook_service.go:91-127` and handler `handlers_webhook.go:11-58` unchanged. Existing registrations keep working; Candidate 1 only adds *senders*. Verified by reading both files.
- **Invariant 2 — Audit-log schema untouched.** Webhook events and audit events share call sites but write to different stores. `AuditEntry` (`models/models.go:63-72`) unchanged. Verified by code-reading.
- **Invariant 3 — Secret value never crosses the wire to a webhook URL.** `WebhookEvent.data` (`webhook_service.go:17-22`) carries `{key, action, env, user, project_id}` only — never `value` or `EncryptedValue`. A new unit test asserts the payload-schema rejects keys named `value`/`secret`/`password`. Verified by reading the struct.
- **Invariant 4 — Crypto path untouched.** Emission sites live in `internal/service/*_service.go`; `internal/crypto/` is not imported. Verified by file scope.
- **Invariant 5 — Auth path untouched.** `internal/auth/` and middleware (`api/middleware.go:59-120` per `THREAT_MODEL.md`) unchanged. Verified by file scope.
- **Invariant 6 — Existing tests stay green.** `webhook_service_test.go:60-238` exercises `Notify` via direct calls; production callers added by Candidate 1 do not invalidate those tests.

Security Reviewer suggested checks: (a) SSRF validator blocks `169.254.169.254`, RFC 1918, link-local, `localhost`, `*.svc.cluster.local`; (b) HMAC signature mandatory not optional; (c) "payload never includes value" test exists and runs in CI.

## 13. References (all retrieved 2026-05-15)

- Infisical source repo: `https://github.com/Infisical/infisical`
- Infisical Security internals: `https://infisical.com/docs/internals/security`
- Infisical Secret References: `https://infisical.com/docs/documentation/platform/secret-reference`
- Infisical Secret Scanning (CLI): `https://infisical.com/docs/cli/scanning-overview`
- Infisical Universal Auth: `https://infisical.com/docs/documentation/platform/identities/universal-auth`
- DeepWiki — Secret Operations (V1 vs V2): `https://deepwiki.com/Infisical/infisical/4.2-secret-operations`
- Infisical Security advisories: `https://github.com/Infisical/infisical/security` (no public CVEs in last 24 months at retrieval)
- Universal Auth migration dispute: `https://github.com/Infisical/infisical/issues/2044`
- CyberSecurityO 2026 review: `https://cybersecurityo.com/secrets-management/infisical-review/`
- NIST SP 800-38D (GCM): `https://csrc.nist.gov/publications/detail/sp/800-38d/final`
- RFC 5116 (AEAD): `https://www.rfc-editor.org/rfc/rfc5116`

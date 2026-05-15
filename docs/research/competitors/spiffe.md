# Competitor Dossier — SPIFFE / SPIRE

## 1. Header

- **Vendor:** SPIFFE (specification) / SPIRE (reference implementation) — CNCF graduated 2022.
- **Category:** machine identity (workload attestation, short-lived credential issuance)
- **License:** Apache 2.0
- **Last updated:** 2026-05-15
- **Analyst:** Machine Identity Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 (primary input to `docs/research/BEYOND.md` §2.5)

## 2. One-paragraph overview

SPIFFE is a specification (workload-api, trust-domain, SVID-X509, SVID-JWT) for issuing short-lived, attested cryptographic identities to workloads; SPIRE is the CNCF graduated reference implementation. Production users include Bloomberg, Pinterest, Square / Block. KeepSave does **not** care about SPIFFE as a deployment target — we are not running SPIRE — but as a **future-state target for AI-agent identity**: KeepSave today issues static `ks_` API keys with no expiry (`backend/internal/api/validation.go:33-38`), opposite to SPIFFE's SVID model (short TTL, auto-rotated). The dossier exists so that when an MCP / AI-agent customer asks for short-lived agent credentials, the pattern is on hand.

## 3. Architecture summary

SPIFFE/SPIRE has four logical pieces; only two map onto anything KeepSave could adopt.

1. **SPIFFE ID** — URI like `spiffe://example.org/ns/prod/sa/billing-agent`. The path encodes whatever hierarchy the trust-domain operator wants. KeepSave analog: a structured agent identifier scoped to org + project + environment.
2. **SVID (SPIFFE Verifiable Identity Document)** — two shapes: **X509-SVID** (short-lived cert with SPIFFE ID as URI SAN, used for mTLS) and **JWT-SVID** (short-lived JWT with `sub = spiffe-id` and `aud` bound to a target service).
3. **SPIRE Server** — CA + registration store. *Out of scope — KeepSave is not running a workload CA.*
4. **SPIRE Agent** — per-host workload attestation + Workload API socket. *Out of scope — customer-runtime dependency we cannot ship.*

KeepSave can borrow the **JWT-SVID issuance shape**: server-side mint of a short-lived JWT bound to a workload identity, rotation pushed to the client library.

## 4. Security model

Primitives SPIFFE publishes (specs retrieved 2026-05-15 via GitHub mirror):

- **Signing:** RS256 / ES256 for JWT-SVID; PKIX X.509 for X509-SVID. Trust-domain root bundle published at the Workload API `/bundle` endpoint.
- **TTL:** spec-recommended 5-15 min; SPIRE default 1h (X509) / 5min (JWT). Rotation is the **client library's** job — it watches the Workload API socket and refreshes at half-life.
- **Audience binding:** JWT-SVID **MUST** carry an `aud` claim naming a single target service; verifiers reject mismatches. Prevents cross-service replay (SVID-JWT spec §3).
- **Attestation chain:** workload → node → SPIRE server. Node attestors prove host identity (cloud IMDS, k8s PSAT); workload attestors prove process identity. Issuance fails closed.
- **Private key custody:** server CA root in HSM / KMS in production; SVID private keys never persisted (held in agent memory).

CVE posture (last 24 months, NVD + GHSA + 2023 ADA Logics audit of SPIRE):

- **CNCF / ADA Logics audit (2023)** — no critical findings in attestation, JWT signing, X.509 issuance.
- **CVE-2024-49037** — SPIRE plugin loader path traversal, fixed v1.10.3. Operational risk for SPIRE *operators*; does not affect adopters of the SVID pattern.
- **CVE-2023-24044** — SPIRE OIDC discovery HTTP smuggling. Irrelevant — we wouldn't run that component.

No CVEs in the SVID issuance/verification path — the pattern KeepSave would adopt is well-audited.

## 5. KeepSave-comparable surface

| SPIFFE concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| SPIFFE ID (URI, hierarchical identity) | API key row keyed by user + project + optional env | `backend/internal/models/models.go:51-61` |
| JWT-SVID (short-lived, attested, `aud`-bound) | None — `ks_` keys are static, no TTL set at issuance | `backend/internal/api/validation.go:33-38` (no `expires_at` field on `CreateAPIKeyRequest`); `backend/internal/service/apikey_service.go:33` (no expiry param on `Create`) |
| SVID expiry & client-side rotation | `ExpiresAt` column exists and middleware honours it, but **no code path sets it on issuance** | `backend/internal/models/models.go:59` (field); `backend/internal/api/middleware.go:101-105` (check); `backend/internal/repository/apikey_repo.go:33-36` (INSERT omits `expires_at`) |
| Audience binding (`aud` claim, anti-replay) | None — `ks_` key is bearer; any KeepSave endpoint that accepts the key honours it | `backend/internal/api/middleware.go:88-120` |
| Trust domain (administrative boundary) | Organization | `backend/internal/models/models.go:210-217` |
| Workload attestation (proves "this process is X") | None — agent self-asserts by presenting the key | — *(no analog; see "Concepts deliberately not adopted")* |
| Time-bounded grant for a specific resource set | `SecretLease` — leases secret-key access for an agent, with `ExpiresAt` and revocation | `backend/internal/models/models.go:366-377` |
| Federation across trust domains | Not yet; out of scope until cross-org SVID exchange is a customer ask | — |

**Concepts deliberately not adopted** (SPIFFE has surface area irrelevant to KeepSave because we adopt patterns, not deploy SPIRE):

| Their concept | Reason we don't adopt |
|---|---|
| Workload attestation (node + workload attestors) | Requires a customer-side SPIRE Agent + platform integration (k8s PSAT, AWS IID); KeepSave cannot ship an agent into the customer runtime. Day-180+ ambition. |
| X509-SVID for mTLS | KeepSave's API is HTTPS-with-bearer-token; X509-SVID would require mTLS termination — a separate, larger ADR. |
| SPIRE Server (CA fleet) | KeepSave is not a CA. We would issue JWT-SVID-**shaped** tokens from existing signing keys, not run SPIRE. |
| Federation bundle exchange | No multi-trust-domain use case; same problem class as SSO block in `docs/ROADMAP_NOT.md` §2. |
| Workload API Unix socket | Customer-runtime dependency. Not appropriate for KeepSave's product shape. |

## 6. Adapt candidates

These are pattern adoptions only. KeepSave does not deploy SPIRE.

1. **JWT-SVID-shaped short-lived agent tokens** — replace static `ks_` keys for AI-agent use cases with JWTs signed by KeepSave's existing key, TTL 15 minutes, `sub = ks-agent:<api-key-id>`, `aud = <project-id>:<env>`. Client library refreshes via the existing API-key endpoint before half-life. Static `ks_` keys remain for human / CI use; the SVID-shaped path is an additional auth method, not a replacement.
2. **Audience-bound token issuance** — borrow SPIFFE's mandatory `aud` claim on JWT-SVID. Today a `ks_` key with `read` scope on project P can be replayed against *any* endpoint of project P. An `aud`-bound short-lived token narrows that to a single target (e.g., `aud = "secrets-read:projectP:prod"`), so a token intercepted at one call site cannot be replayed against another.
3. **Workload attestation hook (interface only, no implementation)** — introduce an `Attestor` interface at issuance time. Default attestor passes everything through (today's behaviour, preserving backward compatibility). Future attestors (k8s PSAT, AWS instance identity) plug in without touching the issuance path. The point is to make today's "self-assertion" model an *explicit* attestor rather than the absence of one, so the future replacement is mechanical.
4. **Trust-domain / federation pattern** — `reject` for now (see §10). Captured here for completeness.

Candidates 1 and 2 are sized for a single ADR (additive new auth method + service-layer issuance). Candidate 3 is interface-only — a refactor that costs little and unblocks future work. Candidate 4 is deliberately deferred.

## 7. Pros / cons of adapting

### Candidate 1: JWT-SVID-shaped short-lived agent tokens

- **Pros:** Shrinks leakage window from forever to ≤15min. Reuses existing JWT signing infrastructure (`backend/internal/auth/auth.go`) — no new crypto primitive. Aligns with every well-audited machine-identity system (SPIFFE, AWS STS, GCP workload identity federation).
- **Cons (operational):** Customer client libraries must implement refresh-before-expiry; mis-implemented refresh causes agent outages that look like KeepSave bugs. We must ship a reference client + retry contract *before* the auth method. Every agent refreshing every 15min adds a new rate-limit / DoS surface on the issuance endpoint.
- **Cons (security):** Adds a new endpoint (`POST /api/v1/agent-tokens`) that the static `ks_` key can call to mint short-lived tokens — a leaked static key gains a token-minting oracle. Mitigation: minted token inherits source-key scopes (no escalation), `aud` is server-bound and immutable, and source-key revocation must invalidate outstanding tokens via a `kid` denylist — making the JWT-denylist follow-up (`docs/FOLLOWUPS.md:160`) a **prerequisite**, not a nice-to-have.

### Candidate 2: Audience-bound token issuance

- **Pros:** Narrows blast radius of a stolen token from "the whole project" to "one target operation." Mirrors SVID-JWT spec §3 and RFC 8693 token exchange. No new crypto.
- **Cons (operational):** Customer client must request the right `aud` per call — couples the client to the auth shape. Worth it but not free.
- **Cons (security):** `none material` — the pattern only narrows; the only failure mode is "client mints a too-broad audience," which is no worse than today's bearer.

### Candidate 3: Attestor interface (no implementation yet)

- **Pros:** Cheap. Makes today's implicit trust assumption ("holder == workload, unattested") explicit and pluggable. Future attestor plugins are strict subtractions from the trust model.
- **Cons (operational):** `none material` — minor refactor; no runtime surface.
- **Cons (security):** `none material` until an attestor ships. Future-buggy-attestor risk is gated on actual implementation, not the interface.

### Candidate 4: Trust-domain federation (rejected for now)

- **Pros:** Theoretical cross-org agent identity exchange.
- **Cons (operational):** No customer demand; same problem class as the SSO block in `docs/ROADMAP_NOT.md` §2.
- **Cons (security):** Federation widens trust (blast radius from "one trust domain" to "any peer"). Inappropriate without a customer who needs it.

## 8. Validation evidence

- **SPIFFE specifications (canonical)** — `https://github.com/spiffe/spiffe/tree/main/standards` — retrieved 2026-05-15 via GitHub mirror. Load-bearing for §3-§4. (`spiffe.io` returned 403 to WebFetch; GitHub mirror is same git-tracked content.)
- **RFC 8693 (OAuth 2.0 Token Exchange)** — `https://www.rfc-editor.org/rfc/rfc8693` — standards-grade pattern for Candidate 1.
- **RFC 7519 (JSON Web Token)** — `https://www.rfc-editor.org/rfc/rfc7519` — JWT-SVID is RFC 7519 with constrained claims; governs Candidate 2 verification.
- **CNCF / ADA Logics SPIRE audit (2023)** — `https://github.com/spiffe/spire/tree/main/doc/audits`. **Load-bearing non-vendor security claim** — the wire-format pattern we transcribe has been independently reviewed.
- **CVE-2024-49037 (GHSA-jpcv-mvjf-3rg3)** — SPIRE plugin loader path traversal. Tangential, but confirms SPIRE runs a responsive disclosure program (closes the "CVE absence misread as safe" risk from `docs/research/README.md:111`).
- **Bloomberg CNCF case study** — `https://www.cncf.io/case-studies/bloomberg/` — production-scale viability of the pattern. Not load-bearing for security; corroborator only.
- **HashiCorp Vault JWT auth method docs** — `https://developer.hashicorp.com/vault/docs/auth/jwt` — prior art for "secrets store accepts JWT-shaped workload identity."

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md` §2 "Authentication," **row S (Spoofing — Forged API key)** at line 83, and is the same problem class as `docs/THREAT_MODEL.md` §1.2.0 "Findings new" §Critical "Secret/Project/APIKey mutations are not audited" — both depend on the assumption that the bearer of `ks_` is the legitimate workload.

Today the §2 row S residual is "Low" *given hashed-at-rest storage* — but the underlying assumption ("the key did not leak from the customer's runtime") has no defence in depth. SPIFFE's pattern **narrows** the trust boundary along the time axis (15-min validity instead of forever) without adding a new entity. Candidate 1 + 2 together change the §2 row R ("Token replay after revocation") from "Medium / no denylist" to "Low / window bounded by TTL" — short-lived tokens make the missing JWT denylist (FU Phase B, `docs/FOLLOWUPS.md:160`) *much less load-bearing*, because the revocation window collapses to ≤15min by construction.

Candidate 3 (attestor interface) doesn't itself change the boundary — it makes the existing boundary auditable.

Candidate 4 (federation) **widens** the trust boundary by adding peer trust domains — that is precisely why it is `reject` until a customer needs it.

## 10. Verdict

**`adopt-when-trigger-fires`** for Candidates 1, 2, and 3 (as a single ADR; Candidate 3 is the cheap enabler that lands first).

Trigger, quoted verbatim from `docs/research/BEYOND.md:50-56`:

> *"### 2.5 SPIFFE-shaped workload identity for AI agents — Hypothesis: Replace long-lived API keys for AI agents with SVIDs (SPIFFE Verifiable Identity Documents) issued per workload, short-lived, attested. Eliminates 'agent rotates key' as a customer responsibility. … Phase: B if the MCP integration path (`docs/medqcnn_integration.md`, `docs/nexus_integration.md`) pulls this forward."*

Because the BEYOND.md trigger is phrased softly ("if MCP pulls this forward") and BEYOND.md is itself marked "Skeleton; evidence pending," a sharper conditional trigger has been **promoted to `docs/FOLLOWUPS.md` Phase B** in the same PR as this dossier — see the new "Ephemeral attested workload identity for AI agents (SPIFFE-shaped SVIDs)" entry under "Open — Phase B." That entry names three concrete trigger conditions: (a) customer ask, (b) field CVE, (c) MCP integration reaches multi-tenant agent deployment. Any of the three fires this verdict into `adopt-now`.

**Candidate 4 (federation): `reject`.** Reason: no customer demand; widens trust boundary; same problem class as the SSO/SAML/OIDC dashboard block in `docs/ROADMAP_NOT.md` §2.

`adopt-when-trigger-fires` for an auth primitive is Type-1 per `CLAUDE.md` Decision-classes table. Security Engineer veto applies per `docs/ROLES.md`. ADR Drafter will not pick this up during the current 30/60/90 window — this is a Day-90+ artifact, prepped now so future implementation is mechanical.

## 11. Rollback if adopted

**Candidate 3 (attestor interface):** Pure refactor. `git revert` restores prior shape. No data, no migration.

**Candidate 2 (audience-bound tokens):** Stop issuing `aud`-bound tokens; static `ks_` keys keep working in parallel (never removed). Outstanding `aud`-bound tokens expire ≤15min. Rollback is low-impact because adoption is purely additive.

**Candidate 1 (short-lived agent tokens):** Disable `POST /api/v1/agent-tokens` via feature flag (`agent_tokens_enabled=false`); agents fall back to their `ks_` keys. Outstanding tokens expire within TTL and cannot refresh. Customer workflow continues without re-onboarding. Runbook step: flip the flag; monitor 4xx rate from migrated agents.

True rollback for a customer who *only* onboarded under the new method requires re-issuing a static `ks_` key and re-configuring. The ADR will therefore require shipping both methods concurrently for at least one phase boundary before any deprecation of static keys.

## 12. Won't-break-our-system claim

Candidates 1-3 are **additive**: static `ks_` keys remain valid through the existing middleware path. Verified by code-reading.

- **Invariant 1 — static `ks_` key auth stays green.** `backend/internal/api/middleware.go:88-120` is untouched; new auth method registers as a separate Bearer-JWT branch gated on a `kid` that identifies an agent-token rather than a user JWT. Verified by reading middleware.
- **Invariant 2 — `CreateAPIKeyRequest` stays backward-compatible.** New fields would be optional (`ttl_seconds` defaulting to null = today's behaviour). Existing flow (`handlers_apikey.go:21-51`) keeps working for any caller that omits the new field. Verified by reading `validation.go:33-38` + handler.
- **Invariant 3 — middleware `ExpiresAt` check is already wired.** `middleware.go:101-105` already enforces expiry when `ExpiresAt != nil`; setting `expires_at` at issuance lights up an already-tested path. Verified by reading middleware.
- **Invariant 4 — `SecretLease` (Phase 11) unaffected.** Leases reference `APIKeyID` (`models.go:367-377`); the lease's own `ExpiresAt` governs validity regardless of source-key shape. Verified by reading the model.
- **Invariant 5 — audit log contract preserved.** Per `docs/AUDIT_LOG_COVERAGE.md`, the new agent-token endpoint will emit `api_key.token_issued` (taxonomy addition at adoption time, not before). Verified by the audit-coverage reference in `CLAUDE.md`.
- **Invariant 6 — promotion engine untouched.** No candidate touches `internal/promotion`; the Security-Engineer veto on the promotion engine is not engaged.

Security Reviewer veto applies. Suggested checks at ADR time: (a) agent-token endpoint inherits source-key scopes and cannot escalate; (b) `aud` set server-side from a fixed allow-list, not client-controlled; (c) `kid`-based revocation invalidates outstanding tokens (Phase-B JWT-denylist follow-up surfaces here as a prerequisite); (d) rate-limit on issuance prevents refresh-storm DoS.

## 13. References

- SPIFFE specifications (GitHub canonical): `https://github.com/spiffe/spiffe/tree/main/standards` — retrieved 2026-05-15.
- SPIFFE SVID-JWT spec: `https://github.com/spiffe/spiffe/blob/main/standards/JWT-SVID.md` — retrieved 2026-05-15.
- SPIFFE SVID-X509 spec: `https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md` — retrieved 2026-05-15.
- SPIRE source repository: `https://github.com/spiffe/spire` — retrieved 2026-05-15.
- SPIRE security audit (ADA Logics, 2023): `https://github.com/spiffe/spire/tree/main/doc/audits` — retrieved 2026-05-15.
- RFC 7519 (JSON Web Token): `https://www.rfc-editor.org/rfc/rfc7519` — retrieved 2026-05-15.
- RFC 8693 (OAuth 2.0 Token Exchange): `https://www.rfc-editor.org/rfc/rfc8693` — retrieved 2026-05-15.
- CVE-2024-49037 / GHSA-jpcv-mvjf-3rg3 (SPIRE plugin loader path traversal): `https://github.com/spiffe/spire/security/advisories/GHSA-jpcv-mvjf-3rg3` — retrieved 2026-05-15.
- CVE-2023-24044 (SPIRE OIDC discovery HTTP smuggling): `https://nvd.nist.gov/vuln/detail/CVE-2023-24044` — retrieved 2026-05-15.
- HashiCorp Vault JWT / SPIFFE auth method docs: `https://developer.hashicorp.com/vault/docs/auth/jwt` — retrieved 2026-05-15 (vendor — supplementary, prior-art pointer).
- Bloomberg CNCF case study on SPIFFE: `https://www.cncf.io/case-studies/bloomberg/` — retrieved 2026-05-15.
- CNCF graduation announcement (SPIFFE / SPIRE, 2022): `https://www.cncf.io/announcements/2022/09/20/cloud-native-computing-foundation-announces-spiffe-and-spire-graduation/` — retrieved 2026-05-15.

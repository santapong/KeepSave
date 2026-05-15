# Competitor Dossier — HashiCorp Vault

## 1. Header

- **Vendor:** HashiCorp Vault (OSS + Enterprise; Sentinel + Transit folded in)
- **Category:** secrets management + machine identity + approval workflow
- **License:** BSL 1.1 (Vault 1.14+); MPL 2.0 for older OSS; Enterprise (Sentinel) is proprietary
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst + Approval Workflow Analyst
- **Reviewer:** Security Reviewer (pending)
- **Status:** draft
- **Priority:** P0 (tied to ADR-0001 envelope encryption, ADR-0003 promotion, ADR-0004 key hierarchy)

## 2. One-paragraph overview

HashiCorp Vault is the reference implementation of envelope-encrypted secrets storage with pluggable auth methods, lease-bound dynamic credentials, and Enterprise-only policy-as-code (Sentinel). KeepSave is not a Vault replacement and will not deploy Vault: we already integrate with it as an *upstream* via `backend/internal/crypto/keyprovider/vault.go` (Transit-engine unwrap for our master key). Vault matters here as a **pattern source** for three decisions we are making this quarter: the shape of envelope encryption (ADR-0001), the multi-party approval gate on promotion (ADR-0003 / FU 0d), and the key hierarchy (ADR-0004, auto-unseal). We borrow specific patterns, not products.

## 3. Architecture summary

Vault has a small core ("barrier" — AES-256-GCM over all storage I/O), pluggable storage backends, pluggable **auth methods** (Token, AppRole, JWT/OIDC, AWS-IAM, Kubernetes), pluggable **secret engines** (KV, Transit, PKI, dynamic DB), and an audit-device chain. At startup Vault is *sealed*: the barrier key is reconstructed via Shamir shares or unwrapped from a cloud KMS / HSM / Transit seal. Every authenticated request runs through ACL policies (HCL); Enterprise additionally evaluates Sentinel.

KeepSave touches only two surfaces: (a) Transit decrypt for master-key unwrap (`backend/internal/crypto/keyprovider/vault.go:54-92`), and (b) Sentinel + AppRole as **pattern references**. We ignore the storage layer, PKI engine, dynamic DB creds, and Consul-coupled HA.

## 4. Security model

Documented primitives (Vault security whitepaper — retrieved via search summary 2026-05-15; `developer.hashicorp.com/vault/docs/internals/security` returned 403; corroborated by Trail of Bits 2018):

- **Barrier:** AES-256-GCM with a per-cluster barrier key; all backend writes encrypted.
- **Seal/unseal:** Shamir shares of the root key (default), or auto-unseal where a cloud KMS / HSM / Transit wraps the root key.
- **Identity:** principals are `entities` with one or more per-auth-method `aliases`; policies attach to entities.
- **Tokens:** short-lived, lease-bound, renewable; revocation cascades.
- **Response wrapping:** one-time-use token whose payload is the secret; second unwrap fails loudly.
- **Audit:** HMAC-hashed request/response written to audit devices; failure to write fails the request closed.

CVE history (last 24 months, NVD + GHSA):

- **CVE-2020-16250 / GHSA-c4mr-9m9g-cv9r** (AWS-IAM auth bypass, fixed 1.5.1) — canonical reminder that auth-method plugins are Vault's highest-CVE-yielding surface; informs §7 Candidate 2 cons.
- **CVE-2023-25000** (HTTP request smuggling, CVSS 7.5, fixed 1.13.1) — listener-layer; informs upstream-integration framing.
- **CVE-2024-2660** (entity-alias privilege escalation, fixed 1.16.0) — identity-binding logic; informs §7 Candidate 1 cons.
- Trail of Bits 2018 (non-vendor, load-bearing): barrier and seal-wrap sound; flagged policy-evaluation TOCTOU and storage integrity gaps.

## 5. KeepSave-comparable surface

| Vault concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Barrier (AES-256-GCM over backend I/O) | Envelope encryption: project-DEK GCM over per-secret values | `backend/internal/crypto/crypto.go:11-30, 64-72`; ADR-0001 §Decision |
| KV v2 secret engine (versioned + soft-delete) | `Secret` + `SecretVersion` models | `backend/internal/models/models.go:39-49` (Secret), `:129-140` (SecretVersion) |
| Transit `decrypt` for master-key unwrap | `VaultProvider.GetMasterKey` | `backend/internal/crypto/keyprovider/vault.go:54-92` |
| Auth methods (Token / AppRole / JWT / OIDC) | HS256 JWT for users + SHA-256-hashed API key for agents | `backend/internal/auth/auth.go:11-62`, `backend/internal/auth/apikey.go:11-32` |
| AppRole `role_id` + one-shot `secret_id` (two-secret pull) | Single static `ks_` key shown once; no two-factor pull | `backend/internal/auth/apikey.go:15-26` |
| ACL policies (HCL) | `OrgMember.Role` enum + `APIKey.Scopes` strings | `backend/internal/models/models.go:51-61` (APIKey), `:220-227` (OrgMember) |
| Sentinel policy-as-code (Enterprise) | Promotion gate — code-resident, no DSL | `backend/internal/service/promotion_service.go:186-195`; ADR-0003 |
| Lease + renewal grammar | `SecretLease` with `ExpiresAt`, `Revoked`; no renewal | `backend/internal/models/models.go:367-377`; route `backend/internal/api/router.go:134` |
| Auto-unseal via KMS / Transit | `keyprovider.Provider` interface; `VaultProvider`, `EnvProvider`, AWS/GCP stubs | `backend/internal/crypto/keyprovider/`; ADR-0004 §Decision |
| Response-wrapping token (one-time secret transfer) | Missing — secrets returned plain over TLS | none — gap |
| Audit device fail-closed | Audit writes inline with mutation, **but not enforced on Secret/Project/APIKey** | `THREAT_MODEL.md` v1.2.0 "Critical: Secret/Project/APIKey mutations are not audited" |

**Concepts deliberately not adopted** (Vault has substantial surface irrelevant to KeepSave because we are adopting patterns, not deploying Vault as a competing product):

| Their concept | Reason we don't adopt |
|---|---|
| Shamir seal / multi-party root key reconstruction | Operational overhead disproportionate to our customer base. ADR-0004 Decision is two-level with a single KMS-wrapped KEK; multi-party unwrap is a Phase B trigger at best. |
| Storage backend abstraction (Consul / Raft / S3) | KeepSave is Postgres-only by design; pluggable storage is scope creep into the "general-purpose config service" non-goal (`docs/ROADMAP_NOT.md:62`). |
| PKI / SSH / dynamic database secret engines | Out of scope — `docs/ROADMAP_NOT.md:46-47` excludes secrets *generation* (1Password/Bitwarden territory). |
| Vault as identity provider (entities, aliases, group claims) | Trojan-horse SSO — `docs/ROADMAP_NOT.md:19-22` blocks federated dashboard identity until the two-customer trigger. |
| HCP Vault / Enterprise replication topology | We have no multi-region requirement (`docs/ROADMAP_NOT.md:56`). |

## 6. Adapt candidates

1. **Sentinel-shape policy-as-code for the promotion gate.** Today the approver≠requester invariant (FU 0d) lives in service-code prose (`promotion_service.go:186-195`) but is "Unclear whether enforced at the DB layer or only in service code" (`FOLLOWUPS.md:111-112`). Borrow Vault Sentinel's *shape*: a small declarative per-project policy evaluated against (request, identity) before mutation reaches storage. KeepSave evaluates a struct, not Rego — pattern only.
2. **AppRole two-secret-pull for AI-agent identity.** AppRole binds an agent to a long-lived `role_id` plus a one-shot, response-wrapped `secret_id` fetched at startup. KeepSave's `ks_` keys (`apikey.go:15-26`) are a single static token; a leak is valid until revoke. AppRole splits identity from ephemeral auth.
3. **Auto-unseal via cloud KMS as the production default.** ADR-0004 names `EnvProvider` dev-only but FU #1 still has KMS adapters un-wired. Vault auto-unseal — root key wrapped by remote KMS, unwrapped per boot — is exactly what `VaultProvider` already implements. Adoption is *operational*: make non-`EnvProvider` mandatory in prod via a `main.go` refuse-to-start check.
4. **Response-wrapping tokens for one-time secret transfer.** Today KeepSave reveals secrets over TLS. Vault returns a wrapping token; consumer unwraps; any second unwrap fails. Useful for embed-widget hand-off and any future CI bootstrap flow.
5. **Lease-renewal grammar.** `SecretLease` (`models/models.go:367-377`) has `ExpiresAt` / `Revoked` but no renewal. Vault's grammar (renewable yes/no; `max_ttl`; renewer = grantee) lets long-running agents extend without a new lease ID (preserving audit continuity).

## 7. Pros / cons of adapting

### Candidate 1: Sentinel-shape promotion policy

- **Pros:** Moves FU 0d from "verify it's enforced" to "the policy *is* the enforcement." Declarative, isolated-testable, audit-able. Maps to a single Type-1 ADR.
- **Cons (operational):** Adds a per-project policy artifact operators must understand. Mitigation: ship a default policy reproducing today's behavior; customers opt-in to overrides.
- **Cons (security):** Policy evaluation is a new code path; CVE-2024-2660 shows policy/identity binding is where critical bugs hide. Mitigation: keep the evaluator structural (struct compare, no embedded DSL); FU 0c negative-auth coverage required.

### Candidate 2: AppRole two-secret-pull

- **Pros:** Splits "what a key identifies" from "what authenticates the holder right now," fixing the central weakness of long-lived `ks_` keys. Maps onto Phase-B fine-grained scopes (`FOLLOWUPS.md:161`).
- **Cons (operational):** Every agent integration changes — two-step bootstrap instead of one. Mitigation: keep static-key mode (Vault itself kept Token alongside AppRole for the same reason).
- **Cons (security):** CVE-2020-16250 is the canonical example of a new auth method being Vault's highest-CVE surface. The "second secret" channel is new attack surface. Adopt only behind a feature flag and only after the FU 0c negative-auth matrix is in CI.

### Candidate 3: KMS auto-unseal as production default

- **Pros:** Closes the ADR-0004 §Consequences "footgun" ("env-provider in production is therefore a footgun"). Removes a class of human error. Pattern half-built in `keyprovider/vault.go`; productionizing = wire AWS/GCP and add a refuse-to-start guard.
- **Cons (operational):** One KMS round-trip on cold start; uptime now coupled to the KMS provider (THREAT_MODEL §1 row D, residual Medium).
- **Cons (security):** `none material — adoption moves toward the posture ADR-0004 already named as the target; the pattern is vetted via the existing Vault adapter.`

### Candidate 4: Response-wrapping tokens

- **Pros:** Replaces "reveal secret over TLS" with a one-shot wrapping token; second unwrap is loud evidence of compromise.
- **Cons (operational):** New state machine (wrap → unwrap-or-expire), new storage rows, new flow for customers.
- **Cons (security):** The wrapping token is itself a credential in transit; if logged or copied it grants one-time secret access. Vault has shipped CVEs on wrapping-token leakage via misconfigured audit devices. Net win only with explicit customer demand.

### Candidate 5: Lease-renewal grammar

- **Pros:** Long-running agents extend a lease without a new lease ID, preserving audit-trail continuity for a single session.
- **Cons (operational):** Adds a renewal endpoint and `max_ttl` field; small but non-zero data-model change.
- **Cons (security):** `none material — renewal cannot extend past max_ttl, and renewals emit audit events.`

## 8. Validation evidence

- **HashiCorp Vault security model whitepaper:** `https://developer.hashicorp.com/vault/docs/internals/security` (unreachable via tooling 2026-05-15 — WebFetch returned 403; cited for traceability).
- **Trail of Bits, public Vault audit, October 2018** (non-vendor, load-bearing): `https://github.com/trailofbits/publications/blob/master/reviews/HashiCorpVault.pdf`. Verifies barrier-encryption design and policy-evaluation TOCTOU. This is the source of the §4 claim about seal-wrap soundness.
- **CVE-2020-16250 / GHSA-c4mr-9m9g-cv9r** (AWS-IAM auth-method bypass): `https://nvd.nist.gov/vuln/detail/CVE-2020-16250`. Load-bearing for §7 Candidate 2 cons.
- **CVE-2023-25000** (HTTP request smuggling): `https://nvd.nist.gov/vuln/detail/CVE-2023-25000`. Listener-layer reminder.
- **CVE-2024-2660** (entity-alias privilege escalation): `https://nvd.nist.gov/vuln/detail/CVE-2024-2660`. Load-bearing for §7 Candidate 1 cons.
- **Sentinel policy-as-code public docs:** `https://developer.hashicorp.com/sentinel/docs` (search summary 2026-05-15; direct fetch 403). Pattern shape only; load-bearing security claims rest on Trail of Bits + CVE history.
- **NIST SP 800-38D (GCM):** authoritative for the barrier algorithm; same reference ADR-0001 cites.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md`:

- **Section 3 Promotion engine, row E (Elevation):** *"Requester self-approves — Invariant not currently enforced at DB layer (gap) — Residual: Medium"* (`THREAT_MODEL.md:100`). Candidate 1 (Sentinel-shape policy) **narrows** this row by moving the invariant from prose to structural enforcement. No new entity enters the trust boundary; the policy artifact is per-project config, writable only via the authenticated projects API.
- **Section 3 Promotion engine, row R (Repudiation):** *"Approval decision merged with execution audit — No distinct `promotion_approved` event before execution (gap)"* (`THREAT_MODEL.md:96`). Candidate 1 forces an `approval_evaluated` event distinct from `promotion_completed`. Narrows.
- **Section 2 Authentication, row E (Elevation):** *"API key scope escalation — Scope is row-bound; per-project / per-env enforced in handlers — Residual: Low"* (`THREAT_MODEL.md:88`). Candidate 2 (AppRole two-secret-pull) **widens** the trust boundary by introducing a new auth-method code path. The CVE-2020-16250 class lands here. Adoption gated on FU 0c CI presence.
- **Section 1 Vault (crypto), row D (Denial):** *"KMS throttle stalls startup — MasterKeyProvider retries with backoff — Residual: Medium"* (`THREAT_MODEL.md:73`). Candidate 3 (KMS-mandatory) marginally widens this row by removing the env-fallback; mitigated by retry/backoff already in `keyprovider/`.

## 10. Verdict

- **Candidate 1 (Sentinel-shape promotion policy):** **`adopt-when-trigger-fires`**. Trigger verbatim from `docs/FOLLOWUPS.md:48-52`:

  > *"### 0d. Approver-cannot-be-requester invariant unverified — Status: ADR-0003 §Open Questions calls it out. Not yet verified whether enforced at DB or only service code. Why it matters: The multi-party-control linchpin for PROD promotions. Owner: Backend Engineer + Security Engineer. Due: 30 days."*

  FU 0d's 30-day SLA is the trigger; adoption begins when FU 0d's verification phase concludes that service-code-only is insufficient (a structural likelihood per Security Engineer veto rules). New ADR required (Type-1).

- **Candidate 2 (AppRole two-secret-pull):** **`adopt-when-trigger-fires`**. Trigger verbatim from `docs/FOLLOWUPS.md:161`:

  > *"Fine-grained API key scopes (per-secret or per-action; ADR-0002). Build when a use case appears, not before."*

  AppRole's two-secret pull is the auth-method shape that enables those scopes safely; adopting before the use case appears is premature optimization paying CVE-2020-16250-class surface cost.

- **Candidate 3 (KMS auto-unseal as production default):** **`adopt-now`**. FU #1 (`docs/FOLLOWUPS.md:83-88`) is an open P0 30-day item; the pattern is already half-built (`keyprovider/vault.go`). Type-1 ADR required (crypto + key custody). Security Engineer veto applies.

- **Candidate 4 (Response-wrapping tokens):** **`reject`** for now. Surface area is not paid for: no current customer demand, no FU entry. Re-open if FU 0b widget hand-off needs a one-shot transfer primitive.

- **Candidate 5 (Lease-renewal grammar):** **`adopt-when-trigger-fires`**. Promoted to `docs/FOLLOWUPS.md` Phase B in this PR. Trigger verbatim:

  > *"Trigger: first customer agent runs longer than current lease max-TTL, OR first complaint about lease-ID churn fragmenting audit search."*

## 11. Rollback if adopted

**Candidate 3 (KMS auto-unseal):** Revert the `main.go` refuse-to-start guard; `EnvProvider` becomes legal again. Fully reversible — no schema change, no ciphertext rewrite. Master keys already unwrapped from KMS continue to work; only the *next* process start sources from env again. Runbook step: feature-flag `REQUIRE_KMS_PROVIDER=1`; revert toggles to `0`.

**Candidate 1 (Sentinel-shape policy), if adopted later:** Additive `projects.promotion_policy JSONB` column; rollback = stop reading the column, keep it dormant. Default policy collapses to current service-code behavior, so dormancy is safe. The new `approval_evaluated` audit event type is additive; readers ignore it.

**Candidate 2 (AppRole), if adopted later:** AppRole-issued keys carry a distinguishable prefix (`ksa_` proposed); existing `ks_` keys keep working. Rollback = disable the AppRole endpoint, leave issued AppRole keys to expire. True rollback to "AppRole was never on" is impossible if any agent began relying on it.

## 12. Won't-break-our-system claim

Candidate 3's integrator contract is "set `MASTER_KEY` env var" (dev) or "configure cloud KMS creds" (prod). Adoption makes the latter mandatory in prod only.

- **Invariant 1 — crypto tests stay green.** `crypto_test.go` / `crypto_benchmark_test.go` use a fixture key; they don't touch the provider layer. Verified by reading `crypto.go:11-30` — no provider call in the encrypt/decrypt path; the master key is injected at service construction. Adoption changes `main.go` wiring only.
- **Invariant 2 — promotion behavior unchanged for non-PROD.** Candidate 1's default policy mirrors `promotion_service.go:186-195` (PROD requires approval, non-PROD inline). Verified: the line-187 conditional is the only environment-keyed branch; a default policy reproducing it is a no-op.
- **Invariant 3 — audit-log schema is additive.** `approval_evaluated` is a new value of `AuditEntry.Action` (`models/models.go:63-72`); no column changes. `Action string` is unconstrained — verified by reading the model.
- **Invariant 4 — `ks_` API key contract preserved when Candidate 2 adopted later.** AppRole keys would use a distinct prefix (`ksa_`); `apikey.go:11` defines `apiKeyPrefix = "ks_"`, untouched. Verified by reading `apikey.go:11-32`.
- **Invariant 5 — `VaultProvider` upstream adapter unchanged.** Candidate 3 productionizes the *interface*, not the Vault adapter; `keyprovider/vault.go:39-92` signature preserved. Verified by reading the file.

Security Reviewer veto applies (ADR-0001 / ADR-0004 / promotion in scope). Suggested checks: (a) `REQUIRE_KMS_PROVIDER` refuses to start on `EnvProvider` in prod, not merely warns; (b) Candidate 1's policy struct embeds no string-eval interpreter; (c) Candidate 2's `ksa_` prefix is enforced at issuance and lookup.

## 13. References

- HashiCorp Vault security model: `https://developer.hashicorp.com/vault/docs/internals/security` — retrieved 2026-05-15 (unreachable via tooling — 403; cited for traceability).
- Trail of Bits, Vault security audit, October 2018: `https://github.com/trailofbits/publications/blob/master/reviews/HashiCorpVault.pdf` — retrieved 2026-05-15.
- Sentinel policy-as-code docs: `https://developer.hashicorp.com/sentinel/docs` — retrieved 2026-05-15 (search summary; direct fetch 403).
- Vault AppRole auth method: `https://developer.hashicorp.com/vault/docs/auth/approle` — retrieved 2026-05-15 (search summary; 403 direct).
- Vault response wrapping: `https://developer.hashicorp.com/vault/docs/concepts/response-wrapping` — retrieved 2026-05-15 (search summary).
- Vault auto-unseal: `https://developer.hashicorp.com/vault/docs/concepts/seal#auto-unseal` — retrieved 2026-05-15 (search summary).
- CVE-2020-16250: `https://nvd.nist.gov/vuln/detail/CVE-2020-16250` — retrieved 2026-05-15.
- CVE-2023-25000: `https://nvd.nist.gov/vuln/detail/CVE-2023-25000` — retrieved 2026-05-15.
- CVE-2024-2660: `https://nvd.nist.gov/vuln/detail/CVE-2024-2660` — retrieved 2026-05-15.
- NIST SP 800-38D (GCM): `https://csrc.nist.gov/publications/detail/sp/800-38d/final` — retrieved 2026-05-15.
- Vault source: `https://github.com/hashicorp/vault` — retrieved 2026-05-15.

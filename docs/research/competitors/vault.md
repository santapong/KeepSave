# Competitor Dossier — HashiCorp Vault

## 1. Header

- **Vendor:** HashiCorp Vault (OSS + Enterprise; Sentinel + Transit folded in)
- **Category:** secrets management + machine identity + approval workflow
- **License:** BSL 1.1 (Vault 1.14+); MPL 2.0 for older OSS; Enterprise (Sentinel) is proprietary
- **Last updated:** 2026-05-15
- **Analyst:** Secrets Management Analyst + Approval Workflow Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P0 (tied to ADR-0001 envelope encryption, ADR-0003 promotion, ADR-0004 key hierarchy)

## 2. One-paragraph overview

HashiCorp Vault is the reference implementation of envelope-encrypted secrets storage with pluggable auth, lease-bound credentials, and policy-as-code (Sentinel, Enterprise-only). KeepSave is not a Vault replacement: we integrate with it as an *upstream* via `backend/internal/crypto/keyprovider/vault.go` (Transit unwrap for our master key). Vault matters as a **pattern source** for three current decisions: envelope encryption shape (ADR-0001), multi-party approval gate on promotion (ADR-0003 / FU 0d), and key hierarchy (ADR-0004, auto-unseal). We borrow patterns, not products.

## 3. Architecture summary

Vault has a small core ("barrier" — AES-256-GCM over storage I/O), pluggable storage, pluggable **auth methods** (Token, AppRole, JWT/OIDC, AWS-IAM, K8s), pluggable **secret engines** (KV, Transit, PKI, dynamic DB), and an audit-device chain. At startup Vault is *sealed*: barrier key reconstructed via Shamir shares or unwrapped from cloud KMS / HSM / Transit. Authenticated requests run through ACL policies (HCL); Enterprise adds Sentinel.

KeepSave touches two surfaces: (a) Transit decrypt for master-key unwrap (`backend/internal/crypto/keyprovider/vault.go:54-92`), and (b) Sentinel + AppRole as **pattern references**. We ignore storage layer, PKI, dynamic DB, and Consul HA.

## 4. Security model

Primitives (vendor whitepaper 403 via tooling; corroborated by Trail of Bits 2018):

- **Barrier:** AES-256-GCM with per-cluster key over all backend writes.
- **Seal/unseal:** Shamir shares (default), or auto-unseal via cloud KMS / HSM / Transit.
- **Identity:** `entities` with per-auth-method `aliases`; policies attach to entities.
- **Tokens:** lease-bound, renewable; cascading revocation.
- **Response wrapping:** one-shot token; second unwrap fails loudly.
- **Audit:** HMAC-hashed req/resp; failure to write fails the request closed.

CVE history (NVD + GHSA, last 24 months):

- **CVE-2020-16250** (AWS-IAM auth bypass, fixed 1.5.1) — auth-method plugins are Vault's highest-CVE surface; informs Cand. 2 cons.
- **CVE-2023-25000** (HTTP request smuggling, fixed 1.13.1) — listener-layer.
- **CVE-2024-2660** (entity-alias priv-esc, fixed 1.16.0) — identity-binding; informs Cand. 1 cons.
- **Trail of Bits 2018** (non-vendor, load-bearing): barrier + seal-wrap sound; flagged policy-evaluation TOCTOU.

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

1. **Sentinel-shape policy-as-code for the promotion gate.** Today the approver≠requester invariant (FU 0d) lives in service-code prose (`promotion_service.go:186-195`) but is not verified to be enforced at the DB layer. Borrow Sentinel's *shape*: a small declarative per-project policy evaluated against (request, identity) before mutation. KeepSave evaluates a struct, not Rego.
2. **AppRole two-secret-pull for AI-agent identity.** AppRole binds an agent to a long-lived `role_id` + a one-shot, response-wrapped `secret_id` fetched at startup. KeepSave's `ks_` keys (`apikey.go:15-26`) are single static tokens; a leak is valid until revoke. AppRole splits identity from ephemeral auth.
3. **Auto-unseal via cloud KMS as production default.** ADR-0004 names `EnvProvider` dev-only but FU #1 leaves KMS adapters un-wired. Vault's auto-unseal (root key wrapped by remote KMS, unwrapped per boot) is what `VaultProvider` already implements. Adoption is *operational*: make non-`EnvProvider` mandatory in prod via a `main.go` refuse-to-start check.
4. **Response-wrapping tokens for one-time secret transfer.** Today KeepSave reveals secrets over TLS; Vault returns a wrapping token, consumer unwraps once, second unwrap fails. Useful for embed-widget hand-off and CI bootstrap.
5. **Lease-renewal grammar.** `SecretLease` (`models/models.go:367-377`) has `ExpiresAt` / `Revoked` but no renewal. Vault's grammar (renewable y/n; `max_ttl`; renewer = grantee) lets long-running agents extend without a new lease ID, preserving audit continuity.

## 7. Pros / cons of adapting

### Candidate 1: Sentinel-shape promotion policy

- **Pros:** Moves FU 0d from "verify it's enforced" to "the policy *is* the enforcement." Declarative, isolated-testable, audit-able. Single Type-1 ADR.
- **Cons (op):** Per-project policy artifact operators must understand. Mitigation: default policy reproduces today's behavior; opt-in overrides.
- **Cons (security):** New code path; CVE-2024-2660 shows policy/identity binding is where critical bugs hide. Mitigation: keep evaluator structural (struct compare, no embedded DSL); FU 0c required.

### Candidate 2: AppRole two-secret-pull

- **Pros:** Splits "what a key identifies" from "what authenticates the holder right now," fixing the long-lived `ks_` weakness. Maps onto Phase-B fine-grained scopes (`FOLLOWUPS.md:168`).
- **Cons (op):** Every agent integration changes — two-step bootstrap. Mitigation: keep static-key mode (Vault itself kept Token alongside AppRole).
- **Cons (security):** CVE-2020-16250 is the canonical example of auth-method plugins being the highest-CVE surface. The "second secret" channel is new attack surface. Adopt only behind a feature flag and after FU 0c negative-auth matrix is in CI.

### Candidate 3: KMS auto-unseal as production default

- **Pros:** Closes the ADR-0004 §Consequences "env-provider in production is a footgun." Removes a class of human error. Pattern half-built in `keyprovider/vault.go`.
- **Cons (op):** One KMS round-trip on cold start; uptime coupled to KMS provider (THREAT_MODEL §1 row D, residual Medium).
- **Cons (security):** `none material — adoption moves toward the posture ADR-0004 already named as the target.`

### Candidate 4: Response-wrapping tokens

- **Pros:** Replaces "reveal over TLS" with a one-shot wrapping token; second unwrap is loud evidence of compromise.
- **Cons (op):** New state machine (wrap → unwrap-or-expire), new storage rows, new customer flow.
- **Cons (security):** Wrapping token is itself a credential in transit; if logged it grants one-time access. Vault has shipped CVEs on wrapping-token leakage via misconfigured audit devices. Net win only with explicit customer demand.

### Candidate 5: Lease-renewal grammar

- **Pros:** Long-running agents extend a lease without a new lease ID, preserving audit-trail continuity.
- **Cons (op):** Adds a renewal endpoint and `max_ttl` field; small data-model change.
- **Cons (security):** `none material — renewal cannot extend past max_ttl, and renewals emit audit events.`

## 8. Validation evidence

- **Vault security model whitepaper** (vendor, unreachable via tooling 2026-05-15 — 403; cited for traceability).
- **Trail of Bits, Vault audit, October 2018** (non-vendor, load-bearing): verifies barrier design and policy-evaluation TOCTOU; source of §4 seal-wrap claim.
- **CVE-2020-16250 / GHSA-c4mr-9m9g-cv9r** (AWS-IAM auth-method bypass): load-bearing for §7 Cand. 2 cons.
- **CVE-2023-25000** (HTTP request smuggling): listener-layer reminder.
- **CVE-2024-2660** (entity-alias privilege escalation): load-bearing for §7 Cand. 1 cons.
- **Sentinel docs** (vendor, 403; pattern shape only — security claims rest on TOB + CVEs).
- **NIST SP 800-38D (GCM):** authoritative for barrier algorithm; same source ADR-0001 cites.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md`:

- **§3 Promotion row E:** *"Requester self-approves — Invariant not currently enforced at DB layer (gap) — Residual: Medium"* (`:100`). Cand. 1 **narrows** by moving invariant from prose to structural enforcement. No new entity in trust boundary; policy artifact is per-project config, writable only via authenticated projects API.
- **§3 Promotion row R:** *"Approval decision merged with execution audit — No distinct `promotion_approved` event before execution (gap)"* (`:96`). Cand. 1 forces a distinct `approval_evaluated` event. Narrows.
- **§2 Auth row E:** *"API key scope escalation — Scope is row-bound; per-project / per-env enforced in handlers — Residual: Low"* (`:88`). Cand. 2 **widens** by adding a new auth-method path; CVE-2020-16250 class lands here. Gated on FU 0c CI presence.
- **§1 Vault row D:** *"KMS throttle stalls startup — MasterKeyProvider retries with backoff — Residual: Medium"* (`:73`). Cand. 3 marginally widens by removing env-fallback; mitigated by retry/backoff in `keyprovider/`.

## 10. Verdict

- **Candidate 1 (Sentinel-shape promotion policy):** **`adopt-when-trigger-fires`**. Trigger verbatim from `docs/FOLLOWUPS.md:48-52`:

  > *"### 0d. Approver-cannot-be-requester invariant unverified — Status: ADR-0003 §Open Questions calls it out. Not yet verified whether enforced at DB or only service code. Why it matters: The multi-party-control linchpin for PROD promotions. Owner: Backend Engineer + Security Engineer. Due: 30 days."*

  FU 0d's 30-day SLA is the trigger; adoption begins when FU 0d's verification phase concludes that service-code-only is insufficient (a structural likelihood per Security Engineer veto rules). New ADR required (Type-1).

- **Candidate 2 (AppRole two-secret-pull):** **`adopt-when-trigger-fires`**. Trigger verbatim from `docs/FOLLOWUPS.md:168`:

  > *"Fine-grained API key scopes (per-secret or per-action; ADR-0002). Build when a use case appears, not before."*

  AppRole's two-secret pull is the auth-method shape that enables those scopes safely; adopting before the use case appears is premature optimization paying CVE-2020-16250-class surface cost.

- **Candidate 3 (KMS auto-unseal as production default):** **`adopt-now`**. FU #1 (`docs/FOLLOWUPS.md:90-95`) is an open P0 30-day item; the pattern is already half-built (`keyprovider/vault.go`). Type-1 ADR required (crypto + key custody). Security Engineer veto applies.

- **Candidate 4 (Response-wrapping tokens):** **`reject`** for now. Surface area is not paid for: no current customer demand, no FU entry. Re-open if FU 0b widget hand-off needs a one-shot transfer primitive.

- **Candidate 5 (Lease-renewal grammar):** **`adopt-when-trigger-fires`**. Promoted to `docs/FOLLOWUPS.md` Phase B in this PR. Trigger verbatim:

  > *"Trigger: first customer agent runs longer than current lease max-TTL, OR first complaint about lease-ID churn fragmenting audit search."*

## 11. Rollback if adopted

**Cand. 3 (KMS auto-unseal):** Revert the `main.go` refuse-to-start guard; `EnvProvider` legal again. Fully reversible — no schema or ciphertext change. Master keys already unwrapped continue to work; next process start sources from env. Runbook: feature-flag `REQUIRE_KMS_PROVIDER=1` → `0`.

**Cand. 1 (Sentinel policy), if adopted later:** Additive `projects.promotion_policy JSONB` column; rollback = stop reading, keep dormant. Default policy collapses to current service-code behavior. New `approval_evaluated` audit event is additive.

**Cand. 2 (AppRole), if adopted later:** AppRole keys carry a distinct prefix (`ksa_`); existing `ks_` keys keep working. Rollback = disable endpoint, let issued AppRole keys expire. True rollback impossible if any agent began relying on it.

## 12. Won't-break-our-system claim

Cand. 3's integrator contract: `MASTER_KEY` env (dev) or cloud KMS creds (prod). Adoption makes the latter mandatory in prod only.

- **Inv 1 — crypto tests stay green.** `crypto_test.go` uses a fixture key; doesn't touch provider layer. Verified by reading `crypto.go:11-30` — no provider call in encrypt/decrypt path; master key injected at service construction. Adoption changes `main.go` wiring only.
- **Inv 2 — promotion behavior unchanged for non-PROD.** Cand. 1's default policy mirrors `promotion_service.go:186-195`. Verified: the line-187 conditional is the only env-keyed branch; default policy reproducing it is a no-op.
- **Inv 3 — audit-log schema is additive.** `approval_evaluated` is a new value of `AuditEntry.Action` (`models/models.go:63-72`); no column changes. `Action string` is unconstrained.
- **Inv 4 — `ks_` contract preserved if Cand. 2 adopted.** AppRole keys use `ksa_`; `apikey.go:11` `apiKeyPrefix = "ks_"` untouched.
- **Inv 5 — `VaultProvider` upstream adapter unchanged.** Cand. 3 productionizes the interface, not the adapter; `keyprovider/vault.go:39-92` signature preserved.

Security Reviewer veto applies. Suggested checks: (a) `REQUIRE_KMS_PROVIDER` refuses to start on `EnvProvider` in prod (not warns); (b) Cand. 1 policy struct embeds no string-eval interpreter; (c) Cand. 2 `ksa_` prefix enforced at issuance + lookup.

## 13. References

All retrieved 2026-05-15. Vendor docs unreachable via WebFetch (HashiCorp returns 403 on tooling UA); cited for traceability, load-bearing claims rest on non-vendor sources.

- Vault security model: `developer.hashicorp.com/vault/docs/internals/security` (403)
- Trail of Bits Vault audit Oct 2018: `github.com/trailofbits/publications/blob/master/reviews/HashiCorpVault.pdf`
- Sentinel docs: `developer.hashicorp.com/sentinel/docs` (403; search summary)
- Vault AppRole: `developer.hashicorp.com/vault/docs/auth/approle` (403)
- Vault response wrapping: `developer.hashicorp.com/vault/docs/concepts/response-wrapping` (search summary)
- Vault auto-unseal: `developer.hashicorp.com/vault/docs/concepts/seal#auto-unseal` (search summary)
- CVE-2020-16250 / CVE-2023-25000 / CVE-2024-2660: `nvd.nist.gov/vuln/detail/<id>`
- NIST SP 800-38D (GCM): `csrc.nist.gov/publications/detail/sp/800-38d/final`
- Vault source: `github.com/hashicorp/vault`

## Security Reviewer notes

**Verdict:** accepted-with-changes. Veto **not** exercised on §9 / §10 / §12.

**Refs spot-checked (Read tool, 2026-05-15) — all resolve:**

- `crypto/keyprovider/vault.go:54-92` — `POST /v1/transit/decrypt/<keyName>` + base64 + `validateKey`. §5, §12 Inv 5 confirmed.
- `crypto/crypto.go:11-30, 64-72` — 32-byte master key in `Service`; GCM/nonce in `encrypt`; no provider call in encrypt path. §12 Inv 1 confirmed.
- `service/promotion_service.go:186-195` — sole `targetEnv == "prod"` branch. §12 Inv 2 confirmed.
- `auth/apikey.go:11-32` — `apiKeyPrefix = "ks_"`, SHA-256, single-token. §12 Inv 4 confirmed.
- `models/models.go:39-49, 51-61, 63-72, 129-140, 220-227, 367-377` — all model shapes match; `AuditEntry.Action string` unconstrained → §12 Inv 3 confirmed.

**§9 STRIDE rows quoted verbatim from `docs/THREAT_MODEL.md` — all match:**

- L100 Promotion-E: `| E | Requester self-approves | **Invariant not currently enforced at DB layer** (gap) | open follow-up | Medium |` — Cand. 1 narrows.
- L96 Promotion-R: `| R | Approval decision merged with execution audit | No distinct ` + "`promotion_approved`" + ` event before execution (gap) | service/promotion_service.go:215-240 | Medium |` — Cand. 1 forces distinct `approval_evaluated`.
- L88 Auth-E: `| E | API key scope escalation | Scope is row-bound; per-project / per-env enforced in handlers | auth/apikey.go, models/models.go:56-59 | Low |` — Cand. 2 widens.
- L73 Vault-D: `| D | KMS throttle stalls startup | MasterKeyProvider retries with backoff | keyprovider/env.go and KMS adapters | Medium |` — Cand. 3 marginally widens.

**§10 trigger verification:**

- Cand. 1 (FU 0d, lines 48-52): match ✓.
- Cand. 2 originally cited `FOLLOWUPS.md:161` — actual is **168**. Corrected inline.
- Cand. 3 (FU#1 KMS) originally cited `FOLLOWUPS.md:83-88` — actual is **90-95** (83-88 is FU 0k `ks_` expiry, unrelated). Corrected inline.
- Cand. 5 lease-renewal promoted clean to `FOLLOWUPS.md:172` under "Open — Phase B" with explicit trigger, owner, and back-reference to this dossier.

**Audit cross-references (2026-05-15):**

- **Cand. 1 ↔ `SECURITY_AUDIT_2026-05-15.md` A04-F1 (L198-202):** P2 High "Approver = requester not enforced" is direct evidence the FU 0d verdict will land "code-only insufficient." Sentinel-shape policy is the structural successor to the audit's `if requester == approver { error }` + DB CHECK fix. Strengthens `adopt-when-trigger-fires` toward "trigger near-certain."
- **Cand. 3 ↔ `BACKEND_SPOF.md` (TL;DR L14: 5 critical; L35, L135, L202-203):** KMS adapters unwired (`main.go:247-248` bails); single 15s attempt at `resolveMasterKey` (`main.go:53-59`) crashes pods on KMS hiccup. Sharpens §7 op-con. **Cand. 3 ADR must land jointly with SPOF mitigations (graceful shutdown + retry/backoff at `resolveMasterKey`), not in isolation.**
- **Cand. 2 ↔ FU 0k + audit L469 ("`ks_` no-TTL... P2 in combination"):** mandatory expiry (FU 0k) ships first as the cheap fix; AppRole is the architectural follow-on. Adoption order correct.

**Inline changes during acceptance:**

1. §1 `draft` → `accepted`; reviewer → `accepted 2026-05-15`.
2. §7 Cand. 2 Pros: `:161` → `:168`. §10 Cand. 2: `:161` → `:168`. §10 Cand. 3: `:83-88` → `:90-95`.
3. This §Security Reviewer notes added.

**For ADR Drafter (non-blocking):** Cand. 3 ADR must specify retry/backoff at `resolveMasterKey` (single 15s attempt is below baseline); §12 Inv 1 must hold as a lint (no provider call inside `encrypt`/`decrypt`); Cand. 1 "no embedded interpreter" should be a lint, not prose. Vendor 403s on HashiCorp docs are acceptable per refined-template; load-bearing claims rest on Trail of Bits 2018 + CVEs.

No reference failed to check out.

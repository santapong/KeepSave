# BEYOND — Where KeepSave Can Leapfrog the Field

- **Status:** Skeleton (seeded ideas only; analyst evidence pending)
- **Last updated:** 2026-05-15
- **Owner:** Research Lead
- **Cadence:** v1 due day 60, refreshed each phase boundary

## 1. Premise

KeepSave sits at an unusual intersection: it is a **secrets store** (overlapping Vault, Doppler, Infisical), a **machine identity issuer** (overlapping GitHub PATs, SPIFFE, Vault auth methods), an **OAuth 2.0 server** (overlapping Ory Hydra, Keycloak), and a **promotion-gate engine** (overlapping Vault Sentinel, GitHub Environments). No incumbent ships all four with the same opinionated promotion-pipeline shape that `docs/adr/0003` describes.

This memo lists candidate directions where KeepSave can ship something the field hasn't, evaluated against (a) feasibility on top of `docs/adr/0004` key hierarchy, (b) Type-1 ADR cost, (c) whether `docs/ROADMAP_NOT.md` blocks the path.

Analysts: fill each candidate with `hypothesis / evidence / feasibility / cost / phase`.

## 2. Leapfrog candidates

### 2.1 Per-secret / per-action API-key scopes

- **Hypothesis:** Replace the current coarse per-project / per-env scope on `ks_` keys with a grammar that lets a key say "read `DATABASE_URL` and `REDIS_URL` only, in `prod` only, never write."
- **Evidence:** `docs/research/competitors/vault.md` Cand. 2 (AppRole secret-id binding); `docs/research/competitors/github.md` Cand. 2 (fine-grained PAT resource grammar); RFC 6749 §1.3.4 (scope as space-delimited capability list); CircleCI 2023 incident post-mortem (over-scoped CI tokens enabled lateral movement).
- **Feasibility:** Existing `Scopes StringList` on `APIKey` at `backend/internal/models/models.go:51-61` already accepts a list; the change is grammar + enforcement, not schema. Service-layer change only.
- **Type-1 cost:** Yes — ADR required; new threat-model row (key compromise blast-radius narrows).
- **Phase:** B candidate (already deferred in `docs/FOLLOWUPS.md`).

### 2.2 Ephemeral envelope keys per promotion

- **Hypothesis:** Today envelope encryption is per-project (`docs/adr/0001`, `docs/adr/0004`). Mint a one-shot DEK for each promotion event, encrypted with both source and destination env KEKs, destroyed after promotion commits. Reduces blast radius if a project DEK leaks: prior promotions remain readable only via the source ciphertext, not via the destination DEK.
- **Evidence:** Extends ADR-0001 envelope scheme; `docs/research/competitors/vault.md` Cand. 3 (KMS auto-unseal + barrier-key rotation pattern); Vault Trail of Bits 2018 audit on barrier-key handling (per-operation key derivation reduces blast radius); AWS KMS `GenerateDataKeyWithoutPlaintext` pattern as production analog.
- **Feasibility:** Extends `docs/adr/0004` rather than replacing — needs ADR-0005-class decision.
- **Type-1 cost:** High — touches crypto core; Security Engineer veto path.
- **Phase:** C candidate; pre-work in B.

### 2.3 Post-quantum readiness for KEK/DEK layer

- **Hypothesis:** Make the KEK envelope hybrid (classical + ML-KEM) so customers with FIPS-204 mandates can opt in without re-encrypting ciphertext. ML-KEM ciphertext stored alongside AES-GCM.
- **Evidence:** NSA CNSA 2.0 migration deadlines (mandatory PQ for NSS by 2033, software signing by 2030); NIST FIPS-203 (ML-KEM / CRYSTALS-Kyber, Aug 2024 final) and FIPS-204 (ML-DSA); NIST PQC migration roadmap NIST IR 8547 (draft 2024).
- **Feasibility:** Library available (Go `crypto/mlkem` since 1.24). Format change in envelope header.
- **Type-1 cost:** Yes — schema-touching, crypto-touching.
- **Phase:** C; B if a customer raises FIPS-204 deal-blocker.

### 2.4 Biscuit-style attenuable tokens for API keys

- **Hypothesis:** Replace opaque `ks_` keys with attenuable capabilities (biscuit, macaroons): an agent can mint a strictly-weaker key from its own key without server roundtrip, useful for short-lived subagent delegation in AI workflows.
- **Evidence:** Biscuit token spec v3 (Clever Cloud, open standard, Ed25519-signed attenuable capabilities); Macaroons paper — Birgisson, Politz, Erlingsson, Taly, Vrable, Lentczner — Google Research, NDSS 2014 (the academic source for caveat-based attenuation); Tailscale node-key delegation as production analog.
- **Feasibility:** Requires verifier in middleware; existing API-key middleware at `backend/internal/api/middleware.go` is single-shot lookup, not capability verification.
- **Type-1 cost:** High — replaces ADR-0002 auth primitive.
- **Phase:** C candidate; pre-research in B.

### 2.5 SPIFFE-shaped workload identity for AI agents

- **Hypothesis:** Replace long-lived API keys for AI agents with SVIDs (SPIFFE Verifiable Identity Documents) issued per workload, short-lived, attested. Eliminates "agent rotates key" as a customer responsibility.
- **Evidence:** Full dossier at `docs/research/competitors/spiffe.md` (SPIFFE/SPIRE spec, SVID issuance flow, attestor model); production case studies named in that dossier — Bloomberg, Netflix, Square — as evidence of viability at scale; HashiCorp Vault SPIFFE auth method as integration precedent.
- **Feasibility:** New auth method alongside JWT/API-key, not a replacement. Adds attestor sidecar requirement on customer side.
- **Type-1 cost:** Yes — new auth primitive, ADR required.
- **Phase:** B if the MCP integration path (`docs/medqcnn_integration.md`, `docs/nexus_integration.md`) pulls this forward.

### 2.6 Cryptographically chained tamper-evident audit log

- **Hypothesis:** Each audit row hashes the prior row's hash (Merkle/hash-chain). Quarterly anchor of the chain root to a public timestamp service. An attacker who reaches DB write must produce a globally-impossible rewrite to hide their tracks.
- **Evidence:** RFC 6962 (Certificate Transparency — production reference design for append-only Merkle log); Trillian (Google's open-source CT log implementation, the canonical reference); CONIKS paper — Melara, Blankstein, Bonneau, Felten, Freedman, USENIX Security 2015 (same primitive applied to key transparency); `docs/audits/SECURITY_AUDIT_2026-05-15.md` flagged audit-log gaps that this candidate would close.
- **Feasibility:** Single-column addition to audit table + verification job. Sigstore's Rekor is the canonical open-source reference.
- **Type-1 cost:** Schema-touching → ADR required.
- **Phase:** B candidate; addresses Repudiation row in `docs/THREAT_MODEL.md`.

### 2.7 Rego / Cedar policy-as-code on promotion gates

- **Hypothesis:** Replace the implicit promotion-rule code in `internal/promotion` with declared policy (Rego or Cedar). Customers can review/diff/audit promotion rules without reading Go.
- **Evidence:** Open Policy Agent (Rego) docs and production case studies — Netflix Lemur (cert lifecycle policy), Chef InSpec (compliance-as-code), Styra DAS (managed OPA); AWS Cedar policy language spec (open-sourced 2023, used in Amazon Verified Permissions); Vault Sentinel as the proprietary alternative (cross-ref `docs/research/competitors/vault.md` Cand. 1).
- **Feasibility:** Side-by-side with existing code path; enable via feature flag.
- **Type-1 cost:** Yes — promotion engine is on the Security Engineer veto list.
- **Phase:** C candidate; depends on customer demand signal.

## 3. Disqualified ideas (re-litigation prevention)

(populated as research progresses — record ideas that looked good and aren't, with one-line rationale)

- **Oathkeeper-as-deployed-proxy** — rejected by Ory dossier (`docs/research/competitors/ory.md` Cand. 4): too heavyweight for KeepSave's scale; existing `backend/internal/api/middleware.go:88-120` is the simpler shape and already covers the same threat model.
- **Vault response-wrapping tokens** — rejected by Vault dossier (`docs/research/competitors/vault.md` Cand. 4): no customer demand signal, additional auth surface not paid for by any committed integration.
- **Infisical native CLI** — rejected by Infisical dossier (`docs/research/competitors/infisical.md` Cand. 4): collides with `docs/ROADMAP_NOT.md:24-27` (no first-party CLI in scope); credential-hygiene downgrade without a paired ADR.
- **SPIFFE federation (cross-trust-domain SVIDs)** — rejected by SPIFFE dossier (`docs/research/competitors/spiffe.md` Cand. 4): no customer demand; widens trust boundary across organizations without a `docs/THREAT_MODEL.md` row to absorb it.
- **Doppler branch config / inherit-and-override** — rejected by Doppler dossier (`docs/research/competitors/doppler.md` Cand. 4): conflicts with ADR-0003 promotion-as-copy semantics; cannot be retrofitted without rewriting the promotion engine.

## 4. Sequencing summary

| Candidate | Phase | Gating dependency |
|---|---|---|
| 2.1 Per-secret API key scopes | B | FU "Phase B deferred"; ADR-0005-class |
| 2.2 Ephemeral promotion DEK | C (B pre-work) | ADR extending `docs/adr/0004` |
| 2.3 Post-quantum hybrid KEK | C (B if customer trigger) | NIST FIPS-204 customer ask |
| 2.4 Biscuit-style attenuable keys | C | Replaces ADR-0002 primitive |
| 2.5 SPIFFE workload identity | B (if MCP pull) / C | New auth method ADR |
| 2.6 Hash-chained audit log | B | `docs/THREAT_MODEL.md` repudiation row |
| 2.7 Policy-as-code promotion | C | Customer demand signal |

Cross-ref: ADRs 0005-0010 each operationalize one of the above leapfrog candidates for Phase B — see `docs/adr/0005-require-project-access-middleware.md` (per-scope enforcement substrate for 2.1), `docs/adr/0006-embed-widget-origin-allowlist.md` (origin policy primitive reusable for 2.5 attestor egress), `docs/adr/0007-approver-not-requester-db-invariant.md` (DB invariant pattern reused for 2.6 audit-chain integrity), `docs/adr/0008-rs256-jwks-rotation.md` (JWKS rotation pattern reusable for 2.3 PQ key rollover), `docs/adr/0009-default-api-key-expiration.md` (TTL substrate for 2.4 short-lived attenuated keys), and `docs/adr/0010-mcp-gateway-command-execution-hardening.md` (workload-context surface for 2.5 SPIFFE attestation).

## 5. Open questions

- Which candidate sees the strongest external-review (day 90) interest? Owner: Research Lead. Due: day 75.
- Are any candidates pre-empted by a competitor shipping the same thing during this initiative? Owner: each analyst. Due: per dossier acceptance.
- What is the smallest viable proof-of-concept for 2.1 that doesn't break existing API-key issuance? Owner: Machine Identity Analyst. Due: day 60.
- Should the `c.MustGet` safego refactor (now tracked as FU 0l) land before or in parallel with ADR-0015 (per-use audit emission)? Owner: Tech Lead + Backend Engineer. Due: day 60.

# Pattern Adoption Matrix

- **Version:** v1
- **Last updated:** 2026-05-15
- **Versioning:** v1 due day 30 (this revision); v2 due day 60 with P1 columns added.

## How to read

- **Rows** are auth/secrets/approval features.
- **Columns** are competitors (P0 only at v1; P1 added at v2).
- **Cells** use one of: `keep` / `adapt: <how>` / `reject: <why>` / `n/a`.
- **Recommendation** column synthesizes across competitors (Research Lead only).
- **Tied FU#** references `docs/FOLLOWUPS.md`. **Mandatory** for every `adapt` recommendation — if a row has no follow-up, it belongs in `BEYOND.md`.

## Matrix v1

| # | Feature | KeepSave today (file:line) | Vault | Ory | Keycloak | GitHub | Infisical | Doppler | SPIFFE | Pomerium | Recommendation | Tied FU# |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | JWT signing alg + JWKS rotation | HS256 only; JWKS empty `handlers_oauth.go:216-218` | reject: HCL/Shamir overkill | adapt: RS256 + `kid`-rotated JWKS (Cand. 1) | adapt: RS256 + retired-`kid` window (refs Ory) | n/a | n/a | n/a | n/a | adapt: ES256 + per-route JWKS verify (Cand. 3, Phase B) | **adopt-now**: RS256 + JWKS w/ `kid` rotation (Ory shape) | new ADR [^1] |
| 2 | Refresh-token rotation + denylist | No refresh; no denylist `auth/auth.go:11-62` | reject: lease grammar tied to Vault primitives | adapt: rotation w/ family-replay-detection (Cand. 2) | adapt: per-realm rotation pattern | n/a | n/a | n/a | adapt: short TTL collapses denylist need (Cand. 1+2) | n/a | adapt-when-trigger-fires: rotation + family invalidation | FU #5 [^2] |
| 3 | OIDC inbound (dashboard) | `SSOConfig` model stub `models.go:277-290` | n/a | adapt: code+PKCE flow shape (Cand. 1 trigger-fires) | adapt: code+PKCE, `(iss,sub)`-keyed JIT (Cand. 1+4) | n/a | n/a | n/a | n/a | reject: KeepSave is RP, not proxy IdP | adapt-when-trigger-fires: OIDC-only first wave | ROADMAP_NOT #2 [^3] |
| 4 | SAML inbound | `SSOConfig` model stub `models.go:277-290` | n/a | reject: Hydra is OAuth/OIDC only | reject: XML-DSig signature-wrapping CVE class | n/a | n/a | n/a | n/a | n/a | reject: defer past OIDC; XML-DSig CVE class | ROADMAP_NOT #2 [^3] |
| 5 | Fine-grained API key scopes (per-secret, per-action) | Coarse `APIKey.Scopes` `models.go:51-61` | adapt: AppRole two-secret-pull (Cand. 2, trigger-fires) | n/a | n/a | adapt: PAT `resource × permission × level` tuple (Cand. 2) | adapt: secret-approval-policy DSL (Cand. 3) | adapt: Service Account multi-project grants (Cand. 1) | n/a | n/a | adapt-when-trigger-fires: PAT-shaped tuple grammar | FU Phase B [^4] |
| 6 | Ephemeral / short-lived credentials | **No `ExpiresAt` set at issuance** `validation.go:33-38`, `apikey_service.go:33-60`, `apikey_repo.go:33-36` | adapt: AppRole + KMS auto-unseal (Cand. 2+3) | n/a | n/a | adapt: PAT mandatory `≤366d` expiry (Cand. 3) | n/a | adapt: Service Token expiry (`ExpiresAt`) | adapt: JWT-SVID `≤15min` TTL + `aud` binding (Cand. 1+2) | n/a | **adopt-now**: convergent finding — default expiration on `ks_` keys; long-term path to SVID-shaped tokens | FU 0k [^5] |
| 7 | Multi-party approval (N-of-M) | Single-approver; `ApprovedBy` slot `promotion_service.go:214-240` | adapt: Sentinel-shape policy-as-code (Cand. 1) | n/a | n/a | adapt: declarative reviewer pool w/ `min_count` (Cand. 5) | adapt: declarative `project_approval_policy` (Cand. 3) | adapt: Change Requests for edits (Cand. 3) | n/a | n/a | adapt-when-trigger-fires: declarative reviewer pool | Phase B [^6] |
| 8 | Approver ≠ requester enforcement | **Service-code only, not DB-enforced** `promotion_service.go:214-240` | adapt: structural policy struct (Cand. 1) | n/a | n/a | **adapt: DB `CHECK` + service guard (Cand. 1)** | adapt: same `CHECK` shape (Cand. 3) | adapt: same `CHECK` shape (Cand. 3) | n/a | n/a | **adopt-now**: `CHECK (approved_by <> requested_by)` + 422 guard | FU 0d |
| 9 | Promotion kill switch | Missing — no runtime flag | reject: not in Vault model | n/a | n/a | reject: GitHub Environment is per-deployment, not global | reject: no analog | reject: no analog | n/a | n/a | adopt-now (KeepSave-internal): runtime env-flag for `/promote` + `/approve` | FU 0e |
| 10 | Policy-as-code on gates | None | adapt: Sentinel-shape struct (Cand. 1, no DSL) | n/a | n/a | adapt: protection-rule struct (Cand. 5) | adapt: secret-approval-policy row (Cand. 3) | n/a | n/a | reject: Rego too heavy for our scale | adapt-when-trigger-fires: struct, not DSL | Phase B [^6] |
| 11 | Signed audit log (tamper-evident) | Plain rows `audit_repo.go:21-35` | adapt: HMAC audit-device pattern (BEYOND) | n/a | n/a | n/a | n/a | reject: closed-source; Activity Log unsigned | n/a | n/a | BEYOND candidate (no FU yet) | — [^7] |
| 12 | Session revocation | `SessionToken` unwired `models.go:352-362` | reject: lease cascade tied to Vault tokens | adapt: Kratos server-tracked sessions + TTL pruner (Cand. 3) | adapt: per-realm session revocation pattern | n/a | n/a | n/a | n/a | reject: cookie-only, no API analog | adapt-when-trigger-fires: wire `SessionToken` + LRU cache | FU #5 [^2] |
| 13 | Origin-validated embed/iframe auth | **No `ev.origin` check; wildcard `*`** `embed/auth.ts:21-26, :33` | n/a | n/a | n/a | n/a | n/a | n/a | n/a | **adapt: per-project allow-list + strict `ev.origin` (Cand. 1+2)** | **adopt-now**: per-project allow-list + strict equality | FU 0b |
| 14 | Workload identity for non-human callers | Static `ks_` only | adapt: AppRole identity split (Cand. 2, trigger-fires) | n/a | n/a | adapt: per-call audit + token-ID grammar (Cand. 4) | adapt: multi-method identity (Cand. 5, deferred) | adapt: Service Account multi-project (Cand. 1) | adapt: JWT-SVID + attestor interface (Cand. 1+3) | n/a | adapt-when-trigger-fires: JWT-SVID-shaped tokens; attestor interface lands first | Phase B [^4] |
| 15 | Rate-limit / lockout patterns | Per-IP exponential backoff `ratelimit*.go` | keep | keep | adapt: built-in account lockout pattern (parity check only) | keep | keep | keep | n/a | keep | keep | — |

## Convergent gaps surfaced

Three independent dossiers (Vault, SPIFFE, GitHub) located the **same root cause** in row 6: `backend/internal/api/validation.go:33-38` (no `expires_at` in `CreateAPIKeyRequest`), `backend/internal/service/apikey_service.go:33-60` (`Create` takes no expiry param), `backend/internal/repository/apikey_repo.go:33-36` (INSERT omits `expires_at`). Middleware at `middleware.go:101-105` already honours `ExpiresAt` when non-NULL — only issuance is broken. The fix is a single PR; the segment-baseline gap is severe (GitHub PAT: mandatory ≤366d; SPIFFE: ≤15min; Doppler: optional but typical). Tracked in new FU 0k.

A secondary convergent gap: rows 7, 8, 10 (GitHub Cand. 5, Infisical Cand. 3, Doppler Cand. 3, Vault Cand. 1) all converge on the same shape — a declarative per-project approval-policy table replacing the hard-coded "PROD = one approver, not requester" logic. Row 8 (`approver ≠ requester` DB invariant) is the minimum viable subset and is **adopt-now** under FU 0d; rows 7 and 10 wait for the Phase B trigger.

## Rules for filling cells

1. `keep` — KeepSave at parity or better; do not change.
2. `adapt: <how>` — borrow a specific aspect, one line. **MUST have a `Tied FU#`.**
3. `reject: <why>` — competitor's approach is worse or incompatible, one line.
4. `n/a` — competitor doesn't address this feature category.
5. `TBD` — analyst hasn't reported. **Zero TBDs in P0 columns is the day-30 exit criterion (met in v1).**

## Synthesis rules

- A row whose `Recommendation` is `adapt` without a tracked FU# moves to `BEYOND.md`.
- A row referencing `ROADMAP_NOT` must quote the trigger verbatim in Footnotes.
- Research Lead is the only role permitted to set the `Recommendation` column.

## Footnotes

[^1]: Row 1 — `adopt-now` for RS256 + JWKS rotation. No existing FU# directly covers this; the Ory dossier (`docs/research/competitors/ory.md` §10 Cand. 1) treats this as the live P0 cited in `docs/research/README.md:11` ("the OAuth 2.0 server in `backend/internal/api/handlers_oauth.go` exposes an empty JWKS and no RS256"). New Type-1 ADR required at adoption (auth-adjacent; Security Engineer veto applies). A new FU entry is **not** added because the ADR itself is the tracking artifact — but if the ADR slips past day-45 it must be tracked as a new FU.

[^2]: Rows 2, 12 — `adopt-when-trigger-fires` against FU #5 (Phase B). FOLLOWUPS.md current Phase-B entry reads verbatim: *"**JWT denylist for pre-expiry revocation** (ADR-0002 §Open Questions). Build when a customer SLA requires it."* Ory Cand. 2+3 land in one ADR when triggered; SPIFFE Cand. 1+2 collapse the denylist need by shortening TTL.

[^3]: Rows 3, 4 — `ROADMAP_NOT.md` §2 trigger quoted verbatim: *"### 2. SSO / SAML / OIDC for end-user auth — What we are not building: federated login for dashboard users. — Why: no customer has asked. Building it before the demand signal arrives means we'll build the wrong shape. — Trigger to revisit: when two customers ask in the same quarter, or when one enterprise prospect makes it a deal-blocker."* Keycloak dossier §10 raises a precision question (does "two customers asking" mean same protocol or any?) — resolution deferred to the ADR that fires the trigger; OIDC-first / SAML-deferred is the default per Keycloak Cand. 2 reasoning (XML-DSig signature-wrapping CVE class).

[^4]: Rows 5, 14 — `adopt-when-trigger-fires` against the FOLLOWUPS Phase-B entry quoted verbatim: *"**Fine-grained API key scopes** (per-secret or per-action; ADR-0002). Build when a use case appears, not before."* Row 14 additionally references the Phase-B SPIFFE entry already in FOLLOWUPS (added in SPIFFE-dossier PR).

[^5]: Row 6 — convergent finding from Vault (Cand. 2 cons), SPIFFE (Cand. 1 + §5 row 2), and GitHub (Cand. 3) dossiers. Added as **FU 0k** in this synthesis PR: default 90d expiration, 365d ceiling, sentinel `9999-12-31` for audited opt-out. Pattern source: GitHub fine-grained PAT mandatory expiration.

[^6]: Rows 7, 10 — `adopt-when-trigger-fires` against the FOLLOWUPS Phase-B entry quoted verbatim: *"**Three-of-N approval for PROD** (ADR-0003). Build if regulatory pressure or a customer commitment forces it."* Same trigger as Doppler Change Requests (`ROADMAP_NOT.md` §4 *"Trigger to revisit: non-technical customer admin role becomes a buyer."* — cross-trigger relevant for row 10's GUI-shaped surface).

[^7]: Row 11 — `BEYOND` candidate per matrix synthesis rule (no Tied FU#). The Vault audit-device HMAC pattern is captured in `docs/research/competitors/vault.md` §5 but no candidate makes it `adopt-now` or `adopt-when-trigger-fires`. Promote to a new FU only if/when an audit-tamper threat scenario gains a sponsor.

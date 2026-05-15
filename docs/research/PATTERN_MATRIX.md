# Pattern Adoption Matrix

- **Version:** v0 (skeleton — analysts fill in)
- **Last updated:** 2026-05-15
- **Versioning:** v1 due day 30; v2 due day 60.

## How to read

- **Rows** are auth/secrets/approval features.
- **Columns** are competitors (P0 only at v1; P1 added at v2).
- **Cells** use one of: `keep` / `adapt: <how>` / `reject: <why>` / `n/a`.
- **Recommendation** column synthesizes across competitors.
- **Tied FU#** column references `docs/FOLLOWUPS.md`. **Mandatory** for every `adapt` recommendation — if a row has no corresponding follow-up, it belongs in `BEYOND.md`, not here.

## Matrix v0 (skeleton)

| Feature | KeepSave today (file:line) | Vault | Ory | Keycloak | GitHub | Infisical | Doppler | SPIFFE | Pomerium | Recommendation | Tied FU# |
|---|---|---|---|---|---|---|---|---|---|---|---|
| JWT signing alg + JWKS rotation | HS256 only; JWKS empty `handlers_oauth.go:217` | TBD | TBD | TBD | n/a | TBD | TBD | n/a | TBD | TBD | — |
| Refresh-token rotation + denylist | No refresh; no denylist `auth.go:11-62` | TBD | TBD | TBD | n/a | TBD | TBD | n/a | TBD | TBD | FU #5 |
| OIDC inbound (dashboard) | `SSOConfig` model only `models.go:278` | n/a | TBD | TBD | TBD | TBD | TBD | n/a | TBD | TBD | ROADMAP_NOT #2 |
| SAML inbound | `SSOConfig` model only `models.go:278` | n/a | TBD | TBD | TBD | TBD | TBD | n/a | TBD | TBD | ROADMAP_NOT #2 |
| Fine-grained API key scopes (per-secret, per-action) | Coarse per-project/per-env `apikey.go:11-32` | TBD | TBD | TBD | TBD | TBD | TBD | TBD | n/a | TBD | Phase B deferred |
| Ephemeral / short-lived credentials | None (long-lived `ks_` keys) | TBD | TBD | TBD | TBD | TBD | TBD | TBD | n/a | TBD | BEYOND candidate |
| Multi-party approval (N-of-M) | Promotion engine `docs/adr/0003` | TBD | n/a | n/a | TBD | TBD | TBD | n/a | n/a | TBD | FU 0d |
| Approver ≠ requester enforcement | Unverified `FU 0d` | TBD | n/a | n/a | TBD | TBD | TBD | n/a | n/a | TBD | FU 0d |
| Promotion kill switch | Missing | TBD | n/a | n/a | TBD | TBD | TBD | n/a | n/a | TBD | FU 0e |
| Policy-as-code on gates | None | TBD | n/a | n/a | n/a | TBD | TBD | n/a | TBD | TBD | BEYOND candidate |
| Signed audit log (tamper-evident) | Plain rows | TBD | n/a | TBD | n/a | TBD | TBD | n/a | n/a | TBD | BEYOND candidate |
| Session revocation | None pre-expiry `SessionToken` exists, unwired `models.go:353` | TBD | TBD | TBD | n/a | TBD | TBD | n/a | TBD | TBD | FU #5 |
| Origin-validated embed/iframe auth | Missing `embed/auth.ts:21-26` | n/a | n/a | n/a | n/a | n/a | n/a | n/a | TBD | TBD | FU 0b |
| Workload identity for non-human callers | API keys only | TBD | n/a | n/a | TBD | n/a | n/a | TBD | n/a | TBD | BEYOND candidate |
| Rate-limit / lockout patterns | Per-IP exponential backoff `ratelimit*.go` | TBD | TBD | TBD | TBD | TBD | TBD | n/a | TBD | TBD | — |

## Rules for filling cells

1. `keep` — KeepSave's current approach is at parity or better; do not change.
2. `adapt: <how>` — borrow a specific aspect. The "how" must fit in one line. The row MUST then have a `Tied FU#` value.
3. `reject: <why>` — competitor's approach is worse or incompatible. State the reason in one line.
4. `n/a` — this competitor doesn't address this feature category.
5. `TBD` — analyst hasn't reported. **Zero TBDs in P0 columns is the day-30 exit criterion.**

## Synthesis rules

- A row whose `Recommendation` is `adapt` without a tracked follow-up moves to `BEYOND.md`.
- A row whose `Recommendation` references `ROADMAP_NOT` must quote the trigger from `docs/ROADMAP_NOT.md` verbatim in the matrix footnote.
- The Research Lead is the only role permitted to set the `Recommendation` column.

## Footnotes

(populated as `ROADMAP_NOT` triggers are referenced)

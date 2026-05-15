# Competitor Dossier — Cloudflare Access

## 1. Header

- **Vendor:** Cloudflare Access (Cloudflare One / Zero Trust)
- **Category:** edge auth (managed)
- **License:** commercial SaaS (no OSS core)
- **Last updated:** 2026-05-15
- **Analyst:** Edge Auth Analyst
- **Reviewer:** Security Reviewer (accepted 2026-05-15)
- **Status:** accepted
- **Priority:** P1

## 5. KeepSave-comparable surface

Managed-edge alternative to Pomerium (P0 dossier, same analyst). Same JWT-signed-header pattern, but Cloudflare runs the proxy on its CDN — no customer infra, but a hard vendor dependency. Patterns only; we adopt no product.

| Cloudflare concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| `Cf-Access-Jwt-Assertion` JWT injected at edge | None — widget passes creds via raw `postMessage` | `frontend/src/embed/auth.ts:21-26`, `:33` |
| Service Tokens (`CF-Access-Client-Id`/`-Secret`) | `ks_` API keys for agents | `backend/internal/auth/apikey.go` |
| Per-application allow-list | Proposed per-project `allowed_origins` | `docs/adr/0006-embed-widget-origin-allowlist.md:51` |
| Per-request edge audit log | Server-side audit log | `docs/AUDIT_LOG_COVERAGE.md` |

**Deliberately not adopted:** Cloudflare-hosted IdP, WAF/DDoS, Warp client, CDN data plane — KeepSave ships self-hosted.

## 6. Adapt candidates

1. **Service-token shape** — Cloudflare's rotatable Client-Id + Client-Secret pair refines KeepSave's single-string `ks_` key. Informative for deferred fine-grained API-key scopes (`docs/FOLLOWUPS.md:176`).
2. **Signed-header at managed edge** — Same JWT shape as Pomerium Cand. 3, but with Cloudflare as signer. Informative *only* if KeepSave ever offers a managed-cloud edition.

## 8. Validation evidence

- **RFC 7515 (JWS)** / **RFC 7519 (JWT)** — `https://www.rfc-editor.org/rfc/rfc7515` (retrieved 2026-05-15). Load-bearing non-vendor source for the signing scheme.
- **Cloudflare Access docs** — `https://developers.cloudflare.com/cloudflare-one/identity/users/validating-json/` (retrieved 2026-05-15, **unreachable via tooling — 403**); search summary corroborates `Cf-Access-Jwt-Assertion` header name and JWKS endpoint.
- **CVE-2023-44487 (HTTP/2 Rapid Reset)** — NVD `https://nvd.nist.gov/vuln/detail/CVE-2023-44487` (non-vendor corroborator, retrieved 2026-05-15). Managed edge is itself a CVE-bearing surface; lock-in risk is real.
- **2020-07-17 Cloudflare global outage** post-incident — operational-risk framing.

## 9. Threat-model implications

Maps to `docs/THREAT_MODEL.md` §4 row S (Embed widget, Spoofing). Both candidates **note-only**: Cand. 1 would narrow API-key blast radius if ever adopted; Cand. 2 *widens* the trust boundary by adding Cloudflare as a new entity — unacceptable for the self-hosted shape.

## 10. Verdict

Both **`note-only`**. KeepSave ships self-hosted; `docs/ROADMAP_NOT.md` §1 defers multi-tenant runtime to Phase B and contains no managed-cloud carve-out, so a managed-edge dependency is out of scope by default. Value here is **trigger readiness** for a hypothetical KeepSave Cloud offering. ADR-0006 (`:51`) takes per-project allow-list as a server-side row, *not* signed-headers at an edge — Cloudflare's pattern is a Phase-B-or-later evolution path, not a substitute.

## Security Reviewer notes

**Verdict:** accepted. Veto **not** exercised on §9 or §10.

**`note-only` upheld — not upgraded to `adopt-when-trigger-fires`.** Template §10 requires the trigger be quoted verbatim with `file:line` from `ROADMAP_NOT.md`, `FOLLOWUPS.md`, or a committed `docs/**` policy doc — and if the trigger only lives in dossier prose, promoted to `FOLLOWUPS.md` in the same PR. I searched: `ROADMAP_NOT.md` has no "KeepSave Cloud" / managed-edge entry (§1 covers multi-tenant runtime as in-product isolation, not a SaaS edition); `FOLLOWUPS.md` Phase-B list (`:174-180`) covers DEK, JWT denylist, scopes, approval, HKDF, SPIFFE, leases — no managed-cloud item. No quotable trigger exists, so `adopt-when-trigger-fires` is structurally unavailable; `note-only` is the correct verdict for both candidates. If a future PM wants to keep this dossier "warm," the right move is a `FOLLOWUPS.md` entry naming the trigger (e.g., "first enterprise prospect demands managed-edge"), *not* changing the verdict here.

**§8 vendor-unreachable triangulation:** procedure applied per template `:66`. Canonical Cloudflare URL cited with `(unreachable via tooling — 403)`; load-bearing security claim rests on **RFC 7515 / 7519** (non-vendor, reachable) for the JWS/JWT scheme — not on the search excerpt. CVE-2023-44487 (NVD URL verified — correct ID and registry) is a sound corroborator for the "managed edge is itself a CVE-bearing surface" framing. The 2020-07-17 outage citation lacks a URL but is operational-risk context only, not load-bearing. **Acceptable.**

**Spot-checks (Read tool, 2026-05-15):**

- `frontend/src/embed/auth.ts:21-26`, `:33` — no `ev.origin` check; wildcard `'*'` target. §5 row 1 accurate.
- `backend/internal/auth/apikey.go` — SHA-256 `ks_` key, no per-env / per-service split. §5 row 2 framing fair: Cloudflare's Client-Id/Client-Secret pair is a refinement over a single string.
- `docs/adr/0006-embed-widget-origin-allowlist.md:51` — "**Adopt Option A.** Server-side per-project origin allow-list…". Line cite correct; §5 row 3 mapping accurate.
- `docs/ROADMAP_NOT.md` §1 — multi-tenant runtime deferred to Phase B, no managed-cloud carve-out. §10 framing accurate.
- `docs/THREAT_MODEL.md` §4 row S at `:112` — embed widget spoofing, residual **High**. §9 mapping correct.

**§6 / §9 candidate framing:** Cand. 1 (service-token split) genuinely narrows blast radius and is consistent with `FOLLOWUPS.md:176` fine-grained scopes — `note-only` is right because the work is already tracked there, not here. Cand. 2 (managed-edge JWT signer) *widens* the trust boundary by introducing Cloudflare as a new entity in the boundary — analyst correctly named the new entity per template §9. Adopting it under the current self-hosted shape would be a Type-1 decision that contradicts `ROADMAP_NOT.md` §1's intent.

**Template-conformance gaps (P1, non-blocking):**

P1 collapses §3–5 but does not waive §2, §7, §11, §12, §13. This dossier omits all five. Given (a) both verdicts are `note-only`, (b) no product is adopted, and (c) no code change ships, §11 (rollback) and §12 (won't-break) have nothing to back out — accept as vacuous. §2 (one-paragraph overview) and §13 (references) **should** have been included even for `note-only`; the §5 lede paragraph and the §8 link list serve as informal stand-ins, so I accept rather than reject, but flag for the next P1 author: the template's section list is not optional outside the explicit §3–5 collapse.

**Non-blocking observations:**

- The 2020-07-17 Cloudflare outage citation in §8 lacks a URL; if this dossier is ever cited as evidence, add `https://blog.cloudflare.com/cloudflare-outage-on-july-17-2020/` with retrieval date.
- §6 Cand. 1 informativeness is partly already captured by Pomerium dossier's session/identity framing; cross-reference would strengthen the dossier without changing the verdict.
- If a "KeepSave Cloud" track ever materialises, this dossier and the Pomerium dossier's Candidate 3 (signed envelope) converge — that's the moment to revisit, with a fresh ADR, not by amending this verdict.

No reference failed to check out. Inline change during acceptance: §1 status flipped to `accepted`.

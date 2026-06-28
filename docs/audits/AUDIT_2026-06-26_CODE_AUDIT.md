# KeepSave — Code Audit (2026-06-26)

Performance / bug / refactoring audit run through the [ADLC](../ADLC.md) as a gated, loop-engineered Workflow (see [`docs/HARNESS_ENGINEERING.md`](../HARNESS_ENGINEERING.md)). This is the durable record of the run and its **adversarially-verified** findings.

## Method

- **Harness:** loop-until-dry discovery (critic-driven rounds) → lens-diverse adversarial verification (default-deny) → synthesis. Authored/run via the [`/workflow`](../../.claude/skills/workflow/SKILL.md) skill.
- **Scale:** 81 agents · ~3.4M tokens · 2 rounds · ~63 min.
- **Funnel:** **143** findings discovered (R1: 100, R2: 43) → top **16** by severity sent to verification (3 lenses each: *is-it-real*, *impact-significant*, *not-intended-design*; majority-upholds, default-deny) → **11 confirmed**, 5 refuted/uncertain.
- **Synthesis caveat:** the automated synthesis agent returned a malformed stub; the prioritization below was done by hand from the verified findings and spot-checked against live code (`SetTrustedProxies` absent in `backend/`; `gin.H{"error": err.Error()}` ×34 in `handlers_intelligence.go`; `IPAllowlistService`/`GetSSOConfig:61`/`DetectDrift:77` attributions confirmed).

> **Coverage limit (no silent caps):** only the top **16 of 143** were verified. **127 were not** — including high-severity items outside the cut (e.g. `organization_repo.ListProjectsByOrg` dropping embed-policy fields; `SecurityTab` fabricating mock data on API failure) and most performance/refactor items. A second verification pass is tracked below.

## Confirmed findings

### Tier 1 — plaintext / privilege exposure

| # | Finding | Location | Class + gate |
|---|---------|----------|--------------|
| 1 | Embed widget writes each secret's **plaintext into a `data-value` attribute** in an open Shadow DOM; host-page JS reads all values with no reveal. Defeats the mask; violates EMBED_STATE transition #7. | `frontend/src/embed/widget.ts:367-382` | Type-2, **Security veto** (embed) |
| 2 | **OAuth scope escalation** — requested scopes never validated ⊆ `client.Scopes`; a `read`-only client can mint `admin`/`promote` tokens. | `oauth_service.go` `Authorize` / `ClientCredentialsGrant` | Type-2 (auth), **Security veto** + negative-auth test |
| 3 | **Raw MCP tool arguments persisted in cleartext** to `mcp_gateway_log.request_payload`; args routinely carry credentials ⇒ plaintext-at-rest. | `handlers_mcp_gateway.go:193-200` → `mcp_repo.go:218-221` | Type-2, **Security veto** + threat delta |
| 4 | **`GetSSOConfig` returns the encrypted IdP client secret with no authz / no caller** — latent account-takeover primitive (inverse of `ListSSOConfigs` hardening). | `sso_service.go:61` | Type-2 (auth), **Security veto**; or delete (Type-3) |

### Tier 2 — audit & forensic integrity

| # | Finding | Location | Class + gate |
|---|---------|----------|--------------|
| 5 | **Spoofable client IP** — `SetTrustedProxies` is never called anywhere, so `X-Forwarded-For`/`X-Real-IP` are trusted: (a) audited source IP on every secret mutation + login is forgeable; (b) per-IP rate limits, incl. the ADR-0006 embed-enumeration control, are bypassable. | `middleware.go:215` (`TrustedProxyMiddleware`) + `router.go:30` (`SetupRouter`) | Type-2, **Security veto** + threat delta |
| 6 | **`DetectDrift` bulk-decrypts every secret in two environments and writes a row, with NO audit event** — violates the CLAUDE.md audit hard-gate. | `handlers_intelligence.go:77` → `drift_service.go:35` | Type-2 + **audit-taxonomy extension** (ADR-0014) + audit-row test |
| 7 | **IP allowlist fully implemented but never wired** (`IPAllowlistService`, fail-open, zero callers) — operators adding rows get false assurance. | `sso_service.go` (`IPAllowlistService` / `CheckIPAllowed`) | Decision: wire (Type-2 + Security) **or** delete table (Type-1 schema) |

### Tier 3 — error leaks & correctness

| # | Finding | Location | Class + gate |
|---|---------|----------|--------------|
| 8 | **`err.Error()` leaked to clients via `gin.H{"error": …}` at 34 sites** (decrypt + LLM-provider errors); the `error_leak_test.go` gate doesn't catch this shape, so all 34 passed CI. | `handlers_intelligence.go` (×34) | **Type-3** httperror migration + Type-2 lint-gate extension |
| 9 | **`err.Error()` in MCP error body** (exec/runtime detail leak) — only inconsistent branch in the handler. | `handlers_mcp_gateway.go:206` | Type-2 (sensitive surface), small |
| 10 | **`RunDetection` hardcodes SQLite `strftime()`** and bypasses the dialect layer ⇒ frequency-spike anomaly detection **silently dead on Postgres (prod)**; the query error is discarded. | `anomaly_service.go:57-152` | Type-2 bug fix (non-sensitive) + per-dialect test |

## Notable unverified (from discovery, not yet adversarially confirmed)

- **High:** `organization_repo.ListProjectsByOrg` zeroes `AllowedOrigins`/`EmbedPolicyEnabled`; `SecurityTab` renders fabricated mock security data when the dashboard API throws.
- **Performance:** promotion **N+1** in `runPromotion` (`promotion_service.go`); crypto **AES+GCM rebuilt per call** (`crypto.go`); `OrganizationManagePage` re-fetches whole list per mutation; `enterprise_repo` unbounded list queries (no pagination).
- **Refactor:** `HelpPage.tsx` is one **1578-line** module (page shell + 12 doc components).
- **Cross-cutting (R2 critic):** **zero request-context / timeout propagation** in the backend — no `QueryContext`/`ExecContext`, no `c.Request.Context()` threaded into services, AI calls use `http.NewRequest` without context. Its own remediation effort.

## Remediation sequence (proposed)

1. **Quick-wins (agent-landable now):** #8 (httperror migration + extend `error_leak_test.go` to catch the `gin.H` shape), #9, #10 — Type-3 / non-sensitive, each with tests.
2. **Security-veto, design-first:** #1, #2, #3, #5, #6 — each needs a `docs/THREAT_MODEL.md` delta (and an ADR for #6's taxonomy change) + Security Engineer sign-off **before** code.
3. **Decision required:** #7 (wire vs. delete the IP-allowlist).
4. **Second verification pass** over the unverified 127 (start with the two highs above).

Findings 1–6 and 8–10 should be filed into [`docs/FOLLOWUPS.md`](../FOLLOWUPS.md) with owners + due dates per that doc's contract.

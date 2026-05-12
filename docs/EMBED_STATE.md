# Embed Widget — State Machine + Security Invariants

Companion to `frontend/src/embed/`. Documents the widget's state machine, the security invariant of each transition, and the audit-log requirement attached to it. **Every transition that touches a secret value MUST emit an audit-log event back to the backend** — today most do not, which is the most important gap on this page.

This is a working spec used by Frontend Engineer 30-day work item §3 of `docs/ROLES_30_60_90.md`. It also feeds the Security Engineer's threat-model refresh.

---

## States

| State              | Definition                                                        | File:line                                |
|--------------------|-------------------------------------------------------------------|------------------------------------------|
| `unauthenticated`  | Widget mounted but no credentials yet; awaiting host postMessage  | `frontend/src/embed/keepsave-widget.ts:85` |
| `authenticated`    | Token or API key received; widget initialized                     | `frontend/src/embed/keepsave-widget.ts:108-113` |
| `loading`          | `listSecrets` (or other read) fetch in flight                     | `frontend/src/embed/widget.ts:45`        |
| `secrets_loaded`   | List in memory                                                    | `frontend/src/embed/widget.ts:50`        |
| `empty`            | List loaded but length 0                                          | `frontend/src/embed/widget.ts:193`       |
| `revealed`         | One or more secret IDs in the `revealed: Set<string>`             | `frontend/src/embed/widget.ts:10`        |
| `editing`          | Inline edit form open on one secret                               | `frontend/src/embed/widget.ts:11`        |
| `adding`           | Add-secret form open                                              | `frontend/src/embed/widget.ts:13`        |
| `error`            | API or network failure surfaced to user                           | `frontend/src/embed/widget.ts:52`        |

## Transitions and their security contracts

| #  | From → To                     | Action            | Security invariant                              | Audit event today | Audit event REQUIRED |
|----|-------------------------------|-------------------|-------------------------------------------------|-------------------|----------------------|
| 1  | unauthenticated → authenticated | postMessage auth | Origin MUST match integrator allow-list (gap §3) | none              | `widget.authenticated` |
| 2  | authenticated → loading       | mount or env switch | identity confirmed; project_id from attribute trusted | none              | none (read precedes audit) |
| 3  | loading → secrets_loaded      | listSecrets ok    | response must be in expected shape              | server-side       | server-side OK |
| 4  | loading → error               | listSecrets fail  | error message MUST NOT contain secret values    | none              | `widget.read_failed` |
| 5  | secrets_loaded → revealed     | reveal click      | plaintext briefly in DOM; **no localStorage write** | none              | **`widget.secret_revealed`** |
| 6  | revealed → secrets_loaded     | hide click / env switch | auto-hide timer fires? (gap)              | none              | `widget.secret_hidden` |
| 7  | secrets_loaded → editing      | edit click        | plaintext in `<input>`; no persist               | none              | `widget.edit_started` |
| 8  | editing → secrets_loaded      | save success      | server side audits update; UI may add own       | server-side       | server-side OK |
| 9  | secrets_loaded → secrets_loaded | delete (confirm) | confirm() shown; cancel = no-op                | server-side       | server-side OK |
| 10 | secrets_loaded → adding       | add click         | empty form; no secret yet                       | none              | none |
| 11 | adding → secrets_loaded       | save success      | server side audits create                       | server-side       | server-side OK |
| any | * → error                    | unhandled         | error UI MUST NOT include the value, only key   | none              | `widget.error` |

## Storage rules

- **In-memory only** for tokens, API keys, plaintext secret values, edit buffers.
- `localStorage` / `sessionStorage` / `indexedDB` writes of any of the above are **forbidden**.
- Today the *embed* widget complies (no storage usage in `frontend/src/embed/`). The *dashboard* (non-embed) has plaintext-token concerns tracked separately — see "Dashboard storage debt" below.

## Dashboard storage debt (not in scope of this widget, but tracked)

Found during the embed audit. These are non-embed paths, but they're real:

- `frontend/src/api/client.ts:27, 31, 35` — JWT in `localStorage('keepsave_token')`.
- `frontend/src/api/ai.ts` — JWT in `localStorage('keepsave_token')`.
- `frontend/src/pages/HelpPage.tsx` — reads `localStorage('jwt')` (note: different key name — likely a bug).
- `frontend/src/hooks/useAuth.ts:9, 36, 46` — user PII (email, id, name) in `localStorage('keepsave_user')`.

This is the standard trade-off (refresh-without-relogin vs. token-on-disk). Acceptable for the dashboard if (a) we keep the JWT 24h TTL of ADR-0002, and (b) we *never* extend this pattern to secret plaintext. Open follow-up: harmonize key names (`keepsave_token` vs `jwt`) and move tokens to `sessionStorage` for tab-scoped survival without disk persistence. Tracked in `docs/FOLLOWUPS.md` (to be added).

## Auto-clear / timeout policy

**Not implemented today.** Required behavior for the widget:

- Revealed secrets auto-hide after **60 seconds** of no interaction.
- Edit buffer auto-clears after **5 minutes** of inactivity (also triggers a save-prompt).
- Page visibility change (`document.visibilitychange` → hidden) hides any revealed secret immediately.

Owner: Frontend Engineer. Due: 30 days.

## Test obligations

A Playwright/Chromatic test for each numbered transition above. Visual regression baseline for state **5** (revealed) is the priority — it's the highest-leakage screen.

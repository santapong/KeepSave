# Embed Widget — postMessage / Cross-Origin Policy

The embed widget (`<keepsave-widget>`) runs on integrator origins. Today's policy is **too permissive** — this doc states both the current behavior (so it's auditable) and the required behavior (so the gap is explicit and fixable).

This is the Frontend Engineer 30-day work item §2 from `docs/ROLES_30_60_90.md`.

---

## What the widget does *today*

**Outbound `postMessage`** (`frontend/src/embed/auth.ts:33`):
```ts
window.parent.postMessage(
  { type: 'keepsave-auth-request', widgetId },
  '*'                          // ← wildcard target origin
)
```

**Inbound listener** (`frontend/src/embed/auth.ts:21-26`):
```ts
window.addEventListener('message', (ev) => {
  if (ev.data?.type === 'keepsave-auth' && (ev.data.token || ev.data.apiKey)) {
    onAuth(ev.data.token, ev.data.apiKey)  // ← no ev.origin check
  }
})
```

**No allow-list** of integrator origins exists. Credentials may be accepted from *any* origin posting a message of the right shape.

**Direct embed mode** (`frontend/src/embed/keepsave-widget.ts:61, 65`) reads `token` and `api-key` as HTML attributes — those are trusted by the host page already, so no cross-origin issue, but a credential in an HTML attribute is observable by every script on the page.

## Threat model

| Threat                                                | Today | After policy enforcement |
|-------------------------------------------------------|-------|--------------------------|
| Malicious page embeds widget, sends fake `keepsave-auth` from `window` itself → widget accepts attacker's token | **Open** | Mitigated by origin allow-list |
| Legit integrator's page is XSS'd; attacker reads the token attribute | Open | Still open (attribute mode) |
| Widget exfiltrates secret via `postMessage` to `*` | Not done today | Forbidden by policy |
| Widget receives `keepsave-secret-set` from a malicious sibling iframe | N/A — no such message exists | Stays N/A — never accept secret values inbound |

## Required policy (target behavior)

### 1. Allow-list of integrator origins

Source of truth: a server-side per-project setting (`projects.allowed_embed_origins TEXT[]`). The widget bootstrap fetches this list with the project ID *before* accepting any auth message. No origin in the list → no widget activates.

- **Schema migration:** additive — `ALTER TABLE projects ADD COLUMN allowed_embed_origins TEXT[] DEFAULT '{}'`.
- **API:** `GET /api/v1/projects/:id/embed-config` returns `{ allowed_origins: string[] }`. Unauthenticated read — only project ID is required, because the widget hasn't been authenticated yet at this point. The endpoint must rate-limit by IP to avoid enumeration of valid project IDs.

### 2. Strict origin checks on every message

```ts
// outbound
window.parent.postMessage(msg, ALLOWED_ORIGIN)   // never '*'
// inbound
addEventListener('message', (ev) => {
  if (!allowedOrigins.has(ev.origin)) return       // drop silently
  // … existing handling
})
```

Wildcard `'*'` is forbidden in both directions. Code reviewers reject it.

### 3. Message schema is fixed and exhaustive

Permitted message types (outbound from widget): `keepsave-auth-request`, `keepsave-resize`, `keepsave-error`.
Permitted message types (inbound to widget): `keepsave-auth`.

**Secret values are NEVER carried in any postMessage**, inbound or outbound. Period. No exceptions. Code review rejects PRs adding a message type that includes a value.

### 4. Framing controls

Widget MUST refuse to run inside a frame whose top window's origin is not in the allow-list:

```ts
if (window.top !== window && !allowedOrigins.has(document.referrer.origin)) {
  renderError("This origin is not authorized to embed KeepSave.")
  return
}
```

This is belt-and-suspenders to the server-side allow-list — even if a malicious page somehow obtains a project ID and serves the widget bundle, the widget self-disables.

### 5. CSP for the *host* page

We publish a recommended CSP for integrators:

```
Content-Security-Policy:
  frame-src https://widget.keepsave.example;
  frame-ancestors 'self';
```

Integrators apply this to their own pages. We document it in `docs/EMBED_INTEGRATION.md` (to be written by Tech Writer interim Tech Lead).

## What gets implemented in Phase A (30 days)

- Allow-list migration + endpoint (Backend Engineer).
- Strict origin checks in the widget (Frontend Engineer).
- Framing guard (Frontend Engineer).
- Documentation: this file + integrator CSP guidance (Tech Writer / interim).

## What gets deferred to Phase B

- **Per-message MAC / replay protection.** Considered but not needed at the current scope; `postMessage` semantics + origin checks are enough. Revisit if a customer with a high-threat model asks.
- **SRI / signed widget bundles.** Browser-level integrity for the script tag is a separate concern, owned by DevOps.

## Test plan

- Unit: origin allow-list helper accepts known good, rejects unknown, rejects malformed.
- Integration: Playwright host page that posts auth from a non-allow-listed origin — widget must not authenticate.
- Integration: Playwright host page that *is* allow-listed — widget authenticates.
- Negative: malformed message types are dropped, no exceptions propagate.

## References

- `frontend/src/embed/auth.ts:21-33`
- `frontend/src/embed/keepsave-widget.ts:61, 65, 85, 108-113`
- ADR-0002 (auth model — origin trust is auth-adjacent)
- HTML Living Standard §`window.postMessage`

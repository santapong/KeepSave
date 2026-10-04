# KeepSave embed origin and messaging policy

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Reconciled 2026-10-04 from [auth.ts](../frontend/src/embed/auth.ts),
[keepsave-widget.ts](../frontend/src/embed/keepsave-widget.ts) and the
[embed handlers](../backend/internal/api/handlers_embed.go). The original
wildcard/no-allowlist proposal is preserved in
[EMBED_ORIGIN_LEGACY](archive/EMBED_ORIGIN_LEGACY.md); it is not current behavior.
See [ADR0006](adr/0006-embed-widget-origin-allowlist.md), [widget states](EMBED_STATE.md)
and [current status](STATUS.md). Local fixtures do not constitute a live integrator
security assessment.

## Current boot and API contract

The postMessage path loads `GET /api/v1/embed-config/:project_id` **before** starting
the credential handshake. This public, rate-limited endpoint returns only
`{project_id, allowed_origins, embed_policy_enabled}`. Missing/disabled projects
share a 404 response; it returns no owner, key, secret identifier or value.

`PUT /api/v1/projects/:id/embed-config` is authenticated and uses current project
management authority. It sets the explicit origins/enabled setting and refuses
a wildcard. A project ID alone never grants vault authority.

The widget:

1. Requires the selected project ID and loads its public embed configuration.
2. Refuses missing/disabled/unavailable configuration and an empty valid origin list.
3. Detects the parent origin from `document.referrer`, falling back to its own
   `window.location.origin`; requires exact membership in the stored allowlist.
4. Starts a handshake bound to that one matched origin. `createAuthHandshake`
   refuses empty/`*`, rejects inbound `event.origin` mismatches and uses the same
   exact outbound target origin.
5. Passes the resulting token or API key to the normal protected API. Stored
   project/environment/key/parent scopes remain server enforced.

The origin is scheme + hostname + port. `localhost` and `127.0.0.1` are distinct.
Do not widen an allowlist to make an unexpected host work; configure the intended
integrator origin deliberately. Failed boot renders an inert auth prompt and
safe console diagnostics. CORS and a handshake allowlist are browser boundaries,
not credential authorization.

## Messages and direct-attribute mode

The implemented outbound request is `keepsave-auth-request` with a widget ID.
The implemented inbound type is `keepsave-auth` with a token or API key. Secret
**values** must never be added to postMessage payloads. Authentication credentials
are distinct from secret content and still require careful host custody.

Current code checks the expected type and presence of a credential but does not
provide a general exhaustive message-schema validator, source-window binding,
per-message MAC or replay protocol. Do not claim these proposed controls exist.
Shadow DOM style isolation is not a security boundary against the host's scripts.

`token` and `api-key` attributes provide a separate direct mode and bypass the
postMessage boot allowlist. They are observable by scripts on the host page.
Use a trusted host with scoped short-lived authority; never place a production
credential in static HTML/source control. Attribute mode does not confer broader
server scope and cannot protect against a compromised integrator page.

## Integration configuration

The widget defaults `api-url` to the hosting page origin. For a third-party host,
set the intended KeepSave application origin explicitly; it is an origin, not
an `/api/v1` suffix:

```html
<script src="https://your-keepsave-host/embed/keepsave-widget.js"></script>
<keepsave-widget
  api-url="https://your-keepsave-host"
  project-id="your-project-id"
  mode="read"
  theme="dark">
</keepsave-widget>
```

The first-release application target is `https://app.keepsave.draveniq.dev`;
the static landing is not an API host. Use exact CORS origins and a host CSP that
allows only the selected script/API/frame destinations required by the actual
embedding mode. CSP/SRI configuration belongs to the integrator deployment; this
source guide does not prove it is installed. The
[integration guide](../frontend/src/embed/INTEGRATION.md) describes supported attributes.

## Security boundaries and required acceptance

| Threat | Implemented control | Remaining boundary |
|---|---|---|
| Another origin injects a message | Exact inbound/outbound origin, public-policy boot | Same-origin hostile script/source-window ambiguity is not attested |
| Missing/disabled project enumeration | Same 404 config response, rate limit, minimal metadata | Enabled embed metadata is intentionally public |
| Host page reads a credential/value | Scoped server authority and in-memory widget handling | Host/XSS can read attributes/DOM/memory; no device or host attestation |
| Credential changes widen project access | Protected API resolves current ownership/scopes | Origin/mode alone is not permission |
| Claimed iframe restrictions | Exact detected-origin check | No claim of an independently enforced top-ancestor-origin protocol |
| Secret exfiltration through messages | Secret content is absent from current message types | New messages need security review; host compromise remains |

Preserve no-wildcard, no-secret-message and no-browser-storage-for-widget-values
requirements. Test matched/mismatched origins, disabled/missing config, malformed
messages, credential substitution, CORS and scoped server denials. Repeat real
integrator/browser acceptance on exact deployed origins and retained client
versions. Unit fixtures and existing widget builds are scoped local evidence.

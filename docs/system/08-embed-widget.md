# 8. Embedded widget

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

The widget is a separate compatibility surface in
[`frontend/src/embed`](../../frontend/src/embed/), built independently from the
React app by `npm run build:widget`. It registers `<keepsave-widget>` and renders
inside an open Shadow Root. Shadow DOM separates styles; it is not a security
boundary against a host page with DOM/script control. The same-origin application
topology does not remove the widget's external-origin requirements.

## Configuration and authentication

Attributes select `project-id`, `api-url`, `theme` (`light`/`dark`) and `mode`
(`read`/`readwrite`). Direct `token` or `api-key` attributes deliberately entrust
credentials to the host page. The alternative postMessage handshake first reads
`GET /api/v1/embed-config/{project_id}` without credentials, refuses disabled/
missing/failed policy, strips wildcard origins and checks the detected parent
origin against the server's project allowlist. Only then does it request auth at
that exact origin. Auth messages contain a token or API key, not secret values.
Inbound messages require exact origin equality; outbound target is never `*`.

The API client keeps one credential in memory: setting a token clears the key
and vice versa. It sends Bearer or `X-API-Key`, using existing project/scoped-secret
routes. The now-implemented POST batch route accepts 1–100 keys and conceals
out-of-scope selections as missing; an older chapter's missing-route claim was
obsolete. Direct auth mode does not perform the postMessage policy bootstrap.

## Rendering and state

[`widget.ts`](../../frontend/src/embed/widget.ts) uses `createElement`,
`textContent` and `setAttribute`, avoiding template-string `innerHTML` sinks for
attacker-influenced keys/values. Destructive deletion uses typed key confirmation,
not a native yes/no dialog. Environment changes reset visible reveal/edit state.
A visibility-change listener immediately remasks revealed values when the tab is
hidden and is removed on renderer destruction. This is implemented; a timed
reveal policy should not be claimed from older requirements alone.

Values/edit buffers and widget auth live in memory; the embed source does not
persist them to browser storage. Masking does not mean the caller never received
plaintext: the underlying vault read returns authorized values. Attribute auth
and a malicious host remain trusted-host risks, and revocation cannot recall
cached data. Do not extend the dashboard's 401/cache-handling claims to this
client without its own acceptance exercise.

## Integration and qualification

Use [embed integration](../../frontend/src/embed/INTEGRATION.md),
[origin policy](../EMBED_ORIGIN_POLICY.md) and [state policy](../EMBED_STATE.md).
Host code must validate both the expected message origin and intended widget
window before responding and use a specific target origin. Do not put a
long-lived project key in public HTML or copy wildcard demo code into production.
Apply the [KeepSave branding](../BRANDING.md) without confusing theme isolation
with credential isolation.

Vitest covers API headers/routes, origin refusals, DOM handling, typed delete and
visibility masking; app/widget builds passed in the dated local receipts.
Production integrator/browser UAT, lifecycle cleanup and any broader widget
contract remain separately tracked in the
[acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).
The widget is not a GitHub broker adapter or a qualified native harness.

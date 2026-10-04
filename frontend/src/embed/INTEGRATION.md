# KeepSave widget integration guide

Source audit: October 4, 2026. The widget is a retained vault compatibility client,
not a GitHub broker or qualified native harness. Build/distribution source exists;
no published `keepsave-widget` NPM package or CDN/version 2 release is established by
this checkout. See the [documentation hub](../../../docs/README.md),
[widget chapter](../../../docs/system/08-embed-widget.md),
[branding](../../../docs/BRANDING.md) and
[acceptance ledger](../../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## Build and host your reviewed bundle

From the canonical project `/mnt/data/company/apps/KeepSave`:

```bash
cd frontend
npm ci
npm run build:widget
```

Vite emits `dist-embed/keepsave-widget.es.js` and
`dist-embed/keepsave-widget.umd.js` without source maps. Host a reviewed immutable
bundle yourself; these example paths are yours to provide, not a live KeepSave CDN.
Both entrypoints register `<keepsave-widget>`. The ES bundle also exports
`KeepSaveAPI`, `KeepSaveWidget` and `register`.

```html
<script src="/assets/keepsave-widget.umd.js"></script>
<keepsave-widget
  id="vault-widget"
  project-id="approved-project-uuid"
  api-url="https://app.example.test"
  theme="light"
  mode="read"
></keepsave-widget>
```

The `api-url` is the installation origin without `/api/v1`. The example application
origin is illustrative; `app.keepsave.draveniq.dev` is currently a deployment
target. Prepare a real host page, configured API origins and scoped test credentials
before browser acceptance.

## Attributes and auth modes

| Attribute | Default | Actual behavior |
|---|---|---|
| `project-id` | Required | Active project UUID; server stored ownership/scopes remain authoritative. |
| `api-url` | Current origin | Base origin of the API. |
| `theme` | `light` | `light` or `dark`; style isolation only. |
| `mode` | `read` | `readwrite` adds mutation controls; not a permission grant. |
| `token` | Absent | Direct browser Bearer token, taking precedence over `api-key`. |
| `api-key` | Absent | Direct scoped key; authorizes only its server-side scope/expiry. |

Direct attributes bypass postMessage bootstrap and deliberately trust the host
with credentials. Do not embed long-lived keys in public HTML. Current human
tokens belong to database-revocable 24-hour SID/hash sessions; the widget has no
human refresh-token flow. Revocation blocks subsequent requests but cannot recall
values already fetched.

Without direct credentials, the widget first reads
`GET /api/v1/embed-config/{project_id}`. A missing/disabled/failed policy or absent
specific parent-origin allowlist refuses the handshake. Only then does it send
`keepsave-auth-request` to the exact allowlisted parent. Inbound `keepsave-auth`
requires that exact origin and contains token or API key, never secret values.
For an iframe integration, the host should also verify the expected window:

```javascript
// Integrator supplies a reviewed credential getter; no secret is committed here.
function installWidgetHandshake(frame, widgetOrigin, getCredential) {
  const onRequest = async (event) => {
    if (event.origin !== widgetOrigin || event.source !== frame.contentWindow) return;
    if (event.data?.type !== 'keepsave-auth-request') return;
    const credential = await getCredential();
    if (event.source !== frame.contentWindow) return;
    if (typeof credential?.token === 'string') {
      event.source.postMessage({ type: 'keepsave-auth', token: credential.token }, widgetOrigin);
    } else if (typeof credential?.apiKey === 'string') {
      event.source.postMessage({ type: 'keepsave-auth', apiKey: credential.apiKey }, widgetOrigin);
    }
  };
  window.addEventListener('message', onRequest);
  return () => window.removeEventListener('message', onRequest);
}
```

`getCredential` must return only the reviewed `{token}` or `{apiKey}` for the
current caller/project; it is an integrator responsibility, not an implemented
public credential-delivery endpoint. `widgetOrigin` must be the fixed origin of
your reviewed iframe page, never `*`. Configure the project's parent origin
allowlist independently. Shadow DOM or postMessage does not protect values from
a malicious host page with DOM/script control.

## Programmatic API and batch

Import the locally built ES bundle and pass a credential obtained in the current
reviewed context; do not print it or returned values:

```javascript
import { KeepSaveAPI } from './assets/keepsave-widget.es.js';

const api = new KeepSaveAPI('https://app.example.test');
// A scopedKey is supplied by your explicitly reviewed integration context.
api.setApiKey(scopedKey);
const result = await api.batchGetSecrets(projectId, 'alpha', ['DATABASE_URL']);
// result.secrets contains permitted values; missing_keys conceals denied/absent keys.
```

The source exports `listSecrets`, `createSecret`, `updateSecret`, `deleteSecret`
and `batchGetSecrets`. Token/key setters clear the other credential. Batch's
actual backend route is POST `/api/v1/projects/{id}/secrets/batch`, accepts
`{environment,keys}` with 1–100 keys and is classified as a **read**. This lightweight
client supplies no full language-SDK retry/circuit-breaker/cache guarantee or
server-revocation-aware plaintext cache clearing.

## Rendering, theme and qualification

Current rendering uses `createElement`/`textContent`/`setAttribute`, typed key
confirmation for delete and immediate remasking when the tab becomes hidden.
Environment changes reset reveal/edit state; disconnect removes global listeners.
A timed reveal timeout is not claimed. Credentials and values remain in memory
in the embed source; direct attribute/host access remains a separate trust risk.

`--ks-color-*` custom properties style the host element. Follow Field Twist's
violet/mint identity with readable light/dark contrasts when customizing, rather
than claiming the legacy blue widget palette already matches every app asset.
A read-only UI mode does not override backend roles. Modern Web Component APIs are
used, but the old Chrome/Firefox/Safari minimum-version list was not an executed
browser matrix and is not a support promise.

Vitest and widget-build receipts are dated in the ledger. Qualified integrator
origin checks, current-session failure handling, actual browser accessibility and
host cleanup require their own tests. New backend history/recovery/tool guarantees
are PostgreSQL-only; merely loading this bundle does not qualify them.

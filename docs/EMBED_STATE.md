# KeepSave widget state and security contract

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Source reconciled 2026-10-04 against [widget.ts](../frontend/src/embed/widget.ts),
[element lifecycle](../frontend/src/embed/keepsave-widget.ts) and
[embed API](../frontend/src/embed/api.ts). This updates the historical Phase A
gap list; it does not claim new live browser/widget acceptance. See
[origin policy](EMBED_ORIGIN_POLICY.md), [current status](STATUS.md) and
[acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## States and transitions

| State / transition | Current source behavior | Security boundary |
|---|---|---|
| Mounted → auth prompt | PostMessage mode loads public per-project policy first; direct attributes are a separate path | Origin equality is not authorization; server checks current credential scope |
| Auth prompt → authenticated | Matched-origin handshake sets either browser token or API key in the API object | No credential or value browser-storage writes in embed source |
| Authenticated → loading | List values for selected project/environment | A real permitted vault read, independently audited by the server |
| Loading → values / empty / error | Populated in-memory list, empty message or visible failure | Masking is presentation; authorized plaintext already exists in memory |
| Values → revealed / hidden | Toggle a record's reveal set | Never persist value to DOM attributes or browser storage; edit gets value from memory |
| Revealed → hidden tab | `visibilitychange` immediately clears reveal set and rerenders | Masks visible value; does not erase all value/edit memory |
| Environment change → loading | Clear revealed set and active forms before new load | Server reauthorizes new environment; async response isolation needs separate review |
| Values → editing / adding | In-memory form state; cancel clears the active form's buffer | No value in postMessage, logs, public errors or data attributes |
| Edit/add → saved | API mutation, clear active buffer and reload | Server transaction owns revision, required audit/outbox and permission |
| Delete requested → confirmed | Modal requires exact key typing; cancel sends no delete | Readwrite UI mode does not grant server write scope |
| Element reconfiguration/disconnect | Destroy handshake/renderer listeners; new setup rebuilds API/renderer | Listener teardown does not prove secure zeroization or cancellation of in-flight fetches |

`mode="read"` is the default and hides mutation controls; `readwrite` exposes them.
The backend enforces actual role/scope. An attacker changing an HTML attribute
cannot obtain server permission. Project/environment context and errors must stay
clear, with KeepSave's Field Twist visual identity and recognizable action icons.

## Audit ownership

Sensitive credential reads and enabled PostgreSQL mutations are audited by the
server's authorized vault service. Local mutation, immutable revision, required
audit and outbox commit together. UI reveal/hide/edit-start transitions do **not**
currently emit `widget.*` journal events. A client-reported UI event would not
prove an authorized server read; do not claim those proposed event names are wired.
See [AUDIT_LOG_COVERAGE](AUDIT_LOG_COVERAGE.md).

## Storage and current limits

The widget API keeps one token/API key in memory, and its renderer retains
permitted values/edit buffers in memory. Embed source uses no localStorage,
sessionStorage or IndexedDB credential/value persistence. Attribute credentials
remain visible to host scripts; Shadow DOM is style isolation, not confidentiality
against a malicious host or XSS.

The dashboard is a separate implementation. Its canonical `keepsave_token`
accessor prefers **sessionStorage**, migrates known legacy keys and explicitly
falls back to **localStorage** if sessionStorage is inaccessible. User metadata
may also use localStorage. Consequently, unconditional tab-only token storage
is not a current guarantee. No dashboard secret-value storage is permitted.

The application client has account-switch guards and protected-401 cache/proof
clearing; the retained embed API does not implement that automatic reset path.
Its error mapper can convert a nested error object to unhelpful text. Do not
claim equivalent cache-clearing/error UX across every adapter. Treat these as
bounded compatibility limitations, not silently successful logout or erasure.

## Auto-clear / timeout policy

Implemented in the widget: visibility-hidden remasking, environment-change
remasking, typed delete confirmation and edit/add cancellation buffer clearing.
The previous proposal's **60-second reveal inactivity timer** and **five-minute
edit inactivity clearing/save prompt** are not implemented in `widget.ts`; they
remain requirements to close before making that widget timeout claim. Renderer
destruction removes listeners, not all plaintext memory.

The dashboard Secrets panel separately implements a **30-second reveal countdown**,
visibility-hidden remasking, typed delete, request-generation guards and a
best-effort **20-second clipboard clear** that first checks the copied value.
These behaviors do not establish the widget timers or guarantee clipboard erasure.

## Verification obligations

Preserve unit checks for exact origin, token/API-key selection, default masking,
hidden-tab remasking, typed delete, safe edit source and DOM construction.
Source fixtures are in [embed tests](../frontend/src/embed/). For changes, extend
actual server/router scope/read-audit/transaction tests and real integrator
acceptance for loading, empty, denied, expired, failed and account/context-switch
states. Exercise late responses and teardown explicitly rather than inferring
cancellation from listener removal. Never record live credentials in screenshots.

Security Engineer review remains required for credential reveal/edit/copy,
origin-policy changes or storage boundaries. Source presence, rendering and
synthetic unit checks are distinct from real end-to-end acceptance.

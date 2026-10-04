# 7. Frontend and product identity

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

The application uses React 19, TypeScript and Vite, with React Router and lazy
loading for authenticated feature pages. Its package remains `1.4.0-rc.1`.
[`App.tsx`](../../frontend/src/App.tsx) is the route composition;
[`api/coreTypes.ts`](../../frontend/src/api/coreTypes.ts) is generated from the
maintained management contract. Node 24.21.0 is the build/CI pin.

## Accepted visual system

KeepSave retains **Field Twist / Event Horizon**: a distinctive black-hole mark,
violet/mint palette, clear dark/light surfaces and familiar task silhouettes with
subtle orbit detail. The shared
[`Brand`](../../frontend/src/components/cosmic/Brand.tsx) uses the common `EhMark`
and accessible “KeepSave home” label. Do not replace it with generic lock/cloud
branding or turn ordinary actions into ambiguous astronomy symbols. See
[branding](../BRANDING.md) for accepted assets and diagram usage.

## Pages and capability gates

| Area | Implementation |
|---|---|
| Public | Landing, password register/login, social callback, identity confirmation, password recovery and MCP consent. |
| Account | Provider connections, login methods, contact proofs, session metadata/revocation and delegated-client metadata. |
| Workspace | Explicit organizations, current member roles, invitation and scoped offboarding controls. |
| Project | Masked values, environments, history/restoration, lifecycle metadata and safe audit browse/export. |
| Notifications | Lifecycle reminders when `team_vault` is available. |
| Developer access | Connections/bindings, immutable review sources/profiles/packages, workloads/grants, runs and receipts under `controlled_tools`. |
| Compatibility | Applications/templates/help and older dashboards; availability is determined per capability. |

`CapabilityProvider` and `CapabilityGate` use backend availability rather than
assuming a visible source page is executable. Old MCP hub, legacy OAuth and
experimental intelligence remain gated unavailable in the restricted profile.
Default-off candidate features must be labeled configured/tested/accepted
truthfully; frontend controls cannot enforce server permission on their own.

## Authentication and credential handling

Browser authentication preserves `{user,token}`. The API client prefers
sessionStorage for the Bearer token, migrates legacy localStorage tokens and
retains its documented storage fallback; it is not an HttpOnly-cookie system.
User display metadata is stored separately. Server admission checks the current
database session, not the browser's expiry estimate.

Account changes, successful logout and protected authentication failures clear
pending provider proofs and credential caches. A late response is applied only
if its originating account/project still matches. Failed server logout is shown
as a failure rather than claiming revocation. Social email collisions invite
explicit recent-session linking, never silent merging. Confirmation proof links
use browser fragments cleared before submission. MCP consent allows only the
validated consent-return route and exact registered callback/issuer.

Vault masking protects shoulder-surfing, not a trusted host page or browser
administrator. Explicit secret views load authorized values; lifecycle/audit
navigation uses metadata routes and does not request secret values. Client
caches/returned data cannot be recalled by server revocation. Model-assisted
repository reads may send permitted content to the harness's configured model.

## Build and evidence

`npm test`, `npm run lint` and `npm run build:all` cover tests, configured lint,
TypeScript/application output and the separate widget bundle. Current ESLint
coverage is concentrated on the embed SDK; a passing command is not a full
application lint/accessibility audit. Existing large-chunk warnings are retained,
with no performance claim. See the
[frontend receipt](../validation/2026-10-02-harness-neutral-platform/FRONTEND.md)
for exact aggregate/affected-suite counts and visible synthetic browser checks.
Real provider consent, email receipt, native client execution and production
browser acceptance remain separate gates. The public landing and planned
same-origin application are distinct deployment surfaces.

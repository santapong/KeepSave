# Frontend Dead-UI Audit

Audit date: 2026-05-15
Scope: `/home/user/KeepSave/frontend/src/` (.tsx and .ts, excluding `*.test.*`)
Auditor: Frontend Dead-UI Auditor (read-only)

---

## 1. Scope and methodology

Every non-test `.tsx`/`.ts` file under `frontend/src/` was opened and read. The
search began with ripgrep / grep patterns over `--include="*.tsx"`:

- `<button`, `<a `, `role="button"` — to enumerate clickable elements
- `onClick=`, `onSubmit=`, `onChange=`, `onKeyDown`, `onKeyPress` — to count
  handler bindings
- `console\.log`, `TODO`, `FIXME`, `onClick=\{\(\) => \{\}\}`,
  `onClick=\{noop\}` — to find stub/placeholder handlers
- `▾`, `▼`, `href="#"`, `cursor:.*pointer` — to find affordances (chevrons,
  pointer cursors) that suggest interactivity
- `<a href` — to find anchor tags pointing to non-routed targets
- For each suspected file, the JSX was read end-to-end and every interactive
  element was traced to its handler. Pages whose handlers all resolve to
  state mutations, API calls, navigations, or modal openings were marked
  clean. Pages with bare buttons, chevron-decorated affordances without
  onClick, or props that are destructured-but-never-used were flagged.
- Component-import sweep: every `export function <Name>` was reverse-searched
  for import sites to detect orphans.
- `console.log` hits were all inside HelpPage doc/code-block strings, not real
  handler bodies. No `// TODO` / `// FIXME` markers were found in production
  source. No `onClick={() => {}}` no-op handlers were found.

---

## 2. Findings table

Severity legend:
- **critical**: load-bearing flow broken (login, save secret, promote)
- **high**: visible to all users, prominent label, looks functional
- **medium**: less prominent or admin-only
- **low**: edge UI, icon-only, rarely-visited
- **info**: orphan import / dead code branch

| # | file:line | element | claimed purpose | actual behaviour | class | severity | suggested fix |
|---|-----------|---------|-----------------|------------------|-------|----------|---------------|
| 1 | `frontend/src/pages/ProjectDetailPage.tsx:122` | `<button className="ks-btn">Rotate all</button>` | rotate every secret in the project | no `onClick`; click does nothing | dead | **high** | wire to a rotation API or hide behind a feature flag; this is a security-sensitive surface |
| 2 | `frontend/src/pages/ProjectDetailPage.tsx:121` | `<button className="ks-btn">Export .env</button>` | export current secrets as a .env file | no `onClick`; click does nothing | dead | **high** | wire to `/projects/:id/export` or remove from header |
| 3 | `frontend/src/pages/ProjectsPage.tsx:174` | `<button className="ks-btn">Import .env</button>` | bulk-import secrets from a .env file | no `onClick`; click does nothing | dead | **high** | wire to file picker → import API; or remove from header |
| 4 | `frontend/src/components/Layout.tsx:63-72` | `<div className="ks-search">` with `⌘K` kbd hint | global command palette / search | no `onClick`, no input, no global `keydown` listener anywhere in the codebase (verified by grep of `keydown`, `metaKey`, `key === 'k'` — zero hits) | dead | **high** | implement a global cmd-K palette, or remove the search affordance and the kbd hint |
| 5 | `frontend/src/pages/LoginPage.tsx:138-143` | `<span className="ks-amber" style={cursor:'pointer'}>RECOVER →</span>` inside Password label | password reset link | no `onClick`, no `Link to`; pointer cursor but inert | dead | **high** | wire to a forgot-password route, or remove |
| 6 | `frontend/src/components/Sidebar.tsx:76` | `<span className="ks-amber">acme-platform ▾</span>` | org switcher dropdown (chevron implies dropdown) | no `onClick`; hardcoded org name, no list | dead | **medium** | implement org switcher or remove the `▾` chevron |
| 7 | `frontend/src/pages/ProjectsPage.tsx:207` | `<button className="ks-filter-btn">SORT · ACTIVITY ▾</button>` | sort dropdown | no `onClick`, no menu; `filtered` is sorted only by the underlying API order | dead | **medium** | implement sort menu or remove |
| 8 | `frontend/src/pages/ProjectsPage.tsx:208` | `<button className="ks-filter-btn on">TABLE</button>` | view-mode toggle (table/grid) | no `onClick`; clicking does nothing — there is only the table view | dead | **low** | implement grid view or remove — the `.on` class also hardcodes the active state |
| 9 | `frontend/src/components/Layout.tsx:84-104` | floating `≡` button (`aria-label="Open menu"`) | open mobile sidebar menu | toggles `mobileOpen` which only renders a black scrim overlay (line 40-45). No sidebar is shown; tapping the overlay closes it. Sidebar component is rendered unconditionally and not affected by `mobileOpen`. | stub | **medium** | actually toggle the Sidebar component's mobile-open state, or remove the floating button on mobile |
| 10 | `frontend/src/components/Sidebar.tsx:60` (props destructured) and `frontend/src/components/Layout.tsx:47-52` (props passed) | `collapsed` / `onToggle` props | collapsible-sidebar feature | `Layout` passes `collapsed={collapsed}` and `onToggle={toggle}` (lines 49-50). Sidebar destructures only `{ user, onLogout }` (line 60). `useSidebar` writes `keepsave_sidebar_collapsed` to localStorage but the value is never read in render. No toggle UI exists in the sidebar. | stub | **medium** | render a collapse toggle in the Sidebar that uses `onToggle` and applies a class based on `collapsed`, or remove the unused hook and props |
| 11 | `frontend/src/pages/AdminDashboardPage.tsx:40` + `:56` | `TimeRangeSelector` value bound to `timeRange` state | filter dashboard data by time range | `timeRange` is set in state and rendered by the selector, but never passed to any tab component (`OverviewTab`, `MetricsTab`, `AgentsTab`, etc.). Verified: `grep -n "timeRange" components/dashboard/*.tsx` returns zero matches. | stub | **medium** | thread `timeRange` into the tabs (key, prop, or context) and use it to scope queries |
| 12 | `frontend/src/pages/LoginPage.tsx:181-189` | "Authenticate →" button under API-key mode | sign in with an API key | rendered with `disabled` attribute always set; the surrounding `<form onSubmit={(e) => e.preventDefault()}>` swallows submits. There is no real auth path. | dead | **medium** | implement API-key login flow, or remove the API-key mode tab |
| 13 | `frontend/src/pages/LoginPage.tsx:192-207` | three SSO buttons (Okta, Google Workspace, GitHub) | sign in with SSO | all three rendered with `disabled` attribute, no `onClick`. No SSO route exists. | dead | **medium** | implement SSO or remove the SSO mode tab |
| 14 | `frontend/src/components/Breadcrumbs.tsx` (entire file) | `export function Breadcrumbs()` | breadcrumb navigation | declared but never imported anywhere. Verified by grep — only self-references. The Layout duplicates ROUTE_LABELS and renders its own crumb string instead. | orphan | **info** | delete the file or wire it into Layout in place of the inline crumb string |
| 15 | `frontend/src/pages/APIKeysPage.tsx` (entire file) | `export function APIKeysPage()` | global API key management page | declared but never imported. App.tsx has no `/api-keys` global route; per-project keys live in `ProjectAPIKeysPanel`. This entire 275-line page is dead. | orphan | **info** | delete the file or add a `/api-keys` route in App.tsx |
| 16 | `frontend/src/pages/ProjectDetailPage.tsx:88-94` | `tabs[].count?: string` field on tab interface | show counts next to tab labels | the count is rendered (line 149) only when truthy, but no tab object actually provides a `count` value. The branch is unreachable. | stub | **info** | populate counts from API or remove the optional field |
| 17 | `frontend/src/components/Layout.tsx:55-62` | breadcrumb `acme-platform / vault / {now}` | navigation breadcrumb | `acme-platform` and `vault` are hardcoded strings. They are not clickable and do not navigate. | dead | **low** | turn into real Link elements or remove the static prefix |
| 18 | `frontend/src/components/Layout.tsx:74-75` | topbar pills `18.4k rps` and `SEALED` | live metrics indicators | hardcoded values, not bound to any state or API. Not clickable, but visually present every page. | dead (info-only) | **low** | bind to live metrics or remove |
| 19 | `frontend/src/pages/LoginPage.tsx:12-18` + `:67-74` | `SESSION_LEDGER` table on login left pane | session statistics ledger | values are hardcoded constants (`1,204`, `318`, etc.) rendered as if live data. Not interactive but misleading. | dead (info-only) | **low** | bind to real metrics or relabel as a sample |
| 20 | `frontend/src/pages/LoginPage.tsx:155-157` | `<label className="ks-login-check"><input type="checkbox" defaultChecked /> Trust this device · 30d</label>` | remember-device checkbox | uses `defaultChecked`, no `onChange`, no state; the value is never read on form submit (`handleSubmit` only sends email/password). | dead | **low** | wire to login payload or remove |
| 21 | `frontend/src/pages/ApplicationDashboardPage.tsx:262-264` | `<div className="w-1.5 h-1.5 rounded-full bg-green-500" /> Active` | per-app status indicator | hardcoded green for every app card. No health-check is performed. | dead (info-only) | **low** | bind to actual health probe or remove |

---

## 3. Summary stats

### Counts by severity
| severity | count |
|----------|-------|
| critical | 0 |
| high     | 4 |
| medium   | 7 |
| low      | 6 |
| info     | 3 |
| **total**| **20** |

Note: zero **critical** findings. The load-bearing flows audited (email-password login on `LoginPage`, secret create/update/delete in `SecretsPanel`, promote in `PromotionWizard`, approve/reject/rollback in `PromotionsList`, organization create/delete, OAuth-client registration, template create/apply, MCP server install/uninstall, application register/delete) are all properly wired.

### Counts by file
| file | findings |
|------|----------|
| `frontend/src/pages/LoginPage.tsx` | 5 (#5, #12, #13, #19, #20) |
| `frontend/src/components/Layout.tsx` | 4 (#4, #9, #17, #18) |
| `frontend/src/pages/ProjectDetailPage.tsx` | 3 (#1, #2, #16) |
| `frontend/src/pages/ProjectsPage.tsx` | 3 (#3, #7, #8) |
| `frontend/src/components/Sidebar.tsx` | 2 (#6, #10) |
| `frontend/src/pages/AdminDashboardPage.tsx` | 1 (#11) |
| `frontend/src/components/Breadcrumbs.tsx` | 1 (#14 — full file) |
| `frontend/src/pages/APIKeysPage.tsx` | 1 (#15 — full file) |
| `frontend/src/pages/ApplicationDashboardPage.tsx` | 1 (#21) |

### Half-shipped features

The audit surfaces several distinct partially-shipped feature clusters
rather than one single half-shipped flow:

1. **Marketing-style placeholders on the login page** (#5, #12, #13, #19, #20). The
   API-key mode and the three SSO buttons are all rendered `disabled` with no
   onClick. The session ledger and "Trust this device" checkbox are
   non-functional. This looks like a visual mock-up that ships in production.
2. **Collapsible / mobile sidebar** (#9, #10). The `useSidebar` hook persists a
   `collapsed` flag in localStorage, `Layout` reads it and passes it to
   `Sidebar`, but `Sidebar` never reads the prop. The mobile menu button shows
   a scrim but no menu. Strong indicator that a responsive-sidebar feature
   was started and then abandoned.
3. **Power-user navigation** (#4, #6, #7, #8, #17). Command palette (`⌘K`),
   org switcher (`▾`), project sort menu (`▾`), table/grid view toggle, and
   clickable top-breadcrumb segments are all decorated with the chevron /
   kbd / pointer-cursor affordances of interactivity but have no handler.
4. **Project-level bulk operations** (#1, #2, #3). "Rotate all", "Export
   .env", "Import .env" all rendered prominently in page headers, none
   wired. These are high-value, security-relevant features.
5. **Admin dashboard time filtering** (#11). The TimeRangeSelector control
   is rendered but its value never reaches the tab data-fetchers.
6. **Orphaned pages/components** (#14, #15). A full `APIKeysPage` (global,
   275 lines) and the `Breadcrumbs` component both exist with no import
   site. Likely cut during refactors that introduced per-project API keys
   and the inline-breadcrumb in `Layout`.

The pattern is **general drift across many features** rather than a single
incomplete feature: the dead surfaces span login, navigation, project
header, dashboard filtering, and orphaned files from older refactors.

---

## 4. False-positive watch-list

Items that look dead but might be intentional. Each line cites reasoning.

- `frontend/src/pages/LoginPage.tsx:181-207` — API-key and SSO modes carry
  `disabled` buttons and are visually present. They could be deliberate
  "coming soon" placeholders. However, the mode-tabs (`['password','key','sso']`)
  are user-selectable (not gated by any role/flag), so a user clicks
  "API key" or "SSO" and sees only dead controls. Flagged as dead
  rather than intentional. (#12, #13)
- `frontend/src/components/Layout.tsx:74-76` — `18.4k rps` and `SEALED` pills
  may be intentionally fake "design language" decoration in this typographic
  theme. They have no aria role and no cursor:pointer, so users would not
  expect them to do anything. Flagged **low** rather than higher. (#18)
- `frontend/src/pages/LoginPage.tsx:12-18` — `SESSION_LEDGER` is clearly
  decorative editorial content in the same style as the rest of the left
  pane. The reason it's flagged at all is the row format (label + value +
  status-dot) reads as live telemetry; a returning user could reasonably
  think these are their stats. (#19)
- `frontend/src/components/Layout.tsx:55-62` — `acme-platform / vault /` is a
  hardcoded breadcrumb prefix that visually frames the dynamic `{now}`
  segment. It is consistent with the typographic styling and probably
  intentional. Flagged **low**. (#17)
- `frontend/src/pages/ProjectsPage.tsx:208` — `TABLE` button has class
  `on`, suggesting it is the *active* item of a view-mode group. It is
  plausible this is intentional placeholder for a future grid/list toggle.
  Flagged **low** rather than higher. (#8)
- `frontend/src/pages/ProjectDetailPage.tsx:88-94` — `count` on the tab
  interface is *optional*. The fact no tab provides one is not strictly a
  bug; it's defensive typing. Flagged **info**. (#16)
- `frontend/src/components/Breadcrumbs.tsx` — could be intentionally kept
  for an upcoming layout refactor that wants per-page breadcrumbs. But
  there is no ADR, FOLLOWUPS.md entry, or TODO referencing it. Flagged
  **info / orphan** to surface, not to delete blindly. (#14)
- `frontend/src/pages/APIKeysPage.tsx` — could be the planned global
  `/api-keys` route. App.tsx has no such route, but a future feature might
  re-enable it. Flagged **info / orphan**. (#15)
- `frontend/src/pages/ApplicationDashboardPage.tsx:262-264` "Active" green
  dot — likely a placeholder until a health-probe endpoint is added.
  Flagged **low**. (#21)
- `frontend/src/components/Layout.tsx:84-104` mobile menu button — the
  scrim-but-no-menu behavior is more likely incomplete than intentional.
  Flagged **medium** as a stub. (#9)

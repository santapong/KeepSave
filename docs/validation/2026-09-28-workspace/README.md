# Workspace and icon verification — 2026-09-28

Observed local implementation on `feat/landing-event-horizon-20260928`, base `9e2b3ba`, in `/mnt/data/company/apps/KeepSave-landing`. Owner requested immediate icon and workspace redesign, continuing the selected Event Horizon direction. Existing landing, login and social-auth changes were preserved. No commit, push, merge or deployment.

## Changes

- Shared editable orbit-vault SVG, favicon version and KeepSave brand components.
- Grouped sidebar, desktop collapse, mobile drawer, account/theme controls and command search. Dialogs contain keyboard focus; command search returns focus to its trigger.
- Project directory with functional search, sort and grid/list controls, real project count, empty/error/retry states, creation and import dialogs.
- Project navigation links with current-page semantics, compact environment selector and contextual guidance. Existing secret masking and mutations remain intact.
- Scoped workspace light/dark colors and responsive layout. Mobile workflow summary is compact; desktop keeps the descriptions. Theme instances synchronize within the page and across storage events.

## Executed checks

- 110 frontend tests in 17 files passed, including search/sort/list/retry and synchronized theme behavior with storage denied. Run with `NODE_OPTIONS=--no-experimental-webstorage` on the installed Node 26 environment.
- Frontend lint passed; app and embedded-widget builds passed; `git diff --check` passed. Existing large-bundle and Node deprecation notices remain.
- Visible in-app Chromium at 1440×1000, the default desktop size, 390×844 and 320×800. Dark and light populated workspace inspected; initial empty workspace captured. At 320px, document scroll width was 310px; no horizontal document overflow.
- Created a disposable local account and `Demo · Storefront API` through the UI; seeded two additional labeled sample projects through the isolated local API. A dummy `DEMO_API_URL` value was saved through the UI, displayed masked in Alpha, and absent in UAT. No real secret material used.
- Checked project opening, grid/list selection, sorting, search/no-results/clear, theme switching from mobile and desktop, sidebar collapse/expand, mobile route navigation and close, command search/Enter-to-create, Ctrl+K, Escape and keyboard focus containment/return.
- Existing login renders the new shared icon and favicon; provider buttons remain truthfully disabled pending OAuth setup. Browser workspace console returned no warning/error entries in the final inspection.

## Corrected issues

The initial drawer inherited Tailwind's separate `translate: -50% -50%`; `transform: none` alone left it half offscreen. Explicit `translate: none` placed it at x=0/y=0. The converted command dialog inherited `position: relative` from the old palette panel, placing it below the document; explicit fixed positioning corrected it. Both were inspected after the fixes. Mobile brand navigation now closes its drawer.

## Evidence and limits

Screenshots and logs: `/mnt/data/keepsave-workspace-design-2026-09-28/`:

- `workspace-desktop.png`, `workspace-light.png`, `workspace-list.png`
- `workspace-empty.png`, `workspace-mobile-390.png`
- `project-secrets-desktop.png`, `project-mobile.png`
- `tests.log`, `lint.log`, `build.log`

The running preview is `http://127.0.0.1:4651/`, using the disposable social-preview backend at 18185. Three sample projects and one dummy secret remain in that preview for owner review. Normal viewport sizing was restored. This is a visual/functional local check, not owner design approval, a full accessibility audit, a backend security re-audit, live OAuth verification, or a real-device/cross-browser certification. Other workspace modules inherit the shell/theme but their full business workflows were not re-exercised. The account display falls back to “Your account” when no email is available.

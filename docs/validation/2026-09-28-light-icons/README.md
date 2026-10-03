# Light mode and KeepSave icons — local verification

Date: 2026-09-28. Worktree: `/mnt/data/company/apps/KeepSave-landing`, branch `feat/landing-event-horizon-20260928`. Existing landing, login, social-auth and workspace changes were preserved. This pass changes frontend appearance and icon rendering; no release, commit, push or deployment was performed.

## Changes

Light workspace surfaces use a neutral canvas, white cards/sidebar/dialogs, stronger text and input borders, clearer selected navigation/environment controls, readable status colors and subtle card separation. The Event Horizon welcome banner intentionally keeps a dark background and local light text so its artwork is no longer washed out. Changes are scoped away from the public landing/auth palettes.

A shared 91-icon SVG family replaces frontend Lucide imports and generic action glyphs. Provider logos and custom application artwork remain recognizable. See the [design notes](../../design/2026-09-28-icon-family/README.md).

Visual verification also exposed a card style overriding a portalled project dialog's fixed positioning. The workspace dialog rule now explicitly preserves fixed positioning and bounds its height. Its observed desktop center is y=500 in a 1000-pixel viewport. Mobile feedback now flows below the workspace rather than covering the grid/list controls; its desktop stacking is below dialogs.

## Automated checks

- 112 frontend tests across 18 files passed. Two tests cover decorative icon behavior, preserved control interaction, standalone accessible names and SVG styling/ref support.
- ESLint passed.
- TypeScript and production application/widget builds passed. The existing large Three.js chunk warning remains; this is not a performance benchmark.
- `git diff --check` passed. No Lucide source imports remain in `frontend/src`.
- Test invocation used `NODE_OPTIONS=--no-experimental-webstorage` for the installed Node 26/jsdom combination.

Logs and screenshots: `/mnt/data/keepsave-light-mode-2026-09-28/` (`tests.log`, `lint.log`, `build.log`).

## Visible browser checks

Local Chromium preview at port 4651, using the existing isolated backend at 18185 and disposable Demo projects:

- Light/dark workspace at desktop 1440 and mobile 390; project cards, navigation symbols, artwork, selected controls and theme switching inspected.
- Light navigation drawer at 390; new-project and command dialogs at 320. No horizontal document overflow at the checked mobile widths. Temporary viewport overrides cleared after verification.
- Light project detail with a masked dummy secret, selected Alpha environment, reveal/edit/delete icons and readable status badge. No secret reveal or modification performed.
- Application registration dialog: new default application icon and labelled optional symbol/upload controls inspected; dismissed without submitting.
- Complete icon gallery inspected at 16 and 32 pixels in both light and dark themes.

Representative evidence: `workspace-light-desktop.png`, `workspace-light-mobile.png`, `workspace-dark-desktop.png`, `workspace-dark-mobile.png`, `navigation-light-mobile.png`, `project-light-desktop.png`, `project-dialog-light.png`, `project-dialog-320.png`, `command-light-320.png`, `application-icon-light.png`, `icon-family-light.png`, `icon-family-dark.png`.

Computed representative solid-color contrast pairs (sRGB formula; rendered foreground/background colors checked in the browser):

| Pair | Ratio |
| --- | ---: |
| Body text / canvas | 13.45:1 |
| Muted text / canvas | 5.66:1 |
| Active navigation text / selected surface | 7.50:1 |
| Primary or selected environment label / violet | 7.05:1 |
| Input border / white field | 3.14:1 |
| Amber detail / selected navigation | 5.40:1 |

These are bounded visual/contrast checks, not a full accessibility audit or a claim of user-tested comprehension. Privileged dashboard populated states and real third-party OAuth login were not exercised in this pass. Existing OAuth setup still requires provider configuration.

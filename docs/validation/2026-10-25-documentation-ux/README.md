# Developer documentation and Robot UX acceptance

Executed 25 October 2026, Asia/Bangkok. This is synthetic acceptance of an
uncommitted frontend/documentation slice, not a release or production qualification.
Base provenance: `809588f06d55845c5325e2123b3f58ea899d246c`, branch
`feat/gargantua-landing-20261024`. The Robot receipt identifies the hash-verified
674-file transferred snapshot; this is not a fabricated remote Git checkout.

## Canonical task record

Worker completion was inspected by the coordinator before local acceptance.
Two workers inherited the coordinator model; exact model IDs, token costs and
causal learning benefit are unknown. Owner/release acceptance remains separate.

| ID | Deliverable and owner | Dependency | Acceptance / budget | State |
| --- | --- | --- | --- | --- |
| D1 | 13 task guides; `developer_docs`, integrated by coordinator | Existing source contracts | 172 frontend tests/35 files, lint/build; final 8 guide tests/lint/build after copy correction | Locally verified |
| D2 | Robot browser/API slice; coordinator, independent fixture review | D1 and disposable PostgreSQL/API | 11 executed scenarios, one browser, 90-second UI/60-second API limits | Locally verified |
| D3 | Resource runner; `robot_resource_runner`, integrated by coordinator | D2 and explicit Pi soft-memory approval | One CPU/256 PIDs, 2 GiB conservative RSS soft guard, 8 GiB host reserve, 70°C stop, bounded time/output | Locally verified |
| D4 | Visual review, evidence and temporary previews; coordinator | D1–D3 | Desktop/mobile screenshots inspected; authenticated live guide opened; finite services | Locally verified |

## Delivered behavior

[The guides](../../../frontend/src/pages/HelpPage.tsx) cover a first secret, local
setup, actual REST contracts, credential choice, scopes, history/restore/rotation,
recovery/export, promotion, repository SDKs/CLI, widget trust, security, developer
tools, MCP availability and troubleshooting. Search matches guide titles,
section names, summaries and topics. URLs such as `/help#developer-guide` retain
the chosen guide. Backend source and the accepted visual identity are preserved.

Outdated paths, parameters, response wrappers, restore preconditions and API-key
descriptions were corrected. Current secret reads, future provider custody and
installation-specific acceptance are explicitly distinguished.

## Executed checks

- **172 frontend tests in 35 files**, lint and application build passed. Final
  provider-setup wording was followed by **8 focused guide tests**, lint and build.
- **11/11 new Robot scenarios passed**, without skips: keyboard chapters,
  independent selections, mobile menu/FAQ, unavailable social providers, guide
  search/deep links, mobile/light mode, preserved drafts, actual secret
  save/reload/reveal/history, missing authentication, batch environment/input
  boundaries, session revocation and twelve serial reads.
- A destination-fixture guard rejected a remote target before access; this is
  not a general network-isolation proof. Each suite created and tombstoned its
  own disposable project. Browser descendants were confirmed stopped.
- Zero private password/key canaries were found in text report artifacts.
  Robot desktop 1280×900 dark and mobile 390×844 light screenshots were inspected;
  the authenticated developer guide was then opened in the visible IAB.
- The older API/widget fixtures were not executed. Their documented blockers
  remain in [the Robot README](../../../tests/robot/README.md). The retained
  six-test API dry-run parsed successfully; parsing is not live acceptance.

## Resource observations

The workstation fell below its 8 GiB RAM reserve and its old preview stopped,
preserving data. The operator explicitly approved Pi testing with CPU/PID caps
and a **soft RAM watchdog**. The Pi memory controller was absent: no hard RAM or
swap isolation is claimed. All four test-stack groups were observed.

| Component | Peak conservative process RSS | Peak PIDs |
| --- | ---: | ---: |
| Robot, Node and Chromium | 1,053.3 MiB | 79 |
| KeepSave API | 24.8 MiB | 6 |
| PostgreSQL | 92.7 MiB | 7 |
| Proxy and watchdog | 30.0 MiB | 3 |

The simultaneous aggregate peak was **1,199.3 MiB (about 1.17 GiB)**. Shared pages
can be counted more than once; individual peaks need not occur simultaneously.
The configured total was **one CPU and 256 PIDs**. The observation window was
**25.188 seconds**, including five baseline seconds; active Robot time was
**20.184 seconds**. Minimum host availability was **11.48 GiB**, and sampled Pi
temperature peaked at **62.8°C**. No resource guard fired during Robot.

Twelve successful serial reads had median **99.89 ms** and nearest-rank P95/max
**299.67 ms** under these constraints. This small sample establishes no capacity,
availability or SLA. Shared production processes, SSH transport and the IAB are
outside the measured component caps.

Preparation ran serially with ten-minute phase bounds. Checksum-verified private
ARM64 toolchains were Go 1.27.3 and Node 24.19.0. Actual test runtimes were Robot
7.4.2, Python 3.13.5, Browser 20.0.0 and Chromium 148.0.7778.0 (build 1223).
These are not the exact CI Go/Node pins or a remote CI rerun. Build CPU was
temporarily reduced for heat; sampled preparation temperature reached 69.4°C
with no firmware fault. Build timings are not product latency measurements.

## Retained corrections and limits

- Pi frontend admission was initially refused at 65°C; a cooled retry passed.
- A disk traversal raced with deletion of owned compiler caches and stopped
  preparation. Only disappearing-file races are now tolerated; other monitoring
  errors still stop work. Corrected preparation and the actual Robot run passed.
- Historical synthetic files on local `fuseblk` retained mode 0755 despite a
  requested chmod 0600. New credential fixtures used verified POSIX storage and
  mode 0600; no real provider credentials were used.
- Pi production services were not restarted/reconfigured. n8n liveness and
  readiness remained HTTP 200 afterward. Original local sample data is retained;
  the Pi trial uses a fresh synthetic workspace and retained private keys/volume.
- This is one browser and one Pi. Other engines/phones, full accessibility,
  every route, real provider consent, SMTP, native harnesses, destructive recovery
  and production load are outside this slice. No commit, push, tag, production
  deployment or independent release sign-off occurred.

## Reproduce and inspect

Use [the bounded runner guide](../../../tests/robot/UX_TESTING.md) and
[runner](../../../scripts/test-robot-ux.py). Soft-memory mode requires applicable
operator approval and the supervisor PID for whole-stack observation.

Receipts, Robot HTML/XML, screenshots, timing samples and retained failures are
in `/mnt/data/keepsave-docs-robot-2026-10-25/`. The executed resource receipt is
`robot-run1/resource-results.json`; the task receipt is `verification.json`.
The Pi snapshot is `/home/santapong/keepsave-ux-20261025`. Private context/env
material is outside Git and report artifacts.

## Palette and sidebar follow-up — 25 October 2026

The owner’s screenshots identified retained lavender in selected UAT states and
an intrusive two-axis sidebar scrollbar. This follow-up supersedes the earlier
assumption that the sampled default-theme checks covered every interaction state.

| Work | Owner | Evidence | Local acceptance |
| --- | --- | --- | --- |
| Scoped palette and duplicate stage paints | Coordinator, independent `theme_all_pages` audit | Gold selected/hover/check/tool/scope styles; neutral masks; one CSS rule for large/mini planets | Verified |
| Sidebar intrinsic widths and scrollbar chrome | `developer_docs`, integrated by coordinator | Wrapped/truncated labels, no horizontal overflow, wheel/keyboard reach, forced-colors restoration | Verified |
| Build, resume and regression | Coordinator | Seven focused tests, lint, app build, static package; 13-case Robot replay | Verified |

The source audit found private landing variables and a separate inline UAT
gradient; changing root tokens had not reached them. The corrected public
package contains ten allowlisted files. Browser checks measured UAT selection
as `rgb(244, 191, 118)` and masked values as neutral `rgb(181, 178, 184)`.
The inspected mobile navigation measured 282-pixel client/scroll widths, with
vertical scrolling retained (383-pixel viewport, 584-pixel content).

The initial build completed tests/lint/app/static compilation but packaging
failed because the transferred test tree omitted `deploy/landing/output-config.json`.
Adding the existing public configuration completed packaging. Robot run R1 had
12 passes and one fixture failure: `Press Keys` received a literal `None` selector.
The pinned Browser 20 API requires `Keyboard Key press Tab` for global key input.
After that correction, the complete R2 journey passed **13/13**, including actual
Tab navigation to the lower Administration link and forced-colors scrollbar
restoration. Both original failures remain retained; neither was filtered away.

The synthetic Pi stack resumed with the same keys, volume, account and project;
it did not register or reseed. The existing browser session still fetched its
Storefront project and three masked Alpha records. Only the new startup
verification session was revoked. Backend source was unchanged.

New evidence is `/mnt/data/keepsave-palette-scroll-2026-10-25/`, including
`robot-palette-scroll-r1` and `robot-palette-scroll-r2`, source/landing manifests,
screenshots and the packaging-failure receipt. The R2 source identity covers
675 hash-verified files. Existing status/data-series colors, opt-in accent
customizations and unused historical assets were not flattened into one color.
This is a focused Chromium/default-theme correction, not every-browser or
every-route acceptance, and no production publication occurred.

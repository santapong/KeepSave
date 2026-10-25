# KeepSave — Gargantua landing storyboard

Design preview, 24 October 2026. Base source `809588f`; implementation branch
`feat/gargantua-landing-20261024`. The owner requested a new palette and the journey
**use cases → what we serve → features → user comments → FAQ → contact**, then
explicitly requested mock comments first. This is a local frontend preview,
not a hosted application launch, customer-endorsement claim or deployment.

## Storyboard

| Beat | Visitor question | Composition and response | Next action |
|---|---|---|---|
| Entry | What is this for? | KeepSave/Field Twist mark, original warm black-hole artwork, plain application-secret/team/tool definition. Static composition keeps the headline and controls readable. | Find your use case |
| 01 · Use cases | Is this for my work? | Three task-led entries: application keys, team access, bounded tool access. The continuation below the hero makes the page's depth visible. | Open the vault example, team features or tool preview |
| 02 · What we serve | What would I actually do? | One storefront example: store → scope → review. Separate Alpha/UAT/PROD values stay masked. Changing chapters preserves independent vault/access choices. | Choose an environment, move through tabs or inspect promotion details |
| 03 · Features | What helps me stay in control? | Four concrete capabilities: environments, scoped access/sessions, revision restoration, promotion/audit. A separately labeled controlled-tools preview shows request → authority check → approved provider operation. | Read implementation/status or integration documentation |
| 04 · User comments | What could this feel like? | Three role-based fictional quotes with persistent “Illustrative feedback · design mockup”, “Sample quote” and a plain disclaimer. No stars, company logos, customer counts or invented measured results. | Continue to questions; replace with approved quotes before publication |
| 05 · FAQ | What do I need to know first? | Six native disclosures explain secrets, promotion, self-hosting, history, harness qualification and hosted availability. | Open a question or go to contact |
| 06 · Contact | How can I get involved? | One clear invitation and working GitHub issue/setup links. No disconnected form, invented support address or false submission success. | Share a workflow in the GitHub composer or inspect self-hosting |

All chapter navigation is ordinary anchor navigation; there is no scroll capture
or essential camera animation. Reversing/returning scroll preserves in-memory
example state. Narrow screens stack the composition and show a dismissible menu.
Keyboard tab navigation, Escape/outside dismissal and native FAQ behavior remain.
The black-hole artwork is decorative concept art, with a static fallback.

## Reference board

Inspected 24 October 2026. Adapt patterns, not another product's claims or assets.

| Reference | Inspected surface and observation | KeepSave adaptation / boundary |
|---|---|---|
| [Doppler](https://www.doppler.com/) | Current homepage text: human/tool tasks precede platform capabilities and customer stories. Text inspection, not a full visual audit. | Make the intended work explicit; do not borrow its compliance, integrations, customer quotes or metrics. |
| [Railway](https://railway.com/) | Visible first viewport plus current DOM chapter sequence. A concise promise leads into distinct task/product chapters. The initial screenshot included its cookie banner. | Use chapter rhythm and concrete product explanation. Keep KeepSave's type, mark and secret-management story; no copied customers or cloud promises. |
| [NASA warped-world visualization](https://www.nasa.gov/universe/nasa-visualization-shows-a-black-holes-warped-world/) | Primary-source explanation of a thin accretion disk, lensed far-side light and dark shadow. Article text inspected; no simulator was reproduced. | Original amber/ivory light around a dark core. Gargantua is visual inspiration, not a film still, NASA asset, physics claim or security guarantee. |
| Existing KeepSave landing | Public desktop screenshot and source inspection. Repeated store/scope/review explanations, no dedicated comments/contact, older MCP execution wording. | Preserve identity and the functional example; replace repetition with the requested story and qualify controlled-tool access. |

## Palette and identity

| Role | Value | Use |
|---|---|---|
| Void | `#08090d` | Page/hero background |
| Quiet surface | `#111217` | Product and feature surfaces |
| Raised surface | `#19191f` | Feedback cards |
| Ivory | `#f5f1e8` | Main text |
| Secondary | `#b5b2b8` | Readable supporting text |
| Horizon gold | `#f4bf76` | Primary action, headings and focus |
| Field Twist violet | `#b8a3ef` | Existing identity and selected product states |
| Mint | `#8cd6b0` | Permitted example state |

The initial landing-only iteration scoped tokens to `.ks-landing`; application/
workspace themes were not repainted at that checkpoint. This boundary is
superseded by the application-wide follow-up below. The exact Field Twist SVGs and recognizable icon family are
retained. Amber gives the marketing scene its focal light; opaque surfaces carry
the product explanation. Important text and controls remain HTML.

## Implementation and availability

The static landing entry and public-asset/module allowlist are selectively
reconciled from the preserved landing checkout. Only explicit source/config files
were copied; no dirty files, operator configuration, credentials or deployment
project link were moved. `npm run build:landing` packages an independent static
output. It rejects application/API/auth-hook modules and unexpected assets.
Full application builds still use their existing entry. No provider or database
is needed for this mockup.

The static entry passes `landingOnly`, hides login and uses useful source/setup
links. The tool illustration is explicitly a preview; actual client/provider and
installation qualification remains pending. Hosted availability is answered in
FAQ/footer. The mock comments must be replaced or removed before publication.

The original generated PNG is retained in the image tool's local output; the
project consumes a versioned sibling WebP, not an overwritten existing asset.
[Prompt](asset-prompt.txt) and [asset identity](asset-manifest.json) retain exact
input and output identity. PNG-to-WebP conversion changes encoding only.

## Verification

Executed results, screenshots and the resource envelope are retained privately in
`/mnt/data/keepsave-gargantua-2026-10-24`. Local checks passed: **163 frontend tests in 34 files**, configured lint, application,
widget and isolated landing builds. Final application/landing builds passed again
after correcting tablet sign-in visibility. Local runtime was bundled Node24.19.0;
the exact CI pin24.21.0 was not rerun. The initial workstation Node26.8.1 run failed
storage-dependent fixtures; those failures are retained, not hidden by source changes.

Actual Chromium IAB review covered desktop1280×720, mobile390×844, narrow320×800
geometry and full-app900×800 sign-in. Section order, chapter keyboard/focus,
independent selections, mobile menu/Escape, FAQ and contact/anchor destinations
passed. No console warnings/errors were observed. Native missing-art behavior was
covered by the existing unit test; it was not separately blocked in the browser.
Ten public artifacts match the package manifest; login/register/API/MCP/draft
routes return404 from the isolated local host. Design-token contrast checks pass,
but do not establish whole-page accessibility.

Builds ran serially with inspected aggregate CPU/RAM/PID/time limits; the temporary
full-app check host expired and closed. One bounded public preview remains for
review. Owner acceptance, other browser engines, real phones, remote CI and
publication remain separate and unclaimed.

## Application-wide follow-up — 24 October 2026

The owner subsequently requested the palette and icon treatment on every page,
with a working application trial. Shared semantic tokens now use charcoal/gold
in dark mode and warm ivory/bronze in light mode. Authentication, workspaces,
portalled dialogs, the embedded widget and shared icons adopt the treatment.
Field Twist's exact vector paths remain intact; the palette changes its light
and dark presentation. Semantic success, warning and error states retain distinct
colors and matching foregrounds. Theme switching uses the existing preference
mechanism rather than navigation or form replacement.

Local evidence is retained privately in
`/mnt/data/keepsave-gargantua-app-2026-10-24`. The initial integrated checks passed
**164 frontend tests in 34 files**, lint and application/widget/landing builds.
Final contrast corrections passed 12 focused tests in two files, lint and the
application build. The widget build passed after its final style corrections.
The local runtimes are Node 24.19.0 and the current-source
Go 1.27.3 backend binary; backend source was not changed for this design work.

An actual Chromium IAB trial at 1280×900 and 390×844 used a fresh synthetic
PostgreSQL workspace. Login succeeded. Switching themes retained the new-secret
key/value; `THEME_TRIAL` was saved, revealed with its exact synthetic value and
shown in revision 1 history. Account, organization management, the templates
modal and Help were inspected. Mobile navigation worked, and the inspected
screens had no whole-document horizontal overflow. This sampled journey does
not establish browser coverage of every route. A separate native-size icon sheet
verified the dark/light marks and favicon at 16, 24 and 32 pixels, with all assets
loaded and the Field Twist silhouette discernible on charcoal and ivory.

Google/GitHub providers remain unavailable in this trial configuration; no real
provider consent was exercised. Remote CI, commit, deployment and production
readiness remain unclaimed. A short packaging timeout at a lower CPU quota and
initial local-demo bootstrap failures are retained in the private evidence; the
corrected, bounded runs succeeded. The live trial uses loopback ports, synthetic
data, retained private recovery material and a finite service lifetime.

# Event Horizon landing verification

Observed 2026-09-28, Asia/Bangkok. Branch `feat/landing-event-horizon-20260928`, isolated checkout `/mnt/data/company/apps/KeepSave-landing`, base `9e2b3ba`. The owner selected Event Horizon and requested a clearer product story. Implementation is local and uncommitted; no integration, publishing or deployment occurred.

## Result

KeepSave's existing mark, Geist typography and amber/violet identity are retained. The public landing explains the product with one illustrative storefront project: store keys, scope a tool's access, review an environment promotion. Environment choices, chapter tabs, integration examples, FAQ disclosures and mobile navigation work locally. Sample values stay masked; the walkthrough creates no keys, approvals or promotions. Existing authentication and authenticated routes are unchanged.

Compared the exact selected image with a rendered 1065×1477 viewport. The image's composition, typography hierarchy, primary action, amber horizon and dark product surface carry through. Intentional differences: a short product introduction precedes the example; working chapters replace decorative controls; more explanatory sections, integrations and FAQs extend beyond the reference's short page. Art is a generated static image, with an existing CSS EventHorizon fallback, rather than a WebGL scene. The original auth/workspace scene remains intact. This is visual review, not pixel equivalence or owner acceptance.

## Checks performed

| Check | Observed result |
|---|---|
| Frontend suite | 13 files, 91 tests passed, including four new behavioral tests for state preservation, keyboard tabs, menu dismissal and image failure |
| Lint | Passed |
| App and embed-widget builds | Passed; final app build repeated after artwork framing adjustment |
| Diff whitespace and scope | Passed; no backend, dependency, route or shared theme edits; canonical checkout still clean |
| Reflow | DOM bounds checked at 320, 390, 768, 1024, 1440 and 1920px; no horizontal overflow in content/controls; images loaded |
| Mobile visual review | 390px hero and expanded review panel; 320px hero refined to preserve the three-line headline |
| Walkthrough | Store PROD selection survives chapter changes; Scope defaults to UAT independently, and preserves Alpha selection; Review explains two selected updates and one unchanged key |
| Keyboard | Home/Arrow keys move tab focus and selection together; visible amber focus; Escape closes the mobile menu and returns focus to its button |
| Other controls | Integration choices replace explanation/flow/docs destination; production FAQ and review details expand; mobile menu opens/closes |
| Auth destinations | Sign-in loads `/login`; Create your vault loads `/register`; forms inspected without entering credentials or creating accounts |
| Links | Documentation, SDK, integration and CLI paths reconciled with the repository; fixed CLI destination to `backend/cmd/keepsave` |
| Art failure | Temporarily removed the owned generated asset; CSS fallback, heading and primary action remained visible; restored image and verified successful reload |
| No JavaScript | A local sandboxed iframe with scripts blocked rendered the actual `noscript` story and documentation link; no global browser security preference changed |
| Motion | Landing mounts no canvas and zero elements declare animations; scoped reduced-motion rule removes transitions/animations. OS-level reduced-motion emulation was not exercised |
| Browser logs | No error/warning entries during normal landing/auth checks before deliberate failure fixtures |

## Evidence and limits

Local evidence directory: `/mnt/data/keepsave-landing-validation-2026-09-28/`. Retained `tests.log`, `lint.log`, `builds.log`, `build-final.log`, `responsive.json`, final desktop/mobile screenshots, `reference-size.png`, `image-fallback.png` and `nojs.png`. The initial `desktop-full.png` predates final copy/framing polish. Responsive measurements precede the final 320px type and 851–1100px artwork refinements; these affected viewports were inspected again afterward. `viewport-320-final.png` is the final narrow hero.

All browser checks used visible Brave/Chromium on this desktop. Firefox, Safari, real phones, screen-reader use, backend end-to-end operations and CI were not tested in this presentation-only change. Build output reports a large Three.js chunk used elsewhere in the existing application. No performance benchmark or conversion claim is made.

Browser-capture limitation: a full-page screenshot at 320px timed out and left the temporary viewport expanded. Reset the viewport, verify actual `innerWidth`/`innerHeight`, and take viewport captures instead. After a viewport change, verify the settled layout before capturing; one immediate reference-size capture still showed the previous mobile frame and was replaced. The final browser viewport was reset to its original size.

## Preview

Local preview: <http://127.0.0.1:4651/>. To restart, run `npm ci`, then `npm run dev -- --host 127.0.0.1 --port 4651 --strictPort` from the isolated checkout's `frontend` directory. This preview serves the frontend; authentication submissions need a configured backend. No account creation is necessary to review the landing.

Standard PR review remains a future integration gate. Local implementation/verification does not imply remote approval, merge or release.

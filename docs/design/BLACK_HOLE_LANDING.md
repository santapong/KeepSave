# KeepSave — Event Horizon experience specification

Date: 2026-09-09. Scope: landing, authentication, and the complete frontend theme. User direction: moving black hole using Three.js and Anime.js; extend the black-hole design to all pages. This revision supersedes the earlier landing-only, static-CSS requirement. Implementation is local on `feature/realistic-product-and-efficiency`; not released.

## Intent and decision

An expressive black hole introduces the product. The workspace carries the same orbital identity through its logo, violet surfaces, typography, and lavender controls, while keeping data screens calm and readable. Keep environment and error colors meaningful; the theme must never obscure warnings or secret visibility.

Type-2 frontend/dependency change. No authentication, cryptography, backend, promotion policy, or trust boundary changes in this pass. Renderers receive only animation time and dimensions; never vault data. Dependencies are installed and bundled locally, with no CDN, texture requests, telemetry, or external rendering service. Standard review is still required before release. Rollback: revert this frontend/design pass on the feature branch while preserving the preceding persistence work.

## Visual system

| Token | Dark / default | Light variant |
| --- | --- | --- |
| Space | `#090812` | `#f7f4fc` |
| Surface | `#131120` | `#ffffff` |
| Border | `#342d47` | `#d9cfe7` |
| Primary text | `#f4f0ff` | `#241c35` |
| Secondary text | `#b9b0cd` | `#675978` |
| Action | `#c7b7ff` | `#62419c` |
| Action label | `#171124` | `#ffffff` |
| Illustration | Amber `#ffcc92`, violet, near-black center | Same cosmic artwork |

Geist is the UI/display face; Geist Mono is used for code and compact labels. This preserves the upstream typography introduced before integration; the earlier local preview used Space Grotesk / JetBrains Mono. Cards use 14–20px corners, quiet borders, and restrained shadows. Forms, menus, dialogs, toasts, tables, and charts inherit shared semantic tokens. Primary actions use lavender; success, warning, danger, and Alpha/UAT/PROD keep their semantic distinctions. Light mode is retained as a violet daylight variant. Landing remains the dark brand presentation.

## Composition and route coverage

- **Landing:** orbital wordmark; “Your secrets. In the right orbit.”; animated black hole; separate functional and explicitly labeled product example; clear workflow, integrations, documentation, and registration links. Values remain masked.
- **Login and registration:** shared live black hole above introductory text on desktop; compact scene above the form on mobile. Bordered form card, clear field labels, password-manager autocomplete, error announcements, and a link back to KeepSave. Preserve working authentication.
- **Projects and project detail:** violet shell, current-page navigation, readable project directory, consistent action hierarchy, static black-hole empty states, and clear environment/tab controls. Secrets, pipeline, history, audit, and API-key panels inherit the same tokens.
- **Organizations and organization management:** shared shell and cards; card grids shrink to available mobile width rather than retaining a fixed minimum.
- **Templates:** same card language and responsive grids for both populated and loading states.
- **MCP Hub and OAuth clients:** consistent cards, labels, tabs, dialogs, and actions.
- **Applications and settings:** shared page headers, data tables, and form controls. Preserve one-time-key and revocation behavior.
- **AI Intelligence and admin:** themed dashboards, charts, tabs, and error states. Authorization remains enforced; do not fabricate chart data for unauthorized accounts.
- **Docs:** matching typography and surfaces; a section selector replaces the fixed sidebar on narrow screens so the article remains readable.
- **Shared UX:** skip-to-workspace link, visible focus, current-page semantics, touch-friendly forms/actions, and persisted motion preference. Small navigation marks and workspace empty states never create WebGL renderers.

## Motion and rendering contract

`BlackHoleScene.tsx` uses Three.js 0.186.0 and Anime.js 4.5.0. One orthographic quad samples precomputed Schwarzschild photon paths to find intersections with a thin disk. A dark capture shadow, direct light plane, upper/lower lensed images, and narrow higher-order rings follow from those paths. Thermal colors, density texture, optical halo, and exposure remain art-directed. This is a bounded non-rotating model, not a complete astrophysical simulation.

Anime.js drives disk flow over a seamless 120-second cycle. The observer and disk inclination stay fixed; the horizon does not wobble. Its per-animation update rate is capped at 30 fps. There is no second requestAnimationFrame loop and no React state update per frame. Three.js and Anime.js are dynamically imported only when a public scene becomes visible and motion is enabled.

- Pause off-screen with IntersectionObserver and when the document is hidden.
- Persist the explicit Pause/Resume preference. Respect `prefers-reduced-motion` and `data-motion="off"`; system reduced motion cannot be overridden with the button.
- Pausing explicitly switches to the static companion artwork and releases the canvas. Reduced-motion startup creates no renderer.
- Cap pixel ratio at 1.5 and logical drawing size at 900×700 before pixel ratio. Request a low-power context; disable antialiasing. One generated 1024×512 red-float lookup texture (2 MiB GPU data, backed by one cached 2 MiB CPU table). No downloaded textures, postprocessing, shadow passes, bloom passes, or per-frame raymarch loops.
- Dispose animations, observers, lookup texture, geometry, material, renderer, context, and listeners on unmount. A lost/unavailable WebGL context shows a static fallback and does not repeatedly retry.
- Canvas/artwork is decorative and hidden from assistive technology. Pause remains a real keyboard-accessible button outside that hidden subtree.

API references: [Three.js ShaderMaterial](https://threejs.org/docs/pages/ShaderMaterial.html), [WebGLRenderer](https://threejs.org/docs/pages/WebGLRenderer.html), [Anime.js frame rate](https://animejs.com/documentation/animation/animation-playback-settings/framerate/), [Anime.js cleanup](https://animejs.com/documentation/scope/scope-methods/revert/).

## Acceptance and observed verification

Local preview: `http://127.0.0.1:13000/`. Browser checks use a disposable `example.test` account, not real vault data.

- Observed actual WebGL canvas and playing state on desktop; visibly changing disk bands. Pause removes the canvas, Resume restores it; following the workflow anchor pauses the off-screen renderer.
- Desktop landing/login/project directory/project detail and mobile login/registration/workspace visually inspected. Docs fixed after visual inspection exposed its fixed sidebar compressing the article at 320px; checking document overflow alone did not catch that usability defect.
- Live top-level route checks: projects, organizations, templates, MCP Hub, OAuth clients, applications, application settings, AI Intelligence, admin, and Docs. Shared dark background resolves to `#090812`; workspace routes create no canvas. Admin correctly shows its forbidden state for the disposable non-admin account; populated privileged dashboards were not verified.
- Project creation and project-detail navigation worked with disposable metadata. No new secrets were needed for this theme review. This is UI verification, not a new backend persistence audit.
- Lifecycle regression tests cover lazy visibility initialization, 30 fps configuration, off-screen pause, GPU disposal, reduced motion, persistent pause, unavailable WebGL, and context loss. These are mocked resource-lifecycle tests; actual browser WebGL was separately observed.
- Final checks: 82 frontend tests passed; ESLint, TypeScript, production app build, widget build, and `git diff --check` passed. Build emits the expected large, deferred Three.js chunk warning.
- At 320px, organizations, templates, MCP Hub, OAuth clients, applications, application settings, and AI screens have neither document nor main-content horizontal overflow. MCP tabs now scroll inside their control; long API paths wrap. Docs article is 278px wide at a 320px viewport, and dark/light screenshots are readable. Desktop was inspected at 1440×1000, mobile at 390×844 and 320×844.
- Disposable project was created, opened, then deleted through the UI; test account was signed out. The empty local test account remains in the disposable preview database. No production or user-owned data was changed.
- Logs: `/mnt/data/keepsave-review-2026-09-09/motion/`. Browser observations were inspected live; OS reduced-motion and document-hidden behavior were not manually exercised. Reduced-motion behavior is covered by mocked media-preference tests.

## Resource limits

The previous static landing had no animation cost. This requested effect adds GPU/CPU work while visible; it is not claimed to be cheaper than the static design. The production Three.js chunk is approximately 747 kB / 192 kB gzip and loaded on demand; Anime.js is also deferred. No sustained CPU/RAM/GPU benchmark was run for this visual pass. Workspace screens retain static artwork and do not initialize either animation dependency through a scene.


## 2026-09-09 — physical light-plane refinement

User request: make the event horizon and light plane more physically real. Baseline was the first procedural moving-ring shader, whose arcs were drawn by formulas rather than derived from light trajectories. This pass changes only the shared black-hole renderer, static companion artwork, tests, and this specification. Other frontend and backend changes are preserved.

### Evidence and adaptation

[Bruneton, *Real-time High-Quality Rendering of Non-Rotating Black Holes*](https://ebruneton.github.io/black_hole_shader/paper.pdf), May 2020 technical report, inspected §§3.1–3.4 and implementation/evaluation §§4–5: precompute null-geodesic quantities, then locate thin-disk intersections with texture lookups. The paper includes a specific beam-filtering method, retarded time, stellar catalogs, spectral color tables, and GPU measurements. KeepSave independently implements the simpler fixed-observer path-table idea; it does not reproduce that renderer, its precision bound, or its performance results.

[NASA's black-hole visualization](https://www.nasa.gov/universe/nasa-visualization-shows-a-black-holes-warped-world/) provides the visual reference for a thin emitting disk, bent images of its far side, and Doppler brightness asymmetry. The visible shadow is not the event-horizon surface: capture and lensing make it larger. This distinction is also supported by the [Oregon State null-geodesic treatment](https://sites.science.oregonstate.edu/physics/coursewikis/GGR/book/ggr/onull).

### Implemented model

- Geometric units `G = c = M = 1`. Event-horizon radius `2M`, photon-sphere radius `3M`, critical impact parameter `sqrt(27) M`.
- Integrate `u'' = 3u² − u`, where `u = 1/r`, starting at infinity with `u = 0` and `u' = 1/b`. RK4 uses two substeps per angular table sample. Captured and escaped trajectories stop and stay marked.
- 1024 impact samples and 512 angular samples through `3π`. Nonlinear impact sampling concentrates resolution near the critical curve. The shader uses manual bilinear interpolation, avoiding a float-linear-filter extension dependency.
- A stationary observer sees a disk inclined about 82° from its normal. The disk spans 6–18M; the inner edge is outside the Schwarzschild ISCO. Up to three ordered disk-plane crossings contribute, with foreground opacity hiding farther emission.
- Circular-orbit frequency shift supplies gravitational/transverse Doppler and approaching/receding asymmetry. A temperature-shaped amber palette and procedural flow provide the visible light; this is not spectral calibration or a fluid simulation.
- Flow remains on the fixed plane. Static CSS fallback now includes upper/lower lensing arcs and a thin foreground band; it approximates the silhouette and does not solve geodesics.

### Validation and limits

The integrator is tested against independent expectations: capture below versus escape above `sqrt(27)`, the limiting photon orbit at `r = 3M`, the weak-field `4M/b` deflection, and angular-step refinement. These checks verify numerical behavior at the sampled cases; they do not validate every interpolated pixel near the critical curve. The resource tests also verify disposal of the GPU lookup texture.

A single local Node measurement built the nonlinear table in **31.75 ms**, with a **0.041 ms** cached call and **2,097,152 bytes** of retained table data. This is a host-side initialization observation, not a mobile/browser benchmark or a sustained GPU/CPU measurement. Browser rendering retains the 30 fps cap, size limits, off-screen pause, reduced motion, and full unmount disposal. Visible desktop and 390px mobile checks showed the new light plane and lensed arcs; Pause/Resume switched between fallback and live WebGL without overflow.

Finite lookup resolution and three-crossing truncation limit very narrow higher-order rings. Omitted: black-hole spin/Kerr frame dragging, time-of-flight delays, full beam filtering, lensed star catalogs, magnetohydrodynamics, and quantitative spectral emission. These are outside this landing-page requirement. Reject a per-pixel numerical ray marcher here because it would repeat the integration every frame. No published scientific-accuracy or performance claim is made for this implementation.

Final verification for this refinement: **87 tests passed**, including five photon-path checks and lookup-disposal coverage; TypeScript, app/widget builds, ESLint, and diff whitespace validation passed. Actual WebGL checked in the visible in-app browser at 1440×1000 and 390×844. Logs and the scoped initialization measurement are in `/mnt/data/keepsave-review-2026-09-09/physical/`. Changes remain local; no commit, push, or deployment.


## 2026-09-09 — icon and identity completion

The canonical icon is `frontend/public/keepsave.svg`: a 1,351-byte standalone SVG with a dark rounded tile, amber lensed arch/light plane, black center, and a violet lower arc. It is a legible brand abstraction of the physical scene. Keep the glyph static, and preserve the aperture and light plane at small sizes.

`EhMark` displays that asset, and `Brand` supplies a consistent **KeepSave** wordmark and accessible home link. Landing header/footer, product example, login, registration, and workspace sidebar now share the same icon. Browser favicon is versioned to refresh the old cached green lock; browser theme color is `#090812` and application name is KeepSave. No new dependency or animation loop was added.

Login and registration use a clear brand/header divider, a subtle amber/violet top accent, and a small vault badge. Their eyebrow now identifies Event Horizon without a status-looking green dot. Keep small-screen account links together so arrows do not wrap alone.

Verification: live desktop login at1440px; login, registration, and landing at320px; icons loaded, favicon/theme metadata correct, home navigation worked, and no horizontal overflow. 87 tests, lint, app/widget builds and diff whitespace check passed. Logs: `/mnt/data/keepsave-review-2026-09-09/brand/`. All changes are local.


## Develop integration — 2026-09-09

Fetched remote `develop` at `5a8af2d`, which contained a parallel landing/auth redesign and releases through v1.3.0. The integration uses the locally reviewed landing, auth forms, shared brand, and bounded `BlackHoleScene` as the active UI. Upstream README, integration hub, architecture diagrams, branch rules, release history, version 1.3.0, Geist typography, and compatible component styling are retained. Upstream `Singularity`, `CometField`, `KsMark`, and entrance-hook source remain available, but are not mounted by these routes; they must not be described as active effects in this design.

The merge was assembled in a separate checkout to keep the user's signed-in local preview available. All 87 frontend tests, ESLint, app/widget builds, full backend race/shuffle tests, and vet passed. Fresh SQLite and disposable PostgreSQL lifecycle checks passed save/update/import/restart/read-back, isolation, audit, and deletion assertions. Vitest was patched to 4.1.11; npm audit reported zero vulnerabilities. Visible merged-preview landing/login checks at 1280px showed the WebGL scene and usable forms; the landing had no horizontal document overflow, and Pause removed the canvas. Earlier mobile acceptance above belongs to the pre-integration visual pass and was not repeated during this merge.

Evidence: `/mnt/data/keepsave-review-2026-09-09/develop/`. These are local checks. The existing GitHub workflow runs for `main` pushes/PRs, so a `develop` push alone does not establish remote CI success. Earlier “local/uncommitted” statements above describe their individual historical passes. No production acceptance is implied.

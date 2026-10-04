# Changelog

All notable changes to KeepSave will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Changed — documentation, branding and workspace organization, 2026-10-04

- Refreshed README, current system/operator/security/client guides and roadmap
  around the harness-neutral source candidate; distinguish implemented, locally
  tested, externally qualified and released behavior.
- Added documentation/status/branding/folder indexes and a complete Markdown
  inventory. Preserved dated research, audits, phase summaries and acceptance
  receipts; moved earlier root notes into `docs/archive/`.
- Rebuilt ten architecture/workflow SVGs using the selected Field Twist geometry,
  and added a self-contained interactive architecture map with typed source and
  artifact-bound visual checks. README uses the matching SVG header.
- Kept `/mnt/data/company/apps/KeepSave` as the canonical local project and
  organized supporting worktrees outside the product-app folder, preserving
  the dirty landing checkout.

### Fixed — integration verification wiring, 2026-10-04

- Invoke the PostgreSQL fixture with Bash so Git's non-executable script mode
  cannot stop CI before its tests run.
- Collect the independent frontend scan and both image SBOMs after a backend
  scan failure; HIGH/CRITICAL image findings remain blocking.

### Availability — 2026-10-04 reconciliation

- October3 main/develop CI both failed; no green remote acceptance is claimed.
  The backend image scan still needs remediation. Formal reviews and real
  providers/native clients/runner/recovery/operational qualification remain gates.
- Contact/recovery proofs have 15-minute expiry; invitations currently have a
  separate 24-hour expiry. This corrects broader earlier documentation without
  changing authentication behavior.
- This update targets `develop`; it creates no release tag, capability activation
  or application deployment. Version remains the unreleased `1.4.0-rc.1` candidate.

### Added — local harness-neutral platform candidate, 2026-10-02

- Shared authority barriers and membership epochs; parent/client-bound OAuth
  delegations and runs; safe denial projection and current-authority result reads.
- Login-method inventory/removal safety, contact verification, account recovery,
  invitations and revision-bound scoped offboarding with durable receipts.
  SMTP proofs are Vault-encrypted, hash-only at verification, and carried by
  identifier-only jobs. Uncertain send attempts are not silently replayed.
- Separate credential lifecycle metadata, declared renewal/expiry and responsible
  members; deduplicated 30/7/1-day in-app reminders with current recipient checks.
- Version2 encrypted recovery bundles retain lifecycle metadata, accept legacy
  version1 bundles and require explicit responsible-member remapping. Recovery
  excludes sessions, tokens, grants, approvals and ephemeral operation results.
- Safe cursor audit browsing and bounded repeatable-read export snapshots, trusted
  publication jobs, expiry and current-authority downloads; read-only doctor.
- Official Go MCP SDK v1.8.0, stateless Streamable HTTP, the two selected protocol
  lanes and a new opaque-token/S256 public-client OAuth service. Consent and
  revocation controls remain separate from browser sessions and provider tokens.
- Immutable instruction-only skills, portable profiles, independently approved
  packages and candidate native exporters for Codex0.153.3/Hermes0.21.5.
- GitHub App custody, explicit repository/environment bindings, preparing and
  commit-bound runs, durable idempotent operations, budgets, attempt tickets,
  encrypted short-lived results, cancellation and safe execution receipts.
- Separate rootless Podman supervisor/connector reference and direct TLS1.3/mTLS
  runner listener. The public application proxy refuses runner routes.
- Account/team, lifecycle/audit and developer-access screens preserve the accepted
  KeepSave design; generated types and actual-router management contracts expand.

### Fixed — current authority and publication

- Authority narrowing participates in shared stored-subject/project/session
  barriers across session, lease, project, template and vault paths. Offboarding
  previews capture one consistent snapshot and stale changes require a new review.
- Project metadata is captured inside admitted transactions. A later authority
  change cannot cause a second unguarded database read to expose newer metadata.
- Failed audit publication terminates safely rather than remaining pending;
  required-audit failures roll back local state. Account changes reject late
  frontend responses; malformed/proof query logging is redacted.
- Account recovery atomically expires retained legacy API keys, revokes their
  leases and denylists recorded agent issuance with password/proof/session
  changes. Late reset completion preserves a newer unrelated browser account.
- Refresh the distroless runtime digest and pin the fixed PCRE2 10.49 Alpine
  package in the frontend image after a check-date vulnerability scan. Runtime
  users remain nonroot; scan receipts retain the unused OpenPGP module advisory.

### Documentation — operator preparation, 2026-10-03

- Add the operator preparation checklist for the application domain/host, Vault
  Transit and recovery material, Google/GitHub OAuth apps, authenticated SMTP,
  separate GitHub App, isolated runner, client qualification and release reviews.
- Record owner-directed source publication separately from production acceptance;
  retain dated validation receipts and outstanding gates.

### Availability and validation

- These additions extend base3878e69 as an unreleased source candidate. The owner
  authorized `main` source publication on2026-10-03; this does not supply formal
  review signatures, operational acceptance, a release tag or deployment.
  New flags default off. See the
  [current acceptance ledger](docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).
- Synthetic PostgreSQL/router/OAuth/broker/runner protocol tests establish only
  their executed scenarios. Live Google/GitHub/SMTP and exact native client UAT,
  rootless host isolation, operational drills and independent Security/Tech Lead
  signatures remain required. CPU delegation is missing on the current host;
  runner preflight fails closed. No production capability is inferred from source.
- The older research plan remains historical context. External secret-delivery
  adapters, script execution, model credentials, enterprise SSO and marketplaces
  stay deferred. SQLite/MySQL compatibility does not establish platform parity.

## [1.4.0-rc.1] - 2026-10-02 (integrated source candidate; untagged)

This candidate adds revocable identity and a reliable PostgreSQL vault. It is
not a production release. Sponsor-directed develop integration is recorded;
independent reviews, remote CI, staging rollback
acceptance and real provider consent remain pending; see the
[candidate notes](docs/releases/v1.4.0-rc.1.md).

### Compatibility and upgrade requirements

- Existing human tokens require reauthentication. Drain older APIs before the
  tracked-session cutover; binaries without session enforcement cannot serve
  traffic or be used for rollback afterward.
- Enroll existing PostgreSQL vault records as a labeled baseline after older
  writers are drained. Retain journal/key dependencies and use only a compatible
  reviewed rollback binary. New history/recovery guarantees are PostgreSQL-only.
- The restricted core profile refuses unfinished capabilities explicitly;
  preserved source is not a support or execution claim.
- Align backend health version, frontend package metadata and the maintained
  core OpenAPI version to `1.4.0-rc.1`. SDK package versions remain independent;
  the Go module is available locally, with remote publication still pending.

### Added — core identity and reliable vault candidate

- Google and GitHub sign-in configuration, verified provider identities,
  one-time state/S256 PKCE and explicit recent-session account linking. Real
  provider registration and UAT remain operator acceptance gates.
- Database-backed human sessions with metadata listing, logout and individual
  revocation; open registration grants no global administrator authority.
  Trusted-host operator commands bind administrator grants to stored user IDs.
- Transactional workspace creation, administrator membership, required audit
  and PostgreSQL outbox; caller-scoped `Idempotency-Key` retries and distinct
  UUID-suffixed slugs. Personal projects remain supported.
- PostgreSQL versioned vault adapters for core writes, imports, templates,
  promotions, rollback and key rotation; metadata history, explicit version
  reads and restoration that appends a new revision with a stale-write guard.
- Encrypted backup verification, selected live restoration and an explicit
  isolated-target recovery command. Recovery never recreates source authority.
- A trusted local worker and backup metadata catalog. Daily scheduling is
  disabled until an operator records isolated recovery acceptance. Private
  storage, deterministic jobs, catalog verification and visible retention
  failures preserve retry and key dependencies.
- A standalone local Go SDK module and dedicated test job; published module
  distribution remains a separate release step.
- One embedded core OpenAPI source, actual-router response contract tests and
  generated frontend wire types with CI drift checking.

### Fixed — authority and consistency

- Lease parent identity, project/environment/key/expiry narrowing and sibling
  API-key denial; token issuance and revocation are persisted and audited.
  Lease and agent-token mutations now join required audit and PostgreSQL outbox
  transactions, with current session/parent/membership checks and rollback tests.
- MCP private-server visibility, installer ownership and project binding.
- Workspace owner demotion/removal, arbitrary project reassignment and
  nonempty organization deletion. First personal-project attachment revokes
  previous delegated credentials; cross-organization transfers remain refused.
  Assigned projects follow current workspace membership even for the original
  owner: demotion narrows credential access and removal denies subsequent calls.
- Project/secret deletion retains encrypted tombstones, historical key
  dependencies and stable audited IDs. Required audit failure rolls back core
  mutations instead of recording misleading success.
- Viewer project metadata uses current stored authority. Core history/recovery
  refuses unsupported non-PostgreSQL execution, and authorized ciphertext
  corruption surfaces a safe server error rather than a misleading 404.
- Private workspace template reads/listing/application use current membership;
  creator provenance cannot bypass removal. Metadata changes require current
  workspace admin or personal creator, a tracked session, and transactional
  audit/outbox. Core global publication is refused; builtins remain available.
  Template defaults remain ordinary configuration, not encrypted vault records.
- Persisted serialized audit-chain head and PostgreSQL job leases/fencing;
  uncertain external effects are not blindly replayed.

### Changed — restricted release and validation

- Upgrade `golang.org/x/crypto` to v0.56.0, `x/net` to v0.57.0, `x/text` to
  v0.41.0 and `x/sys` to v0.47.0. Pin govulncheck v1.8.0 for Go 1.27 support;
  the final scan found no called/imported-package vulnerabilities. The unimported
  `x/crypto/openpgp` module advisory remains disclosed, not presented as fixed.
- The running application uses a restricted core profile. Legacy OAuth
  issuance, experimental intelligence, policy metadata, enterprise SSO,
  webhook automation, event replay and plugin execution return explicit
  capability-unavailable responses. API-host MCP builds/execution stay off.
- The bounded MySQL 8.4 legacy identity/session/scoped CRUD/batch and workspace
  role exercise passed; PostgreSQL-only history/recovery still returns explicit
  capability refusal on unsupported dialects.
- Replace the frontend nginx runtime with the digest-pinned
  `nginx-unprivileged:1.30.5-alpine-slim` after the old base scan exposed
  40 high/two critical findings; the replacement scan reported zero findings.
- CI and container builds pin Go 1.27.1 and Node 24.21.0; the module
  minimum is Go 1.26.0 for current dependency compatibility. The PostgreSQL harness
  creates unique synthetic Docker resources, waits for TCP readiness, bounds
  build parallelism, ignores operator DSNs and cleans up only its own resources.
  Existing integration/release checks and `develop` pull-request triggers remain.
- Container CI pins Trivy 0.74.0 and reviewed immutable scan/setup actions,
  disables binary caching and verifies the published archive checksum before
  extraction. Nested action metadata and YAML/shell checks are recorded; remote
  GitHub Actions execution is still pending.
- Additive migrations 015–025 preserve compatibility. PostgreSQL startup
  requires coordinated baseline enrollment after older writers are drained;
  rollback must respect the new journal/key format and retained references.
- Architecture, threat and system references are rebased to the current core
  candidate and preserve the explicit future broker/runner enforcement boundary.
- README and the capability acceptance ledger separate implemented local
  behavior, compatibility source, unavailable features and the approved future
  MCP/broker/runner/skills program. This is unreleased; independent security
  review, real social-provider UAT and production deployment are not claimed.
- A separate same-origin control-host deployment reference with TLS, private
  PostgreSQL, API, frontend and worker. Its structure is validated; it has not
  been deployed and does not establish runner isolation or M5 acceptance.

### Changed — documentation

- **`README.md` is about KeepSave only.** 637 lines down to ~300. The
  308-line "Integrations" section — nearly half the file, and entirely
  about NEXUS and MedQCNN — was fully duplicated by
  `docs/nexus_integration.md` and `docs/medqcnn_integration.md`, so it was
  removed rather than moved. The two overlapping documentation sections
  are merged into one, and the 97-line API dump is replaced by an
  endpoint map pointing at the full reference.
- **`docs/INTEGRATIONS.md`** is new: the single hub for everything that
  plugs in. First-party (SDKs, GitHub Actions, GitLab CI, Terraform, the
  embed widget, MCP clients) and partner products (NEXUS, MedQCNN,
  Grovernance, Seidr), plus the five-step shape every integration follows.

### Fixed — documentation accuracy

- **Historical finding, 2026-08-11:** the Go SDK was documented as `go get`-able
  while `sdks/go/` had no module. The 2026-10-02 local candidate now includes its
  standalone `go.mod` and test job. Publishing/tagging and remote module
  resolution remain separate release steps; this preserves the earlier finding
  without claiming it is still the current source state.

---

## [1.3.0] - 2026-08-11

**Physically-modelled cosmic layer, and documentation that matches the
product.** Frontend and docs only; no API, crypto, auth or
promotion-engine changes.

### Fixed — the comets were orbiting nothing

- On the auth screens the comets orbit the scene origin of `CometField`,
  a full-viewport canvas, which projects to the centre of the *screen*.
  The black hole was rendered inside `.cz-login-aside-bg`, the centre of
  the *left pane*. Two canvases, two different centres — so the comets
  were orbiting a point where nothing was drawn. The hole now lives in a
  viewport-centred `.cz-cosmos-hole` layer, and the comets visibly sweep
  around it.
- Perihelion distances were raised so every orbit clears the shadow's
  apparent radius. Two comets previously passed inside it, which looked
  like diving through the black hole.

### Changed — documentation theme

- The ten C4 / 4+1 SVG diagrams are re-themed to the frontend's own Event
  Horizon tokens: deep-space void ground, periwinkle-violet accent,
  aurora teal for healthy state, Geist and Geist Mono. Documentation and
  product now read as one thing. Values stay hard-coded hex rather than
  `oklch()` or `prefers-color-scheme`, because GitHub sanitises SVG and
  honours neither reliably inside `<img>`.
- `README.md` gains status badges and surfaces the architecture view set
  in its documentation table.

### Changed — auth screens: real comets, properly modelled

- `CometField` now runs **four** comets instead of thirty, each modelled
  rather than drawn.
  - **Orbits from orbital elements.** Each nucleus is defined by the six
    Keplerian elements an astronomer would quote (a, e, i, Ω, ω, M₀), not
    a hand-picked velocity. Converting them to a state vector means
    solving Kepler's equation `M = E − e·sin E`, which has no closed form
    — done here by Newton–Raphson, which converges even at the high
    eccentricities real comets have (0.55–0.86 here; Halley is 0.967).
  - **Tails from Finson–Probstein dust dynamics.** A dust grain feels
    radiation pressure outward against gravity inward; the ratio is β, set
    by grain size, so the grain moves under *reduced* gravity
    `a = −GM(1−β)r̂/r²` — its own Keplerian orbit around the same mass.
    Integrating a population with a spread of β, released continuously
    along the nucleus's path, *is* the standard model of cometary dust.
    The curved dust tail and the straight ion tail are no longer two
    drawn shapes: they are one mechanism at two ends of the β range.
  - Emission scales as 1/r², so a comet is bare far out and blooms
    through perihelion.

### Added — quasar jets on the landing hero

- The landing black hole now launches a pair of collimated relativistic
  jets along its spin axis, which is what makes an accreting black hole a
  quasar. Emission is beamed by the Doppler factor
  `δ = 1/(Γ(1 − β·cos θ))` and scales as δ³, computed per fragment — so
  the brightness ratio between the approaching and receding jet is real,
  and the bright jet swaps sides as the camera orbits. The cone is drawn
  double-sided so the silhouette accumulates more emission than the
  centre, reproducing the limb-brightened hollow sheath real jets have.
- Note the geometry is honest about itself: viewed near the equatorial
  plane, as here, both jets are close to transverse and neither gets much
  boost. The spectacular one-sided jet needs a line of sight near the
  axis — that is a blazar, not this.

---

## [1.2.1] - 2026-08-10

**WebGL consistency and performance.** The auth screens get the same
ray-traced black hole as the landing page, and the geodesic march is made
affordable on integrated graphics. Frontend only; no API, crypto, auth or
promotion-engine changes.

### Changed

- The login and register screens now use the ray-traced `Singularity`
  instead of the CSS `EventHorizon`, so every black hole in the product is
  the same physically-derived object. `EventHorizon` survives as the
  no-WebGL fallback and in small empty-state slots.

### Fixed — WebGL performance

- The geodesic march was far too expensive on integrated graphics: **15
  fps** on the landing hero and **9 fps** on the auth screens, which run a
  second WebGL context. Both now hold **~60 fps** on Intel UHD. Three
  changes, in order of impact:
  - an impact-parameter early-out — a ray passing wider than the disk, or
    already heading away from the mass, can neither be lensed into it nor
    hit the shadow, so it skips the march and samples the sky directly.
    Most of the frame is empty sky, so this is the bulk of the saving;
  - march steps 150 -> 110, and default resolution scale 0.7 -> 0.55;
  - both WebGL loops capped at 30fps. The disk turns slowly enough that 30
    and 60 are indistinguishable, and it halves GPU cost.
- `Singularity` no longer drives a renderer when its host has zero size —
  the auth aside is `display:none` below 1100px, and it was marching a 0x0
  buffer there.

---

## [1.2.0] - 2026-08-10

**Public landing page and the Event Horizon design refresh**, on top of
the project governance and 30-day plan execution. No API, crypto, auth
or promotion-engine changes — this release is frontend and docs only.

### Added — public landing page

- **`frontend/src/pages/LandingPage.tsx`** - the public front door at
  `/`. Sections: hero, an interactive environment-promotion diff panel,
  an MCP-gateway / OAuth / SDK-reach bento, a border-connected
  guarantees grid, and a close. Its *structure* comes from the Eventide
  design system; its *skin* is Event Horizon, reusing the existing
  `cz-*` primitives rather than forking them. Every claim on the page
  is drawn from the repository — no invented customers, logos or
  benchmarks, and the promotion panel is labelled illustrative product
  UI rather than measured telemetry.
- **`frontend/src/components/cosmic/Singularity.tsx`** - a ray-traced
  Schwarzschild black hole anchoring the hero. Each pixel integrates a
  null geodesic (`d²u/dφ² + u = 3Mu²` in Cartesian form), so the
  gravitational lensing, photon ring and Einstein ring emerge from the
  physics instead of being drawn on. Disk shading uses Keplerian
  orbital velocity, relativistic Doppler beaming at `g³`, and
  gravitational redshift. Geometric units, `rs = 1`, disk inner edge at
  the ISCO.
- **`frontend/src/styles/landing.css`** - page-level layout only
  (`ks-*`); defines no colours or type of its own.
- **`frontend/public/keepsave.svg`** + `KsMark` - a real brand mark
  (an event horizon drawn far-disk → core → photon ring → near-disk in
  front). The same artwork serves as the favicon, replacing Vite's
  default.

### Added — architecture documentation

- **`docs/ARCHITECTURE_VIEWS.md`** + **`docs/diagrams/`** - ten checked-in
  SVG diagrams replacing the ASCII art in `README.md`: C4 levels 1-3
  (context, container, component) and the 4+1 views (logical, process,
  development, physical, and a scenarios view of an agent tool call with
  secret injection), plus OAuth and promotion flow diagrams.
- **`scripts/gen_diagrams.py`** - generates the whole set, so geometry,
  palette and type stay consistent. Edit the script, not the SVG.
- `docs/ARCHITECTURE.md` keeps its annotated package-dependency map as
  text — the inline annotations are its whole value — and now
  cross-references the view set.

### Added — cosmic auth screens

- **`frontend/src/components/cosmic/CometField.tsx`** - WebGL backdrop for
  the login and register screens, and the companion piece to the landing
  page's `Singularity`: the landing shows the hole, this shows what falls
  into it. Orbits integrate `a = -(GM/r³)·r·(1 + 3h²/c²r²)` — Newtonian
  gravity plus the first post-Newtonian correction, so the ellipses
  precess — using velocity Verlet, which is symplectic and therefore does
  not let orbital energy drift the way forward Euler does on a screen left
  open. Each comet renders the two tails real comets have: a straight
  anti-radial ion tail and a curved, lagging dust tail, both scaled by
  outgassing proportional to 1/r². Tails point *away from the mass*, not
  backwards along the path — a trail of past positions is a trajectory,
  not a tail.
- **`frontend/src/hooks/useCosmicEntrance.ts`** - shared anime.js staggered
  entrance for both auth screens.
- **`frontend/src/lib/motion.ts`** - one feature-detected
  `prefersReducedMotion()`, replacing four duplicated copies that called
  `window.matchMedia` unguarded. It is absent in jsdom and some embedded
  webviews, which broke the LoginPage test suite.

### Fixed

- The auth-screen entrance could leave the form **permanently invisible**.
  anime.js v4 drives animations through the Web Animations API, which
  composites over the inline style rather than replacing it, so the
  resting value underneath stayed `opacity: 0`. The hook now clears the
  inline hiding styles on completion, with a timer as a backstop.

### Changed — design system

- **Typography** is now Geist + Geist Mono, replacing Space Grotesk +
  JetBrains Mono. Display weights moved 300 → 400 with tightened
  tracking, since Geist runs lighter at the same nominal weight.
  `font-feature-settings` reduced to `tnum` — `ss01`/`cv01`/`cv11` were
  Space Grotesk features with no meaning for Geist.
- **Surfaces** gained a three-step elevation ladder (`--cz-elev-1`
  cards, `--cz-elev-2` + `.cz-float` for dialogs and the command
  palette, `.cz-flat` for in-page panels), a `--cz-rim` top-edge light,
  and a grain overlay so large glass panels stop banding. Light theme
  gets its own shorter, violet-tinted shadows. Login and the whole app
  shell inherit all of this.
- **Routing**: logged-out `/` now renders the landing page and the
  login form moved to `/login`. Any other logged-out path still falls
  through to login, so deep links keep working.

### Removed

- `frontend/src/components/cosmic/EhMark.tsx` and its `.cz-eh-mark`
  styles, superseded by `KsMark`. The old mark was CSS-only, so it
  could not be used as a favicon and blurred below ~20px.

### Notes — landing page

- `three` and `animejs` are bundled, not CDN-loaded, so the existing
  `script-src 'self'` CSP needs no new rules. three.js is code-split
  into its own lazy chunk (~190 kB gzip) fetched only when the hero
  mounts.
- Motion is gated on `prefers-reduced-motion` and `data-motion="off"`
  throughout; the singularity renders a single static frame and falls
  back to the CSS `EventHorizon` when WebGL is unavailable.
- Verified at ~492px viewport width; the desktop breakpoints have not
  been visually confirmed.

### Added — governance artifacts

- **`docs/ROLES.md`** - 9-role operating model (Tech Lead, Security
  Engineer with veto on crypto/auth/promotion, Backend, Frontend,
  DevOps, QA, PM, UX, Tech Writer). Each role mandate is anchored to
  owned artifacts already in the repo, not abstract titles. Includes
  Type-1/2/3 decision classification, phased team composition, hiring
  filters, and an anti-pattern list.
- **`docs/ROLES_30_60_90.md`** - per-role 30/60/90 action plan for
  Phase A (MVP hardening). Interim owners named for the un-staffed
  roles. Critical-chain dependencies + explicit "not building" list.

### Added — ADRs (Architecture Decision Records)

- **`docs/adr/`** - directory with README (lifecycle + numbering),
  `0000-template.md`, and four backfilled ADRs documenting decisions
  already in the codebase:
  - `0001-envelope-encryption.md` - AES-256-GCM with two-level envelope;
    rejects CBC+HMAC and XChaCha20-Poly1305 with reasons.
  - `0002-auth-model.md` - JWT for humans, API keys for agents;
    rejects JWT-everywhere and API-keys-everywhere.
  - `0003-promotion-engine.md` - decrypt-and-rewrap with PROD approval
    gate; explicitly rejects verbatim-ciphertext copy on nonce-reuse
    grounds.
  - `0004-key-hierarchy.md` - two-level hierarchy (master KEK +
    per-project DEK); rejects three-level and per-secret-DEK.
- Each ADR includes file:line references to the actual code, alternative
  options with explicit rejection rationale, consequences, and rollback
  plan.

### Added — 30-day execution artifacts

- **Architecture and security**
  - `docs/ARCHITECTURE.md` - one-page dependency map with three trust
    boundaries (network, identity, key custody); "no upward edges" rule.
  - `docs/THREAT_MODEL.md` re-baselined to v1.2.0 with file:line refs
    and a "Findings new" block surfacing five critical/high gaps the
    v1.1.0 model missed.
  - `docs/VETO_LIST_AUDIT.md` - 90-day commit triage in protected paths
    + proposed CODEOWNERS + commit-message convention.
- **Backend specs**
  - `docs/AUDIT_LOG_COVERAGE.md` - canonical event taxonomy, service
    pattern, test obligations, read-path deferral note.
  - `docs/ERROR_HANDLING_STANDARD.md` - `httperror` package design,
    handler/service pattern, migration plan, lint rule to forbid
    `err.Error()` in `c.JSON`.
- **Frontend specs**
  - `docs/EMBED_STATE.md` - widget state machine, audit-log requirement
    per transition, auto-clear policy.
  - `docs/EMBED_ORIGIN_POLICY.md` - per-project origin allow-list spec;
    forbids wildcard `'*'` postMessage; framing guard; integrator CSP.
- **DevOps**
  - `docs/SECRET_SOURCES.md` - dev/staging/prod source map per secret;
    rotation cadence; break-glass principles.
  - `docs/CI_PERMISSIONS.md` - job-by-job permission map + branch
    protection requirements.
  - `docs/RUNBOOK.md` extended with §6 deploy-rollback drill and §7
    break-glass production secret read procedure.
- **QA**
  - `tests/PYRAMID.md` - package-by-package census; flags the
    "inverted pyramid" shape.
  - `tests/NEGATIVE_AUTH_PLAN.md` - 12-endpoint × 11-attacker-case
    matrix; every cell missing today.
  - `tests/FLAKY.md` - triage SLA + fix patterns + quarantine etiquette.
- **PM / UX / Tech Writer (interim Tech Lead)**
  - `docs/ROADMAP_NOT.md` - explicit non-goals + exception process.
  - `docs/UX_STATE_INVENTORY.md` - per-screen state inventory; rule
    that destructive actions must use typed confirmation, not
    `window.confirm`.
  - `docs/DOCS_SANITIZATION_AUDIT.md` - example-value rules + detector
    script spec.

### Added — code / tooling

- `.github/workflows/ci.yml` - top-level
  `permissions: contents: read` (least-privilege CI default).

### Changed — `docs/FOLLOWUPS.md`

Re-shaped from the v1.1.0 audit delta into a persistent tracker with
owner + due date per item. Top 10 Phase A items are ordered by leverage;
Phase B items captured but explicitly deferred. New P0 / P1 items
discovered during the 30-day audit are slotted ahead of the original
list:
- Secret/Project/API-key mutations are not audit-logged today.
- Multiple handlers return `err.Error()` directly, leaking
  `pgx`/`pq`/`crypto/cipher` text to clients.
- The embed widget accepts auth from any origin
  (no `ev.origin` check, outbound target origin `'*'`).
- No handler-level negative-auth tests exist anywhere.
- Approver-cannot-be-requester invariant is unverified at the DB layer.

### Fixed — in this branch's earlier commits

- Backend: `gofmt` clean (13 files reformatted); fixed a password-policy
  test typo (`wantErr: false` on a too-short password); fixed the
  Prometheus histogram renderer which was emitting invalid format
  (`name{label="x"}_count` instead of `name_count{label="x"}`).
- Frontend: cleared every `npm audit` finding (high `picomatch` ReDoS
  advisory and follow-on `esbuild`/`vitest` chain); bumped `vitest` 2 → 4.1.6.
- Backend deps: bumped `x/net`, `x/crypto`, `x/text`, `x/sys`,
  `protobuf`, `gin`, `validator`, `lib/pq`, `mysql`, `sqlite3`.

### Notes — governance work

- The governance changes above are docs and one workflow line; no API
  or runtime behavior changes.
- CI was blocked on a GitHub Actions quota during development; the
  changes have been verified locally (`go vet`, `gofmt`, `go test`,
  `npm test`, `npm audit`, `tsc --noEmit`, `npm run build`).

---

## [1.1.0] - 2026-04-19

**Phase 16 - Production Hardening.** Closes every open item in
`SECURITY_AUDIT.md` "Recommendations for Production Deployment". See
`PHASE16_CHANGELOG.md` for the full summary.

### Added

- **Pluggable MasterKeyProvider** (`backend/internal/crypto/keyprovider/`)
  - `Provider` interface with `Name / GetMasterKey / Rotate`
  - `EnvProvider` (default) wraps the existing `MASTER_KEY` base64 path
  - `AWSKMSProvider` and `GCPKMSProvider` accept narrow decrypter
    interfaces so SDK wiring stays out of this package
  - `VaultProvider` calls HashiCorp Vault Transit via stdlib `net/http`
  - Table-driven tests with fakes for every provider
  - Selected at runtime via `KEEPSAVE_KEY_PROVIDER=env|awskms|gcpkms|vault`
- **In-app TLS** - `TLS_CERT_FILE` + `TLS_KEY_FILE` enable TLS 1.2+
  listener; `TLS_REDIRECT=true` runs an 80->443 redirect goroutine;
  `TLS_CIPHER_SUITES` takes a comma-separated IANA name list
- **Dependabot config** for gomod, npm (frontend + nodejs SDK), pip
  (python SDK), docker (backend + frontend), and github-actions
- **GitHub secret scanning config** (`.github/secret_scanning.yml`)
- **Frontend npm audit** CI job; blocks `docker-build` on high+ findings
- **`govulncheck` gating** - `security-scan` is now a `docker-build`
  dependency; previously advisory
- **Audit log retention knob** - `AUDIT_LOG_RETENTION_DAYS` env var
  (default 365); nightly pruner lands in 1.1.1
- **Docs** - `docs/THREAT_MODEL.md`, `docs/RUNBOOK.md`,
  `docs/PENTEST_CHECKLIST.md`, `docs/SEIDR_INTEGRATION.md`
- **Config tests** - `backend/internal/config/config_test.go` covers
  defaults, prod lockdown, and KMS-provider skip-MASTER_KEY behavior

### Changed

- `config.Load()` now reads `KEEPSAVE_ENV`, `KEEPSAVE_KEY_PROVIDER`,
  `TLS_*`, `AUDIT_LOG_RETENTION_DAYS`, `VAULT_*`, and KMS params;
  the existing `MasterKey []byte` field remains populated only when
  `KEEPSAVE_KEY_PROVIDER=env`
- `KEEPSAVE_ENV=production` refuses `CORS_ORIGINS=*`
- `KEEPSAVE_ENV=production` refuses `sslmode=disable` in `DATABASE_URL`
- `Content-Security-Policy` drops `'unsafe-inline'` from `style-src`
  and adds `frame-ancestors 'none'`, `base-uri 'self'`,
  `form-action 'self'`, `connect-src 'self'`
- `Strict-Transport-Security` gains `preload`
- Helm chart bumped to 1.1.0 with KMS env plumbing, optional TLS volume
  mount, and sslmode=require default `DATABASE_URL`
- Frontend bumped to 1.1.0; Go/Node/Python SDKs bumped 2.0.0 -> 2.1.0
  (SDK line stays on 2.x to avoid a downgrade for existing consumers)

### Security

- **SECURITY_AUDIT.md "Recommendations for Production Deployment" fully
  closed.** Each of the 8 items now shows a tick with a link to the
  commit, config file, or docs page that closed it.
- Master key now cached in memory only; never written to disk
- TLS 1.2+ enforced at the application tier when in-app TLS is enabled

### Known follow-ups

- AWS/GCP KMS SDK adapters (pending `go mod tidy` with real SDK deps)
- Phase 15 service unit tests (feature-complete but untested)
- Backup tamper-detection test formalizing AEAD-auth guarantee
- Audit-log pruner consumer wiring in `main.go`
- Roadmap.md Phase 15 checkboxes and README.md feature-phases table

---

## [1.0.0] - 2026-03-14

### Added

- **Phase 7: Observability & Monitoring** - Full observability stack
  - **Prometheus metrics** - Custom metrics collector with counters, gauges, and histograms; `/metrics` endpoint in Prometheus exposition format
  - **Metrics middleware** - Per-request tracking of latency, error rates, in-flight requests, rate limit hits
  - **Application metrics** - `keepsave_http_requests_total`, `keepsave_secrets_encrypted_total`, `keepsave_auth_attempts_total`, `keepsave_promotions_total`, and 10 more metrics
  - **Distributed tracing** - Request tracing with trace/span ID propagation via `X-Trace-ID`/`X-Span-ID` headers
  - **Tracing middleware** - Automatic span creation for every HTTP request with status, method, and IP attributes
  - **Admin dashboard** - System health overview page with traces, events, plugins, and security event views
  - **Test suite** - 8 metrics tests (counters, gauges, histograms, concurrency, formatting) + 6 tracing tests (spans, propagation, limits, eviction)

- **Phase 8: SDK & Developer Experience** - Multi-language SDKs and API documentation
  - **Python SDK** (`sdks/python/`) - Full-featured client with auth, projects, secrets, promotions, key rotation, import/export
  - **Node.js SDK** (`sdks/nodejs/`) - TypeScript client with complete type definitions and all API endpoints
  - **Go SDK** (`sdks/go/`) - Idiomatic Go client with context support, functional options pattern, and error types
  - **OpenAPI 3.0 specification** - Interactive API documentation served at `/api/docs` with schemas for all resources
  - All SDKs support both JWT token and API key authentication

- **Phase 9: Enterprise Features** - Enterprise-grade security and compliance
  - **SSO integration** - OIDC/SAML provider configuration per organization with encrypted client secret storage
  - **IP allowlisting** - CIDR-based IP restrictions per organization and per project
  - **Compliance reports** - SOC 2, GDPR, and PCI report generation with automated security posture assessment
  - **Secret lifecycle policies** - Configurable max age, rotation reminders, and mandatory rotation per project
  - **Encrypted backups** - Full/incremental backup snapshots with AES-256-GCM encryption
  - **Security event logging** - Dedicated security events table with severity levels and detailed context
  - Database migration `005_phase7_12.sql` with 11 new tables for all Phase 7-12 features

- **Phase 10: Security Hardening Deep Dive** - Production security hardening
  - **CSRF protection** - Single-use CSRF tokens for all mutation endpoints; API key requests exempted
  - **Security headers middleware** - HSTS, X-Content-Type-Options, X-Frame-Options, CSP, Referrer-Policy, Permissions-Policy
  - **Request body size limits** - 1MB default limit to prevent payload DoS
  - **Password complexity** - Configurable policy requiring uppercase, lowercase, digits, and special characters
  - **Password strength scoring** - 0-4 strength score for UI feedback
  - **Session token tracking** - Database-backed session tokens for revocation support
  - **Test suite** - Password policy validation tests and strength scoring tests

- **Phase 11: AI Agent Experience** - Purpose-built features for AI agents
  - **Just-in-time secret leases** - Time-limited access grants (1 min to 24 hours) with automatic expiration
  - **Lease management** - Create, list active, and revoke leases via API
  - **Agent activity tracking** - Per-action logging with API key, project, environment, secret key, and IP
  - **Activity analytics** - Action summaries grouped by type with last-used timestamps
  - **Access heatmaps** - Secret access frequency data bucketed by hour for the last 7 days
  - **Lease and activity tables** - `secret_leases` and `agent_activities` tables with proper indexing

- **Phase 12: Platform Ecosystem** - Extensible platform architecture
  - **Event bus** - In-process event bus with persistent event log, pub/sub pattern, wildcard subscribers, and event replay
  - **Event types** - 16 defined event types: `secret.created`, `promotion.completed`, `key.rotated`, `lease.created`, etc.
  - **Plugin system** - Registry for secret providers, notification senders, and validators with interface contracts
  - **Plugin management** - Database-backed plugin registry with enable/disable toggle
  - **Access policies** - Time-window, IP-restriction, and geo-restriction policy types per project
  - **Platform API** - Events listing, event replay, plugin registration, and access policy CRUD
  - **Test suite** - 5 event bus tests (pub/sub, wildcards, payloads) + 5 plugin registry tests (providers, senders, validators)

- **Frontend updates**
  - Admin Dashboard page with 5 tabs: Overview, Traces, Events, Plugins, Security
  - Navigation updated with Admin link
  - API client extended with 20+ new endpoint functions for all Phase 7-12 features

### Fixed

- **API key error display** - Frontend showed `[object Object]` instead of actual error message when API key creation failed; fixed error extraction from nested `{ error: { code, message } }` response format
- **API key scopes parsing** - Empty scopes input no longer sends `[""]` to the backend; added `.filter(Boolean)` to scope splitting
- **API key empty projects guard** - Create form now shows informational message when no projects exist instead of rendering a broken form
- **API key environment validation** - Backend now rejects invalid environment values with binding tag `oneof=alpha uat prod`
- **API key scope validation** - Backend now validates each scope value must be `read` or `write`
- **API key project authorization** - Backend verifies project ownership before creating API key; returns 403 for unauthorized access and 404 for missing projects
- **API key delete authorization** - Delete query now filters by `user_id` to prevent users from deleting other users' API keys; returns error when key not found or not owned

### Changed

- **Router** - Extended with 30+ new endpoints across admin, enterprise, agent, and platform route groups
- **Main entry point** - Wired all new services, repositories, and handlers for Phases 7-12
- **Health endpoint** - Version bumped to 1.0.0

---

## [0.6.0] - 2026-03-14

### Added

- **Completed Roadmap** - Extended project roadmap from 9 to 12 phases with detailed task breakdowns
- **idea.md** - Comprehensive ideas document with 36 improvement proposals
- Updated milestone summary table with Phase 10-12

---

## [0.5.0] - 2026-03-13

### Added

- **Phase 5: Hardening and Operations** - Rate limiting, key rotation, secret versioning, webhooks, health checks, structured logging, Helm chart, CI/CD pipeline, OWASP security audit checklist
- Database migration for `secret_versions`, `webhook_configs`, and `webhook_deliveries` tables
- Comprehensive test suite expansion (49+ backend tests)

---

## [0.3.0] - 2026-03-13

### Added

- **Frontend Dashboard** - React 18 + TypeScript + Vite web application with auth, project management, secret CRUD, environment switcher, promotion wizard, diff review, audit log viewer, API key management
- Docker support with Nginx reverse proxy for frontend container
- Comprehensive frontend test suite (27 tests across 5 test files)

---

## [0.2.0] - 2026-03-13

### Added

- **Environment Promotion Engine** - Promote secrets between environments (Alpha -> UAT -> PROD) with diff preview, rollback, approval workflow, and audit logging
- Database migration for `promotion_requests` and `secret_snapshots` tables

---

## [0.1.0] - 2026-03-13

### Added

- **Project scaffold** - Go 1.22+ backend with Gin framework, PostgreSQL 16, Docker Compose
- **Database schema** - `users`, `projects`, `environments`, `secrets`, `api_keys`, `audit_log` tables
- **AES-256-GCM encryption** - Envelope encryption with per-project data encryption keys (DEK)
- **Full CRUD API** for projects and secrets with JWT auth and API keys
- **Unit tests** - Crypto layer, auth service, and JWT validation

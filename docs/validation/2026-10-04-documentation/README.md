# Documentation, diagrams and integration wiring — 4 October 2026

Scope: source candidate `6684aca`, already published through `develop`/`main` on
October 3. This change updates documentation, branded SVG/HTML assets and CI
wiring. It changes no API/auth/crypto/database/runner runtime behavior and starts
no Docker services. The canonical project remains `company/apps/KeepSave`.
Supporting worktrees are organized using Git, preserving landing changes.

## Document and workflow checks

The repository checker covers every tracked/nonignored Markdown document,
including hidden workflow guidance; it validates local link targets/Markdown
headings, code-fence closure, the document inventory, SVG XML/IDs and deterministic
diagram drift. It makes no external URL or runtime support claim. Current guides
are source reconciled; dated research, findings, metrics and execution receipts
retain their original scope. Root historical notes moved to `docs/archive/` and
relative links were rebased. No reviewer signatures were invented. Final local validation passed for **196
Markdown documents, 1,049 local links and 11 SVG assets**, with zero errors or
historical-link warnings.

Independent reviews checked the system, operator/security, client/fixture and
historical-document slices. OpenAPI counts were read directly: 112 paths, 141
operations and 177 schemas. Client examples expose remaining qualification gaps;
Robot/Seidr fixtures and unsafe retained CI/Terraform delivery examples are not
advertised as accepted integrations.

Workflow validation uses actionlint 1.7.12 plus Python parsing and shell syntax.
A mode-0644 fixture reproduces direct execution exit126 and verifies explicit
Bash reaches the intended script. No real PostgreSQL/container suite ran for this
bounded wiring/docs change. Frontend scanning and both SBOMs continue after a
backend image failure; HIGH/CRITICAL checks remain blocking. The new documentation
job uses read-only repository permission and the standard-library checker.

## Static visuals

Ten architecture/workflow SVGs now derive the accepted Field Twist geometry from
`frontend/public/keepsave.svg`. The header uses the same mark. Bundled Chromium
renders were inspected for all ten diagrams and the header; node/text containment
and arrow semantics were reviewed independently. Corrected overlapping edge
labels, the optional-refresh path, opaque one-use ticket wording and supervisor
control direction. XML validity and deterministic regeneration pass independently
of perceptual review. No application visual acceptance was rerun.

## Interactive architecture receipt

- Diagram type: `architecture`.
- Output: [harness-neutral.html](../../diagrams/harness-neutral.html).
- Source: [typed JSON](../../diagrams/harness-neutral.architecture.json), linked to
  exact candidate `6684aca5343543932c62ffa720947c3d9b400fd6`.
- Validation: **9/9 showcase checks, zero composition errors/warnings**.
- Browser evidence: **passed**, using bundled Chromium. Four desktop sizes:
  1440×900, 1600×1000, 1920×1080 and 2048×1320; no page overflow.
- Light/dark captures: both endpoint sizes, READ / Still state.
- Visual review: **passed** after actual rendered-image inspection; separate from
  machine containment evidence. Two focused visual correction rounds.
- No provider operation, client authentication, live execution or deployment is
  performed by opening this self-contained viewer.

Specification SHA-256: `1a81672fc0339d4e408564a860f45e192686dcf6b09cb5e51b8aa2dd96a25d79` (5099 bytes).

Artifact SHA-256: `ca517f866748ee8d5fdf582070397cb35aa9631c904927f93cf65c91a30b9678` (716159 bytes).

[Automated browser receipt](../../diagrams/harness-neutral.visual-check.json) and
[light/dark contact sheet](../../diagrams/harness-neutral.visual-check.html) bind
this exact artifact. A failed first composition/containment attempt was repaired
and retested; only the final hash is claimed. Private raw review/workflow/source
receipts remain under `/mnt/data/keepsave-docs-2026-10-04`.

## Release limits

The previous exact October 3 CI runs failed. The new docs/wiring push needs its
own exact-revision run; it does not supply a green CI result. The backend image
security issue remains unresolved. [Current status](../../STATUS.md) records
scope and run links. Formal Security/TL reviews, real providers/SMTP/native clients,
GitHub App and runner isolation plus installation recovery/operational drills
remain gates. New flags stay off. No tag or full-application deployment occurs.

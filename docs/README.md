![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

# KeepSave documentation

Reviewed against the harness-neutral source candidate on **4 October 2026**.
KeepSave combines a self-hosted encrypted vault with controlled developer-tool
access. The source is integrated; the full application is **unreleased**.
Implementation, synthetic checks, independent review and installation acceptance
are separate states. Start with the [current status](STATUS.md) before enabling
capabilities or interpreting historical documents.

## Find the right guide

| Task | Guide |
|---|---|
| Understand the product and prerequisites | [Project README](../README.md) |
| Find the canonical project and module folders | [Project structure](PROJECT_STRUCTURE.md) |
| Understand trust and module ownership | [Architecture](ARCHITECTURE.md), [visual views](ARCHITECTURE_VIEWS.md) |
| Preserve the accepted visual identity | [KeepSave branding](BRANDING.md) |
| Set up login | [Google/GitHub setup](SOCIAL_LOGIN_SETUP.md) |
| Prepare a team installation | [Control-host reference](../deploy/self-hosted/README.md), [platform setup](design/2026-10-02-harness-neutral-platform/SETUP.md) |
| Prepare an isolated connector host | [Runner reference](../deploy/runner/README.md) |
| Diagnose an incident, revoke access or recover | [Runbook](RUNBOOK.md) |
| Inspect supported request contracts | [API chapter](system/03-api-reference.md), [OpenAPI](../backend/internal/api/openapi/core.json) |
| Review security and audit | [Threat model](THREAT_MODEL.md), [audit coverage](AUDIT_LOG_COVERAGE.md) |
| Understand every subsystem | [System chapters](system/README.md) |
| Add a harness/provider or client adapter | [Harness engineering](HARNESS_ENGINEERING.md), [integrations](INTEGRATIONS.md) |
| Review decisions and next work | [ADRs](adr/README.md), [roadmap](../Roadmap.md), [follow-ups](FOLLOWUPS.md) |
| Inspect executed checks | [Core ledger](validation/2026-10-01-core-release/ACCEPTANCE.md), [platform ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md) |
| Find historical plans and audits | [Documentation map](DOCUMENTATION_MAP.md), [audit archive](audits/README.md), [root archive](archive/README.md) |

## What each folder means

`system/` explains current source. `design/` records dated design and implementation
checkpoints. `validation/` records checks actually executed, including skips and
limits; its dates are evidence dates, not promises of ongoing health. `releases/`
distinguishes source publication from tagged/released behavior. `adr/` records
architectural decisions and their review status. `research/`, `audits/` and
`archive/` retain original observations and superseded plans. `diagrams/` and
`assets/` contain reproducible documentation visuals.

The [documentation map](DOCUMENTATION_MAP.md) inventories repository Markdown,
including client and engineering guides outside `docs/`. Historical material is
retained with scope pointers instead of rewriting earlier findings as current
acceptance. Governance requirements remain in [CLAUDE.md](../CLAUDE.md),
[ADLC](ADLC.md) and [roles](ROLES.md); a documentation refresh grants no sign-off.

The landing remains at [keepsave.draveniq.dev](https://keepsave.draveniq.dev/).
The full application's intended origin is `app.keepsave.draveniq.dev`.
These guides do not claim that application has been deployed.

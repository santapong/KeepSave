# KeepSave roadmap

KeepSave is a self-hosted vault and harness-neutral developer-access platform.
Reviewed 4 October 2026. The current source is integrated and locally exercised;
formal release, external qualification and deployment remain gated. Read
[status](docs/STATUS.md) and [current architecture](docs/ARCHITECTURE.md) before
interpreting a milestone as supported behavior.

## Accept the current candidate

| Gate | Candidate scope | Next acceptance |
|---|---|---|
| G0 — Identity and vault | Login, revocable sessions, permissions, history, restore, retained keys and recovery | Real Google/GitHub, operator recovery material/fresh database/cutover and independent reviews |
| G1 — Shared authority | Stored ownership, epochs, barriers, safe diagnostics/audit exports | Reviewed entrypoint matrix and lock/failure tests |
| G2 — Team vault | Lifecycle/reminders, method safety, proofs, invitations/recovery/offboarding | Accepted SMTP/mailbox and live team/account exercises |
| G3 — Portable MCP/OAuth | Stateless transport, exact resource, opaque delegation and synthetic tools | Exact-version Codex and isolated Hermes login/discovery/invocation/cancel |
| G4 — GitHub broker/runner | One repository/commit per run, custody, operations, fencing and private mTLS | Repository A succeeds/B fails, revocation/result denials, canaries and separate-host isolation |
| G5 — Private profiles | Immutable instruction-only source, portable profile, native packages/exporters | Both clients discover/use the approved review skill; tampered/revoked dependency denial |
| G6 — Team operations | Control/runner references, worker and runbooks | Two APIs, failure/restore/upgrade/compatible rollback and measured capacity |

G3 synthetic protocol work can accompany G2 after authority review. Keep concurrency
bounded to one core/team slice and one protocol slice. New flags remain off until
their gates pass. Do not equate completed source with passed acceptance.

## Follow-on feature order

First finish current-release evidence and remediation. Then refine credential
expiry/renewal reminders, understandable safe denials, scoped developer offboarding
and account recovery. Additional harness/provider adapters reuse shared policy,
profiles, runs and broker ports and publish support only by tested version and
capability. Do not couple the core to Codex or promise universal interoperability.

External secret delivery, arbitrary connector builds, executable skill scripts,
model/subscription credential custody, enterprise SSO, marketplaces, device
attestation and multi-region operation remain deferred. See the
[non-goals](docs/ROADMAP_NOT.md) and [researched backlog](docs/design/2026-10-02-product-roadmap/BACKLOG.md).

## Governance and history

Type-1 changes retain [ADR](docs/adr/README.md) and independent Security/TL gates
under [development rules](CLAUDE.md). CI/recovery/provider/isolation and operational
checks precede tags/deployment. The full application target is
`app.keepsave.draveniq.dev`; retain the landing at `keepsave.draveniq.dev`.

The former Phase1–16 checkbox roadmap is preserved in
[the dated legacy archive](docs/archive/ROADMAP_LEGACY.md). Its historical checks
are not a present release-acceptance claim. The approved harness-neutral plan and
implementation limits are in the [October checkpoint](docs/design/2026-10-02-harness-neutral-platform/README.md).

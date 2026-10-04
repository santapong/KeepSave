# KeepSave implementation and acceptance status

Checked **4 October 2026**. Source candidate `6684aca` was integrated through
`develop` (`1021435`) and published to `main` (`202cc5d`) on October 3. Their
product-source trees match. The October 4 documentation/CI-wiring update does not
create a release tag or deploy the full application.

| Area | Implemented/local evidence | Still required |
|---|---|---|
| Identity and vault | Password/social identity, tracked sessions, scoped teams, history, retained keys and recovery; actual-router/database fixtures | Real Google/GitHub consent, installation key/cutover/recovery, independent reviews |
| Shared authority | Ordered barriers, member epochs, current-result checks, safe denials and bounded two-process session revocation | Complete reviewed entrypoint coverage and full fault exercises |
| Team features | Lifecycle/reminders, methods, proofs, invitations, recovery and scoped offboarding | Installation SMTP/mailbox and live account/team acceptance |
| MCP/OAuth | Official Go SDK v1.8.0, stateless transport, two synthetic lanes and opaque resource-bound delegation | Exact native Codex/Hermes authentication, tools, native-stop and compatibility evidence |
| Broker and runs | GitHub App custody, stored bindings, commit-bound grants, attempts/tickets/budgets/private results | Disposable live GitHub App exercise, credential canaries and separate-host isolation |
| Private profiles | Immutable instruction-only sources, portable profiles and candidate native packaging | Both clients discover and use approved skills; tested local control evidence |
| Operations | Reference bundles, fenced worker, private mTLS listener and operator/runbook source | Production TLS/Transit/storage, two-API/failure/upgrade/recovery drills and measured capacity |

New capabilities default off. The current host's runner preflight fails because
CPU control is not delegated. SQLite/MySQL tests establish bounded legacy behavior,
not new platform parity. The app version remains `1.4.0-rc.1`, unreleased.

## Remote CI is not green

The exact October 3 runs were freshly checked through GitHub's public API:

| Branch/revision | Run | Outcome |
|---|---|---|
| `main` / `202cc5d` | [37136794513](https://github.com/santapong/KeepSave/actions/runs/37136794513) | 13 successful jobs, 2 failed, 1 intentionally skipped signing scaffold |
| `develop` / `1021435` | [37136794569](https://github.com/santapong/KeepSave/actions/runs/37136794569) | Same bounded job outcomes |

The PostgreSQL job failed with exit 126 before tests started: Git recorded its
script as non-executable while CI invoked it directly. October 4 changes invoke
Bash explicitly; a local mode-0644 fixture reproduced the failure and verified the
interpreter path. No PostgreSQL suite is claimed from the failed remote job.

The backend Trivy image scan also failed. Unauthenticated raw-log access was
refused, so the precise remote advisory set is not known. Separately, the retained
October 2 refreshed **local** scan records tzdata/OpenPGP findings; that is dated
local evidence, not a reconstruction of the inaccessible remote log. The
currently published same-line distroless digest still matches the pinned base.
The scan remains blocking. CI now continues independent frontend scanning/SBOM
collection after a backend scan failure. No severity threshold was weakened.

A new push needs its own exact-revision CI result. Fixing workflow invocation,
publishing documents or generating diagrams cannot mark the source accepted.

## Evidence and limitations

Use the [core ledger](validation/2026-10-01-core-release/ACCEPTANCE.md),
[platform ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md) and
[documentation checks](validation/2026-10-04-documentation/README.md) for scoped
executed evidence. Historical totals remain tied to their original snapshots.
The final frontend guard was tested in affected files; no new aggregate full-suite
result is invented. Tests using synthetic providers do not prove actual consent,
SMTP delivery, native harness use or enforced runner isolation.

Independent Security Engineer and Tech Lead review remains pending for the
relevant Type-1 changes. The owner-authorized October 3 source publication was a
bounded exception; it is not a standing waiver. Tagging or production deployment
requires the applicable review, provider, recovery, CI and operational gates.

Preparation tasks remain in the [project README](../README.md#what-the-operator-needs-to-prepare).
The [roadmap](../Roadmap.md) prioritizes acceptance of the existing core/platform
before new provider or harness expansion. The public landing is separate from
this full application and is retained.

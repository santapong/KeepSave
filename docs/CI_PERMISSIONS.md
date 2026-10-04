# KeepSave CI permissions and release evidence

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Source reconciled 2026-10-04 against [.github/workflows/ci.yml](../.github/workflows/ci.yml).
CI runs on pushes and pull requests targeting **both `develop` and `main`**.
The canonical project is `/mnt/data/company/apps/KeepSave`. Local passing checks
and remote CI are separate evidence; a source push is not a production deployment.

## Least-privilege token policy

The workflow declares `contents: read` at the top level. Job-local overrides add
only their publishing needs. Do not describe every job as read-only: security
publishing and the disabled signing scaffold have explicit extra permissions.

| Job group | Current permissions | Behavior |
|---|---|---|
| Documentation and diagrams | `contents: read` | Python standard-library `check_docs.py`: document map, local links/fences, SVG structure and deterministic generator |
| PostgreSQL contracts, backend lint/test/build, SDK tests | `contents: read` | Synthetic test databases, vet/race/shuffle/fuzz/coverage, binaries and local SDK contracts |
| Frontend type/test/audit/build | `contents: read` | Pinned lockfile install, generated-type check, unit tests, high-level npm audit and app/widget build |
| govulncheck | `contents: read` | Blocking reachable-vulnerability command and report artifact |
| gosec and CodeQL | `contents: read`, `security-events: write` | SARIF/security publishing; gosec uses `-no-fail` and is not a clean-finding gate |
| Docker build/Trivy/SBOM | `contents: read`, `security-events: write` | Local images, HIGH/CRITICAL Trivy gate with `ignore-unfixed: true`, security results and SBOM artifacts |
| Cosign scaffold | `contents: read`, `packages: write`, `id-token: write` | **Disabled with `if: false`**; no signed/pushed-image/provenance claim |
| Robot API acceptance | `contents: read` | **Dry-run parsing**, not live HTTP/browser acceptance |

Actions are pinned to full commit SHAs. The initial runtime pins are Go **1.27.1**
and Node **24.21.0**, with Trivy **0.74.0** and govulncheck **1.8.0**. Record actual
built image digests and scan database dates in the acceptance receipt; a pinned
installer does not prove a fresh vulnerability database or a clean image.

The PostgreSQL script owns disposable fixtures and must never use an operator
DSN. Do not expose production secrets through test databases, artifacts, shell
arguments or logs. Synthetic `MASTER_KEY`, `JWT_SECRET` or `DATABASE_URL` names
in a test configuration are not production access. Real providers, SMTP, Transit,
GitHub and native harness qualification use separately authorized exercises.

## Repository review and protection

[CODEOWNERS](../.github/CODEOWNERS) exists. It names the owner placeholder for
crypto, auth, promotion and workflow paths; it does not prove an independent
Security Engineer/Tech Lead signature or coverage of all newly sensitive modules.
Review the ownership map as module boundaries grow. No edit to this document
changes actual hosted branch-protection rules.

Verify in the GitHub web settings:

- Required checks match the actual current workflow job names on `develop` and
  the release line, including PostgreSQL and SDK/generated-contract gates.
- No force pushes to permanent branches; required reviews/code-owner review are
  configured as intended by [repository rules](../CLAUDE.md).
- Fork/PR jobs receive no production secrets or broadly privileged token.
- `pull_request_target` does not execute untrusted source with target privileges.
- Security publishing behavior is handled explicitly for forks/unavailable
  repository security features; do not silently call a failed upload accepted.

Current workflow source contains no `pull_request_target` trigger. Hosted review
rules, repository security-feature availability and organization/team assignments
still require read-back evidence from the web UI. Plain Git manages branches;
GitHub-only settings/reviews use the web UI, not `gh`.

## Current remote status and interpretation

The October 3 publication runs were rechecked on October 4 and **failed**:
[main run 37136794513](https://github.com/santapong/KeepSave/actions/runs/37136794513)
and [develop run 37136794569](https://github.com/santapong/KeepSave/actions/runs/37136794569).
These dated results must not be rewritten as passing because local tests succeeded.
Record any repair/rerun at its own exact revision. See the current documentation
hub and publication/validation receipts for reconciled evidence.

CI has no enabled full-application production deployment job. Image signing is a
scaffold, SAST findings are not all blocking, Robot dry-run does not call the API,
and a frontend type check is not an application-wide lint/a11y review. Preserve
these limits in README/release notes. Formal reviews, real-provider recovery and
operational gates remain separate even when every applicable CI job passes.

## Before declaring a release gate closed

Confirm exact revision/run conclusion; inspect failed/skipped jobs, report scope
and scan date; retain binary/image digests and actual fixture runtimes. Verify the
supported client contract and rollback/recovery exercise at that source. A tag
or source merge alone is not acceptance. Consult [DEPLOYMENT_PLAN](DEPLOYMENT_PLAN.md),
[SECRET_SOURCES](SECRET_SOURCES.md) and the
[acceptance ledger](validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

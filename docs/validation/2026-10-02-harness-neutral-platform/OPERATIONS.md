# Local process and container checks

Event/check date: October2,2026 (Asia/Bangkok). Dirty neutral candidate based on
3878e69. Synthetic loopback infrastructure only. This is not a deployment,
staging rollback, production recovery or capacity report.

## Reference/build checks

API and frontend Dockerfiles build with the pinned Go/Node runtimes documented
in [BACKEND](BACKEND.md) and [FRONTEND](FRONTEND.md). Final runtime images:

| Image | SHA256 image ID | Runtime user |
|---|---|---|
| `keepsave-neutral-api:local-20261002` | `43a2316ffaeb4014a30b4e0447469989cbcea11ff2a9bc0d4e46d37bc9be643b` | 65532:65532 |
| `keepsave-neutral-frontend:local-20261002` | `2b8c39b193c84e1a0fdf2bc09614477f06a7a43fbdb59dc175dbd30567199564` | 101 |

Trivy0.74.0 was checked against the retained official binary receipt, SHA256
`d89bcc6510a267f11b773398cbf1be5520ce39f9e8b6633178c4487f05b7d791`.
The first frontend scan used an October1 database and reported zero findings.
Refreshing the database exposed PCRE2 CVE-2026-103111 in the pinned nginx base.
The exact Alpine package was upgraded from10.48-r0 to10.49-r0 during the image
build; runtime privileges stay unchanged. This follows the
[PCRE2 maintainer advisory](https://github.com/PCRE2Project/pcre2/security/advisories/GHSA-r9hj-j2rw-4q3m).
The final frontend scan reports zero findings.

The API runtime digest was refreshed to
`gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab`.
The refreshed upstream image still contains tzdata2026b. Its final scan retains
[DLA-4792-1](https://security-tracker.debian.org/tracker/DLA-4792-1), whose fixed
package is2026c-0+deb12u1, plus four binary records for the same
[GO-2026-5932 OpenPGP advisory](https://pkg.go.dev/vuln/GO-2026-5932).
The source dependency check found no imported/reachable affected OpenPGP package.
These findings are retained, not suppressed; neither the refreshed digest nor
zero reachable functions establishes an entirely clean dependency inventory.

Both final scans use the same database, updated2026-10-02T12:48:00.080865328Z
and downloaded2026-10-02T13:50:00.609863398Z. The scans use offline/skip-update
flags after that explicit download. Receipts in
`/mnt/data/keepsave-neutral-2026-10-02/` include before-patch findings,
`container-*-build-patched.log`, `container-*-scan-patched.log`,
`container-*-vulnerabilities-final.json` and `trivy-cache/db/metadata.json`.
This is a scoped check-date scan, not an attestation or permanent assurance.

The final frontend image started on loopback4773 with a read-only root filesystem
and temporary runtime directories. HTTP checks passed after rebuilding and restarting the final images;
`container-http-final.json` records the exact IDs:

| Request | Observed status |
|---|---|
| `/readyz` |200 |
| `/developer-access` SPA fallback |200 |
| `/api/v1/capabilities` proxy |200 |
| `/api/v1/runner/operations/claim` |404, private runner route absent from public proxy |

Both self-hosted Compose files pass `config --quiet`, including the private mTLS
listener override. This uses empty synthetic fixture files and TEST-NET address
192.0.2.10; it starts no services and cannot verify actual key/TLS/storage access.
The real deployment still requires private installation certificates, database
TLS, Vault Transit, recovery material, reviewed admission flags and enrollment.

## Two-process journey

`scripts/test-platform-two-api.py` targets only fixed loopback4770/4772. It
creates synthetic users/projects and does not accept an operator DSN/token.
An optional `--context` saves only generated email/project IDs for browser use.
The owned PostgreSQL database is ephemeral; random disposable signing/wrapping
configuration is kept outside Git and never printed.

The two final running API containers passed these real HTTP checks:

- API1-created session works on API2; revocation committed on API1 denies its
  next protected request on API2, while a fresh login remains usable on API1.
- Foreign and nonexistent secret IDs share the denial; no credential value is
  returned to the other user.
- Lifecycle/history lists contain metadata only; value updates/restoration,
  historical reads after key rotation and v2 backup verification succeed.

The final repeat is `two-api-http-final.log`, elapsed0.383s, on the exact API
image above. Both old API processes were drained; API1 applied the additive
registry before API2 started. The ephemeral PostgreSQL target was preserved
across those API restarts. Earlier receipts are retained as historical support;
they do not establish the later recovery change. This brief functional journey
is not a load, migration-contention or complete failure-recovery drill.

## Runner recheck

After the interrupted host restart, `podman info` was rechecked rather than
assuming its earlier state survived: rootless=true, seccomp=true, cgroups=v2,
delegated controllers=`memory,pids`. CPU is not delegated. The implementation's
preflight requires CPU/memory/PID control and refuses this host. No connector
container or isolation acceptance was run. Synthetic command/TLS/relay tests
do not establish real host limits or direct network denial.

## Remaining exercises

Two-process committed revocation is bounded evidence. It does not prove shared
provider capacity, migration contention, worker/supervisor restart, database or
Transit outage recovery, complete upgrade/rollback, TLS/storage recoverability,
or advertised throughput/availability. Qualification uses a separate accepted
runner host, disposable GitHub installation and exact clients, then fixed
hardware measurements and fault/recovery exercises before release.

## Cleanup and source identity

Temporary browser/test infrastructure and disposable private configuration are
removed after acceptance. Only resources with the task ownership label are
eligible for cleanup; operator services and the published landing are preserved.
Built images and raw evidence remain available for review.

`source-manifest-final.json` identifies659 backend/frontend/deploy/script files,
including non-ignored untracked files. Its documented compact-JSON tree digest
is `5c92dca566bc1e48c7e3c1713fd29c5b55c02f10251bba4d85d265e3d45bb4d2`.
It describes the uncommitted candidate on base3878e69, not a Git release identity.

# Backend — local validation

Event/check date: October 2, 2026 (Asia/Bangkok). Scope: the uncommitted
`feat/harness-neutral-platform-20261002` working tree, based on
`3878e696cbcdeb97615320e5af864eae0885e4df`. The branch and dirty candidate are
preserved. These are local synthetic acceptance receipts, not a release,
independent review, remote CI result or production deployment.

## Reproducible environment

The shipped `scripts/test-platform-postgres.sh` creates a unique owned network
and disposable PostgreSQL container, waits for TCP readiness, initializes
synthetic databases, and runs the complete embedded migration registry. Tests
use isolated fixture schemas or independently created fresh databases. The
script never reads or forwards an operator database URL and removes only its
own resources. Additional test flags can select a narrow reproduction.

| Runtime | Verified pin |
| --- | --- |
| Go | `golang:1.27.1@sha256:e0174e51e81218523251d85d248a90d24c3d5e81543b4f07a5d66229397db190` |
| PostgreSQL | `postgres:16@sha256:1a6ab3f5345eb6dbe04a1349529caabdb0ab09293a09590fad07b2246bfa4b54` |
| Test limits | Go container: two CPUs, 2 GiB memory, `GOMAXPROCS=2`, package parallelism two and five-minute per-package timeout. |

Receipt files below are outside Git under
`/mnt/data/keepsave-neutral-2026-10-02/`. Synthetic HTTP providers and SMTP senders
were used; no live provider credentials, installed harness sessions or operator
mail/key configuration were read.

## Executed checks

| Check | Result and evidence |
| --- | --- |
| Prior whole-backend race/shuffle and vet | Passed before the last review fixes; `backend-full.log`. This is historical supporting evidence, not the final-source receipt. |
| PostgreSQL final-review run | API, service, repository, MCP OAuth/gateway, harness, broker, runs and runner packages passed. The pending lifecycle CLI fixture failed because it requested an environment absent from the stored project. `backend-postgres-final.log` preserves that failure; it is not a green aggregate gate. |
| Fresh external v2 lifecycle recovery CLI | Passed after the fixture correction: `TestPlatformRecoveryLifecycleFreshDatabaseCLI`, 2.28s, race-enabled package 3.293s. `recovery-lifecycle-final.log`. Other packages in that narrowly filtered command had no matching tests and establish no new coverage. |
| Complete PostgreSQL gate before recovery follow-up | Passed on all shipped migrations 001–033 with race detection: all ten requested packages, including the new scope and lifecycle recovery assertions. `backend-postgres-final-green.log`; captured source file hashes in `backend-validation-source-sha256.txt`. The later delegation change is covered by the final follow-up below. |
| Focused race/shuffle and vet | Config, logging, policy and audit projection tests passed. Vault, authority and diagnostics compiled and vetted but have no standalone test files; their routed/transaction behavior is covered by the PostgreSQL checks. `backend-focused-final.log`. |
| Bounded MySQL legacy compatibility | Passed: MySQL 8.4/Go 1.27.1, race-enabled API package 4.367s. `backend-mysql-legacy-final.log`. This exercises the legacy contract, not journal/platform parity. |
| Whole-backend race/shuffle and vet before recovery follow-up | Passed with pinned Go 1.27.1, two CPUs/2 GiB, shuffled race tests and complete `go vet ./...`. `backend-full-final.log`; the later recovery delegation follow-up is validated separately below. |

The preceding PostgreSQL command executed API, service, repository, vault CLI, MCP
OAuth, MCP gateway, harness packaging, broker, runs and runner packages. API
passed in 64.095s, service in 38.223s, runs in 51.143s and the vault CLI package
in 4.476s. These durations are test receipts, not platform capacity measurements.
The new run-scope contract checks repository/name/reference/installation/
environment drift denial before credential custody and database immutability of
admitted scope/reference. The final API run also includes terminal audit-export
failure erasure, current-parent publication checks, offboarding races and selected
lifecycle restore metadata preview.

The first run with the additional preview assertion expected one bundle record,
but the real router fixture contains a retained baseline `DB_URL` as well as the
new selected record. The assertion was corrected to find the selected immutable
secret ID and check every required lifecycle field. The unchanged implementation
then passed the complete gate. That failed receipt is retained as
`backend-postgres-preview-fixture-failure.log`.

The final ordinary Go suite ran with container networking disabled and a fixed
synthetic hosts entry for `example.com`. Three legacy webhook registration tests
otherwise perform a real DNS lookup; the disconnected first attempt failed
those fixtures and is preserved in `backend-full-no-dns-failure.log`. The fixed
mapping makes their public-address registration assertion deterministic without
disabling SSRF validation or permitting external traffic. The PostgreSQL harness
now preserves that same entry. No production authorization code was changed.

A separate two-process synthetic HTTP journey passed against actual API
processes: cross-instance session revocation, foreign/missing secret denial,
lifecycle metadata without values, vault history/restoration, rotation
continuity and v2 bundle verification. Its initial receipt is `two-api-http.log`;
the main implementation task repeated it on the frozen recovery source and
patched final image in `two-api-http-final.log`. This exercises
shared authority, not queue/worker/supervisor failure recovery or capacity.

One attempted complete repeat was intentionally stopped while the final scope
review introduced another additive migration. Its owned containers/network were
cleaned, and `backend-postgres-held-for-final-scope.log` remains an interrupted
receipt. An interruption is not a passing check.

## Password recovery delegation follow-up

Review after the preceding checks found that a password reset revoked browser
sessions and therefore their MCP children, while legacy API keys/leases remained
usable. The plan requires invalidation of those delegated credentials too. The
identity module now requires an injected recovery revoker; missing wiring fails
closed. The legacy session/repository adapter expires retained API keys, revokes
retained leases and denylists persisted agent-token issuance in the same reset
transaction. It retains immutable credential identities and preserves unrelated
users. The exclusive human subject barrier covers concurrent mint and use.

Four real-PostgreSQL API contracts passed with verbose race evidence in
`backend-recovery-focused-final.log` (API package 9.242s): verified-account/session
recovery, retained key/lease/JTI revocation and required-audit rollback, absent
revoker refusal, and serialized mint/use denial after recovery commit. The first
race fixture rejected HTTP 404 even though the existing concealed lease/resource
denial uses it. The assertion now accepts documented 401/403/404 denials, rejects
every success response and retains exact no-new-lease/no-new-JTI checks. The
initial failure is preserved in
`backend-postgres-recovery-denial-fixture-failure.log`.

The final complete PostgreSQL gate passed on the frozen recovery source, with
all migrations 001–033 and all ten requested packages under race detection:
API 56.233s, service 35.167s, repository 3.402s, vault CLI 3.538s, MCP OAuth 2.894s,
MCP gateway 2.980s, harness 1.077s, broker 2.045s, runs 40.189s and runner 1.391s.
The command exited successfully; `backend-postgres-recovery-final.log` is the
current full PostgreSQL receipt.

The affected packages then passed shuffled race tests and chained `go vet` with
exit status zero: API 13.410s, service 24.946s, repository 1.081s, architecture
2.574s and vault CLI 1.010s. Identity and server have no standalone test files;
both compiled and vetted, with their recovery behavior exercised through the
real PostgreSQL router contracts above. This final check used the pinned,
network-disabled Go container and two-CPU/2-GiB limits.
`backend-recovery-race-vet-final.log` records it;
`backend-recovery-validation-source-sha256.txt` identifies the frozen follow-up
source. The whole-backend ordinary suite preceding this narrowly bounded change
remains supporting evidence, rather than a claimed repeat on the later source.

## Fresh lifecycle recovery assertions

The new CLI acceptance creates a source database and three independently fresh
target databases. It writes two credential revisions, two lifecycle metadata
revisions, rotates the project key and writes a real external encrypted v2
bundle. The source also contains sessions, an API key, organization authority,
an operator grant and reminders specifically to detect accidental authority
recovery.

- Missing and incorrect lifecycle owner maps fail without committing vault
  values, retained keys, revisions or lifecycle rows. The failed isolated CLI
  target retains its locally created custodian/project scaffolding, so a retry
  requires a new genuinely fresh target; no source authority is restored.
- An explicit source-owner mapping creates an isolated, nonlogin custodian and
  personal target project. It preserves declared expiry, provenance and metadata
  revision while assigning responsibility to that new custodian.
- Historical and current values remain readable with retained key dependencies;
  the original source identity cannot read the recovered target.
- Sessions, organization membership/epochs, API keys, leases, platform grants,
  OAuth authority, contacts/proofs/invitations, tool grants/connections/workloads/
  runs, promotion requests and reminder rows are absent from the recovered
  target.
- The recovered audit chain verifies, and a second external v2 backup verifies
  the recovered key/history/lifecycle dependencies.

The child executes the real CLI entry point with an explicitly constructed
synthetic environment. Its successful stdout is parsed as JSON; migration
diagnostics remain on stderr. The corrected fixture uses the project’s stored
`alpha` environment. Neither correction changes recovery semantics or relaxes an
acceptance assertion.

## Limits and remaining release gates

New platform transaction/concurrency guarantees are PostgreSQL-only. SQLite
compatibility tests and synthetic provider tests cannot establish PostgreSQL
locking guarantees or live consent. The existing bounded MySQL legacy receipts
do not establish platform parity.

This receipt does not prove real Google/GitHub consent, authenticated installation
SMTP acceptance, GitHub App interoperability, native Codex/Hermes acceptance,
multi-instance fault/worker failure drills, capacity, production key/storage recovery or
runtime container isolation. The current runner host lacks CPU cgroup delegation
and fails its preflight; no successful Podman isolation exercise is implied.

Independent Security Engineer and Tech Lead approval, applicable remote CI,
installation recovery/provider acceptance, staging rollback and deployment
remain separate gates. New admission flags stay off by default.

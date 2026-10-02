# Core release acceptance ledger

Checked 2026-10-02 (Asia/Bangkok). Scope: the local identity/workspace/reliable-vault
candidate `v1.4.0-rc.1` in `KeepSave-backend-platform`, base `9e2b3ba`.
This is not a release, remote CI result, provider UAT or independent review.

Status meanings: **verified local** means an executed synthetic check described
below; **implemented / pending** means code exists without a completed acceptance
gate; **compatibility** means preserved source/routes with no new end-to-end claim;
**unavailable** means the restricted composition refuses the capability;
**planned** means approved future delivery. Do not promote code presence to evidence.

## Capability inventory

| User surface | Current status | Evidence / remaining gate |
|---|---|---|
| Password registration/login | Verified local PostgreSQL/SQLite | Canonical identities, transactional user/session/audit and failed-login recording; identity tests cover collision, rollback and revocation. |
| Google/GitHub sign-in | Synthetic verification only | State/PKCE, exact callback, provider identity verification and explicit linking; real provider applications and sign-in UAT remain operator work. |
| Account linking | Verified local PostgreSQL/SQLite | Same recent active session, no implicit email linking, provider collision refusal and audit rollback. |
| Human sessions | Verified local PostgreSQL/SQLite | Metadata list, logout, individual revoke, foreign target denial, cross-instance checks, failed-revoke rollback. Actual two running API instances observe revocation after commit; `live-client-drill.json` records the denied revoked token and permitted sibling session. No human refresh flow. |
| Open-registration workspace creation | Verified local SQLite/PostgreSQL | Actual schema, caller-scoped retries, same-name workspaces, atomic administrator membership, failed-audit rollback and no default project/secret. Concurrent PostgreSQL creation passed. |
| Workspace member administration | Verified local SQLite/PostgreSQL | Owner demotion/removal denied; viewer mutations denied; stored organization authority and required audit. PostgreSQL transaction checks passed. |
| Personal-project attachment | Verified local SQLite/PostgreSQL | Stored owner plus destination admin, same-org idempotency, cross-org refusal, source delegation revocation, rollback on audit failure, deleted-project denial. Current membership governs the assigned owner, keys and policy; SQLite anonymous parameter/list/policy regression passed too. |
| Empty-workspace deletion | Verified local SQLite/PostgreSQL | All attached projects, including tombstones, prevent deletion; retry key cannot resurrect a deleted workspace. |
| Project CRUD and archive | Verified local PostgreSQL router | Atomic creation/enrollment, viewer metadata allowed, assigned-owner demotion/removal denied for credential access, retained tombstones and denied delegated access. |
| Secret CRUD and batch | Verified local PostgreSQL router | Authorized values, exact POST batch route, bounded selections, missing-key metadata, live parent scope and current revisions. |
| Secret reference resolution | Verified local PostgreSQL router | Scoped dependencies cannot expand credential access. |
| History and version restoration | Verified local PostgreSQL router | Metadata list, explicit value read, append-as-new revision, stale/concurrent denial and no undelete. |
| Key rotation and verification | Verified local PostgreSQL router | Retained historical/snapshot key references; promotion/rollback after rotation passed. Legacy dialect profile remains compatibility. |
| Environment imports/exports | Verified local PostgreSQL journal | Strict parse/atomic journal import and authorized export. API export deliberately contains permitted values. |
| Templates and application | Verified local PostgreSQL router/journal | Current workspace membership for read/list/apply, admin metadata changes, personal creator isolation, removed-creator/global publication denial, required audit rollback/outbox and revoked admitted session passed in the final PostgreSQL harness. Defaults are ordinary config, not encrypted vault storage. |
| Promotion/diff/approval/reject/rollback | Verified local PostgreSQL router | Exact source artifact, no self-approval, changed source refusal, required audit/history and rotation-safe rollback; kill switch retained. |
| Scoped API keys | Compatibility + M0 local evidence | Existing scope behavior retained; narrow project/environment/keys, parent expiry/revocation and role matrix. Actual scoped SDK batch/write-denial checks passed; broad client parity remains separate. |
| Agent leases/tokens | Bounded local PostgreSQL/SQLite + live two-API verification | Bound current parent/member/session authority, narrower grants, persisted issuance/revocation, required audit and local outbox. All four mutation audit/outbox failure rollbacks and router responses passed. Ten live final-image checks prove exact-parent mint, scoped batch, sibling revoke denial and cross-instance token/lease revocation. These are vault read access, not broker runs. |
| Encrypted backup/verify/preview/selected restore | Verified local PostgreSQL synthetic journey | Encrypted bundle plus metadata diff and revision checks. Fresh isolated project recovery, wrong keys/corruption and authority exclusion passed. The actual trusted CLI external-file drill passed all seven checks, including sentinel-schema refusal before migration. |
| Scheduled backup storage/retention | Verified local PostgreSQL worker; operational drill pending | Private storage, deterministic day/job IDs, catalog and two-phase retention. Default recovery flag false. Never purge keys/history speculatively. |
| Audit-chain continuity and outbox | Verified local PostgreSQL foundation | Persisted serialized head, required transactional events; immutable audited IDs retained. Queue fencing/uncertain effects have synthetic tests. |
| Durable external webhooks | Unavailable | Legacy configuration/delivery routes return core capability unavailable. Trusted production worker not delivered. |
| Policy metadata/access-policy CRUD | Unavailable | Legacy metadata is not enforced policy; explicit refusal until an authorized use case exists. |
| Enterprise SSO/compliance reports | Unavailable | Existing source is not current acceptance. Social sign-in is separate. |
| AI chat/query/providers/rules/drift/anomalies/analytics/recommendations/quotas | Unavailable | No experimental intelligence operation is a core feature. |
| Dependency analysis | Unavailable | No credential-based analysis execution in core. |
| Legacy OAuth issuance/consent/client administration | Unavailable | Old routes retained as refusal adapters; modern resource-bound MCP OAuth is M2. JWKS metadata alone is not interoperable OAuth evidence. |
| MCP registry/catalog/install metadata | Compatibility | M0 ownership/visibility checks proven historically; management metadata does not imply tool execution or client compatibility. |
| API-host MCP register/build/rebuild/execute/config generation | Unavailable | Connector build and execution remain disabled; no API-host process isolation claim. |
| Standards-based `/mcp` | Planned M2 | SDK pin, consent, discovery, invocation/cancellation, resource audience and real Codex contract exercise pending. |
| GitHub App broker/connections/run grants | Planned M3 | Social GitHub login is not a provider connection. No real installation token custody or repository pilot yet. |
| Separate rootless runner/workload enrollment | Planned M3 | No file/network/process-limit or stolen-runner-grant acceptance yet. |
| Private skills and managed Codex profiles | Planned M4 | No immutable registry, approval, compatibility enforcement or Skills over MCP acceptance yet. |
| Self-hosted production bundle/two replicas | Reference core control-host bundle, M5 pending | New same-origin Compose/Caddy structure validated with synthetic files and network-disabled config validation. No deployment, runner or broad two-replica/availability evidence; only the bounded live two-API session-revocation exercise is verified. |
| Operator dashboard/traces/global events/plugins | Compatibility, operator-ID-gated | Explicit local operator grant replaces email-derived administrator authority. New operational UAT pending; replay/plugin mutation unavailable. |
| Applications/favorites | Compatibility | Preserved metadata routes; not an agent harness or deployment integration. |
| Embed widget/config | Compatibility | Existing origin policy retained; widget batch/401/cache UAT pending. |
| Go/Node/Python SDKs, CI actions, Terraform, CLI | Selected local acceptance + compatibility inventory | Actual Python scoped-batch/session (21 HTTP checks), Go/Node clients (four checks each), and five shipped CLI checks passed. Go local module/test job added; published modules and broader widget/CI-action/Terraform/client parity remain separate. |
| Feedback GitHub issue creation | Compatibility, configured external effect | Requires explicit operator configuration; no hidden writes or test provider credentials are used. |
| Health/readiness/metrics | Compatibility | Disposable readonly/nonroot image health/readiness, frontend proxy/header/body-limit checks passed; broader outage/two-instance/production metrics operation remains pending. |
| Static marketing | Previously deployed, outside this backend work | `keepsave.draveniq.dev`; application hostname is planned and not deployed by this slice. |
| Core OpenAPI/generated frontend types | Verified local SQLite/PostgreSQL router | 52 paths/70 operations/87 schemas; embedded source, mounted paths, management/identity/session/vault/history/recovery/import/export/promotion/rotation/key/grant/template response shapes. Extended controls race test and final flagged PostgreSQL/template/grant checks passed; type drift and post-contract frontend builds passed. |
| SQLite/MySQL parity | SQLite targeted checks / bounded MySQL 8.4 legacy exercise verified | Shipped migration bootstrap, password/sessions, scoped CRUD/batch, current org roles, source-key revocation and SQL self-approval denial passed. Core history/recovery 503 and logout/sibling session behavior verified. Versioned vault and durable jobs remain PostgreSQL-only. |

## Executed evidence

All artifact paths below use `/mnt/data/keepsave-core-2026-10-01/` unless an
absolute `/tmp/` path is shown. Checks use synthetic local databases/providers
and disposable resources; they do not establish real provider UAT or deployment.
The base is `9e2b3ba`; the final implementation snapshot is recorded in
`source-manifest-final.json`. Candidate version/documentation packaging is
recorded separately and is not a new image scan.

The `v1.4.0-rc.1` packaging check passed backend health/core identity/control
contract tests with race detection (3.961s), generated-type drift checking,
all 127 frontend tests/23 files, the configured lint command and app/widget
builds. The exact displayed Go SDK example compiled against the local module.
Version labels agree in five places; Python bytecode is excluded. Receipts:
`rc1-backend-contract.log`, `rc1-frontend-validation.log`. This does not replace
image scans, independent human review, remote CI or staging rollback acceptance.

### Backend and dialect checks

- The full backend `go test -race -shuffle=on ./...` passed under Go 1.27.1:
  `backend-race-pass.log`; the later post-template/grant run passed in
  `backend-race-source-final.log` (API 12.774s/service 34.442s). These full runs include local fixtures; PostgreSQL
  flagged integration checks have their separate harness below.
- `go vet ./...` exited 0 with empty `backend-vet-final.log`; `go mod verify`
  reported all modules verified in `backend-mod-final.log`. Race coverage passed:
  crypto 86.4%, auth 93.1%, recorded in `backend-coverage-final.log`.
  The post-template/grant `backend-vet-source-final.log` is also empty with exit 0.
  `core-contract-extended.log` passed the expanded controls router/race test in
  6.379s; generated 52-path/70-operation/87-schema type drift checking passed.
- Ten-second fuzz smoke exercises passed: `fuzz-json.log`, `fuzz-encrypt.log`,
  `fuzz-command.log`, `fuzz-sql.log`. These are bounded smoke checks, not exhaustive
  fuzzing or a performance benchmark.
- SQLite shipped-migration workspace/required-audit/current-membership and
  actual-router OpenAPI checks passed with race detection. Generated-type drift
  checking and TypeScript compilation passed.
- The final expanded PostgreSQL harness passed API 24.403s, service 12.835s,
  repository 1.322s and `cmd/keepsave-vault` 1.577s, with race detection, in
  `/tmp/keepsave-platform-20261002-recovery-final.log`. It covers concurrent
  workspace retries, current assigned-owner membership, viewer metadata 200 and
  credential 403, removed-owner session/key/policy denial, vault/history/
  promotion/rotation/recovery, worker retention and identity/link rollback.
  The command's fresh-target preflight denies concurrent recovery and existing
  non-system tables before migration without changing the sentinel schema.
- MySQL 8.4 `TestPlatformMySQLLegacy` passed in an earlier 4.821s run, then
  passed in 4.248s after final shared wiring in `mysql-legacy-final.log`.
  It bootstraps all shipped migrations;
  covers registration/login/two sessions, API-key CRUD/list/batch denial/missing
  behavior, cross-tenant 403, workspace creation/assignment, viewer metadata 200
  with credential/write 403, editor credential 200 and removal 403, source-key 401,
  self-approved SQL insert/update refusal, recovery 503, logout 401 and permitted
  sibling session 200. This is bounded legacy parity, not PostgreSQL journal,
  history, recovery or durable job parity.

The PostgreSQL script creates unique owned containers/network, no published
DB port, fixed synthetic fixture aliases and explicit TCP readiness. Shared
public pgcrypto initialization precedes per-schema migrations. It never uses an
operator DATABASE_URL and cleans up only its own resources. Failed earlier runs
are retained: `/tmp/keepsave-platform-20261002-bounded.log` (queue fixture),
`/tmp/keepsave-platform-20261002-final.log` (concurrent edit compilation). A prior
cold import failure had no proven cause. Earlier September 28 evidence remains
[historical](../2026-09-28-backend-platform/README.md), not proof of later edits.

`/tmp/keepsave-platform-20261002-grants-templates.log` retains the next run:
service/repository/operator CLI passed; new strict template/router contract,
required-audit rollback/outbox and revoked-session tests passed. One old trusted
vault integration fixture supplied an empty stack, which service validation now
refuses as the handler already did. The fixture was corrected to an explicit
synthetic stack, preserving production validation. The single consolidated
rerun passed in `grant-postgres-validation.log`: API 23.629s, service 27.235s,
repository 1.142s and trusted recovery command 1.273s with race detection.
This includes new template/grant actual-router contract and required transaction
checks. The script exited 0 and removed its own containers/network; the historical
September 28 network was untouched. The earlier failed aggregate is retained.

### Browser, frontend and clients

`browser-acceptance-2026-10-02.md` records the controlled browser procedure on
`http://127.0.0.1:4652` with real disposable PostgreSQL: signup, explicit project,
secret create/edit, historical revision 1 restored as new revision 3, encrypted
backup download/verify/metadata preview, selected backup revision 3 over current 4
restored as revision 5, two-session list/other revoke and current logout.
Google/GitHub correctly show unconfigured status; live provider UAT is pending.
Screenshots are `browser-history.jpg`, `browser-backup-preview.jpg`,
`browser-changed-backup-preview.jpg`, `browser-backup-restored.jpg`,
`browser-sessions.jpg`, `browser-provider-setup.jpg`,
`browser-revoked-session.jpg`, `browser-logout.jpg`; the downloaded synthetic
ciphertext is `browser-encrypted-vault.json`.

The final frontend command
`npm audit fix && npm audit && npm test -- --maxWorkers=2 && npm run lint && npm run build:all`
exited 0: 23 files/127 tests, lint, TypeScript/Vite application and widget builds
passed. Full npm audit found zero vulnerabilities; exact JSON is
`frontend-npm-audit-final.json` and the observed command/result receipt is
`frontend-validation-final.md`. Only development transitive brace-expansion lock
entries changed. Existing React act/optional Three.js size warnings do not imply
failing assertions or a measured performance result. The first unbounded frontend
run stalled; the final bounded two-worker run passed, without claiming a cause.
After the core HelpPage/MCP availability notice, all 23 files/127 frontend tests,
lint and application/widget builds exited 0 in `frontend-post-help-tests.log`,
`frontend-post-help-lint.log` and `frontend-post-help-build.log`. The later
`frontend-contract-build.log` exited 0 for application/widget builds after the
expanded generated contract; type drift checking passed.

Live receipts: `live-client-drill.json` records 21 HTTP checks including the
actual Python scoped batch/write denial and two running API instances observing
revocation (revoked token 401; sibling session 200). `go-sdk-live.json` and
`node-sdk-live.json` each record four passing real-client checks. `cli-drill.json`
records five checks: scoped pull, local child injection, project listing,
transactional import and authorized export. SDK session/cache unit tests and
compilation are separately summarized in `frontend-validation-final.md`.
Published module versions, broad client parity and external CI integrations are
not established by these bounded local exercises.

`uat-final-images.json` records the task-owned preview's two API instances using
the final rebuilt backend image and passing health/readiness, while preserving
the synthetic-database/loopback guard. The 21-check actual Python SDK/two-instance
session drill was rerun and passed against that final image. `live-grant-drill.json`
adds ten live checks: exact-parent token mint/scoped batch, sibling-key revoke
404, individual token revocation 401 on the other API, then a new token denied
401 on the other API after lease revocation. This is bounded legacy vault-lease
compatibility, not broker/runner/Codex UAT or a production topology claim. The
task-owned preview remains `http://127.0.0.1:4652`; provider apps are not configured.

### Trusted recovery and containers

`operator-recovery-drill.json` is the completed seven-check actual CLI drill with
an external encrypted bundle. Good verify succeeds; corrupt/wrong-key verifies
fail; occupied live and sentinel-table targets are refused before migration;
a genuinely fresh PostgreSQL target recovers three revisions/two key versions;
reusing the recovered target is refused. The sentinel table count stays one.
The recovered project has zero sessions, API keys and operator grants. Source
accounts/grants are not imported and no authority is resurrected.

`backend-image-final.log` and `frontend-image-final.log` record image builds;
`image-inspect-final.json` records local image identities/nonroot users.
`container-runtime-final.json` passed synthetic readonly-root, no-new-privileges,
capability-drop/resource-limit, backend health/readiness/current-session,
frontend proxy/SPA/header, exact body-limit and OAuth-query-log checks. The
backend has no shell. These are control-host image checks, not runner isolation,
production TLS, host failures, availability or M5 acceptance. The first container
scan found frontend 40 HIGH/two CRITICAL base vulnerabilities. Replacing its
runtime with pinned `nginx-unprivileged:1.30.5-alpine-slim` produced image
`sha256:dd347aded6bf966e887677fa366d7f880e7d3f3b0e05d2d46e229d73c2fd9557`
with zero Trivy 0.74 findings and passing runtime checks. The old report remains
`trivy-results/frontend-before-base-update.json`. The failed old-base scan
is retained as failure evidence. Final builds after template/grant/HelpPage/core
contract changes passed and were rescanned/runtime-tested:

| Final image | Identity | Findings |
|---|---|---|
| Backend | `sha256:4192a7f762fc5328561e74ca6691e0f101e9317310a439f55ad172f6245ca030` | 0 HIGH/CRITICAL/MEDIUM/LOW; 5 UNKNOWN records, two distinct advisories |
| Frontend | `sha256:9e6ba729617527ce460668a6bc8172624efb8a9a271119240ad9316c637dd457` | 0 reported findings |

`container-security-final.json` and `.md` record exact images, archive hashes,
scanner 0.74.0/published-checksum provenance, vulnerability database timestamp
and matching runtime image IDs. Backend UNKNOWN records retain timezone database
update `DLA-4792-1` and unmaintained openpgp `GO-2026-5932` across four binaries'
module metadata; none were suppressed. Source reachability is assessed separately
by govulncheck below. Trivy is vulnerability-only, not secret/misconfiguration or
universal security certification. Frontend scanning detected runtime OS packages
and no language files, so bundled JavaScript uses separate npm audit. The scanner
warns Alpine 3.24 is absent from its EOL list; no support certification is claimed.
Archive scans were isolated/offline with no Docker socket/host credentials.
Body limits were tested using 3 MiB requests and declared 91 MiB envelopes, not
a full successful 90 MiB recovery transfer. Runtime uses synthetic development
configuration, not production TLS/Vault Transit/provider acceptance.

### Dependency security scan

The original govulncheck 1.1.4 panicked under Go 1.27.1 and did not pass. CI now
pins govulncheck 1.8.0. A supported initial scan reported a called vulnerability;
`x/crypto` v0.56.0, `x/net` v0.57.0, `x/text` v0.41.0 and `x/sys` v0.47.0 updates
were followed by the completed `govulncheck-final.log` (exit 0): zero affected
symbols and zero vulnerabilities in imported packages. The scanner still lists
[GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932), the unsafe/unmaintained
`x/crypto/openpgp` package at module level. KeepSave does not import/call that
package in this scan; the advisory is disclosed and not claimed fixed. This is
a bounded Go source/dependency result, not a clean container or universal
security guarantee. The module minimum is now Go 1.26.0; CI/builds pin 1.27.1.

## Final source checkpoint

The trusted operator recovery drill was repeated using the final backend image
`sha256:4192a7f762fc5328561e74ca6691e0f101e9317310a439f55ad172f6245ca030`:
all seven external-file checks passed. The fresh target contained one project,
three revisions and two key versions, with zero sessions, API keys, platform
grants, leases, agent-token issuance or active promotion approvals. Its receipt
is `operator-recovery-drill.json`; it is a synthetic key-source exercise, not
production Vault Transit recovery acceptance.

All bounded local source, PostgreSQL, frontend and current container gates above
passed. The final dirty-source manifest is
`/mnt/data/keepsave-core-2026-10-01/source-manifest-final.json`, captured after
documentation finishes. Base `9e2b3ba` alone does not identify the uncommitted candidate; use that
manifest and exact final image IDs when reproducing it. Review found lease
best-effort audit and template metadata authority gaps; both were fixed and
their final current-authority/transaction/contract checks passed. Provider UAT,
independent human review, remote CI and application deployment remain unclaimed.

`ci-trivy-provenance.json` records immutable Trivy scan/setup action and nested
source inspection, exact scanner 0.74.0, cache disabled, published-checksum
verification and local YAML/shell checks. This CI configuration evidence is
separate from executed local image scans. No remote GitHub Actions execution,
cosign/attestation verification or credential compromise finding is implied.

## Release gates

1. Consolidated PostgreSQL and SQLite authorization/transaction tests, backend
   race/vet/format/fuzz/dependency checks, frontend test/build/audit and images.
2. Independent Security/Tech Lead review of identity, tenancy, key continuity,
   audit semantics and privileged execution boundaries.
3. Real Google/GitHub sign-in UAT after operator configuration; application TLS,
   exact callback and allowed-origin validation.
4. External encrypted backup plus isolated restore, current/live revision diff,
   key recovery and no authority resurrection. Scheduled storage remains off
   until dependencies are proven.
5. No M2–M5 claims before their specific client/provider/runner/operations gates.

The approved dependency order remains M0 → M1/M2 → M3 → M4 → M5. Core identity
and reliable vault are the current bounded slice, not a restart of settled
provider/harness/topology choices.

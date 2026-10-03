# Harness-neutral candidate acceptance ledger

October3 publication note: the owner authorized committing this source candidate
and publishing through develop to main. The October2 working-tree/test status
below is historical; see the [publication note](../../releases/2026-10-03-source-publication.md)
for the scope and still-pending production gates.

Event/check date: October 2, 2026 (Asia/Bangkok). Source: uncommitted
`feat/harness-neutral-platform-20261002`, based on
`3878e696cbcdeb97615320e5af864eae0885e4df`. This ledger extends the
[core inventory](../2026-10-01-core-release/ACCEPTANCE.md); those dated historical
receipts remain scoped to their original candidate. It does not announce a
release, remote CI, independent review or deployment.

**Local verified** means an executed synthetic journey on the stated runtime.
**Implemented, qualification pending** means code or a reference exists but its
external/operator acceptance is incomplete. **Compatibility** retains existing
contracts without extending support claims. New platform guarantees are
PostgreSQL-only. Default-off flags are not evidence of acceptance.

## Feature inventory

| Surface | Candidate status | Executed evidence and remaining gate |
|---|---|---|
| Password accounts, browser sessions, project/workspace permissions | Local verified: PostgreSQL; shipped SQLite compatibility | Actual router, audit rollback, current session/membership/parent checks, scoped references, owner protection and project tombstones. Two actual HTTP API processes observe committed session revocation. No public-signup global authority. |
| Google/GitHub sign-in and explicit linking | Synthetic verification; real consent pending | Existing PKCE/provider-bound state/stable subjects/Google verification retained; method inventory and recent-auth safety tested. Real independent development/production apps and repeat login/link UAT required. |
| Shared authority and member epochs | Local verified: PostgreSQL | Sorted subject/org/project/session barriers, demotion/offboard/rejoin, lease-mint races, drift denial, current-parent and rollback tests. Ownership changes fail closed; bounded automatic rediscovery retry is not claimed. |
| Contact verification, password recovery, invitations | Implemented; synthetic PostgreSQL contracts pass | Hashed purpose-bound proofs, replay/resend/recovery revocation, fragment clearing, current inviter/recipient authority, ID-only jobs and encrypted delivery material. Live authenticated SMTP and actual receipt remain installation gates; endpoints require recorded SMTP acceptance. |
| Login-method removal and scoped offboarding | Local verified: PostgreSQL router/UI contracts | Actual remaining-method authentication; exact preview/impact/epoch, idempotent receipt, inherited approvals/connections/workloads/grants, old-grant invalidation on rejoin. Personal projects/unrelated organizations/global sessions retained. |
| Vault CRUD, batch, imports, templates, promotion/rollback, history, rotation and deletion | Local verified: PostgreSQL; legacy compatibility separately tested | Every enabled mutation uses retained keys/history/audit/outbox and current authority. Expected-revision restoration, audit continuity, scoped references, stale edits and tombstones remain covered by final PostgreSQL gates. |
| Lifecycle metadata and reminders | Local verified: PostgreSQL and UI contracts | Independent metadata revisions, current-member responsibility, declared dates, 30/7/1-day dedup/current authority and value-free metadata endpoints. Metadata is declared; no upstream expiry inspection. Worker deployment/delivery acceptance pending. |
| Recovery v1/v2 | Local verified: PostgreSQL router and real isolated CLI | External encrypted v2 bundle, wrong/missing owner-map refusal, fresh target, retained revisions/rotated keys, metadata preview and explicit selected current-member mapping. No restored sessions/grants/approvals/results/reminders. Installation-specific recovery material/Transit/storage drill remains required. |
| Safe audit search and export | Local verified: PostgreSQL and UI contracts | Authorized deterministic cursors/filters, repeatable-read snapshot, fenced worker publication, pending/failed/ready states, terminal failure erasure, expiry and current-parent download checks. No values/proofs/provider tokens/raw payloads. Actual worker/fault deployment pending. |
| Read-only readiness / `keepsave doctor` | Local synthetic validation | Operator-ID-gated configured-versus-accepted diagnostics; no hidden external writes/reads. Full installation readiness exercise pending. |
| Delegated OAuth / stateless `/mcp` | Local verified: PostgreSQL and synthetic SDK clients | SDK v1.8.0, both restricted lanes, public S256 clients, exact issuer/resource/callback/Origin, concurrent code redemption, refresh replay/current parent and revocation. Exact Codex/Hermes authentication/negotiation UAT pending. |
| GitHub connections/bindings and structured broker | Synthetic local contracts pass; provider qualification pending | Stored target/ref/environment/install ownership, explicit repository read permissions, token custody, bounded tree/UTF-8 file, denial/canary/timeout/truncation/path tests. No live GitHub App used. Disclosed check requires explicit execution. |
| Portable profiles and client-bound runs | Local verified: PostgreSQL and management contracts | Independent exact digest/epoch approvals, preparing-to-commit activation, immutable admitted scope, binding drift, parent narrowing, expiry, revocation, cross-client denial and budgets. Minimal fixed-limit pilot profiles; no general policy editor. |
| Asynchronous operations and receipts/results | Local verified: PostgreSQL | Stable request keys, conflict, fences/one-use tickets, duplicate delivery, explicit retry/cancel, encrypted expiring results and separate outcome/delivery authority. Kill switches deny new work while preserving permitted status/receipts/cancel/revoke. |
| Private runner listener and enrolled supervisor | Synthetic router/preflight tests; host qualification pending | Verified TLS1.3 client chain/exact certificate fingerprint, no forwarded-header authority or public route exposure. Current host lacks delegated CPU cgroup and preflight refuses it. No successful rootless isolation claim. |
| Skills/native packages/export/check/unpack | Local verified: parser/renderer/CLI contracts | Immutable single instruction-only `SKILL.md`, manifests/digests, native candidate configuration, safe private extraction, tamper/symlink/traversal/no-overwrite refusal and known/unknown compatibility. Rich reference-file source trees are follow-on work. Native discovery/use/admin enforcement pending for each exact client. |
| CLI/SDK/widget and frontend contracts | Selected contracts verified; broader compatibility retained | Sole `core.json`, routed response contracts, generated types; actual `keepsave-vault` and harness commands. Existing SDK/widget fixtures retained and builds pass; prior dated core client UAT is not new provider/harness acceptance. |
| Self-hosted control/runner bundle | Built and structurally checked reference | Pinned API/frontend images; Compose/private-listener config validation; additive migrations001–033; local two-API restart/revocation journey. Production TLS/Transit/private storage, worker/supervisor faults, coordinated rollback and measured capacity remain gates. |
| Legacy webhook execution, AI/analytics, enterprise SSO/compliance, metadata policy enforcement, old OAuth and API-host MCP execution | Unavailable in restricted profile | Source/compatibility fixtures preserved. No claim of safe executable behavior or assessed compliance. |
| Applications/favorites/operator plugins/older integrations | Compatibility inventory | Existing local metadata/explicit operator gates retained. No extension of operational acceptance from source presence. |
| External secret delivery, arbitrary builds/scripts, model keys, device attestation, marketplace and multi-region | Deferred | No enabled endpoint or claimed enforcement. |

## Executed aggregate checks

The detailed [backend receipt](BACKEND.md), [identity/team receipt](IDENTITY_TEAM.md)
and [protocol/packaging receipt](../../design/2026-10-02-harness-neutral-platform/PROTOCOL.md)
contain commands, assertions, failures and limits. Raw local evidence is in
`/mnt/data/keepsave-neutral-2026-10-02/`, outside Git; it contains no production
credentials. Database fixtures/HTTP providers/SMTP senders are synthetic.

- Final PostgreSQL race gate: all ten packages pass on migrations001–033,
  including lifecycle recovery CLI, immutable scope/drift, revocation, offboarding,
  audit publication failures and selected restore preview. Bounded MySQL8.4
  legacy race check passes; it establishes no new-platform parity.
- Frontend aggregate:34 files/161 tests pass on pinned Node24.21.0; final affected
  pages pass nine tests including a new delayed reset/account-switch case.
  See the [frontend/browser receipt](FRONTEND.md). Configured lint,
  TypeScript application build and widget build pass. Existing ESLint rules
  cover the embed SDK; this is not a full application lint/a11y audit.
- Follow-up project-route checks verify direct lifecycle/audit navigation never
  requests values and late responses from a previous project are ignored.
  The browser's earlier `secret.read` events came from the explicitly opened
  Secrets view; no metadata read leak was reproduced.
- Visible local browser: disabled unconfigured social login, password sign-in,
  account method/session metadata, masked vault view, lifecycle editing controls
  and safe audit records inspected. This is synthetic local browser acceptance,
  not provider consent, actual SMTP or native harness acceptance.
- Module verification and bounded policy/crypto fuzz pass. `govulncheck` reports
  zero reachable/imported-package findings and one uncalled required-module
  advisory. No blanket clean dependency inventory is claimed.

- Final API and frontend images build and run as nonroot users. The frontend
  PCRE2 finding from the refreshed database is fixed and its final scan reports
  zero findings. The API retains the timezone-data update advisory and unimported
  OpenPGP module records. See [exact image/scan receipts](OPERATIONS.md).
- Two final API processes pass the synthetic HTTP session-revocation, permission,
  history/restore, rotation and v2 verification journey after draining the older
  processes. Final frontend HTTP checks confirm SPA/API behavior and refusal of
  public runner routes. This is bounded functionality evidence, not capacity.
- Generated types pass the pinned Node24.21.0 check after final contract review.
  The source manifest identifies659 backend/frontend/deploy/script files on the
  uncommitted candidate; the exact digest is recorded in [OPERATIONS](OPERATIONS.md).

Build warnings about large existing frontend chunks are retained; no performance
claim follows. Full backend race/shuffle/vet preceded the recovery follow-up;
the final ten-package PostgreSQL gate and affected-package follow-up establish
that later change, with exact scopes in [BACKEND](BACKEND.md).

## Release gates

Do not advertise the complete platform as supported until G0–G6 exit gates pass.
Outstanding: independent Security Engineer and Tech Lead signatures; remote CI;
real Google/GitHub and authenticated SMTP acceptance; exact Codex0.153.3 and
isolated Hermes0.21.5 qualification; disposable live GitHub App A-success/B-denial
and revocation; actual separate-host Podman/cgroups/seccomp/Unix-relay isolation;
production key/TLS/storage recovery; full worker/supervisor/database/Transit
failure and upgrade/compatible rollback drills; measured fixed-hardware capacity.

The installed Hermes0.21.3/provider/sessions/memory and published landing were
not changed. The application domain is a deployment target. No commit, merge,
tag, push, provider enrollment or production deployment occurred in this task.

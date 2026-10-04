# 11. Testing and acceptance

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

The [current ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
extends dated [core evidence](../validation/2026-10-01-core-release/ACCEPTANCE.md).
Every receipt identifies executed scenarios, source/runtime scope, failures/skips
and limitations. Code, generated contracts, a workflow or metadata-only package
check is not evidence of real provider/harness/isolation/production acceptance.
October 3 main/develop remote CI runs were checked **failed** on October 4; this
must not be replaced by the earlier in-progress observation or local green tests.

## Reproducible local gates

| Check | Scope |
|---|---|
| `scripts/test-platform-postgres.sh` | Unique disposable database/network, complete migrations 001–033, race-enabled actual router/domain/recovery fixtures. Ignores operator DATABASE_URL and cleans only owned resources. |
| `scripts/test-legacy-mysql.sh` | Bounded MySQL 8.4 legacy identity/session/scoped-vault/workspace-role compatibility; no platform parity claim. |
| Backend race/shuffle and vet | Shared invariants, rollback/concurrency and adapter contracts; architecture boundaries and safe-error checks. |
| Bounded fuzz/module/dependency checks | Policy/crypto malformed input and dependency reachability; crashes/unused-module advisories remain explicit. |
| Frontend tests and build:all | App contracts plus separate widget output. Configured lint covers embed SDK, not a full app lint/accessibility audit. |
| OpenAPI/types | Actual handler response tests against `core.json`, generated frontend type drift check. |
| Container builds/scans | Exact image digests, nonroot execution, current scanner database and retained advisories. |

Do not run fixtures against an operator database or expose a test database port
by convenience. One source manifest ties the October 2 executed receipts to
backend/frontend/deploy/scripts. Later docs-only edits do not create a new
backend execution receipt.

## Required failure matrix

- Identity: stable subjects/email collisions/linking, last-method safety, state/
  proof replay, session revoke, resend/reset races, missing revocation port and
  database failure. Real Google/GitHub/SMTP journeys are separate exercises.
- Authority: foreign/missing IDs, viewer values, parent widening/sibling keys,
  membership/epoch changes, project assignment/tombstones, offboard/rejoin and
  current resource discovery under ordered barriers.
- Vault: all mutation paths, expected revisions/concurrent rotation, required
  audit/outbox rollback, source-bound promotion, snapshot continuity, corrupt/
  wrong-key bundles, v1/v2 compatibility and fresh isolated owner-mapped recovery.
- OAuth/MCP: exact callback/resource/issuer/client/Origin, S256, concurrent code
  redemption, refresh replay, parent/session revocation, both protocol lanes,
  bounded schemas/full wire envelope and explicit cancellation.
- Broker/operations: wrong repo/ref/action, binding/artifact drift, client-bound
  handles, budget races, idempotency/fencing/tickets, canaries, publication failure,
  result expiry/revocation and honest uncertain/admitted outcomes.
- Runner: forbidden files/keys/engine socket, IPv4/IPv6/DNS, CPU/memory/PID limits,
  relay tampering, teardown and crash recovery on a qualified separate host.

## Evidence already recorded and remaining gates

The final local PostgreSQL race gate passed ten packages; full backend
race/shuffle/vet preceded the last recovery follow-up, and final affected checks
cover that change. Frontend aggregate passed 34 files/161 tests; final affected
pages passed nine tests including a new reset/account-switch case, rather than a
new full 162-test aggregate. Native packaging tests passed 14 top-level/35 cases;
they did not execute Codex or Hermes. Two real local API processes passed bounded
HTTP session-revocation/permission/history/restore/rotation/recovery checks.

Final frontend scanning fixed the refreshed PCRE2 finding; the API retained
timezone-data/unimported OpenPGP records. Govulncheck reported zero reachable/
imported-package findings and one unused required-module advisory. Read
[backend](../validation/2026-10-02-harness-neutral-platform/BACKEND.md),
[frontend](../validation/2026-10-02-harness-neutral-platform/FRONTEND.md),
[operations](../validation/2026-10-02-harness-neutral-platform/OPERATIONS.md) and
[protocol](../design/2026-10-02-harness-neutral-platform/PROTOCOL.md) receipts for
exact scopes. Never summarize these as a blanket clean security inventory.

Release still requires exact-revision remote CI, independent Security Engineer/
Tech Lead review, real Google/GitHub/SMTP consent/delivery, exact Codex/Hermes native
use, disposable GitHub App A-success/B-denial/revoke, actual runner restrictions,
production TLS/Transit/key/storage recovery, full worker/supervisor/database faults,
compatible upgrade/rollback and measured fixed-hardware capacity. Synthetic CI
and separately authorized real/model-assisted acceptance remain distinct.

# 11. Testing and acceptance

Part of the [system documentation](README.md), reconciled 2026-10-02.

The [acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md) records
commands, dates, source scope, failures and remaining gates. A generated contract,
workflow file, implemented feature or synthetic fixture is not proof of live
provider integration, deployment, runner isolation or production capacity.

`./scripts/test-platform-postgres.sh` creates a unique disposable container and
network with fixed fixture aliases, no published database port, explicit TCP
readiness and cleanup of only its resources. It ignores an operator DATABASE_URL,
initializes shared public pgcrypto before per-test schemas and runs API/service/
repository platform+identity+social and trusted recovery-command PostgreSQL fixtures with race detection.
SQLite compatibility fixtures apply shipped migrations too. MySQL compatibility
passed its bounded disposable 8.4 harness with shipped migrations and legacy
identity/session/scoped-vault/workspace roles;
new journal/recovery capabilities remain PostgreSQL-only.

Meaningful authorization tests include cross-tenant/stored ownership, viewer
value denial, assigned-owner demotion/removal, sibling/parent keys, narrowed
lease expiry/scope, revoked sessions and fail-closed authority outages. Mutation
tests force required audit/outbox rollback, private template creator-removal/admin
denials, concurrent restoration and workspace retry
races. Vault tests cover current/history/snapshot key continuity, strict imports,
promotion exact-source approval and rotation-safe rollback, corruption/wrong
keys, isolated recovery and no authority resurrection. Maintenance tests cover
restart deduplication, fencing and audited retention failures.

The core OpenAPI test uses the actual router and validates identity/workspace/
secret/history/recovery/session/import/export/promotion/rotation/template/key/
grant response envelopes. Frontend wire types are
generated from the same source; CI checks drift. SDK/widget/CLI compatibility is
an additional supported-client acceptance task, not assumed from route existence.

Retain backend race/shuffle/vet/format/fuzz/coverage/dependency gates; run frontend
tests/build/audit when integration contracts change. Preserve container checks,
SAST, CodeQL and Robot fixtures on develop/main changes. Record a scanner crash
as a tool failure, not a clean security result. Tests may be broadened after new
changes or failures; do not endlessly rerun a passed suite without a changed risk.

Before release require independent Security Engineer/Tech Lead review, real
Google/GitHub UAT after operator configuration and a production recovery/key/
storage drill. The local seven-check external-file trusted recovery CLI exercise
passed; it does not prove deployed storage. M2 adds actual supported Codex protocol/OAuth exercises;
M3 adds real runner restrictions/provider canaries; M4 adds exact artifact/
requirements compatibility; M5 adds restart/outage/upgrade/two-API and measured
capacity/recovery. Synthetic CI fixtures stay separate from those UAT exercises.

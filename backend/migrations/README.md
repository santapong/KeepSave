# KeepSave migrations

Source audit: October 4, 2026. The canonical platform database is PostgreSQL.
See [data model](../../docs/system/04-data-model.md),
[architecture](../../docs/ARCHITECTURE.md), [cutover/recovery](../../docs/system/10-operations.md)
and [acceptance ledger](../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

[`repository.RunMigrationsFS`](../internal/repository/migrate.go) selects the
stored driver's directory, sorts SQL filenames and records applied filename
versions in `schema_migrations`. On-disk and embedded sources share this path.
An already recorded version is skipped; this registry is not permission to edit
an applied migration or claim checksum-validated migration history.

| Directory | Current scope |
|---|---|
| `postgres/` | Migrations 001–033 for the current identity/vault/harness-neutral platform candidate. Real PostgreSQL transaction/concurrency evidence exists, with installation/release gates still open. |
| `sqlite/` | Shipped local/legacy compatibility. New PostgreSQL-only entries can be explicit no-ops; equal registry counts do not imply feature parity. |
| `mysql/` | Bounded MySQL 8.4 legacy identity/session/scoped-vault/workspace-role exercise. Journal/recovery/run parity is unproven. |
| Root historical `.sql` | Predialect fallback only if the selected dialect directory is absent; not new migration source. |

## Evolution and transactions

Use the next monotonically increasing three-digit prefix for additive owning-slice
changes. Never renumber/rewrite applied files. New tables/relationships must keep
stored tenant/project ownership and immutable-journal constraints; a no-op adapter
must be accurately marked unsupported at runtime. Do not promise MySQL parity
because a matching filename exists.

PostgreSQL applies each migration body and registry insert in a transaction.
SQLite/MySQL execute split statements through dialect adapters; MySQL DDL can
implicitly commit, so this is **not atomic MySQL DDL**. Existing legacy adapters
preserve applied file identities while handling driver syntax. The runner does
not establish a global multi-instance migration contention guarantee. Coordinate
one migration writer during cutover; concurrent startup/upgrade acceptance remains
a release gate.

Drain incompatible API/worker writers, preserve external encrypted backups and
independent key material, apply additive migrations and explicitly baseline old
active vault projects. The labeled baseline does not invent history. Deploy
compatible recovery readers before new bundle writers; do not resume old session/
vault writers after enrollment or use unsupported binaries as rollback.

## Validation

From the canonical repository root (`/mnt/data/company/apps/KeepSave`):

```bash
bash scripts/test-platform-postgres.sh
bash scripts/test-legacy-mysql.sh
```

These create only unique disposable fixtures, ignore operator DATABASE_URL and
publish no database port. Retain shipped SQLite migration fixtures too. Source
checks/registry counts are not fresh execution; the dated ledger records actual
migrations 001–033 race/restore tests. Production Transit/key recovery, migration
contention and compatible upgrade/rollback still require operator acceptance.

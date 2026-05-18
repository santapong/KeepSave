# Migrations

Per dialect. `repository.RunMigrations` (`internal/repository/migrate.go`)
auto-selects the subdirectory matching the detected DB driver:

| Dialect    | Directory  | Status                                           |
|------------|------------|--------------------------------------------------|
| PostgreSQL | `postgres/`| **Canonical.** Production target. Complete.      |
| MySQL      | `mysql/`   | Parity-maintained. Community-supported.          |
| SQLite     | `sqlite/`  | Tests + embedded. Rarely used in production.     |

Migrations run transactionally at startup. The `schema_migrations` table
records what has already been applied so reruns are no-ops.

The flat `.sql` files at the top of `backend/migrations/` (e.g.
`001_initial_schema.sql`, `006_application_dashboard.sql`,
`008_phase15_quotas.sql`) predate the per-dialect split and are kept as a
historical fallback (`migrate.go` falls back when the dialect subdir is
absent). They are **not** the source of truth - add new migrations only
under the three dialect subdirectories.

## Conventions

- Numeric prefix, three digits, monotonically increasing.
- One feature or schema concern per file.
- Use `CREATE TABLE IF NOT EXISTS` / `ALTER TABLE ... IF NOT EXISTS` so
  partial runs do not block re-execution.
- Cross-dialect: if a feature is dialect-specific (e.g. JSONB), provide a
  parity file in the other two dialects even if it only contains
  `SELECT 1` - this keeps the schema_migrations row counts identical.
- Test against an in-memory SQLite plus a docker-compose Postgres before
  merging.

## Why MySQL is supported

KeepSave runs primarily on Postgres (Neon for managed, RDS / Cloud SQL
otherwise). MySQL parity exists for integrators with an existing MySQL
estate and is not gated by the cutover plan - see
`docs/DEPLOYMENT_PLAN.md` §3 (UAT tier picks).

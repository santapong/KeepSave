# KeepSave–Seidr-style HTTP fixture

Source audit: October 4, 2026. This retained Go tester mimics the vault HTTP calls
associated with the historical Seidr provider contract; it does **not** start the
Seidr runtime or qualify a currently installed Seidr release. See the
[documentation hub](../../../docs/README.md), [integrations](../../../docs/INTEGRATIONS.md)
and [current ledger](../../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## Fixture journey

The tester uses synthetic credentials to register/login, create a personal project,
write an `alpha` marker, issue a project/environment-scoped read key and compare
the returned value. Human login now creates a database-revocable 24-hour browser
SID; no legacy signature-only human token is accepted by the application. API
keys remain separate narrowed credentials. A successful roundtrip would establish
that fixture's HTTP path, not exhaustive crypto correctness or all environment
denial scenarios.

The tester does not check Seidr caches/circuit breakers/SLOs, runtime rotation,
Google/GitHub consent, new MCP delegated OAuth, broker token custody, native skills
or actual runner isolation. Future runtime work is historical context in the
[archived security audit](../../../docs/archive/SECURITY_AUDIT.md); it is not
completed acceptance.

## Current execution blockers

The retained Compose healthcheck invokes `CMD-SHELL`/`wget` against the current
distroless image, which contains neither a shell nor wget. The tester's
`createProject` expects top-level `id`, while the current core endpoint returns
`{project:{id}}`. These findings require fixture repair and an exact-revision
rerun before this old Compose is described as passing. No source fixes or live
execution occurred in this documentation audit.

Use the current isolated PostgreSQL suite from the canonical repository root:

```bash
bash scripts/test-platform-postgres.sh
```

The retained Compose specification can be inspected without starting services:

```bash
docker compose -f tests/e2e/seidr/docker-compose.yml config --quiet
```

After repair, its tester exit code is intended to propagate via
`--abort-on-container-exit --exit-code-from tester`. The old fixture publishes
API 18080 and uses fixed project/container identities; do not confuse it with the
unique isolated platform suite or use it on an operator database. Record actual
source/image/runtime/result evidence and remove only fixture-owned resources.

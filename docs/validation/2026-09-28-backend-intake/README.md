# Backend intake and secret-ID authorization — 2026-09-28

Owner direction: frontend accepted; focus next on backend development and features. The current turn made no frontend changes. Recovery, integration and team feature priorities were offered; the feature sequence in [the intake](../../research/BACKEND_NEXT_2026-09-28.md) remains a proposal pending that preference.

## Implemented and verified

An API key restricted to Alpha could previously address a Production secret by ID; history routes also omitted key-name glob enforcement. A metadata-only lookup and guard now apply those existing restrictions against the stored secret before the handler runs. This is a Type-2 correction, with no schema, crypto, token format or permission expansion. Threat delta is recorded in `docs/THREAT_MODEL.md`.

The regression fixture uses real embedded migrations, encrypted random synthetic secrets, API keys, human JWTs and `SetupRouter`, with a disposable SQLite database per case. It does not use the preview's database or production data.

- Original backend `go test ./...`: passed.
- Before the fix: 9 of the 23 new cases failed with unexpected 200/204 responses. These include cross-environment read/update/delete and out-of-scope history reads.
- After the fix: all 23 cases pass, covering permitted operations too. Denied operations do not change values or emit mutation audit rows; permitted writes/deletes emit their existing canonical events.
- Final `go test -race ./...`: passed.
- Final `go vet ./...`: passed.
- `gofmt` applied to the three new Go files; `git diff --check` passed.

Executed using cached official `golang:1.25`, CGO-enabled, two CPUs, existing `keepsave-go-mod` and `keepsave-go-build` caches. Logs: `/mnt/data/keepsave-backend-baseline-2026-09-28/{tests,scope-before,scope-after,race,vet}.log`.

## Limits and handoff

PostgreSQL/MySQL runtime checks and an independent security review were not performed in this bounded pass. Repository integration review remains pending. No commit, merge, push, deployment, real secret access, or credential configuration occurred. The running frontend preview/backend container was not restarted; the new guard is in local source and verified test execution, not in that existing preview binary.

Recovery findings are code-inspection evidence: normal writes do not call `CreateVersion`; current backups contain metadata rather than recoverable secret contents; rotation currently visits live secrets, so retained history/snapshots must be included in a future recovery design. No new restore capability is claimed.

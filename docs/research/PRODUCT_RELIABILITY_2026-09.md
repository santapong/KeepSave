# KeepSave: product, persistence, efficiency, and research

Implemented and checked locally on 2026-09-08–09 (Asia/Bangkok). Baseline: clean `main` at `15d84aa`, matching the local origin ref; no remote fetch. Work branch: `feature/realistic-product-and-efficiency`. This initial report describes local execution, not production deployment or remote CI. Its baseline, frontend test count, static-landing observations, and dependency snapshot are historical; see the later design passes and develop integration record in [the design specification](../design/BLACK_HOLE_LANDING.md).

## What changed

**Product.** The public root now has a complete responsive landing page, an explicitly labeled interactive product example, working registration/login links, and links to the actual CLI/SDK documentation. The authenticated project directory and vault show real project metadata. Fabricated traffic, health scores, organization names, activity tickers, and login options were removed from these screens. Empty, loading, error, retry, import, and save states now reflect API results. Dark/light colors and mobile navigation use the same product identity.

**Saving.** Secret mutations wait for the server, block competing edits, clear sensitive inputs after success, and reload the saved data. Failed loads have a retry action. Stale responses cannot replace the currently selected environment. Editing masks the value. Import counts tolerate the backend's nullable created/updated/skipped arrays, fixing a case where a successful import was reported as failed. Protected deep links still reach their original page after login.

**Backend.** Real-binary testing exposed two SQLite failures missed by handcrafted unit schemas: an invalid `schema_migrations` timestamp default and scanning existing TEXT timestamps directly into Go time values. Startup now uses valid SQLite syntax; a shared scanner handles SQLite strings and native driver timestamps without rewriting the schema. This includes nullable OAuth expiry and MCP sync timestamps. SQLite foreign keys, WAL, and busy timeout are attached to every physical connection, including replacements. Malformed options fail closed. The drift scheduler closes its result set before nested queries, avoiding deadlock with one connection.

**Resources.** Database pools now default to 10 open / 2 idle connections instead of 25 / 5, with validated environment overrides and Docker Compose wiring. SQLite keeps one connection. Trace retention overwrites a bounded ring instead of repeatedly appending/reslicing the retained history. Decorative continuous animations and background blur were removed from the main workspace. No aggregate secret-decryption queries or additional runtime service were added for the overview.

Scope classification: Type-2 product/persistence compatibility; Type-3 resource implementation. Existing crypto algorithms, key hierarchy, token authority, and promotion rules are unchanged. Threat delta: one static public UI route, with no new privileged API. Audit assertions cover the exercised create/update/import/delete paths. Three independent read-only reviewers checked frontend behavior, persistence, and resources; their concrete findings were fixed and rechecked. Rollback is the feature diff; no data migration rollback is needed.

## Executed checks

The repeatable [persistence script](../../scripts/verify-persistence.py) starts the actual binary from a temporary directory, using embedded migrations. It generates credentials and probe values in memory and removes its disposable databases/containers afterward.

| Check | Observed result |
| --- | --- |
| SQLite core API | Register, project create, secret create/update/read, import create/skip/overwrite, process restart, existing token, new login, read-back and delete passed |
| PostgreSQL 16 core API | The same lifecycle passed against a disposable local container |
| Isolation | Anonymous access returned 401; another account returned 403; Alpha/UAT/PROD remained separate |
| Audit and storage | Create/update/import/delete events present; SQLite integrity and foreign-key checks passed; probe values absent from SQLite DB/WAL and API logs |
| Visible browser, real API | Register → create project → create/edit secret → restart API → reload/read-back passed; `.env` import displayed success and remained visible after reload |
| Browser privacy/layout | Saved value remained masked and absent from browser storage; 390px/mobile and 1440px/desktop checked; light/dark checked; no document overflow; zero running animations at idle on inspected landing/vault |
| Frontend | 77 tests passed; ESLint, TypeScript, app and embed-widget builds passed; compatible lockfile security updates left npm audit at zero reported vulnerabilities |
| Backend | Full suite passed; full race/shuffle run passed; `go vet` passed; CGO-enabled and CGO-disabled binaries built |

Signing-key regression tests additionally exercise reload, verification overlap, retirement, corruption, wrong wrapping keys, and unavailable key storage. Auth coverage reached 91.8% (required 90%); crypto coverage was 86.4% (required 85%). Go formatting is clean. All five CI fuzz smoke targets passed for 15 seconds each: encryption round-trip, decryption robustness, SQL splitting, canonical JSON, and MCP entry-command validation. Logs are in the evidence bundle.

Build environment: Linux x86-64, Intel i3-9100, Go 1.25.11 through cached `golang:1.25` Docker tooling, Node 26.8.1. Backend build/test containers were limited to two CPUs. This host has no Go executable. Node 26's native webstorage conflicts with the project's jsdom tests; tests passed with `NODE_OPTIONS=--no-experimental-webstorage`. CI uses Node 20. Restore the lockfile with `npm ci` before testing stale local dependencies.

Evidence bundle: `/mnt/data/keepsave-review-2026-09-09/`. It contains logs, screenshots, and measurement JSON, without probe values or credentials. The runtime preview uses separate disposable local data.

### Measured resource use

API process only, `GOMAXPROCS=2`, `GOMEMLIMIT=128MiB`, remote DB pool 4 open / 1 idle; SQLite remains one connection. Each database run used five idle seconds followed by 100 authenticated secret reads at concurrency four. PostgreSQL used a separate container capped at one CPU and 256 MiB.

| Measurement | SQLite | PostgreSQL 16 |
| --- | ---: | ---: |
| Idle API RSS | 23.40 MiB | 22.19 MiB |
| Idle CPU during 5-second sample | 0.00% of one core | 0.00% of one core |
| API RSS after reads | 26.34 MiB | 25.65 MiB |
| 100-read elapsed time | 0.081 s | 0.482 s |
| Read p95 | 9.00 ms | 60.04 ms |
| API CPU time for reads | 0.08 s | 0.29 s |

These are short local observations, not production capacity limits. RSS is sampled, not peak memory. Figures exclude PostgreSQL, browser, build tooling, and MCP child processes. A zero CPU sample means no measurable CPU ticks during that interval. There is no reliable before/after whole-API comparison because the baseline SQLite binary failed startup.

The isolated warm trace-retention benchmark retains 10,000 spans and runs three repetitions. The original append/reslice approach took 222.5–270.3 ns/op and 638 B/op of amortized allocation; the ring took 38.42–38.96 ns/op and 0 B/op. That is about 5.7–7.0× faster **for retention only**, not the complete request path. Span creation, attributes, and snapshot copies still allocate.

For a small deployment, start by measuring a 4/1 database pool and leave CPU scheduling aligned with the container budget. Treat `GOMEMLIMIT` as a soft Go-runtime limit with headroom for native memory and children. Setting it too low can increase collection work; it is not an RSS cap. [Go GC guide](https://go.dev/doc/gc-guide)

SQLite's foreign-key setting is connection-local, and WAL permits readers alongside a writer while retaining a single-writer limit; those properties explain the connection and scheduler fixes. [SQLite foreign keys](https://www.sqlite.org/foreignkeys.html), [WAL](https://www.sqlite.org/wal.html)

## Research mapped to this repository

This is a focused primary-source review. Methods/evaluation sections and relevant limitations were read; no paper's experiments were reproduced. The three papers were saved to the alphaXiv folder **KeepSave — secrets and tool security** and **Reading**, not marked completed. [Open library](https://www.alphaxiv.org/bookmarks)

### ACLE-MCP — execution-time capability checks

[ACLE-MCP: Attested Capability Leases for Execution-Time Trust in Remote LLM Tool Use](https://arxiv.org/abs/2609.02690), v1, September 2026. Read the lease/execution-gate design, agent-suite evaluation, and runtime-cost limitations. The mechanism binds authorization to the specific sender, workload, operation, parameters, and fresh execution evidence. Its controlled extension uses four benign and six misuse families, ten repetitions across five modes: 500 observations. Full ACLE-MCP preserved benign behavior and blocked the tested attacks; warm normal-request p95 rose from 12.20 to 15.34 ms. Those numbers use simulated verification components and a small controlled suite, not a production hardware-attestation benchmark.

**KeepSave inference:** existing lease-bound agent tokens already constrain project, environment, secret keys, expiry, and revocation (`internal/auth/auth.go`, `handlers_agent.go`, `agent_token_confine_test.go`). Extend those boundaries with an approved tool/manifest digest checked at dispatch. Do not build another token service. Hardware attestation and new one-use authorization contracts need a separate design/ADR.

### FlowGuard — distinguish signals from demonstrated leaks

[FlowGuard: From Signals to Evidence for MCP Security Detection](https://arxiv.org/abs/2607.14754), v1, July 2026. Read the evidence-adjudication method, fixture construction, evaluation tables, and validity limits. FlowGuard separates reflected input, normal defensive behavior, and evidence of an actual backend-origin violation. Its benchmark has 1,880 executable cases across five categories. Credential detection reports precision 0.9722 and recall 0.7778: high precision still leaves misses. Its evaluation uses Qwen3-235B-A22B-Instruct, up to five probe rounds and a 900-second run timeout. Semantic tool-poisoning findings do not prove that an agent obeyed the injected instruction or exfiltrated data.

**KeepSave inference:** expand the existing gateway subprocess tests with randomly generated canaries, valid JSON-RPC echoes, encoded/split outputs, oversized outputs, and failed authorization. The current gateway has exact-value redaction, a 4 MiB stdout cap, and a 30-second process timeout (`handlers_mcp_gateway.go`). Measure actual disclosure and preserved benign behavior separately. Run deterministic checks offline; an LLM scanner would add cost and latency to the normal request path.

### ACE — keep authority outside tool-controlled content

[ACE: A Security Architecture for LLM-Integrated App Systems](https://arxiv.org/abs/2504.20984), v3, September 2025, NDSS 2026. Read the planning/execution split, capability and information-flow controls, evaluations, and ASB adaptation caveats. ACE builds an immutable abstract plan from trusted context, maps it to concrete apps, and enforces boundaries during execution. It evaluates INJECAGENT, Agent Security Bench, and a tool-usage benchmark with several models. Some adapted tools return constant strings, so those results do not establish safe real-world side effects for KeepSave's gateway.

**KeepSave inference:** keep tool descriptions/results as data. Add explicit permission/effect declarations to reviewed tool registrations and compare requested effects with existing middleware authority. Test that malicious tool output cannot grant PROD access or widen an agent lease. KeepSave is a credential gateway, so importing ACE's planner is unnecessary.

## Useful tools and bounded next work

| Tool | Practical use here | Cost and decision |
| --- | --- | --- |
| [Go pprof](https://pkg.go.dev/net/http/pprof) | Profile a bounded load against an isolated local binary to identify allocation and CPU hotspots | Run on demand; do not expose profiling endpoints publicly |
| [Gitleaks](https://github.com/gitleaks/gitleaks) | Add a redacted repository/CI secret scan, including tests and generated artifacts | Offline/CI work; never store unredacted findings |
| [MCP Inspector](https://github.com/modelcontextprotocol/inspector) | Verify tools/list, schemas and calls for a registered disposable MCP server | Developer diagnostic, not another production daemon |
| [OpenTelemetry memory limiter](https://github.com/open-telemetry/opentelemetry-collector/blob/main/processor/memorylimiterprocessor/README.md) | Bound collector memory if exporting traces becomes necessary | Defer a collector until there is a measured need; the in-process ring is sufficient for the current local scope |
| [SPIRE workload identity](https://spiffe.io/docs/latest/spire-about/spire-concepts/) | Evaluate workload identity for a real multi-host deployment | Defer until a pilot demonstrates that service identity/attestation is the bottleneck |

Recommended order below is a project judgment, not a measured paper result. These are proposals; they were not implemented in this change.

| Priority | Idea | Small experiment and acceptance condition |
| --- | --- | --- |
| 1 | Automated restore and corruption drill | Restore an isolated copy with the intended keys, authenticate, compare synthetic values/audit events, then corrupt ciphertext and require a safe failure. Builds on the existing backup-tamper follow-up in `docs/FOLLOWUPS.md`. |
| 2 | A guided first-agent connection | Select one project, environment, key allowlist and expiry; show a redacted command example and a real connection-check result. Accept only when an allowed read succeeds and a disallowed key/environment is denied. |
| 3 | Bind tools to reviewed manifests | Record a digest for tool schema/build metadata, change it after approval, and require dispatch to reject it until reviewed again. Preserve allowed calls and existing revocation behavior. |
| 4 | Gateway disclosure regression set | Use generated canaries to test raw/encoded/split outputs and tool-metadata injection. Score backend-origin disclosures, benign false positives and run cost separately. Begin without LLM calls. |
| 5 | Sustained resource budget | Run a repeatable 30-minute representative load, including trace saturation and MCP child execution; capture API/DB/children separately, peak RSS, p95, CPU, and pool waits. Compare against this branch before selecting deployment limits. |

The most useful near-term direction is **a small, dependable vault with verifiable agent boundaries**. Restore evidence and an easy first connection improve its practical value before additional AI features or always-on infrastructure.

## Reproduce and resume

From the repository, with Go/CGO and Node dependencies available:

```sh
cd backend
go test -race -shuffle=on ./...
go vet ./...
go build -o /tmp/keepsave ./cmd/server
cd ..
python3 scripts/verify-persistence.py /tmp/keepsave --database sqlite
python3 scripts/verify-persistence.py /tmp/keepsave --database postgres
NODE_OPTIONS=--no-experimental-webstorage npm --prefix frontend test
npm --prefix frontend run lint
npm --prefix frontend run build:all
```

PostgreSQL verification requires Docker and `postgres:16`; the script binds only loopback and cleans up its own container. SQLite requires a CGO-enabled binary; the existing CGO-disabled deployment build is suitable for the remote-database path, not this SQLite test. Keep the same encryption/signing configuration across restarts; never substitute fresh production keys to diagnose a read failure. For PostgreSQL test readiness, check TCP readiness after initialization; the temporary init server's Unix socket can report ready too early.

Production data, Vercel deployment, MySQL runtime persistence, all advanced application modules, long-duration load and real MCP workload resource totals were not exercised. Recheck live branch/deployment state before any subsequent release.


## Integration verification — 2026-09-09

Fresh fetch found `origin/develop` and `origin/main` at `5a8af2d`. The feature work was integrated with that history using a normal merge, preserving upstream documentation and release metadata. The later black-hole/theme implementation and explicit motion preference supersede this report's initial static landing. See the [design integration record](../design/BLACK_HOLE_LANDING.md#develop-integration--2026-09-09) for conflict choices and rendering limits.

Re-ran full backend race/shuffle tests and vet, built the actual CGO-enabled binary, and passed both disposable SQLite/PostgreSQL persistence scripts. The merged frontend passed 87 tests, lint, and both builds; patched Vitest 4.1.11 gives zero npm audit findings. Logs are under `/mnt/data/keepsave-review-2026-09-09/develop/`. The original resource measurements remain scoped historical measurements, not production or peak-load claims.

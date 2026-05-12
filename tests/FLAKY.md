# Flaky Test Tracker (QA 30-day)

A flaky test — one that passes or fails for reasons unrelated to the code under test — is a *bug*, not an annoyance. Tolerating flakes erodes trust in CI: the first time engineers learn to re-run, they've lost the signal forever.

This is QA / Test Engineer 30-day work item §3 from `docs/ROLES_30_60_90.md`.

---

## Definition of flaky

A test is flaky if any of the following is true on `main`:

- Failed at least once and passed at least once **without a code change between runs**, OR
- Pass rate over the last 20 runs is < 99%, OR
- Has any of these patterns in its body: `time.Sleep` without justification, hard-coded port number, dependence on wall-clock time without `clock` injection, `runtime.NumCPU()`-sensitive assertion, race-conditional setup.

## Tracking surface

Lightweight on-purpose: a checked-in table here, owned per row.

| ID    | Test (package + name)                                    | First seen | Symptom                                  | Owner   | Triage status |
|-------|-----------------------------------------------------------|------------|-------------------------------------------|---------|---------------|
| FL-001 | (placeholder — first real flake gets this number)         | YYYY-MM-DD | (e.g., timeout on shared port 8080)        | Backend | Open          |

Rows stay until **closed by a code change** (link the commit SHA) or **deleted** (test removed). No row is closed by "ran it three times and it was fine."

## Process

### When a flake is found

1. **In CI:** if a test fails on `main`, page the test owner. If the same test passes on retry without a code change, add a row to this table and open an issue in GitHub with label `flaky-test`.
2. **In a PR:** if a test fails on the PR and passes on retry, the PR author is responsible for either (a) fixing the test in the same PR or (b) opening the flaky-test row and proceeding with the PR only if the test is unrelated to the PR's diff.
3. **Never re-run silently.** A re-run without a row in this table is a process violation.

### Triage SLA

- **24 hours** to triage (assign owner, root-cause hypothesis).
- **7 days** to fix or quarantine (move to `t.Skip("flaky FL-NNN")` with the row ID).
- **30 days** to permanently fix or delete the test.

A test that lives in `t.Skip` longer than 30 days is presumed dead and removed.

### Fixing patterns

Common root causes and the standard fix:

| Cause                                            | Fix                                                                    |
|--------------------------------------------------|------------------------------------------------------------------------|
| `time.Sleep` waiting for "things to settle"      | Inject a clock or use `eventually`/`require.Eventually` with a tight loop. |
| Shared global state (env vars, singletons)       | Reset in `t.Cleanup`. Or `t.Setenv` for env vars (built-in cleanup).      |
| Port conflicts                                   | Use port 0 (OS-assigned) and read back. Never hard-code.                 |
| Race conditions                                  | Re-run with `-race`. If race detected, fix the code, not the test.       |
| External network calls                           | Mock or use a local fake. No internet in unit tests, ever.               |
| Filesystem state leaking between tests           | `t.TempDir()` and stop sharing paths.                                    |
| Database state leaking between tests             | Truncate or transaction-rollback at the end of each test.                |
| Order dependence                                 | `t.Parallel()` exposes it. Run with `-shuffle=on`.                       |

### What is *not* a flake

These are real bugs that *look* like flakes — don't add a row, fix the code:

- Test fails under `-race` only → genuine data race in production code.
- Test fails on slower CI hardware → timeout is too tight; widen, but also profile the code.
- Test fails when run in a different time zone → assumption about `time.Local`; fix to use UTC.

## Quarantine etiquette

```go
func TestFoo(t *testing.T) {
    t.Skip("flaky FL-042 — see tests/FLAKY.md; owner: alice; due: 2026-06-10")
    // … existing test body unchanged
}
```

The `t.Skip` comment is mandatory and includes the row ID, owner, and due date. Reviewers reject quarantines without all three.

## CI integration

- The `go test` invocation passes `-shuffle=on` and `-race` (already true). This exposes more flakes early.
- A weekly job runs the full test suite five times. Any test that fails any one of those five runs gets an automatic row added to this file via PR.
- Coverage of "tests that touched real network" by static analysis (`semgrep` rule); fail PR if found.

## Anti-patterns (rejected at review)

- "Just add a retry to the test." Retries hide the bug; they don't fix it.
- "Sleep a bit longer." The right wait is event-driven, not time-based.
- "It's only flaky on Windows." We run Linux CI; if we add Windows CI, the rule applies there too. Don't ignore a platform-specific flake.
- Closing a row because "I couldn't reproduce." Reproduction is *your* problem — write the test that reproduces it.

## Initial census

As of `2026-05-12`, no documented flakes. This is either (a) the test suite is too small to have flakes yet — possible given the inverted pyramid documented in `tests/PYRAMID.md`, or (b) flakes exist but aren't being captured. The weekly 5x job (above) will tell us which.

## References

- `docs/ROLES_30_60_90.md` §6 (QA action plan)
- `tests/PYRAMID.md`

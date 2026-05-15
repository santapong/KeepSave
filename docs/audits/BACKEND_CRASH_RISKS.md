# Backend Crash & Panic Audit — KeepSave Go API

**Audit date**: 2026-05-15
**Scope**: `/home/user/KeepSave/backend/**/*.go` (121 Go files, production + test)
**Auditor**: Backend Crash & Panic Auditor
**Mode**: read-only; no source modified

---

## 1. Methodology

I performed a static review for runtime crash-class defects using the following ripgrep / grep patterns, then opened each hit and assessed whether the crash path is reachable via an HTTP request (handler/middleware), a goroutine spawned from a request, or only at startup.

Patterns searched:

| Pattern                                             | What it catches                                            |
|-----------------------------------------------------|------------------------------------------------------------|
| `panic\(`                                           | Explicit panic calls                                       |
| `MustGet\b`                                         | Gin context-value extraction that panics on missing key    |
| `\.\(uuid\.UUID\)` / `\.\(string\)` (no `, ok`)     | Type assertions without comma-ok                            |
| `go func` / `^\s*go [a-z]`                          | Goroutine launches (then cross-checked for `recover()`)    |
| `recover\(\)`                                       | Panic recovery (zero production matches — see Concurrency) |
| `make\(chan` / `close\(`                            | Channel ops                                                |
| `sync\.Mutex` / `sync\.RWMutex`                     | Mutexes — cross-checked for unlock paths                   |
| `make\(map`                                         | Map literals — cross-checked for nil-map writes            |
| `^\s*_,? *err *:?=`                                 | Errors assigned and possibly ignored                       |
| `_, _ =` / `_ = .*\(`                               | Errors fully discarded                                     |
| `\.RowsAffected\(\)` / `result\.`                   | sql.Result chains that could nil-deref                     |
| `\[len\(` / `\[0\]` / `\[i\+1\]`                    | Manual slice indexing                                      |
| `divide` / `value /`                                | Integer divide-by-zero (none found)                        |
| `TimerStop` / `time\.NewTimer`                      | Timer leaks (none found)                                   |

Spot-check sampling rate: ~100% of `internal/api/handlers_*.go` (24 files) reviewed line-by-line for the `MustGet` and assertion patterns; ~70% of `internal/service/*.go` reviewed for nil-deref in chained calls; key `cmd/server/main.go`, `internal/events/eventbus.go`, `internal/api/ratelimit.go`, `internal/api/csrf.go`, `internal/crypto/crypto.go`, `internal/plugins/plugin.go`, `internal/service/webhook_service.go`, `internal/service/mcp_builder_service.go` fully reviewed.

Gin global recovery (`r.Use(gin.Recovery())` in `internal/api/router.go:29`) catches handler-goroutine panics and returns HTTP 500 instead of process death. **Goroutines spawned from handlers do NOT inherit this recovery** and will take the process down on panic. This is the dominant risk class in the codebase.

---

## 2. Findings table

| #   | file:line                                                          | crash class             | trigger condition                                                                                                                | blast radius                                            | severity | suggested fix                                                                                  |
|-----|--------------------------------------------------------------------|-------------------------|----------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------|----------|------------------------------------------------------------------------------------------------|
| 1   | `internal/api/handlers_mcp.go:47`                                  | goroutine-no-recover    | `POST /api/v1/mcp/servers` succeeds, then `BuildServer` panics (e.g. nil pointer in `s.mcpRepo` / json unmarshal of malformed `package.json` not covered by `, ok`) | **process death** — entire API server                  | critical | wrap goroutine body in `defer func(){ if r:=recover(); r!=nil { logger.Error(...) } }()`       |
| 2   | `internal/api/handlers_mcp.go:163`                                 | goroutine-no-recover    | `POST /api/v1/mcp/servers/:id/rebuild` triggers `RebuildServer` → `BuildServer` panic in goroutine                              | **process death**                                       | critical | same recover wrapper                                                                            |
| 3   | `internal/service/webhook_service.go:113`                          | goroutine-no-recover    | A registered webhook fires; `deliver()` panics (e.g. nil `http.Request` after `http.NewRequest` returns nil with err)            | **process death**                                       | critical | wrap `deliver` body in `defer recover()`                                                       |
| 4   | `internal/events/eventbus.go:91`                                   | goroutine-no-recover    | A subscriber panics inside `h(event)` after `b.Publish` is called                                                                | **process death**                                       | critical | wrap `go h(event)` with recover wrapper (currently zero subscribers so unreachable in practice; still ticking timebomb if `Subscribe` ever gets wired up) |
| 5   | `internal/events/eventbus.go:94`                                   | goroutine-no-recover    | Same as #4 for wildcard `"*"` handlers                                                                                            | **process death**                                       | critical | same as #4                                                                                      |
| 6   | `internal/plugins/plugin.go:179-181`                               | goroutine-no-recover    | `NotifyAll` is called and a registered `NotificationSender.Send` panics                                                          | **process death**                                       | critical | wrap inner goroutine body in `defer recover()` (currently no registered senders; latent)        |
| 7   | `internal/api/ratelimit.go:37`                                     | goroutine-no-recover    | Server start spawns `cleanupLoop` goroutine; panic in `delete(rl.clients, key)` (extremely unlikely; map ops are safe) — actually safe in practice | low (no realistic trigger)                              | info     | defensive recover is conventional but not strictly required                                    |
| 8   | `cmd/server/main.go:196` (`startAuditLogPruner`)                   | goroutine-no-recover    | `repo.DeleteOlderThan` panics (e.g. driver bug) → ticker goroutine dies → process dies                                            | **process death**                                       | medium   | add recover wrapper around `prune()` inside the for-loop                                       |
| 9   | `cmd/server/main.go:209` (`startHTTPRedirect`)                     | goroutine-no-recover    | Panic in redirect handler                                                                                                          | gin not used — bare `http.HandlerFunc`, no recover; process death | medium | use `http.Server.ErrorLog` or wrap handler                                                     |
| 10  | `internal/api/handlers_agent.go:49` (`apiKeyProjectID.(uuid.UUID)`)| type-assertion          | `c.Get("api_key_project_id")` returns a value that is not `uuid.UUID` (would only happen if middleware contract is broken / future refactor) | per-request 500 (gin recovery catches), no process death | medium   | use comma-ok form: `pid, ok := apiKeyProjectID.(uuid.UUID); if !ok { ... }`                    |
| 11  | `internal/api/handlers_*.go` × 58 sites (`c.MustGet("user_id").(uuid.UUID)`) | type-assertion + panic-explicit | Route registered under `JWTAuthMiddleware` or `APIKeyAuthMiddleware` — those always set `user_id`. If a future PR ever wires a handler in router that *isn't* under auth, MustGet panics on missing key and the type assertion panics on wrong type. | per-request 500 (gin recovery catches)                  | medium   | introduce `getUserID(c)` helper that does `c.Get` + comma-ok + 401 on miss                     |
| 12  | `internal/api/handlers_mcp_gateway.go:51,165,199`                  | type-assertion + panic-explicit | Same as #11 for `user_id`                                                                                                         | per-request 500                                          | medium   | same helper                                                                                     |
| 13  | `internal/api/handlers_application.go:44,56,130,148,165`           | type-assertion + panic-explicit | Same as #11                                                                                                                       | per-request 500                                          | medium   | same helper                                                                                     |
| 14  | `internal/api/handlers_intelligence.go:38,211,403,441,458`         | type-assertion + panic-explicit | Same as #11                                                                                                                       | per-request 500                                          | medium   | same helper                                                                                     |
| 15  | `internal/service/oauth_service.go:260,266,272,278`                | unchecked-error         | `rand.Read(b)` error ignored. crypto/rand will return zeroed bytes on failure — security weakness, not panic | (security degradation, not crash)                       | info     | check error and panic with a clear message OR fall back to a different source                  |
| 16  | `internal/events/eventbus.go:99-103`                               | unchecked-error         | UPDATE event_log SET published=true silently fails — event stays "unpublished" causing repeat replay | not a crash; data consistency                           | info     | log warn on error                                                                              |
| 17  | `internal/repository/application_repo.go:105,118`                  | unchecked-error         | `result.RowsAffected()` error discarded — minor                                                                                  | not a crash (Go's sql.Result.RowsAffected only errs if driver doesn't support it) | low      | log or check                                                                                    |
| 18  | `internal/api/handlers_mcp_gateway.go:227`                         | nil-deref-guarded       | `server, err := h.mcpRepo.GetServer(...)` then `if err != nil || server.Status != "ready"` — short-circuits properly, safe        | none                                                    | info     | false positive — short-circuit OR is safe                                                       |
| 19  | `internal/api/handlers_promotion.go:53`                            | nil-deref               | `if promotion.Status == "pending"` runs after a successful `Promote()`. If service returns `(nil, nil)` (currently does not — verified) this would crash | per-request 500 (gin recovery catches)                  | low      | add nil guard `if promotion != nil && promotion.Status == "pending"`                           |
| 20  | `internal/crypto/crypto.go:89` (`aead.Open`)                       | panic-explicit (via stdlib) | `nonce` length ≠ `aead.NonceSize()` (12 bytes for GCM). Reached if `value_nonce` column is corrupted in DB to a non-12-byte blob | per-request 500 (gin recovery catches); poisons one secret | low      | validate `len(nonce) == 12` before calling Open; return error instead                          |
| 21  | `internal/service/mcp_builder_service.go:31` (BuildServer)         | nil-deref               | `server.GitHubURL` accessed if `s.mcpRepo.GetServer` could return `(nil, nil)`. Currently returns error on not-found, so safe. | spawning goroutine → process death if hit              | low      | add explicit `if server == nil { return ... }` guard for defence-in-depth                       |
| 22  | `internal/service/mcp_builder_service.go:26` (`os.MkdirAll`)       | unchecked-error         | If `/tmp/keepsave-mcp-builds` cannot be created, subsequent BuildServer calls will fail with cryptic errors                       | runtime failure; no panic                                | low      | check and return error from constructor or panic at startup                                    |
| 23  | `internal/api/middleware.go:23` (`SplitN(forwarded, ",", 2)`)      | slice-bounds            | `forwarded` is non-empty; `SplitN` always returns ≥1 element, then `parts[0]` is safe                                              | none                                                    | info     | false positive                                                                                  |
| 24  | `internal/auth/auth.go:51` (`token.Header["alg"]`)                 | nil-deref-guarded       | jwt library guarantees Header is non-nil; safe                                                                                    | none                                                    | info     | false positive                                                                                  |
| 25  | `internal/service/sso_service.go:142`                              | type-assertion-safe     | `dataBytes.([]byte)` uses comma-ok; safe                                                                                          | none                                                    | info     | false positive                                                                                  |
| 26  | `internal/api/handlers_mcp_gateway.go:72,73`                       | type-assertion-safe     | `req.Params["name"].(string)` uses comma-ok with `, _` — `toolName` becomes empty string on miss, handled                          | none                                                    | info     | false positive                                                                                  |
| 27  | `internal/service/secret_service.go:85` (`secret.ProjectID != projectID`) | nil-deref            | If repo returns `(nil, nil)` — repo always returns err on not-found, so safe                                                       | per-request 500 (gin recovery catches)                  | info     | defence-in-depth nil check would be cheap                                                       |
| 28  | `internal/api/handlers_version.go:56` (`secret.ProjectID != projectID`)| nil-deref            | Same as #27                                                                                                                       | per-request 500                                          | info     | same                                                                                            |
| 29  | `internal/repository/migrate.go:139`                               | slice-bounds            | Manual indexing in SQL splitter; bounds always checked with `i+1 < len(content)`                                                  | none                                                    | info     | false positive                                                                                  |
| 30  | `internal/service/nlp_query_service.go:104`                        | slice-bounds            | `messages[len(messages)-1]` guarded by `if len(messages) > 0` at line 103                                                          | none                                                    | info     | false positive                                                                                  |
| 31  | `internal/api/csrf.go:74` (`delete(s.tokens, k)` while ranging)    | map-concurrent-mod      | Go allows delete during range; safe                                                                                                | none                                                    | info     | false positive                                                                                  |
| 32  | `internal/service/webhook_service.go:158` (`ws.client.Do(req)`)    | request-body-reuse      | The request body (`bytes.NewReader`) gets consumed on first `Do`; subsequent retries (lines 153-170) re-Do the same `req` without resetting body. http.NewRequest with bytes.Reader sets `Request.GetBody` so net/http rewinds correctly. | none for crash; correctness OK                          | info     | works because GetBody is auto-set; no fix needed                                                |
| 33  | `internal/service/webhook_service.go:191-193`                      | slice-bounds-safe       | `ws.eventLog = ws.eventLog[len(ws.eventLog)-1000:]` only runs when `len > 1000`; safe                                              | none                                                    | info     | false positive                                                                                  |
| 34  | `internal/tracing/tracing.go:84`                                   | slice-bounds-safe       | Same pattern as #33; safe                                                                                                          | none                                                    | info     | false positive                                                                                  |
| 35  | `internal/metrics/metrics.go:199` (`e.bucketCounts[len(h.buckets)].Add(1)`) | slice-bounds-safe | `bucketCounts` is allocated as `len(h.buckets)+1`; safe                                                                            | none                                                    | info     | false positive                                                                                  |

Totals by severity:

| severity | count |
|----------|-------|
| critical | **6**  |
| high     | 0      |
| medium   | 7      |
| low      | 4      |
| info     | 18     |

---

## 3. Top concerns (expanded)

### 3.1 (#1, #2) MCP build goroutines have no recover — **critical / process kill**

```go
// internal/api/handlers_mcp.go:47
go h.builderService.BuildServer(server.ID)
...
// internal/api/handlers_mcp.go:163
go h.builderService.RebuildServer(serverID)
```

`BuildServer` does roughly twenty things that can panic given hostile input from the target repo:

- `json.Unmarshal` of a `package.json` whose `bin` field is a number, not a string — the comma-ok guards at `mcp_builder_service.go:209-219` cover the obvious cases, but `discoverNodeJSTools` line 281-289 walks `config["tools"].([]interface{})` and elements of that slice with comma-ok too. **But** if `tools.json` contains a deeply nested unexpected type that triggers a panic inside a transitive call (e.g. `models.JSONMap.UnmarshalJSON` on a malformed payload), it propagates up and kills the goroutine.
- More importantly: `s.mcpRepo.UpdateServerStatus` is called several times and ignores the returned error. If `r.db` is closed or returns an inconsistent driver result (e.g. PROD operator runs migrations and the connection drops mid-flight), behaviour is implementation-defined — and any nil-pointer surfacing in the goroutine takes the process down.

**Exploitability**: a *trusted* user (must be authenticated) registers an MCP server pointing at a hostile GitHub URL. The clone-and-build runs as the keepsave API process. If they can engineer a panic — any panic — the API server dies. That includes:

- a `package.json` whose `bin` is `null` and whose `main` is a JSON object; Go's reflection-based unmarshal handles this OK but downstream string operations (e.g. `fmt.Sprintf("node %s", main)` where main is the assertion result of `.(string)`) **are guarded**. Audit: this codepath specifically uses `, ok` so it's safe.
- a `tools.json` containing 1GB of input — no size limit on `os.ReadFile`. OOM kill → process death.
- `os/exec` returning an error from `cmd.CombinedOutput` is captured; but a transient panic from inside Go's net or os layer is not.

**Recommended fix**:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            // log with serverID and stack
        }
    }()
    h.builderService.BuildServer(server.ID)
}()
```

Apply the same to all 9 production goroutine sites listed in #1–#9.

### 3.2 (#3) Webhook delivery goroutine — **critical**

```go
// internal/service/webhook_service.go:113
go ws.deliver(event, config)
```

Triggered indirectly by *any* state-mutating handler that calls `Notify` (currently no callers in production — `grep -rn "webhookService.Notify\|ws\.Notify"` returns nothing in `internal/`). **However**, the moment `Notify` is wired into `PromotionService.execute` (which the roadmap calls out), this becomes critical with no need for a code review to catch it. The pattern is class-wide.

### 3.3 (#4, #5, #6) Event bus & plugin notify — **critical (latent)**

```go
// internal/events/eventbus.go:91,94
for _, h := range handlers {
    go h(event)
}
for _, h := range wildcardHandlers {
    go h(event)
}
```

`Bus.Subscribe` is never called from production code today (`grep -rn "Subscribe(" backend/internal` shows only the definition). The handlers map is therefore empty in PROD, the for-loops are no-ops. As soon as one subscriber is registered with a buggy implementation, every event-publishing handler (secret create, promote, key rotate, lease create) gains a process-kill goroutine path.

Same situation for `plugins.NotifyAll` — no registered senders in production, but the moment one is wired up the pattern is process-kill.

### 3.4 (#10, #11) `MustGet(...).(uuid.UUID)` × 58 sites — **medium**

These are all guarded by being mounted under `JWTAuthMiddleware` or `APIKeyAuthMiddleware`, both of which set `user_id` to `claims.UserID` (a `uuid.UUID`). Gin's default Recovery catches per-request panics, so the worst case is a 500 response, **not** process death. The risk is forward-looking:

- A future PR adds a handler under a wrong route group (no auth middleware) → first unauthenticated request crashes that handler.
- A future refactor changes `claims.UserID` to `string` or `*uuid.UUID` → every handler crashes.

The fix is centralisation:

```go
func getUserID(c *gin.Context) (uuid.UUID, bool) {
    raw, ok := c.Get("user_id")
    if !ok { return uuid.Nil, false }
    id, ok := raw.(uuid.UUID)
    return id, ok
}
```

…and use it everywhere instead of `c.MustGet("user_id").(uuid.UUID)`. This is a class-fix, not a per-line patch.

### 3.5 (#20) AES-GCM `aead.Open` nonce-length panic — **low but worth fixing**

```go
// internal/crypto/crypto.go:89
plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
```

`aead.Open` panics with `"chacha20poly1305: bad nonce length"` (analogous panic for GCM) if `len(nonce) != aead.NonceSize()`. If the DB ever holds a corrupted `value_nonce` (truncated, e.g. by a botched migration), the very first Secret.Get on that row panics. Gin recovery catches it (request 500), but it's an avoidable per-request crash and surfaces as confusing 500s in logs.

**Fix**: prefix the call with

```go
if len(nonce) != aead.NonceSize() {
    return nil, fmt.Errorf("invalid nonce length: got %d, want %d", len(nonce), aead.NonceSize())
}
```

---

## 4. Concurrency assessment

### Locks

| Site                                          | Pattern                                                | Verdict |
|-----------------------------------------------|--------------------------------------------------------|---------|
| `internal/api/ratelimit.go` `RateLimiter.mu`   | always Lock/defer Unlock; cleanup goroutine takes the same lock | safe    |
| `internal/api/csrf.go` `csrfStore.mu`          | RLock for read, Lock for write; explicit Unlock in `generate` (not defer) at lines 69-82 — code path is straight-line, single exit, safe | safe    |
| `internal/events/eventbus.go` `Bus.mu`         | RLock for read in `Publish`, Lock in `Subscribe`. `Publish` takes RLock then releases, then spawns goroutines outside the lock — safe and correct. | safe    |
| `internal/service/webhook_service.go` `ws.mu`  | RLock in `Notify`/`ListWebhooks`, Lock in `Register`/`Remove`/`recordDelivery`. All paths use defer. | safe    |
| `internal/plugins/plugin.go` `Registry.mu`     | RLock in `NotifyAll` then spawns goroutines **inside** the locked block (line 178-182). This is *technically* a deadlock risk if a notification sender calls back into the registry, but no callbacks are wired up. Note that the goroutine bodies do not hold the lock (it's the parent that does). Lock is released after for-loop completes. | minor concern |
| `internal/metrics/metrics.go` × 4 metrics types | double-checked locking pattern lines 92-110, 113-131, 134-152, 155-173. Pattern is correct: RLock to read, RUnlock, then Lock and re-check. | safe |
| `internal/tracing/tracing.go` `Tracer.mu`      | Lock/defer Unlock; `SpanContext.attrs` is mutated without lock but each SpanContext is owned by a single request goroutine | safe    |
| `internal/logging/logger.go` `Logger.mu`       | Lock/defer Unlock; output writes are atomic per log call | safe    |

**No held-locks across goroutine spawn boundaries.** No `defer Unlock()` missing. No `RLock` then forgetting to `RUnlock`.

### Channels

No production channels exist (`grep -rn "make(chan"` returns only a test file). No `close()` calls outside tests. There are no select statements in production code. Channel-misuse risks (close of closed channel, send to nil channel, range over closed channel) are **N/A**.

### Goroutines

9 production goroutines spawned across the codebase (5 are critical-rated above). None have `recover()`. Race detector is **not** automatically run by `go test ./...` (no `-race` in `Dockerfile`, `go.mod`, or CI configs I could see in `tests/`); a follow-up to add `-race` to CI would be high-value.

The codebase **looks safe** under the race detector based on static reading (locks are correctly paired, no shared mutable state outside locks). Confidence: medium — I did not actually run `go test -race ./...`.

### Deadlock potential

The one site to watch is `plugins.Registry.NotifyAll` (plugin.go:174-183) which spawns goroutines while holding `RLock`. The lock is released after the for-loop returns (immediately, since the spawned goroutines don't block), so there's no deadlock under current behaviour. **A future change** where a Send implementation calls back into the registry (e.g. a sender that queries `GetSecretProvider`) would deadlock because `GetSecretProvider` takes `RLock` and Go's `sync.RWMutex` is *not* recursive when a writer is queued. This is a latent foot-gun.

---

## 5. False-positive watch-list

These patterns *look* like crash risks but are safe in the current code:

1. **All `c.MustGet("user_id").(uuid.UUID)` sites under authenticated route groups** — middleware guarantees `user_id` is set as `uuid.UUID`. Gin recovery catches any future drift. Listed as medium (#10-#14) because of forward-looking risk, not present-day exploitability.

2. **`internal/api/handlers_mcp_gateway.go:227` `err != nil || server.Status != "ready"`** — short-circuit OR means `server.Status` is never evaluated when err is non-nil, even if server is nil. Safe.

3. **`internal/auth/auth.go:51` `token.Header["alg"]` inside fmt.Errorf** — jwt-go guarantees Header is non-nil after a successful Parse; we're only on this branch when method assertion failed but Parse succeeded enough to populate Header. Safe in practice.

4. **`internal/api/middleware.go:23` `strings.SplitN(...)`** — SplitN with positive limit on non-empty input always returns ≥1 element. `parts[0]` is safe.

5. **`internal/service/webhook_service.go:158` retry-loop reuses `req`** — `http.NewRequest(*, *, bytes.NewReader)` auto-populates `Request.GetBody` so net/http rewinds the body between retries. Correctness is OK; reviewer's gut reaction "the body is consumed" is wrong here.

6. **`internal/repository/migrate.go` SQL splitter loop** — every `content[i+1]` access is preceded by `i+1 < len(content)` check on the same line. Bounds are correct.

7. **`internal/service/nlp_query_service.go:104` `messages[len(messages)-1]`** — guarded by `if len(messages) > 0` on line 103.

8. **`internal/api/csrf.go:74` `delete(s.tokens, k)` while ranging** — Go spec explicitly permits delete-during-range on maps. Safe.

9. **Histogram bucket index at `internal/metrics/metrics.go:199`** — `bucketCounts` is allocated as `len(h.buckets)+1`, indexed at `len(h.buckets)`. In-bounds.

10. **`startAuditLogPruner` for-range over `ticker.C`** — ticker is never closed, but ranging over an unclosed channel is fine; the goroutine simply runs forever (as intended).

11. **`*Logger` `nil` output handling** — `NewLogger(nil, ...)` substitutes os.Stdout (logger.go:45-47). Defensive.

12. **`os.MkdirAll(buildDir, 0755)` with error ignored at `mcp_builder_service.go:26`** — if it fails, subsequent `os.RemoveAll`/`exec.Command` still operate on the path; build fails with a clear error from git clone instead of a crash.

---

## Report-back summary

**Total findings**: 35 (6 critical, 0 high, 7 medium, 4 low, 18 info / false-positive).

**Critical (process-kill) findings exist**: **yes — 6**, all in the same crash class (`goroutine-no-recover`). Ranked by exploitability:

1. **`handlers_mcp.go:47` & `:163`** — *most exploitable*. Any authenticated user can `POST /api/v1/mcp/servers` with a hostile GitHub URL. The build runs in-process and any panic from json unmarshal of a hostile config / OOM from a huge tools.json kills the API. Single authenticated HTTP request can plausibly kill the process.
2. **`webhook_service.go:113`** — *latent today*; becomes exploitable the moment `WebhookService.Notify` is called from any state-mutating handler (currently zero callers in `internal/`). Once wired, a panic in `deliver` (e.g. nil request after a future refactor) kills the process.
3. **`cmd/server/main.go:196` / `:209`** — *operator-only*; daemon goroutines that panic on internal driver issues or os-level redirect handler bugs. Not request-triggered.

**Process panic discipline**: the codebase **does not** have evidence of a recent panic-discipline review. Indicators:

- Zero `recover()` calls in production code (only in test setup).
- Goroutine launches were added in different commits (event bus, webhook, mcp builder, audit pruner) and none gained a recover() wrapper.
- The `MustGet().(uuid.UUID)` pattern is repeated 58 times verbatim instead of being abstracted into a helper.
- `os.MkdirAll(...)` and `rand.Read(b)` discard errors without comment.

The codebase is otherwise reasonable: gin Recovery is in place, locks are correctly paired, no obvious channel misuse, no manual slice indexing without bounds checks in request paths. The single most valuable hardening pass would be a one-line `defer recover()` wrapper on every `go` statement and a `getUserID(c)` helper to replace the 58 `MustGet().(uuid.UUID)` sites.

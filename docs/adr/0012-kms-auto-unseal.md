# ADR-0012: KMS auto-unseal as production default for the master key

- **Status:** Accepted (sponsor-authorized 2026-05-18; retroactive Security/Tech Lead sign-off pending per CLAUDE.md §Type-1)
- **Date:** 2026-05-15
- **Authors:** ADR Drafter (Backend Engineer role)
- **Reviewers required:** Tech Lead; Security Engineer (mandatory veto — touches crypto / key-custody, per `docs/ROLES.md` §3.1 Type-1)
- **Supersedes:** none
- **Prerequisites:** **ADR-0011 (graceful shutdown + per-query DB timeouts).** Without ADR-0011's drain semantics, a KMS reconnect during a runtime key-rotation event can race against in-flight handlers holding pool connections. ADR-0011 must land first or jointly.
- **Related:** ADR-0001 (envelope encryption), ADR-0004 (key hierarchy — names `EnvProvider` development-only), `docs/research/competitors/vault.md` Cand. 3 (`adopt-now`), `docs/FOLLOWUPS.md` FU#1 (lines 90-95), `docs/audits/BACKEND_SPOF.md` §1.2 / §3.2 / §3.6, `docs/THREAT_MODEL.md` §1 Vault row D, `docs/AUDIT_LOG_COVERAGE.md`.

---

## Context

ADR-0004 §Consequences: *"Production deployments should source from a KMS, not env vars; the `EnvProvider` is acceptable for development only."* That posture is unfulfilled in code.

`docs/FOLLOWUPS.md` Phase A FU#1 (lines 90-95) is the tracked work, verbatim:

> *"### 1. AWS / GCP KMS adapters wired into `main.go` — **Status:** Code exists (`kms_aws.go`, `kms_gcp.go` in `backend/internal/crypto/keyprovider/`); blocked on `go mod tidy` to add `aws-sdk-go-v2/service/kms` and `cloud.google.com/go/kms/apiv1` to `go.sum`. — **Why it matters:** ADR-0004 calls `EnvProvider` development-only; production deployments need KMS. Without this, the runbook tells customers 'use a KMS' but the binary doesn't support one yet. — **Owner:** DevOps + Backend Engineer (pair). — **Due:** 30 days (Phase A). — **Related:** ADR-0004 §Open Questions."*

The SPOF audit makes the runtime cost concrete (verbatim, `docs/audits/BACKEND_SPOF.md` §1.2 line 35):

> *"KMS providers (`internal/crypto/keyprovider/kms_aws.go`, `kms_gcp.go`) — interfaces only; main.go bails out with 'not implemented' error at `cmd/server/main.go:247-248`, so production cannot today use AWS/GCP KMS without a custom build."*

And §3.2 lines 134-136: *"Vault startup has no retry: `cmd/server/main.go:53-59` calls `resolveMasterKey` once with a 15s timeout. If Vault is having a hiccup, pod crashes... Recommend: exponential backoff with cap (e.g., 5 attempts, 1s/2s/4s/8s/16s)."*

The Vault dossier's accepted Candidate 3 (`docs/research/competitors/vault.md` §6) names the pattern: *"Auto-unseal via cloud KMS as production default... Adoption is operational: make non-`EnvProvider` mandatory in prod via a `main.go` refuse-to-start check."*

Today a production deploy must either (a) silently run with `EnvProvider`, (b) attempt `awskms`/`gcpkms` and crash with "not implemented", or (c) use `vault` and crash-loop on the first 15s hiccup. None is acceptable in Phase A. The narrow `AWSKMSDecrypter` / `GCPKMSDecrypter` interfaces already exist (`kms_aws.go:12-14`, `kms_gcp.go:11-13`); ADR-0004's `keyprovider.Provider` already isolates the rest of the app. Remaining work: wiring + retry/backoff + audit emission.

## Options considered

### Option A — Wire adapters behind `KEEPSAVE_KEY_PROVIDER`; startup retry/backoff

- **How:** Extend `resolveMasterKey` (`main.go:234-252`) to construct `AWSKMSProvider` / `GCPKMSProvider` from thin SDK adapters in `main.go`. Wrap in exponential backoff: 5 attempts, 1s/2s/4s/8s/16s (~31s total). Distinguish permanent (auth fail, missing key, malformed ciphertext) from transient (timeout, throttle); retry only transient. Emit `master_key.fetch_attempt` and `master_key.fetch_success` audit events.
- **Pros:** Closes ADR-0004's gap. Reuses validated adapter code. Defensible failure model. No data-model change. Single-revert rollback.
- **Cons (op):** Uptime coupled to KMS provider (THREAT_MODEL §1 Vault row D, residual Medium). Worst-case ~31s boot.
- **Cons (security):** Per `docs/research/competitors/vault.md` §7 Cand. 3 — *"none material — adoption moves toward the posture ADR-0004 already named as the target."*

### Option B — HashiCorp Vault Transit as recommended default

- **How:** Point operators at `keyprovider/vault.go` (already implemented, `vault.go:54-92`) with the same retry wrapper.
- **Pros:** Adapter production-quality; no new SDK deps.
- **Cons (op):** Vault is an external dependency many KeepSave customers won't run; forcing it narrows the production story.
- **Cons (security):** Same residual class as A, plus Vault's broader CVE surface (CVE-2024-2660). Keep as available, not default.

### Option C — Shamir secret sharing for human-operator unseal

- **How:** N-share split, K-of-N operator unseal at boot.
- **Pros:** No KMS dependency; multi-party control.
- **Cons (op):** Per `docs/research/competitors/vault.md` §5 — *"Operational overhead disproportionate to our customer base."*
- **Cons (security):** New unseal endpoint = high-value target; unjustified at our scale.

## Decision

**Option A.** ADR-0004 names KMS as target; adapter code is already written and SDK-free; the failure model is what `docs/audits/BACKEND_SPOF.md` §3.2 recommends; `crypto.Service.GetMasterKey()` is provider-agnostic so the rest of the app doesn't move. Option B remains an available (not default) adapter. Option C is scope-inappropriate.

The decision is conditional on **ADR-0011 landing first or jointly** — see §Prerequisites.

## Rejection rationale

- **Option B** rejected as default: Vault is an external dependency not all customers run.
- **Option C** rejected: overhead unjustified; revisit in Phase B if compliance forces it.

## Consequences

- **Operational:** Four supported `KEEPSAVE_KEY_PROVIDER` values — `env` (default, dev only), `awskms`, `gcpkms`, `vault`. New env vars `KEEPSAVE_KMS_KEY_ID`, `KEEPSAVE_KMS_CIPHERTEXT`. Cold boot worst case ~31s under transient failure. New runbook entries.
- **Security:** Closes the ADR-0004 "env-provider in production is a footgun" gap. Trust boundary unchanged — master key still in process memory after boot. THREAT_MODEL §1 Vault row D marginally widened (already Medium).
- **Audit:** Two new taxonomy entries — `master_key.fetch_attempt`, `master_key.fetch_success` — added to `docs/AUDIT_LOG_COVERAGE.md` in the same PR.
- **Migration:** None for dev (`env` default). Production provisions a KMS key, pre-wraps the master key, sets three env vars. No ciphertext re-encryption.
- **Reversibility:** Fully reversible — see §Rollback.

## Failure modes

- **KMS permanently unavailable at startup** (auth failure, missing key, malformed ciphertext): process exits non-zero after the first attempt. Operator resolves and restarts. Silent `EnvProvider` fallback would lose the security property — fail-fast is correct.
- **KMS transient failure at startup** (timeout, throttle): retry up to 5 attempts on 1s/2s/4s/8s/16s (max ~31s). Emit `master_key.fetch_attempt` per attempt.
- **KMS transient failure during runtime** (after boot): master key already in memory in `cryptoSvc` (`crypto.go:11-30, 64-72` — no provider call in encrypt/decrypt path; cross-ref `vault.md` §12 Inv 1). Runtime KMS outage does **not** crash the process; only a key-rotation event lazily refreshes the cache.
- **KMS unavailable during key rotation**: rotation fails loud; existing master key remains valid; operator retries.

## Observability

- `master_key.fetch_attempt` per retry — `{provider, attempt_n, backoff_ms}`.
- `master_key.fetch_success` on first success — `{provider, total_duration_ms, attempts}`.
- Infrastructure events (no project/user binding); cross-ref `docs/AUDIT_LOG_COVERAGE.md`.

## Implementation plan

1. **`backend/cmd/server/main.go:53-59`** — wrap `resolveMasterKey` in an exponential-backoff retry: for `attempt = 1..5`, build a 15s timeout context, call `resolveMasterKey`, emit `master_key.fetch_attempt`; on success emit `master_key.fetch_success` and break; on permanent error log and `os.Exit(1)`; on transient sleep `backoff(attempt)` (1s/2s/4s/8s/16s) and retry. After 5 attempts exhausted, exit non-zero. `isPermanent` keys off typed adapter errors (AWS `kms.ErrCodeInvalidKeyUsageException`, GCP `codes.PermissionDenied`, Vault 403); default to retry on unknown.

2. **`backend/cmd/server/main.go:247-248`** — replace the "not implemented" bail with adapter construction:
   ```
   case "awskms":
       return keyprovider.NewAWSKMSProvider(awskmsAdapter(cfg), cfg.KMSKeyID, cfg.KMSCiphertext).GetMasterKey(ctx)
   case "gcpkms":
       return keyprovider.NewGCPKMSProvider(gcpkmsAdapter(cfg), cfg.KMSKeyName, cfg.KMSCiphertext).GetMasterKey(ctx)
   ```
   SDK imports land in `main.go` only; `keyprovider` stays SDK-free per its package doc.

3. **`backend/internal/crypto/keyprovider/`** — no changes expected. Lint: `validateKey` called in every `GetMasterKey` path (already true at `kms_aws.go:48`, `kms_gcp.go:45`, `env.go:34`, `vault.go`).

4. **`backend/internal/config/config.go`** — add `KMSKeyID`, `KMSKeyName`, `KMSCiphertext` fields + env-var parsing.

5. **`docs/AUDIT_LOG_COVERAGE.md`** — add the two new event names.

6. **`docs/RUNBOOK.md`** — KMS-unavailable-at-boot + KMS-rotation procedures.

7. **`go.mod` / `go.sum`** — add `github.com/aws/aws-sdk-go-v2/service/kms` and `cloud.google.com/go/kms/apiv1` (the `go mod tidy` blocker FU#1 names).

**New tests — `tests/integration/kms_unseal_test.go`**, table-driven:
- `env → awskms` boot completes with a fake `AWSKMSDecrypter`.
- Transient-fail-retry-success: mock returns 2 errors then OK; assert 3 `master_key.fetch_attempt` + 1 `master_key.fetch_success` audit rows.
- Permanent-fail: mock returns permission-denied; assert exit 1 after 1 attempt.
- Five-attempt exhaustion: mock always times out; assert exit 1 after 5 attempts.

## Rollback plan

Fully reversible without data migration. Single revert commit:

1. Revert `main.go:53-59` retry wrapper (back to single 15s attempt).
2. Revert `main.go:247-248` adapter cases (back to "not implemented" bail).
3. Leave `keyprovider/kms_aws.go`, `kms_gcp.go` in tree (harmless dead code).
4. Leave `go.sum` SDK entries in tree (no behavioral effect when not imported).

No ciphertext re-encryption, no schema migration, no data loss. Operators rolling back switch `KEEPSAVE_KEY_PROVIDER` back to `env`. Per `docs/research/competitors/vault.md` §11: *"Fully reversible — no schema or ciphertext change. Master keys already unwrapped continue to work; next process start sources from env."*

## Open questions

1. **Default backoff schedule.** 1s/2s/4s/8s/16s × 5 attempts = ~31s worst-case boot latency. Acceptable, or should we tier — faster fail in dev (1 attempt), slower fail in prod (e.g., 10 attempts, ~8 min)? Per-env policy adds config surface; uniform 5-attempt is proposed. *Owner: Backend Engineer + DevOps. Due: before merge.*
2. **`EnvProvider` runtime rejection in production.** ADR-0004 documents `EnvProvider` dev-only but does not enforce it. Should `main.go` refuse to start when `KEEPSAVE_KEY_PROVIDER=env` and a production indicator is set (`KEEPSAVE_ENV=production`), or only warn? Vault dossier §12 recommends *"refuses to start on `EnvProvider` in prod (not warns)"*. *Owner: Security Engineer (veto applies). Due: 30 days.*
3. **Key-rotation event surface.** Does `Rotate()` (currently `ErrUnsupported` on all adapters) trigger an in-process re-fetch, or is rotation an operator-driven restart? ADR-0004 §Consequences says rotation is the provider's responsibility. If in-process, atomic swap of `cryptoSvc.masterKey` without restarting handlers requires ADR-0011's drain semantics. *Owner: Backend Engineer + Security Engineer. Due: 60 days; implementation in a separate ADR.*

## References

- `backend/cmd/server/main.go:53-59` (retry target), `:247-248` (adapter-wiring target)
- `backend/internal/crypto/keyprovider/`: `provider.go:21-25`, `env.go:12-42`, `kms_aws.go:12-14, 27-52`, `kms_gcp.go:11-13, 24-49`, `vault.go:54-92`
- `backend/internal/crypto/crypto.go:11-30, 64-72`
- ADR-0001, ADR-0004, ADR-0011 (prerequisite)
- `docs/research/competitors/vault.md` §6 / §7 / §10 / §11 / §12 Cand. 3
- `docs/FOLLOWUPS.md` lines 90-95; `docs/audits/BACKEND_SPOF.md` §1.2 line 35, §3.2 lines 134-136, §3.6 lines 202-203
- `docs/THREAT_MODEL.md` §1 Vault row D; `docs/AUDIT_LOG_COVERAGE.md`; `docs/RUNBOOK.md`

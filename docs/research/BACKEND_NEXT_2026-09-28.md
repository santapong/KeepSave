# Backend development intake — 2026-09-28

> **Historical intake, clarified 2026-10-04.** “Current evidence” and the recommended sequence below refer to the September 28 source and tests. Subsequent identity, journal, recovery and harness-neutral code is recorded in the [October checkpoint](../design/2026-10-02-harness-neutral-platform/README.md) and its linked acceptance ledger. The stated historical test counts and unresolved review obligations are preserved.

The owner accepted the frontend in the conversation on 2026-09-28 and asked to shift effort to backend development and features. Keep the current frontend as the design baseline. Worktree: `KeepSave-landing`, `feat/landing-event-horizon-20260928`, base `9e2b3ba`; earlier uncommitted frontend/social-login work is preserved.

Later owner direction on the same date: plan the broader backend architecture for maintainability and expansion, with MCP, skills, and controlled developer-harness credentials. The [backend platform proposal](../design/2026-09-28-backend-platform/README.md) extends this intake with a repository-wide architecture review, module boundaries, credential security design, current standards research, and staged acceptance criteria. It is a proposal; the recovery and platform features are not marked implemented.

## Current evidence

The backend already exposes secrets, environments, promotion/rollback, audit, scoped API keys, agent leases/tokens, MCP integration, organizations and social-login plumbing. Existing `go test ./...` passed in the local Go 1.25 Docker environment on this date. That result does not establish completion of every advertised feature.

Source inspection found these recovery gaps:

1. `SecretVersionRepository.CreateVersion` has no application caller. Normal secret create/update paths do not populate version history, despite history routes being present.
2. `BackupService.CreateBackup` encrypts project ID, secret count and snapshot type; it does not include the secret records needed to recover the vault. No restore service/route is present. Existing backup tests assert snapshot/audit creation, not recovery.
3. Project DEK rotation updates current secrets and the project DEK. A history implementation must account for retained versions and promotion snapshots before claiming history survives rotation.
4. Existing follow-up documents contain stale open entries for already implemented rotation/auth work. Use live source and executable acceptance checks to reconcile each item, rather than copying old checkboxes.

## First bounded foundation fix

**Type-2 authorization bug fix, no new grant or crypto/schema change.** Individual-secret and version-history routes currently enforce project/action scope, but do not consistently enforce the stored secret's actual environment and key-name restriction before handler execution. A caller-supplied query environment is not evidence of the secret's environment.

Implemented a metadata-only repository lookup (`backend/internal/repository/secret_access.go`) and a shared secret-route guard (`backend/internal/api/secret_scope.go`, wired in `router.go`). It resolves project/secret/environment/key from storage and enforces the existing API-key scope grammar before any secret/history decryption or mutation. Missing and out-of-scope resources share a 404. Human JWT behavior and collection routes stay as currently defined. This adds no new state-mutating endpoint.

`TestSecretIDScopeBoundary` uses real embedded SQLite migrations, real API-key/JWT middleware and the actual production router. Its 23 cases cover allowed/denied reads, updates, deletes, version list/read, forged environment query, cross-project IDs, read-only keys and missing credentials. Nine denial cases failed before the fix (unexpected 200/204); all pass after it. Denied requests preserve secret values and mutation-audit counts; permitted mutations retain their canonical audit event, allowed reads return their expected value, and permitted updates persist it. The full backend race suite, `go vet ./...` and diff whitespace checks also passed. See the [validation record](../validation/2026-09-28-backend-intake/README.md).

Security/Tech Lead review remains an integration gate for the resulting diff. No merge, push, deployment or real-vault operation is part of this intake.

## Recommended feature sequence (proposed, not delivered)

| Milestone | User outcome | Acceptance boundary |
| --- | --- | --- |
| 1. Encrypted history and restore | Recover a previous value after an accidental edit | Every supported write path records an atomic version; concurrent edits do not lose versions; scoped history access; restoring appends a new version and audit event; versions remain decryptable after DEK rotation; failure leaves current state unchanged. |
| 2. Recoverable backups and restore drill | Recover a project after loss | Versioned snapshot includes actual recovery data; legacy metadata-only snapshots are identified as non-restorable; preview conflicts before applying; restore only into an isolated target for initial acceptance; corruption/wrong key/foreign project rejected; no partial restore. Track with FOLLOWUPS #2 and `PRODUCT_RELIABILITY_2026-09.md`. |
| 3. App/agent connection check | Know an integration works with the intended restrictions | One allowed synthetic read succeeds; disallowed key/environment and expired credential fail; results and examples never reveal values or tokens. |

For milestone 1, settle retention and deletion semantics explicitly: existing foreign keys deliberately delete secret history when its secret is deleted. Do not introduce a recycle bin or change that promise incidentally. Design each recovery format and key-rotation interaction before implementation; a changed key hierarchy or breaking schema elevates the decision class and requires an ADR.

Team permissions, approval workflows and change notifications are an alternative subsequent feature track, pending the owner's priority response. GitHub/Google login still requires provider app setup; local fixtures are not proof of live OAuth completion.

Evidence logs for this intake: `/mnt/data/keepsave-backend-baseline-2026-09-28/`.

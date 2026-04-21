## Follow-up delta for v1.1.0 SECURITY_AUDIT

This PR (branch `claude/review-keepsave-status-lCbyI`) closes two of the
four v1.1.0 follow-ups listed in SECURITY_AUDIT.md. Updated status:

- [x] **Nightly audit-log pruner wiring in `main.go`** (was: knob exists;
      consumer pending). Closed by `startAuditLogPruner` in
      `backend/cmd/server/main.go` + `AuditRepository.DeleteOlderThan` +
      table-driven tests in `backend/internal/repository/audit_repo_test.go`.
- [x] **KeepSave <-> Seidr regression harness** (new; was implicit in the
      v1.0-rc Seidr integration but never tested here). Closed by
      `tests/e2e/seidr/` (docker-compose + stdlib-only Go tester).

Remaining (unchanged):

- [ ] **Backup tamper-detection test** to formalize AEAD-auth guarantee.
- [ ] **AWS KMS / GCP KMS SDK adapters** wired in `main.go` (interface +
      `kms_aws.go` / `kms_gcp.go` already exist in
      `backend/internal/crypto/keyprovider/`; blocked on running
      `go mod tidy` locally to add `aws-sdk-go-v2/service/kms` and
      `cloud.google.com/go/kms/apiv1` to `go.sum`).
- [ ] **Phase 15 service unit tests** (drift, anomaly, usage-analytics,
      recommendation, nlp-query). Feature-complete but untested; writing
      them requires a SQLite test harness that doesn't yet exist in the
      repo and belongs in a dedicated PR.
- [ ] **Seidr-runtime boot** against this KeepSave in the E2E harness.
      Currently the tester mimics `KeepSaveSecretProvider.Get`'s HTTP
      contract; booting a real Seidr container needs that image
      published to a registry this compose can pull from.

Once those four close, SECURITY_AUDIT.md v1.1.0 can be re-stamped v1.1.1
and the "Known follow-ups" block removed.

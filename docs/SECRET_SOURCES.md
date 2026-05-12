# Secret Sources Map (DevOps 30-day)

Where does every secret KeepSave depends on for its own operation live? Who can read it? How is it rotated? This is the bootstrap-paradox doc — KeepSave stores other people's secrets, so its own secrets need a clear story.

This is DevOps Engineer 30-day work item §1 from `docs/ROLES_30_60_90.md`. Required reading before any production deployment.

---

## The secrets KeepSave itself needs

| Secret               | Used by                                        | Read access (production)                            |
|----------------------|------------------------------------------------|------------------------------------------------------|
| `MASTER_KEY`         | `internal/crypto/keyprovider` to wrap DEKs     | KeepSave process at startup; KMS service principals |
| `JWT_SECRET`         | `internal/auth` to sign / verify session tokens | KeepSave process; rotation operator                |
| `DATABASE_URL`       | `internal/repository` (Postgres connection)    | KeepSave process; on-call operator (read-only)      |
| OAuth client secrets | `internal/auth` (OAuth provider integrations)  | KeepSave process; integration owner                 |
| TLS private key      | Ingress (load balancer or sidecar)             | Ingress only; never reaches the Go process          |
| Webhook signing keys | Audit / event webhooks                         | KeepSave process; webhook receiver                  |

## Source per environment

### Local development (`docker-compose.yml`)

| Secret         | Source                                   | Notes                                                                          |
|----------------|------------------------------------------|--------------------------------------------------------------------------------|
| `MASTER_KEY`   | Hard-coded env value in `docker-compose.yml` | **Dev only.** Value `43uH/WMSJGjGgaJseq39Mt0h5eAoGgElK3k53ddRZMM=` is a known test key. **MUST be different in every non-dev env.** |
| `JWT_SECRET`   | `dev-jwt-secret-change-me`               | Dev only.                                                                       |
| `DATABASE_URL` | Plaintext in compose                     | Dev only.                                                                       |

**Risk:** the dev `MASTER_KEY` value is in git. If someone reuses it in staging or production by accident, all secrets in that environment are effectively unprotected. Treat the dev key as a known-leaked value forever; never reuse it.

### Staging (Kubernetes via Helm chart `helm/keepsave/`)

| Secret         | Source                                              | Rotation                                            |
|----------------|-----------------------------------------------------|-----------------------------------------------------|
| `MASTER_KEY`   | Kubernetes `Secret` `keepsave-master`, populated from cloud KMS at deploy time. **Should** be ExternalSecrets / SealedSecrets backed; today is plain `Secret` (gap). | Manual; quarterly drill (per RUNBOOK §4). |
| `JWT_SECRET`   | Kubernetes `Secret` `keepsave-jwt`                  | Manual; 90 days.                                    |
| `DATABASE_URL` | Kubernetes `Secret` `keepsave-db`                   | When credentials rotate; via DB IAM if available.   |
| TLS key        | cert-manager via Let's Encrypt staging              | Automatic.                                          |

**Gap:** Helm chart currently expects plain Kubernetes Secrets. Production-grade deployments need ExternalSecrets / SealedSecrets / Vault-injector. Tracked as DevOps 60-day item.

### Production (Kubernetes + KMS)

| Secret         | Source                                                                                  | Read access                                                    |
|----------------|-----------------------------------------------------------------------------------------|----------------------------------------------------------------|
| `MASTER_KEY`   | **Not stored directly.** `MasterKeyProvider` calls KMS (AWS / GCP / Vault) at boot.    | KeepSave service account (KMS IAM grant); KMS auditors.        |
| `JWT_SECRET`   | KMS-encrypted Kubernetes Secret OR HashiCorp Vault dynamic secret                       | KeepSave service account.                                      |
| `DATABASE_URL` | Cloud-managed DB IAM token (preferred) OR rotated credentials via Vault                  | KeepSave service account.                                      |
| TLS key        | cert-manager via Let's Encrypt production OR cloud LB-managed certs                      | Ingress controller only.                                       |
| OAuth secrets  | Vault                                                                                   | KeepSave service account; integration owners (read-only audit).|

**Important:** the production `MasterKeyProvider` MUST NOT be `EnvProvider`. ADR-0004 §Operational calls this dev-only; `keyprovider/env.go:40-42` returns `ErrUnsupported` for rotation, so any rotation drill against `EnvProvider` fails. Wire the AWS / GCP adapters before going to production (`FOLLOWUPS.md` §1).

## Who can read what

| Principal                  | Dev | Staging | Production |
|----------------------------|-----|---------|------------|
| Local developer            | ✅   | ❌       | ❌          |
| CI runner                  | ❌   | ❌       | ❌          |
| KeepSave service account   | n/a | ✅       | ✅          |
| On-call operator           | n/a | ✅ (read-only via `kubectl get secret`) | ✅ (audited break-glass) |
| Tech Lead                  | n/a | ✅       | break-glass only |
| Random engineer            | n/a | ❌       | ❌          |

**Break-glass procedure** (production secret read) is documented in `docs/RUNBOOK.md`. Every break-glass read writes to an external audit log (cloud-provider audit, not KeepSave's own log — KeepSave is what's being recovered).

## CI runner secrets

CI does NOT need any production secret. The current `.github/workflows/ci.yml` does not reference `MASTER_KEY`, `JWT_SECRET`, or `DATABASE_URL`. If a future workflow needs DB access (e.g., migration smoke tests), it MUST use a CI-scoped dummy DB, not production.

`GITHUB_TOKEN` is auto-issued by Actions. Currently the workflow has **no explicit `permissions:` block**, which defaults to broad write access. This is addressed in the workflow edit accompanying this doc.

## Rotation cadence summary

| Secret         | Rotation cadence                                          | Drill cadence                                |
|----------------|-----------------------------------------------------------|----------------------------------------------|
| `MASTER_KEY`   | On suspicion of compromise; otherwise annual               | Quarterly table-top; semi-annual live (staging) |
| `JWT_SECRET`   | 90 days; immediate on incident                            | Per rotation event                            |
| `DATABASE_URL` | When credential expires (90 days max with rotation policy) | Per rotation event                           |
| OAuth secrets  | Per-provider; min annual                                  | Per rotation event                            |
| TLS key        | cert-manager handles automatically (60 days typical)       | Watch monitoring; alert if not renewed       |

## Migration to ExternalSecrets / SealedSecrets

Phase A 60-day item. Two viable paths:

- **External Secrets Operator** — preferred if AWS / GCP customer; binds Kubernetes Secrets to KMS-stored values without checking encrypted secrets into git.
- **SealedSecrets (Bitnami)** — preferred for fully-cluster-internal model; encrypts to a controller-held cluster key. Encrypted secrets safe to commit.

Decision is a Type-1 (key-source choice) → write an ADR before implementing.

## Open follow-ups

These feed `docs/FOLLOWUPS.md`:

1. Wire AWS / GCP KMS adapters in `main.go` (already tracked).
2. Choose ExternalSecrets vs. SealedSecrets via new ADR.
3. Add audit hook: every read of `MASTER_KEY` Kubernetes Secret in staging/production must emit a Kubernetes audit event consumed by SIEM.
4. Document break-glass procedure in `docs/RUNBOOK.md` (currently undocumented).

## References

- `docker-compose.yml` (dev values)
- `helm/keepsave/values.yaml`, `helm/keepsave/templates/` (staging/production patterns)
- ADR-0004 (key hierarchy)
- `docs/RUNBOOK.md` §1 (lost master key)

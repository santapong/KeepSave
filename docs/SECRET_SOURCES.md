# KeepSave credential custody and configuration sources

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Reconciled 2026-10-04 against current source. The canonical project is
`/mnt/data/company/apps/KeepSave`; actual private configuration and recovery
material must live **outside** it. This describes the self-hosted reference,
not a deployed key service. Read the [control reference](../deploy/self-hosted/README.md),
[runbook](RUNBOOK.md) and [threat model](THREAT_MODEL.md).

## Runtime custody

| Material | Source / location | Authorized consumer and boundary |
|---|---|---|
| Master wrapping material | Vault Transit plus retained Transit-wrapped master ciphertext | Trusted API/worker/recovery host; not frontend or isolated runner |
| Project encryption keys | Versioned encrypted vault records | Vault custody; retain keys referenced by values/history/promotion snapshots/backups |
| Browser JWT signing secret | Installation-private runtime configuration | API/identity; browser receives signed token only |
| PostgreSQL credential and CA | Private runtime/database files; `sslmode=verify-full` | API and trusted worker; separate runner has no database access |
| Google/GitHub sign-in client secrets | Backend-only runtime configuration | Social identity adapters, transient provider checks; never `VITE_` variables |
| GitHub App private key/token | Authorized encrypted broker connection | Broker performs structured authenticated upstream requests; no connector/model token |
| SMTP authenticated credential | Private trusted-worker runtime configuration | Certificate-verified STARTTLS mail adapter; no plaintext fallback |
| Proof delivery material | Short-lived Vault-encrypted payload, hash-only verification proof | Trusted mail adapter decrypts for sending; jobs contain identifiers only |
| Runner client certificate/key | Separate enrolled supervisor host, private regular files | Supervisor mTLS; connector receives only one-attempt Unix socket |
| Private listener server key/client CA | API-only private mount on control host | Direct TLS 1.3 verified runner listener; no forwarded-header identity |
| Recovery bundles | Private local storage plus independent external encrypted copy | Trusted verification/recovery CLI; exclude source sessions/grants/tokens/approvals/results |
| Operation results | Short-lived encrypted run-bound storage | Authorized result retrieval; excluded from backups, audit exports, logs and job payloads |

The broker's GitHub App is distinct from the GitHub OAuth **sign-in** app. KeepSave
OAuth delegation credentials authorize KeepSave, not GitHub. Model credentials,
subscription sessions and external secret-delivery adapters remain deferred.

## Development versus production

Development Compose deliberately uses disposable synthetic keys/passwords.
Those values are public test fixtures and must never be reused in staging,
production or real recovery. Production startup guards are a safety net, not
permission to reuse development material. Do not print fixture values in setup
instructions or copy them into a deployment checklist.

`KEEPSAVE_KEY_PROVIDER=vault` is the implemented production reference. The
`env` provider is development-only and does not supply a production rotation
procedure. AWS/GCP KMS adapter source exists but is **not wired** in the server;
startup rejects those selections. This is deferred expansion, not a prerequisite
to claim Vault Transit exists. Kubernetes/ExternalSecrets patterns in older
ADRs remain historical alternatives, not the chosen deployment.

Supply private files with mode 0600 and storage directories with mode 0700,
owned for the selected nonroot service identity. Follow the reference's actual
mount/user permissions. Store only file paths/placeholders in commands and docs.
Do not commit completed env files, keys, certificates containing private keys,
raw bundle recovery material or credential-bearing diagnostic output.

## Identity and authority

Public signup, provider email assertions and workspace ownership confer no
platform operator role. An operator-issued grant is keyed to an immutable user
UUID through `keepsave-operator`; deprecated email-admin configuration stays
empty. Provider identities use stable subject IDs; matching email does not merge
accounts automatically.

Browser tokens have tracked SID/JTI/hash/current status and a 24-hour maximum.
API keys, agent issuance, OAuth families and workload identities have distinct
lineage and revocation. All protected operations reconstruct current authority
from stored records. Database unavailability denies rather than using stale allow.
A secret read/export intentionally reveals permitted plaintext; revocation cannot
recall data that already reached a consumer's memory or environment.

## Recovery and rotation

Retain wrapping/key dependencies for every required ciphertext, history revision,
promotion snapshot and external bundle. Automatic history/key purge is disabled.
Rewrapping/project encryption-key rotation does not renew an upstream secret;
rotate the upstream credential separately after exposure.

Record a successful external-bundle/fresh-isolated-database drill using independent
recovery material before enabling daily 02:00 UTC backups. Retain 30 verified
scheduled bundles and at least two; manual/pre-upgrade bundles require explicit
deletion. A local file on the control host alone is not disaster recovery.

Recovery imports encrypted vault records, not original identity or authority.
Version 2 lifecycle owners require explicit mapping to permitted current members
or the new isolated custodian. Existing missing metadata remains unknown.
Compatible readers precede new writers; old session/vault binaries are unsupported
rollback targets. For key loss/incident access, follow the installation's reviewed,
externally audited two-person recovery process in [RUNBOOK](RUNBOOK.md).

## CI and evidence

CI uses synthetic databases/keys/provider fixtures and has no need for production
credentials. A workflow-scoped PostgreSQL DSN is test configuration, not proof that
CI should access an operator database. Real Transit, SMTP, social providers,
GitHub and native harness acceptance are separate authorized exercises.

The workflow has explicit least-privilege permissions; selected security publishing
jobs add `security-events: write`. Image signing remains a disabled scaffold.
See [CI_PERMISSIONS](CI_PERMISSIONS.md). Source checks and local tests do not prove
installation-specific key custody, delivery, recovery or independent review.

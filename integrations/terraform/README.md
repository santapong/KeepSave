# KeepSave Terraform example

Source audit: October 4, 2026. [`main.tf`](main.tf) is a retained configuration
example, not a KeepSave Terraform provider or a qualified production adapter.
It uses Terraform's external data source to read plaintext vault values, then
writes a local env file. See [integration status](../../docs/INTEGRATIONS.md),
[architecture](../../docs/ARCHITECTURE.md), [branding](../../docs/BRANDING.md) and
[acceptance ledger](../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## Actual interface

| Variable/output | Source behavior |
|---|---|
| `keepsave_api_url` | Installation origin, without `/api/v1`; source default is disposable localhost 8080. |
| `keepsave_token` | Sensitive Terraform variable containing a current human Bearer token. |
| `keepsave_project_id` | Selected project UUID. |
| `keepsave_environment` | Default `alpha`; explicit allowed environment for the caller. |
| `secret_keys`, `secrets_count` | Metadata outputs; the module does not export a secret-values map. |
| `local_file.env_file` | Writes `.env.<environment>` under `path.module` with permission 0600. |

A parent configuration must vendor the reviewed source and define/pass these
variables explicitly. Terraform examples referencing
`data.external.keepsave_secrets` outside this module's scope are incorrect; the
only module outputs are key names and count. Credential/provider setup and
published module distribution are not supplied by this source.

## Limits requiring qualification

The shell command interpolates the token and caller-controlled URL/project/
environment into program text; `sensitive=true` hides normal display but does not
remove credentials from process invocation or Terraform state. External data
values and generated-file content can persist as plaintext in state/backups.
The env writer does not provide robust multiline/escaping support. This example
needs hardened credential transport/input handling and synthetic qualification
before use; this documentation does not endorse arbitrary-input safety.

Human tokens are database-revocable 24-hour sessions, not permanent automation
credentials. Session/database failure must stop new reads. Ordinary permitted
vault release remains distinct from broker provider-token custody, and revocation
cannot recall state/files already created. Protect/delete state and generated
artifacts explicitly; never commit or share them as examples.

The Terraform/provider/CI integration journey was not executed during this audit.
New journal/recovery/tool-platform guarantees are PostgreSQL-only; legacy source
availability is not equivalent support. External secret-delivery adapters remain
deferred until their own tested contract and operator acceptance exist.

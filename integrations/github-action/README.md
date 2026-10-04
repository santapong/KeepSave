# KeepSave GitHub Actions example

Source audit: October 4, 2026. This repository retains a composite vault-read
example in [`action.yml`](action.yml); it is not a published/qualified marketplace
action or the later credential-confined GitHub App broker. The earlier
`santapong/keepsave-action@v1` reference has no publication evidence here. See
[integration status](../../docs/INTEGRATIONS.md),
[acceptance ledger](../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
and [branding](../../docs/BRANDING.md).

## Local source use

Only after reviewing the implementation and qualifying it on synthetic keys,
check out an immutable KeepSave revision into your repository/workspace's
`vendor/KeepSave`. A local example then refers to the actual source path:

```yaml
- name: Fetch permitted configuration
  uses: ./vendor/KeepSave/integrations/github-action
  with:
    api-url: ${{ secrets.KEEPSAVE_API_URL }}
    api-key: ${{ secrets.KEEPSAVE_API_KEY }}
    project-id: 'approved-project-uuid'
    environment: 'alpha'
    export-to: 'json'
    env-file-path: '.keepsave-input'
```

`api-url` is the installation origin, without `/api/v1`; the source adds that
prefix. This example assumes the reviewed local source already exists; no
external action distribution is installed. Do not echo returned values or publish
the generated file as an artifact.

| Input/output | Actual source behavior |
|---|---|
| `api-url`, `api-key`, `project-id` | Required; `X-API-Key` requests the project's environment secret list. |
| `environment` | Required input with source default `alpha`. |
| `export-to` | `env` default, `file`, or `json`; no general unsupported-mode validation. |
| `env-file-path` | Default `.env`; JSON mode writes this path plus `.json`. |
| `secrets-count` | Number of returned records, not provider acceptance. |

## Limits requiring qualification

The action deliberately exports authorized plaintext into the job environment or
files. It does not retain credentials inside a broker. Its `env`/`file` modes
write raw `key=value` lines without robust multiline/value validation; validate
keys/values, output permissions and cleanup before using them. Inputs are also
interpolated into shell code in this retained example; caller review is required
and this documentation does not assert arbitrary-input safety. Automatic masking
of returned values is not implemented. API-key scope/expiry and current project
membership remain server checks; revocation cannot recall values already read.

New platform guarantees are PostgreSQL-only. No real GitHub Actions job, published
action reference or deployment was executed in this documentation audit. Further
adapters and hardened delivery remain deferred under the [architecture](../../docs/ARCHITECTURE.md).

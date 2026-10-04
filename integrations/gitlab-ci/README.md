# KeepSave GitLab CI example

Source audit: October 4, 2026. The retained
[`keepsave.gitlab-ci.yml`](keepsave.gitlab-ci.yml) is a vault compatibility example,
not a qualified secret-delivery adapter. See the [documentation hub](../../docs/README.md),
[integration status](../../docs/INTEGRATIONS.md),
[acceptance ledger](../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
and [branding](../../docs/BRANDING.md).

Review and vendor an immutable template revision into your own GitLab repository
before using a `local` include. The original `local` path assumes this file is
present in that repository; it does not fetch a separate published integration.

```yaml
include:
  - local: 'vendor/KeepSave/integrations/gitlab-ci/keepsave.gitlab-ci.yml'
```

| Template | Inputs and actual effect |
|---|---|
| `.keepsave-pull` | Installation origin, masked project read API key/project ID and `KEEPSAVE_ENVIRONMENT` (default `alpha`); reads plaintext and evaluates generated shell exports. |
| `.keepsave-pull-to-file` | Same read inputs plus `KEEPSAVE_ENV_FILE` (default `.env`); writes a plaintext env file. |
| `.keepsave-promote` | Current human `KEEPSAVE_TOKEN`, project ID and adjacent `KEEPSAVE_FROM`/`KEEPSAVE_TO`; submits a promotion request using Bearer authentication. |

`KEEPSAVE_API_URL` is the installation origin without `/api/v1`. Keep actual
credentials in protected CI variables, not committed examples. Human tokens have
a database-revocable 24-hour SID, so this is not unattended permanent login or a
refresh-session integration. API keys cannot drive the human promotion route.

## Current limitations

The environment template uses `eval` on secret-derived exports and does not
validate arbitrary key names. It requires hardening and synthetic adversarial
qualification before use; do not treat it as safe for untrusted vault keys.
File mode needs explicit permission/cleanup and value-format review. Returned
values are not automatically masked; avoid log/artifact/container-build inclusion.
The source template's “Promotion complete!” message means only that HTTP succeeded:
production promotion is pending until a different eligible approver acts. This
example supplies no approval/status/revision-safe workflow.

Revocation prevents future server admissions but cannot recall CI data already
exported. This is ordinary permitted credential release, separate from the
broker-held GitHub App token flow. New platform guarantees remain PostgreSQL-only.
No real GitLab run or delivery/production acceptance occurred in this audit;
retained source and examples are not a support claim.

# Docs Sanitization Audit (Tech Writer interim — Phase A)

Docs that contain example secrets in shell snippets, screenshots with real values, or curl examples with plaintext credentials are a leak surface. This audit walks every doc file and flags anything that needs sanitization. Tech Writer is not yet hired in Phase A; interim owner is Tech Lead.

This is Tech Writer / Developer Advocate 30-day work item §1 from `docs/ROLES_30_60_90.md`.

---

## Method

For each doc file under the repo (excluding `node_modules/`, `dist/`, `vendor/`):

1. Grep for credential-shaped strings: `MASTER_KEY=`, `JWT_SECRET=`, `Authorization: Bearer`, `password`, `api-key`, `apikey`, `token`, `secret`, base64 strings > 30 chars.
2. Inspect each hit. Classify:
   - **clean** — placeholder text (`<your-master-key>`, `xxxxx`, obvious sentinel).
   - **dev-only** — known dev value (e.g., the `docker-compose.yml` master key) that's already public and labeled as dev-only.
   - **leak risk** — looks real, no "example" / "dev only" annotation.
3. For each leak risk: open a fix issue.

## Findings (initial pass)

| File                                      | Hit                                                                          | Class         | Action                                          |
|-------------------------------------------|------------------------------------------------------------------------------|---------------|-------------------------------------------------|
| `docker-compose.yml`                      | `MASTER_KEY: 43uH/WMSJGjGgaJseq39Mt0h5eAoGgElK3k53ddRZMM=`                    | dev-only      | Add a comment `# DEV ONLY — never reuse` above the line. Tracked. |
| `docker-compose.yml`                      | `JWT_SECRET: dev-jwt-secret-change-me`                                        | dev-only      | Same. Already labeled by the value itself.       |
| `docker-compose.yml`                      | `POSTGRES_PASSWORD: keepsave_dev`                                             | dev-only      | Same.                                            |
| `README.md`                               | (audit — verify no real values in `curl` examples)                            | TBD           | Audit needed.                                    |
| `docs/RUNBOOK.md`                         | command examples include `aws kms ...` placeholders                          | clean         | OK.                                              |
| `docs/SEIDR_INTEGRATION.md`               | (audit — verify any "send a secret" example uses placeholders)               | TBD           | Audit needed.                                    |
| `docs/medqcnn_integration.md`             | same                                                                          | TBD           | Audit needed.                                    |
| `docs/nexus_integration.md`               | same                                                                          | TBD           | Audit needed.                                    |
| Code-sample `.go` files in `integrations/`| any literal `MASTER_KEY` / `JWT_SECRET` in non-test files                     | TBD           | Audit needed; if found, replace with `os.Getenv`. |

**Status:** initial pass identified `docker-compose.yml` as the obvious cluster of dev-labeled credentials. Other doc files need a second pass (Tech Writer interim work, 30 days).

## Rules going forward

### 1. Example values in docs

Every credential-shaped string in any doc MUST be one of:
- An obvious placeholder: `<your-api-key>`, `ks_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`, `replace-me`.
- A known-dev value labeled `DEV ONLY — never reuse` on the same or preceding line.
- A literal `os.Getenv("VAR_NAME")` or `${VAR_NAME}` reference (no value shown).

Real credentials in docs are a P1 incident.

### 2. Shell snippets

Bad:
```sh
curl -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
     https://api.example.com/v1/secrets
```

Good:
```sh
# Replace $TOKEN with the JWT from `keepsave login`.
curl -H "Authorization: Bearer $TOKEN" \
     https://api.example.com/v1/secrets
```

Always export the env var separately, not on the same line, so the secret isn't in the user's shell history adjacent to a working command.

### 3. Screenshots

If a screenshot shows a revealed secret value (e.g., a dashboard screenshot of "Reveal" clicked), redact the value with a black bar before committing. The dashboard's default mask state is fine to screenshot. Use a fake project named `example-corp` rather than a real one.

### 4. Output captures

Output of `curl` or CLI commands often includes tokens. Redact before pasting into docs:
```
{"token": "<redacted>", "expires_at": "2026-05-13T00:00:00Z"}
```

### 5. Issue / PR templates

Mention this rule in `CONTRIBUTING.md` (to be created during Tech Lead 30-day work). Reviewers reject PRs with new credential-shaped strings that aren't obviously placeholders.

## Detector

A pre-commit hook (and a CI step) runs a simple regex check:

```bash
#!/usr/bin/env bash
# scripts/check-docs-secrets.sh — fails if any unannotated credential pattern found in docs
PATTERNS=(
  '[A-Za-z0-9+/]{40,}={0,2}'            # base64 chunks
  'eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'   # JWT
  'ks_[a-f0-9]{32,}'                    # KeepSave API key prefix
  'sk-[A-Za-z0-9]{30,}'                 # generic API key pattern
)
# … grep all .md and .yml files under docs/ and README.md
# … exclude any line that contains "DEV ONLY" or "placeholder" or is in a code-fence labeled "redacted"
```

This is a 30-day deliverable; the script itself is small and fits in `scripts/`.

## OpenAPI / API reference completeness

Adjacent to sanitization: the API reference should be complete. Audit by enumerating handlers in `backend/internal/api/handlers_*.go` and checking each is documented somewhere (`docs/`, `swagger.yaml`, or a hosted OpenAPI spec). Gaps go into Tech Writer 60-day work.

## References

- `docker-compose.yml`
- `docs/ROLES_30_60_90.md` §9 (Tech Writer action plan)
- `docs/SECRET_SOURCES.md` (which credentials are real vs. example)

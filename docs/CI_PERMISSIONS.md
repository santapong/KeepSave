# CI Runner Permissions (DevOps 30-day)

`GITHUB_TOKEN` is auto-issued to every workflow run. Without an explicit `permissions:` block, Actions defaults to **broad write** (contents, packages, pull-requests, issues, actions, security-events). On a secrets product, that's a credential a workflow exploit can use to push malicious code or alter security alerts. We narrow it.

This is DevOps Engineer 30-day work item §2 from `docs/ROLES_30_60_90.md`.

---

## What we changed in this PR

`.github/workflows/ci.yml` now declares a top-level least-privilege default:

```yaml
permissions:
  contents: read
```

Effect: every job inherits `contents: read` only. No write to PRs, issues, packages, security-events, or repository contents. Jobs that need more must override locally.

## Job-by-job permission needs (current pipeline)

| Job                   | Needs                                                                 | Permission override            |
|-----------------------|-----------------------------------------------------------------------|---------------------------------|
| Backend Lint          | Reads source                                                          | inherit (`contents: read`)      |
| Backend Tests         | Reads source; uploads coverage artifact                               | inherit + `actions: write` (only required if using `actions/upload-artifact@v4` to publish to a different repo; on same-repo, `contents: read` is enough). Today: inherit. |
| Backend Build         | Reads source; builds binary                                           | inherit                         |
| Frontend Lint         | Reads source                                                          | inherit                         |
| Frontend Tests        | Reads source                                                          | inherit                         |
| Frontend npm audit    | Reads source; calls `npm audit`                                       | inherit                         |
| Frontend Build        | Reads source                                                          | inherit                         |
| Security Scan         | Reads source; uploads security-report artifact                        | inherit                         |
| Docker Build          | Reads source; builds images locally (no registry push currently)      | inherit                         |

**No job needs write access today.** If a future workflow needs to (e.g., push images to GHCR, create release tags), it MUST declare its own narrower `permissions:` block — not relax the default.

## Future jobs that will need permission overrides

| Future job                          | Permission needed                | Justification                                                |
|-------------------------------------|-----------------------------------|--------------------------------------------------------------|
| Release tagging / GitHub release    | `contents: write`                 | Create tags and release records.                              |
| Image push to GHCR                  | `packages: write`                 | Push container images.                                        |
| CodeQL / security-events publish    | `security-events: write`          | Required by `github/codeql-action/analyze`.                   |
| Auto-PR on Dependabot               | `pull-requests: write`            | Open / update PRs from Dependabot's branch.                   |
| Issue comment from CI bot           | `issues: write`                   | Post lint summaries on PRs.                                   |

Each of these should be added job-locally (`jobs.<id>.permissions:`), not raised to top-level.

## Branch protection requirements

Permissions alone do not stop a malicious PR from a fork running with elevated tokens. Pair this with:

- **Required status checks** on `main` for: Backend Lint, Backend Tests, Backend Build, Frontend Lint, Frontend Tests, Frontend Build, Frontend npm audit, Security Scan. (Configure in GitHub branch protection rules.)
- **No force-pushes** to `main` or release branches.
- **Required reviews** = 1 (minimum), plus CODEOWNERS for protected paths (see `docs/VETO_LIST_AUDIT.md` §3b).
- **`pull_request_target` is forbidden** in this repo's workflows — it grants the target branch's secrets to fork PRs.

## Secrets in CI

CI does **not** need any production secret. The current workflow does not reference `MASTER_KEY`, `JWT_SECRET`, or `DATABASE_URL`. If a future workflow needs:

- **DB tests:** spin up a Postgres service in the job (`services.postgres:`). Use a workflow-scoped throwaway DB. Never connect to production.
- **Integration tests against a real KMS:** never. Use a mock provider or skip those tests in CI; they belong in a separate `integration-staging` workflow gated by environment protections.
- **Image push:** use OIDC federation to short-lived cloud credentials, never long-lived secrets.

## Forking and PR-from-fork safety

PRs from forks run with read-only token by default (GitHub policy). With our explicit `contents: read`, fork PRs get exactly the same permission set as the maintainer-branch PRs — no privilege gap. This is the desired behavior.

## Verification checklist

- [x] Top-level `permissions: contents: read` set.
- [ ] Add CODEOWNERS (Tech Lead 30-day — separate file).
- [ ] Verify branch protection rules in GitHub UI match this doc (Tech Lead).
- [ ] Confirm no workflow uses `pull_request_target` (grep clean today; add a lint rule).
- [ ] Add an annual review to `docs/ROLES_30_60_90.md` quarterly retro section.

## References

- GitHub docs: "Workflow permissions for the `GITHUB_TOKEN`".
- `.github/workflows/ci.yml` (this PR's change).
- `docs/SECRET_SOURCES.md` (where production secrets *do* live — and why CI doesn't see them).

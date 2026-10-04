# KeepSave landing deployment boundary

The published landing stays at `keepsave.draveniq.dev`. Vercel serves this static
marketing surface; it is separate from the full self-hosted application intended
at `app.keepsave.draveniq.dev`. The former three-project/Vercel/Neon guide is
[preserved as historical](archive/DEPLOY_VERCEL_LEGACY.md) and is not the current
application setup procedure.

## Current source and branding

The preserved landing checkout is a supporting worktree. Use `git worktree list`
to locate it; the canonical repository is `/mnt/data/company/apps/KeepSave`.
[Project structure](PROJECT_STRUCTURE.md) documents the folder roles. The accepted
Event Horizon page and Field Twist mark remain intact; see [branding](BRANDING.md).

A backend/docs merge or push to `develop` does not publish that landing or activate
the full application. Keep landing-only build/routing configuration in its owning
worktree; verify the exact build and links before a separately authorized deploy.
Do not direct sign-in callbacks or provider secrets at the static marketing site.

## Full application

Use the [self-hosted deployment plan](DEPLOYMENT_PLAN.md),
[control-host reference](../deploy/self-hosted/README.md),
[social-login setup](SOCIAL_LOGIN_SETUP.md) and [operator runbook](RUNBOOK.md).
Platform-admin authority comes from operator-issued user-ID grants, never a
configured email allowlist. New delegated MCP auth is separate from social login
and general legacy OAuth issuance. Formal reviews, provider/recovery/isolation and
operational acceptance remain gates; see [status](STATUS.md).

# ADR-0025: Brokered credentials and isolated connector execution

- Status: Accepted for local implementation by sponsor instruction, 2026-09-28.
- Authority: the project owner explicitly requested implementation of the complete backend plan in this Codex task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not a reviewer sign-off, merge approval or production authorization.
- Scope: isolated `feat/backend-secure-tool-access-20260928` implementation and synthetic validation.

## Decision

Provider credentials remain in the trusted broker. A separate enrolled runner executes vetted immutable connectors in rootless per-job containers, with no host homes, sockets, control-plane credentials or direct provider network access. Brokered operations validate typed targets and live authority before provider requests. Admission is durable before dispatch; uncertain external outcomes are recorded and not blindly retried.

## Alternatives and tradeoffs

API-host subprocesses share the vault trust boundary. Raw environment injection exposes credentials and cannot provide recall; it remains a separately authorized legacy capability, excluded from the secure pilot. Full agent hosting is outside this release.

## Compatibility and threat boundary

Reference pilot is a read-only GitHub App connector for one repository and pinned commit, with ten-minute KeepSave authority. Upstream token expiry is independent. Rootless containers on a separate runner host reduce exposure but do not prove absence of sandbox escapes.

## Rollback

Kill switches reject new runs; revoke grants, cancel queued attempts, stop containers and attempt upstream revocation. Already-dispatched operations or returned data cannot be recalled.

## Required evidence

The applicable milestone gates in [the execution record](../design/2026-09-28-backend-platform/IMPLEMENTATION.md) must pass. Failures or absent live-provider evidence remain visible. Independent review is required before integration/release.

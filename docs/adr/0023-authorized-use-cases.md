# ADR-0023: Authorized use cases and bounded grants

- Status: Accepted for local implementation by sponsor instruction, 2026-09-28.
- Authority: the project owner explicitly requested implementation of the complete backend plan in this Codex task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not a reviewer sign-off, merge approval or production authorization.
- Scope: isolated `feat/backend-secure-tool-access-20260928` implementation and synthetic validation.

## Decision

Identity, policy and broker modules share typed principals, actions, authoritative resource attributes and deny-by-default decisions. Preserve existing API-key grammar through a compatibility adapter. Child grants are bounded by parent target, action, expiry and live revocation. New provider access requires an explicit binding; membership or skill text does not confer authority.

## Alternatives and tradeoffs

Continuing handler-only checks duplicates policy and leaves non-HTTP paths unprotected. A general policy scripting engine is deferred until the typed rule model proves insufficient.

## Compatibility and threat boundary

Existing lease principal attribution, MCP ownership and role corrections restore existing documented intent and are regression fixes. Resource-bound MCP OAuth is additive; existing clients remain behind explicit legacy adapters.

## Rollback

Default-disable new platform admission; keep revocation and evidence available. Existing scoped vault routes remain available. Never fall back to a broader legacy credential.

## Required evidence

The applicable milestone gates in [the execution record](../design/2026-09-28-backend-platform/IMPLEMENTATION.md) must pass. Failures or absent live-provider evidence remain visible. Independent review is required before integration/release.

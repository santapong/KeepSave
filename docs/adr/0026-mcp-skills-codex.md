# ADR-0026: MCP, approved skills and managed Codex compatibility

- Status: Accepted for local implementation by sponsor instruction, 2026-09-28.
- Authority: the project owner explicitly requested implementation of the complete backend plan in this Codex task.
- Independent Security Engineer and Tech Lead review: **pending**. This is not a reviewer sign-off, merge approval or production authorization.
- Scope: isolated `feat/backend-secure-tool-access-20260928` implementation and synthetic validation.

## Decision

Use the pinned official MCP Go SDK, explicit protocol compatibility, resource-bound OAuth with pre-registered Codex public clients and S256 PKCE, atomic code consumption and refresh rotation. Tool identities bind installation, artifact and schema digest. Skills use immutable Agent Skills manifests; governance is separate from instructions. Profiles pin reviewed skill/tool versions and constraints. Export Codex requirements and report tested versus unverified enforcement.

## Alternatives and tradeoffs

A proprietary protocol increases client maintenance. Tool annotations, display names and client-reported configuration cannot establish permission or attest a device. Automatic arbitrary skill scripts and public client registration are deferred.

## Compatibility and threat boundary

Codex CLI is the first adapter; native skill distribution is the baseline. Skills over MCP is negotiated and tested separately. No model-provider API key or subscription custody. Local administrator-installed settings are not remote attestation.

## Rollback

Disable new MCP admission without disabling revocation/status. Retain documented legacy management adapters. Unsupported installations are denied rather than executed on the API host. Changed artifacts require approval.

## Required evidence

The applicable milestone gates in [the execution record](../design/2026-09-28-backend-platform/IMPLEMENTATION.md) must pass. Failures or absent live-provider evidence remain visible. Independent review is required before integration/release.

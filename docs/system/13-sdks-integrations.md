# 13. SDKs and integrations

Part of the [system documentation](README.md), reconciled 2026-10-02.

Go (`sdks/go`), Node (`sdks/nodejs`), Python (`sdks/python`), the existing
`cmd/keepsave` API CLI, CI actions and Terraform are compatibility clients.
Source presence does not establish every advertised client path or new session/
restore behavior. The core now provides POST secret batch at
`/api/v1/projects/:id/secrets/batch`; the prior missing-route claim is historical.
The standalone Go SDK local module now exists with a dedicated CI test job;
published distribution and SDK/widget cache/401/UAT gates remain individually tracked in the [acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md).

Clients authenticate with an exact project-scoped API key or active human session.
API-key precedence and scope behavior remain compatibility contracts. A vault
read returns authorized plaintext to that caller; putting it in a developer
runtime is not future broker credential confinement. Retry of mutations must
respect explicit idempotency/revision semantics rather than blindly replay
unknown outcomes. Caller-side cached data cannot be recalled by revocation.

The embedded widget lives on an integrator's host page and remains a separate
trust domain. Origin policy does not contain a malicious host with DOM control.
The new same-origin app topology does not remove the widget's third-party-origin
requirements. No widget/browser or SDK production UAT is inferred from backend
permission fixtures.

Google/GitHub login authenticates people. It is separate from a GitHub App
provider connection. Old OAuth issuance, enterprise SSO, MCP execution/build/
install/config generation, webhook automation and unfinished intelligence are
unavailable in the core profile. MCP registry/catalog metadata is compatibility,
not a standards-based `/mcp` server or a supported Codex connection.

M2 delivers official MCP and resource-bound OAuth/consent against a synthetic
tool, then a real supported Codex contract. M3 delivers broker-held read-only
GitHub App credentials, explicit repository bindings/runs and a separate restricted
runner. M4 delivers instruction-only immutable private skills and profiles with
verified managed Codex requirements. M5 proves self-hosted team operations and
safe durable external effects. See [architecture](../ARCHITECTURE.md) for the
ordered plan and exact enforcement boundaries. Additional harnesses/providers,
model-key relaying and arbitrary skill scripts/builds remain deferred.

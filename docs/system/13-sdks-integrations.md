# 13. SDKs, MCP and harness adapters

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

## Existing vault clients

Go (`sdks/go`), Node (`sdks/nodejs`), Python (`sdks/python`), `cmd/keepsave`, the
widget and CI/Terraform integrations remain compatible vault clients. The Go SDK
has its own local module and CI test job. POST
`/api/v1/projects/{id}/secrets/batch` is implemented with 1–100 keys,
`{environment,keys}` and `{secrets,missing_keys}`; its exact POST is a read operation.
Out-of-scope records are concealed as missing before decryption.

These clients use an active browser Bearer session or narrowed API-key/agent
lineage. Authorized reads intentionally export plaintext to that caller. Mutation
retry must honor current revisions/idempotency; unknown outcomes cannot be blindly
replayed. Server revocation cannot recall cached values. Source/build/route tests
are not evidence for every published SDK/widget/integration journey; use
[integration notes](../INTEGRATIONS.md) and the dated
[acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md).

## Harness-neutral MCP and OAuth candidate

The source now implements stateless Streamable HTTP `/mcp` using official Go SDK
v1.8.0. Tested synthetic protocol lanes are 2026-07-28 and 2025-11-25. Delegated OAuth
has configured discovery/consent, exact canonical `/mcp` resource, S256 public
clients, one-time hashed codes, opaque ten-minute access, rotating refresh-family
replay denial and current browser-parent checks. It is separate from browser JWTs
and GitHub App credentials. Legacy OAuth issuance and API-host MCP builds/execution
stay disabled; registry metadata is not the new transport.

Installation-qualified tools include `keepsave_v1__available_runs`,
`repository_tree`, `read_file`, `operation_status`, `operation_result`,
`cancel_operation` and `cancel_run` under the same prefix. Read tools queue durable
operations; status is metadata and explicit result delivery reauthorizes. Schemas
accept run/request keys and bounded paths, never arbitrary URLs/headers/commands.
Cancellation of accepted work uses an explicit tool independently of native RPC
interruption. The broker alone makes authenticated GitHub reads against stored
repository/install/commit scope, and only a separate enrolled supervisor executes
the digest-pinned connector reference.

## Native packaging and version-specific qualification

| Candidate | Dedicated client and callback | Status |
|---|---|---|
| Linux Codex 0.153.3 | `keepsave-codex-linux-v1`, `http://127.0.0.1:17701/callback` | Native package/parser contracts pass; real client acceptance pending. |
| Isolated Hermes 0.21.5 / v2026.9.24 | `keepsave-hermes-linux-v1`, `http://127.0.0.1:17702/callback` | Explicit legacy/strict-redirect package; real client acceptance pending. |

Both packages declare the 2025-11-25 qualification lane; SDK support for a modern
lane does not prove either client negotiated it. The existing Hermes 0.21.3 and
its selected non-Anthropic provider/session/memory stay separate. Check callback
port availability; do not silently substitute callbacks or add weaker fallback.

Portable profiles and immutable instruction sources are independent of native
packages. Each run is bound to one client's owned delegation, so clients share a
profile through separate runs. The pilot source is a single bounded instruction-
only `SKILL.md`; reference-file trees and skills-over-MCP are deferred. Native
export/check/unpack validates exact manifests/digests and uses new private output
directories without overwriting user settings. Metadata check does not launch a
harness or attest local enforcement. Changed/revoked artifacts cannot obtain new
server authority.

Additional harnesses implement the same protocol/native-export ports without
placing harness-specific types in shared policy/runs. They require their own
version/protocol/auth/native-discovery/cancellation evidence. Managed defaults,
administrator controls and server enforcement remain separately labeled. See the
[protocol/package receipt](../design/2026-10-02-harness-neutral-platform/PROTOCOL.md)
for source pins, safe import commands and exact tests.

Live Google/GitHub/SMTP, both native harnesses, disposable GitHub App A/B/revocation,
separate-host isolation and operational drills remain unaccepted. No universal
compatibility, provider enrollment or production integration follows from this
source candidate.

# MCP protocol and exact Codex/Hermes test candidates

Checked October 2, 2026 (Asia/Bangkok). This records the local unreleased implementation and its checks. ADR0029 sponsor authorization permits local implementation; it does not supply independent Security/Tech Lead review, real-client acceptance or release approval. The platform flags default to false.

## Boundaries and selected versions

`backend/internal/mcpgateway` is a thin transport adapter over typed `internal/runs` operations. It has no SQL, provider credential decryption, process execution or arbitrary URL tool. `internal/mcpauth` owns public-client consent and stored OAuth lineage; its access credential is separate from the broker's GitHub App installation token. `internal/harness` renders public native configuration and the exact approved portable instruction source. A client package, skill or claimed tool name does not grant authority.

The transport uses the official Go SDK **v1.8.0**, pinned in go.mod/go.sum, with stateless Streamable HTTP, JSON responses, a 64 KiB request limit, and request cancellation propagation. The server selects only **2026-07-28** and **2025-11-25**. It emits no MCP session identifier; authenticated GET and DELETE do not open a session. The modern lane uses `server/discover` and per-request metadata; the compatibility lane uses legacy initialization. Synthetic tests execute the actual SDK HTTP handler in both lanes. Older protocol headers are rejected rather than silently accepted.

The Linux native candidates are Codex **0.153.3 / rmcp 3.1.3** and Hermes **v2026.9.24 / 0.21.5 / Python MCP 2.0.0**. Both packages declare the 2025-11-25 qualification lane. rmcp 3.1.3's default `LATEST` is 2025-11-25 despite its additional modern protocol types. Hermes configuration explicitly selects `protocol: legacy` and `strict_redirect_headers: true`. These are source-verified candidates, not accepted real clients.

The existing Hermes checkout remains unchanged: parent inspection reported clean HEAD `1e4952ddba1bc585416ad43438d60183380035cd`, metadata 0.21.3 and declared MCP 2.0.0. Qualification of 0.21.5 requires a separate isolated profile; this implementation did not download a harness, install native instructions or inspect/configure provider credentials. Hermes must retain the user's chosen **non-Anthropic provider**, with separate memory, session and authority.

## Resource-bound public OAuth

The canonical HTTPS application origin is also the authorization-server issuer. The one protected resource is that exact origin plus `/mcp`. Configuration rejects another host, userinfo, query, fragment, trailing slash or a resource path other than `/mcp`.

| Route | Behavior |
| --- | --- |
| `GET /.well-known/oauth-protected-resource/mcp` | Exact resource, canonical authorization server, `keepsave:tools`, header bearer method. |
| `GET /.well-known/oauth-authorization-server` | Code/refresh, public `none`, S256 and issuer response binding; no registration endpoint. |
| `GET /oauth/mcp/authorize` | Validates one copy of every security parameter; stores a five-minute pending request and redirects to app consent. |
| `GET /api/v1/mcp/consent` | Current tracked human-session preview; no state or PKCE proof. |
| `POST /api/v1/mcp/consent` | Explicit recent-human decision; rejects duplicate/unknown/trailing JSON. Returns only the fixed callback with state, issuer and code/denial. |
| `POST /oauth/mcp/token` | Form-only public-client code redemption or refresh, with exact resource and client. |
| `POST /oauth/mcp/revoke` | Possession-based family revocation; unknown tokens and client mismatch return empty success. |
| `GET /api/v1/account/delegations` | Up to 100 owned live families, metadata only, including whether native refresh is needed. |
| `DELETE /api/v1/account/delegations/{familyId}` | Current human can revoke an owned family atomically with audit. |

The pre-registered public identifiers and callbacks are fixed:

| Client | Exact callback | Native fields |
| --- | --- | --- |
| `keepsave-codex-linux-v1` | `http://127.0.0.1:17701/callback` | `[mcp_servers.keepsave.oauth] client_id`, `callback_port=17701`, `callback_url`; requested scopes include `keepsave:tools` and `offline_access`. |
| `keepsave-hermes-linux-v1` | `http://127.0.0.1:17702/callback` | `mcp_servers.keepsave.oauth.client_id`, `token_endpoint_auth_method: none`, `redirect_host: 127.0.0.1`, `redirect_port: 17702`, scope. |

No client secret, arbitrary dynamic registration, client metadata fetch, shared provider credential, OAuth bridge or weaker fallback is implemented. Missing/mismatched resource, callback, PKCE, current parent or registered client fails closed. Codex exports omit `oauth_resource`: the pinned rmcp adds the resource to authorization, exchange and refresh itself; the inspected Codex override would add another authorization resource parameter, while this server deliberately rejects duplicates. The server advertises and returns RFC9207 `iss`, which avoids the inspected clients' issuer-omission fallback behavior.

Codes are random 256-bit opaque values with **exactly sixty-second expiry** and atomic one-time redemption. Access credentials last at most ten minutes. Refresh families last at most eight hours from consent, bounded by the original human SID expiry. Only SHA256 token/code hashes are stored. Rotation consumes the old refresh token; replay atomically revokes its family, including the successor access credential. Consent, issuance, rotation and revocation require audit in the same transaction. Nothing successful is returned before commit.

The shared subject/session authority barrier precedes consent, family and token locks. Standalone OAuth transactions set five-second lock and ten-second statement timeouts, including legacy audit statements. `RequireGrantTx` checks the exact stored token ID, unrotated access deadline, resource, client, family, consent and original parent after domain authority locks. `FamilyPrincipalTx` narrows a current human's browser management action to a stored current **OAuthDelegation**; it never reconstructs a bearer credential. Nonhuman management, another user's family and another client are rejected. Runs reauthorize the delegation independently of initial HTTP validation.

Every metadata, OAuth and MCP endpoint checks a present Origin against the exact configured HTTPS allowlist. Native clients may omit Origin. The router places this check before CORS preflight and authentication. Token endpoints reject authorization headers, secrets/assertions, duplicate fields and query parameters. The browser consent journey allowlists only `/mcp/consent?request_id=<UUID>` as a sign-in return; social login stores it inside the existing state-bound proof. Approval and denial validate the exact loopback callback and canonical issuer before navigation. Access/refresh credentials are never rendered on a page.

## Tools, schemas and bounded result delivery

The reviewed core installation is `keepsave_v1`. Each tool publishes its installation, logical operation and SHA256 input-schema digest in `_meta`; `internal/mcpgateway/catalog` is the shared source for runtime listing and exported `references/tools.json`.

| Tool | Admission / delivery |
| --- | --- |
| `keepsave_v1__available_runs` | Current account/client run metadata. |
| `keepsave_v1__repository_tree` | Queue an approved, commit-bound read with an idempotent request key. |
| `keepsave_v1__read_file` | Queue an approved path read; no URL, shell or credential fields. |
| `keepsave_v1__operation_status` | Metadata/receipt only; optional wait is at most 2000 ms. |
| `keepsave_v1__operation_result` | Explicit plaintext delivery after current stored domain and exact delegation authority checks. |
| `keepsave_v1__cancel_operation` | Durable cancellation of an owned operation. |
| `keepsave_v1__cancel_run` | Durable cancellation of an owned run, preventing later dispatch. |

Input schemas reject unknown fields. Queue/reconciliation/status responses defensively remove any result payload even if a domain DTO accidentally includes one. Domain errors become a fixed unavailable message rather than SQL, storage or provider error text.

New runs capture their admitted repository ID and owner/name, environment ID/name, reference and GitHub installation ID before reference resolution. Discovery, management listing and run status return that immutable snapshot plus the resolved commit. Changing the current binding, environment, reference or installation denies subsequent operation admission and protected result access; it cannot silently substitute another target. Migration 033 deliberately leaves older missing snapshots unknown: their repository/installation IDs serialize as 0, repository/environment as empty strings and environment ID as the zero UUID. A stored reference may still be known independently. Those metadata sentinels grant no authority, and such runs cannot perform provider operations. The maintained ToolRun response schema permits the unknown sentinels without relaxing creation or authorization checks.

The controlled-tools kill switch denies new grants, runs, operations, runner claims, dispatch and protected result publication. Structural readiness retains currently authorized metadata, receipts, cancellation, revocation, scoped offboarding and expiry maintenance. A current human owner can inspect/cancel after a grant issuer or approver departs; departed authority still denies protected operations and results. Disabling then reenabling a flag does not resurrect revoked records.

The actual serialized MCP result, including legacy text fallback **and** structured content, must fit within a 4 MiB envelope with a 64 KiB reserve for transport/wrapper fields. The gateway measures the full representation; a payload-half estimate alone does not cover JSON escaping. The broker's payload cap is an additional bound, not a substitute. Tests cover escaped content that fits the logical payload limit but exceeds the complete wire envelope.

Stopping a native client RPC is separate from durable cancellation of accepted work. Go SDK request-abort propagation applies to the modern lane; separate legacy cancellation notifications cannot establish durable stateless operation cancellation. Hermes interrupt/future cancellation source is not evidence that all accepted server work stops. Required run cancellation uses the explicit owned tool. Already returned data cannot be recalled.

## Portable source and native packages

The first portable SKILL subset accepts only YAML `name` and `description` plus a nonempty instruction body, at most 64 KiB. Duplicate keys, unsupported extensions and unknown required controls are rejected. The renderer preserves the exact approved source and verifies its SHA256; it does not silently rewrite private instructions. Codex instructions use `.agents/skills/<name>/SKILL.md`; Hermes instructions use `skills/<name>/SKILL.md` relative to a separate `HERMES_HOME`.

Each package includes public MCP configuration, preserved SKILL.md, profile references, the qualified tool catalog, import guidance and an exact version/source/hash manifest. Required controls are the server baseline: credential custody, current authority, client-bound run, commit-bound target, result reauthorization, expiry, revocation, bounded output and explicit cancellation. Native Stop remains unverified, native skill immutability and skills-over-MCP are unavailable, and native tool filtering needs administrator evidence. Requiring one of those unsupported/unproven controls refuses the package.

`keepsave-harness export` writes only a new directory and never installs over user settings. `check` accepts an explicit regular `package.json` or `pyproject.toml`, verifies the expected project identity and version and checks every rendered file/digest; symlinks, unexpected files or changed configuration fail. It does not execute the harness or read provider settings. Even a matching candidate remains `real_harness_acceptance_pending`. Mutable local instruction copies are not server attestation.

The tool-platform package download is the complete neutral package JSON, including its text files and manifest. Unpack it into a new private directory before importing any native settings:

```sh
keepsave-harness unpack --package ./downloaded-package.json --out ./keepsave-hermes-candidate
keepsave-harness check --package ./keepsave-hermes-candidate --metadata ./isolated-hermes/pyproject.toml
```

`unpack` validates the declared package identity, every rendered file and the complete embedded/outer manifest through the pinned renderer. It rejects duplicate/unknown JSON, traversal, symlinks and existing output directories. Limits are 1 MiB input JSON, 32 files, 128 KiB per file and 256 KiB total content. Each input/output ancestor is opened and identity-checked through a pinned directory handle; replacing a pathname cannot redirect later writes. Output directories use mode 0700 and files use 0600 with exclusive creation through a confined directory handle. The command does not read native version metadata or install/configure a client. The separate `check` is metadata-only: the existing Hermes 0.21.3 is not a match for the isolated 0.21.5 candidate.

## Executed local checks

October 2, 2026: Go1.27.1 Docker runtime, fixed disposable PostgreSQL test alias, complete shipped migrations and unique schema per fixture. No production DSN or external provider was used.

- Full owned `go test -race ./internal/mcpauth ./internal/mcpgateway/... ./internal/harness ./cmd/keepsave-harness -count=1` passed after delegation support; subsequent focused composed-router and strict decision checks passed.
- The final focused race command covering every owned test prefix plus `./internal/api` passed: mcpauth4.192s, gateway4.140s, harness1.139s, command1.039s, API1.427s. Tests prove one code winner across two service instances, one refresh winner and family replay revocation, hash-only storage, bad resource/client/callback/PKCE/expiry/SID denial, audit rollback, owned family mapping/revocation, Origin-before-auth/preflight, exact schemas, typed delegation, result-only plaintext and complete wire limit.
- Final `go vet ./internal/mcpauth ./internal/mcpgateway/... ./internal/harness ./cmd/keepsave-harness` passed after the bounded-transaction helper; root's final full validation remains the aggregate receipt.
- Frontend consent/return/social tests: **7 files, 26 tests passed**. Production TypeScript/Vite build passed. The configured ESLint rules currently cover the embed SDK; targeting these application files produced ignored-file warnings, so this is not an application lint claim.
- The composed router test uses actual tracked JWT session issuance, consent, native public form exchange, owned delegation discovery/revocation and replay refusal. It does not use a fake handler router.
- Final focused Go 1.27.1 race checks passed with no skips: harness CLI 7 top-level tests / 28 test cases, native renderer 3 tests and architecture 4 tests; 14 top-level / 35 total cases. `/mnt/data/keepsave-neutral-2026-10-02/harness-final-tests.jsonl` records both candidate trees, self-consistent file/tree-digest tampering, malformed JSON, size/path limits, input/output symlinks, substituted ancestors, private permissions and refusal of existing outputs. The architecture check permits known Gin request query parsing while continuing to reject arbitrary SQL Query calls.
- The final aggregate PostgreSQL race gate passed all ten packages on migrations 001–033 after the password-recovery delegation fix, including MCP OAuth/gateway/harness, actual-router contracts, immutable run-scope discovery/listing/status, five binding-drift denial cases before credential custody and the kill-switch/departed-ancestor controls. Evidence is `/mnt/data/keepsave-neutral-2026-10-02/backend-postgres-recovery-final.log`; four focused PostgreSQL recovery tests also passed in `backend-recovery-focused-final.log`, covering retained key/lease/token revocation, required-audit rollback and concurrent mint/use denial. The [backend ledger](../../validation/2026-10-02-harness-neutral-platform/BACKEND.md) records source hashes, executed recovery scenarios and earlier failed/interrupted receipts. This synthetic aggregate is not real native-client acceptance.
- An earlier response-schema-only correction permits 0 for legacy unknown repository/installation metadata. A temporary test overlay outside Git validated actual known and unknown Run DTOs and refused negative repository IDs: `run-scope-contract-final.log`, Go 1.27.1 race-enabled API package 1.108s. Generated frontend types remained unchanged and passed the Node 24.21.0 `--check`. That schema correction preceded the final recovery PostgreSQL gate; it made no domain authorization or handler change.

## Remaining acceptance and release gates

Real fresh OAuth consent and native skill loading for the exact Codex and isolated Hermes candidates remain unexecuted. Capture actual issuer/resource/callback/PKCE exchange and refresh, resource omission refusal, parent/family revocation between requests, cross-client run denial, schema pins, cancellation/interruption and native mutable-package behavior. Keep Hermes's chosen non-Anthropic provider and separate memory/session.

Real GitHub App custody, restricted runner isolation/egress and execution, operator SMTP delivery, browser consent visual UAT, multi-instance restart/worker exercises and independent Security/Tech Lead/release review remain separate gates. No deployment, external provider action, commit or harness installation occurred in this protocol slice. Future harness adapters and protocol/skills extensions require their own primary-source verification, declared capabilities and the same domain acceptance contract.

## Pinned primary-source evidence

Source check date October2, 2026. Release tags and source pins support the candidate design; they do not prove installed/runtime acceptance.

- [Official Go SDK v1.8.0 release](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.8.0), [pinned Streamable HTTP source](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/mcp/streamable.go), [pinned SDK documentation](https://github.com/modelcontextprotocol/go-sdk/blob/v1.8.0/README.md).
- [MCP2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28), [MCP2025-11-25 authorization](https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization), [RFC8707 resource indicators](https://www.rfc-editor.org/rfc/rfc8707), [RFC9207 issuer response](https://www.rfc-editor.org/rfc/rfc9207).
- [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference), [Codex MCP configuration](https://learn.chatgpt.com/docs/extend/mcp?surface=cli), [Codex0.153.3 OAuth login source](https://github.com/openai/codex/blob/rust-v0.153.3/codex-rs/rmcp-client/src/perform_oauth_login.rs), [rmcp3.1.3 authorization resource handling](https://github.com/modelcontextprotocol/rust-sdk/blob/rmcp-v3.1.3/crates/rmcp/src/transport/auth.rs), [rmcp3.1.3 protocol default](https://github.com/modelcontextprotocol/rust-sdk/blob/rmcp-v3.1.3/crates/rmcp/src/model.rs).
- [Hermes v2026.9.24 release](https://github.com/NousResearch/hermes-agent/releases/tag/v2026.9.24), [pinned project dependencies](https://github.com/NousResearch/hermes-agent/blob/v2026.9.24/pyproject.toml), [fixed public-client OAuth and issuer callback handling](https://github.com/NousResearch/hermes-agent/blob/v2026.9.24/tools/mcp_oauth.py), [protocol and redirect behavior](https://github.com/NousResearch/hermes-agent/blob/v2026.9.24/tools/mcp_tool_transport.py), [interrupt handling](https://github.com/NousResearch/hermes-agent/blob/v2026.9.24/tools/mcp_tool_loop.py), [Python MCP2 OAuth provider](https://github.com/modelcontextprotocol/python-sdk/blob/v2.0.0/src/mcp/client/auth/oauth2.py).
- [Portable Agent Skills specification](https://agentskills.io/specification), [Codex skill locations](https://learn.chatgpt.com/docs/extend/skills), [Hermes native skill management](https://github.com/NousResearch/hermes-agent/blob/v2026.9.24/tools/skill_manage_tool.py). Native package support is independently bounded; a protocol proposal is not evidence of Go SDK skills-extension implementation.

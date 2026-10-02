# Primary-source research and design implications

Checked 2026-09-28. These sources inform the proposal; none proves KeepSave implements a standard. Current pages can change, so pin exact specification/SDK/artifact revisions in the implementation spike. A release candidate blog found during search was not used to infer release status; the official specification’s `latest` link resolved to `2026-07-28` at review time.

| Source | Observed guidance | KeepSave implication |
|---|---|---|
| [MCP specification 2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28) | Defines the current protocol surface and capabilities | Separate protocol compatibility from internal application APIs |
| [Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http) | Current revision uses per-request metadata; older revisions used sessions/initialization. Origin validation and cancellation matter. | Test current and legacy clients explicitly; do not build around remembered transport behavior |
| [MCP authorization](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) | Resource discovery and token audience binding; separate resource server and authorization server roles | Distinguish dashboard login, inbound MCP authorization and upstream provider grants |
| [MCP security guidance](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices) | Covers confused deputies, token passthrough, SSRF and subprocess-proxy risks | Policy/broker separation, safe destination checks, isolated third-party execution |
| [Official Go SDK](https://github.com/modelcontextprotocol/go-sdk) | Provides client/server/auth primitives and a published protocol compatibility table | Evaluate a pinned SDK release in the existing Go toolchain; verify its advertised features with fixtures |
| [Agent Skills specification](https://agentskills.io/specification) | `SKILL.md` plus optional supporting code/resources; `allowed-tools` support is experimental | Reuse the format; govern requested permissions independently of Markdown |
| [Skills extension overview](https://modelcontextprotocol.io/extensions/skills/overview) | Official `io.modelcontextprotocol/skills`; discovery and resource retrieval; host support still developing | Private catalog with a negotiated extension path and explicit adapter fallback |
| [Skills normative specification](https://github.com/modelcontextprotocol/ext-skills/blob/main/specification/stable/skills.mdx) | Defines origins, manifests, content verification and approval behavior | Preserve origin/digest identity; test changes, nested activation and caches before claiming support |
| [Skills working-group charter](https://modelcontextprotocol.io/community/working-groups/skills-over-mcp) | Records SEP-2640 as Final and merged on 2026-09-13 | Skills-over-MCP is an available standard direction, not a reason to invent a proprietary protocol |
| [Extension client matrix](https://modelcontextprotocol.io/extensions/client-matrix) | Extensions are opt-in; support differs by client | Maintain KeepSave’s own tested harness/version matrix; never assume universal compatibility |
| [Enterprise-managed authorization](https://modelcontextprotocol.io/extensions/auth/enterprise-managed-authorization) | Describes centralized IdP-mediated MCP access | Future enterprise adapter; verify offboarding and outstanding tokens end to end |
| [OAuth client credentials extension](https://modelcontextprotocol.io/extensions/auth/oauth-client-credentials) | Describes machine-to-machine MCP authorization | Distinct workload credentials for CI/services; no impersonation of a dashboard user |
| [OAuth security BCP, RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html) | Minimum privileges, audience restrictions, replay protection and refresh-token controls | Review the existing OAuth service against the full flow before using it as the MCP authorization server |
| [GitHub App installation tokens](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app) | Tokens can be narrowed to granted repositories/permissions; documented expiry is one hour | A ten-minute KeepSave run is enforced by the broker; it is not a claim of a ten-minute provider token |

## Research conclusions versus product hypotheses

Supported by sources: MCP has authorization/security requirements; Agent Skills has a reusable format; an official skills extension and enterprise authorization extensions exist; implementations require compatibility checks.

KeepSave design judgments: a modular monolith is the lowest-maintenance starting point for this repository; third-party execution warrants isolation; brokered credential use provides stronger ongoing control than exporting raw values; a private team catalog is a tractable first skill product.

Unvalidated product hypotheses: engineering teams will adopt KeepSave as the common credential boundary for their harnesses; one approved workflow provides enough value to justify onboarding; the proposed controls match a paying team’s security requirements. Validate those with a working pilot and observed use before expanding the roadmap.

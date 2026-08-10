# Integrations

Everything that plugs into KeepSave, in one place.

There are two kinds. **First-party integrations** ship in this repository and
are how most people wire KeepSave into a codebase or pipeline. **Partner
integrations** are separate products that use KeepSave as their secret vault,
identity provider, or MCP host — each has its own guide.

---

## First-party

These live in this repo and are versioned with it.

### SDKs

Fetch secrets at runtime without ever putting them in a `.env` file. Each SDK
authenticates with a scoped API key and caches in memory only.

| Language | Path | Package |
|---|---|---|
| Go | [`sdks/go/`](../sdks/go/) | `package keepsave` — see the note below |
| Node.js | [`sdks/nodejs/`](../sdks/nodejs/) | `@keepsave/sdk` |
| Python | [`sdks/python/`](../sdks/python/) | `keepsave` |

> **Note on the Go SDK.** It ships as a single source file with no `go.mod`
> of its own, and the repository has no root module — the only module is
> `backend/`. So it cannot be `go get`-ed at
> `github.com/santapong/KeepSave/sdks/go` today; vendor the file, or give the
> directory its own `go.mod`. Tracked in
> [`FOLLOWUPS.md`](FOLLOWUPS.md).

The pattern is the same in all three: the only configuration your service needs
is a KeepSave URL, an API key, and a project ID. Everything else is fetched.

```python
from keepsave import KeepSave

ks = KeepSave(url=os.environ["KEEPSAVE_URL"], api_key=os.environ["KEEPSAVE_API_KEY"])
secrets = ks.get_secrets(project_id=os.environ["KEEPSAVE_PROJECT_ID"], environment="prod")
```

### CI/CD

Inject environment variables at build or deploy time, scoped to the environment
being deployed.

| Integration | Path |
|---|---|
| GitHub Actions | [`integrations/github-action/`](../integrations/github-action/) |
| GitLab CI | [`integrations/gitlab-ci/`](../integrations/gitlab-ci/) |

### Infrastructure as code

| Integration | Path |
|---|---|
| Terraform provider | [`integrations/terraform/`](../integrations/terraform/) |

Manage projects, environments and API keys declaratively, so vault topology is
reviewed in the same pull request as the infrastructure that consumes it.

### Embeddable widget

A `<keepsave-widget>` Web Component that drops a secrets panel into any web app
with a single `<script>` tag. Shadow DOM isolates styles from the host page, and
`postMessage` is origin-restricted.

```html
<script src="https://your-keepsave-host/embed/keepsave-widget.js"></script>
<keepsave-widget
  api-url="https://your-keepsave-host"
  project-id="your-project-id"
  theme="dark">
</keepsave-widget>
```

See [`EMBED_STATE.md`](EMBED_STATE.md) for the state machine and
[`EMBED_ORIGIN_POLICY.md`](EMBED_ORIGIN_POLICY.md) for the cross-origin rules —
including why wildcard `postMessage` targets are forbidden.

### MCP clients

Any MCP-speaking client (Claude Desktop, Claude Code, or your own agent) can use
KeepSave's gateway as a single endpoint for every registered tool. KeepSave
resolves each server's secrets and injects them as environment variables at call
time, so the agent never receives the credential.

```bash
curl http://localhost:8080/api/v1/mcp/config -H "Authorization: Bearer <jwt>"
```

---

## Partner products

Separate products that use KeepSave. Each guide covers project setup, secret
import, environment promotion, MCP registration and OAuth client registration
for that product specifically.

| Product | What it is | KeepSave's role | Guide |
|---|---|---|---|
| **NEXUS** | Agentic AI "Company-as-a-Service" — every department staffed by an AI agent | Vault for LLM API keys, OAuth provider for the A2A gateway, MCP host for agent tools, per-environment spend limits | [`nexus_integration.md`](nexus_integration.md) |
| **MedQCNN** | Hybrid quantum-classical CNN for medical image diagnostics | Vault for DB/JWT/Kafka credentials, promotion across qubit-count tiers, MCP host for `diagnose` tooling | [`medqcnn_integration.md`](medqcnn_integration.md) |
| **Grovernance** | Governance platform | Secret storage and identity | [`grovernance_integration.md`](grovernance_integration.md) |
| **Seidr** | — | **Design only.** No code ships yet; the document fixes the contract so both sides agree on shape before implementation | [`SEIDR_INTEGRATION.md`](SEIDR_INTEGRATION.md) |

---

## The shape of an integration

Every integration above follows the same five steps. If you are wiring up
something new, this is the path:

1. **Create a project** — the unit of isolation. Secrets, API keys and OAuth
   clients are all scoped to it.
2. **Import secrets** — bulk-import an existing `.env`, or push keys
   individually. Values are sealed with AES-256-GCM before they reach storage.
3. **Issue a scoped API key** — read-only, bound to one project and one
   environment, so a compromised runtime key cannot reach production.
4. **Register an MCP server** *(optional)* — point KeepSave at a GitHub repo and
   declare `env_mappings`. The gateway injects those secrets at call time.
5. **Register an OAuth client** *(optional)* — use KeepSave as the identity
   provider instead of standing up a second one.

Steps 1–3 are the minimum. Full command sequences for each are in
[`system/03-api-reference.md`](system/03-api-reference.md).

## Adding a new integration

Partner guides live in `docs/` and are linked from the table above. Keep them
self-contained: a reader should be able to follow one guide end to end without
also reading another. Cross-reference the API reference rather than duplicating
endpoint documentation, so there is one place to update when an endpoint moves.

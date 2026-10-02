# Integrations

Integration inventory, reconciled 2026-10-02. Supported local exercises and
remaining client gates are in the [acceptance ledger](validation/2026-10-01-core-release/ACCEPTANCE.md).
MCP execution, old OAuth issuance, SSO and webhook automation are unavailable
in the current core profile; their guides are legacy/design references.

There are two kinds. **First-party integrations** ship in this repository and
are how most people wire KeepSave into a codebase or pipeline. **Partner
integrations** are separate products that use KeepSave as their secret vault or historical integration target — each has its own guide. A guide
does not establish a current end-to-end acceptance result.

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

> **Go SDK status.** `sdks/go/go.mod` now declares the standalone local module
> `github.com/santapong/KeepSave/sdks/go`, and CI has a dedicated SDK test job.
> No new published module version/tag or remote `go get` acceptance is claimed.
> Use a local module replacement for this uncommitted candidate; published
> distribution remains the tracked release step in [FOLLOWUPS](FOLLOWUPS.md).

The pattern is the same in all three: the only configuration your service needs
is a KeepSave URL, an API key, and a project ID. Everything else is fetched.

```python
from keepsave import KeepSaveClient

ks = KeepSaveClient(os.environ["KEEPSAVE_URL"], api_key=os.environ["KEEPSAVE_API_KEY"])
secrets = ks.list_secrets(os.environ["KEEPSAVE_PROJECT_ID"], "alpha")
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
| Terraform data-source consumer | [`integrations/terraform/`](../integrations/terraform/) |

The existing Terraform adapter consumes permitted secret values; it is not a
full KeepSave resource provider. Review its state/output handling before use.

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

Standards-based `/mcp` and resource-bound Codex OAuth are M2. The current core
refuses API-host gateway execution/build/install/config generation. Registry and
catalog metadata do not establish client compatibility or credential confinement.
The approved M3 broker makes structured authenticated GitHub requests and keeps
GitHub App tokens from the connector/model; social GitHub login is separate.
See the [ordered architecture plan](ARCHITECTURE.md).

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

For an authorized core vault integration, use these steps:

1. **Create a project** — the vault scope. Explicit workspace attachment requires
   stored personal ownership and destination administrator authority.
2. **Import secrets** — bulk-import an existing `.env`, or push keys
   individually. Values are sealed with AES-256-GCM before they reach storage.
3. **Issue a scoped API key** — read-only, bound to one project and one
   environment, so a compromised runtime key cannot reach production.
MCP/provider/harness setup is deferred to the specific M2–M4 slices. Core vault
clients receive authorized plaintext; caching cannot recall previously returned
data after revocation. The [API contract](system/03-api-reference.md) describes
current paths and safe revision/idempotency behavior.

## Adding a new integration

Partner guides live in `docs/` and are linked from the table above. Keep them
self-contained: a reader should be able to follow one guide end to end without
also reading another. Cross-reference the API reference rather than duplicating
endpoint documentation, so there is one place to update when an endpoint moves.

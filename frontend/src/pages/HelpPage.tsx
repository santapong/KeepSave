import { useState } from 'react';

type Section = 'getting-started' | 'concepts' | 'api' | 'api-keys' | 'sdks' | 'promotion' | 'embed' | 'security' | 'developer-guide' | 'agent-integration' | 'mcp-server' | 'examples';

export function HelpPage() {
  const [active, setActive] = useState<Section>('getting-started');

  const sections: { key: Section; label: string }[] = [
    { key: 'getting-started', label: 'Getting Started' },
    { key: 'concepts', label: 'Concepts' },
    { key: 'api', label: 'API Reference' },
    { key: 'api-keys', label: 'API Keys & Scopes' },
    { key: 'sdks', label: 'SDKs & CLI' },
    { key: 'promotion', label: 'Promotion Pipeline' },
    { key: 'embed', label: 'Embed Widget' },
    { key: 'security', label: 'Security Model' },
    { key: 'developer-guide', label: 'Developer Guide' },
    { key: 'agent-integration', label: 'Agent Integration' },
    { key: 'mcp-server', label: 'MCP Server' },
    { key: 'examples', label: 'Examples' },
  ];

  return (
    <div style={{ display: 'flex', gap: 32, minHeight: 'calc(100vh - 120px)', overflow: 'auto' }}>
      {/* Sidebar */}
      <nav style={sidebar}>
        <h3 style={{ fontSize: 13, fontWeight: 600, color: 'var(--color-text-secondary)', textTransform: 'uppercase', letterSpacing: '0.05em', marginBottom: 12 }}>Documentation</h3>
        {sections.map(s => (
          <button
            key={s.key}
            onClick={() => setActive(s.key)}
            style={{
              ...sidebarItem,
              background: active === s.key ? 'var(--color-primary-glow, rgba(99,102,241,0.15))' : 'transparent',
              color: active === s.key ? 'var(--color-primary-hover, #818cf8)' : 'var(--color-text-secondary)',
              fontWeight: active === s.key ? 600 : 400,
              borderLeft: active === s.key ? '2px solid var(--color-primary)' : '2px solid transparent',
            }}
          >
            {s.label}
          </button>
        ))}
      </nav>

      {/* Content */}
      <div style={{ flex: 1, maxWidth: 780, minWidth: 0 }}>
        {active === 'getting-started' && <GettingStarted />}
        {active === 'concepts' && <Concepts />}
        {active === 'api' && <APIReference />}
        {active === 'api-keys' && <APIKeysDetail />}
        {active === 'sdks' && <SDKs />}
        {active === 'promotion' && <Promotion />}
        {active === 'embed' && <EmbedWidget />}
        {active === 'security' && <SecurityModel />}
        {active === 'developer-guide' && <DeveloperGuide />}
        {active === 'agent-integration' && <AgentIntegration />}
        {active === 'mcp-server' && <MCPServer />}
        {active === 'examples' && <Examples />}
      </div>
    </div>
  );
}

function GettingStarted() {
  return (
    <div>
      <h1 style={pageTitle}>Getting Started</h1>
      <p style={intro}>KeepSave is a secure environment variable storage and promotion system. Here's how to get up and running.</p>

      <div style={stepCard}>
        <div style={stepNumber}>1</div>
        <div>
          <h3 style={stepTitle}>Create a Project</h3>
          <p style={stepDesc}>Go to the <strong>Projects</strong> page and click "New Project". A project groups your secrets across environments (Alpha, UAT, PROD).</p>
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>2</div>
        <div>
          <h3 style={stepTitle}>Add Secrets</h3>
          <p style={stepDesc}>Open your project, select an environment tab (ALPHA by default), and click "Add Secret". Each secret is a key-value pair encrypted with AES-256-GCM.</p>
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>3</div>
        <div>
          <h3 style={stepTitle}>Create an API Key</h3>
          <p style={stepDesc}>Open your project and click the <strong>API Keys</strong> tab. Create a scoped key for your application or AI agent — select scopes as checkboxes and optionally restrict to a single environment.</p>
          <CodeBlock code={`# Create an API key (JWT auth)
curl -X POST http://localhost:8080/api/v1/api-keys \\
  -H "Authorization: Bearer <your_jwt>" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"my-agent","project_id":"<uuid>","scopes":["read"],"environment":"alpha"}'`} />
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>4</div>
        <div>
          <h3 style={stepTitle}>Connect Your App</h3>
          <p style={stepDesc}>Use the raw key returned at creation time in the <code style={inlineCode}>X-API-Key</code> header. Your app never stores sensitive values in code or config files.</p>
          <CodeBlock code={`# Fetch all secrets for an environment
curl -H "X-API-Key: ks_your_key_here" \\
  "http://localhost:8080/api/v1/agent/secrets?env=alpha"

# Response: flat key-value map
{
  "DATABASE_URL": "postgres://...",
  "REDIS_URL":    "redis://..."
}`} />
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>5</div>
        <div>
          <h3 style={stepTitle}>Promote to Production</h3>
          <p style={stepDesc}>When ready, use the <strong>Promote</strong> tab to push secrets from Alpha → UAT → PROD. The diff view shows exactly what will change before you confirm. PROD promotions require approval.</p>
        </div>
      </div>
    </div>
  );
}

function Concepts() {
  return (
    <div>
      <h1 style={pageTitle}>Concepts</h1>
      <p style={intro}>
        Understanding how Projects, Environments, Secrets, and API Keys relate to each other is the foundation for using KeepSave effectively.
      </p>

      <div style={docCard}>
        <h3 style={docTitle}>Projects</h3>
        <p style={stepDesc}>
          A project is the top-level container for a set of related secrets — typically one per application or service. All secrets, environments, API keys, and audit history belong to a project. A user can own and manage multiple projects.
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Environments</h3>
        <p style={stepDesc}>Each project has three fixed environments. They are not user-configurable, which enforces a consistent promotion discipline across all teams.</p>
        <table style={{ ...apiTable, marginTop: 12 }}>
          <thead>
            <tr>
              <th style={apiTh}>Environment</th>
              <th style={apiTh}>Purpose</th>
              <th style={apiTh}>Typical use</th>
            </tr>
          </thead>
          <tbody>
            <tr style={apiTr}>
              <td style={apiTd}><span style={{ color: '#22c55e', fontWeight: 700 }}>ALPHA</span></td>
              <td style={apiTd}>Development</td>
              <td style={apiTd}>Local development, unit tests, rapid iteration. Low-risk secrets (test DB URLs, sandbox API keys).</td>
            </tr>
            <tr style={apiTr}>
              <td style={apiTd}><span style={{ color: '#818cf8', fontWeight: 700 }}>UAT</span></td>
              <td style={apiTd}>Staging / QA</td>
              <td style={apiTd}>Pre-production validation. Mirrors production topology. Shared by QA and CI pipelines.</td>
            </tr>
            <tr style={apiTr}>
              <td style={apiTd}><span style={{ color: '#f59e0b', fontWeight: 700 }}>PROD</span></td>
              <td style={apiTd}>Production</td>
              <td style={apiTd}>Live traffic. Requires approval for promotions. Access restricted via API key scopes and environment restrictions.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Secrets</h3>
        <p style={stepDesc}>
          A secret is a single key-value pair scoped to a project and environment. Values are encrypted at rest using AES-256-GCM with per-project envelope keys. The plaintext value is never logged and is only decrypted in-process when a request with valid credentials asks for it.
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>API Keys</h3>
        <p style={stepDesc}>
          API keys allow non-human callers — AI agents, CI/CD pipelines, microservices — to authenticate without a user session. Each key is scoped to exactly one project and can optionally be restricted to a single environment. The raw key is shown once at creation; only its SHA-256 hash is stored server-side.
        </p>
        <p style={{ ...stepDesc, marginTop: 8 }}>
          Keys are managed from the <strong>API Keys</strong> tab inside each project, keeping all project-related configuration in one place.
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>How They Fit Together</h3>
        <CodeBlock code={`Project "my-api"
  ├── Environment: alpha
  │     ├── SECRET: DATABASE_URL = postgres://localhost/dev
  │     └── SECRET: STRIPE_KEY  = sk_test_...
  ├── Environment: uat
  │     └── (secrets promoted from alpha)
  ├── Environment: prod
  │     └── (secrets promoted from uat, after approval)
  └── API Keys (API Keys tab)
        ├── "ci-pipeline"  scopes:[read,write]  env:alpha
        └── "prod-agent"   scopes:[read]         env:prod`} />
      </div>
    </div>
  );
}

function APIReference() {
  return (
    <div>
      <h1 style={pageTitle}>API Reference</h1>
      <p style={intro}>All endpoints are under <code style={inlineCode}>/api/v1</code>. Authenticate with JWT Bearer token or API key header.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Authentication</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="POST" path="/auth/register" desc="Create a new account" />
            <ApiRow method="POST" path="/auth/login" desc="Login, returns JWT token" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Projects</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="GET" path="/projects" desc="List all projects" />
            <ApiRow method="POST" path="/projects" desc="Create a project" />
            <ApiRow method="GET" path="/projects/:id" desc="Get project details" />
            <ApiRow method="DELETE" path="/projects/:id" desc="Delete a project" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Secrets</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="GET" path="/projects/:id/secrets?env=alpha" desc="List secrets for environment" />
            <ApiRow method="POST" path="/projects/:id/secrets" desc="Create a secret" />
            <ApiRow method="PUT" path="/projects/:id/secrets/:sid" desc="Update a secret value" />
            <ApiRow method="DELETE" path="/projects/:id/secrets/:sid" desc="Delete a secret" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Promotions</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="GET" path="/projects/:id/promotions/diff?source=alpha&target=uat" desc="Preview promotion diff" />
            <ApiRow method="POST" path="/projects/:id/promotions" desc="Create a promotion request" />
            <ApiRow method="POST" path="/projects/:id/promotions/:pid/approve" desc="Approve a promotion" />
            <ApiRow method="POST" path="/projects/:id/promotions/:pid/reject" desc="Reject a promotion" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>API Keys</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="GET" path="/api-keys" desc="List your API keys" />
            <ApiRow method="POST" path="/api-keys" desc="Create an API key" />
            <ApiRow method="DELETE" path="/api-keys/:id" desc="Delete an API key" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Agent Access (via API Key)</h3>
        <p style={stepDesc}>AI agents and services authenticate using the <code style={inlineCode}>X-API-Key</code> header:</p>
        <CodeBlock code={`curl -H "X-API-Key: ks_abc123..." \\
  http://localhost:8080/api/v1/agent/secrets?env=alpha`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Request / Response Examples</h3>
        <p style={stepDesc}><strong>Login</strong></p>
        <CodeBlock code={`POST /api/v1/auth/login
Content-Type: application/json

{ "email": "user@example.com", "password": "hunter2" }

# Response 200
{ "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." }`} />

        <p style={{ ...stepDesc, marginTop: 16 }}><strong>Create a Secret</strong></p>
        <CodeBlock code={`POST /api/v1/projects/{project_id}/secrets
Authorization: Bearer <jwt>
Content-Type: application/json

{ "key": "DATABASE_URL", "value": "postgres://...", "environment": "alpha" }

# Response 201
{
  "id": "3fa85f64-...",
  "key": "DATABASE_URL",
  "project_id": "...",
  "environment_id": "...",
  "created_at": "2026-03-16T10:00:00Z"
}`} />

        <p style={{ ...stepDesc, marginTop: 16 }}><strong>Create a Promotion Request</strong></p>
        <CodeBlock code={`POST /api/v1/projects/{project_id}/promotions
Authorization: Bearer <jwt>
Content-Type: application/json

{
  "source_environment": "alpha",
  "target_environment": "uat",
  "override_policy": "skip",
  "notes": "Ready for QA"
}

# Response 201
{
  "id": "...",
  "status": "pending",
  "source_environment": "alpha",
  "target_environment": "uat"
}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Error Codes</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Status</th><th style={apiTh}>Meaning</th></tr></thead>
          <tbody>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>400</code></td><td style={apiTd}>Invalid request body or missing required fields</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>401</code></td><td style={apiTd}>Missing or invalid JWT / API key</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>403</code></td><td style={apiTd}>Insufficient scopes or environment restriction</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>404</code></td><td style={apiTd}>Project, secret, or key not found</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>409</code></td><td style={apiTd}>Duplicate key in the same environment</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>500</code></td><td style={apiTd}>Internal server error — check API logs</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

function APIKeysDetail() {
  return (
    <div>
      <h1 style={pageTitle}>API Keys & Scopes</h1>
      <p style={intro}>API keys allow AI agents, CI/CD pipelines, and services to access secrets without a user session. Keys are scoped per-project and optionally per-environment.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Creating an API Key</h3>
        <ol style={{ ...docList, listStyleType: 'decimal' }}>
          <li>Open your project from the <strong>Projects</strong> page and click the <strong>API Keys</strong> tab.</li>
          <li>Click <strong>Create API Key</strong>.</li>
          <li>Enter a descriptive name (e.g. <code style={inlineCode}>ci-pipeline</code> or <code style={inlineCode}>prod-agent</code>).</li>
          <li>Check one or more <strong>Scopes</strong> using the checkbox group (see table below).</li>
          <li>Optionally restrict the key to a single <strong>Environment</strong> (Alpha, UAT, or PROD). Leave blank for all environments.</li>
          <li>Click <strong>Create Key</strong> and copy the generated key immediately — it is shown only once and stored as a hash.</li>
        </ol>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Available Scopes</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Scope</th><th style={apiTh}>What it allows</th></tr></thead>
          <tbody>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>read</code></td><td style={apiTd}>Fetch and list secrets, export .env files</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>write</code></td><td style={apiTd}>Create and update secret values</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>delete</code></td><td style={apiTd}>Permanently delete secrets from an environment</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>promote</code></td><td style={apiTd}>Create promotion requests and approve/reject them</td></tr>
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Using an API Key in Requests</h3>
        <p style={stepDesc}>Pass the key in the <code style={inlineCode}>X-API-Key</code> header on every request:</p>
        <CodeBlock code={`curl -H "X-API-Key: ks_abc123..." \\
  http://localhost:8080/api/v1/agent/secrets?env=alpha

# Response
{
  "DATABASE_URL": "postgres://...",
  "REDIS_URL": "redis://...",
  "JWT_SECRET": "s3cr3t"
}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Environment Restriction</h3>
        <p style={stepDesc}>If a key is restricted to <code style={inlineCode}>alpha</code>, requests for <code style={inlineCode}>uat</code> or <code style={inlineCode}>prod</code> will be rejected with <code style={inlineCode}>403 Forbidden</code>. This lets you issue agent keys that can never access production secrets.</p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Key Format</h3>
        <p style={stepDesc}>Keys are prefixed with <code style={inlineCode}>ks_</code> and are 32 random bytes encoded as base64url. Only the SHA-256 hash is stored server-side. If you lose a key you must delete it and create a new one.</p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Revoking a Key</h3>
        <p style={stepDesc}>Open the project's <strong>API Keys</strong> tab, find the key in the table, and click <strong>Delete</strong>. The key is immediately invalid — any in-flight requests using it will start receiving <code style={inlineCode}>401 Unauthorized</code>.</p>
      </div>
    </div>
  );
}

function SDKs() {
  return (
    <div>
      <h1 style={pageTitle}>SDKs & CLI</h1>
      <p style={intro}>KeepSave provides official SDKs for Go, Node.js, and Python.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Node.js</h3>
        <CodeBlock code={`npm install @keepsave/sdk`} />
        <CodeBlock code={`import { KeepSave } from '@keepsave/sdk';

const ks = new KeepSave({
  apiKey: 'ks_your_api_key',
  baseURL: 'http://localhost:8080',
});

// Fetch all secrets for an environment
const secrets = await ks.getSecrets('alpha');
console.log(secrets.DATABASE_URL);`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Error Handling</strong></p>
        <CodeBlock code={`import { KeepSave, KeepSaveError } from '@keepsave/sdk';

const ks = new KeepSave({ apiKey: 'ks_your_api_key' });

try {
  const secrets = await ks.getSecrets('alpha');
} catch (err) {
  if (err instanceof KeepSaveError && err.status === 403) {
    console.error('Insufficient permissions — check API key scopes');
  } else if (err instanceof TypeError) {
    console.error('Connection failed — is the KeepSave server running?');
  } else {
    console.error('Unexpected error:', err);
  }
}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Python</h3>
        <CodeBlock code={`pip install keepsave`} />
        <CodeBlock code={`from keepsave import KeepSaveClient

ks = KeepSaveClient(
    api_key="ks_your_api_key",
    base_url="http://localhost:8080",
)

secrets = ks.get_secrets("alpha")
print(secrets["DATABASE_URL"])`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Error Handling</strong></p>
        <CodeBlock code={`from keepsave import KeepSaveClient, KeepSaveError
import requests

ks = KeepSaveClient(api_key="ks_your_api_key")

try:
    secrets = ks.get_secrets("alpha")
except KeepSaveError as e:
    if e.status_code == 403:
        print("Insufficient permissions — check API key scopes")
    else:
        print(f"API error {e.status_code}: {e.message}")
except requests.ConnectionError:
    print("Connection failed — is the KeepSave server running?")`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Go</h3>
        <CodeBlock code={`go get github.com/your-org/keepsave-go`} />
        <CodeBlock code={`import "github.com/your-org/keepsave-go"

client := keepsave.NewClient(
    keepsave.WithAPIKey("ks_your_api_key"),
    keepsave.WithBaseURL("http://localhost:8080"),
)

secrets, err := client.GetSecrets(ctx, "alpha")
dbURL := secrets["DATABASE_URL"]`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Error Handling</strong></p>
        <CodeBlock code={`secrets, err := client.GetSecrets(ctx, "alpha")
if err != nil {
    var apiErr *keepsave.APIError
    if errors.As(err, &apiErr) && apiErr.StatusCode == 403 {
        log.Fatal("Insufficient permissions — check API key scopes")
    }
    if errors.Is(err, keepsave.ErrConnectionFailed) {
        log.Fatal("Connection failed — is the KeepSave server running?")
    }
    log.Fatalf("Unexpected error: %v", err)
}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>.env File Export</h3>
        <p style={stepDesc}>Export secrets as a <code style={inlineCode}>.env</code> file for local development:</p>
        <CodeBlock code={`curl -H "X-API-Key: ks_abc123..." \\
  "http://localhost:8080/api/v1/projects/{id}/envfile?env=alpha" \\
  -o .env`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Environment Variable Integration</h3>
        <CodeBlock code={`// Node.js: Load secrets into process.env at startup
import { KeepSave } from '@keepsave/sdk';

const ks = new KeepSave({ apiKey: process.env.KEEPSAVE_API_KEY });

async function loadSecrets() {
  const secrets = await ks.getSecrets('alpha');
  Object.assign(process.env, secrets);
  console.log('Loaded', Object.keys(secrets).length, 'secrets into process.env');
}

loadSecrets().then(() => {
  // Start your app after secrets are loaded
  require('./server');
});`} />
      </div>
    </div>
  );
}

function Promotion() {
  return (
    <div>
      <h1 style={pageTitle}>Promotion Pipeline</h1>
      <p style={intro}>KeepSave enforces a controlled promotion flow across environments.</p>

      <div style={pipelineVisual}>
        <div style={pipelineStage}>
          <span style={{ ...envLabel, background: 'rgba(34, 197, 94, 0.15)', color: '#22c55e' }}>ALPHA</span>
          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>Development</span>
        </div>
        <span style={pipelineArrow}>&rarr;</span>
        <div style={pipelineStage}>
          <span style={{ ...envLabel, background: 'rgba(99, 102, 241, 0.15)', color: '#818cf8' }}>UAT</span>
          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>Staging</span>
        </div>
        <span style={pipelineArrow}>&rarr;</span>
        <div style={pipelineStage}>
          <span style={{ ...envLabel, background: 'rgba(245, 158, 11, 0.15)', color: '#f59e0b' }}>PROD</span>
          <span style={{ fontSize: 12, color: 'var(--color-text-secondary)' }}>Production</span>
        </div>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>How It Works</h3>
        <ul style={docList}>
          <li>Secrets are first created in <strong>Alpha</strong> (development)</li>
          <li>When ready, promote Alpha secrets to <strong>UAT</strong> for testing</li>
          <li>After UAT validation, promote to <strong>PROD</strong></li>
          <li>PROD promotions require <strong>multi-party approval</strong></li>
          <li>Every promotion creates an audit trail entry</li>
          <li>Completed promotions can be <strong>rolled back</strong></li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Override Policies</h3>
        <ul style={docList}>
          <li><strong>Skip</strong> - Only add new keys, don't overwrite existing values in target</li>
          <li><strong>Overwrite</strong> - Replace all values in target with source values</li>
        </ul>
      </div>
    </div>
  );
}

function EmbedWidget() {
  return (
    <div>
      <h1 style={pageTitle}>Embed Widget</h1>
      <p style={intro}>KeepSave provides an embeddable Web Component for managing secrets from any web application.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Quick Setup</h3>
        <CodeBlock code={`<!-- Add the widget script -->
<script src="http://localhost:3002/keepsave-widget.js"></script>

<!-- Use the web component -->
<keepsave-widget
  api-url="http://localhost:8080"
  project-id="your-project-id"
  token="jwt_token_here"
  theme="dark"
></keepsave-widget>`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Attributes</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Attribute</th><th style={apiTh}>Required</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>api-url</code></td><td style={apiTd}>Yes</td><td style={apiTd}>KeepSave API base URL</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>project-id</code></td><td style={apiTd}>Yes</td><td style={apiTd}>Project UUID</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>token</code></td><td style={apiTd}>Yes</td><td style={apiTd}>JWT or API key</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>theme</code></td><td style={apiTd}>No</td><td style={apiTd}>light or dark (default: light)</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>environment</code></td><td style={apiTd}>No</td><td style={apiTd}>Default env (alpha/uat/prod)</td></tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SecurityModel() {
  return (
    <div>
      <h1 style={pageTitle}>Security Model</h1>
      <p style={intro}>KeepSave is designed for zero-trust secret management with defense in depth.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Encryption</h3>
        <ul style={docList}>
          <li>All secrets encrypted at rest with <strong>AES-256-GCM</strong></li>
          <li>Envelope encryption: each project has its own Data Encryption Key (DEK)</li>
          <li>DEKs are encrypted by a Master Key (from KMS or env variable)</li>
          <li>Master Key is never stored in the database</li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Authentication</h3>
        <ul style={docList}>
          <li><strong>JWT tokens</strong> for user sessions (login/register)</li>
          <li><strong>API keys</strong> for machine-to-machine access (AI agents, CI/CD)</li>
          <li>API keys are scoped per-project and optionally per-environment</li>
          <li>Password policies enforce minimum complexity</li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Access Control</h3>
        <ul style={docList}>
          <li>Organization-based RBAC: <strong>Admin</strong>, <strong>Promoter</strong>, <strong>Editor</strong>, <strong>Viewer</strong></li>
          <li>PROD promotions require explicit multi-party approval</li>
          <li>Full audit trail for every secret access and modification</li>
          <li>CORS restrictions for embed widget origins</li>
        </ul>
      </div>
    </div>
  );
}

function DeveloperGuide() {
  return (
    <div>
      <h1 style={pageTitle}>Developer Guide</h1>
      <p style={intro}>Everything you need to set up, understand, and extend the KeepSave codebase.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Project Setup</h3>
        <CodeBlock code={`git clone https://github.com/santapong/KeepSave.git
cd KeepSave
docker-compose up --build`} />
        <p style={{ ...stepDesc, marginTop: 12 }}>Once running, the services are available at:</p>
        <ul style={docList}>
          <li>Backend: <code style={inlineCode}>http://localhost:8080</code></li>
          <li>Frontend: <code style={inlineCode}>http://localhost:3002</code></li>
          <li>PostgreSQL: <code style={inlineCode}>localhost:5435</code></li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Backend Architecture</h3>
        <p style={stepDesc}>Go 1.22+ with Gin framework</p>
        <CodeBlock code={`backend/
├── cmd/server/           # Entry point (main.go)
├── internal/
│   ├── api/              # HTTP handlers, middleware, router, validation
│   ├── auth/             # JWT tokens, password hashing, API key auth
│   ├── crypto/           # AES-256-GCM encryption, envelope encryption
│   ├── models/           # Domain models (User, Project, Secret, etc.)
│   ├── repository/       # PostgreSQL data access layer
│   ├── service/          # Business logic layer
│   └── promotion/        # Environment promotion engine
├── migrations/           # SQL migration files
└── Dockerfile`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Frontend Architecture</h3>
        <p style={stepDesc}>React 18 + TypeScript + Vite</p>
        <CodeBlock code={`frontend/src/
├── api/client.ts         # Centralized API client with typed fetch wrapper
├── components/           # Reusable UI components (SecretsPanel, Layout, etc.)
├── hooks/                # Custom React hooks (useAuth)
├── pages/                # Route-level page components
├── types/index.ts        # TypeScript interfaces for all models
├── embed/                # Embeddable widget (Shadow DOM Web Component)
└── utils/                # Shared utilities (formatDate)`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Database Schema</h3>
        <p style={stepDesc}>PostgreSQL 16</p>
        <p style={{ ...stepDesc, marginTop: 8 }}>Core tables:</p>
        <ul style={docList}>
          <li><code style={inlineCode}>users</code>, <code style={inlineCode}>projects</code>, <code style={inlineCode}>environments</code>, <code style={inlineCode}>secrets</code></li>
          <li><code style={inlineCode}>api_keys</code>, <code style={inlineCode}>audit_log</code>, <code style={inlineCode}>promotion_requests</code></li>
          <li><code style={inlineCode}>secret_snapshots</code>, <code style={inlineCode}>secret_versions</code></li>
          <li><code style={inlineCode}>organizations</code>, <code style={inlineCode}>organization_members</code></li>
          <li><code style={inlineCode}>secret_templates</code>, <code style={inlineCode}>secret_dependencies</code></li>
        </ul>
        <p style={{ ...stepDesc, marginTop: 8 }}>Migrations are located in <code style={inlineCode}>backend/migrations/postgres/</code></p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Adding New Features</h3>
        <p style={stepDesc}>Follow this workflow when adding a new feature end-to-end:</p>
        <ol style={{ ...docList, listStyleType: 'decimal' }}>
          <li>Add request/response types in <code style={inlineCode}>internal/api/validation.go</code></li>
          <li>Create handler in <code style={inlineCode}>internal/api/handlers_*.go</code></li>
          <li>Register route in <code style={inlineCode}>internal/api/router.go</code></li>
          <li>Add service logic in <code style={inlineCode}>internal/service/</code></li>
          <li>Add repository methods in <code style={inlineCode}>internal/repository/</code></li>
          <li>Add migration if schema changes</li>
          <li>Add API client function in <code style={inlineCode}>frontend/src/api/client.ts</code></li>
          <li>Build UI component in <code style={inlineCode}>frontend/src/components/</code> or <code style={inlineCode}>pages/</code></li>
        </ol>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Testing</h3>
        <p style={stepDesc}><strong>Go</strong> — <code style={inlineCode}>cd backend && go test ./...</code> (table-driven tests)</p>
        <CodeBlock code={`func TestCreateSecret(t *testing.T) {
    tests := []struct {
        name    string
        key     string
        value   string
        wantErr bool
    }{
        {"valid secret", "DB_URL", "postgres://...", false},
        {"empty key", "", "value", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test logic
        })
    }
}`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Frontend</strong> — <code style={inlineCode}>cd frontend && npm test</code> (Vitest + React Testing Library)</p>
      </div>
    </div>
  );
}

function AgentIntegration() {
  return (
    <div>
      <h1 style={pageTitle}>Agent Integration</h1>
      <p style={intro}>Connect AI agents and automated workflows to KeepSave for secure, scoped secret access at runtime.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Overview</h3>
        <p style={stepDesc}>
          KeepSave is designed as a secrets backend for AI agents. Agents authenticate with scoped API keys and fetch secrets at runtime — never storing sensitive values in code, config files, or conversation history.
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Authentication Flow</h3>
        <p style={stepDesc}><strong>Step 1:</strong> Create an API key (via the UI or API)</p>
        <CodeBlock code={`curl -X POST http://localhost:8080/api/v1/api-keys \\
  -H "Authorization: Bearer <jwt>" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"my-agent","project_id":"<uuid>","scopes":["read"],"environment":"alpha"}'`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Step 2:</strong> Use the returned raw key in the <code style={inlineCode}>X-API-Key</code> header</p>
        <CodeBlock code={`curl -H "X-API-Key: ks_abc123..." \\
  "http://localhost:8080/api/v1/agent/secrets?env=alpha"`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Available Agent Endpoints</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Endpoint</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <ApiRow method="GET" path="/api/v1/agent/secrets?env=alpha" desc="Fetch all secrets as flat key-value map" />
            <ApiRow method="GET" path="/api/v1/agent/activity" desc="Get agent activity summary" />
            <ApiRow method="POST" path="/api/v1/projects/:id/leases" desc="Create a time-limited secret lease" />
            <ApiRow method="GET" path="/api/v1/projects/:id/leases" desc="List active leases" />
            <ApiRow method="DELETE" path="/api/v1/projects/:id/leases/:lid" desc="Revoke a lease" />
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Rate Limiting & Security</h3>
        <ul style={docList}>
          <li>Scope API keys to minimum required permissions (read-only for most agents)</li>
          <li>Use environment restriction to prevent dev agents from touching PROD</li>
          <li>Rotate API keys periodically</li>
          <li>Use HTTPS in production</li>
          <li>Monitor agent activity via the Audit Log tab</li>
          <li>Set up secret leases for time-limited access</li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Example: Claude Code Agent</h3>
        <CodeBlock code={`#!/bin/bash
# Fetch secrets from KeepSave and export as environment variables
SECRETS=$(curl -s -H "X-API-Key: $KEEPSAVE_API_KEY" \\
  "https://keepsave.example.com/api/v1/agent/secrets?env=alpha")

# Parse JSON and export each key-value pair
echo "$SECRETS" | jq -r 'to_entries[] | "export \\(.key)=\\(.value)"' | while read line; do
  eval "$line"
done

# Now run your application with secrets injected
node app.js`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Example: GitHub Actions CI/CD</h3>
        <CodeBlock code={`name: Deploy with KeepSave Secrets
on: [push]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Fetch secrets from KeepSave
        run: |
          SECRETS=$(curl -s -H "X-API-Key: \${{ secrets.KEEPSAVE_API_KEY }}" \\
            "https://keepsave.example.com/api/v1/agent/secrets?env=prod")
          echo "$SECRETS" | jq -r 'to_entries[] | "\\(.key)=\\(.value)"' >> $GITHUB_ENV
      - name: Deploy
        run: ./deploy.sh`} />
      </div>
    </div>
  );
}

function MCPServer() {
  return (
    <div>
      <h1 style={pageTitle}>MCP Server</h1>
      <p style={intro}>Expose KeepSave as a Model Context Protocol server for AI assistants like Claude.</p>

      <div style={docCard}>
        <h3 style={docTitle}>What is MCP</h3>
        <p style={stepDesc}>
          The Model Context Protocol (MCP) is an open standard that lets AI assistants like Claude call external tools and access data sources. KeepSave can be exposed as an MCP server, allowing AI agents to manage secrets through natural language — creating, reading, and promoting secrets across environments without leaving the conversation.
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Configuration</h3>
        <p style={stepDesc}>Add KeepSave to your Claude Desktop or Claude Code MCP config:</p>
        <CodeBlock code={`{
  "mcpServers": {
    "keepsave": {
      "command": "npx",
      "args": [
        "@keepsave/mcp-server",
        "--api-url", "http://localhost:8080",
        "--api-key", "ks_your_api_key_here"
      ]
    }
  }
}`} />
        <p style={{ ...stepDesc, marginTop: 12 }}>For Claude Code, add to <code style={inlineCode}>.claude/settings.json</code> or use <code style={inlineCode}>claude mcp add</code>.</p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Available MCP Tools</h3>
        <table style={apiTable}>
          <thead><tr><th style={apiTh}>Tool</th><th style={apiTh}>Description</th></tr></thead>
          <tbody>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_list_projects</code></td><td style={apiTd}>List all projects the API key has access to</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_list_secrets</code></td><td style={apiTd}>List secrets for a project environment (returns key names only, values masked)</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_get_secret</code></td><td style={apiTd}>Get a specific secret value by key name</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_set_secret</code></td><td style={apiTd}>Create or update a secret value</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_delete_secret</code></td><td style={apiTd}>Delete a secret by key name</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_promote</code></td><td style={apiTd}>Trigger a promotion between environments (e.g. alpha to uat)</td></tr>
            <tr style={apiTr}><td style={apiTd}><code style={inlineCode}>keepsave_list_environments</code></td><td style={apiTd}>List available environments for a project</td></tr>
          </tbody>
        </table>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Tool Schemas</h3>
        <p style={stepDesc}>Example input/output for each tool:</p>
        <CodeBlock code={`// keepsave_list_secrets
{
  "input": {
    "project_id": "uuid",
    "environment": "alpha"
  },
  "output": {
    "secrets": [
      { "key": "DATABASE_URL", "updated_at": "2026-03-16T10:00:00Z" },
      { "key": "REDIS_URL", "updated_at": "2026-03-15T08:30:00Z" }
    ]
  }
}

// keepsave_get_secret
{
  "input": {
    "project_id": "uuid",
    "environment": "alpha",
    "key": "DATABASE_URL"
  },
  "output": {
    "key": "DATABASE_URL",
    "value": "postgres://user:pass@host:5432/db",
    "updated_at": "2026-03-16T10:00:00Z"
  }
}

// keepsave_set_secret
{
  "input": {
    "project_id": "uuid",
    "environment": "alpha",
    "key": "NEW_API_KEY",
    "value": "sk-abc123..."
  },
  "output": {
    "success": true,
    "key": "NEW_API_KEY"
  }
}

// keepsave_promote
{
  "input": {
    "project_id": "uuid",
    "source": "alpha",
    "target": "uat",
    "override_policy": "skip"
  },
  "output": {
    "promotion_id": "uuid",
    "status": "pending",
    "keys_promoted": 5
  }
}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Security Considerations</h3>
        <ul style={docList}>
          <li>Always use scoped API keys with minimum required permissions</li>
          <li>Use <code style={inlineCode}>read</code> scope for agents that only need to fetch secrets</li>
          <li>Add <code style={inlineCode}>write</code> scope only for agents that need to create/update secrets</li>
          <li>Restrict API keys to specific environments (e.g. alpha only for dev agents)</li>
          <li>Never expose PROD API keys to development MCP server instances</li>
          <li>All MCP tool calls are logged in the KeepSave audit trail</li>
          <li>Use secret leases for time-limited MCP access in automated workflows</li>
          <li>Rotate MCP API keys on a regular schedule</li>
        </ul>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Setup Guide</h3>
        <ol style={{ ...docList, listStyleType: 'decimal' }}>
          <li>Install the MCP server: <code style={inlineCode}>npm install -g @keepsave/mcp-server</code></li>
          <li>Create a scoped API key in your project's API Keys tab</li>
          <li>Add the MCP config to your AI assistant (see Configuration above)</li>
          <li>Test by asking your AI assistant: "List my KeepSave projects"</li>
          <li>Optional: set up separate MCP configs for different environments (dev vs staging)</li>
        </ol>
      </div>
    </div>
  );
}

function Examples() {
  return (
    <div>
      <h1 style={pageTitle}>Examples</h1>
      <p style={intro}>Practical, end-to-end usage examples for common KeepSave integration scenarios. Each example is self-contained and can be adapted to your stack.</p>

      <div style={docCard}>
        <h3 style={docTitle}>Docker Compose Quick Start</h3>
        <p style={stepDesc}>Spin up a complete KeepSave stack from scratch, create a project, and store your first secret.</p>
        <CodeBlock code={`# 1. Clone and start the stack
git clone https://github.com/santapong/KeepSave.git
cd KeepSave
docker-compose up --build -d

# 2. Register a user
curl -s -X POST http://localhost:8080/api/v1/auth/register \\
  -H "Content-Type: application/json" \\
  -d '{"email":"admin@example.com","password":"Str0ngP@ss!"}'

# 3. Login and capture the JWT
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \\
  -H "Content-Type: application/json" \\
  -d '{"email":"admin@example.com","password":"Str0ngP@ss!"}' \\
  | jq -r '.token')

# 4. Create a project
PROJECT_ID=$(curl -s -X POST http://localhost:8080/api/v1/projects \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"name":"my-first-project","description":"Quick start demo"}' \\
  | jq -r '.id')

# 5. Add a secret to the alpha environment
curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/secrets" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"key":"DATABASE_URL","value":"postgres://app:secret@db:5432/myapp","environment":"alpha"}'

# 6. Create an API key for programmatic access
API_KEY=$(curl -s -X POST http://localhost:8080/api/v1/api-keys \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d "{\\\"name\\\":\\\"dev-key\\\",\\\"project_id\\\":\\\"$PROJECT_ID\\\",\\\"scopes\\\":[\\\"read\\\",\\\"write\\\"],\\\"environment\\\":\\\"alpha\\\"}" \\
  | jq -r '.raw_key')

# 7. Fetch secrets using the API key
curl -s -H "X-API-Key: $API_KEY" \\
  "http://localhost:8080/api/v1/agent/secrets?env=alpha"
# => {"DATABASE_URL":"postgres://app:secret@db:5432/myapp"}`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Node.js Express App</h3>
        <p style={stepDesc}>Load secrets from KeepSave at application startup and inject them into <code style={inlineCode}>process.env</code> before your Express server begins accepting requests.</p>
        <CodeBlock code={`npm install @keepsave/sdk express`} />
        <CodeBlock code={`// server.js
import express from 'express';
import { KeepSave } from '@keepsave/sdk';

const app = express();

async function start() {
  // Load secrets before anything else
  const ks = new KeepSave({
    apiKey: process.env.KEEPSAVE_API_KEY,
    baseURL: process.env.KEEPSAVE_URL || 'http://localhost:8080',
  });

  const env = process.env.NODE_ENV === 'production' ? 'prod' : 'alpha';
  const secrets = await ks.getSecrets(env);
  Object.assign(process.env, secrets);

  console.log(\`Loaded \${Object.keys(secrets).length} secrets from KeepSave (\${env})\`);

  // Now process.env.DATABASE_URL, process.env.STRIPE_KEY, etc. are available
  app.get('/health', (_req, res) => res.json({ status: 'ok' }));

  const port = process.env.PORT || 3000;
  app.listen(port, () => console.log(\`Server running on :\${port}\`));
}

start().catch(err => {
  console.error('Failed to start:', err);
  process.exit(1);
});`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Python FastAPI App</h3>
        <p style={stepDesc}>Use the KeepSave Python SDK to load secrets during FastAPI's lifespan startup hook.</p>
        <CodeBlock code={`pip install keepsave fastapi uvicorn`} />
        <CodeBlock code={`# main.py
import os
from contextlib import asynccontextmanager
from fastapi import FastAPI
from keepsave import KeepSaveClient

secrets_cache: dict[str, str] = {}

@asynccontextmanager
async def lifespan(app: FastAPI):
    # Startup: load secrets from KeepSave
    ks = KeepSaveClient(
        api_key=os.environ["KEEPSAVE_API_KEY"],
        base_url=os.environ.get("KEEPSAVE_URL", "http://localhost:8080"),
    )
    env = "prod" if os.environ.get("ENV") == "production" else "alpha"
    secrets_cache.update(ks.get_secrets(env))
    print(f"Loaded {len(secrets_cache)} secrets from KeepSave ({env})")
    yield

app = FastAPI(lifespan=lifespan)

@app.get("/health")
def health():
    return {"status": "ok"}

@app.get("/db-check")
def db_check():
    # Access secrets from the cache (never log values!)
    db_url = secrets_cache.get("DATABASE_URL", "not set")
    return {"database_configured": db_url != "not set"}`} />
        <CodeBlock code={`# Run it
KEEPSAVE_API_KEY=ks_your_key uvicorn main:app --reload`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>CI/CD Pipeline</h3>
        <p style={stepDesc}>Inject KeepSave secrets into your build and deploy steps. Examples for both GitHub Actions and GitLab CI.</p>
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>GitHub Actions</strong></p>
        <CodeBlock code={`# .github/workflows/deploy.yml
name: Deploy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Fetch secrets from KeepSave
        run: |
          SECRETS=$(curl -sf -H "X-API-Key: \${{ secrets.KEEPSAVE_API_KEY }}" \\
            "\${{ vars.KEEPSAVE_URL }}/api/v1/agent/secrets?env=prod")
          echo "$SECRETS" | jq -r 'to_entries[] | "\\(.key)=\\(.value)"' | while IFS= read -r line; do
            echo "::add-mask::$(echo "$line" | cut -d= -f2-)"
            echo "$line" >> $GITHUB_ENV
          done

      - name: Build
        run: npm ci && npm run build

      - name: Deploy
        run: ./scripts/deploy.sh`} />
        <p style={{ ...stepDesc, marginTop: 16 }}><strong>GitLab CI</strong></p>
        <CodeBlock code={`# .gitlab-ci.yml
stages:
  - deploy

deploy:
  stage: deploy
  image: alpine/curl
  before_script:
    - apk add --no-cache jq
  script:
    - |
      SECRETS=$(curl -sf -H "X-API-Key: $KEEPSAVE_API_KEY" \\
        "$KEEPSAVE_URL/api/v1/agent/secrets?env=prod")
      eval $(echo "$SECRETS" | jq -r 'to_entries[] | "export \\(.key)=\\(.value|@sh)"')
    - ./scripts/deploy.sh
  only:
    - main`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Terraform Integration</h3>
        <p style={stepDesc}>Use KeepSave secrets as Terraform input variables via an external data source.</p>
        <CodeBlock code={`# fetch_secrets.sh — called by Terraform external data source
#!/bin/bash
set -e
SECRETS=$(curl -sf -H "X-API-Key: $KEEPSAVE_API_KEY" \\
  "$KEEPSAVE_URL/api/v1/agent/secrets?env=prod")
echo "$SECRETS"`} />
        <CodeBlock code={`# main.tf
data "external" "keepsave" {
  program = ["\${path.module}/fetch_secrets.sh"]
}

resource "aws_db_instance" "main" {
  engine   = "postgres"
  username = "app"
  password = data.external.keepsave.result["DB_PASSWORD"]
}

resource "aws_ssm_parameter" "api_key" {
  name  = "/myapp/stripe_key"
  type  = "SecureString"
  value = data.external.keepsave.result["STRIPE_KEY"]
}`} />
        <p style={stepDesc}>
          Run with: <code style={inlineCode}>KEEPSAVE_API_KEY=ks_xxx KEEPSAVE_URL=https://keepsave.example.com terraform apply</code>
        </p>
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Webhook Setup</h3>
        <p style={stepDesc}>React to secret changes in real time. Register a webhook URL and KeepSave will POST events when secrets are created, updated, deleted, or promoted.</p>
        <CodeBlock code={`# Register a webhook for your project
curl -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/webhooks" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "url": "https://your-app.example.com/hooks/keepsave",
    "events": ["secret.created", "secret.updated", "secret.deleted", "promotion.completed"],
    "environment": "prod"
  }'`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Webhook receiver (Express)</strong></p>
        <CodeBlock code={`// hooks/keepsave.js
import express from 'express';
const router = express.Router();

router.post('/hooks/keepsave', express.json(), (req, res) => {
  const { event, project_id, environment, key, timestamp } = req.body;

  switch (event) {
    case 'secret.updated':
      console.log(\`Secret "\${key}" updated in \${environment} at \${timestamp}\`);
      // Trigger config reload, restart workers, invalidate cache, etc.
      reloadConfig();
      break;

    case 'promotion.completed':
      console.log(\`Promotion to \${environment} completed at \${timestamp}\`);
      // Notify Slack, trigger deployment, etc.
      notifyTeam(environment);
      break;

    default:
      console.log(\`Unhandled event: \${event}\`);
  }

  res.status(200).json({ received: true });
});

export default router;`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Multi-Environment Workflow</h3>
        <p style={stepDesc}>Walk through a complete secrets lifecycle: create in Alpha, test in UAT, promote to PROD with approval.</p>
        <CodeBlock code={`# Assume $TOKEN and $PROJECT_ID are set (see Docker Compose Quick Start above)

# ── Step 1: Add secrets to Alpha ──
curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/secrets" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"key":"DATABASE_URL","value":"postgres://dev:dev@localhost/myapp_dev","environment":"alpha"}'

curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/secrets" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"key":"STRIPE_KEY","value":"sk_test_abc123","environment":"alpha"}'

# ── Step 2: Preview what will be promoted to UAT ──
curl -s "http://localhost:8080/api/v1/projects/$PROJECT_ID/promotions/diff?source=alpha&target=uat" \\
  -H "Authorization: Bearer $TOKEN"

# ── Step 3: Promote Alpha → UAT ──
PROMO_ID=$(curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/promotions" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"source_environment":"alpha","target_environment":"uat","override_policy":"overwrite","notes":"Ready for QA"}' \\
  | jq -r '.id')

# ── Step 4: Verify secrets in UAT ──
curl -s "http://localhost:8080/api/v1/projects/$PROJECT_ID/secrets?env=uat" \\
  -H "Authorization: Bearer $TOKEN"

# ── Step 5: Promote UAT → PROD (requires approval) ──
PROD_PROMO=$(curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/promotions" \\
  -H "Authorization: Bearer $TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"source_environment":"uat","target_environment":"prod","override_policy":"skip","notes":"QA passed, shipping to prod"}' \\
  | jq -r '.id')

# ── Step 6: Another team member approves the PROD promotion ──
curl -s -X POST "http://localhost:8080/api/v1/projects/$PROJECT_ID/promotions/$PROD_PROMO/approve" \\
  -H "Authorization: Bearer $APPROVER_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{"notes":"LGTM"}'

# ── Step 7: Verify PROD secrets ──
curl -s "http://localhost:8080/api/v1/projects/$PROJECT_ID/secrets?env=prod" \\
  -H "Authorization: Bearer $TOKEN"`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Embed Widget in React App</h3>
        <p style={stepDesc}>Integrate the KeepSave widget into an existing React application using a wrapper component.</p>
        <CodeBlock code={`// KeepSaveWidget.tsx
import { useEffect, useRef } from 'react';

interface KeepSaveWidgetProps {
  apiUrl: string;
  projectId: string;
  token: string;
  theme?: 'light' | 'dark';
  environment?: 'alpha' | 'uat' | 'prod';
}

export function KeepSaveWidget({ apiUrl, projectId, token, theme = 'dark', environment }: KeepSaveWidgetProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const scriptId = 'keepsave-widget-script';
    if (!document.getElementById(scriptId)) {
      const script = document.createElement('script');
      script.id = scriptId;
      script.src = \`\${apiUrl.replace(':8080', ':3002')}/keepsave-widget.js\`;
      script.async = true;
      document.head.appendChild(script);
    }
  }, [apiUrl]);

  useEffect(() => {
    const el = containerRef.current?.querySelector('keepsave-widget');
    if (el) {
      el.setAttribute('api-url', apiUrl);
      el.setAttribute('project-id', projectId);
      el.setAttribute('token', token);
      el.setAttribute('theme', theme);
      if (environment) el.setAttribute('environment', environment);
    }
  }, [apiUrl, projectId, token, theme, environment]);

  return (
    <div ref={containerRef}>
      <keepsave-widget
        api-url={apiUrl}
        project-id={projectId}
        token={token}
        theme={theme}
        {...(environment ? { environment } : {})}
      />
    </div>
  );
}`} />
        <p style={{ ...stepDesc, marginTop: 12 }}><strong>Usage in your app:</strong></p>
        <CodeBlock code={`// AdminPanel.tsx
import { KeepSaveWidget } from './KeepSaveWidget';

export function AdminPanel() {
  return (
    <div>
      <h1>Secret Management</h1>
      <KeepSaveWidget
        apiUrl="https://keepsave.example.com"
        projectId="your-project-uuid"
        token={localStorage.getItem('jwt') || ''}
        theme="dark"
        environment="alpha"
      />
    </div>
  );
}`} />
        <p style={{ ...stepDesc, marginTop: 12 }}>For TypeScript, add a type declaration so JSX recognizes the custom element:</p>
        <CodeBlock code={`// global.d.ts
declare namespace JSX {
  interface IntrinsicElements {
    'keepsave-widget': React.DetailedHTMLProps<
      React.HTMLAttributes<HTMLElement> & {
        'api-url'?: string;
        'project-id'?: string;
        token?: string;
        theme?: string;
        environment?: string;
      },
      HTMLElement
    >;
  }
}`} />
      </div>
    </div>
  );
}

/* Shared sub-components */

function CodeBlock({ code }: { code: string }) {
  return (
    <pre style={codeBlock}><code>{code}</code></pre>
  );
}

function ApiRow({ method, path, desc }: { method: string; path: string; desc: string }) {
  const methodColor: Record<string, string> = {
    GET: '#22c55e',
    POST: '#6366f1',
    PUT: '#f59e0b',
    DELETE: '#ef4444',
  };
  return (
    <tr style={apiTr}>
      <td style={apiTd}>
        <span style={{ ...methodBadge, color: methodColor[method] || 'var(--color-text)' }}>{method}</span>
      </td>
      <td style={apiTd}><code style={{ fontSize: 12 }}>{path}</code></td>
      <td style={apiTd}><span style={{ fontSize: 13, color: 'var(--color-text-secondary)' }}>{desc}</span></td>
    </tr>
  );
}

/* Styles */

const sidebar: React.CSSProperties = {
  width: 200,
  flexShrink: 0,
  position: 'sticky',
  top: 80,
  alignSelf: 'flex-start',
};

const sidebarItem: React.CSSProperties = {
  display: 'block',
  width: '100%',
  textAlign: 'left',
  padding: '8px 12px',
  border: 'none',
  borderRadius: 4,
  fontSize: 13,
  cursor: 'pointer',
  marginBottom: 2,
  transition: 'background 0.15s, color 0.15s',
};

const pageTitle: React.CSSProperties = {
  fontSize: 28,
  fontWeight: 700,
  marginBottom: 8,
};

const intro: React.CSSProperties = {
  fontSize: 15,
  color: 'var(--color-text-secondary)',
  marginBottom: 28,
  lineHeight: 1.6,
};

const stepCard: React.CSSProperties = {
  display: 'flex',
  gap: 16,
  alignItems: 'flex-start',
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  marginBottom: 12,
  overflow: 'auto',
};

const stepNumber: React.CSSProperties = {
  width: 32,
  height: 32,
  borderRadius: '50%',
  background: 'var(--color-primary)',
  color: '#fff',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontWeight: 700,
  fontSize: 14,
  flexShrink: 0,
};

const stepTitle: React.CSSProperties = {
  fontSize: 16,
  fontWeight: 600,
  marginBottom: 4,
};

const stepDesc: React.CSSProperties = {
  fontSize: 14,
  color: 'var(--color-text-secondary)',
  lineHeight: 1.6,
};

const docCard: React.CSSProperties = {
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  padding: 20,
  marginBottom: 16,
  overflow: 'auto',
};

const docTitle: React.CSSProperties = {
  fontSize: 16,
  fontWeight: 600,
  marginBottom: 12,
};

const docList: React.CSSProperties = {
  paddingLeft: 20,
  display: 'flex',
  flexDirection: 'column',
  gap: 8,
  fontSize: 14,
  color: 'var(--color-text-secondary)',
  lineHeight: 1.6,
};

const codeBlock: React.CSSProperties = {
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 6,
  padding: 16,
  fontSize: 13,
  fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace',
  overflow: 'auto',
  lineHeight: 1.6,
  marginTop: 8,
};

const inlineCode: React.CSSProperties = {
  background: 'var(--color-input-bg)',
  border: '1px solid var(--color-border)',
  borderRadius: 4,
  padding: '1px 6px',
  fontSize: 12,
  fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace',
};

const apiTable: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
};

const apiTh: React.CSSProperties = {
  textAlign: 'left',
  padding: '8px 12px',
  fontSize: 11,
  fontWeight: 600,
  color: 'var(--color-text-secondary)',
  borderBottom: '1px solid var(--color-border)',
  textTransform: 'uppercase',
  letterSpacing: '0.05em',
};

const apiTd: React.CSSProperties = {
  padding: '8px 12px',
  borderBottom: '1px solid var(--color-border)',
  fontSize: 13,
};

const apiTr: React.CSSProperties = {};

const methodBadge: React.CSSProperties = {
  fontWeight: 700,
  fontSize: 11,
  fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace',
};

const pipelineVisual: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: 24,
  padding: 32,
  background: 'var(--color-surface)',
  border: '1px solid var(--color-border)',
  borderRadius: 'var(--radius)',
  marginBottom: 24,
};

const pipelineStage: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  gap: 6,
};

const envLabel: React.CSSProperties = {
  padding: '8px 20px',
  borderRadius: 8,
  fontSize: 14,
  fontWeight: 700,
  letterSpacing: '0.05em',
};

const pipelineArrow: React.CSSProperties = {
  fontSize: 24,
  color: 'var(--color-text-secondary)',
};

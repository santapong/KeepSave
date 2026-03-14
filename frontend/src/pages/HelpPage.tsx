import { useState } from 'react';

type Section = 'getting-started' | 'api' | 'sdks' | 'promotion' | 'embed' | 'security';

export function HelpPage() {
  const [active, setActive] = useState<Section>('getting-started');

  const sections: { key: Section; label: string }[] = [
    { key: 'getting-started', label: 'Getting Started' },
    { key: 'api', label: 'API Reference' },
    { key: 'sdks', label: 'SDKs & CLI' },
    { key: 'promotion', label: 'Promotion Pipeline' },
    { key: 'embed', label: 'Embed Widget' },
    { key: 'security', label: 'Security Model' },
  ];

  return (
    <div style={{ display: 'flex', gap: 32, minHeight: 'calc(100vh - 120px)' }}>
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
      <div style={{ flex: 1, maxWidth: 780 }}>
        {active === 'getting-started' && <GettingStarted />}
        {active === 'api' && <APIReference />}
        {active === 'sdks' && <SDKs />}
        {active === 'promotion' && <Promotion />}
        {active === 'embed' && <EmbedWidget />}
        {active === 'security' && <SecurityModel />}
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
          <p style={stepDesc}>Go to <strong>API Keys</strong> and create a scoped key for your application or AI agent. Keys can be restricted by project and environment.</p>
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>4</div>
        <div>
          <h3 style={stepTitle}>Connect Your App</h3>
          <p style={stepDesc}>Use the API or an SDK to fetch secrets at runtime. Your app never stores sensitive values in code or config files.</p>
          <CodeBlock code={`# Fetch secrets via API
curl -H "X-API-Key: ks_your_key_here" \\
  http://localhost:8080/api/v1/projects/{id}/secrets?env=alpha`} />
        </div>
      </div>

      <div style={stepCard}>
        <div style={stepNumber}>5</div>
        <div>
          <h3 style={stepTitle}>Promote to Production</h3>
          <p style={stepDesc}>When ready, use the <strong>Promote</strong> tab to push secrets from Alpha to UAT, then UAT to PROD. PROD promotions require approval.</p>
        </div>
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
        <CodeBlock code={`import { KeepSave } from '@keepsave/sdk';

const ks = new KeepSave({
  apiKey: 'ks_your_api_key',
  baseURL: 'http://localhost:8080',
});

// Fetch all secrets for an environment
const secrets = await ks.getSecrets('alpha');
console.log(secrets.DATABASE_URL);`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Python</h3>
        <CodeBlock code={`from keepsave import KeepSaveClient

ks = KeepSaveClient(
    api_key="ks_your_api_key",
    base_url="http://localhost:8080",
)

secrets = ks.get_secrets("alpha")
print(secrets["DATABASE_URL"])`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>Go</h3>
        <CodeBlock code={`import "github.com/your-org/keepsave-go"

client := keepsave.NewClient(
    keepsave.WithAPIKey("ks_your_api_key"),
    keepsave.WithBaseURL("http://localhost:8080"),
)

secrets, err := client.GetSecrets(ctx, "alpha")
dbURL := secrets["DATABASE_URL"]`} />
      </div>

      <div style={docCard}>
        <h3 style={docTitle}>.env File Export</h3>
        <p style={stepDesc}>Export secrets as a <code style={inlineCode}>.env</code> file for local development:</p>
        <CodeBlock code={`curl -H "X-API-Key: ks_abc123..." \\
  "http://localhost:8080/api/v1/projects/{id}/envfile?env=alpha" \\
  -o .env`} />
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

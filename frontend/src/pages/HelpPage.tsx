import { useState, type ReactNode, type CSSProperties } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useCapabilities } from '../hooks/useCapabilities';

const guides = [
  { key: 'getting-started', label: 'Your first secret', summary: 'Create a project, save a sample value, and check the result.', terms: 'start onboarding workspace alpha add secret' },
  { key: 'developer-guide', label: 'Run KeepSave locally', summary: 'Prerequisites, configuration, health checks, and code ownership.', terms: 'developer setup docker compose ports postgres go node architecture migrations' },
  { key: 'api', label: 'Make an API request', summary: 'Real paths, request bodies, response envelopes, and batch reads.', terms: 'curl rest json contract endpoint environment batch missing_keys' },
  { key: 'api-keys', label: 'Choose credentials & scopes', summary: 'Browser sessions, API keys, leases, and revocation boundaries.', terms: 'auth login token jwt sid bearer read write delete scope expiry' },
  { key: 'history', label: 'History, restore & rotation', summary: 'Read revision metadata and recover an earlier value safely.', terms: 'version revision concurrent conflict 409 stale restore rotate keys' },
  { key: 'recovery', label: 'Export & recover data', summary: 'Plaintext exports, encrypted bundles, and isolated recovery.', terms: 'backup recovery env import export restore key material database' },
  { key: 'promotion', label: 'Promote between environments', summary: 'Preview a diff, choose a policy, and inspect approval state.', terms: 'pipeline alpha uat prod production approve rollback' },
  { key: 'sdks', label: 'Use SDKs & CLI', summary: 'Use the clients in this repository without assuming package publication.', terms: 'nodejs python go client code automation cli cache retry' },
  { key: 'embed', label: 'Embed the vault widget', summary: 'Build your widget and understand the host-page trust boundary.', terms: 'web component shadow dom iframe postmessage origin token api-key' },
  { key: 'security', label: 'Understand the security model', summary: 'Stored permissions, encryption, and what a read actually exposes.', terms: 'roles organization tenant admin editor viewer promoter custody audit' },
  { key: 'agent-integration', label: 'Connect developer tools', summary: 'Separate current secret reads from controlled provider operations.', terms: 'agent harness codex hermes github broker profile run skills portable' },
  { key: 'mcp-server', label: 'Check MCP availability', summary: 'Capabilities and qualification come before client configuration.', terms: 'mcp oauth discovery protocol tools synthetic enabled unavailable' },
  { key: 'troubleshooting', label: 'Troubleshoot a request', summary: 'Authentication, permission, conflict, outage, and browser failures.', terms: 'error 400 401 403 404 409 429 500 503 cors network denied' },
] as const;
type Section = typeof guides[number]['key'];
const source = 'https://github.com/santapong/KeepSave/blob/develop/';

export function HelpPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const { capabilities, error, enabled } = useCapabilities();
  const requested = location.hash.slice(1);
  const active = guides.find((guide) => guide.key === requested)?.key ?? 'getting-started';
  const words = query.trim().toLowerCase().split(/\s+/).filter(Boolean);
  const matches = guides.filter((guide) => words.every((word) => `${guide.key} ${guide.label} ${guide.summary} ${guide.terms}`.toLowerCase().includes(word)));
  const current = guides.find((guide) => guide.key === active)!;
  function select(section: Section) {
    void navigate({ pathname: location.pathname, search: location.search, hash: `#${section}` });
  }
  const articles: Record<Section, ReactNode> = {
    'getting-started': <GettingStarted />,
    'developer-guide': <DeveloperGuide />,
    api: <APIReference />,
    'api-keys': <Credentials />,
    history: <History />,
    recovery: <Recovery />,
    promotion: <Promotion />,
    sdks: <SDKs />,
    embed: <EmbedWidget />,
    security: <SecurityModel />,
    'agent-integration': <DeveloperTools />,
    'mcp-server': <MCPServer enabled={enabled('mcp_delegation')} />,
    troubleshooting: <Troubleshooting />,
  };
  return (
    <div>
      <header style={{ padding: '28px 32px 0', maxWidth: 920 }}>
        <p className="cz-eyebrow">KeepSave documentation</p>
        <h1 style={{ ...pageTitle, marginTop: 8 }}>Build with a clear boundary.</h1>
        <p style={{ ...intro, marginBottom: 16 }}>Start with one project and one synthetic secret. Follow the task you need, inspect its outcome, and understand what reaches your client.</p>
        <label htmlFor="docs-search" style={{ ...stepDesc, display: 'block', marginBottom: 6 }}>Search documentation</label>
        <input id="docs-search" type="search" className="cz-input" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search tasks, APIs, and errors…" aria-describedby="docs-search-status" style={{ width: '100%' }} />
        <p id="docs-search-status" role="status" aria-label="Documentation search results" style={{ ...stepDesc, marginTop: 8 }}>{query.trim() ? `${matches.length} matching ${matches.length === 1 ? 'guide' : 'guides'}. Search matches guide titles, summaries, and topics.` : '13 task guides · examples use placeholders and synthetic data'}</p>
      </header>
      <div className="cz-docs">
        <div className="cz-docs-mobile">
          <label htmlFor="docs-section">Documentation section</label>
          <select id="docs-section" className="cz-input" value={active} onChange={(event) => select(event.target.value as Section)}>
            {guides.map((guide) => <option key={guide.key} value={guide.key}>{guide.label}</option>)}
          </select>
          {query.trim() && <div aria-label="Matching documentation guides">{matches.length ? matches.map((guide) => <button key={guide.key} className="cz-btn cz-btn-ghost" onClick={() => select(guide.key)} style={{ display: 'block', width: '100%', textAlign: 'left' }}>{guide.label}</button>) : <p>No guides match. Try “restore”, “401”, or “API key”.</p>}</div>}
        </div>
        <nav className="cz-docs-nav" aria-label="Documentation sections">
          <p className="cz-eyebrow" style={{ marginBottom: 12 }}>Choose a task</p>
          {matches.map((guide) => <button key={guide.key} aria-current={active === guide.key ? 'page' : undefined} aria-controls="docs-article" onClick={() => select(guide.key)} style={{ ...sidebarItem, background: active === guide.key ? 'var(--color-accent)' : 'transparent', color: active === guide.key ? 'var(--color-accent-foreground)' : 'var(--color-muted-foreground)', borderLeft: `2px solid ${active === guide.key ? 'var(--color-primary)' : 'transparent'}` }}>{guide.label}</button>)}
          {!matches.length && <p style={stepDesc}>No guides match. Try “restore”, “401”, or “API key”.</p>}
          {!!query && <button className="cz-btn cz-btn-ghost" onClick={() => setQuery('')}>Clear search</button>}
        </nav>
        <article id="docs-article" aria-label={current.label} style={{ flex: 1, maxWidth: 820, minWidth: 0 }}>
          <aside style={{ ...docCard, padding: 16 }} aria-label="Instance availability">
            <strong style={{ fontSize: 13 }}>This installation</strong>
            <p style={{ ...stepDesc, marginTop: 6 }}>{!capabilities ? error ? 'Feature availability could not be checked. Do not assume an integration is enabled; retry the capabilities request.' : 'Checking enabled features…' : capabilities.restricted_profile ? 'Core release profile. Account, workspace, and permitted vault operations are available. Optional platform controls depend on explicit operator configuration and acceptance.' : 'Feature availability depends on this installation. An enabled route is not evidence of a qualified external integration.'}</p>
            {capabilities && <p style={{ ...stepDesc, marginTop: 6 }}>Secret history: {enabled('secret_history') ? 'enabled' : 'unavailable'} · Encrypted recovery: {enabled('encrypted_recovery') ? 'enabled' : 'unavailable'} · MCP protocol: {enabled('mcp_delegation') ? 'enabled; client qualification still required' : 'unavailable'}</p>}
          </aside>
          <p className="cz-eyebrow">Task guide</p>
          <h2 style={{ ...pageTitle, marginTop: 8 }}>{current.label}</h2>
          <p style={intro}>{current.summary}</p>
          <p style={{ ...stepDesc, marginBottom: 24 }}><Link to={{ pathname: location.pathname, search: location.search, hash: `#${active}` }}>Link to this guide</Link> · API examples below use paths relative to your installation, not a promised hosted service.</p>
          {articles[active]}
          <footer style={{ ...stepDesc, padding: '12px 0 24px' }}>Source-aligned on October 25, 2026. These guides cover common workflows, not every deployment or failure mode. <a href={`${source}docs/README.md`} target="_blank" rel="noreferrer">Open the repository documentation</a> for operator procedures and acceptance limits.</footer>
        </article>
      </div>
    </div>
  );
}

function Card({ title, children }: { title: string; children: ReactNode }) {
  return <section style={docCard}><h3 style={docTitle}>{title}</h3>{children}</section>;
}
function GettingStarted() {
  return <>
    <Card title="Starting point"><p style={stepDesc}>Sign in to your installation. A local demo contains synthetic records; changes there are real changes to that demo database. Google and GitHub sign-in become available after the operator configures those providers; setup may still be shown as pending. A matching email address does not automatically link accounts.</p></Card>
    <Card title="1 · Save one synthetic secret"><ol style={docList}><li>In <strong>Projects</strong>, choose <strong>New project</strong> and name it <strong>Storefront</strong>.</li><li>Open the project, select <strong>Alpha</strong>, and choose <strong>Add Secret</strong>.</li><li>Use key <code>PAYMENTS_API_KEY</code> and value <code>sample-alpha-only</code>. Save the form.</li><li>Filter for the key, reveal the value deliberately, then hide it again. Switch to <strong>UAT</strong>: creating an Alpha value does not create a UAT value.</li></ol><p style={{ ...stepDesc, marginTop: 12 }}><strong>Expected result:</strong> one Alpha record in Storefront. No provider is contacted and no other environment changes.</p></Card>
    <Card title="2 · Change it and inspect history"><p style={stepDesc}>Edit the synthetic value and save. On PostgreSQL installations with history enabled, the row’s <strong>History</strong> button opens immutable revision metadata. The project’s <strong>History</strong> tab instead lists promotions. Follow “History, restore & rotation” for restoration and conflicts.</p></Card>
    <Card title="3 · Choose your next task"><ul style={docList}><li>Application or CI integration: create an explicitly scoped key and use a secret-read endpoint.</li><li>Release configuration: open <strong>Pipeline</strong>, preview a diff, and inspect approval state.</li><li>Recovery: use the <strong>Recovery</strong> tab where enabled. A plaintext .env export is a different operation.</li><li>Shared ownership: configure organization membership explicitly. A personal project does not become shared just because you create a workspace.</li></ul></Card>
  </>;
}

function DeveloperGuide() {
  return <>
    <Card title="Choose a disposable development environment"><p style={stepDesc}>The checkout uses Go/Gin, React 19 + TypeScript, and PostgreSQL 16. CI/build pins are Go 1.27.1 and Node 24.21.0; the Go module minimum is 1.26. Read the repository’s development and resource-budget instructions before starting builds. Do not run the development Compose file with real secrets.</p><CodeBlock code={`git clone https://github.com/santapong/KeepSave.git
cd KeepSave
# Review .env.example and docker-compose.yml first.
# Prepare unique local keys/configuration; never overwrite existing recovery material.
docker compose up --build`} /><p style={{ ...stepDesc, marginTop: 12 }}>Default Compose ports: frontend <code>http://127.0.0.1:3002</code>, API <code>http://127.0.0.1:8080</code>, PostgreSQL <code>127.0.0.1:5435</code>. The current demo may use different ports. Compose has development defaults; it is not the production control/runner topology.</p></Card>
    <Card title="Check readiness before debugging the UI"><CodeBlock code={`# Use your actual API origin, without /api/v1.
BASE_URL='http://127.0.0.1:8080'
curl --fail-with-body "$BASE_URL/healthz"
curl --fail-with-body "$BASE_URL/readyz"
curl --fail-with-body "$BASE_URL/api/v1/capabilities"`} /><p style={{ ...stepDesc, marginTop: 12 }}>A healthy process and a ready database are separate checks. The capabilities response tells you which features this installation enables. Social sign-in availability is reported by <code>GET /api/v1/auth/providers</code>; it does not prove a real consent/callback journey has passed.</p></Card>
    <Card title="Connect frontend and API"><p style={stepDesc}>The frontend defaults to <code>/api/v1</code>. Vite proxies that path to the local API. For a deployed application, serve frontend and API at the same origin or deliberately configure <code>VITE_API_BASE_URL</code> and allowed origins. Vite variables are public build-time configuration: put no provider client secret or vault key in them.</p></Card>
    <Card title="Know where a change belongs"><CodeBlock code={`backend/internal/api/          request translation, middleware, OpenAPI
backend/internal/policy/       shared authority and decisions
backend/internal/vault/        revisions, custody, recovery
backend/internal/service/      application services and compatibility adapters
backend/internal/repository/   persistence
frontend/src/api/client.ts     browser requests and session invalidation
frontend/src/pages/            task flows
frontend/src/embed/            isolated-style widget client`} /><p style={{ ...stepDesc, marginTop: 12 }}>Keep permission decisions in authorized services, not a UI or harness adapter. Use additive registered migrations. Extend <code>backend/internal/api/openapi/core.json</code> and regenerate client types when changing supported wire contracts. Crypto/auth/promotion changes have additional review requirements in <code>CLAUDE.md</code>.</p></Card>
    <Card title="Validate the affected behavior"><CodeBlock code={`# Run separately with the repository's resource limits.
cd frontend
npm ci
npm test
npm run lint
npm run build
npm run build:widget
# Backend checks belong in backend/; concurrency guarantees require real PostgreSQL.`} /><p style={{ ...stepDesc, marginTop: 12 }}>A successful build is not a login, recovery, provider, or UI acceptance result. Record the database, source revision, commands, tested behaviors, and limitations. See <a href={`${source}tests/PYRAMID.md`} target="_blank" rel="noreferrer">the test strategy</a> and <a href={`${source}docs/RUNBOOK.md`} target="_blank" rel="noreferrer">operator runbook</a>.</p></Card>
  </>;
}

function APIReference() {
  return <>
    <Card title="Use one installation origin"><p style={stepDesc}>Examples use placeholders. <code>BASE_URL</code> is your origin without <code>/api/v1</code>; <code>PROJECT_ID</code> is a project UUID you may access. Supply a current Bearer session in a private environment. Never print credentials or paste real values into issue reports, recorded terminals, or AI conversations.</p><CodeBlock code={`BASE_URL='https://your-keepsave.example'
PROJECT_ID='<authorized-project-uuid>'
# TOKEN and API_KEY are obtained privately, not committed to this script.`} /></Card>
    <Card title="Management requests use a browser session"><CodeBlock code={`POST /api/v1/auth/login
Content-Type: application/json

{"email":"developer@example.invalid","password":"<your-password>"}

# 200 — abbreviated shape; token is a secret
{"user":{"id":"<user-uuid>","email":"developer@example.invalid",…},"token":"<session-token>"}

POST /api/v1/projects
Authorization: Bearer <session-token>
Content-Type: application/json

{"name":"Storefront","description":"Synthetic integration project"}

# 201 — project is nested, not the response root
{"project":{"id":"<project-uuid>","name":"Storefront",…}}`} /><p style={{ ...stepDesc, marginTop: 12 }}>The ellipsis marks omitted fields in illustrative responses; it is not literal JSON. Project management, recovery, export, and promotion routes use human Bearer authentication. An API key is not a substitute on every route.</p></Card>
    <Card title="Create and list a secret"><CodeBlock code={`curl --fail-with-body -X POST "$BASE_URL/api/v1/projects/$PROJECT_ID/secrets" \\
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \\
  --data '{"key":"PAYMENTS_API_KEY","value":"sample-alpha-only","environment":"alpha"}'
# 201: {"secret":{"id":"<secret-uuid>","key":"PAYMENTS_API_KEY",…}}

curl --fail-with-body "$BASE_URL/api/v1/projects/$PROJECT_ID/secrets?environment=alpha" \\
  -H "X-API-Key: $API_KEY"
# 200: {"secrets":[{"id":"<secret-uuid>","key":"PAYMENTS_API_KEY","value":"sample-alpha-only",…}]}`} /><p style={{ ...stepDesc, marginTop: 12 }}><strong>Expected result:</strong> permitted records only. The project secret-list query is <code>environment=alpha</code>, not <code>env=alpha</code>. Reads return plaintext to your client even when the UI masks values.</p></Card>
    <Card title="Read a bounded selection"><CodeBlock code={`POST /api/v1/projects/{project_id}/secrets/batch
X-API-Key: <scoped-key>
Content-Type: application/json

{"environment":"alpha","keys":["PAYMENTS_API_KEY","ABSENT_KEY"]}

# 200 — synthetic abbreviated response
{"secrets":[{"key":"PAYMENTS_API_KEY","value":"sample-alpha-only",…}],"missing_keys":["ABSENT_KEY"]}`} /><p style={{ ...stepDesc, marginTop: 12 }}>Select 1–100 keys. This exact POST is classified as a read. Missing and out-of-scope keys appear in <code>missing_keys</code>; this does not reveal which denied keys exist. Do not automatically widen scope to make missing entries disappear.</p></Card>
    <Card title="Common route map"><RouteTable rows={[
      ['GET', '/projects', 'Current caller’s visible projects'], ['GET', '/projects/{id}/secrets?environment=alpha', 'Permitted current secret values'], ['PUT', '/projects/{id}/secrets/{secretId}', 'Update value; send expected_revision'], ['GET', '/projects/{id}/secrets/{secretId}/versions', 'Revision metadata in versioned vault'], ['POST', '/projects/{id}/secrets/{secretId}/versions/{revision}/restore', 'Append a restored current revision'], ['POST', '/projects/{id}/promote/diff', 'Preview hashes and actions'], ['GET', '/projects/{id}/promotions', 'Promotion history'], ['POST', '/auth/logout', 'Revoke the current human session'], ['GET', '/account/sessions', 'Owned session metadata'], ['DELETE', '/account/sessions/{sessionId}', 'Revoke an owned session'],
    ]} /><p style={{ ...stepDesc, marginTop: 12 }}>Paths are relative to <code>/api/v1</code>. This task map is intentionally partial. <a href={`${source}backend/internal/api/openapi/core.json`} target="_blank" rel="noreferrer">The maintained OpenAPI contract</a> defines supported management shapes.</p></Card>
  </>;
}

function Credentials() {
  return <>
    <Card title="Pick the credential for the job"><table style={apiTable}><thead><tr><th style={apiTh}>Credential</th><th style={apiTh}>Purpose and boundary</th></tr></thead><tbody>{[
      ['Human Bearer session', 'Account and management requests. Signed token, fixed maximum 24 hours, checked against current database session state. Account → sessions can revoke owned sessions.'],
      ['Scoped API key', 'Programmatic vault reads/writes on routes that accept X-API-Key. Bound to one project, optional environment, scopes, current authority, and expiry. Raw key is returned once.'],
      ['Agent lease token', 'Bounded secret-read authority where the legacy lease routes are enabled. Not a management session, provider token, or permission to redelegate.'],
      ['Delegated tool access', 'Separate OAuth/client-bound run authority for controlled tools where explicitly enabled and qualified. Never treat a browser token, API key, or self-reported harness name as a GitHub broker grant.'],
    ].map(([label, description]) => <tr key={label}><td style={apiTd}><strong>{label}</strong></td><td style={apiTd}>{description}</td></tr>)}</tbody></table></Card>
    <Card title="Create a narrow key"><p style={stepDesc}>Open the project’s <strong>API Keys</strong> tab for action/environment controls. The API also supports key-name patterns: the example below limits a test reader to <code>read:PAYMENTS_*</code> and <code>alpha</code>. Bare <code>read</code> retains the legacy all-key meaning within its authorized project/environment.</p><CodeBlock code={`POST /api/v1/api-keys
Authorization: Bearer <session-token>
Content-Type: application/json

{"name":"storefront-alpha-reader","project_id":"<project-uuid>","scopes":["read:PAYMENTS_*"],"environment":"alpha"}

# 201 — abbreviated; copy raw_key once into a private credential store
{"api_key":{"id":"<key-uuid>",…},"raw_key":"<one-time-key>"}`} /><p style={{ ...stepDesc, marginTop: 12 }}>Set an explicit expiry for a shorter integration. Omitted expiry defaults to 90 days; the service caps it at 365 days. Legacy <code>write</code> also grants delete; do not treat it as create/update-only. Secret removal uses tombstones in the versioned vault, rather than an assurance of ciphertext erasure.</p></Card>
    <Card title="Revoke and check the next request"><p style={stepDesc}>Delete the key in <strong>API Keys</strong> or revoke a session in <strong>Account</strong>. Subsequent authorized admissions must fail after revocation commits. An already admitted request may finish, and returned or cached data cannot be recalled. A failed network/database logout is not a successful server revocation.</p><CodeBlock code={`POST /api/v1/auth/logout
Authorization: Bearer <current-session-token>
# 204 — no response body

# Reusing that session on a protected request must fail authentication.`} /><p style={{ ...stepDesc, marginTop: 12 }}>Explicitly linking Google/GitHub requires a recent authenticated session. KeepSave uses stable provider identities; the same email does not grant an automatic account merge.</p></Card>
  </>;
}

function History() {
  return <>
    <Card title="Know which history you opened"><p style={stepDesc}>A secret row’s <strong>History</strong> button shows its revision metadata. The project <strong>History</strong> tab shows promotion requests. Versioned history, restore, and historical-key continuity require the PostgreSQL vault capability; the page does not promise equivalent behavior for other database modes.</p></Card>
    <Card title="Restore an earlier value as a new revision"><ol style={docList}><li>Open the secret’s <strong>History</strong> and inspect its current revision.</li><li>Choose <strong>Restore this revision</strong> on the desired earlier version. Values remain hidden in this view.</li><li>Confirm the exact key name. Restoration appends a new current revision; it does not rewrite the old entry.</li><li>If another caller edited the secret, reload and compare before deciding again.</li></ol><CodeBlock code={`GET /api/v1/projects/{project_id}/secrets/{secret_id}/versions
Authorization: Bearer <session-token>
# 200 — metadata array, not historical values
[{"revision":2,"operation":"update","created_at":"2026-10-25T00:00:00Z"},…]

POST /api/v1/projects/{project_id}/secrets/{secret_id}/versions/1/restore
Authorization: Bearer <session-token>
Content-Type: application/json

{"expected_current_revision":2}
# 200 — restored record now has a new revision (3 in this example).`} /><p style={{ ...stepDesc, marginTop: 12 }}>An explicitly authorized historical-value read uses <code>GET …/versions/1</code>. Treat its returned value as a credential. Baseline entries label what existed at migration; they do not reconstruct a missing past.</p></Card>
    <Card title="Protect concurrent edits"><CodeBlock code={`PUT /api/v1/projects/{project_id}/secrets/{secret_id}
Authorization: Bearer <session-token>
Content-Type: application/json

{"value":"sample-replacement","expected_revision":3}
# 409 if revision 3 is no longer current — reload before another decision.`} /><p style={{ ...stepDesc, marginTop: 12 }}>Send the last observed revision on updates; it is optional for legacy update callers, so omitting it does not give the same stale-write protection. Restore requires <code>expected_current_revision</code>. Never retry a conflict by silently replacing the expected revision.</p></Card>
    <Card title="Rotation changes encryption keys"><p style={stepDesc}>The project’s <strong>Rotate all</strong> action rotates data-encryption keys. It does not rotate a database password or external service API key. Retained revisions and promotion snapshots keep the referenced historical keys. Losing required master/Transit recovery material is not repaired by a new empty configuration.</p></Card>
  </>;
}

function Recovery() {
  return <>
    <Card title="A plaintext export is a credential copy"><p style={stepDesc}>Use <strong>Export plaintext .env</strong> and choose an environment. The resulting file contains readable values. Keep it out of Git, build logs, screenshots, and public downloads. Deleting a browser view does not delete exported copies.</p><CodeBlock code={`# Human Bearer authentication; this route is not an API-key endpoint.
# The file form returns text; without format=file the response is JSON {content,environment}.
curl --fail-with-body \\
  "$BASE_URL/api/v1/projects/$PROJECT_ID/env-export?environment=alpha&format=file" \\
  -H "Authorization: Bearer $TOKEN" --output storefront.alpha.env`} /></Card>
    <Card title="Import deliberately"><p style={stepDesc}>Import into an explicit project/environment. Review whether overwrite is intended before submitting. An import is a vault mutation; it is not an encrypted disaster-recovery bundle.</p><CodeBlock code={`POST /api/v1/projects/{project_id}/env-import
Authorization: Bearer <session-token>
Content-Type: application/json

{"environment":"alpha","content":"PAYMENTS_API_KEY=sample-alpha-only\n","overwrite":false}`} /></Card>
    <Card title="Verify an encrypted recovery bundle"><ol style={docList}><li>Where <strong>Recovery</strong> is enabled, create/download an encrypted bundle and retain it externally with documented key dependencies.</li><li>Verify integrity and decryptability using the installation’s authorized recovery material.</li><li>Prove recovery in a fresh isolated PostgreSQL target before relying on it.</li><li>For live recovery, preview metadata and explicitly select records with current revision checks.</li></ol><p style={{ ...stepDesc, marginTop: 12 }}>The API provides <code>POST /projects/&#123;id&#125;/backups</code>, <code>/backups/verify</code>, <code>/backups/preview</code>, and <code>/backups/restore</code> beneath <code>/api/v1</code>. The encrypted bundle is not self-sufficient without its required recovery material. Verification against the current server is not proof of an independent restore.</p></Card>
    <Card title="Recovery restores vault data, not authority"><p style={stepDesc}>Recovery does not import sessions, API keys, grants, tokens, or approvals. Live restoration does not silently replace a whole project or resurrect deleted records. Keep recovery keys separate from bundles and use the <a href={`${source}docs/RUNBOOK.md`} target="_blank" rel="noreferrer">operator recovery procedure</a> for exact environment requirements.</p></Card>
  </>;
}

function Promotion() {
  return <>
    <Card title="Preview before applying"><p style={stepDesc}>From the project’s <strong>Pipeline</strong> tab, choose Alpha → UAT or UAT → Production and inspect the diff. Diff entries disclose actions and comparison hashes, not plaintext source/target values. <strong>Skip</strong> leaves existing target keys untouched; <strong>Overwrite</strong> replaces selected matching target keys.</p><CodeBlock code={`POST /api/v1/projects/{project_id}/promote/diff
Authorization: Bearer <session-token>
Content-Type: application/json

{"source_environment":"alpha","target_environment":"uat","keys":["PAYMENTS_API_KEY"]}
# 200: {"diff":[{"key":"PAYMENTS_API_KEY","action":"add",…}]}

POST /api/v1/projects/{project_id}/promote
Authorization: Bearer <session-token>
Content-Type: application/json

{"source_environment":"alpha","target_environment":"uat","keys":["PAYMENTS_API_KEY"],"override_policy":"skip","notes":"Synthetic QA example"}`} /></Card>
    <Card title="Inspect state, not just the HTTP request"><p style={stepDesc}>The create endpoint returns <code>&#123;promotion: …&#125;</code>: <code>200</code> for immediately applied work or <code>202</code> for pending approval. A pending request has not changed production values. Production approval rules and role checks apply; the requester cannot self-approve. Inspect <strong>History</strong> for actual status and permitted approve/reject/rollback actions. A kill switch may refuse new promotions/approvals while inspection remains available.</p></Card>
    <Card title="A promotion copies configuration"><p style={stepDesc}>It does not deploy an application, restart a service, or prove that an external credential works. Compare the target environment after completion. Rollback is a separate authorized vault mutation with its own audit/history effects.</p></Card>
  </>;
}

function SDKs() {
  return <>
    <Card title="Start with the clients in this checkout"><p style={stepDesc}>Node.js, Python, and Go SDK source lives in <code>sdks/</code>; CLI commands live in <code>backend/cmd/keepsave/</code>. Repository source and tests do not establish npm/PyPI/remote Go publication. Build or install the reviewed local source using its package metadata; pin the revision you qualified.</p></Card>
    <Card title="Node.js: read into application code"><CodeBlock code={`// Import from your locally built/pinned SDK distribution.
import { KeepSaveClient } from './vendor/keepsave/index.js';

const client = new KeepSaveClient({
  baseUrl: process.env.KEEPSAVE_ORIGIN,
  apiKey: process.env.KEEPSAVE_API_KEY,
  cacheTtl: 0, // Avoid retaining a plaintext read cache for this example.
  maxRetries: 0, // Keep retry decisions explicit during integration.
});
const secrets = await client.listSecrets(process.env.KEEPSAVE_PROJECT_ID, 'alpha');
const paymentKey = secrets.find((secret) => secret.key === 'PAYMENTS_API_KEY')?.value;
if (!paymentKey) throw new Error('Required credential missing or outside scope');
// Pass paymentKey directly to your application's SDK. Do not log it.`} /><p style={{ ...stepDesc, marginTop: 12 }}>The actual class is <code>KeepSaveClient</code>; configuration uses <code>baseUrl</code>. Revocation cannot recall a value already returned to your process. Review the client’s retry/cache defaults before adopting it for sensitive workflows.</p></Card>
    <Card title="CLI and Go module"><CodeBlock code={`# From a reviewed checkout, inspect available commands first.
cd backend
go run ./cmd/keepsave --help

# For a Go application using the local SDK module:
go mod edit -replace=github.com/santapong/KeepSave/sdks/go=/absolute/path/to/KeepSave/sdks/go`} /><p style={{ ...stepDesc, marginTop: 12 }}>Do not pipe untrusted secret values into shell <code>eval</code>, shell source, or a workflow command channel. Parse structured JSON in the application language. Secret values may contain newlines or shell syntax.</p></Card>
  </>;
}

function EmbedWidget() {
  return <>
    <Card title="Host a reviewed bundle"><p style={stepDesc}>Build with <code>npm run build:widget</code> from <code>frontend/</code>. It emits <code>dist-embed/keepsave-widget.umd.js</code> and the ES module equivalent. Host that reviewed immutable bundle yourself; no public KeepSave CDN is assumed.</p><CodeBlock code={`<script src="/assets/keepsave-widget.umd.js"></script>
<keepsave-widget
  id="vault-widget"
  api-url="https://your-keepsave.example"
  project-id="<authorized-project-uuid>"
  theme="dark"
  mode="read"
></keepsave-widget>`} /><p style={{ ...stepDesc, marginTop: 12 }}><code>api-url</code> is an origin without <code>/api/v1</code>. Read mode is the default; <code>readwrite</code> adds controls but grants no server permission.</p></Card>
    <Card title="Choose a reviewed credential handoff"><p style={stepDesc}>A direct <code>token</code> or <code>api-key</code> attribute trusts the host page with that credential. Do not put a long-lived key into public HTML. Without direct credentials, the widget uses the project’s enabled specific-origin allowlist before its postMessage handshake. The host must verify both origin and expected window; wildcard origins are not an integration shortcut.</p><p style={{ ...stepDesc, marginTop: 12 }}>Shadow DOM isolates appearance, not data from a malicious host. Follow the full <a href={`${source}frontend/src/embed/INTEGRATION.md`} target="_blank" rel="noreferrer">widget integration guide</a> for handoff, events, cache boundaries, and qualification.</p></Card>
  </>;
}

function SecurityModel() {
  return <>
    <Card title="Server authority is the boundary"><p style={stepDesc}>Stored project ownership, organization membership, current credentials, scopes, environment, expiry, and revocation determine access. Hiding a button or supplying an ID does not authorize an operation. Roles remain Admin, Editor, Promoter, and Viewer; public signup or organization ownership does not grant platform-wide operator authority.</p></Card>
    <Card title="Encryption does not hide authorized reads"><p style={stepDesc}>Values use AES-256-GCM envelope encryption with project data keys and separately held root/Transit material. The database alone is not the complete recovery set. On a normal vault read, the server decrypts the authorized value and returns it to the client. UI masking only affects display; the browser, application, exported file, or tool process may hold plaintext.</p></Card>
    <Card title="Use evidence appropriate to the claim"><p style={stepDesc}>Audit records help inspect decisions and mutations. They are not proof that every external device is controlled or that every arbitrary integration is compliant. Do not send real values into screenshots, test logs, analytics, or model prompts. Self-hosting KeepSave does not make a developer tool’s configured model local.</p></Card>
  </>;
}

function DeveloperTools() {
  return <>
    <Card title="Current vault integration"><p style={stepDesc}>A backend application, CI job, or developer tool can use a scoped key on accepted vault-read routes. The returned secret leaves KeepSave’s custody and reaches that process. This is credential delivery, not a guarantee that an AI model never sees the credential. Give only required non-production access and keep model inputs under your control.</p></Card>
    <Card title="Controlled provider access is a separate capability"><p style={stepDesc}>The harness-neutral platform separates portable profiles from client-bound runs. Its GitHub pilot is designed for broker-held provider tokens and bounded repository operations. An ordinary API key, browser session, installed skill, or reported profile digest is not that authority. Codex and Hermes are qualification candidates; support is published per tested version/capability, not promised for every present or future harness.</p></Card>
    <Card title="Qualification checklist"><ul style={docList}><li>Check the installation’s capabilities and operator acceptance record.</li><li>Distinguish server-enforced controls from local administrator settings and unverified claims.</li><li>Test allowed/denied targets, expiry, revocation, cancellation, and result retrieval separately.</li><li>Use synthetic fixtures before any separately approved real GitHub/model exercise.</li><li>Do not install an unverified package or enable API-host connector execution to bypass an unavailable integration.</li></ul><p style={{ ...stepDesc, marginTop: 12 }}>See the <a href={`${source}docs/design/2026-10-02-product-roadmap/README.md`} target="_blank" rel="noreferrer">implementation roadmap</a> and acceptance records for current scope. Plans and source presence are not release evidence.</p></Card>
  </>;
}

function MCPServer({ enabled }: { enabled: boolean }) {
  return <>
    <Card title={enabled ? 'Protocol enabled · qualification required' : 'MCP protocol unavailable on this installation'}><p style={stepDesc}>{enabled ? 'The server reports the new MCP protocol capability. This does not establish a successful login/discovery/tool journey for your chosen client or grant provider access.' : 'Do not configure a fictional npm server or assume that a saved catalog entry executes tools. The core vault remains usable through its accepted APIs.'}</p></Card>
    <Card title="Protocol, credentials, and tools are distinct"><p style={stepDesc}>The intended standards-based transport is <code>/mcp</code>, separate from <code>/api/v1</code>. Delegated OAuth tokens authorize KeepSave operations; they are not GitHub sign-in tokens or broker-held GitHub App credentials. A configured canonical origin, exact callbacks, client registration, consent, and resource binding must match the accepted installation.</p></Card>
    <Card title="Do not infer support from configuration"><p style={stepDesc}>Check exact protocol negotiation, discovery, invocation, cancellation, expiry, and revocation with the actual client build. Optional profiles, artifacts, runs, runner dispatch, and provider integration have separate gates. Legacy MCP execution and OAuth issuance remain unavailable in the core release. An enabled synthetic protocol test does not establish a real GitHub or native-harness workflow.</p></Card>
  </>;
}

function Troubleshooting() {
  return <>
    <Card title="Start with the request you actually sent"><p style={stepDesc}>Record method, path, status, timestamp, source/client version, database mode, and sanitized error code. Keep headers, request values, tokens, recovery bundles, and secret-bearing response bodies out of shared logs. Check capabilities before interpreting a disabled control as a UI failure.</p></Card>
    <Card title="Common responses"><table style={apiTable}><thead><tr><th style={apiTh}>Status</th><th style={apiTh}>Next action</th></tr></thead><tbody>{[
      ['400', 'Check JSON, UUIDs, environment spelling, required fields, and the actual route contract. The secret list uses environment, while legacy agent endpoints may differ.'],
      ['401', 'Sign in again for expired/revoked human sessions. For accepted API-key routes, check the key and expiry privately. Do not repeatedly retry the same invalid credential.'],
      ['403 / 404', 'Confirm current membership, project ownership, key action/environment/key scope. Missing and denied resources may be deliberately indistinguishable; a 404 is not proof a foreign resource exists or does not exist.'],
      ['409', 'A duplicate key or stale revision may conflict. Reload the current state and make a fresh decision; do not silently overwrite a concurrent edit.'],
      ['429', 'Respect the rate limit and back off. Do not multiply retries across UI, SDK, and proxy layers.'],
      ['500 / 503', 'Check readyz, database/Transit connectivity, and sanitized server diagnostics. CAPABILITY_UNAVAILABLE means the feature is disabled in this release, not that credentials should be broadened.'],
    ].map(([status, action]) => <tr key={status}><td style={apiTd}><code>{status}</code></td><td style={apiTd}>{action}</td></tr>)}</tbody></table><CodeBlock code={`# Representative structured error shape; messages depend on the failing route.
{"error":{"code":503,"error_code":"CAPABILITY_UNAVAILABLE","message":"capability is unavailable in the core release"},"capability":"mcp_execution"}`} /></Card>
    <Card title="Browser and provider failures"><ul style={docList}><li><strong>Network/CORS:</strong> verify frontend API base path, TLS, reverse proxy, and exact allowed origin. A browser failure does not prove a mutation failed; inspect authoritative state before retrying.</li><li><strong>Google/GitHub button unavailable:</strong> the operator must configure backend-only provider secrets, canonical origin, and exact callback URLs, then complete real acceptance. No client secret belongs in Vite configuration.</li><li><strong>Missing History/Recovery:</strong> check PostgreSQL vault capabilities and server configuration. Do not interpret a legacy database mode as equivalent recovery support.</li><li><strong>Logout failed:</strong> retain the failure status; reconnect and revoke from Account. Closing a tab does not prove server-side revocation.</li><li><strong>Restore conflict:</strong> reload the secret, compare the new revision, then choose whether restoration is still appropriate.</li></ul></Card>
    <Card title="Escalate safely"><p style={stepDesc}>Include the smallest synthetic reproduction, expected/actual behavior, safe request metadata, and whether the server accepted the operation. Use the repository runbooks for corruption, lost recovery keys, migration failures, or uncertain external effects. Do not run live restoration, credential changes, or destructive cleanup as a generic troubleshooting step.</p></Card>
  </>;
}

function CodeBlock({ code }: { code: string }) {
  return <pre style={codeBlock}><code>{code}</code></pre>;
}
function RouteTable({ rows }: { rows: [string, string, string][] }) {
  return <div style={{ overflowX: 'auto' }}><table style={apiTable}><thead><tr><th style={apiTh}>Method</th><th style={apiTh}>Path</th><th style={apiTh}>Outcome</th></tr></thead><tbody>{rows.map(([method, path, outcome]) => <tr key={`${method}:${path}`}><td style={apiTd}><code>{method}</code></td><td style={{ ...apiTd, overflowWrap: 'anywhere' }}><code>{path}</code></td><td style={apiTd}>{outcome}</td></tr>)}</tbody></table></div>;
}
const sidebarItem: CSSProperties = { display: 'block', width: '100%', textAlign: 'left', padding: '9px 12px', border: 'none', borderRadius: 4, fontSize: 13, cursor: 'pointer', marginBottom: 3, lineHeight: 1.45 };
const pageTitle: CSSProperties = { fontSize: 28, fontWeight: 700, marginBottom: 8 };
const intro: CSSProperties = { fontSize: 15, color: 'var(--color-muted-foreground)', marginBottom: 24, lineHeight: 1.65 };
const stepDesc: CSSProperties = { fontSize: 14, color: 'var(--color-muted-foreground)', lineHeight: 1.7 };
const docCard: CSSProperties = { background: 'var(--color-card)', border: '1px solid var(--color-border)', borderRadius: 'var(--radius)', padding: 20, marginBottom: 16, overflow: 'auto' };
const docTitle: CSSProperties = { fontSize: 17, fontWeight: 600, marginBottom: 12, lineHeight: 1.45 };
const docList: CSSProperties = { paddingLeft: 20, display: 'flex', flexDirection: 'column', gap: 10, fontSize: 14, color: 'var(--color-muted-foreground)', lineHeight: 1.7 };
const codeBlock: CSSProperties = { background: 'var(--color-secondary)', border: '1px solid var(--color-border)', borderRadius: 6, padding: 16, fontSize: 12, fontFamily: 'ui-monospace, "SF Mono", Menlo, monospace', overflow: 'auto', lineHeight: 1.7, marginTop: 12, maxWidth: '100%' };
const apiTable: CSSProperties = { width: '100%', borderCollapse: 'collapse', lineHeight: 1.6 };
const apiTh: CSSProperties = { textAlign: 'left', padding: '8px 10px', fontSize: 11, fontWeight: 600, color: 'var(--color-muted-foreground)', borderBottom: '1px solid var(--color-border)', textTransform: 'uppercase', letterSpacing: '0.05em' };
const apiTd: CSSProperties = { padding: '10px', borderBottom: '1px solid var(--color-border)', fontSize: 13, verticalAlign: 'top' };

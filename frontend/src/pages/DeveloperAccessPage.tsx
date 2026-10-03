import { useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useSearchParams } from 'react-router-dom';
import { listProjects, getAuthToken } from '../api/client';
import * as api from '../api/toolPlatform';
import type { ToolPackage, ToolReceipt, ToolRun, ToolOperation } from '../api/coreTypes';
import type { Project } from '../types';
import { ConfirmDialog } from '../components/ConfirmDialog';
import { PageHeader } from '../components/cosmic/primitives';

type Workspace = Awaited<ReturnType<typeof api.toolWorkspace>>;
const clients = { codex: 'keepsave-codex-linux-v1', hermes: 'keepsave-hermes-linux-v1' };
const versions = { codex: '0.153.3', hermes: '0.21.5' };
const skill = '---\nname: keepsave-review-v1\ndescription: Review the repository approved for a KeepSave run.\n---\nUse available_runs to choose your approved run. Request repository_tree and read_file with explicit request keys. Inspect operation_status and retrieve operation_result only for completed operations. Stay within the resolved commit and approved repository. Cancel the operation or run explicitly when finished.\n';
function download(name: string, text: string) {
  const url = URL.createObjectURL(new Blob([text], { type: 'application/json' }));
  const a = document.createElement('a'); a.href = url; a.download = name; a.click(); URL.revokeObjectURL(url);
}
function Field({ name, label, children, ...props }: { name: string; label: string; children?: ReactNode; type?: string; placeholder?: string; defaultValue?: string; required?: boolean; min?: number; max?: number }) {
  return <label>{label}{children || <input name={name} required {...props} />}</label>;
}
function ResourceForm({ title, busy, submit, children }: { title: string; busy: boolean; submit: (values: FormData, form: HTMLFormElement) => Promise<void>; children: ReactNode }) {
  function handle(e: FormEvent<HTMLFormElement>) { e.preventDefault(); const form = e.currentTarget; void submit(new FormData(form), form); }
  return <form className="cz-card ks-platform-form" onSubmit={handle}><h2>{title}</h2>{children}<button className="cz-btn" disabled={busy}>Save {title.toLowerCase()}</button></form>;
}
const val = (f: FormData, name: string) => String(f.get(name) || '');

export function DeveloperAccessPage() {
  const [params, setParams] = useSearchParams();
  const project = params.get('project') || '';
  const [projects, setProjects] = useState<Project[]>([]);
  useEffect(() => { let active = true; listProjects().then(v => { if (active) setProjects(v); }).catch(() => {}); return () => { active = false; }; }, []);
  return <div className="cz-page"><PageHeader eyebrow="Controlled tools" title="Developer access" sub="Connect a repository, approve a review profile and grant short-lived access to a developer’s tools." />
    <label className="ks-platform-project">Project <select className="cz-input" aria-label="Project" value={project} onChange={e => setParams(e.target.value ? { project: e.target.value } : {})}><option value="">Choose a project</option>{projects.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
    {project && <AccessWorkspace key={project} project={project} />}
  </div>;
}

function AccessWorkspace({ project }: { project: string }) {
  const [data, setData] = useState<Workspace | null>(null);
  const [tab, setTab] = useState('runs');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [message, setMessage] = useState('');
  const [receipts, setReceipts] = useState<ToolReceipt[] | null>(null);
  const [operation, setOperation] = useState<{ run: ToolRun; value: ToolOperation } | null>(null);
  const [confirm, setConfirm] = useState<{ title: string; description: string; action: () => Promise<void> } | null>(null);
  const alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  async function reload() { const result = await api.toolWorkspace(project); if (alive.current) setData(result); }
  useEffect(() => { let active = true; api.toolWorkspace(project).then(v => { if (active) setData(v); }).catch(() => { if (active) setError('Could not load this project’s tool access. Check your permissions or server setup.'); }); return () => { active = false; }; }, [project]);
  async function act(task: () => Promise<unknown>, success: string, refresh = true) {
    if (busy) return; setBusy(true); setError(''); setMessage('');
    try { await task(); if (!alive.current) return; if (refresh) await reload(); if (alive.current) setMessage(success); }
    catch (e) { if (alive.current) setError(e instanceof Error ? e.message : 'The operation could not be confirmed.'); }
    finally { if (alive.current) setBusy(false); }
  }
  const save = (kind: string, input: unknown) => act(() => api.createToolResource(project, kind, input), 'Saved. Permission and approval checks apply before use.');
  const revoke = (kind: string, id: string) => setConfirm({ title: 'Revoke this access?', description: 'Subsequent admissions and result retrieval through this authority will be denied. Work already admitted may finish.', action: () => act(() => api.revokeToolResource(project, kind, id), 'Revocation confirmed.') });
  const select = (name: string, rows: { id: string; label: string }[]) => <select name={name} required><option value="">Choose…</option>{rows.map(x => <option key={x.id} value={x.id}>{x.label}</option>)}</select>;
  const options = (rows: { id: string }[] = []) => rows.map(x => ({ id: x.id, label: x.id.slice(0, 8) }));
  const controls = (kind: string, id: string) => <button className="cz-btn" disabled={busy} onClick={() => revoke(kind, id)}>Revoke</button>;
  async function exportPackage(id: string) {
    const started = getAuthToken(); const pkg: ToolPackage = await api.toolPackage(project, id);
    if (!alive.current || getAuthToken() !== started) throw new Error('Your account changed. Reload to export.');
    download(`keepsave-${pkg.harness}-package.json`, JSON.stringify(pkg, null, 2));
  }
  return <>
    <p className="cz-muted">Server permissions govern every call. Client packages remain compatibility candidates until tested on your installation.</p>
    {error && <p role="alert" className="ks-auth-error">{error}</p>}{message && <p role="status">{message}</p>}
    <nav className="ks-platform-tabs" aria-label="Access setup">{[['runs', 'My runs'], ['repository', 'Repository'], ['profiles', 'Review profiles'], ['grants', 'Team access']].map(([id, label]) => <button key={id} className="cz-btn" aria-pressed={tab === id} onClick={() => setTab(id)}>{label}</button>)}<button className="cz-btn" disabled={busy} onClick={() => void act(reload, 'Updated.', false)}>Refresh</button></nav>
    {!data && !error && <p role="status">Loading tool access…</p>}
    {data && tab === 'repository' && <>
      <ResourceForm title="GitHub connection" busy={busy} submit={async (f, form) => {
        const file = f.get('key') as File;
        try { if (!file || !file.size || file.size > 16384) throw new Error('Choose a GitHub App private key file smaller than 16 KiB.'); await save('connections', { app_id: Number(val(f, 'app')), installation_id: Number(val(f, 'installation')), private_key_pem: await file.text() }); }
        catch (e) { setError(e instanceof Error ? e.message : 'Could not read the key.'); }
        finally { form.reset(); }
      }}>
        <p>Use a separate GitHub App with repository contents read access. Your sign-in connection is separate.</p>
        <Field name="app" label="GitHub App ID" type="number" min={1} /><Field name="installation" label="Installation ID" type="number" min={1} /><Field name="key" label="App private key file" type="file" />
      </ResourceForm>
      <ResourceForm title="Repository binding" busy={busy} submit={f => save('bindings', { connection_id: val(f, 'connection'), environment: val(f, 'environment'), target: { repository_id: Number(val(f, 'repository_id')), owner: val(f, 'owner'), repository: val(f, 'repository'), reference: val(f, 'reference') } })}>
        <Field name="connection" label="Connection">{select('connection', options(data.connections))}</Field><Field name="environment" label="Environment"><select name="environment"><option>alpha</option><option>uat</option><option>development</option></select></Field>
        <Field name="repository_id" label="GitHub repository ID" type="number" min={1} /><Field name="owner" label="Repository owner" /><Field name="repository" label="Repository name" /><Field name="reference" label="Reference" defaultValue="main" />
      </ResourceForm>
      <h2>Connected repositories</h2>{data.bindings.map(b => <div className="ks-connections-row" key={b.id}><div><strong>{b.target.owner}/{b.target.repository}</strong><p>{b.target.reference} · {b.id.slice(0, 8)}</p></div>{controls('bindings', b.id)}<button className="cz-btn" disabled={busy} onClick={() => setConfirm({ title: 'Check this GitHub connection?', description: 'KeepSave will mint a repository-scoped read token and read repository metadata and the configured reference from GitHub. It will not write repository content.', action: () => act(async () => { const result = await api.checkToolConnection(project, b.connection_id, b.id); if (result.status !== 'succeeded') throw new Error(`Connection check ${result.status}. See its audit record.`); }, 'Read-only connection check succeeded.') })}>Check connection</button></div>)}
      {data.connections.map(c => <div className="ks-connections-row" key={c.id}><div>GitHub App {c.app_id} · installation {c.installation_id}<p>{c.id.slice(0, 8)}</p></div>{controls('connections', c.id)}</div>)}
    </>}
    {data && tab === 'profiles' && <>
      <ResourceForm title="Review skill" busy={busy} submit={f => save('artifacts', { name: 'keepsave-review-v1', source: val(f, 'source') })}><p>Each save creates an immutable instruction-only version.</p><Field name="source" label="Review instructions"><textarea name="source" required rows={10} defaultValue={skill} maxLength={65536} /></Field></ResourceForm>
      <ResourceForm title="Portable profile" busy={busy} submit={f => save('profiles', { artifact_id: val(f, 'artifact') })}><Field name="artifact" label="Skill version">{select('artifact', data.artifacts.map(a => ({ id: a.id, label: `${a.name} · ${a.digest.slice(0, 12)}` })))}</Field><p>One repository, ten minutes, read-only tree and file access. An independent administrator must approve this version.</p></ResourceForm>
      {data.profiles.map(p => <div className="ks-connections-row" key={p.id}><div><strong>Review profile {p.id.slice(0, 8)}</strong><p>{p.approved ? 'Approved' : 'Awaiting independent approval'} · {p.digest.slice(0, 12)}</p></div><button className="cz-btn" disabled={busy || p.approved} onClick={() => setConfirm({ title: 'Approve this exact profile?', description: `Approve digest ${p.digest}. Approval lasts at most 24 hours and cannot be granted by its creator.`, action: () => act(() => api.approveToolResource(project, 'profiles', p.id, p.digest), 'Profile approval confirmed.') })}>Approve</button>{controls('profiles', p.id)}</div>)}
      <ResourceForm title="Client package" busy={busy} submit={f => { const harness = val(f, 'harness') as keyof typeof versions; return save('packages', { profile_id: val(f, 'profile'), harness, version: versions[harness] }); }}><Field name="profile" label="Profile">{select('profile', options(data.profiles))}</Field><Field name="harness" label="Client"><select name="harness"><option value="codex">Codex 0.153.3</option><option value="hermes">Hermes 0.21.5</option></select></Field><p>Export into a dedicated client workspace. Generated settings do not attest or control an unrestricted device.</p></ResourceForm>
      {data.packages.map(p => <div className="ks-connections-row" key={p.id}><div><strong>{p.harness} {p.version}</strong><p>{p.approved ? 'Approved' : 'Awaiting independent approval'} · {p.digest.slice(0, 12)}</p></div><button className="cz-btn" disabled={busy} onClick={() => void act(() => exportPackage(p.id), 'Package downloaded. Use the documented exporter and compatibility check.', false)}>Export</button><button className="cz-btn" disabled={busy || p.approved} onClick={() => setConfirm({ title: 'Approve this exact package?', description: `Approve digest ${p.digest} and its pinned profile. Its creator cannot approve it.`, action: () => act(() => api.approveToolResource(project, 'packages', p.id, p.digest), 'Package approval confirmed.') })}>Approve</button>{controls('packages', p.id)}</div>)}
    </>}
    {data && tab === 'grants' && <>
      <ResourceForm title="Runner enrollment" busy={busy} submit={f => save('workloads', { certificate_sha256: val(f, 'certificate'), image_digest: val(f, 'image') })}><p>The operator provisions a separate runner and verifies its isolation before enabling dispatch.</p><Field name="certificate" label="Runner certificate SHA-256" /><Field name="image" label="Approved connector image digest" placeholder="registry/connector@sha256:…" /></ResourceForm>
      {data.workloads.map(w => <div className="ks-connections-row" key={w.id}><div>Runner {w.id.slice(0, 8)}<p>{w.image_digest}</p></div>{controls('workloads', w.id)}</div>)}
      <ResourceForm title="Developer access" busy={busy} submit={f => { const harness = val(f, 'harness') as keyof typeof clients; return save('grants', { profile_id: val(f, 'profile'), package_id: val(f, 'package'), binding_id: val(f, 'binding'), workload_id: val(f, 'workload'), actor_id: val(f, 'actor'), client_id: clients[harness], expires_at: new Date(Date.now() + 10 * 60000).toISOString() }); }}>
        <Field name="actor" label="Developer member ID" /><Field name="profile" label="Approved profile">{select('profile', options(data.profiles.filter(p => p.approved)))}</Field><Field name="package" label="Approved client package">{select('package', data.packages.filter(p => p.approved).map(p => ({ id: p.id, label: `${p.harness} ${p.version} · ${p.id.slice(0, 8)}` })))}</Field><Field name="binding" label="Repository">{select('binding', data.bindings.map(b => ({ id: b.id, label: `${b.target.owner}/${b.target.repository}` })))}</Field><Field name="workload" label="Runner">{select('workload', options(data.workloads))}</Field><Field name="harness" label="Client"><select name="harness"><option value="codex">Codex</option><option value="hermes">Hermes</option></select></Field><p>This access expires in ten minutes. A new authorization is required for an extension.</p>
      </ResourceForm>
      {data.grants.map(g => <div className="ks-connections-row" key={g.id}><div>Access {g.id.slice(0, 8)}<p>{g.client_id} · expires {new Date(g.expires_at).toLocaleString()}</p></div>{controls('grants', g.id)}</div>)}
    </>}
    {data && tab === 'runs' && <>
      <ResourceForm title="Review run" busy={busy} submit={f => act(async () => { const delegation = data.delegations.find(d => d.family_id === val(f, 'family')); if (!delegation) throw new Error('Connect your client to KeepSave first.'); await api.createToolRun(project, { grant_id: val(f, 'grant'), family_id: delegation.family_id, client_id: delegation.client_id, request_key: val(f, 'request_key'), duration_seconds: 600 }); }, 'Run created. Refresh to see its state.') }>
        <Field name="grant" label="Granted access">{select('grant', data.grants.map(g => ({ id: g.id, label: `${g.client_id} · ${g.id.slice(0, 8)}` })))}</Field><Field name="family" label="Your connected client">{select('family', data.delegations.filter(d => !d.requires_refresh).map(d => ({ id: d.family_id, label: `${d.harness} · ${d.family_id.slice(0, 8)}` })))}</Field><Field name="request_key" label="Run request identifier" defaultValue={crypto.randomUUID()} /><p>Connect the client through its exported package first. Separate clients require separate runs. Reuse the request identifier if the response is lost.</p>
      </ResourceForm>
      {data.runs.length === 0 && <p>No current runs. A client connection and approved access are required.</p>}
      {data.runs.map(r => <div className="cz-card ks-platform-form" key={r.id}><h2>{r.repository || `Run ${r.id.slice(0, 8)}`}</h2><p>{r.environment || 'Scope awaiting verification'} · {r.reference || 'No captured reference'}</p><p>{r.client_id} · {r.state} · expires {new Date(r.expires_at).toLocaleString()}</p><p>Commit {r.commit || 'Waiting for reference resolution'}</p><div className="ks-platform-tabs"><button className="cz-btn" disabled={busy} onClick={() => void act(async () => { const response = await api.toolReceipts(project, r); if (alive.current) setReceipts(response.receipts); }, 'Receipts loaded.', false)}>View receipts</button><button className="cz-btn" disabled={busy || !['active', 'preparing'].includes(r.state)} onClick={() => setConfirm({ title: 'Cancel this run?', description: 'New admissions and result retrieval will stop. Already admitted provider work may finish.', action: () => act(() => api.cancelToolRun(project, r), 'Cancellation confirmed.') })}>Cancel run</button></div></div>)}
      {receipts && <section><h2>Execution receipts</h2>{receipts.length === 0 && <p>No receipts yet.</p>}{receipts.map(r => <p key={r.id}>{r.action} · {r.outcome} · {r.id}</p>)}</section>}
      <ResourceForm title="Read operation" busy={busy} submit={f => act(async () => { const run = data.runs.find(r => r.id === val(f, 'run')); if (!run) throw new Error('Choose an active run.'); const value = await api.createToolOperation(project, run, { request_key: val(f, 'request_key'), kind: val(f, 'kind') as 'repository_tree' | 'read_file', arguments: val(f, 'path') ? { path: val(f, 'path') } : {} }); if (alive.current) setOperation({ run, value }); }, 'Operation queued.', false)}>
        <Field name="run" label="Active run">{select('run', options(data.runs.filter(r => r.state === 'active')))}</Field><Field name="kind" label="Read"><select name="kind"><option value="repository_tree">Repository tree</option><option value="read_file">UTF-8 file</option></select></Field><Field name="path" label="File path" required={false} placeholder="README.md" /><Field name="request_key" label="Operation request identifier" defaultValue={crypto.randomUUID()} />
      </ResourceForm>
      {operation && <div className="cz-card ks-platform-form"><h2>Operation {operation.value.operation_id.slice(0, 8)}</h2><p>{operation.value.status} · {operation.value.outcome || 'Waiting'}</p><button className="cz-btn" disabled={busy} onClick={() => void act(async () => { const value = await api.toolOperationStatus(project, operation.run, operation.value.operation_id); if (alive.current) setOperation({ ...operation, value }); }, 'Status updated.', false)}>Check status</button><button className="cz-btn" disabled={busy} onClick={() => void act(() => api.cancelToolOperation(project, operation.run, operation.value.operation_id), 'Cancellation confirmed.', false)}>Cancel operation</button></div>}
    </>}
    <ConfirmDialog open={confirm !== null} onOpenChange={open => { if (!open) setConfirm(null); }} title={confirm?.title || ''} description={confirm?.description || ''} confirmLabel="Confirm" onConfirm={() => { if (confirm) void confirm.action(); }} />
  </>;
}

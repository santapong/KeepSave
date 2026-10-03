import { request } from './client';
import type { ToolRunInput, ToolOperationInput, ToolCheck, MCPDelegations, ToolArtifacts, ToolProfiles, ToolPackages, ToolConnections, ToolBindings, ToolWorkloads, ToolGrants, ToolRuns, ToolRun, ToolReceipts, ToolPackage, ToolOperation } from './coreTypes';
const base = (project: string) => `/projects/${encodeURIComponent(project)}/tool-platform`;
const send = <T>(project: string, path: string, input: unknown, method = 'POST') => request<T>(base(project) + path, { method, body: JSON.stringify(input), cache: 'no-store', redirect: 'error' });
export async function toolWorkspace(project: string) {
  const [artifacts, profiles, packages, connections, bindings, workloads, grants, runs, delegations] = await Promise.all([
    request<ToolArtifacts>(base(project) + '/artifacts'), request<ToolProfiles>(base(project) + '/profiles'), request<ToolPackages>(base(project) + '/packages'), request<ToolConnections>(base(project) + '/connections'), request<ToolBindings>(base(project) + '/bindings'), request<ToolWorkloads>(base(project) + '/workloads'), request<ToolGrants>(base(project) + '/grants'), request<ToolRuns>(base(project) + '/runs'), request<MCPDelegations>('/account/delegations'),
  ]);
  return { ...artifacts, ...profiles, ...packages, ...connections, ...bindings, ...workloads, ...grants, ...runs, ...delegations };
}
export const createToolResource = (project: string, kind: string, input: unknown) => send<unknown>(project, '/' + kind, input);
export const approveToolResource = (project: string, kind: 'profiles' | 'packages', id: string, digest: string) => send<void>(project, `/${kind}/${encodeURIComponent(id)}/approve`, { digest });
export const revokeToolResource = (project: string, kind: string, id: string) => request<void>(base(project) + `/${kind}/${encodeURIComponent(id)}`, { method: 'DELETE', redirect: 'error' });
export const checkToolConnection = (project: string, id: string, binding: string) => send<ToolCheck>(project, `/connections/${encodeURIComponent(id)}/check`, { binding_id: binding, allow_external_read: true, allow_authorization_token_mint: true });
export const createToolRun = (project: string, input: ToolRunInput) => send<ToolRun>(project, '/runs', input);
const runPath = (run: ToolRun) => `/runs/${encodeURIComponent(run.id)}`;
const clientQuery = (run: ToolRun) => `?client_id=${encodeURIComponent(run.client_id)}`;
export const cancelToolRun = (project: string, run: ToolRun) => send<void>(project, runPath(run) + '/cancel' + clientQuery(run), {});
export const toolReceipts = (project: string, run: ToolRun) => request<ToolReceipts>(base(project) + runPath(run) + '/receipts' + clientQuery(run));
export const toolPackage = (project: string, id: string) => request<ToolPackage>(base(project) + `/packages/${encodeURIComponent(id)}`);
export const createToolOperation = (project: string, run: ToolRun, input: ToolOperationInput) => send<ToolOperation>(project, runPath(run) + '/operations' + clientQuery(run), input);
export const toolOperationStatus = (project: string, run: ToolRun, id: string) => request<ToolOperation>(base(project) + runPath(run) + `/operations/${encodeURIComponent(id)}` + clientQuery(run));
export const cancelToolOperation = (project: string, run: ToolRun, id: string) => send<void>(project, runPath(run) + `/operations/${encodeURIComponent(id)}/cancel` + clientQuery(run), {});

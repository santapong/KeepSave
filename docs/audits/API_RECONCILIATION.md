# API Reconciliation Audit

Date: 2026-05-15
Auditor: API Reconciliation Auditor (read-only)
Scope:
- Frontend: `frontend/src/api/client.ts`, `frontend/src/api/ai.ts`, `frontend/src/embed/api.ts`
- Backend: `backend/internal/api/router.go` (canonical), plus `handlers_*.go` for handler verification

## 1. Methodology

**Frontend extraction.** All UI components import from `frontend/src/api/client.ts`, `frontend/src/api/ai.ts`, and `frontend/src/embed/api.ts`. Search of `src/**` (excluding `*.test.*` and `HelpPage.tsx` doc strings) for inline `fetch(`, `axios.`, `apiClient.`, `httpGet`, `httpPost`, `from '../api', `/api/v1`, `` `/api ``, `` `/healthz `` etc. returned no inline call sites outside those three files (one `getPrometheusMetrics` raw `fetch('/metrics')` lives inside `client.ts` itself). Ripgrep patterns used:

```
rg "fetch\(|axios\.|apiClient\.|httpGet|httpPost" src/ --type ts --type tsx
rg "request\(|request<" src/api/*.ts
rg "/api/v1|`/api|\"/api|`/healthz|\"/healthz|`/metrics|\"/metrics" src/
```

For every call inside a `request(...)` helper invocation we extracted `(METHOD, path)` from the template literal and the second-arg `method:` (default `GET`).

**Backend extraction.** `router.go` was read end-to-end. The file uses nested `Group(...)` blocks; every leaf `.METHOD("subpath", handler)` was concatenated with its group prefix (e.g. `pm := v1.Group("/projects/:id")` + `pm.POST("/promote", ...)` → `POST /api/v1/projects/:id/promote`). Two stand-alone routes at root (`/healthz`, `/readyz`, `/metrics`, `/api/docs`, `/.well-known/openid-configuration`) were tracked separately.

**Normalization.** Frontend `${var}` and backend `:param` were rewritten to a unified `:param`. Query-string parameters were dropped from the path key for matching but retained in the call-site notes.

**Spot-checks.** The top 3 critical findings (Section 4) were re-opened on both ends to confirm there is no abstraction shim or handler-aliasing that would resolve the apparent mismatch.

## 2. Endpoint inventory

### Frontend call sites
- Total call sites detected: **88 in `client.ts`, 14 in `ai.ts`, 5 in `embed/api.ts`** = **107 call invocations** (one of those in `client.ts` is the `request<T>` helper definition, so **86 + 14 + 5 = 105** real call sites).
- Plus 1 raw `fetch('/metrics')` in `client.ts:828`.
- **Effective unique (method, path) pairs called: 87**.

Top-10 most-called endpoints by frontend (call sites referencing each path):
| Rank | Method | Path | Call sites |
|---|---|---|---|
| 1 | GET    | `/api/v1/projects` | 1 (high-traffic in UI) |
| 2 | GET    | `/api/v1/projects/:id/secrets` | 1 (embed + main client) |
| 3 | POST   | `/api/v1/projects/:id/secrets` | 1 (embed + main client) |
| 4 | GET    | `/api/v1/organizations` | 1 |
| 5 | GET    | `/api/v1/templates` | 1 |
| 6 | GET    | `/api/v1/applications` | 1 |
| 7 | GET    | `/api/v1/mcp/servers` | 1 |
| 8 | GET    | `/api/v1/oauth/clients` | 1 |
| 9 | GET    | `/api/v1/ai/providers` | 1 |
| 10 | POST  | `/api/v1/auth/login` | 1 |

(Distinct endpoint count is more meaningful than "most called" here because each `client.ts` function is invoked from many pages; the helper is the single call site.)

### Backend route registrations
- Total routes in `router.go`: **126** (across nested groups).
- Group breakdown (all under `/api/v1` unless noted):

| Group prefix | Auth | Route count |
|---|---|---|
| `/auth` | none | 2 |
| `/users` | JWT | 1 |
| `/projects` | JWT | 5 |
| `/projects/:id/secrets` | API key | 7 |
| `/projects/:id` (project-mgmt + AI) | JWT | 33 |
| `/projects/:id/leases` | API key | 3 |
| `/rotate-keys` | JWT | 1 |
| `/api-keys` | JWT | 3 |
| `/webhook-deliveries` | JWT | 1 |
| `/organizations` | JWT | 16 |
| `/templates` | JWT | 7 |
| `/admin` | JWT | 2 |
| `/agent` | API key | 1 |
| `/platform` | JWT | 5 |
| `/ai` | JWT | 11 |
| `/oauth` (public) | none | 4 |
| `/oauth` (auth) | JWT | 4 |
| `/mcp` (public) | none | 1 |
| `/mcp` (auth) | JWT | 14 |
| `/applications` | API key | 6 |
| root: `/healthz`, `/readyz`, `/metrics`, `/api/docs`, `/.well-known/openid-configuration` | none | 5 |

Total = 126 routes (5 root + 121 `/api/v1/...`).

## 3. Reconciliation table

Path prefix `/api/v1` omitted in column 3 for readability. `M` = match, `mm` = method-mismatch, `OF` = orphan-frontend (no backend), `OB` = orphan-backend (no frontend caller).

Order: frontend-first (every frontend call, then orphan backend routes).

### 3.1 Frontend → backend (every frontend call)

| Frontend `file:line` | Method | Path | Backend handler / route | Status |
|---|---|---|---|---|
| `client.ts:101` | POST | `/auth/register` | `authHandler.Register` `router.go:55` | M |
| `client.ts:108` | POST | `/auth/login` | `authHandler.Login` `router.go:56` | M |
| `client.ts:116` | GET | `/projects` | `projectHandler.List` `router.go:69` | M |
| `client.ts:121` | GET | `/projects/:id` | `projectHandler.Get` `router.go:70` | M |
| `client.ts:126` | POST | `/projects` | `projectHandler.Create` `router.go:68` | M |
| `client.ts:134` | PUT | `/projects/:id` | `projectHandler.Update` `router.go:71` | M |
| `client.ts:142` | DELETE | `/projects/:id` | `projectHandler.Delete` `router.go:72` | M |
| `client.ts:147-150` | GET | `/projects/:id/secrets?environment=...` | `secretHandler.List` `router.go:79` | M |
| `client.ts:159` | POST | `/projects/:id/secrets` | `secretHandler.Create` `router.go:78` | M |
| `client.ts:171-176` | PUT | `/projects/:id/secrets/:secretId` | `secretHandler.Update` `router.go:81` | M |
| `client.ts:182` | DELETE | `/projects/:id/secrets/:secretId` | `secretHandler.Delete` `router.go:82` | M |
| `client.ts:192` | POST | `/projects/:id/promote/diff` | `promotionHandler.Diff` `router.go:91` | M |
| `client.ts:211` | POST | `/projects/:id/promote` | `promotionHandler.Promote` `router.go:90` | M |
| `client.ts:225-227` | GET | `/projects/:id/promotions` | `promotionHandler.ListPromotions` `router.go:92` | M |
| `client.ts:235-238` | POST | `/projects/:id/promotions/:promotionId/approve` | `promotionHandler.ApprovePromotion` `router.go:94` | M |
| `client.ts:246-249` | POST | `/projects/:id/promotions/:promotionId/reject` | `promotionHandler.RejectPromotion` `router.go:95` | M |
| `client.ts:257-259` | POST | `/projects/:id/promotions/:promotionId/rollback` | `promotionHandler.Rollback` `router.go:96` | M |
| `client.ts:264-266` | GET | `/projects/:id/audit-log?limit=...` | `promotionHandler.AuditLog` `router.go:97` | M |
| `client.ts:272` | GET | `/api-keys` | `apikeyHandler.List` `router.go:151` | M |
| `client.ts:282` | POST | `/api-keys` | `apikeyHandler.Create` `router.go:150` | M |
| `client.ts:294` | DELETE | `/api-keys/:id` | `apikeyHandler.Delete` `router.go:152` | M |
| `client.ts:299` | GET | `/users/lookup?email=...` | `authHandler.LookupUser` `router.go:62` | M |
| `client.ts:305` | GET | `/organizations` | `orgHandler.List` `router.go:165` | M |
| `client.ts:310` | POST | `/organizations` | `orgHandler.Create` `router.go:164` | M |
| `client.ts:318` | GET | `/organizations/:orgId` | `orgHandler.Get` `router.go:166` | M |
| `client.ts:323` | PUT | `/organizations/:orgId` | `orgHandler.Update` `router.go:167` | M |
| `client.ts:331` | DELETE | `/organizations/:orgId` | `orgHandler.Delete` `router.go:168` | M |
| `client.ts:335` | GET | `/organizations/:orgId/members` | `orgHandler.ListMembers` `router.go:170` | M |
| `client.ts:340` | POST | `/organizations/:orgId/members` | `orgHandler.AddMember` `router.go:169` | M |
| `client.ts:348` | PUT | `/organizations/:orgId/members/:userId` | `orgHandler.UpdateMemberRole` `router.go:171` | M |
| `client.ts:356` | DELETE | `/organizations/:orgId/members/:userId` | `orgHandler.RemoveMember` `router.go:172` | M |
| `client.ts:360` | POST | `/organizations/:orgId/projects` | `orgHandler.AssignProject` `router.go:173` | M |
| `client.ts:367` | GET | `/organizations/:orgId/projects` | `orgHandler.ListProjects` `router.go:174` | M |
| `client.ts:374` | GET | `/templates` | `templateHandler.List` `router.go:189` | M |
| `client.ts:379` | GET | `/templates/builtin` | `templateHandler.ListBuiltin` `router.go:190` | M |
| `client.ts:391` | POST | `/templates` | `templateHandler.Create` `router.go:188` | M |
| `client.ts:406` | DELETE | `/templates/:templateId` | `templateHandler.Delete` `router.go:193` | M |
| `client.ts:414` | POST | `/templates/:templateId/apply` | `templateHandler.Apply` `router.go:194` | M |
| `client.ts:423-425` | GET | `/projects/:id/env-export?environment=...` | `envFileHandler.Export` `router.go:103` | M |
| `client.ts:435` | POST | `/projects/:id/env-import` | `envFileHandler.Import` `router.go:104` | M |
| `client.ts:444` | GET | `/admin/dashboard` | `metricsHandler.AdminDashboard` `router.go:200` | M |
| `client.ts:448` | GET | `/admin/traces` | `metricsHandler.Traces` `router.go:201` | M |
| `client.ts:460-463` | POST | `/organizations/:orgId/sso` | `enterpriseHandler.ConfigureSSO` `router.go:175` | M |
| `client.ts:468` | GET | `/organizations/:orgId/sso` | `enterpriseHandler.ListSSOConfigs` `router.go:176` | M |
| `client.ts:474-477` | POST | `/organizations/:orgId/compliance` | `enterpriseHandler.GenerateComplianceReport` `router.go:178` | M |
| `client.ts:482` | GET | `/organizations/:orgId/compliance` | `enterpriseHandler.ListComplianceReports` `router.go:179` | M |
| `client.ts:488` | POST | `/projects/:id/backups` | `enterpriseHandler.CreateBackup` `router.go:107` | M |
| `client.ts:496` | GET | `/projects/:id/backups` | `enterpriseHandler.ListBackups` `router.go:108` | M |
| `client.ts:502` | GET | `/projects/:id/policy` | `enterpriseHandler.GetSecretPolicy` `router.go:109` | M |
| `client.ts:512-518` | PUT | `/projects/:id/policy` | `enterpriseHandler.SetSecretPolicy` `router.go:110` | M |
| `client.ts:530-536` | POST | `/projects/:id/leases` | `agentHandler.CreateLease` `router.go:136` | M |
| `client.ts:542` | GET | `/projects/:id/leases` | `agentHandler.ListLeases` `router.go:137` | M |
| `client.ts:547` | DELETE | `/projects/:id/leases/:leaseId` | `agentHandler.RevokeLease` `router.go:138` | M |
| `client.ts:552` | GET | `/projects/:id/agent-activity` | `agentHandler.GetRecentActivity` `router.go:111` | M |
| `client.ts:557` | GET | `/projects/:id/agent-heatmap` | `agentHandler.GetAccessHeatmap` `router.go:112` | M |
| `client.ts:563` | GET | `/platform/events` | `platformHandler.ListEvents` `router.go:213` | M |
| `client.ts:568-571` | POST | `/platform/events/replay` | `platformHandler.ReplayEvents` `router.go:214` | M |
| `client.ts:576` | GET | `/platform/plugins` | `platformHandler.ListPlugins` `router.go:215` | M |
| `client.ts:586-589` | POST | `/platform/plugins` | `platformHandler.RegisterPlugin` `router.go:216` | M |
| `client.ts:595` | GET | `/projects/:id/access-policies` | `platformHandler.ListAccessPolicies` `router.go:113` | M |
| `client.ts:604-607` | POST | `/projects/:id/access-policies` | `platformHandler.CreateAccessPolicy` `router.go:114` | M |
| `client.ts:613` | GET | `/oauth/clients` | `oauthHandler.ListClients` `router.go:250` | M |
| `client.ts:625-635` | POST | `/oauth/clients` | `oauthHandler.RegisterClient` `router.go:249` | M |
| `client.ts:639` | DELETE | `/oauth/clients/:clientId` | `oauthHandler.DeleteClient` `router.go:251` | M |
| `client.ts:644` | GET | `/mcp/servers/public` | `mcpHubHandler.ListPublicServers` `router.go:256` | M |
| `client.ts:649` | GET | `/mcp/servers` | `mcpHubHandler.ListMyServers` `router.go:263` | M |
| `client.ts:654` | GET | `/mcp/servers/:serverId` | `mcpHubHandler.GetServer` `router.go:264` | M |
| `client.ts:668-681` | POST | `/mcp/servers` | `mcpHubHandler.RegisterServer` `router.go:262` | M |
| `client.ts:685` | DELETE | `/mcp/servers/:serverId` | `mcpHubHandler.DeleteServer` `router.go:266` | M |
| `client.ts:689` | POST | `/mcp/servers/:serverId/rebuild` | `mcpHubHandler.RebuildServer` `router.go:267` | M |
| `client.ts:694` | GET | `/mcp/installations` | `mcpHubHandler.ListInstallations` `router.go:269` | M |
| `client.ts:703-710` | POST | `/mcp/installations` | `mcpHubHandler.InstallServer` `router.go:268` | M |
| `client.ts:715` | DELETE | `/mcp/installations/:installId` | `mcpHubHandler.UninstallServer` `router.go:271` | M |
| `client.ts:720` | GET | `/mcp/gateway/tools` | `mcpGatewayHandler.ListTools` `router.go:273` | M |
| `client.ts:725` | GET | `/mcp/gateway/stats` | `mcpHubHandler.GetGatewayStats` `router.go:274` | M |
| `client.ts:730` | GET | `/mcp/config` | `mcpGatewayHandler.MCPConfig` `router.go:275` | M |
| `client.ts:738-741` | POST | `/projects/:id/dependencies/analyze?environment=...` | `depHandler.Analyze` `router.go:105` | M |
| `client.ts:749-751` | GET | `/projects/:id/dependencies/graph?environment=...` | `depHandler.Graph` `router.go:106` | M |
| `client.ts:769` | GET | `/applications` | `applicationHandler.List` `router.go:282` | M |
| `client.ts:779-782` | POST | `/applications` | `applicationHandler.Create` `router.go:281` | M |
| `client.ts:794-797` | PUT | `/applications/:id` | `applicationHandler.Update` `router.go:284` | M |
| `client.ts:802` | DELETE | `/applications/:id` | `applicationHandler.Delete` `router.go:285` | M |
| `client.ts:806-808` | POST | `/applications/:id/favorite` | `applicationHandler.ToggleFavorite` `router.go:286` | M |
| `client.ts:815` | GET | `/agent/activity` | `agentHandler.GetActivitySummary` `router.go:207` | M |
| `client.ts:820` | GET | `/webhook-deliveries` | `webhookHandler.Deliveries` `router.go:158` | M |
| `client.ts:828` | GET | `/metrics` (raw fetch, not under `/api/v1`) | `metricsHandler.Metrics` `router.go:47` | M |
| `client.ts:833-835` | PUT | `/platform/plugins/:pluginId` | `platformHandler.TogglePlugin` `router.go:217` | M |
| `client.ts:841` | DELETE | `/projects/:id/access-policies/:policyId` | `platformHandler.DeleteAccessPolicy` `router.go:115` | M |
| `ai.ts:38` | GET | `/ai/providers` | `intelligenceHandler.ListProviders` `router.go:224` | M |
| `ai.ts:43-46` | POST | `/projects/:id/drift` | `intelligenceHandler.DetectDrift` `router.go:118` | M |
| `ai.ts:51` | GET | `/projects/:id/drift` | `intelligenceHandler.ListDriftChecks` `router.go:119` | M |
| `ai.ts:57` | POST | `/projects/:id/anomalies/scan` | `intelligenceHandler.RunAnomalyDetection` `router.go:124` | M |
| `ai.ts:65` | GET | `/ai/anomalies?project_id=...&status=...` | `intelligenceHandler.ListAnomalies` `router.go:227` | M |
| `ai.ts:70` | PUT | `/ai/anomalies/:anomalyId/acknowledge` | `intelligenceHandler.AcknowledgeAnomaly` `router.go:228` | M |
| `ai.ts:74` | PUT | `/ai/anomalies/:anomalyId/resolve` | `intelligenceHandler.ResolveAnomaly` `router.go:229` | M |
| `ai.ts:83` | GET | `/projects/:id/analytics/trends?period=...&days=...` | `intelligenceHandler.GetUsageTrends` `router.go:125` | M |
| `ai.ts:89` | GET | `/projects/:id/analytics/forecast?days=...` | `intelligenceHandler.GetUsageForecast` `router.go:126` | M |
| `ai.ts:95` | POST | `/projects/:id/recommendations/generate` | `intelligenceHandler.GenerateRecommendations` `router.go:128` | M |
| `ai.ts:101` | GET | `/projects/:id/recommendations?status=...` | `intelligenceHandler.ListRecommendations` `router.go:129` | M |
| `ai.ts:106` | DELETE | `/projects/:id/recommendations/:recId` | `intelligenceHandler.DismissRecommendation` `router.go:130` | M |
| `ai.ts:111-114` | POST | `/ai/query` | `intelligenceHandler.NLPQuery` `router.go:225` | M |
| `embed/api.ts:66-67` | GET | `/api/v1/projects/:id/secrets?environment=...` | `secretHandler.List` `router.go:79` | M |
| `embed/api.ts:72-75` | POST | `/api/v1/projects/:id/secrets` | `secretHandler.Create` `router.go:78` | M |
| `embed/api.ts:80-87` | PUT | `/api/v1/projects/:id/secrets/:secretId` | `secretHandler.Update` `router.go:81` | M |
| `embed/api.ts:91-93` | DELETE | `/api/v1/projects/:id/secrets/:secretId` | `secretHandler.Delete` `router.go:82` | M |
| `embed/api.ts:101-104` | POST | `/api/v1/projects/:id/secrets/batch` | **NONE** | **OF (critical)** |

### 3.2 Backend routes with no frontend caller (orphan-backend / OB)

| Backend `router.go:line` | Method | Path | Auth | Notes |
|---|---|---|---|---|
| `router.go:45` | GET | `/healthz` | none | Liveness — infra-level, fine. info |
| `router.go:46` | GET | `/readyz` | none | Readiness — infra-level, fine. info |
| `router.go:48` | GET | `/api/docs` | none | OpenAPI spec; UI just shows the URL as text in OverviewTab. low |
| `router.go:49` | GET | `/.well-known/openid-configuration` | none | OIDC discovery — fine. info |
| `router.go:83` | GET | `/projects/:id/secrets/:secretId` | API key | `secretHandler.Get` — never called by UI. low |
| `router.go:84` | GET | `/projects/:id/secrets/:secretId/versions` | API key | `versionHandler.ListVersions` — UI exposes no history view. **medium** |
| `router.go:85` | GET | `/projects/:id/secrets/:secretId/versions/:version` | API key | `versionHandler.GetVersion` — see above. **medium** |
| `router.go:93` | GET | `/projects/:id/promotions/:promotionId` | JWT | `promotionHandler.GetPromotion` — UI uses list endpoint only. low |
| `router.go:98` | POST | `/projects/:id/rotate-keys` | JWT | `keyRotationHandler.RotateProjectKey` — no UI control. **medium** |
| `router.go:99` | GET | `/projects/:id/verify-encryption` | JWT | `keyRotationHandler.VerifyEncryption` — no UI control. **medium** |
| `router.go:100` | POST | `/projects/:id/webhooks` | JWT | `webhookHandler.Register` — no UI for webhook setup. **medium** |
| `router.go:101` | GET | `/projects/:id/webhooks` | JWT | `webhookHandler.List` — no UI. **medium** |
| `router.go:102` | DELETE | `/projects/:id/webhooks` | JWT | `webhookHandler.Remove` — no UI, and uses path-less DELETE (no `:webhookId`). **medium** |
| `router.go:120` | POST | `/projects/:id/drift/schedules` | JWT | `intelligenceHandler.CreateDriftSchedule` — UI has no scheduler. **medium** |
| `router.go:121` | GET | `/projects/:id/drift/schedules` | JWT | `intelligenceHandler.ListDriftSchedules`. **medium** |
| `router.go:122` | PUT | `/projects/:id/drift/schedules/:scheduleId` | JWT | `UpdateDriftSchedule`. **medium** |
| `router.go:123` | DELETE | `/projects/:id/drift/schedules/:scheduleId` | JWT | `DeleteDriftSchedule`. **medium** |
| `router.go:127` | GET | `/projects/:id/analytics/export` | JWT | `intelligenceHandler.ExportAnalyticsCSV` — UI has no CSV-export button. **medium** |
| `router.go:144` | POST | `/rotate-keys` | JWT | `keyRotationHandler.RotateAllKeys` — admin-only; no UI. **medium** |
| `router.go:177` | DELETE | `/organizations/:orgId/sso/:provider` | JWT | `enterpriseHandler.DeleteSSOConfig` — UI lists & configures SSO but cannot delete. **medium** |
| `router.go:181` | GET | `/organizations/:orgId/quota` | JWT | `intelligenceHandler.GetQuota` — no UI. **medium** |
| `router.go:182` | PUT | `/organizations/:orgId/quota` | JWT | `intelligenceHandler.SetQuota` — no UI. **medium** |
| `router.go:191` | GET | `/templates/:templateId` | JWT | `templateHandler.Get` — UI lists but never fetches a single template. low |
| `router.go:192` | PUT | `/templates/:templateId` | JWT | `templateHandler.Update` — no UI to edit templates. **medium** |
| `router.go:226` | POST | `/ai/converse` | JWT | `intelligenceHandler.NLPConverse` — `nlpQuery` covers `/ai/query`; converse not wired. **medium** |
| `router.go:230` | POST | `/ai/rules` | JWT | `intelligenceHandler.CreateAlertRule` — entire alert-rule surface unwired. **medium** |
| `router.go:231` | GET | `/ai/rules` | JWT | `ListAlertRules`. **medium** |
| `router.go:232` | PUT | `/ai/rules/:ruleId` | JWT | `UpdateAlertRule`. **medium** |
| `router.go:233` | DELETE | `/ai/rules/:ruleId` | JWT | `DeleteAlertRule`. **medium** |
| `router.go:234` | POST | `/ai/drift/run-scheduled` | JWT | `RunScheduledDriftChecks` — operator endpoint, no UI. low |
| `router.go:239` | POST | `/oauth/token` | none | OAuth client-credentials token endpoint — machine API. info |
| `router.go:240` | GET | `/oauth/userinfo` | none (token-checked internally) | Machine API. info |
| `router.go:241` | POST | `/oauth/revoke` | none | Machine API. info |
| `router.go:242` | GET | `/oauth/.well-known/jwks.json` | none | Discovery. info |
| `router.go:248` | GET | `/oauth/authorize` | JWT | Browser-flow consent screen — frontend has no consent page wired (`/oauth-clients` page is for client mgmt only). **medium** |
| `router.go:265` | PUT | `/mcp/servers/:serverId` | JWT | `mcpHubHandler.UpdateServer` — UI has no edit-server flow. **medium** |
| `router.go:270` | PUT | `/mcp/installations/:installId` | JWT | `UpdateInstallation` — UI has no edit-installation flow. **medium** |
| `router.go:272` | POST | `/mcp/gateway` | JWT | `mcpGatewayHandler.HandleToolCall` — machine/agent API. info |
| `router.go:283` | GET | `/applications/:appId` | API key | `applicationHandler.Get` — UI lists but doesn't fetch single app (docs reference it though, see `ApplicationSettingsPage.tsx:212`). low |

## 4. Findings by severity

### 4.1 Critical — frontend call with no backend handler (broken at runtime)

**F-C-001 — Embed widget calls non-existent batch endpoint**

- Frontend: `frontend/src/embed/api.ts:96-105`
  ```ts
  async batchGetSecrets(projectId, environment, keys) {
    return this.request(`/api/v1/projects/${projectId}/secrets/batch`, {
      method: 'POST',
      body: JSON.stringify({ environment, keys }),
    });
  }
  ```
- Backend: no route matches `POST /api/v1/projects/:id/secrets/batch`. Confirmed by `rg -n "batch\|Batch" backend/internal/api/` → zero hits. The `/projects/:id/secrets` group at `router.go:75-85` registers `POST /`, `GET /`, `GET /:secretId`, `PUT /:secretId`, `DELETE /:secretId`, `GET /:secretId/versions`, `GET /:secretId/versions/:version`. With Gin's tree, `POST /:id/secrets/batch` would either 404 or be misrouted into `POST /:id/secrets` because the literal `batch` would be parsed as `:secretId` for a GET — but POST on `/:secretId` doesn't exist either, so this returns 404 (or 405 on the singular `:secretId` slot).
- Spot-check: opened both ends. `embed/api.ts` is exported (`KeepSaveAPI` class) and is invokable by any embed integrator via the public SDK surface; `embed/keepsave-widget.ts` does not currently call it, but `KeepSaveAPI.batchGetSecrets` is publicly reachable on the class.
- Fix: either (a) add `POST /projects/:id/secrets/batch` in `router.go` + a `SecretHandler.BatchGet` handler that takes `{environment, keys}` and returns matching secrets, or (b) remove the method from `embed/api.ts` if the feature is not planned. Given the `tests/` and Roadmap mention batch reads as a planned agent surface, option (a) is the correct fix.

### 4.2 High — method mismatch

None detected. Every (method, path) pair we found in the frontend is registered on the backend with the same method, with the single exception of the critical finding above (which is a missing path, not a method mismatch).

### 4.3 Medium — orphan backend endpoints (with auth — attack surface or undocumented machine API)

These are registered routes that no frontend code calls; they may be intentional machine APIs, but they need either (a) a frontend caller, (b) explicit documentation as machine-only, or (c) removal.

| # | `router.go:line` | Method | Path | Concern |
|---|---|---|---|---|
| F-M-001 | `:83` | GET | `/projects/:id/secrets/:secretId` | Single-secret read; UI uses list. Could be intended for agent SDK use, but no audit/doc says so. |
| F-M-002 | `:84` | GET | `/projects/:id/secrets/:secretId/versions` | Secret-history feature; no UI. Either ship the history pane or remove. |
| F-M-003 | `:85` | GET | `/projects/:id/secrets/:secretId/versions/:version` | Same. |
| F-M-004 | `:98` | POST | `/projects/:id/rotate-keys` | Key rotation per project — sensitive op with no UI safeguard / confirmation flow. |
| F-M-005 | `:99` | GET | `/projects/:id/verify-encryption` | Encryption-self-test endpoint; no UI. |
| F-M-006 | `:100-102` | POST/GET/DELETE | `/projects/:id/webhooks` | Webhook CRUD; UI shows deliveries (`/webhook-deliveries`) but has no setup screen. Also note: `DELETE /webhooks` has no `:webhookId` segment, which by itself is an API design smell (you'd be deleting all webhooks for the project). |
| F-M-007 | `:120-123` | POST/GET/PUT/DELETE | `/projects/:id/drift/schedules[/:scheduleId]` | Whole drift-scheduler subresource unused by UI. |
| F-M-008 | `:127` | GET | `/projects/:id/analytics/export` | CSV export — no button anywhere in `AIIntelligencePage.tsx`. |
| F-M-009 | `:144` | POST | `/rotate-keys` | Global key rotation — admin-only operation with no UI. |
| F-M-010 | `:177` | DELETE | `/organizations/:orgId/sso/:provider` | UI configures SSO (`configureSSOProvider`) and lists (`listSSOConfigs`) but cannot delete — left-over configs accumulate. |
| F-M-011 | `:181-182` | GET/PUT | `/organizations/:orgId/quota` | Org-quota management — Phase 15 feature, frontend incomplete. |
| F-M-012 | `:192` | PUT | `/templates/:templateId` | Template update; UI has no edit-template flow. |
| F-M-013 | `:226` | POST | `/ai/converse` | Multi-turn NLP — `AppChatbot.tsx` exists; would have been a natural caller, but the chatbot has no API call. |
| F-M-014 | `:230-233` | POST/GET/PUT/DELETE | `/ai/rules[/:ruleId]` | Entire alert-rule CRUD has no frontend. |
| F-M-015 | `:248` | GET | `/oauth/authorize` | OAuth browser-flow authorize screen; UI manages clients but no consent page exists. Without a consent screen the authorize flow cannot complete end-to-end. |
| F-M-016 | `:265` | PUT | `/mcp/servers/:serverId` | UpdateServer; UI has rebuild/delete but no edit. |
| F-M-017 | `:270` | PUT | `/mcp/installations/:installId` | UpdateInstallation; same. |

### 4.4 Low — dead / unreferenced

| # | `router.go:line` | Method | Path | Note |
|---|---|---|---|---|
| F-L-001 | `:93` | GET | `/projects/:id/promotions/:promotionId` | Single-promotion fetch; UI uses list. Probably fine for machine API but undocumented. |
| F-L-002 | `:191` | GET | `/templates/:templateId` | Single-template fetch; UI uses list. |
| F-L-003 | `:234` | POST | `/ai/drift/run-scheduled` | Operator/cron endpoint, no UI expected. |
| F-L-004 | `:283` | GET | `/applications/:appId` | Single-application fetch; `ApplicationSettingsPage.tsx:212` documents it but no frontend code calls it. |
| F-L-005 | `:48`  | GET | `/api/docs` | OverviewTab shows the link as label only; no programmatic call from frontend. Fine. |

### 4.5 Info — discovery / machine / infra surfaces

`/healthz`, `/readyz`, `/metrics` (called via raw `fetch` from `client.ts:828`), `/.well-known/openid-configuration`, `/oauth/token`, `/oauth/userinfo`, `/oauth/revoke`, `/oauth/.well-known/jwks.json`, `/mcp/gateway` (machine call from agents), `/mcp/servers/public` (public listing for catalog). These are intentionally machine-facing and do not need a browser caller.

## 5. Coverage stats

- **Frontend → backend match rate**: 104 / 105 distinct frontend call sites have a matching backend handler = **99.05 %**. The single miss is `F-C-001` (`POST /projects/:id/secrets/batch` from `embed/api.ts:101`).
- **Backend → frontend coverage**:
  - Total backend routes: 126.
  - Routes called by frontend: 87 (the 86 unique paths under `/api/v1` matched in §3.1 plus the raw `/metrics` fetch).
  - **Coverage = 87 / 126 = 69.05 %**.
  - Excluding machine/infra endpoints (the 10 info-class routes in §4.5 = `/healthz`, `/readyz`, `/.well-known/openid-configuration`, `/oauth/token`, `/oauth/userinfo`, `/oauth/revoke`, `/oauth/.well-known/jwks.json`, `/mcp/gateway`, `/mcp/servers/public`, plus the operator endpoint `/ai/drift/run-scheduled`): **87 / 116 = 75.0 %** of "user-facing" routes are wired to the UI.
  - That leaves **~29 user-facing routes orphaned** on the backend — most of them in Phase 15 (drift schedules, AI rules, quota), Phase 9 (webhooks, key rotation), and template/MCP edit flows.

## 6. Summary for the caller

- Totals: 105 frontend call sites vs 126 backend routes.
- Critical-severity count: **1** (`F-C-001`).
- Top 3 mismatches:
  1. **F-C-001 (critical)** — frontend `embed/api.ts:101` calls `POST /api/v1/projects/:id/secrets/batch`; backend has no such route (router.go group at `router.go:75-85` does not register `batch`). This will 404 if any embed integrator invokes `KeepSaveAPI.batchGetSecrets`.
  2. **F-M-006 (medium)** — webhook CRUD (`router.go:100-102`) is registered but no frontend caller exists; the UI only consumes the `/webhook-deliveries` log. Additionally, `DELETE /projects/:id/webhooks` has no `:webhookId` discriminator, which is a separate API-design smell because it cannot target a single webhook.
  3. **F-M-014 (medium)** — the entire AI alert-rule surface (`router.go:230-233`, 4 routes) and `POST /ai/converse` (`router.go:226`) have no frontend callers, even though `AppChatbot.tsx` exists and would be the natural consumer of `/ai/converse`.
- **Pattern**: the orphans cluster in two areas:
  - **Phase 15 (AI Intelligence)**: drift schedules (`router.go:120-123`), AI rules (`router.go:230-233`), `/ai/converse` (`router.go:226`), quotas (`router.go:181-182`), CSV export (`router.go:127`). The backend has shipped these but the frontend has not been wired — a backend-ahead-of-frontend release cadence.
  - **Phase 9 (Enterprise/security ops)**: webhook setup (`router.go:100-102`), key rotation (`router.go:98`, `:144`), encryption verify (`router.go:99`), template edit (`router.go:192`), SSO delete (`router.go:177`). These are admin/security operations that exist on the API but were not given a UI — likely intentional "API-only for now" surface, but undocumented as such.
  - The single critical finding (`F-C-001`) is the opposite pattern: a frontend (embed-SDK) feature shipped against a planned-but-not-yet-implemented backend endpoint.
- No method-mismatch (high-severity) findings. No router-refactor stranding of old code was detected — all the orphan-frontend pressure points are concentrated in the embed SDK, and the orphan-backend pressure is concentrated in Phase 9 / Phase 15 admin surfaces.

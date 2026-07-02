# Audit-Log Coverage Spec (Backend 30-day)

The 30-day audit revealed that **secret, project, and API-key mutations are not audit-logged today**. This document is the spec to close that gap. It enumerates every state-mutating handler, the audit event it MUST emit, and the test that asserts the audit row exists.

This is Backend Engineer 30-day work item §1 from `docs/ROLES_30_60_90.md`. Security Engineer review required (per ROLES.md §2.2).

---

## Principle

> If an action is not in the audit log, it didn't happen — and the feature isn't done. (ROLES.md §1, operating principle 5.)

This is non-negotiable. A PR adding or modifying a state-mutating handler must include:
1. An audit emit call in the service or handler, with the canonical event name and metadata fields below.
2. A test that asserts the audit row was created.

Reviewers reject PRs that fail either.

## Canonical event taxonomy

| Action            | Event name                | Required metadata fields                                                       |
|-------------------|---------------------------|--------------------------------------------------------------------------------|
| Secret create     | `secret.created`          | `project_id`, `environment`, `secret_key`, `actor_id`                          |
| Secret update     | `secret.updated`          | `project_id`, `environment`, `secret_key`, `actor_id`, `previous_version_id`    |
| Secret delete     | `secret.deleted`          | `project_id`, `environment`, `secret_key`, `actor_id`                          |
| Secret read (sensitive) | `secret.read`        | `project_id`, `environment`, `secret_key`, `actor_id`, `auth_method` (jwt/apikey) |
| Project create    | `project.created`         | `project_id`, `actor_id`, `name`                                                |
| Project update    | `project.updated`         | `project_id`, `actor_id`, `changed_fields[]`                                    |
| Project delete    | `project.deleted`         | `project_id`, `actor_id`, `name`                                                |
| API key create    | `apikey.created`          | `project_id`, `actor_id`, `key_id`, `scopes[]`, `environment` (nullable)         |
| API key delete    | `apikey.deleted`          | `project_id`, `actor_id`, `key_id`                                              |
| Promotion request | `promotion_requested`     | (already present) `project_id`, `actor_id`, `source_env`, `target_env`, `keys[]` |
| Promotion approve | `promotion_approved`      | **new** — `project_id`, `approver_id`, `promotion_id`                            |
| Promotion execute | `promotion_completed`     | (present) `promoted_keys[]`, `skipped_keys[]`, `override_policy`                  |
| Promotion reject  | `promotion_rejected`      | (present) `project_id`, `approver_id`, `promotion_id`, `reason`                  |
| Promotion rollback| `promotion_rollback`      | (present) `restored_keys[]`                                                      |
| Auth login        | `auth.login`              | `user_id`, `ip`, `success` (bool)                                                |
| Auth login failed | `auth.login_failed`       | `email_attempted`, `ip` — **no user_id** (the email may not be a real user)      |
| Auth logout       | `auth.logout`             | `user_id`                                                                        |
| Key rotation (master) | `key.master_rotated`  | `actor_id`, `provider`                                                          |
| Key rotation (DEK)| `key.dek_rotated`         | `project_id`, `actor_id`, `secrets_rotated`, `environments` — emitted by `keyrotation_service.go::RotateProjectKey` (one row per project; bulk rotation emits per-project rows) |
| Origin allow-list update | `embed.origins_updated` | `project_id`, `actor_id`, `added[]`, `removed[]` — once allow-list lands       |

### A-02 sweep — previously-uncovered service mutations (added 2026-06)

These 13 service groups performed state mutations but emitted no audit event.
The A-02 sweep wired `auditRepo` into each and now emits one row per successful
mutation. All events use the `entity.action` form. `actor_id` is the audit row's
`user_id`; `ip` is `ip_address`. Secret values, client secrets and SSO secrets
are NEVER placed in `details`.

| Action                  | Event name                  | Required metadata fields                                  |
|-------------------------|-----------------------------|-----------------------------------------------------------|
| Org create              | `org.created`               | `actor_id`, `organization_id`, `name`                     |
| Org update              | `org.updated`               | `actor_id`, `organization_id`, `name`                     |
| Org delete              | `org.deleted`               | `actor_id`, `organization_id`, `name`                     |
| Org member add          | `org.member_added`          | `actor_id`, `organization_id`, `target_user_id`, `role`   |
| Org member role update  | `org.member_role_updated`   | `actor_id`, `organization_id`, `target_user_id`, `role`   |
| Org member remove       | `org.member_removed`        | `actor_id`, `organization_id`, `target_user_id`           |
| Webhook register        | `webhook.registered`        | `actor_id`, `project_id`, `url`, `events`                 |
| Webhook remove          | `webhook.removed`           | `actor_id`, `project_id`, `removed_count`                 |
| Template create         | `template.created`          | `actor_id`, `template_id`, `name`                         |
| Template update         | `template.updated`          | `actor_id`, `template_id`, `name`                         |
| Template delete         | `template.deleted`          | `actor_id`, `template_id`                                 |
| Env-file import         | `envfile.imported`          | `actor_id`, `project_id`, `environment`, `created_count`, `updated_count`, `skipped_count` |
| SSO configure           | `sso.configured`            | `actor_id`, `organization_id`, `provider`                 |
| SSO delete              | `sso.deleted`               | `actor_id`, `organization_id`, `provider`                 |
| Compliance report       | `compliance.generated`      | `actor_id`, `organization_id`, `report_id`, `report_type` |
| Backup create           | `backup.created`            | `actor_id`, `project_id`, `backup_id`, `type`, `secret_count` |
| Secret policy set        | `policy.set`                | `actor_id`, `project_id`, `max_age_days`, `require_rotation` |
| Lease create            | `lease.created`             | `actor_id` (api key id), `project_id`, `environment`, `lease_id`, `secret_keys` |
| Lease revoke            | `lease.revoked`             | `actor_id`, `lease_id` (no `project_id` — UPDATE is keyed by lease id alone) |
| Agent token mint        | `agent.token.minted`        | `actor_id`, `project_id`, `lease_id`, `jti`, `expires_at` (ADR-0021) |
| Agent token revoke      | `agent.token.revoked`       | `actor_id`, `project_id`, `jti` (ADR-0021) |
| MCP server register     | `mcp.server_registered`     | `actor_id`, `mcp_server_id`, `name`                       |
| MCP server update       | `mcp.server_updated`        | `actor_id`, `mcp_server_id`, `name`                       |
| MCP server delete       | `mcp.server_deleted`        | `actor_id`, `mcp_server_id`                               |
| MCP server install      | `mcp.server_installed`      | `actor_id`, `mcp_server_id`, `installation_id`, `project_id` (nullable) |
| MCP installation update | `mcp.installation_updated`  | `actor_id`, `installation_id`, `enabled`                  |
| MCP installation uninstall | `mcp.installation_deleted` | `actor_id`, `installation_id` |
| OAuth client register   | `oauth.client_registered`   | `actor_id`, `oauth_client_id`, `client_id`, `name`        |
| OAuth client delete     | `oauth.client_deleted`      | `actor_id`, `oauth_client_id`                             |
| Application create      | `application.created`       | `actor_id`, `application_id`, `name`                      |
| Application update      | `application.updated`       | `actor_id`, `application_id`, `name`                      |
| Application delete      | `application.deleted`       | `actor_id`, `application_id`                              |
| Anomaly rule create     | `anomaly.rule_created`      | `actor_id`, `rule_id`, `rule_type`, `project_id` (nullable) |
| Anomaly rule update     | `anomaly.rule_updated`      | `actor_id`, `rule_id`, `enabled`                          |
| Anomaly rule delete     | `anomaly.rule_deleted`      | `actor_id`, `rule_id`                                     |
| Anomaly acknowledge     | `anomaly.acknowledged`      | `actor_id`, `anomaly_id`                                  |
| Anomaly resolve         | `anomaly.resolved`          | `actor_id`, `anomaly_id`                                  |
| Feedback submit         | `feedback.submitted`        | `actor_id`, `category`, `issue_number`, `repo` — the feedback message text is NEVER placed in `details` |

### Naming convention

Event names use the `entity.action` form: a dotted `entity` and a snake_case
sub-action (e.g. `key.dek_rotated`, `auth.login_failed`, `org.member_added`).
The five `promotion_*` events (`promotion_requested`, `promotion_approved`,
`promotion_completed`, `promotion_rejected`, `promotion_rollback`) predate this
convention and use a flat `promotion_<action>` form; they are **legacy** and are
kept as-is for compatibility. New events MUST use `entity.action`.

Events are currently emitted as string literals via the `emitAudit` helper in
`backend/internal/service/audit_helper.go` (e.g.
`emitAudit(s.auditRepo, &actor, &project, "org.created", env, details, ip)`).
A shared `backend/internal/events/events.go` constants file is a tracked
follow-up; until it lands, the canonical spelling is the one in the tables above.

## Implementation pattern

Every state-mutating service gets an `auditRepo` dependency:

```go
type SecretService struct {
    repo      SecretRepo
    cryptoSvc *crypto.Service
    auditRepo AuditRepo  // ← new
}

func (s *SecretService) Create(ctx context.Context, in CreateSecretInput) (*Secret, error) {
    // … existing logic
    if _, err := s.auditRepo.Create(ctx, &AuditEntry{
        Action:    events.SecretCreated,
        ProjectID: in.ProjectID,
        ActorID:   in.ActorID,
        Metadata: map[string]any{
            "environment": in.Environment,
            "secret_key":  in.Key,
        },
    }); err != nil {
        // log but do not fail the request — audit failures get their own alert
        s.logger.Error("audit emit failed", "event", events.SecretCreated, "err", err)
    }
    return secret, nil
}
```

**Why audit-emit failures don't fail the request:** if Postgres is healthy enough to commit the secret mutation, audit emit on the same DB should also succeed. If audit emit fails, that's its own operational signal (likely a schema or migration issue, not a request issue). We do NOT want a working write path blocked by a degraded audit table.

However, **a failed audit emit MUST raise a metric** (`keepsave_audit_emit_failed_total{event}`) which is alertable. See DevOps work item.

## Test obligations

For each event in the taxonomy above, at least one test:

1. **Handler-level integration test** that exercises the endpoint and reads the `audit_log` table to confirm a matching row exists. Example shape:

```go
func TestCreateSecret_EmitsAudit(t *testing.T) {
    db := testdb.New(t)
    svc := buildSecretService(db, …)
    h := api.NewSecretHandler(svc, …)

    req := newAuthedRequest(t, "POST", "/api/v1/projects/"+pid+"/secrets",
        `{"key":"DATABASE_URL","value":"postgres://...","environment":"alpha"}`)
    w := httptest.NewRecorder()
    h.Create(w, req)

    require.Equal(t, 201, w.Code)
    rows := testdb.AuditRowsFor(t, db, pid)
    require.Len(t, rows, 1)
    require.Equal(t, events.SecretCreated, rows[0].Action)
    require.Equal(t, "DATABASE_URL", rows[0].Metadata["secret_key"])
    require.Equal(t, "alpha", rows[0].Metadata["environment"])
}
```

2. **Service-level unit test with mock `auditRepo`** asserting the call shape (action, fields). Cheaper than the integration test; runs on every commit. Both layers are required — the unit test catches refactor breakage, the integration test catches plumbing breakage.

## Order of implementation

1. **Secret service** — highest value, highest exposure. (Days 1-7.)
2. **Project service.** (Days 7-10.)
3. **API key service** — keys are credentials; audit creation/deletion is critical. (Days 10-14.)
4. **Auth login / logout** — populates the actor identity for all of the above. (Days 14-17.)
5. **Promotion `promotion_approved`** — emit the distinct event before executing. (Days 17-20.)
6. **All read-side audits (`secret.read`)** — lowest priority because reads happen at high volume and need rate-aware emission; defer to days 20-30 with separate volume-handling design.

## Reads — special handling

`secret.read` is high-frequency. Emitting a row per read may overwhelm `audit_log`. Options for the read path:

- **(a) Per-read row, partitioned table.** Real audit, real cost.
- **(b) Sampled audit** (1 in N) — loses individual events; not acceptable for a security audit log.
- **(c) Aggregate counter per (actor, project, secret) per minute** — preserves "who read what when" at minute granularity without row blowup.

**Decision deferred** to a Type-1 ADR. For now, do not emit `secret.read` until the ADR lands. Tracked in `FOLLOWUPS.md`.

## Failure modes and alerts

| Failure                                                        | Alert (where)                                                       |
|-----------------------------------------------------------------|---------------------------------------------------------------------|
| Audit emit returns error                                       | `keepsave_audit_emit_failed_total{event}` > 0 → page on-call         |
| Audit row count for a mutation endpoint is zero for > 5 minutes during steady traffic | composite metric: `requests_total{...} > 0 ∧ audit_rows_total{...} == 0` |
| Audit log grows faster than expected baseline                  | informational; check if a new read path was added without sampling design |

## References

- `backend/internal/service/secret_service.go` (Create/Update/Delete — currently no audit emit)
- `backend/internal/service/project_service.go` (same)
- `backend/internal/service/apikey_service.go` (same)
- `backend/internal/service/promotion_service.go:189, 257, 357, 399` (existing emit points — model to follow)
- `backend/internal/models/models.go:63-72` (audit_log shape)
- `backend/internal/repository/audit_repo_test.go` (existing test shape to extend)

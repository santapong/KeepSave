# Evidence register — feature inspiration and pain points

Checked 2 October 2026. Use primary sources and separate observations from
product hypotheses. Vendor documentation proves a documented approach, not
customer demand, an independent security assessment or KeepSave capability.
Recheck live pages before implementation; no price/plan parity claim is made.

| ID | Source and provenance | What it supports | KeepSave inference and limit |
|---|---|---|---|
| S01 | [It Should Be Easy but…](https://arxiv.org/abs/2509.09036), research preprint submitted 10 September 2025. | A qualitative study observed 21 new users performing secret storage/access and injection tasks; documentation/navigation problems led to difficulty and workarounds. | Prioritize task-based setup and tested examples. This narrow study does not establish KeepSave market demand or an onboarding success rate. |
| S02 | [Infisical Agent Vault](https://infisical.com/docs/documentation/platform/agent-vault/overview), official product docs. | Describes service access bundles, time-bound agent sessions, credential-attaching proxies and request logs. | The approved credential broker direction has product precedent. Prefer structured repository operations, pinned commits and exact receipts; do not claim the broader concept is unique or independently secure. |
| S03 | [Bitwarden machine accounts](https://bitwarden.com/help/machine-accounts/), official product docs. | Separates nonhuman identities, scoped projects, tokens and event views. | Expose accountable owners and distinguish people, API clients and future runners. KeepSave's current keys/leases are not a delivered workload registry. |
| S04 | [Doppler secrets](https://docs.doppler.com/docs/secrets), official docs, Secret Reminders section. | Documents per-secret recurring reminders and authorized recipients. | Lifecycle ownership and renewal reminders are a reasonable investigation. Begin in-app and metadata-only; arbitrary stored credential validity cannot be proven from a declared date. |
| S05 | [Doppler change requests](https://docs.doppler.com/docs/change-requests), official product docs. | Documents proposed secret edits, authorized review and invalidated approvals when affected changes are edited. | Future change proposals should pin exact revisions/artifacts. KeepSave already has protected environment promotion; this is a later extension, not a missing promotion engine. |
| S06 | [Doppler automated syncs](https://docs.doppler.com/docs/integrations), official product docs. | Lists delivery integrations including GitHub and Vercel. | Investigate one demanded destination only after durable effect handling. Export increases credential custody; an API that cannot read current values cannot prove secret equality. |
| S07 | [Vault lease revoke](https://developer.hashicorp.com/vault/docs/commands/lease/revoke), official command docs. | Distinguishes synchronous/queued revocation; force removal can leave an upstream secret engine inconsistent. | Local access denial and upstream cleanup need separate states. KeepSave vault leases do not automatically rotate arbitrary provider credentials. |
| S08 | [Infisical issue #5309](https://github.com/Infisical/infisical/issues/5309), user report opened 29 January 2026; closed and linked to #5480 when checked. | Reports a fresh self-hosted install missing project KMS keys and failing its first secret write. | A historical failure case motivates fresh-install/key readiness tests. It is not evidence of a present competitor defect or a general failure rate. |
| S09 | [MCP security guidance](https://modelcontextprotocol.io/docs/2026-07-28/tutorials/security/security_best_practices), official protocol documentation. | Covers token audience/passthrough, consent, SSRF and unauthorized state-handle use. | Keep the planned resource-bound auth, structured broker, endpoint validation and caller-bound runs. A run handle or skill instruction cannot authorize a tool call. |

## Observed KeepSave gaps

These observations were made against candidate `8fa8d6e`. They are evidence of
source behavior, not measured user frustration:

- `backend/internal/policy/policy.go` and `api/errors.go`: generic denied
  decisions and `FORBIDDEN`, without a safe diagnostic projection.
- `backend/internal/repository/audit_repo.go`: project/limit browsing without
  stable cursors or an incident-filtered export contract.
- `backend/internal/models/models.go`: no credential lifecycle owner/declared
  expiry fields. The restricted profile disables legacy policy metadata.
- `backend/internal/api/handlers_organization.go`: adding a member requires an
  existing user UUID; no invitation acceptance journey.
- `backend/internal/service/organization_service.go`: membership removal and
  audit exist; a scoped aggregate offboarding preview/receipt is absent.
- No complete password recovery or last-login-method safety journey was found.
- Current core refuses executable MCP/config generation; installations are
  metadata, not evidence of functioning Codex or broker connections.

The [current acceptance ledger](../../validation/2026-10-01-core-release/ACCEPTANCE.md)
is authoritative for existing local verification. Old Phase A/B backlog prose
must not turn implemented history/rotation/revocation into new feature claims.

## Validation before committing to more scope

The new pain-point hypotheses are: confusing denial troubleshooting, forgotten
renewal ownership, UUID-based onboarding friction, offboarding uncertainty and
slow incident review. Ask pilot users to demonstrate their current workaround
and frequency before promising value or adding delivery providers.

No user interviews, competitor deployment tests, purchases, external messaging
or provider operations were performed for this research. Sources support
candidate designs; the plan's product priority and estimates are inferences.

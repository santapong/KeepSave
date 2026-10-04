# KeepSave UX state and security inventory

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Source inventory reconciled 2026-10-04. This identifies implemented states and
remaining checks; it is **not a new complete browser/a11y audit**. The prior Phase A
gaps and hiring assumptions are preserved in [UX_STATE_LEGACY](archive/UX_STATE_LEGACY.md).
Keep the accepted Event Horizon layout, Field Twist mark, violet/mint identity,
recognizable icons and clear task language. See [branding](BRANDING.md),
[current status](STATUS.md) and [dated frontend evidence](validation/2026-10-02-harness-neutral-platform/FRONTEND.md).

## Current application surfaces

Paths in this table are under `frontend/src/`. Code presence identifies a review
surface; it does not prove all runtime branches or external journeys passed.

| Surface / source | Implemented source states | Boundary / remaining acceptance |
|---|---|---|
| Login, register, social callback — `pages/LoginPage.tsx`, `RegisterPage.tsx`, `SocialCallbackPage.tsx` | Disabled unconfigured providers; pending form/provider actions; generic error; state-bound callback | Real Google/GitHub consent/repeat/linking UAT pending; no account-existence hints |
| Account — `pages/AccountConnectionsPage.tsx`, `components/AccountSafety.tsx` | Connection/session loading/error/active/revoked; method/contact proof controls; failed revoke shown unconfirmed | Recent actual remaining-method authentication; real proof delivery; no token/session hash display |
| Recovery/identity confirmation — `pages/RecoveryPage.tsx`, `IdentityConfirmPage.tsx` | Capability check, unavailable/disabled, enumeration-resistant requested state, cleared fragment and explicit confirm/failure | Contact/recovery 15 minutes; invitations 24 hours. SMTP accepted is not delivered; live receipt remains pending |
| Workspace/project lists — `pages/OrganizationsPage.tsx`, `ProjectsPage.tsx` | Loading/error/populated/empty and explicit create/manage actions | Workspace name required; no sample secrets/global permissions. Nonempty workspace deletion and tenant denial remain server decisions |
| Organization management — `pages/OrganizationManagePage.tsx`, `components/OrganizationSafetyPanel.tsx` | Member/project management, owner-disabled controls, invitations and preview-bound offboard receipt | Exact impact/epoch/idempotency; unrelated organization/personal/global-session authority preserved |
| Secrets — `components/SecretsPanel.tsx` | Loading/error/empty/search; masked/revealed/edit/add/delete; history action; 30-second countdown and hidden-tab re-mask | Plaintext exists after an authorized read. Request-generation guards prevent stale view publication; typed delete and current server scope |
| History — `components/SecretHistoryPanel.tsx` | Metadata-only list, current/selected revision, typed restore, busy/error/stale reload guidance | Restore appends and checks expected revision; no historical plaintext in this view |
| Project recovery — `components/ProjectRecoveryPanel.tsx` | Bundle verify/preview, explicit selected restore, revision/current-owner mappings and errors | No implicit project replacement/undelete. External isolated drill and live preconditions remain explicit |
| Rotation/export — `pages/ProjectDetailPage.tsx` | Typed project-name rotation confirmation; environment-select export, pending/success/failure | Encryption-key rotation preserves history; downloaded `.env` is an intentional plaintext boundary |
| Lifecycle — `components/SecretLifecyclePanel.tsx` | Value-free list, unknown/current metadata revision, owner/date/provenance editor, save/error | Declared dates only; not provider expiry inspection. Stale metadata must not overwrite |
| Notifications — `pages/NotificationsPage.tsx` | Loading/error/empty/current reminders and lifecycle links | 30/7/1-day/current-authority reminders; worker/operational delivery acceptance separate |
| Audit — `components/SafeAuditPanel.tsx` | Safe search/filter/page, export queued/ready/failed/download states | Current access and bounded safe metadata; no raw secrets/results/proofs in exports |
| Promotion — `components/PromotionWizard.tsx`, `PromotionsList.tsx` | Review/diff/policy/execute states, failure/kill-switch banner, protected approval and typed rollback | Exact current source/destination/snapshot; requester cannot self-approve; verify all confirmation branches |
| API keys — `components/ProjectAPIKeysPanel.tsx` | Metadata list/create/error, one-time raw-key card with hold-to-reveal/copy, typed revoke/delete | Raw card is a credential boundary; source has no automatic expiry of its retained `newRawKey` state |
| Developer access — `pages/DeveloperAccessPage.tsx` | Loading/error, repository/profile/grant/run tabs, disclosed external connection check, approve/revoke/export, commit preparing/status/receipt/cancel | Default-off candidate. Separate portable profile/client run; selected dialogs are ordinary confirmations, not typed. Real harness/App/isolation UAT pending |
| MCP consent — `pages/MCPConsentPage.tsx` | Sign-in required, pending/expired/request error, scoped consent/deny decision, callback validation | No credentials rendered; reject changed account/pending request; exact native OAuth acceptance pending |
| Help/integration references — `pages/HelpPage.tsx` and integration pages | Source instructions/catalog examples with retained compatibility surfaces | Guides must follow actual API/availability; old API-host execution/SSO/AI metadata is not enabled enforcement |

## Cross-cutting requirements

These retain the security requirements of the original inventory. They describe
target behavior; the table's limits prevent confusing a requirement with implementation.

1. **Plaintext secrets must auto-hide.** Never promise erasure merely because a
   display is masked. The dashboard timeout is 30 seconds; widget timeout requirements
   remain incomplete as documented in [EMBED_STATE](EMBED_STATE.md).
2. **Destructive actions must require typed confirmation** for project/secret/API-key
   delete, production promotion and rollback. Existing typed and ordinary-dialog
   paths must be reviewed against that requirement; no blanket compliance claim.
3. **Errors must not echo internal state.** Use the safe public contract from
   [ERROR_HANDLING_STANDARD](ERROR_HANDLING_STANDARD.md), with no value/proof/token.
4. **Every asynchronous operation needs a clear loading state.** The original
   requirement uses 200 ms as the visible-indicator threshold; no timing audit is
   claimed here. Prevent duplicate actions and do not invent completion on failure.
5. **Empty states need a next step.** Distinguish no current records from unavailable
   or failed loading. A denied foreign resource must not acquire an existence hint.
6. **Denied states need safe guidance.** Reauthenticate after session expiry;
   do not show protected diagnostics before authentication or expose another tenant.
7. **Production environments must be visibly distinct** with labels/icons and
   meaningful color, without relying on color alone.
8. **Clipboard copy needs feedback.** The dashboard's best-effort 20-second clipboard
   clear checks the copied value; operating-system/browser policy can prevent erasure.
9. **Hidden pages must immediately hide revealed secrets.** Dashboard/widget reveal
   masking exists; this does not establish edit-buffer zeroization or recall copied data.

## Evidence and unresolved review surfaces

The application token accessor prefers sessionStorage but has a localStorage
fallback when unavailable; user metadata can persist locally. Secret-value storage
is forbidden. Account changes and protected 401 clear applicable application
identity/proofs/caches and reject late responses; the retained embed API does not
provide all of those guards. Document this distinction rather than advertising
one cache policy across every adapter.

Configured, locally verified, externally qualified and operationally accepted
states must be labelled separately. A profile approval or native package digest
is not device attestation, model locality or local administrator enforcement.
Revocation stops subsequent admissions/protected result access; already-admitted
work may finish and returned content cannot be recalled.

Outstanding targeted review includes whole-application keyboard/focus/reduced-motion
and light/dark contrast, all confirmation variants, one-time API-key card expiry,
widget inactivity/reset/late-response behavior and new platform busy/error transitions.
These are review targets, not findings from a newly executed full browser sweep.
Preserve the dated local browser/test evidence; real provider/SMTP/native-harness
and separate-host acceptance remain unexecuted gates.

Security Engineer review is mandatory for reveal/edit/copy, production promotion,
raw-key presentation, origin/storage and other credential boundaries. Independent
human review requirements remain in [repository rules](../CLAUDE.md) and
[ROLES](ROLES.md); agent documentation review does not satisfy those signatures.

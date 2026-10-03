# Identity and team controls — local validation

Event/check date: October 2, 2026 (Asia/Bangkok). Scope: the uncommitted local
candidate on `feat/harness-neutral-platform-20261002`, under ADR0029 sponsor
implementation authorization. This is a bounded implementation receipt, not
independent Security/Tech Lead approval or installation acceptance.

## Implemented boundary

`backend/internal/identity` owns verified contacts, authentication-method safety,
email proof custody/delivery, invitations and scoped offboarding. Handlers in
`handlers_identity_platform.go` parse input and call the injected application
service; they perform no SQL or decryption. Proof custody is the narrow
`Seal`/`WithOpened` port implemented by the vault. Authority uses the existing
shared guard and policy evaluator; this slice adds no second evaluator.

The feature requires PostgreSQL and defaults off. Email proof actions additionally
require authenticated operator SMTP and a canonical same-origin HTTPS app URL.
Additive migrations 027/031 supply identity records, session method evidence and
five-minute offboarding previews; their other-dialect files are no-op markers.
Migration 026's membership trigger is the sole member-epoch writer.

- Contact confirmation and password recovery proofs expire after 15 minutes;
  invitations expire after 24 hours. Random 256-bit proof values appear only in
  encrypted delivery payloads and email fragments. Responses/jobs/audit contain
  nonsecret IDs. Confirmation accepts JSON body proofs, never URL/query proofs.
- A contact proof is bound to its original account and browser session. Recovery
  uses a currently verified contact, returns generic accepted metadata for
  unknown contacts and creates no account merge. A completed reset revokes
  existing sessions. Existing Google/GitHub sign-in responses remain compatible.
- Method removal requires a recent session authenticated through a different
  remaining usable method. Legacy sessions with unknown method evidence do not
  prove that requirement. Merely having another configured method is insufficient.
- Invitation acceptance rechecks current inviter administration, the original
  inviter session and the accepting session. It matches an immutable target user
  or that account's currently verified contact; acceptance cannot silently change
  an existing member's role. The organization owner is protected.
- Offboarding preview counts and ownership metadata share a repeatable-read
  snapshot. The persisted actor/org/target/epoch/impact digest expires after five
  minutes. Execution obtains exclusive shared-authority barriers, revalidates
  the digest and consumes the preview atomically with required audit/outbox and
  a durable idempotency receipt. Exact retries recover the committed receipt.
- The impact includes workspace keys/leases, direct and inherited tool authority,
  active child runs, target-approved profile/package metadata, pending approvals,
  invitations, project ownership and credential lifecycle revisions. Missing run
  cascade wiring fails closed. Ownership and credential responsibility are listed
  for explicit reassignment; neither transfers silently. Global browser sessions,
  other organizations and personal projects survive scoped offboarding.

## External delivery semantics

The worker's `StepDelivery` claims only ID-only `identity.proof_delivery` external
jobs, with a 60-second lease and one attempt. It commits queue dispatch and proof
dispatch before SMTP. SMTP has a 20-second network deadline and authenticated
STARTTLS with certificate verification. No database lock is held during network
use; payload decryption stays inside the custody callback.

SMTP acceptance, pre-dispatch failure and uncertain acceptance remain distinct.
An SMTP response followed by failed outcome persistence is uncertain. A stale
queue/proof dispatch is swept into uncertainty and cannot send again. Expired,
revoked or consumed payloads are cleared. Manual resend creates a fresh proof and
invalidates the older proof; it never replays an uncertain external attempt.
Invitations require explicit revoke/recreate rather than hidden resending.

## Executed checks

The twelve `TestPlatformIdentity*` API contracts passed on disposable PostgreSQL 16
with all shipped migrations 001–032 and Go 1.27.1 race detection. The final run,
including repeatable-read preview and inherited tool authority, passed in 10.569s.
The procedure uses fixed synthetic database credentials, isolated fixture schemas
and owned disposable containers, never an operator DSN. The run output is
`/tmp/keepsave-identity-check-result.log` in the local task environment.

| Contract | Observed assertion |
| --- | --- |
| Contact proof atomicity | Exact 15-minute expiry, double-consume refusal, audit rollback and no raw proof in response/jobs/audit. |
| SMTP uncertainty | Explicit fresh proof only; failed outcome audit after accepted SMTP remains uncertain. |
| Recovery/account safety | Verified account selection, generic unknown request, session revocation, purpose/expiry refusal and no email auto-merge. |
| Method evidence | Last method protected; same-method/unknown sessions cannot remove it; a proven remaining method can. |
| Invitation authority | Exact 24-hour expiry, one-time accept, intended account/contact and current original-parent authority. |
| Scoped offboarding | Audit rollback, member epoch/rejoin behavior and unrelated organization/session/ownership preservation. |
| Revision-bound preview | Missing/expired/cross-org/stale preview refusal; real lease mint after preview and lifecycle revision change invalidate it; exact retry preserves receipt. |
| Inherited tool authority | Synthetic shipped-schema rows exercise the real run cascade: another actor's two runs depend on the departing approver; mint/approval changes invalidate preview; receipt counts two runs rather than one grant. |
| Worker crash recovery | Queue/proof dispatch precede synthetic sender; crash before/after proof dispatch cannot replay SMTP; revoked payload cleanup and manual fresh proof. |
| Concurrent legacy mint | Real guarded lease admission waits behind offboarding and fails after the membership change commits. |

The browser controls use the accepted existing UI classes. Contact/recovery pages
strip and capture the proof fragment before the first availability request, keep
proofs ephemeral and perform confirmation only after an explicit user action.
The workspace panel displays reassignment consequences, requires a stored preview
and keeps the same execution key across an uncertain retry. It refreshes the member
list only after a matching committed receipt. Tool connection controls use
metadata-only MCP delegation records, server capability gating and confirmed
revocation. Neither page treats a listed client or run ID as authority.

Earlier focused frontend runs passed nine account/confirmation/recovery tests and
seven workspace/delegation tests. They cover fragment cleanup under StrictMode,
JSON-only explicit confirmation, generic recovery, protected last methods,
uncertain manual resend, stable offboarding retries, member-list changes only
after commit, email readiness/owner protection, delegation gating and late account
response refusal. A subsequent combined run encountered Vitest worker startup
timeouts while host memory and swap were exhausted; that is not a passing check.
The final shared pinned Node24 run passes34 files/161 tests, configured lint and
application/widget builds. A further three-file/nine-test affected-page run
includes delayed reset completion after an account switch. See [FRONTEND.md](FRONTEND.md)
for exact receipts and browser scope. Extra owned builds were stopped during
resource exhaustion; the unchanged failed environment was not blindly retried.

## Remaining gates

The final recovery follow-up adds a narrow injected legacy revocation port.
Successful reset also expires retained API keys, revokes their leases and
denylists recorded agent issuance through its expiry in the same proof/password/
session/audit/outbox transaction. Formerly usable key/token reads, unrelated-user
preservation, final-audit rollback, missing-port refusal and serialized mint/use
pass four focused real-PostgreSQL tests. The full final-source gate is recorded
in [BACKEND.md](BACKEND.md). Recovery imports no authority; this login recovery
revocation is separate from encrypted vault restoration.

The sender in these contracts is synthetic: authenticated live operator SMTP,
mail receipt, actual provider sign-in and real native Codex/Hermes qualification
remain installation/UAT gates. These checks do not establish deployed worker
operation, browser visual acceptance, remote CI, independent reviews or release
approval. No external secret-delivery adapter was implemented. No provider/mail
action, credential configuration, live migration, commit or deployment occurred.

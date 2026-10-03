# Frontend and browser — local candidate validation

Event/check date: October2,2026 (Asia/Bangkok). Source: dirty
`feat/harness-neutral-platform-20261002`, base3878e69. The accepted landing,
Field Twist identity and workspace design remain preserved. The application is
not deployed by this work.

## Executed checks

Pinned runtime:
`node:24.21.0-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1`.
The verification container is capped at two CPUs/two GiB with one Vitest worker.
Evidence is outside Git in `/mnt/data/keepsave-neutral-2026-10-02/`.

| Check | Result |
|---|---|
| Full suite before final reset response guard |34 files/161 tests passed, `frontend-final-green.log`. |
| Final affected-page checks |Three files/nine tests passed, `frontend-reset-final.log`; includes the additional delayed reset after account switch case. |
| Configured lint |Passed in both receipts; existing rules target the embed SDK, not all application TSX. |
| TypeScript/application/widget builds |Passed after final affected-page changes. Existing large-chunk warnings remain. No browser performance benchmark is implied. |
| Generated API types |`generate-core-api-types.mjs --check` passed after sole-contract consolidation and unknown legacy run-scope response relaxation. |
| Full dependency audit |`frontend-npm-audit.json`:zero reported vulnerabilities, including development dependencies, at this check date. |

Earlier receipts include resource-exhaustion worker-start timeouts and an
outdated AccountSafety fixture that unintentionally made an unmocked availability
request. Those are retained failures, not successful gates. The fixture now
explicitly disables its unrelated identity panel; account-safety tests exercise
that panel independently. The final bounded runs pass.

## Meaningful assertions

- Authentication failures/account switches reject late responses and clear only
  the initiating browser session. Password reset completion cannot sign out a
  newer unrelated account. Its proof/password stay ephemeral and never enter
  browser persistent storage or URL queries.
- Contact/invitation proofs are stripped from browser fragments before calls;
  confirmation is explicit and JSON-only. Public recovery remains generic.
- Offboarding uses a stored exact preview and stable retry identifier; member
  lists change only after a matching committed receipt.
- Direct lifecycle/audit routes do not request credential values. Switching
  projects ignores a late response from the former project. Entering Secrets
  explicitly still fetches authorized masked-display values.
- Developer-access opening fetches metadata only. Provider checks disclose the
  read and await confirmation; profile/package approval requires an independent
  actor. Run/operation request IDs survive retry and client grants stay separate.
- Recovery v2 lifecycle restoration is opt-in per selected record, includes the
  expected target metadata revision and requires explicit current-member owner
  mapping. Pending/failed audit exports cannot be downloaded.

## Visible local browser

The controlled visible in-app browser used a disposable owner/account and project
at `http://127.0.0.1:4771`, backed by loopback API4770/PostgreSQL test containers.
Observed views: disabled unconfigured GitHub/Google sign-in, password sign-in,
account-method safety and current/revoked session metadata, masked Secrets,
lifecycle ownership/date controls, safe audit records, project-scoped developer
access and repository/profile/client setup. Responsive navigation in the narrow
browser pane was opened successfully. Rendered screenshots confirmed headers,
card padding, fields and tab controls use the existing KeepSave styling. Console
error/warning inspection returned no entries during the observed views.

No live provider connection check, private key upload, invitation/email, grant,
profile approval, native installation or model request was performed. The account
and vault fixture was populated through the synthetic HTTP acceptance driver.
This bounded browser check does not establish full responsive/accessibility,
real consent, mail delivery or native-client interoperability acceptance.

The initial inspection mistook earlier `secret.read` audit events for metadata
page fetches. Source tracing and the route regression confirmed they belonged
to the explicitly entered Secrets page; no metadata fetch leak was reproduced.
An actual late project-header response race was fixed and verified instead.

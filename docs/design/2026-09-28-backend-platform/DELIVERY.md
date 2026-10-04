# Incremental delivery plan

> **Program boundary, clarified 2026-10-04.** The M0–M5 table below preserves the earlier staged plan and its original Codex-first assumption. The approved harness-neutral G0–G6 program, local implementations and pending exit gates are recorded in the [October checkpoint](../2026-10-02-harness-neutral-platform/README.md). Codex and Hermes are qualification candidates through separate client-bound runs; no universal harness support or completed qualification is implied. Use the [current documentation index](../../README.md) for delivery instructions.

Approved program, originally drafted 2026-09-28; status reconciled 2026-10-02.
Codex first, a read-only GitHub App, one approved repository/commit per ten-minute
run, broker-held provider credentials, separate restricted Linux runner and
self-hosting are settled decisions under ADR0023–0027. Current implementation
and pending exit gates are in [IMPLEMENTATION](IMPLEMENTATION.md) and the
[acceptance ledger](../../validation/2026-10-01-core-release/ACCEPTANCE.md).
The current bounded release is identity and reliable vault under ADR0028.
The stage table is the delivery target; it does not mark every target delivered.

**Owner clarification, 2026-10-02:** KeepSave must support different harnesses
and accommodate future technology. Codex is the first pilot candidate, not a
product restriction. Shared authorization, runs, custody and audit remain
harness-neutral; transport/configuration/artifact adapters are separate from
provider adapters. The [extension contract](../2026-10-02-product-roadmap/README.md#harness-neutral-extension-contract)
and [F07](../2026-10-02-product-roadmap/BACKLOG.md#f07--controlled-access-across-harnesses-strategic-m2m5)
refine the M2/M4 targets below. Require equivalent acceptance through two real
harnesses before advertising broad compatibility. Other custody, isolation,
review and first-provider defaults remain unchanged; new support is still planned.

## Delivery rules

Use bounded feature branches off the verified integration branch, preserve the accepted frontend, and integrate one tested capability at a time. A directory-wide rewrite is not a prerequisite. The existing dirty frontend/social-auth/scope baseline is preserved in the feature checkout; do not discard it or silently claim integration/release. Each subsequent capability remains a bounded vertical slice.

Each item owns a user outcome, an authorization matrix, a failure test, and a rollback path. Classification and review follow [CLAUDE.md](../../../CLAUDE.md), [ADLC](../../ADLC.md), and the repository’s [workflow skill](../../../.claude/skills/workflow/SKILL.md). Proposed crypto/key hierarchy, audit semantics, and execution trust-boundary changes need the appropriate ADR and Security/Tech Lead review before implementation/integration as those rules require. Drafting this plan does not record that review as completed.

## Stages

| Stage | Outcome and concrete work | Dependency | Exit evidence |
|---|---|---|---|
| M0 — Authorization baseline | Reproduce/fix R01–R03; establish typed principal/action/resource model; scope MCP metadata/installations; map current roles; establish a feature/capability ledger and unsafe-build gate | Approved implementation scope | Real router + real database tests for foreign installation/private server, wrong role, wrong parent key, wider lease/glob/expiry, actual lease FK, and policy denial before decryption; mutation audit assertions |
| M1 — Reliable vault lifecycle | Unit of Work and audit append; complete key/version design; create/update/import/template/promotion history; restore as new revision; encrypted recoverable bundle; key/backup/corruption drill | M0; accepted key/audit ADRs | Concurrent edit/rotation, stale restore rejection, complete rollback/history after rotation, transaction failure rollback, clean restore from external bundle + recovery keys, deletion/retention tests |
| M2 — MCP interoperability | Pinned Go SDK spike; `/mcp` transport/auth adapter; namespaced tool identity, bounded schemas; typed UI catalog; diagnostics; replace unverified generated client instructions | M0; approved protocol/auth design | Exact client/SDK/protocol matrix, real discovery/list/call/cancel, wrong audience/origin, expired/invalid token, malformed/oversized output, duplicate tool names, actual frontend response contract |
| M3 — Broker and isolated pilot | Workload enrollment, run grants/lineage, provider connection, brokered operation, one vetted connector, independent restricted runner, audit receipts/revoke | M0, M1 audit primitives, M2; accepted runner/credential ADR | End-to-end first pilot below; no access to API secrets/host files, no unauthorized egress, revoke/outage denials, queue/time/memory bounds, crash/unknown-outcome handling |
| M4 — Private skills and harness profiles | Immutable skill catalog, review/revocation, requested capabilities, official skills-extension adapter where supported, portable access profiles and separately versioned harness packages, compatibility report | M3 | Artifact/export tamper and same-name origin tests, fresh approval after changes, nested-skill boundaries, required capability mismatch denied, secrets absent from bundles and model context; two-harness portability evidence before broad support claims |
| M5 — Team scale and enterprise readiness | DB-serialized audit head, durable outbox/jobs/webhooks, shared admission limits, migration coordination, signing-key/cache coherence, backup operations; separate runner host and measured self-hosted team operation; enterprise SSO/cloud-KMS expansion deferred | M1–M4 for the relevant product paths | Two-replica fault/load runs, cross-instance revoke behavior, tenant isolation, queue recovery, restore-time measurements, chosen read-only GitHub/Codex live verification and operational runbook |

M1 and the M2 protocol spike can proceed independently after the authorization contract is settled. M3 may reuse M1’s audit work before the entire recovery UI is finished, but no broader production rollout should precede a successful recovery drill. M4 does not unblock unsafe execution; it depends on it being controlled already.

Durable job/outbox primitives enter as soon as a stage needs recoverable background work. M5 is their multi-instance verification and operational rollout, not permission to use unreliable queues in earlier credential flows.

## First implementation slices

These are small enough to review independently and should precede a broad module migration:

1. **M0a: principal and lease correctness.** Start in `api/middleware.go`, `handlers_agent.go`, `service/{lease,agent_token}_service.go`, and real-FK tests. Carry both actor user ID and API-key/workload ID. Verify requested environment, keys and TTL against parent authority. Existing read-token confinement stays in place.
2. **M0b: MCP object ownership.** Update the installation/server use cases and scoped repository queries. Include private-server read/install, project binding and cross-user update tests. Separate public catalog DTOs from private configuration/build diagnostics.
3. **M0c: policy boundary.** Define the action/resource matrix, introduce a service-level authorizer, move the pilot’s sensitive paths through it, and record which legacy paths remain. Default-disable any new feature whose complete path is not covered.
4. **M1a: audit and transaction contract.** Add transaction-aware audit append and failure tests before history or grant creation depends on it. Keep existing chain history readable. Do not label all legacy emissions transactional until every relevant caller moves.
5. **M1b: history vertical slice.** After the key-version ADR, deliver create→update→metadata-history→value-read→restore for one secret, including concurrency and key rotation. Then route import/template/promotion/rollback through the same mutation logic.
6. **M2a: protocol-only spike.** Use a synthetic metadata tool and local fixture server. Prove current/legacy version behavior and authentication plumbing without vault values or external actions. Pick/pin SDK and protocol versions from that evidence.
7. **M3a: brokered read pilot.** One synthetic provider, then one explicitly configured real GitHub App connection restricted to one test repository. A GitHub App is separate setup from the prepared GitHub social-login app.

Implementation ownership is by responsibility: backend owns use cases and contracts; security reviews authority/custody changes; platform owns runner and failure/recovery behavior; client work owns adapter/SDK compatibility; the product owner accepts the user journey. These are role requirements, not claims that reviewers have been assigned or signed off.

## First pilot: “Review this repository with controlled access”

The developer uses an existing harness connected to KeepSave. The team approves a skill/profile and a read-only connector for repository A. KeepSave opens a ten-minute run and mediates repository reads. The model receives permitted content and tool results without the provider credential.

Required acceptance cases:

| Case | Required result |
|---|---|
| Approved person, profile and repository A | Successful read; receipt records actor, run, tool/artifact, policy revision, grant and outcome |
| Repository B, another environment or another tenant | Denial before credential delivery/provider call; scoped denial receipt |
| Prompt asks the skill to reveal the token or use a new tool | No additional authority; denied or unsupported operation |
| Skill/tool/profile changes after approval | Reapproval required; old grant cannot authorize the changed artifact |
| Run expires or is revoked | New admissions/redemptions denied; queued work cancelled; in-flight status reported honestly |
| Another runner steals a grant handle | Rejected by runner/run binding |
| Database/policy service unavailable | Protected work denied; no cached unrestricted fallback |
| Connector prints a canary credential or attempts forbidden network/file access | No canary in returned results/logs; sandbox blocks forbidden channels; limitations of permitted data remain documented |
| Worker crashes after external dispatch | Attempt becomes unknown/reconcilable, no blind duplicate write |
| Full local restart | Required configuration/approvals/evidence survive; no revoked run is revived |

Run the synthetic version in CI. The real-provider version is a controlled UAT exercise with a dedicated connection; it is not enabled by this plan. No publishing, repository mutation, account administration or production credential use is needed for the read-only demonstration.

## API and compatibility work

Proposed management resources, with exact payloads finalized per slice:

- Project secret history metadata, explicit version value read, restore request and expected revision.
- Backup jobs, verification results, restore preview and approved restore execution.
- Organization workload identities and enrollment/revocation.
- Project provider connections and credential bindings, returning redacted metadata only.
- Organization policies, immutable revisions, and scoped approval requests.
- MCP artifacts/installations/catalog and connection diagnostics; protocol calls at `/mcp`.
- Private skill versions, harness profile versions, run admission/status/cancel and audit receipts.

Use stable error codes (`permission_denied`, `approval_required`, `profile_incompatible`, `binding_unavailable`, `grant_expired`, `revision_conflict`) with safe text and correlation IDs. Final HTTP/MCP mappings must follow the relevant protocol. All list endpoints need tenant scoping, pagination and bounded filters. Idempotency keys for retries bind to caller, target and request digest; changed requests cannot reuse an earlier approval/result.

Connection diagnostics check authentication, selected protocol, permitted tool discovery, binding metadata, reachability and runner capability. A “test connection” must declare whether it performs an external read; it must not quietly execute a mutating tool. Results expose safe stages/reason codes, never credential values or raw subprocess logs.

## Verification matrix

| Layer | Minimum meaningful checks |
|---|---|
| Policy/domain | Permission intersection and deny precedence; monotonic attenuation; bounded delegation; temporal/state-machine tests |
| Repositories | Real PostgreSQL constraints/transactions/concurrency; SQLite migration/parity fixtures; MySQL tests for supported release profile |
| HTTP/MCP | Actual router+auth+service+DB flows; every mutation with audit assertion; negative-auth cells per new surface |
| Crypto/recovery | Corruption/tamper/wrong-key tests, version migration, concurrent rotation, retained snapshots, restart and independent restore |
| Runner/broker | Synthetic malicious connector fixtures, identity replay, files/network/process/resource isolation, output leakage and cancellation |
| Contracts/clients | OpenAPI and actual responses, SDK retries/cache expiry, supported MCP versions, real harness adapter compatibility |
| Operations | Two API replicas, worker crash/redelivery, authority outage, unavailable KMS, coordinated migrations, audit continuity and retention |

Retain existing race/vet/fuzz and frontend checks. Run frontend checks when client contracts or integration components change; a backend proposal alone does not warrant repeating visual tests. CI file presence is not proof a job ran successfully; archive actual output with environment/tool versions and skips.

For scaling, first measure one API + one runner on a recorded fixed resource budget using synthetic tenants/secrets and provider latency. Record p50/p95/p99 added gateway and policy latency, DB pool wait, audit contention, memory, queue age and rejection rates. Then repeat with two API replicas and failure injection. Set release capacity/SLO claims from measured results; the plan does not invent a throughput promise.

## Recorded design choices and rationale

These choices are recorded by sponsor authorization in ADR0023–0027 and the
core refinement in ADR0028. Independent Security/Tech Lead review remains
pending; it is a review gate, not a request to restart the approved pilot.

| Decision | Approved direction | Alternative/tradeoff |
|---|---|---|
| Module boundaries and execution topology | Modular Go core plus isolated runner/build services | Microservices add operational and distributed-transaction complexity before demand |
| Key/version/recovery continuity | Versioned DEKs referenced by all ciphertext, explicit backup-key recovery | Re-encrypt every retained record on each rotation is simpler in schema but grows lock time and recovery risk |
| Authorization and grant lineage | Shared typed evaluator, deny precedence, parent-bounded grants | Handler-specific checks repeat today’s inconsistencies |
| Audit consistency | Transactional local mutations, durable pre-dispatch admission, DB-serialized chain head | Best-effort audit cannot prove credential-use decisions; external effects remain non-atomic |
| Credential custody | Brokered operations first; tightly scoped injection as explicit compatibility mode | Raw injection is broadly compatible but loses control after delivery |
| MCP and skills compatibility | Pin supported protocol/SDK versions; use official skills format/extension when tested | A proprietary skill wire format increases adapter maintenance |
| Team tenant migration | Explicit personal scopes plus organization scopes and validated transfers | Implicit ownership mapping risks breaking existing users or widening access |

Do not preassign new ADR numbers or mark them Accepted here. Check the live ADR registry when each decision is formalized.

## Scope tradeoff and stop conditions

The written reason for revisiting Phase A’s runtime exclusion is the user’s request for company-controlled developer harness access. The bounded scope is isolated approved connectors and private skills. Public marketplace distribution, general agent orchestration, broad model routing, new AI dashboards, mobile SDKs and multi-region services remain deferred. KeepSave can supply policy and credentials to existing harness/orchestration products through adapters.

Stop expansion of a stage when it cannot prove ownership/attenuation, audit durability, custody/isolation or restoration under its advertised mode. Narrow the advertised capability until the evidence exists. No stage is complete because an endpoint, migration or UI card exists.

Three product choices can be settled during the first pilot without blocking this architecture: the first real harness adapter, the initial customer-hosted versus managed runner deployment, and the first enterprise IdP/KMS. Default planning direction is one generic MCP client plus one tested harness adapter, an operator-managed isolated connector runner, and the existing supported vault-key path. Recheck those choices against an actual team’s constraints before committing to broad vendor support.

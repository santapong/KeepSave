# 12. Governance and release decisions

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

[`CLAUDE.md`](../../CLAUDE.md), [roles](../ROLES.md), the
[ADR index](../adr/README.md) and [threat model](../THREAT_MODEL.md) define the
repository's process. The current documentation refresh does not award missing
review signatures or change security invariants. Historical phase checklists
record planning/completed source, not current product acceptance.

## Review responsibilities

| Decision class | Required process |
|---|---|
| Type-1: crypto/key hierarchy, audit-field removal, breaking schema, tenancy boundary or promotion semantics | ADR and independent Security Engineer plus Tech Lead sign-off. |
| Type-2: reversible endpoint/component/dependency change | RFC when substantive and standard review. |
| Type-3: local refactor/documentation | Standard review. |

Security retains a veto over crypto, authentication and promotion; Tech Lead owns
architecture/ADR decisions. Agent engineering reviews provide input, not those
signatures. Threat-model deltas, rollback/cutover constraints, required audit and
failure tests must accompany the relevant feature. Safe client errors and
transactional audit assertions remain repository gates.

## Branch and publication model

Feature branches integrate through **develop**; **main** is the release line.
Use conventional commits and verified repository rules. A source push, a tag,
remote CI success and production acceptance are distinct events. Source candidate
`6684aca` entered develop at `1021435`; the owner directed source publication to
main at `202cc5d` on October 3. The
[publication note](../releases/2026-10-03-source-publication.md) records that bounded
owner-authorized exception. It did not establish a release/tag/deployment or
standing waiver of future independent review. Package version stays 1.4.0-rc.1.

October 3 main/develop CI was checked failed on October 4. Exact-revision green
remote evidence and release reviews remain gates; tests on an earlier candidate
cannot be substituted. This documentation refresh is a develop integration task,
not an instruction to publish main or deploy the application.

## Current decisions and historical provenance

Use the ADR index rather than a copied partial table. Important current records:

- [0023 authorized use cases](../adr/0023-authorized-use-cases.md) and
  [0024 transactional audit/recovery](../adr/0024-transactional-audit-recovery.md).
- [0025 brokered isolation](../adr/0025-brokered-isolated-execution.md),
  [0027 self-hosted topology](../adr/0027-self-hosted-team-topology.md) and
  [0028 core identity/vault](../adr/0028-core-identity-and-vault-release.md).
- [0029 harness-neutral platform](../adr/0029-harness-neutral-platform.md), which
  supersedes single-harness assumptions and separates server/admin/unverified
  controls. Earlier [0026 Codex](../adr/0026-mcp-skills-codex.md) and hosted
  [0016 topology](../adr/0016-deployment-topology.md) remain historical decisions.

Preserve dated ADR/audit receipts and supersession rationale rather than rewriting
an old “accepted” label into a new implementation claim. The current roadmap is
G0 core acceptance → G1 shared authority → G2 team vault → G3 portable MCP/OAuth →
G4 broker/runner → G5 private skills/profiles → G6 team operations. Source exists
across those slices, but their operational exit gates remain individually open.

## Delivery truthfulness

Publish support only for tested client builds/capabilities. All new flags default
off; configuration, synthetic tests and operational acceptance use distinct labels.
Do not claim universal harness compatibility, assessed compliance, device
attestation, exactly-once external effects or unmeasured uptime/throughput.
Deferred work includes arbitrary connector/skill scripts, external secret delivery,
model credentials, enterprise SSO, marketplace and multi-region operation.

The [documentation hub](../README.md), [project structure](../PROJECT_STRUCTURE.md),
[branding](../BRANDING.md) and current
[acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
are the reviewer/operator navigation points. Historical role staffing/due dates
are context and must be rechecked before assigning current work.

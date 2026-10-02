# KeepSave system documentation

Reconciled 2026-10-02 (Asia/Bangkok). The current implementation is an unreleased
identity/workspace/reliable-vault candidate. This index describes source and
contracts; executed acceptance, compatibility limits and remaining gates belong
in the [acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md).
Independent security review, real Google/GitHub UAT and deployment are not claimed.

Start with the [architecture](../ARCHITECTURE.md), [threat model](../THREAT_MODEL.md)
and [implementation checkpoint](../design/2026-09-28-backend-platform/IMPLEMENTATION.md).
Older audit documents and architecture SVGs are dated historical evidence, not
proof of supported integrations or runner isolation.

| Chapter | Scope |
|---|---|
| [01 — Overview](01-overview.md) | Current core and approved next journey. |
| [02 — Backend](02-backend.md) | Composition, authorized services and incremental module boundaries. |
| [03 — API](03-api-reference.md) | Maintained core OpenAPI and compatibility/refusal boundaries. |
| [04 — Data](04-data-model.md) | Additive migration, authority, journal and key continuity. |
| [05 — Security](05-security.md) | Current trust controls and explicit enforcement limits. |
| [06 — Promotion](06-promotion.md) | Promotion mechanics; current journal/source-digest adapter note. |
| [07 — Frontend](07-frontend.md) | Accepted frontend implementation reference. |
| [08 — Widget](08-embed-widget.md) | Separate host-page trust domain; compatibility UAT pending. |
| [09 — Infrastructure](09-infrastructure.md) | Exact runtimes, development vs self-hosted reference. |
| [10 — Operations](10-operations.md) | Cutover, recovery, retention and incident limits. |
| [11 — Testing](11-testing.md) | Executed-evidence discipline and meaningful release gates. |
| [12 — Governance](12-governance.md) | Repository review requirements and dated ADR reference. |
| [13 — SDKs/integrations](13-sdks-integrations.md) | Existing client inventory and future MCP/broker program. |

Go/Gin remains a modular monolith with PostgreSQL transactions. The current API
returns permitted vault values to its caller. Future GitHub broker operations
hold provider tokens within the broker. Skills, prompts, tool annotations and
client-reported harness identity never grant authority. The marketing site stays
static at `keepsave.draveniq.dev`; `app.keepsave.draveniq.dev` is the intended
same-origin application, not a deployed result of this work.

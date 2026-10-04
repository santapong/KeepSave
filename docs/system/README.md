# KeepSave system documentation

**Your secrets. In the right orbit.** Source reconciled October 4, 2026
(Asia/Bangkok), for the integrated harness-neutral candidate. The canonical local
project is `/mnt/data/company/apps/KeepSave`. Start with the
[documentation hub](../README.md), [architecture](../ARCHITECTURE.md),
[architecture views](../ARCHITECTURE_VIEWS.md) and [branding](../BRANDING.md).

These chapters explain current code/contracts. **Implemented, locally tested and
operationally accepted are different states.** The package stays 1.4.0-rc.1,
unreleased/untagged. Owner-directed October 3 source publication did not complete
independent reviews or deployment. The October 3 main/develop CI runs were checked
failed on October 4; exact-revision green remote CI is still required.

| Chapter | Current scope |
|---|---|
| [01 — Overview](01-overview.md) | Vault/team journeys, harness-neutral candidate and trust boundaries. |
| [02 — Backend](02-backend.md) | Typed composition, modules, process ownership and ordered authority. |
| [03 — API](03-api-reference.md) | Maintained management contract, new interface groups and refusals. |
| [04 — Data](04-data-model.md) | Migrations001–033, epochs, journal, artifacts/runs and recovery v2. |
| [05 — Security](05-security.md) | Current identity, custody, delegation/result checks and explicit limits. |
| [06 — Promotion](06-promotion.md) | Source-bound approval, vault transactions, snapshots and rollback. |
| [07 — Frontend](07-frontend.md) | Field Twist identity, capability gates, account/team/tool flows and cache handling. |
| [08 — Widget](08-embed-widget.md) | Host-page trust, origin handshake, safe DOM rendering and compatibility limits. |
| [09 — Infrastructure](09-infrastructure.md) | Runtime pins, control/private runner references and acceptance gaps. |
| [10 — Operations](10-operations.md) | Compatible cutover, v1/v2 recovery, retention and honest revocation/uncertainty. |
| [11 — Testing](11-testing.md) | Exact executed evidence and provider/host/release qualification. |
| [12 — Governance](12-governance.md) | Develop/main process, reviews, current ADRs and historical provenance. |
| [13 — SDKs/integrations](13-sdks-integrations.md) | Vault clients, MCP/OAuth, native candidates and future extension contracts. |

The [implementation checkpoint](../design/2026-10-02-harness-neutral-platform/README.md),
[setup](../design/2026-10-02-harness-neutral-platform/SETUP.md),
[protocol receipt](../design/2026-10-02-harness-neutral-platform/PROTOCOL.md) and
[current acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
record dated source/test scope and pending gates. Earlier core/audit/design
records remain historical evidence, not fresh external qualification.

New platform guarantees are PostgreSQL-only and new flags default off. Ordinary
vault reads return permitted values; controlled tools keep GitHub credentials
inside the trusted broker. Skills/native configuration do not grant permission
or attest a device. Real provider/mail/native harness/isolation acceptance and
full operational drills remain required. The static landing stays at
`keepsave.draveniq.dev`; the same-origin full application at
`app.keepsave.draveniq.dev` is a deployment target.

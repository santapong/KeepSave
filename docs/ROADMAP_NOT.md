# What We Are Not Building (PM interim — Phase A)

The hardest PM discipline is saying no — out loud, in writing, with a reason. This is the explicit "not building" list for Phase A and the first half of Phase B. Treat it as a contract: anything on this list moving onto the roadmap requires (a) a written reason and (b) something else getting pushed off the roadmap to make room.

PM is not yet hired in Phase A; interim owner is the Tech Lead. Updated at the monthly review (`docs/ROLES.md` §6).

---

## Hard "no" for Phase A

These either expand scope beyond MVP hardening or commit us to capabilities we cannot reverse cheaply.

### 1. Multi-tenant runtime
- **What we are not building:** isolation primitives that separate one customer's data from another at the process / network level.
- **Why:** Phase B scope. Building it during Phase A means we cannot finish hardening the single-tenant case, which is the prerequisite for trusting any multi-tenant design.
- **Trigger to revisit:** start of Phase B (after Phase A exit criteria are met).
- **Exception:** schema preparation (additive migrations only) is allowed in Phase A — see Backend 90-day in `docs/ROLES_30_60_90.md`.

### 2. SSO / SAML / OIDC for end-user auth
- **What we are not building:** federated login for dashboard users.
- **Why:** no customer has asked. Building it before the demand signal arrives means we'll build the wrong shape.
- **Trigger to revisit:** when two customers ask in the same quarter, or when one enterprise prospect makes it a deal-blocker.

### 3. Mobile or native SDK
- **What we are not building:** iOS / Android SDKs, Electron app, native desktop client.
- **Why:** the embed widget covers the web surface. The cost of a native SDK is in maintenance (security advisories, OS-version drift, app-store distribution), not in initial implementation.
- **Trigger to revisit:** a customer with a mobile-only deployment use case who can't use the web widget.

### 4. GUI policy editor
- **What we are not building:** a dashboard UI for editing promotion / approval / scope policies.
- **Why:** policies today are code-driven (per ADR-0003). The current customer set is technical enough to manage policies as code. A GUI editor is high implementation cost and low value until non-technical customers exist.
- **Trigger to revisit:** non-technical customer admin role becomes a buyer.

### 5. Internationalization
- **What we are not building:** translated UI strings, locale-aware date/number formats, RTL layout.
- **Why:** all current customers use English. Premature i18n is one of the most reversible decisions to defer — adding it later is straightforward; building it now consumes time better spent on hardening.
- **Trigger to revisit:** non-English-locale customer signs.

### 6. Public marketplace for integrations / MCP servers
- **What we are not building:** a registry where third parties publish KeepSave integrations.
- **Why:** the trust model for community-published integrations is hard, and we have no signature scheme yet. Phase B at earliest.
- **Trigger to revisit:** after Phase B multi-tenant lands; signed-manifest design exists.

### 7. Built-in secrets generator / password manager features
- **What we are not building:** generating credentials, suggesting strong passwords, browser-extension autofill.
- **Why:** scope creep into adjacent product space (1Password / Bitwarden territory). We store secrets; we don't generate them.
- **Trigger to revisit:** never (deliberate non-goal).

## Soft "no" for Phase A (revisit at Phase B planning)

Less absolute — would not refuse a small spike if a clear win emerged, but no scheduled work.

- **Audit-log streaming to SIEM** (Splunk, Datadog, etc.). Customer signal exists; defer because the audit log itself isn't complete (see `docs/AUDIT_LOG_COVERAGE.md`).
- **Compliance certifications** (SOC 2, ISO 27001). Process work that requires the engineering substrate built in Phase A first.
- **Self-service tier / signup flow.** Manual onboarding is fine at current scale.
- **Multi-region deployment.** No customer requires it yet.

## Explicit non-goals (the *never*)

These are not deferrals. They are deliberate choices about what KeepSave will not be.

- **A general-purpose database for arbitrary structured data.** We store secrets. Don't accumulate features that turn this into "a config service that also does secrets."
- **A code-deployment tool.** Promotion of secrets is in scope; promotion of code is not.
- **A vendor-independent key-management abstraction layer.** The `MasterKeyProvider` interface is intentionally thin — enough to swap KMS vendors, not enough to be a competitor to vendor-native key management.
- **A logging or observability platform.** We emit audit events for our own actions. We are not building general-purpose log aggregation.

## How to propose an exception

1. Open an issue with title `roadmap-exception: <thing>`.
2. State the customer signal or technical forcing function. "It would be nice" is not a signal.
3. State what comes *off* the roadmap to make room. Time is finite.
4. Tech Lead + Security (if security-adjacent) sign off, or it's closed.

A merged exception is documented at the bottom of this file (with date and reason) so the next PM has the history.

## Granted exceptions

(none yet)

## References

- `Roadmap.md` (the *yes* list)
- `docs/ROLES_30_60_90.md` (Phase A action plan)
- `docs/ROLES.md` §4 (phase composition)

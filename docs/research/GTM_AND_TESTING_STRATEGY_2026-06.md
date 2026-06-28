# KeepSave — Go-To-Market & Testing Strategy (2026-06)

> **How this was produced:** the [`/workflow`](../../.claude/skills/workflow/SKILL.md) skill ran a loop-engineered research harness ([`docs/HARNESS_ENGINEERING.md`](../HARNESS_ENGINEERING.md)) — **37 agents · 16 research streams · 16 claims fact-checked**, in three phases (Research → Verify → Synthesize), opus for judgement/synthesis and sonnet for breadth-gathering. Research streams read the in-repo competitor dossiers (`docs/research/competitors/`) and the codebase, then web-searched for current data.
>
> **Classification:** Type-3 (research/strategy). No code changed by this document. Any recommendation touching crypto/auth/promotion/embed remains gated by the [ADLC](../ADLC.md).
>
> ⚠️ **Fact-check caveats (read first):** the verification phase flagged these claims — do **not** ship them in marketing assets:
> - **refuted** — _SEO / SEM / GEO_: AI-referred sessions grew 527% YoY; AI platforms generated 1.13 billion referral visits in June 2025 (Yotpo / Adobe Analytics, 2025) — cited in search results, 
> - **unverifiable** — _Robot Framework_: k
>
> One research stream (`rf_impl`) under-returned; the Robot Framework **example suite** is the one follow-up gap (the rest of §4 — feasibility, the standalone-vs-integrated decision, CI wiring — is complete). A ready-to-commit `tests/robot/` scaffold can be generated on request.

## Contents
1. [Executive Summary](#executive-summary)
2. [Monetization](#part-1--monetization)
3. [Marketing & Launch](#part-2--marketing--launch)
4. [SEO / SEM / GEO](#part-3--seo--sem--geo)
5. [Robot Framework](#part-4--robot-framework)
6. [Appendix — fact-check verdicts](#appendix--fact-check-verdicts)

---

<a id="executive-summary"></a>
Both load-bearing facts confirmed: the `data-value` plaintext leak in the embed widget (line 372: `dataset: { ..., value: secret.value || '' }`) and the hard Phase-A "no" on multi-tenant runtime (ROADMAP_NOT §1). I have everything needed. Writing the Executive Summary now.

---

# Executive Summary

KeepSave enters a real, fast-moving, and increasingly contested category — "the secrets layer for AI agents" — with a genuinely rare four-part bundle that no single incumbent ships together: an opinionated, approval-gated Alpha→UAT→PROD promotion pipeline, per-project/per-environment scoped agent keys, true single-tenant self-hostability (Go/Postgres), and an embeddable `<keepsave-widget>` Web Component. The strategy that wins is disciplined honesty: **lead relentlessly with what is verifiably shipped — scoped per-env keys, gated promotion with diff/rollback/kill-switch, DB-enforced approver-≠-requester separation of duties, hash-chained tamper-evident audit, and AES-256-GCM-at-rest — and frame everything else (default key expiry, short-lived tokens, N-of-M approval, SSO, SOC 2, hosted SaaS) as explicit roadmap, not current state.** Two findings gate the entire plan and are non-negotiable launch blockers: the flagship widget currently leaks every secret's plaintext into a `data-value` DOM attribute inside an *open* Shadow DOM (`frontend/src/embed/widget.ts:372`, AUDIT_2026-06-26 #1) and performs no inbound-origin check (wildcard `postMessage`), so the widget can be *named* as a differentiator but cannot be *sold as secure* until both land. Monetization should be open-core self-host plus a paid Enterprise license, metered on a hybrid value metric that prices AI agents an order of magnitude below incumbents — but it is blocked by a real product gap (`APIKey.ProjectID` is non-nullable, so agents are single-project today) and the absence of any billing or per-request metering substrate, while a hosted SaaS is structurally impossible at launch given the hard Phase-A "no" on multi-tenant runtime (`docs/ROADMAP_NOT.md` §1). On testing, adopt Robot Framework narrowly as the black-box acceptance tier above the existing Go/Seidr harness — viable precisely because the widget's shadow root is `mode: 'open'`.

## Prioritized GTM Roadmap

### 0–30 days — fix the blockers, build the foundation, validate the unknowns
- **Engineering gate (blocks the widget pillar):** fix the plaintext-in-DOM leak (AUDIT_2026-06-26 #1) and ship the strict-origin postMessage allowlist (FU 0b / ADR-0006). Until both land, the widget is foregrounded nowhere.
- **Land the launch substrate:** README opening with the AI-agent scenario, a tested `docker-compose` quickstart verified on a fresh machine, GitHub Discussions on, stars > 100, and confirm/stand up a marketing domain separate from the app (GEO has no on-site substrate without it).
- **Pick and hold one positioning sentence** ("the auditable, self-hostable secrets control plane for AI agents") everywhere; drop "env-var manager" entirely.
- **Correct every load-bearing stat before any asset ships:** GitGuardian **81%** (not 81.5%); 80:1 → **CyberArk**, 144:1 → **Entro**, 28.65M → **GitGuardian**; AI-referral = **Similarweb 1.13B/+357% YoY** and **Previsible 527%-over-5-months** (never Yotpo/Adobe); purge the Console.dev "68%", the comparison-page "40–60%", and the PATTERN_MATRIX-row-13 differentiation claim.
- **Run 3–5 design-partner conversations** to resolve the binding unknowns (beachhead runtime, trial-to-paid/ACV/motion, widget demand, licensing) before spending on paid or locking prices.

### 30–90 days — launch, capture bottom-of-funnel intent, instrument
- **Sequenced developer launch** (technical credibility → Show HN → Product Hunt → Reddit; newsletters in parallel) on the **promotion pipeline + scoped agent keys + self-host**, with the widget introduced only as "origin-pinned embedding, shipping now."
- **Ship the five comparison/"alternative" pages** (Doppler, Infisical, Vault, 1Password Connect, dotenv) in parallel — highest near-term ROI, purchase-stage intent — using only §1.5-corrected claims (DB-enforced separation of duties; hash-chained audit; **never "multi-party/N-of-M"**).
- **Own the zero-competition differentiator content:** "promotion pipeline / Alpha→UAT→PROD / approval gates," which no incumbent markets.
- **Paid search: Reddit-first and own-brand-only** until unit economics are proven; bid the near-zero-competition KeepSave-specific terms; wire offline-conversion tracking and a "How did you hear about us?" field on day 1; avoid Performance Max.
- **Pre-revenue monetization:** stand up the Enterprise-license + self-host motion (needs no multi-tenancy); design the tier table now, switch on only the self-host columns.

### 90–180 days — convert, expand, and unlock the paying ICPs
- **Sequence the engineering prerequisites for billing agents:** the multi-project **service-account identity** (Type-1, Security-veto) is the gating change; until it lands, meter agents per-project-key as a conservative interim proxy and price low. Build a crash-safe `CurrentRequests`/operations counter if consumption pricing is pursued.
- **Ship the roadmap items that convert Segment-1 design partners into Segment-3 (regulated-SMB) buyers:** default key expiry (FU 0k / ADR-0009), N-of-M PROD approval, audit-retention tiers, and the "what your auditor sees" proof asset.
- **Decide the strategic forks:** core licensing (permissive vs source-available — itself arguably Type-1), and whether a hosted multi-tenant SaaS is wanted at all (Phase-B, multi-tenant trigger, Security-veto) or KeepSave stays self-host-only on Enterprise license + support.
- **GEO measurement loop:** weekly ~30-prompt citation scoreboard across ChatGPT/Claude/Perplexity/Gemini; seed Reddit/GitHub/review profiles; one corrected entity line everywhere.

## Robot Framework Verdict

**Adopt it — but narrowly: Robot Framework belongs *integrated* in-repo at `tests/robot/` as the thin black-box acceptance tier above the existing Go/Seidr harness, the single strongest reason being that the "standalone" property KeepSave actually values is *process* isolation (already achieved by a dedicated `docker-compose.e2e.yml` + pinned `requirements.txt`, exactly as Seidr isolates via its own `go.mod`), while a separate repo would break atomic API↔test PRs and duplicate the SHA-pinned CI governance for zero real isolation benefit.**

## Top 5 Risks / Unknowns

1. **The embed-widget plaintext leak + missing origin allowlist (AUDIT_2026-06-26 #1, FU 0b/ADR-0006).** The flagship differentiator is currently a security liability; a Show HN audience will read `frontend/src/embed/` within the hour. Launching with the widget foregrounded before these land invites a top-comment teardown of the exact feature the positioning rests on. *Highest-severity, fully in-repo-verified, and blocking.*
2. **Per-agent metering is blocked by `APIKey.ProjectID` being non-nullable (`models.go:58`) and the total absence of billing/operations-counter substrate.** The dedup'd "active agent identity" value metric is *unimplementable* as designed until the multi-project service-account identity (Type-1, Security-veto) ships; the "counters already are the metering substrate" claim holds only for seats/keys/secrets, not operations.
3. **Unknown unit economics gate the whole monetization model.** No KeepSave cost/CAC/ACV/trial-to-paid data exists in-repo; every dollar figure is a June-2026 competitor benchmark, not KeepSave-derived. Below ~5% trial-to-paid, the proposed paid-search budget tiers are uneconomic, and the ~$3–5/agent-key overage is unvalidated against real per-agent storage+crypto cost.
4. **Unvalidated demand for the beachhead and the widget.** No documented external signal confirms *which* agent runtime to land first (MCP servers vs Claude-Code harnesses vs LangChain/CrewAI) or whether an embeddable secrets widget is a white space or a feature looking for a market — both must be validated with ≥2–3 real prospects before committing GTM motion.
5. **Strategic governance forks that constrain the plan and are not yet decided.** Hosted SaaS is structurally impossible at launch (hard Phase-A "no" on multi-tenant runtime, `ROADMAP_NOT.md` §1), SSO/SOC 2 are deferred (Team tier can't fully launch its anchor feature), and the core's open-source license — which the entire open-core and comparison-page narrative depends on — is unstated and arguably a Type-1 decision. *Lower-confidence on timing; all data sourced from competitors should be re-verified against live vendor pages before customer-facing use.*

---

<a id="part-1--monetization"></a>
# Part 1 — Monetization

Verified the load-bearing repo facts:

1. **Quota substrate exists** (`quota.go`) but `UsageQuota` has **no `CurrentRequests` counter** — only `CurrentSecrets/Projects/APIKeys`. So the per-request/per-operation metering the research assumes is *not* fully backed; the request-per-day limit has a max but no current-count field. This refines the "substrate already exists" claim — I'll flag it.
2. **No billing code** — grep confirms zero Stripe/billing/invoice/entitlement/subscription matches (the one hit is `scope_grammar_test.go`, unrelated). The "metering→billing pipe is the missing piece" claim holds.
3. **`APIKey.ProjectID` is non-nullable** (`models.go:58`, `ProjectID uuid.UUID` — not a pointer), confirming the hard dependency: agent identities are single-project today, blocking the dedup'd "active agent identity" meter.

Now I have everything needed. Writing the Monetization section.

Note: per my operating constraints I return the section content directly as my final message rather than writing a report `.md` file.

---

## Monetization

KeepSave's monetization strategy is constrained on one side by its own governance (multi-tenant runtime is a hard Phase-A "no", self-service signup a soft "no" — `docs/ROADMAP_NOT.md` §1, §55) and pulled on the other by where the secrets-management market is moving (away from per-human-seat, toward per-identity and consumption pricing). The recommendation below resolves that tension: **lead with open-core self-host + a paid Enterprise license, meter on a hybrid value metric that prices AI agents cheaply, and design — but do not yet launch — a hosted SaaS tier table.** Every dollar figure here is benchmarked to June-2026 competitors, not derived from KeepSave economics (none exist in-repo), and is flagged accordingly.

### 1. Headline recommendation

| Decision | Recommendation | Confidence |
|---|---|---|
| **Business model** | Open-core: source-available/permissive self-host core (free) + paid self-host **Enterprise license** for governance/compliance/SSO/support. Hosted SaaS is a Phase-B+ overlay, gated on the multi-tenant trigger. | High (model logic); the launch-vs-Phase-B sequencing is forced by `ROADMAP_NOT.md`, not a judgment call |
| **Value metric** | **Hybrid**: human seats + a *pooled, deduplicated* allotment of active agent/machine API keys (cheap metered overage) + governance features as the tier-gating axis. | High (strategic logic); Medium on exact prices |
| **Primary monetization wedge** | Per-agent metering priced an order of magnitude below incumbents — the concrete expression of the "secrets layer for AI agents" angle. | Medium (depends on unit economics KeepSave hasn't published) |
| **What never gets gated** | Encryption-at-rest, never-surface-plaintext, per-project envelope keys. These are trust primitives, not features. | High |

Two findings make the open-core-first stance non-negotiable rather than preferred. First, a hosted multi-tenant SaaS *cannot be the launch model*: `docs/ROADMAP_NOT.md` §1 makes multi-tenant runtime a hard Phase-A "no" (only additive schema prep allowed), and you cannot bill a SaaS whose isolation you have explicitly declined to build. Second, the product already *is* self-hostable and single-tenant (docker-compose/Helm), which is exactly the shape open-core fits — and the shape Infisical itself uses (MIT core + commercial Enterprise license). The repo confirms the GTM-infrastructure gap directly: a grep across `backend/` for `stripe|billing|invoice|usage_record|entitlement|plan_id|subscription` returns **zero** payment-integration code. The binding near-term constraint is therefore *missing billing infrastructure*, not cannibalization.

### 2. The value metric is the defining choice — and the AI-agent angle forces a specific answer

The single most important monetization decision is the value metric, and the two reference competitors sit at opposite poles:

- **Doppler** charges per **human** seat (Team **$21/user/mo**) and explicitly does **not** charge for service accounts or machine identities. Good developer experience, but it captures *zero* of the agent value KeepSave is built around.
- **Infisical** charges per **identity** (Pro **$18/identity/mo**) where a machine identity counts the same as a human. It captures agent value but punishes exactly the agent-heavy fleets that are KeepSave's ICP.

Neither pole works for "the secrets layer for AI agents." Pure per-human-seat leaves the agent value on the table; pure per-identity triggers the documented backlash where buyers "feel they're paying for your architecture, not their value."

**Recommended answer: a hybrid value metric** = (human seats) + (a pooled, *deduplicated* allotment of active agent/machine API keys, with cheap metered overage) + governance surface (promotion pipeline depth, approvals, SSO, audit retention, embed widget) as the tier-gating axis. Deduplicate à la Akeyless's published rule — *multiple instances of the same application count as a single client* — so scaling an agent fleet horizontally does not inflate the bill. This satisfies the three standard value-metric criteria: it correlates with value (more agents under management = more value), it is visible to the buyer (they know their agent count), and it is trackable (KeepSave issues the `ks_` keys).

The market direction supports moving off pure per-seat, though the supporting macro statistics are vendor-attributed and should be cited as such:

> **Low-confidence / attribution flags.** The IDC "≈70% of vendors move off pure per-seat by 2028" forecast, the Bessemer "hybrid pricing rose 27%→41% / per-seat fell 21%→15% in 12 months" figures, the "≥40% of enterprise SaaS spend shifts to usage/agent/outcome by 2030" Gartner projection, and the non-human-identity ratios (45:1 enterprise-wide, 144:1 cloud-native, up from 92:1) and NHI market sizing ($9.45B→$18.71B by 2030) are all single-analyst or vendor-attributed numbers that this report's research gathered from secondary summaries, not primary reports. Treat the *direction* (per-seat is declining; hybrid/consumption is rising; non-human identities are proliferating) as well-supported and the *precise figures* as illustrative, not load-bearing.

### 3. Competitor pricing landscape (the anchors)

The value metric divides the field into four mental models, each with a different pain point at scale. KeepSave's self-hosted model sidesteps all four meters in exchange for an ops burden.

| Vendor | Primary meter | Free tier | Pain point at scale | Source confidence |
|---|---|---|---|---|
| **Doppler** | Per **human** seat — Team ~$21/user/mo | 3 users, 3-day audit log | Captures no agent value; SSO at Team, but Change Requests/EKM/dynamic secrets/SCIM are Enterprise | Medium (see flag below) |
| **Infisical** | Per **identity** (human *or* machine) — Pro $18/identity/mo | 5 identities (humans+machines) | Machine-identity inflation: 5 engineers + 20 machine identities = $450/mo | High on the $18 figure |
| **Akeyless** | Per **client** (consumption, dedup'd) — Business/Enterprise unpublished | 5 clients, 500 secrets | Opaque pricing; enterprise-only positioning; DFC zero-knowledge is a stronger vendor-trust claim than Doppler/Infisical | Medium |
| **HCP Vault Dedicated** | Cluster-hour + per authenticated client | Dev cluster ~$22/mo, no SLA | ~$1,150/mo floor before the first secret; **$72.92/client/mo** per agent identity | High |
| **AWS Secrets Manager** | $0.40/secret/mo + $0.05/10K API calls | $200 AWS credits (post-Jul 2025 accounts) | API-call meter explodes for uncached microservice fleets | High |
| **GCP Secret Manager** | $0.06/active-version/mo + $0.03/10K ops | 6 versions + 10K ops/mo | Version-count billing penalizes rotation history; immutable replication policy | High |
| **1Password Secrets Automation** | Per seat — Business $7.99/user/mo (annual) | n/a (bundled) | Hard 50K-requests/day account-wide cap you cannot pay through on Business | Medium |
| **EnvKey** | **Dead** — cloud shut down Feb 1 2025 | n/a | Community open-source only; unmaintained | High |

> **Fact-check flags on the pricing anchors (do not paper over these):**
> - **Doppler Team at $21/user/mo is potentially stale.** The fact-check rated this only *partly* supported: $21 was accurate as of late 2024 (G2 capture, Doppler's own student-program page), but multiple 2026 aggregators (Vendr, EnvManager, CyberSecTool) now cite **$12/user/mo annual / $14/user/mo monthly**. Doppler's canonical pricing page returns HTTP 403 and could not be directly verified. **Re-verify against the live page before using $21 in any customer-facing material.** The SAML-SSO-at-Team half of the claim *is* well-supported.
> - **Doppler "Change Requests at Team" is incomplete.** Basic Change Requests are now on Team, but **Change Request *Policies*** (enforced approver counts) remain Enterprise-only. Earlier research that placed all Change Requests at Enterprise is outdated; the precise line is: basic feature on Team, policy enforcement on Enterprise.
> - **Infisical's "$18/identity, machine == human" is supported; the quoted phrase "scales with infrastructure, not headcount" is not attributable to Infisical.** It appears to be analyst paraphrase, not Infisical's own copy. Use the *mechanism* (machine identities billed like humans), not the quote.
> - **HCP Vault Secrets EOL is more nuanced than "July 1 2026."** End-of-sale was June 30 2025; **pay-as-you-go customers hit EOL August 27 2025** (already passed); July 1 2026 is the *Flex-contract* EOL only. The former per-secret SaaS ($0.50/secret/mo, 25-secret free tier) no longer exists in the HashiCorp portfolio — but the single-date framing collapses two distinct deadlines.
> - **Akeyless Business/Enterprise rates and the "$400–500/mo moderate traffic" signal are unpublished/unverified** — do not use the dollar estimate in customer materials.
> - **1Password vault-access-credit $/credit pricing above the 3-credit free tier is not public** (page returned 403; community sources confirm the model, not the rate).

The clearest takeaway for positioning: **EnvKey is a dead competitor** — model nothing against it, but cite it in win/loss as "previously a self-hostable alternative, now unmaintained" to lower switching-cost fear for prospects migrating off it. And **HCP Vault Secrets refugees** are a live opportunity: that product's EOL left price-sensitive mid-market customers choosing between self-hosting Vault Community (ops burden) and $1,150+/mo Vault Dedicated, with nothing from HashiCorp in the $0–$200/mo band — exactly where KeepSave's self-host model lands.

### 4. Proposed tier table

Designed now; the SaaS columns switch on when the Phase-B multi-tenant trigger fires. Self-host columns are live near-term. **All dollar figures are Medium-confidence benchmarks, not KeepSave-derived.**

| | **Free** | **Pro** | **Team** (revenue center) | **Enterprise** |
|---|---|---|---|---|
| **Price** | $0 (self-host or future hosted) | ~$25/dev/mo *or* per-node self-host license | ~$22/user/mo + agent-key blocks | Custom + self-host Enterprise license |
| **Projects** | 1 | Unlimited | Unlimited | Unlimited |
| **Promotion pipeline** | Alpha + UAT only (PROD = paid) | Full Alpha→UAT→PROD | Full + multi-party N-of-M approvals | Full + change-requests on secret edits |
| **Human seats** | 3 | Unlimited devs | Per-seat | Per-seat |
| **Agent/machine keys** | 5 pooled | 25 included, then metered overage | 100+ included | Committed / unlimited capacity |
| **Audit retention** | 3-day | 30-day | 90-day | Unlimited + SIEM streaming + tamper-evident chain |
| **SSO** | — | — | **Baseline SAML/OIDC** | + SCIM, advanced directory sync, enforced MFA |
| **Embed widget** | None (or 1 localhost/dev domain) | 1 production domain | Multi-domain | Unlimited + white-label / custom CSP |
| **Crypto / keys** | Per-project envelope (always) | Same | Same | + BYOK / customer-managed KMS |
| **Support** | Community | Community/email | Business | Dedicated + SLA |

### 5. What to gate, and what never to gate

**Gate the promotion pipeline, not the encryption core.** KeepSave's differentiated value versus flat secret stores (Doppler and Infisical both have flat environment models; promotion is an explicit copy) is the opinionated Alpha→UAT→PROD pipeline with approvals and audit. That is where enterprise willingness-to-pay concentrates, so it is the right axis to gate:

- **DO gate:** pipeline depth (PROD promotion is a paid capability), promotions/month, single-approver (Pro) vs. multi-party N-of-M (Team/Enterprise), audit-retention ladder (3d → 30d → 90d → unlimited), agent-key pool + overage, embed-widget domains, and Enterprise-only items (BYOK/KMS, SCIM, tamper-evident hash-chained audit, change-requests, SLA).
- **DO NOT gate:** never-surface-plaintext, encryption-at-rest, per-project envelope keys. Gating a trust primitive signals "security-as-upsell" and breaks the core promise.

**Put baseline SSO at Team, not Enterprise-only.** Both reference competitors gate SAML above free (Doppler Team, Infisical Pro), so SSO *is* a legitimate paid line — but gating it Enterprise-only (custom-quote) is what the `sso.tax` "Wall of Shame" criticizes as selling baseline security as a luxury. Reserve only SCIM / advanced directory sync / enforced MFA for Enterprise.

> **Two caveats specific to KeepSave that the gating plan must respect:**
> - **SSO does not exist yet.** `SSOConfig` is a model stub (`models.go:277-290`), and per `ROADMAP_NOT.md` §2, dashboard SSO/OIDC is a hard Phase-A "no" until the trigger fires ("two customers ask in the same quarter, or one enterprise prospect makes it a deal-blocker"). The *pricing line is ready; the feature ships when that demand signal arrives.* Until then, the Team tier cannot fully launch its anchor feature.
> - **The audit log isn't complete, and SIEM streaming is a soft-no** (`ROADMAP_NOT.md` §53). The unlimited-retention/SIEM/tamper-evident Enterprise line depends on `docs/AUDIT_LOG_COVERAGE.md` work landing and on the hash-chained audit candidate (`BEYOND.md` §2.6, a Phase-B/Type-1 item). The tamper-evident chain is a genuine Enterprise-exclusive differentiator for regulated/forensic buyers — but it is unbuilt today.

### 6. Price agent keys cheap — this is the headline of the AI-agent pitch

The wedge only works if per-agent economics beat the alternatives. Reference points (June 2026): Infisical **$18/identity/mo**, HCP Vault Dedicated **$72.92/client/mo**, Akeyless from ~$40/mo for 4 clients. **Recommendation: bundle a generous agent-key pool in each paid tier and price overage in the low single digits (~$3–5/key/mo)** — an order of magnitude under Vault Dedicated and well under Infisical. The message: *run a fleet of scoped, auditable agents without per-identity sticker shock.*

Pair "cheap" with "safe" so competitors charging more can't match the story on self-host: default expiration on `ks_` keys (ADR-0009 / FU 0k — the repo confirms this is **not yet shipped**: `validation.go:33-38`, `apikey_service.go`, `apikey_repo.go` omit `expires_at` at issuance, though middleware already honors it) and per-secret scopes (`BEYOND.md` §2.1, Phase B). The pricing calculator that makes this land should be a public, repeatable sales asset anchored on three deterministic scenarios — (1) 100 secrets / 1M calls/mo, (2) 1,000 secrets / 10M calls/mo, (3) 10,000 secrets / 100M calls/mo — where every competitor's cost is computable and KeepSave's infra cost is a small range (~$80–400/mo of self-hosted Postgres + compute).

### 7. The execution risk that gates the whole model

**Per-agent metering is blocked by a real product gap.** The repo confirms `APIKey.ProjectID` is a non-nullable `uuid.UUID` (`models.go:58`), so one agent needing access across N projects requires N separate `ks_` keys today. That makes a naive key-count meter inflate non-linearly and makes the dedup'd "active agent identity" metric *unimplementable* as designed. The workplace-scoped, multi-project **service-account identity** (the Doppler/1Password shape) is the required primitive — and it is a Type-1 change under Security-Engineer veto, currently a Phase-B "adopt-when-trigger-fires" item (Pattern Matrix rows 5/14). **Sequence the service-account identity as the gating engineering prerequisite for billing agents; until it lands, meter agents per-project-key as a conservative interim proxy and price low.** This is the single biggest execution risk to the proposed value metric.

A second, narrower substrate gap: the research claims the per-org quota counters are "already the metering substrate." Partly true. The repo shows `UsageQuota` (`quota.go`) tracks `CurrentSecrets/Projects/APIKeys` against maxes — but it has **no `CurrentRequests` counter** (only a `MaxRequestsPerDay` ceiling). So per-operation/per-call metering, which the consumption story leans on, is *not* backed by an existing counter and would need to be built to metering-grade accuracy (idempotent, crash-safe, no double-count) — a higher bar than quota enforcement. The "counters exist, only the billing pipe is missing" framing holds for *seats/keys/secrets*, not yet for *operations*.

### 8. Sequencing and secondary levers

- **Hosted SaaS goes last, not first.** Self-host→cloud conversion is real but blocked by the Phase-A hard "no" on multi-tenancy and soft "no" on self-serve signup; multi-tenancy is itself a Type-1, Security-veto decision. Monetize single-tenant self-host (per-identity metering + Enterprise license need no multi-tenancy) now; treat managed cloud as the Phase-B conversion target. Expect low-volume/high-value conversion (open-core community→cloud conversion is typically a low single-digit percentage), so design for high-value conversions + expansion, not volume.
- **Embed widget as a dual PLG + upsell channel.** The `<keepsave-widget>` (Shadow DOM) is a distribution surface no secrets competitor ships — the secrets-management analog of Stripe/Clerk embedded components. Gate it by domains/seats (Free: none/localhost → Pro: 1 prod domain → Team: multi-domain → Enterprise: white-label). **Hard prerequisite: ship the strict-origin postMessage allowlist (ADR-0006 / FU 0b) before marketing it** — the repo confirms the widget currently has *no* origin check and uses wildcard `*` (`embed/auth.ts:21-26,:33`, Pattern Matrix row 13). Marketed before that lands, it is a security liability, not a growth lever.
- **Defend on the combination, not the wedge.** The agent-secrets angle is validated by demand but increasingly contested — Infisical shipped Agent Vault + MCP governance, 1Password "Unified Access", CyberArk "Secure AI Agents", and the field is converging on SPIFFE/SPIRE workload identity (which KeepSave has only as unbuilt `BEYOND.md` §2.5 candidates). Do not out-spend incumbents on raw agent-identity. The defensible bundle is **promotion pipeline + embeddability + self-hostable-and-auditable** together — no single competitor combines all three.
- **Avoid the "AI tax / pay-twice" failure mode.** Pick consumption *or* seats as the primary axis (recommend the agent-key/consumption axis); do not bolt an agent meter on top of full per-seat pricing, which is the documented source of renewal friction.

### 9. Before committing prices — open validation work

The dollar figures in this section are benchmarked to June-2026 competitors, **not derived from KeepSave economics**. Before any pricing launch:

1. **Run a unit-economics + willingness-to-pay pass.** No KeepSave cost/CAC/segment data exists in-repo; the ~$3–5/agent-key overage in particular needs validation against real per-agent storage + crypto cost.
2. **Re-verify the Doppler anchor** ($21 vs. $12/$14) against the live pricing page — it gates the Team-tier comparison.
3. **Confirm the licensing model of KeepSave's own core** (permissive MIT/Apache like Infisical, vs. source-available/BSL like Sentinel). Open-core cannibalization logic only applies if the core is openly licensed, and this choice is itself arguably a Type-1 governance decision affecting the competitive moat.
4. **Decide whether a hosted SaaS is wanted at all,** or whether KeepSave stays self-host-only and monetizes purely via Enterprise license + support — the answer changes how much of the SaaS tier table is ever switched on.

---

**Files referenced (all absolute):** `/home/user/KeepSave/docs/ROADMAP_NOT.md`, `/home/user/KeepSave/docs/research/BEYOND.md`, `/home/user/KeepSave/docs/research/PATTERN_MATRIX.md`, `/home/user/KeepSave/backend/internal/models/quota.go`, `/home/user/KeepSave/backend/internal/models/models.go` (APIKey.ProjectID at line 58; SSOConfig stub 277-290).

---

<a id="part-2--marketing--launch"></a>
# Part 2 — Marketing & Launch

I now have all the load-bearing facts verified directly against the repo. One note: the audit cites the plaintext finding at `frontend/src/embed/widget.ts:367-382`, while the competitive research cited `keepsave-widget.ts` — there are two embed files (`widget.ts` and `keepsave-widget.ts`); the audit's path is authoritative. I have what I need.

Here is the **Marketing & Launch** section.

---

# Marketing & Launch

This section converts KeepSave's positioning research and launch playbooks into a decisive plan: what to say, who to say it to, where to launch, in what order — and, critically, **what not to claim**. KeepSave is entering a real and fast-moving category ("the secrets layer for AI agents") with a genuinely rare differentiator (an embeddable, self-hostable, promotion-gated secrets control plane) and a set of partially-built capabilities that a security-savvy buyer will probe in the first POC. The discipline that determines whether this launch succeeds is **leading with what is shipped and framing the rest as roadmap.** Several flagged items below are not optional caveats — they are launch blockers.

> **Launch-blocker callout (read first).** The flagship differentiator — the embeddable `<keepsave-widget>` — currently **leaks every secret's plaintext into a `data-value` attribute inside an open Shadow DOM** (adversarially-verified, `frontend/src/embed/widget.ts:367-382`, AUDIT_2026-06-26 Finding #1; classed Type-2, Security veto). Marketing the widget as "secure secrets in your product" before this is fixed would be dishonest and reputationally fatal. The widget can be *named* as a differentiator; it cannot be *sold as secure* until this and the origin-allowlist work (FU 0b / ADR-0006) land. Everything in this section that touches the widget is written to that constraint.

---

## 1. The one-sentence positioning (ship this verbatim everywhere)

Every channel leads with the same claim, adapted only to character limits:

> **KeepSave is the auditable, self-hostable secrets control plane for AI agents — scoped access, gated Alpha→UAT→PROD promotion, embeddable anywhere.**

This is the intersection of three 2026 tailwinds — AI-agent/non-human-identity security, self-hosting, and the post-Vault-license-change churn in secrets management — and it foregrounds the four-part bundle no single incumbent ships together: (1) opinionated promotion pipeline with approval + audit, (2) per-project/per-environment scoped agent keys, (3) genuinely self-hostable single-tenant Go/Postgres, and (4) an embeddable Web Component. The in-repo competitive memo states plainly that "no incumbent ships all four with the same opinionated promotion-pipeline shape" (`docs/research/BEYOND.md` §1), and independent web search found **no dedicated embeddable/white-label secrets-manager widget** among the 13 researched competitors.

**Category framing — decision:**

| Frame | Verdict | Reason |
|---|---|---|
| "Env-var manager" | **Drop entirely** | Smallest TAM; anchors KeepSave to the indie/commodity tier; invites a DX comparison (CLI, integrations, SDK breadth) it loses to Infisical/Doppler. |
| "Secrets manager" | **Secondary, SEO-only** | Accurate but undifferentiated; head-to-head with Vault/Doppler/Infisical on maturity, where KeepSave is behind. Use only as a discovery keyword. |
| **"Secrets control plane / layer for AI agents"** | **Primary** | Rides the NHI wave, narrows the comparison set to a young, contested field, and is the only framing where KeepSave is *first* rather than *behind*. |

**Honest framing note:** the category is contested, not greenfield. Infisical launched "Agent Vault" (Apr 22, 2026) and rebranded its homepage to "the modern security platform for developers and agents"; Phase.dev pitches "teams and AI agents" almost identically. KeepSave differentiates on **architecture and layer**, not on naming the category first (see §5).

---

## 2. The market narrative (the numbers, attributed correctly)

The category is real and the data is strong — but the fact-check flagged **attribution errors** in the source research that must be corrected before any of these numbers appear in a deck, blog post, or ad. **Use the table below, not the raw research phrasing.**

| Stat | Correct attribution | Fact-check status |
|---|---|---|
| ~**80:1** machine-to-human identity ratio | **CyberArk** *2025 State of Machine Identity Security Report* (relayed by KPMG's Jan 2026 "Invisible Access, Visible Risk"). **Do not attribute to KPMG as their finding.** | Partly supported — number real, **source was misattributed to KPMG in the research** |
| **144:1** in cloud-native (up from **92:1** in H1 2024) | **Entro Security** *NHI and Secrets Risk Report – H1 2025* (Jul 22, 2025). **Not a KPMG figure.** | Verified figure; **must be attributed to Entro, not KPMG** |
| NHI management as a top-8 CISO priority for 2026 | **KPMG** *Cybersecurity Considerations 2026* | Supported |
| **28.65M** (rounded to "~29M") new hardcoded secrets on public GitHub in 2025; **+34% YoY**; **AI-service secret leaks +81%** (research said 81.5%) | **GitGuardian** *State of Secrets Sprawl 2026* (Mar 17, 2026) | **Supported.** "29M" is GitGuardian's own headline rounding of 28.65M. Use **+81%** (the official press-release figure); 81.5% is a defensible secondary precision but the official report says 81%. |
| **8,000+** public MCP servers (492 with zero auth/encryption); **36.7%** of ~7,000 analyzed SSRF-vulnerable; NSA/DoD MCP security guidance, Jun 2026 | Mixed vendor scans (Trend Micro, BlueRock) + NSA/DoD CSI | Used as **Segment-1 urgency context**, not headline marketing. Cite the specific scan source per use; treat individual vendor-scan counts as **medium-confidence**. |

**One unsupported figure to drop:** the research repeated a claim that "**68% of Console.dev readers sign up for featured tools**." The fact-check rates this **unverifiable self-reported marketing copy with no disclosed methodology**, and the "verified via web search" attribution is **false**. Do not put this number in any KeepSave-facing asset. The defensible Console.dev facts (features ~2–3 developer tools/week; editorial, not pay-to-play) are fine to rely on operationally.

**Two analyst projections to label as projections, not facts:** "AI agent software spend $206.5B in 2026, +139% YoY" (n8n 2026 report) and "blogs with 400+ posts generate 4.2x more leads / posts drive traffic ~3.5 years" (digitalapplied.com). These are useful directional support; attribute them and mark them analyst/vendor projections, never audited figures.

---

## 3. ICP sequencing — who to land, in what order

The four segments are ranked by fit, and the GTM motion is **sequential, not parallel.** Lead with Segment 1 as design partners; expand into Segment 2 *through* the agent angle; treat Segment 3 as the Phase-B paying ICP that the roadmap unlocks; treat Segment 4 purely as top-of-funnel.

| # | Segment | Role in GTM | Core JTBD | KeepSave fit (shipped) | Gating roadmap |
|---|---|---|---|---|---|
| **1** | **AI-agent platform builders / MCP teams** (Seed–Series B, internal platform teams shipping agentic apps) | **Lead. Design-partner ICP, now.** | "Give my agent exactly the secrets it needs for one environment, prove every secret it touched, and rotate/revoke without redeploying — so a compromised or hallucinating agent can't drain my credential store." | Per-project/per-env scoped keys; promotion path for agent configs; access audit trail; self-host keeps secrets in the customer's trust boundary | Short-lived/audience-bound agent tokens (BEYOND §2.5, Phase-B); default key expiry (FU 0k / ADR-0009) |
| **2** | **Platform / DevOps / IDP teams** (50–500-eng orgs) | **Expansion path.** Enter *through* the agent angle: "you already trust us for agent secrets — here's the same control plane for your services." | "Right env vars per environment for every service and agent, an approval gate from staging to prod, an audit trail, self-hosted in our VPC." | Promotion + approvals + audit + self-host speak this team's language directly | **Do not lead here on "better env-var sync"** — KeepSave lacks secret-references-on-read, a first-party CLI, and dynamic secrets (see §5) |
| **3** | **Regulated SMBs** (20–200 ppl, HIPAA/SOC2 pressure, no Vault-ops team) | **Phase-B paying ICP.** Sell the substrate that enables *their* compliance — not KeepSave being certified. | "Immutable/exportable audit tying every access + promotion to an approver, self-hosted so regulated data never touches a vendor SaaS, without running Vault." | Single-tenant self-host; promotion approvals | **Honesty caveat:** KeepSave has **no SOC2/HIPAA certification** (Phase-B soft-no, `ROADMAP_NOT.md`); tamper-evident audit chain is a candidate, **not shipped** (BEYOND §2.6). Today's audit rows are plain. |
| **4** | **Indie hackers / <10-person teams** | **Top-of-funnel only.** OSS stars, GitHub credibility, inbound. **No roadmap built for it.** | "Stop committing secrets / emailing .env files." | Self-host + free is attractive | Most commoditized, lowest-WTP, highest-support-cost segment; KeepSave lacks the table-stakes indie DX (no first-party CLI — explicit non-goal). Court for stars; **never position around "env-var manager."** |

---

## 4. The three message pillars (with strict shipped-vs-roadmap discipline)

Each pillar pairs a claim with an explicit **USE today** vs **ROADMAP (do not assert as shipped)** split. The roadmap items are not hidden — they are framed as the 6–12 month path that converts Segment-1 design partners into Segment-3 paying customers. Overclaiming any of them is the failure mode that burns the design-partner relationships the whole motion depends on.

### Pillar 1 — "Scoped, auditable access for every agent"
*vs. static API keys / `.env` (no scope, no audit, no revoke-without-redeploy) and opaque brokers.*

- **USE today:** per-project, per-environment scoped keys (shipped, verified — `RequireProjectAccess` + `EnforceAPIKeyScope` are implemented and wired into the router as of the Waves 1–7 hardening, commit #63); audit-log on state-mutating actions (Phase A).
- **ROADMAP — label explicitly:** **default key expiration at issuance** (today `ks_` keys have **no `ExpiresAt` set at issuance** — they are indefinite; FU 0k / ADR-0009, *proposed, not landed*); short-lived audience-bound (SVID-shaped) tokens (BEYOND §2.5, Phase-B); per-secret/per-action scopes (Phase-B; today scopes are coarse).
- **Honesty guardrail:** a leaked `ks_` key today has broad, indefinite reach. **Lead with scoping + audit + self-host; frame expiry and short-lived tokens as the near-term roadmap, never as current state.**

### Pillar 2 — "Promotion you can trust: gated Alpha→UAT→PROD"
KeepSave's most genuinely opinionated, least-copied feature — the **only** framing where KeepSave is first, not behind.

- **USE today:** promotion endpoint + diff + rollback + a per-promotion audit row (Roadmap Phase 2); promotion **kill switch** (`KEEPSAVE_PROMOTIONS_ENABLED`, shipped 2026-06-09; `/promote` + `/approve` return 503 when off).
- **ROADMAP — label explicitly:** N-of-M / three-of-N PROD approval (today **single-approver**; Phase-B); **approver≠requester enforced at the DB layer** (today **service-code only, not DB-enforced** — ADR-0007 in-flight); **hash-chained tamper-evident audit** (BEYOND §2.6 — today audit rows are **plain**).
- **Honesty guardrail:** say **"full audit trail today, cryptographically tamper-evident on the roadmap."** Do not use the words "tamper-proof" or "tamper-evident" about the *current* log. Differentiation framing: Doppler does branch/inherit configs + Change Requests (Enterprise-tier, for edits — not a promotion gate); Infisical has env hierarchy + approval policies; Vault has Sentinel (Enterprise-only). The *Alpha→UAT→PROD promotion-as-copy with approval* is KeepSave's distinct shape.

### Pillar 3 — "Yours to run, yours to embed"
The single rarest thing KeepSave has.

- **USE today:** self-host via docker-compose/Helm, no SaaS dependency (Go/Postgres); `<keepsave-widget>` Web Component (Shadow DOM); SDKs for **Go, Python, and Node** (verified: `sdks/go`, `sdks/python`, `sdks/nodejs` — **not Python-only**; the earlier "Python-only" claim was a known research error, BACKLOG §4).
- **Honesty guardrail (critical):** the embed widget is **not production-secure today** — wildcard `postMessage('*')`, no `ev.origin` check on inbound auth (`docs/EMBED_ORIGIN_POLICY.md`), **and** the plaintext-in-DOM leak (AUDIT_2026-06-26 #1). Origin-allowlist fix is in-flight (FU 0b / ADR-0006 / PR #50). **Until both land, frame embeddability as "origin-pinned embedding shipping now" — never "secure by default."** This pillar uniquely serves the "platform builder who embeds/resells" motion, which is the rarest and highest-leverage wedge — but it is also the most-broken surface, so it is the **last** pillar to lean on publicly and the **first** to fix in engineering.

---

## 5. Competitive narrative & honest differentiation

KeepSave is **not** a better Doppler/Infisical/Vault, and the research is unambiguous that on every maturity axis those peers lead (third-party audits, multi-method machine identity, first-party CLIs, K8s operators, large integration ecosystems, ~10 SDKs vs KeepSave's 3). The narrative must **concede the maturity gap first, then counter on the wedge.** The wedge is *embed + on-prem + promotion-gate*, not "better secrets storage."

**Battlecards — concede-then-counter:**

| Competitor | Concede first (don't fight here) | Counter on (the wedge) |
|---|---|---|
| **Doppler** | SOC 2 Type II; mature disclosure pipeline; workplace-scoped Service Accounts (multi-project identity); large integrations layer; commercial scale | **Self-hostable** (Doppler is **cloud-only / cannot self-host** — decisive for regulated/sovereignty buyers); first-class approval-gated **promotion pipeline + kill switch**; **embeddable widget** (Doppler has none); inspectable source |
| **Infisical** (closest peer) | Native CLI + leak-scanner; Universal Auth multi-method identity (AWS/GCP/Azure/K8s/OIDC); dynamic secrets with external auto-revoke; K8s operator; SOC 2/HIPAA claims; far larger contributor base | Promotion-pipeline-with-approval as the **product centerpiece**; **embeddable widget** (match on self-host, **beat on embeddability**) |
| **HashiCorp Vault** | Deepest, most-audited security model in the category (Trail of Bits 2018); dynamic secret engines; CNCF-scale deployments; real CVE/disclosure track record | **Do not position as a Vault replacement** — the dossier states KeepSave *integrates* Vault as an upstream KMS. Counter on dramatically lower operational complexity + a built-in promotion UX + embeddable widget. |
| **Cloud-native SM** (AWS/GCP/Azure) | Zero ops; native IAM/KMS; inherited compliance; huge ecosystem | Cloud-portability / no lock-in; explicit reviewable promotion-with-approval (cloud SMs have versioning but **no promotion semantics**); embeddable UI. **⚠ Lowest-confidence column** — these vendors are P2/unresearched in-repo; **validate against current vendor docs before using in sales.** |

**The sharpest one-liner against the whole field:** *"The secrets layer agents promote through, not just read from."* Incumbents treat agent secrets as a flat KV store an agent **reads**; KeepSave's opinion is that agent secrets have a **lifecycle** (Alpha→UAT→PROD) needing a reviewable, audited, approver≠requester gate — and that this control surface should be **embeddable and self-hosted on the team's own infra.**

**Differentiating from Infisical Agent Vault — by architecture, not parity (do not claim to do what it does):** Agent Vault is a credential-injecting **outbound HTTP proxy** (the agent's traffic is routed through `HTTPS_PROXY`; the agent never holds the secret). KeepSave is a **scoped-read + promotion + audit** model (the agent holds a scoped, auditable, revocable key and reads what it's entitled to). These are **complementary**. Own *"the layer that governs and proves what agents do with secrets,"* not *"the layer that intercepts agent traffic."*

**DO-NOT-CLAIM list (guardrails for every asset, ad, and rep):** "more secure than Vault/Doppler/Infisical"; "enterprise-ready" / "production-proven"; "zero-trust" / "end-to-end encrypted in the browser" (the widget currently exposes plaintext in the DOM); "broad SDK coverage" (3 SDKs); "rich integrations" (none shipped; marketplace is a hard-no); "multi-method workload identity" (single `ks_` key today); "SOC 2 / HIPAA / FIPS" (no certifications); "tamper-proof audit log." **Safe to claim:** self-hostable; open/inspectable; opinionated promotion-with-approval; embeddable Web Component; AI-agent-scoped keys; AES-256-GCM envelope encryption (verified correct).

> **One correction to the source research, surfaced by the fact-check:** the competitive memo cited "PATTERN_MATRIX row 13" as proof that no competitor ships an embeddable widget. That is a **misread** — row 13 tracks *origin-validated embed/iframe auth* (a postMessage security property), not the presence/absence of a widget product. The "no competitor ships a comparable Web Component" claim is still **well-supported by independent web search**, but **cite the web-search result, not row 13.** Do not build a marketing claim on the row-13 reference.

---

## 6. Launch sequence — a 6-week, channel-by-channel plan

The plan is a sequence, not a simultaneous blast. Each community is given room to breathe; cross-posting the same link to multiple communities in the same hour reads as a marketing operation and suppresses organic engagement. Two independent playbooks (Infisical's documented 0→3,000-stars run and the general 2026 devtool-launch literature) converge on this ordering: **technical credibility first → Show HN → Product Hunt → Reddit, with newsletters seeded in parallel.**

### Pre-launch (Weeks −4 to −1)
- **Week −4:** Age a Product Hunt maker profile (new accounts get vote-filtered). Secure a **Lobsters invite** (invite-only; no workaround — start now). Publish **dev.to Article 1**: a pure technical deep-dive on the AES-256-GCM envelope-encryption architecture or the promotion-pipeline design — **no product pitch**, just engineering reasoning, under a real name. This is the single highest-leverage pre-launch action: it builds credibility + SEO and gives you something to link in HN/Reddit comments. Start a weekly "building in public" thread on X (one post/week, agent angle).
- **Week −2:** Submit to **Console.dev** for *editorial* consideration (3 weeks out; not paid — KeepSave qualifies on developer-as-user, self-service signup, fits-dev-workflow). If a paid newsletter budget exists (**≥$3K**), book a **TLDR DevSecOps/Cybersecurity** slot at `advertise.tldr.tech` (slots fill weeks ahead; TLDR's team writes the ad in their voice). If budget is **<$3K, skip paid sponsorship** and double down on Console.dev editorial + organic HN/Reddit. Also target **tl;dr sec** (security-focused; exact fit for the threat-model angle).
- **Week −1:** Finalize all channel assets (§7). Brief **3–5 authentic users/advisors** to leave genuine first-hour comments on HN and PH — **to comment, never to vote-bomb.** Confirm **GitHub stars > 100** before any public post (visible social proof on PH and the README; gates the whole sequence).

### Launch week (Week 0)
- **Day 1 (Tue/Wed/Thu only — never Fri–Sun):**
  - **12:01 AM Pacific** — submit on Product Hunt; maker comment goes live immediately and pins to top.
  - **8:00 AM Pacific** — post **Show HN** (title below). Stay online all day.
  - **10:00 AM Pacific** — r/selfhosted, "Show and Tell"/"New Release" flair.
  - **Noon Pacific** — LinkedIn founder story from a **personal profile** (≈8x the reach of a company page under the 2026 algorithm).
- **Day 2:** r/devops (reframed as a *secrets-pipeline problem*, not a self-hosted-tool post). **Do not post r/devops and r/selfhosted the same day** — mods share flagging signals across related subs.
- **Day 3:** r/programming **only if** you can frame it as a technical article linking to dev.to/GitHub (not the product). Publish **dev.to Article 2** (the launch tutorial).
- **Days 4–5:** Lobsters submission (only with an invite + real karma; link the HN thread in your first comment). X launch thread (10–12 tweets, problem → architecture → GIF demo → GitHub star CTA; pin it; repost the HN link in tweet ~8).

### Post-launch (Week +1)
- Reply to **every** still-active HN/Reddit thread. Publish a **"what we learned" retrospective** on dev.to + LinkedIn (often more-read than the launch post). **Ship a meaningful response to the top HN/GitHub feedback within 72 hours and post the PR link in the original HN thread** — this is the single move that converts launch traffic into sustained attention. **Do not re-post** to any sub/HN — one shot per launch.

**Channel-specific titles & framings (use exactly):**
- **Show HN:** `Show HN: KeepSave – self-hosted secrets + promotion pipeline for AI agents` — no exclamation mark, no superlatives; parenthetical URL points to the **GitHub repo or a runnable demo**, not a signup landing page.
- **r/selfhosted:** `KeepSave — self-hosted secrets + Alpha→UAT→PROD promotion pipeline [docker-compose, OSS]` (lead with the repo link; 3-line what/why/how; ask a specific feedback question).
- **r/devops:** `We built a promotion pipeline for secrets (Alpha→UAT→PROD) with approval gates — here's why and how [OSS]` (3-paragraph write-up including an **honest "what we haven't built yet"** — this sub rewards transparency).
- **Lobsters:** `KeepSave: self-hosted secrets with Alpha→UAT→PROD promotion pipeline [Go, OSS]`.

**Non-negotiable rules (all channels):** never solicit upvotes anywhere (HN guidelines explicitly prohibit it; PH strips votes from new accounts; Reddit bans coordinated voting). Never use AI-generated/AI-edited replies on HN or Lobsters — these communities detect it instantly and it destroys credibility. Never post identical text across channels. Never launch on PH Fri–Sun. Never pay for stars or upvotes. **Never launch before the README, docker-compose quickstart, and docs are complete and tested on a fresh machine** — the highest-risk moment is HN engineers running the tool in the first two hours and hitting a bug.

> **Launch-readiness gate (must be true before Day 1):** the embed-widget plaintext leak (AUDIT_2026-06-26 #1) and the origin-allowlist gap (FU 0b) are either **fixed** or **conspicuously absent from every launch asset.** A Show HN audience will read `frontend/src/embed/` within the hour. Launching with the widget foregrounded while that finding is open invites a top-comment teardown of the exact feature the positioning rests on. Recommended: **fix #1 + ship the origin allowlist first**, or launch on the **promotion pipeline + scoped agent keys + self-host** and introduce the widget as "origin-pinned embedding, shipping now."

---

## 7. Asset checklist (per channel)

| Channel | Required assets |
|---|---|
| **GitHub (gates everything)** | README that **opens with the AI-agent scenario** (not the architecture); tested docker-compose quickstart; architecture diagram; a 30–90s GIF of the promotion flow; "self-hosted in 5 minutes" section; **stars > 100** before launch; GitHub **Discussions** enabled (Google-indexed Q&A → long-tail SEO); submit to `awesome-selfhosted` + `opensourcealternative.to` (persistent referral, zero upkeep) |
| **Product Hunt** | Icon 240×240 (no text); 4–6 screenshots (dashboard, secret-create with env selector, promotion-approval UI, widget in an iframe, API-key scope assignment) + 60–90s demo video; tagline <60 chars (e.g. *"Self-hosted secrets + promotion pipeline for AI agents"*); description <260 chars (pain → solution → one differentiator); maker comment 200–400 words (origin story + hardest technical decision + concrete ask + a question); topics: Developer Tools, Security, Open Source, DevOps. **No superlatives** ("best/revolutionary/game-changing") anywhere. |
| **Hacker News** | Exact Show HN title (§6); first comment 300–500 words posted immediately (problem → one genuinely hard technical decision, e.g. per-project envelope keys vs a global master key → invite critique → end with *"happy to discuss the threat model or the AES-256-GCM key hierarchy"*) |
| **Reddit** | Three **separately written** posts (selfhosted/devops/programming — mods cross-check); each links GitHub or a dev.to article, never a marketing landing page |
| **dev.to** | Article 1 (Week −4, architecture deep-dive, no pitch); Article 2 (Week 0 Day 3, step-by-step tutorial: docker-compose → create project → secret in Alpha → promote to UAT with approval → issue a scoped agent key). Tags: `#security #devops #opensource #selfhosted` |
| **Lobsters** | Repo link + factual title + `security`/`devops`/`open-source` tags + maker first-comment linking the HN thread |
| **LinkedIn** | Four personal-profile posts (one/week): −2 the problem; −1 "why we built our own AES-256-GCM envelope vs Vault"; Day 0 launch + a genuine question; +1 "what we learned." Tags: `#DevSecOps #AIAgents #SelfHosted #SecretsManagement #OpenSource` |
| **X** | 10–12-tweet pinned launch thread (problem → architecture → **GIF of the promotion flow** → GitHub CTA, HN link in tweet ~8); one update/day for 5 days |
| **Newsletters** | Console.dev editorial submission (Week −2); optional TLDR sponsorship (≥$3K, booked 4–6 weeks out); tl;dr sec submission |
| **SDK READMEs** | Each of Go/Python/Node gets a **5-line quickstart that actually runs** + a copy-pasteable docker-compose snippet + a link to a "Security model" doc page. Time-to-first-value is the #1 retention driver. |

---

## 8. Two proof assets that close Segment 1

Build these before/at launch — they answer the JTBD and the 2026 MCP-security/NHI-governance anxiety directly:

1. **An MCP-server / Claude-Code-style quickstart** showing a scoped agent key reading exactly one environment's secrets, with the audit trail visible. Tie it to the LangChain/CrewAI/AutoGen and MCP context that the NSA/DoD Jun-2026 guidance made urgent.
2. **A one-page "what your auditor sees"** artifact showing the promotion + access audit trail end-to-end. This is the content equivalent of a SOC 2 summary, readable by a developer — and the bridge that converts a Segment-1 design partner into a Segment-3 paying customer.

---

## 9. Open questions to resolve before locking the GTM

These are unresolved in the research and should be answered with 3–5 design-partner conversations, not assumed:

- **Beachhead runtime:** MCP servers specifically, coding-agent harnesses (Claude Code), or LangChain/CrewAI multi-agent apps? No external demand signal is documented yet.
- **Widget demand:** is "embeddable secrets widget for SaaS integrators" a unique-and-valuable white space, or a feature looking for a market? No external signal surfaced — validate with ≥2 platform/ISV prospects who would actually embed it.
- **Pricing unit:** per-seat (Doppler ~$21/user/mo; Infisical Pro ~$8/user/mo with free self-host) doesn't fit an agent-centric model where the unit of value is agents/environments. An agent- or environment-based model may differentiate but is **unvalidated** — and these competitor prices should be re-verified against current vendor pages before appearing anywhere.
- **Segment-3 timeline:** is regulated-SMB a 6-month or 18-month ICP given zero certifications today, and does it eventually force a managed/cloud option (currently a multi-tenant non-goal)?
- **Licensing:** the comparison-page narrative ("Doppler is closed-source; Infisical is open-core") depends on KeepSave's own license being stated clearly. Confirm before publishing any "vs" page.

---

**Bottom line for Marketing & Launch.** KeepSave has a real category, a defensible four-part bundle, and a genuinely rare embeddable differentiator — and it has a flagship feature that currently leaks plaintext. The launch wins if it does three things relentlessly: **(1)** says the same agent-control-plane sentence everywhere and drops "env-var manager"; **(2)** leads with what is shipped — scoped per-env keys, gated promotion (promote/diff/rollback/kill-switch), self-host — and labels expiry, short-lived tokens, N-of-M approval, tamper-evident audit, and SOC 2 as explicit roadmap; **(3)** fixes the embed-widget plaintext + origin findings before foregrounding the widget, or introduces it only as "origin-pinned embedding shipping now." The numbers are strong but must be re-attributed (CyberArk for 80:1, Entro for 144:1, GitGuardian for 28.65M/+81%), and the Console.dev "68%" figure and the PATTERN_MATRIX-row-13 differentiation claim must be dropped as written. Get those right and the design-partner motion that the entire strategy depends on stays intact.

---

*Key source files cited above (all absolute): `/home/user/KeepSave/docs/research/BEYOND.md`, `/home/user/KeepSave/docs/research/PATTERN_MATRIX.md`, `/home/user/KeepSave/docs/research/BACKLOG.md`, `/home/user/KeepSave/docs/EMBED_ORIGIN_POLICY.md`, `/home/user/KeepSave/docs/ROADMAP_NOT.md`, `/home/user/KeepSave/docs/audits/AUDIT_2026-06-26_CODE_AUDIT.md`, `/home/user/KeepSave/sdks/{go,python,nodejs}/`.*

---

<a id="part-3--seo--sem--geo"></a>
# Part 3 — SEO / SEM / GEO

Confirmed the ground truth. Now I have everything I need to write an accurate section. Summary of what's actually shipped vs. roadmap (this corrects the research):

- **Promotion kill switch** (`KEEPSAVE_PROMOTIONS_ENABLED`) — SHIPPED (CLAUDE.md env var, PATTERN_MATRIX row 9, ADR-0003 follow-up closed).
- **Approver ≠ requester, DB-enforced** — SHIPPED (migration 008 CHECK constraint on Postgres + trigger on SQLite, service guard `promotion_service.go:260` returning `ErrSelfApproval`). The research said it was "service-code only" — that was the pre-fix state; it's now DB-enforced. So the claim is *stronger* than the research stated.
- **Tamper-evident audit chain** — SHIPPED/wired (`main.go:108` calls `SetChainKey(cryptoSvc.DeriveAuditChainKey())`, migration 012, HMAC-SHA256 hash chain per ADR-0019). ADR-0019 status line says "design," but the code is wired — so the honest framing is "implemented, keyed in prod."
- **Single-approver only; N-of-M NOT implemented** — confirmed (ADR-0003 line 74, PATTERN_MATRIX row 7). This is the one place the research's suggested copy ("multi-party secret approval") overstates the product.
- **AES-256-GCM at rest, per-project envelope keys** — confirmed (ADR-0001/0004, CLAUDE.md).

Now I'll write the section. This is a synthesis/writing task — the deliverable is my final message (the parent agent consumes it). I'll fold in the fact-check corrections explicitly: the 81% (not 81.5%) GitGuardian figure, the $80–$200 (not $50–$200) competitor-CPC floor, the Previsible/Similarweb misattribution (not Yotpo/Adobe), and the single self-interested source behind the CPC tiers.

Let me write it now.

# SEO / SEM / GEO

> **Scope and confidence.** This section covers organic discovery (SEO), paid search (SEM), and getting KeepSave cited by AI answer engines (GEO). The single most important caveat up front: **no proprietary keyword tool (Ahrefs/Semrush) or PPC account was accessible during this research.** Every search-volume number, keyword-difficulty rating, and CPC band below is a *qualitative, directional estimate* derived from SERP inspection, competitor content investment, and a small set of self-interested third-party benchmark blogs — not validated demand data. Treat the whole section as a hypothesis to test, and **validate volumes/difficulty in Ahrefs or Semrush and CPCs in a live Google Ads account before committing budget or a content calendar.** Claims the fact-check did not support are flagged inline with **⚠ FACT-CHECK**.

---

## 0. Strategic frame: one visibility system, three surfaces

SEO, SEM, and GEO are not three programs — they are three output surfaces of one buyer-discovery system, and KeepSave's situation makes the priority order unusually clear:

- **KeepSave has zero brand SERP presence today.** Searches for "keepsave secrets manager" return no product hits. There is no branded-search base, no backlink profile, and (pending the open question below) no confirmed marketing domain. Every early visit must therefore come from *non-branded* informational and commercial-investigation queries — which means the content and comparison-page program is the foundation, not a nice-to-have.
- **KeepSave occupies a genuinely under-contested intersection.** AES-256-GCM-at-rest secrets storage (ADR-0001/0004) + an approval-gated Alpha→UAT→PROD promotion pipeline (ADR-0003) + per-project/per-environment scoped API keys + an embeddable `<keepsave-widget>` + self-hostability. No incumbent markets all of these together, and — verified against competitor SERPs and the in-repo Pattern Adoption Matrix (`docs/research/PATTERN_MATRIX.md` row 9) — **no competitor markets "promotion pipeline" / "approval gates" / "Alpha→UAT→PROD" as a first-class concept.** That is KeepSave's lowest-competition, highest-differentiation angle and it recurs across all three surfaces.
- **The market is large and the timing is live.** The secrets-management solutions market was ~USD 4.22B in 2025, forecast ~USD 8.05B by 2030 (~13.8% CAGR; KBV Research — single-source market sizing, treat as directional). GitGuardian's *State of Secrets Sprawl 2026* (published March 2026) found **24,008 unique secrets in MCP configuration files on public GitHub, of which 2,117 were still valid**, and that **AI-service credential leaks surged 81% in 2025** (to 1,275,105 detected secrets). ⚠ **FACT-CHECK:** the research draft cited "81.5%" — that figure is **not supported**; every primary GitGuardian source and press release says **81%**. The 24,008 MCP figure *is* corroborated and citable. Use "81%," never "81.5%."

**Resulting priority order** (same across SEO/SEM/GEO):
1. **AI-agent secrets cluster** — lowest competition, tightest product fit, fastest-growing intent, no incumbent comparison content.
2. **Comparison / "alternative" pages** — highest near-term ROI; bottom-of-funnel intent.
3. **Promotion-pipeline pillar** — zero-competition differentiator content.
4. **Foundation "what is secrets management" pillar** — authority/entity-building cornerstone.
5. **Head term "secrets management"** — a 12-month goal, *not* a launch target.

---

## 1. SEO

### 1.1 Keyword intent clusters

Five clusters, ordered by recommended sequencing rather than volume. **All volume/difficulty values are low-confidence estimates** (see scope caveat).

| # | Cluster | Representative terms | Est. volume* | Est. difficulty* | Intent | KeepSave priority |
|---|---------|---------------------|-------------|-----------------|--------|-------------------|
| 1 | **Broad / informational** | "secrets management", "what is secrets management", "secrets management best practices", "secret sprawl", "hardcoded secrets" | Med–High (head term ~5k–20k/mo, LC) | **High** (AWS, Google, HashiCorp, Infisical, GitGuardian dominate) | Informational | Long-tail derivatives now; head term at 12mo |
| 2 | **Commercial investigation** | "Doppler alternative", "Infisical alternative", "HashiCorp Vault alternative", "self-hosted secrets manager", "open source secrets manager" | ~500–3,000/mo per competitor name (LC) | **Medium** (competitors self-rank for these) | Commercial | **Highest near-term ROI** |
| 3 | **Emerging / AI-agent** | "AI agent secrets management", "secrets for AI agents", "scoped API keys for agents", "MCP secrets", "agent credential management" | Low today (sub-500/mo, LC) but rising fast | **Low–Med** | Informational→commercial | **Launch here** |
| 4 | **Practitioner / long-tail** | "promote secrets to production", "secret promotion pipeline", "environment variable audit log", ".env alternative for teams", "stop committing .env to git" | ~200–2,000/mo each (LC) | **Low–Med** | Informational→commercial | High — owns the differentiator |
| 5 | **Enterprise / compliance** | "secrets management audit trail", "tamper-evident audit log secrets", "AES-256-GCM secrets at rest", "secret promotion approval workflow", "SOC 2 secrets management" | Low (~100–500/mo each, LC) | **Low–Med** | Enterprise commercial | Medium — higher ACV, longer cycle |

*\*All figures are SERP-density estimates, **not** tool-validated. Re-baseline before planning.*

**Cluster sourcing notes:** Cluster-2 viability is confirmed by competitors running these exact pages — Infisical's "Top Doppler Alternatives" (`infisical.com/blog/doppler-alternatives`) and Doppler's "Infisical vs Doppler" comparison (`doppler.com/blog/...comparison-2025`) prove the format ranks. Cluster-3 commercial investment is signalled by 1Password, Aembit, Descope, and WorkOS all publishing AI-agent-credential content in 2025–2026 (e.g., `workos.com/blog/ai-agent-secrets-management`, `1password.com/blog/credential-management-for-ai-agents`), while ranking content remains low-DA developer blogs. Cluster-4's anchor stat — dotenv at **45M+ weekly npm downloads** — frames the replacement opportunity (`dev.to` developer-tooling survey).

### 1.2 Content pillars

| Pillar | Working title | Target cluster(s) | Why it wins |
|--------|--------------|-------------------|-------------|
| **1. Foundation** | "Secrets Management: The Complete Guide" (3,000–4,000 words) | 1, 5 | Authority cornerstone; every other page links to it; earns roundup backlinks. **Publish first.** |
| **2. Differentiator** | "How the Alpha→UAT→PROD Promotion Pipeline Works" + "Why Secret Promotion Needs Approval Gates (Not Just CI/CD Variables)" | 4 | **Zero direct competitor content.** Verified: no Doppler/Infisical/Vault page uses "promotion pipeline" as a heading. |
| **3. Audience** | Hub `/secrets-for-ai-agents` + children (scoping keys for Claude/GPT agents, prompt-injection credential theft, MCP server secrets) | 3 | Fastest-growing intent, tightest fit, lowest competition. Cite GitGuardian 24,008-MCP stat + OWASP agentic-apps guidance as authority anchors. |
| **4. Integrator** | "Embedding KeepSave with One Script Tag" / "Building a Secrets Layer for Your SaaS with the KeepSave Widget" | 4 | The `<keepsave-widget>` Web Component (Shadow DOM) is undocumented by any competitor as a headline product. Low volume, zero competition. |

### 1.3 Comparison / "alternative" page program — the highest-ROI short-term investment

⚠ **FACT-CHECK on the headline justification.** The research repeatedly asserts that *"bottom-of-funnel 'alternative' and 'vs' pages drive 40–60% of organic SaaS conversions."* This figure traces to a single agency blog (`tripledart.com`) and was **not independently corroborated**. The *directional* claim — that comparison pages are disproportionately high-converting because they capture purchase-stage intent — is well-established and supported by competitors investing heavily in them. **Do not present "40–60%" as a validated benchmark**; frame it as "comparison pages capture the highest-intent, closest-to-purchase searches, which is why competitors prioritize them."

Build five pages in parallel, each ≥2,000 words, indexable (not gated), self-canonical, and internally linked from the foundation pillar and the site footer. Structure per page: comparison summary → feature table → "When to choose KeepSave vs [X]" → migration guide → CTA.

| Priority | Page | KeepSave's honest angle |
|----------|------|------------------------|
| 1 | `/doppler-alternative` | Self-hostable (no SaaS lock-in); built-in promotion pipeline; AI-agent-native scoped API keys |
| 2 | `/infisical-alternative` | Embeddable widget; promotion-gate engine; simpler deploy |
| 3 | `/hashicorp-vault-alternative` | Lighter, embeddable, promotion gates, no Sentinel DSL. **Migration-cohort angle:** Vault moved to BSL in 2023 and HashiCorp was acquired by IBM (2025), creating a displaced OSS user base actively searching "vault alternative." Lead with "self-hosted, MIT-licensed core, no IBM dependency." |
| 4 | `/1password-connect-alternative` | Self-hostable; multi-env promotion; open architecture |
| 5 | `/dotenv-alternative` | Highest top-of-funnel volume (45M weekly npm downloads) but longest conversion cycle — awareness-stage, not purchase-stage |

> **Honesty guardrail for all comparison content (binding):** see §1.5. Several differentiators in the research's suggested copy must be tightened to match what KeepSave *actually ships today*.

### 1.4 Technical SEO checklist

Implement **before** publishing any content:

1. **Pre-render / SSR all content pages.** The React + Vite SPA is not reliably crawled by Googlebot *or* by AI answer engines (which extract visible HTML — see §3). This is the single highest-impact technical item.
2. **Core Web Vitals:** LCP < 2.5s, CLS < 0.1, INP < 200ms.
3. **Structured data:** `SoftwareApplication` (applicationCategory: SecurityApplication) on the homepage; `FAQPage` on how-to and comparison pages; `BreadcrumbList` on inner pages. (Treat as a *disambiguation* layer, not a citation guarantee — see §3.)
4. **XML sitemap** auto-generated, submitted to Google Search Console.
5. **Self-referencing canonicals** on every comparison page; avoid `/blog/doppler-alternative` vs `/doppler-alternative` duplication.
6. **robots.txt:** block private routes (`/api/`, `/admin/`, `/embed-config/`, `/auth/`, `/audit-log/`); allow all content. **But** allow AI crawlers on content (§3) — these two rules coexist.
7. **Internal linking:** every cluster post → foundation pillar + nearest comparison page; homepage → all five comparison pages.
8. **AI-Overviews optimization:** definitional sentence in the first ~40–100 words of every page; question-phrased H2s; FAQ schema. ⚠ **FACT-CHECK qualifier:** the research cites "featured-snippet visibility dropped 64% in H1 2025" from a single vendor blog (Keywords Everywhere) — directional, not independently verified. The *actionable* takeaway (optimize for AI Overviews, not just snippets) stands regardless of the exact percentage.
9. **OG / Twitter Card meta** on all pages for shareability.
10. **Docs indexation:** serve docs on a `/docs` subdomain or `noindex` them if not intended to rank, to avoid diluting the marketing domain.

### 1.5 Honesty guardrail — claims to tighten (load-bearing; verified against the repo)

The research draft's suggested marketing copy over- and under-states KeepSave in specific places. I verified the actual shipped state in-repo; correct the copy as follows:

| Claim in research draft | Actual shipped state (verified) | Required copy |
|------------------------|-------------------------------|---------------|
| "multi-party secret approval" / "N-of-M approval" as a headline feature | **NOT implemented.** ADR-0003 §74 and Pattern Matrix row 7 confirm **single-approver only**; N-of-M is explicitly Phase-B/deferred. | Market **"approval-gated promotion with a DB-enforced approver-≠-requester invariant,"** *not* "multi-party"/"N-of-M." Over-claiming here is a factual defect. |
| "approver-not-requester invariant" (research treated it as service-code-only) | **STRONGER than the draft says.** It is now **DB-enforced**: a `CHECK (... requested_by <> approved_by)` constraint on PostgreSQL (`migrations/postgres/008_promotion_self_approval_check.sql`), an equivalent trigger on SQLite, **plus** a service-layer guard (`promotion_service.go:260`, `ErrSelfApproval`). | Safe to claim **"database-enforced separation of duties on PROD promotion"** — this is a real, defensible differentiator. |
| "tamper-evident audit trail" | **Shipped and wired.** Keyed HMAC-SHA256 hash chain per ADR-0019; the chain key is derived under the master key and set at boot (`cmd/server/main.go:108` → `SetChainKey(cryptoSvc.DeriveAuditChainKey())`; migration `012_audit_chain.sql`). ADR status line still reads "design," but the code path is live. | Safe to claim **"tamper-evident, hash-chained audit log."** Pair with the honest nuance that pruning/checkpointing is the open edge (ADR-0019 §Consequences) if asked in depth. |
| "AES-256-GCM at rest, per-project envelope keys" | **Confirmed** (ADR-0001 envelope encryption; ADR-0004 per-project DEK). | Claim as-is. |
| "promotion kill switch" | **Shipped** as `KEEPSAVE_PROMOTIONS_ENABLED` (CLAUDE.md; closes ADR-0003 follow-up). `false` ⇒ `/promote` + `/approve` return 503. | Fine to reference in ops/enterprise content. |

### 1.6 Backlink acquisition (zero-budget, pre-revenue appropriate)

1. Pursue inclusion in GitGuardian's "Top Secrets Management Tools" roundup (high-DA; all competitors appear) — outreach required.
2. Submit to `openalternative.co` Doppler/Infisical alternative lists (aggregates traffic, triggers downstream directory backlinks).
3. Publish an "AI Agent Credentials" data-backed post citing the GitGuardian 24,008-MCP stat + original KeepSave architecture analysis — citable by "agentjacking"/AI-security writers.
4. Show HN on the promotion-pipeline engine or the embeddable widget.
5. Cross-post technical content to DEV Community and Hashnode (high-DA dofollow).
6. Contribute to the OWASP Secrets Management Cheat Sheet (contributor credit links back).

---

## 2. SEM (paid search)

> **Source-credibility caveat (applies to the entire CPC framework).** The CPC tiers below originate **predominantly from a single self-interested agency blog (`growthspreeofficial.com`)** whose figures are self-reported ("$60M+ managed spend"), unaudited, and content-marketing for PPC services. Several pages were inaccessible for line-level verification. Independent aggregators corroborate the *cybersecurity category* band but **not** the competitor-evaluation floor (see below). Use these as rough planning brackets, not benchmarks; **the live account's own data should replace them within 60–90 days.**

### 2.1 CPC bands

| Tier | Example queries | Est. CPC (2026) | Est. CVR | Notes |
|------|----------------|----------------|----------|-------|
| **DevTools / developer-productivity** | "environment variable manager", "API key management tool", "developer secrets tool" | **$7–$9** | 1–3% | Lower-cost frame; recommended starting posture |
| **Secrets-mgmt / security-SaaS category** | "secrets management platform", "encrypted environment variables", "secrets management CI/CD" | **$16–$22** | 1–3% | Corroborated by multiple independent aggregators |
| **Evaluation / procurement** | "Doppler alternative", "Vault alternative", "secrets manager pricing comparison" | **$80–$200+** ⚠ | 4–8% | See fact-check below |

⚠ **FACT-CHECK — corrected:** the research draft stated the evaluation tier runs **"$50–$200."** The cited source's own text says **"$80–$200+"** — the $50 floor is **not supported** by the source or any independent reference found. **Use $80–$200+** for the cybersecurity-procurement tier. (Independent practitioners put *general* SaaS "alternative" keywords far lower, ~$10–$25; only security-adjacent enterprise-procurement queries reach the $80–$200+ band — so KeepSave's actual CPC will depend heavily on how Google classifies each query, another reason to bid narrow-and-measured first.)

The companion claim that **DevTools cost-per-SQL ~$650 vs cybersecurity ~$3,500** is from the same single source; the relevance is directional — KeepSave's PLG/trial-signup motion targets the *lower* (devtools) cost structure and bypasses the enterprise-CISO sales cost entirely, *if* trial signup (not "request a demo") is the conversion event.

### 2.2 Keyword plan by funnel stage

**Bottom-of-funnel (bid first):** "Doppler alternative", "Infisical alternative", "HashiCorp Vault alternative", "secrets manager for AI agents" (emerging — few bidders), "self-hosted secrets manager".

**KeepSave-specific, near-zero-competition (bid early, exact match):** "environment variable promotion pipeline", "promote secrets alpha UAT production", "secure secrets for MCP servers". These mirror the product's differentiator, have almost no competing bidders, and attract exactly the developer who has outgrown `.env` + CI/CD variable silos. Est. CPC $3–$12; small volume, very high intent.

**Mid-funnel / category (bid second):** "secrets management platform", "API key management SaaS", "secrets management CI/CD".

**Top-of-funnel (do NOT bid; retargeting/SEO only):** "secrets management best practices", "how to manage API keys", "environment variables security".

**Negative keywords (add immediately):** `free`, `tutorial`, `course`, `salary`, `internship`, `AWS Secrets Manager`, `GCP Secret Manager` (product-name confusion), and `open source` *unless* deliberately bidding the OSS-alternative queries. Developer query pollution is real and will drain budget on broad match.

### 2.3 Competitor brand-term rules (verified)

Google's trademark policy (updated **February 2025**) — corroborated across sources — permits **bidding on competitor brand names as keywords** but **prohibits using those trademarks in ad headlines/descriptions** if the owner has filed a complaint. Practical rules for KeepSave:
- **You CAN** bid on "Doppler", "Infisical", "HashiCorp Vault" as keywords.
- **You CANNOT** safely write "Doppler alternative" *in the ad copy* if Doppler has filed. Instead surface via keyword targeting with copy that frames *your own* differentiators: e.g., *"Self-Hosted Secrets Manager · Alpha→UAT→PROD Promotion."*
- **De-prioritize AWS/GCP brand terms** — high volume but mostly locked-in customers; low CVR (1–2%), elevated CPC.
- **Note on "Infisical alternative":** Infisical's own pages rank for it organically and they may bid on it — expect elevated CPC and self-competition; only bid if the landing page converts strongly.
- **Defend your own brand** ("KeepSave") from day 1 at every budget tier (CPC ~$0.50–$2.00; 10–15% of budget). Register the "KeepSave" trademark to unlock copy-restriction protection as awareness grows (open question — see §4).

### 2.4 Budget tiers (illustrative, contingent on unknown unit economics)

> Every ROI figure below depends on three **currently unknown** KeepSave inputs: trial-to-paid rate, ACV, and whether the motion is self-serve trial vs sales-demo (see §4). The projections use industry placeholders (PLG trial-to-paid ~8–15%; ACV ~$210–$250/mo Doppler-tier). **If trial-to-paid is below ~5%, the $1k–$5k tiers are uneconomic regardless of CPC.**

| Tier | Recommended allocation | Realistic expectation |
|------|----------------------|----------------------|
| **$1,000/mo** | **Reddit-first**, not Google-first: ~$700 Reddit (r/devops, r/selfhosted, r/sysadmin; CPC $0.50–$2.00) + ~$300 Google **own-brand defense only**. | A *learning* budget. Pure Google at ~$16 blended CPC ≈ 60 clicks → ~2–3 trials → ~0.2–0.4 paid/mo. Not meaningful pipeline. Goal: accumulate 30+ conversions to unlock Smart Bidding. |
| **$5,000/mo** | ~50% non-brand category+competitor, ~20% AI-agent cluster, ~20% brand+remarketing, ~10% Reddit. | Minimum viable Google program. ~180–250 Google clicks + Reddit → ~18–35 trials/mo → ~2–4 paid/mo → ~$420–$840 new MRR. Cost per new-MRR-dollar ~$6–$12, acceptable if LTV ≥ 24 mo. |
| **$20,000/mo** | ~40% non-brand Google, ~15% AI-agent, ~15% brand+remarketing, ~15% Reddit, ~15% LinkedIn (DevOps/Platform/Security Engineers, 50–2,000 employees). | Full-funnel. ~60–120 trials/mo → ~6–18 paid/mo → ~$1,500–$4,500 new MRR. Only makes unit-economic sense with annual/enterprise deals (≥$2,400 ACV). Use LinkedIn for *content* promotion ("how we designed our promotion pipeline"), not "Sign up free" CTAs — dev audiences reject direct trial CTAs there. |

### 2.5 Campaign structure, bidding, and what to avoid

- **One campaign per intent cluster** (Brand / Competitor / AI-agent / Category / DevTools-generic / RLSA-remarketing) for budget control and distinct Quality Scores.
- **Bidding progression:** Manual CPC or Maximize Conversions for the first 60–90 days to accumulate the ~30 conversions/campaign threshold, *then* migrate to Target CPA.
- **Match-type discipline:** exact match for competitor terms; phrase for category; **avoid broad match** on security/devtools terms (pollution from "tutorial"/"what is" queries). Run the Search Term Report weekly.
- **Do NOT use Performance Max.** Devtools/security buyer intent concentrates on Google Search; PMax pushes budget into Display/YouTube/Discover where this audience doesn't convert. Reconsider only as a remarketing vehicle once you have 500+ converted customers feeding closed-won audience signals.

### 2.6 Landing-page CRO for paid traffic

- **One dedicated, message-matched page per ad group** — the ad headline must appear near-verbatim in the H1. Generic homepage sends are the single largest paid-budget waste.
- **Competitor pages:** lead with the **audit trail + approval-gated promotion**, not price; include the feature table and a migration guide. Use the §1.5-corrected claims (DB-enforced separation of duties; hash-chained audit; *not* "multi-party").
- **AI-agent page:** "agents need secrets *with guardrails*, not just secrets" — scoped per-project/per-environment keys, hash-chained audit of every agent action, promotion gate preventing untested secrets reaching PROD.
- **Self-hosted page:** `docker-compose up --build` as the above-fold hero claim — **but only if that is genuinely one command** (open question §4; an inflated claim produces trials that immediately churn).
- **Form length:** email + company only (2 fields max). **Trust bar:** "No credit card required · Self-hosted · open core." **Speed:** sub-2.5s LCP. Industry-median landing CVR ~3.8%; top devtools performers 10%+ (single-source benchmark — directional).

### 2.7 Conversion tracking (do this on day 1, not later)

1. **Google Tag via GTM:** fire `trial_signup` on the signup-success event; assign value = est. LTV × trial-to-paid, so Smart Bidding optimizes for value, not form-fills.
2. **Enhanced Conversions for Leads:** hash email at signup → Google Ads. **Required replacement** for the legacy `UploadClickConversions` API, **deprecated June 15, 2026** (verified across multiple sources).
3. **Offline conversion import (CRM → Google Ads):** import `closed_won` with actual MRR; **90-day conversion window** to match the ~84-day B2B SaaS close cycle (the default 30-day window systematically undercounts and starves Smart Bidding). This is the highest-leverage tracking upgrade — documented to cut CAC and lift SQL volume materially (single-source figures; directional).
4. **GCLID capture** at signup, stored in the DB, passed back with offline conversions.
5. **Diagnostic micro-conversions** (not bidding signals): `docs_viewed`, `api_key_created`, `first_secret_stored`, `promotion_requested`.
6. **Reddit pixel** on signup-success; fire `Lead`.

---

## 3. GEO (getting cited by AI answer engines)

GEO optimizes for **citation share** — the % of buyer-intent prompts where an engine *names KeepSave inside the generated answer* — not for link rank. The two decouple sharply: practitioner tracking (directional, attributed) finds only ~12–17% of URLs cited in AI answers also rank in Google's organic top 10. SEO gets you into the *candidate pool*; GEO converts pool-presence into in-answer citation.

### 3.1 The evidence base

The foundational source is **"GEO: Generative Engine Optimization," Aggarwal et al., arXiv 2311.09735, ACM SIGKDD/KDD 2024.** ✅ **FACT-CHECK: supported** (with two precisions): it is a **multi-institution paper with a Princeton core** — 3 of 6 authors are Princeton, but **first author Pranjal Aggarwal is at IIT Delhi** (also Georgia Tech, Allen Institute for AI), so "Princeton-co-led" is more accurate than "Princeton-led." It introduced **GEO-bench (~10,000 queries)**; the "**9 domains**" detail is **slightly over-precise** — sources split 8 vs 9, likely reflecting source datasets vs thematic domains, so say "**≈8–9 domains**." The headline finding — **content edits can lift visibility in AI answers by up to ~40%** — holds. *Caveat:* the primary PDF was egress-blocked during fact-check; numbers were corroborated via convergent secondary sources, not read off the primary — so retain medium-high (not "verified-off-primary") confidence and re-baseline on KeepSave's own prompt set, since the paper itself notes efficacy varies by domain.

**The three tactics that empirically move citations** (apply directly to KeepSave docs + comparison pages): **(1) Cite Sources**, **(2) add Quotations**, **(3) add Statistics.** Crucially, **Keyword Stuffing performed *below* baseline** — the classic-SEO reflex actively hurts in generative engines.

### 3.2 Where LLMs actually pull from (concentrated — seed these)

Citation sources are highly concentrated; off-site presence beats on-page tweaks. Directional ranking is high-confidence; exact percentages vary by study/window (medium confidence):

- **Reddit ≈ #1 cited source (~40% across engines).** Authentic, non-spammy participation in devsecops / secrets-management / AI-agent-tooling subreddits. (Perplexity leans hardest on Reddit; Gemini far less.)
- **GitHub** — a strong README carrying the canonical entity line; get into relevant `awesome-secrets` / `awesome-ai-agents` lists; accumulate stars (ChatGPT can read and cite READMEs directly).
- **B2B review profiles** (G2, Capterra, Trustpilot) — practitioner-reported ~3× higher ChatGPT citation odds for B2B software (single practitioner source — medium confidence).
- **Wikipedia** (~#2 for ChatGPT) — pursue only if independent-coverage **notability** is met (open question §4).
- **Independent "best secrets manager" listicles** — being *named in third-party* lists is what lifts recommendation; see §3.4.

### 3.3 On-site GEO

- **One canonical entity line, repeated verbatim everywhere** (homepage, README, docs intro, About, G2/Capterra/Trustpilot, guest posts). Suggested, corrected to match shipped reality (§1.5): *"KeepSave is a self-hostable, embeddable secrets layer for AI agents — AES-256-GCM-encrypted at rest, with an approval-gated Alpha→UAT→PROD promotion pipeline and a tamper-evident, hash-chained audit log."* (Note: drop "machine-identity" framing unless/until that's a shipped product claim.)
- **Answer-first structure:** direct extractable answer in the first ~40–60 words; H2/H3s phrased as buyer questions ("Is KeepSave a Vault alternative?", "How do AI agents get scoped secrets?"); short sentences; comparison tables; ~one verifiable fact per ~80 words. Engineer pages to be the *most credible* thing on the topic, not merely the most extractable.
- **Crawl access — marketing site only.** Allow `GPTBot`, `OAI-SearchBot`, `ClaudeBot`, `Claude-SearchBot`, `Claude-User`, `PerplexityBot`, `Google-Extended` in robots.txt. **The application (login, secrets API, dashboard, embed widget, any secret values) must stay non-indexable and behind auth — GEO crawlability is strictly a marketing-site concern and must never touch the product's security boundary.** (A few crawlers — Bytespider, some Perplexity stealth fetchers — ignore robots.txt; only WAF-level rules enforce.)
- **Schema (FAQPage/HowTo/Article/Organization/SoftwareApplication):** implement as a cheap disambiguation/entity-clarity layer, but treat as **secondary** — the evidence is genuinely mixed. Correlational data shows 65–71% of cited pages carry structured data, but a rigorous Ahrefs study (1,885 pages adding JSON-LD) found **no citation uplift**, and real-time tests found ChatGPT/Claude/Perplexity/Gemini all **extracting visible HTML and ignoring JSON-LD**. Put the marginal effort into well-structured *visible* content first.
- **`llms.txt` / `llms-full.txt`:** ship for docs/SDKs as **developer experience, not a citation lever.** Adoption is ~10% of domains, **no** major provider commits to it for answer-surface citations, and multiple studies (ALLMO 94k+ URLs; an OtterlyAI experiment where it was hit in ~0.1% of crawler visits) found no uplift. **But** coding/IDE agents (Cursor, Claude Code, Copilot) *do* fetch it — which fits the "secrets layer for AI agents" positioning. Ship it; don't expect citation movement from it.

### 3.4 The listicle trap (counterintuitive, high-confidence)

A 2026 study (Lily Ray / ALM Corp) found that when a brand publishes its *own* "best-of" listicle and Google AI Overviews cites it, the **publishing brand is omitted from the actual recommendation ~69% of the time** — ranking yourself #1 often promotes your competitors. **Implication:** still publish a genuinely balanced category comparison (give competitors honest strengths — engines reward the most credible page), but **do not expect your own listicle to get *you* recommended.** Recommendation lift comes from being named in *third-party* lists and review sites.

### 3.5 Measurement loop

- Define **~30 buyer-intent prompts** ("best secrets manager for AI agents", "self-hosted Vault/Doppler/Infisical alternative", "how should AI agents store scoped API keys", "secrets promotion pipeline Alpha UAT PROD").
- Run weekly across ChatGPT, Claude, Perplexity, Gemini, Google AI Overviews; log who is cited and where; compute **citation rate** (e.g., 20/50 = 40%) and answer position.
- **Expect 6–12 weeks** before citation share moves. Manual the first ~6 months; automate with Otterly.ai / Peec / Profound later. Re-baseline every 4 weeks.

### 3.6 Disambiguation note

"GEO" here means **Generative Engine Optimization.** The geographic/local-SEO meaning (Google Business Profile, Local Pack/Maps) **does not apply** to a self-hostable/embeddable developer-infra product with no physical-location buyer intent. No action needed unless KeepSave later markets a physical presence.

---

## 4. Measuring AI-referral traffic (the hardest attribution problem)

⚠ **FACT-CHECK — a load-bearing headline stat in the research is REFUTED.** The draft states: *"AI-referred sessions grew 527% YoY; AI platforms generated 1.13 billion referral visits in June 2025 (Yotpo / Adobe Analytics, 2025)."* This is **wrong on every axis checked** and must not be published as written:
- The **527%** figure is from **Previsible's 2025 State of AI Discovery Report** (not Yotpo/Adobe) and describes a **5-month surge** (Jan→May 2025), **not** a year-over-year change. Calling it "527% YoY" mischaracterizes the methodology.
- The **1.13 billion / June 2025** figure is real but comes from **Similarweb** (not Adobe Analytics), and the correct growth label is **+357% YoY**.
- **Corrected, citable form:** "AI-platform referrals to the top 1,000 websites reached ~1.13 billion in June 2025, up ~357% YoY (Similarweb). Separately, one 19-property study found LLM-referred sessions up ~527% over five months, Jan–May 2025 (Previsible)." Do **not** attribute either to Yotpo or Adobe.

The directional point survives the correction: **AI referral is the fastest-growing discovery channel and the hardest to attribute.** Recommended measurement stack:

- **GA4 custom channel "AI Search"**, set *above* "Referral" in priority, regex: `chatgpt\.com|chat\.openai\.com|perplexity\.ai|claude\.ai|gemini\.google\.com|copilot\.microsoft\.com|deepseek\.com|grok\.com|meta\.ai`. Pair with GA4's **native "AI Assistant" channel (added May 2026)**, which covers ChatGPT/Gemini/Claude but **misses Perplexity and Copilot** — hence the custom group too.
- **Server-log monitoring** for `GPTBot`/`ClaudeBot`/`PerplexityBot` user-agents as a 2–4-week leading indicator of branded-search lift. Cloudflare's crawl-to-referral ratios (Q1 2026; directional) — Perplexity ~111:1 (most efficient), GPTBot ~1,276:1, **ClaudeBot ~24,000:1** (least efficient per referral) — mean crawler volume ≠ referral volume.
- **The unsolvable residual:** a large majority of AI-influenced traffic arrives with **no referrer header** and lands as "Direct"/"Unassigned" (one single-study figure puts it at ~70.6%). Report "attributed AI traffic" and an "estimated dark-AI range" as **separate rows** — never sum them as if equally precise.
- **The single most reliable attribution tactic for a devtool:** a **"How did you hear about us?"** field on every signup and on the self-hosted first-run consent screen. It captures the dark funnel (Reddit, Slack, Discord, HN, AI-assistant recommendations) that no analytics tool can.

---

## 5. Open questions blocking execution (resolve before spending)

1. **Marketing domain?** A dedicated marketing site (e.g., `keepsave.io`) *separate from the authenticated app* is required — serving content from the app domain creates robots.txt/indexation/security conflicts and breaks the §3 crawl-access split. **GEO has no on-site substrate without this.**
2. **Public repo / open-core?** GitHub star count is a trust signal competitors' comparison content uses as social proof (e.g., Infisical 12,700+ stars — directional). A public repo also underpins the §3.2 GitHub citation play and any "open core" copy.
3. **Tooling budget?** All keyword volumes/difficulty here are estimates — validate in Ahrefs/Semrush before building a content calendar.
4. **Initial ICP?** Solo devs vs startup teams vs enterprise DevSecOps changes cluster priority (developer-workflow vs compliance content) and SEM tier economics.
5. **Trial-to-paid rate, ACV, and motion (self-serve trial vs sales demo)?** These three unknowns determine whether *any* of the §2.4 budget tiers are economic; below ~5% trial-to-paid, paid search is premature.
6. **Trademark "KeepSave"?** Registration unlocks competitor-copy protection (§2.3) as awareness scales.
7. **Is `docker-compose up --build` genuinely one-command?** This is the proposed self-hosted hero claim across SEO and SEM; an inflated claim manufactures immediately-churning trials.
8. **Will the `<keepsave-widget>` be publicly documented?** A public integration guide would rank for currently zero-competition terms ("embed secrets manager", "secrets widget SDK").

---

### Bottom line

Lead with the **AI-agent secrets cluster** (lowest competition, tightest fit, no incumbent comparison content), build the **five comparison pages** in parallel, and own the **promotion-pipeline** differentiator that no competitor markets. Pre-render the site before publishing anything. In paid, **start Reddit-first and own-brand-only** until unit economics are proven, bid the near-zero-competition KeepSave-specific terms early, avoid PMax, and wire offline conversion tracking on day 1. In GEO, **seed Reddit/GitHub/review sites**, repeat one corrected entity line everywhere, apply Cite-Sources/Quotations/Statistics (never keyword-stuff), and run a weekly ~30-prompt citation scoreboard. Throughout, hold the **honesty guardrail** from §1.5: claim "DB-enforced approver-≠-requester," "hash-chained tamper-evident audit," and "AES-256-GCM at rest" — all verified shipped — but **never "multi-party / N-of-M approval," which is not built.** And purge the three unsupported stats before anything ships: GitGuardian is **81%** (not 81.5%); the cybersecurity evaluation-CPC floor is **$80** (not $50); and the AI-referral figures are **Similarweb (1.13B / +357% YoY)** and **Previsible (527% over 5 months)** — **not** Yotpo/Adobe, and **not** "527% YoY."

---

<a id="part-4--robot-framework"></a>
# Part 4 — Robot Framework

All facts now verified, including a correction to the research: the env tabs use `.ks-tab` with `dataset: { env }` — which renders as the `data-env` attribute (line 241 `dataset: { env }`, read back at line 394 `tab.dataset.env`). So `.ks-tab[data-env="uat"]` IS a valid selector. The research claim was correct after all.

I now have everything needed. The CI is a single SHA-pinned least-privilege pipeline (`permissions: contents: read`, every `uses:` pinned to a commit SHA). I have enough to write a decisive, fully-cited section.

---

## Robot Framework

> **Scope of this section.** Whether — and how — to adopt [Robot Framework](https://robotframework.org/) (RF) as an acceptance-test layer for KeepSave's three surfaces: the 144-route REST API, the React dashboard, and the Shadow-DOM `<keepsave-widget>` embed. The verdict is *yes, but narrowly*: RF earns its keep as the **black-box acceptance tier** sitting just above the existing Go E2E harness, and nowhere else. Every recommendation below is gated against KeepSave's existing test pyramid, coverage gates, and CI model.

### 1. Bottom line

| Question | Verdict |
|---|---|
| Is RF technically viable for KeepSave's API + dashboard + widget? | **Yes** — all three are reachable; the widget is reachable *because* it uses an open shadow root (see §4). |
| Where does RF add non-overlapping value? | **Two tiers only:** (1) HTTP contract / negative-auth tests over the 144 routes; (2) a thin browser E2E for dashboard + widget *positive* journeys. |
| Where must RF **not** go? | Crypto/audit invariants, in-process middleware rejection logic, the 132-cell negative-auth matrix, frontend component logic, fuzz. These stay in Go/Vitest (§6). |
| Where does it live? | **`tests/robot/` inside this repo** (integrated), mirroring the existing `tests/e2e/seidr/` precedent (§7). Not a separate repo, not a top-level `/qa`. |
| What's the real cost? | A third toolchain (Python + Playwright browser binaries) in CI, and the cross-origin `postMessage` harness plumbing (§5). Classified **Type-2** under `CLAUDE.md`. |

The single most important framing: **RF is the *tip* of the pyramid, not a second copy of the middle.** `tests/PYRAMID.md` already diagnoses KeepSave as an *inverted* pyramid — heavy unit coverage in crypto/auth, "essentially none for state-mutating endpoints" at integration, and "minimal coverage" at E2E ([`tests/PYRAMID.md:40-46`](tests/PYRAMID.md)). RF's job is to thicken the very top with black-box, human-readable acceptance scenarios — not to relitigate what Go already gates.

---

### 2. What KeepSave already has (the baseline RF must not duplicate)

| Layer | Tooling today | Coverage | Source |
|---|---|---|---|
| Unit (crypto/auth/promotion) | Go `testing`, table-driven, benchmarks | Strong; crypto is "strongest coverage" | [`tests/PYRAMID.md:13-26`](tests/PYRAMID.md) |
| Handler integration | Go `httptest` | **Dangerously thin** — `api` package: 30 code files, 3 test files, "NO handler-level integration tests" | [`tests/PYRAMID.md:15`](tests/PYRAMID.md) |
| Frontend / widget | Vitest + Testing Library + jsdom | Component-level only; no real browser, no real cross-origin, no real shadow boundary | [`frontend/package.json:40-54`](frontend/package.json) (verified: no Playwright/Selenium/RF present) |
| E2E | One Go tester + `docker-compose` (Seidr harness) | **One** scenario: register → login → create project → store secret → mint API key → fetch secret | [`tests/e2e/seidr/README.md:9-18`](tests/e2e/seidr/README.md) |

The Seidr harness is the apex today, and it proves exactly one contract. Its own README is explicit that it does **not** prove other user stories — though note the precise scope of that disclaimer (it concerns the *Seidr runtime*: circuit breaker, TTL cache, SLO ledger, key-rotation observation — [`tests/e2e/seidr/README.md:55-67`](tests/e2e/seidr/README.md)), **not** a blanket statement about KeepSave's promotion/approval/rotation user stories.

> **Fact-check flag (claim mis-cited in research).** One research item asserted that "all other multi-step user stories are untested at the acceptance level (**confirmed by README.md 'What this test does NOT prove' section**)." The *conclusion* is correct — `tests/e2e/` contains only the Seidr harness, and no Playwright/Cypress/acceptance suite exists anywhere — **but the cited evidence is wrong.** That README section is scoped to the Seidr runtime, not to KeepSave's user stories. The claim that the gap exists is backed by [`tests/PYRAMID.md:40-46`](tests/PYRAMID.md) ("minimal coverage" / "essentially none"), not by the Seidr README. Treat the *gap* as real; treat the *attribution* as corrected here.

---

### 3. Tier 1 — REST API contract tests (the clean win)

**Tooling:** [`robotframework-requests`](https://github.com/MarketSquare/robotframework-requests) (RequestsLibrary) + [`robotframework-jsonlibrary`](https://github.com/robotframework-thailand/robotframework-jsonlibrary).

KeepSave's API is JSON-over-HTTP with header auth, which maps 1:1 onto RequestsLibrary's canonical pattern (`Create Session` → extract token from login JSON → put it in an `Authorization` / `X-API-Key` header dict → `POST/GET On Session` → assert status + body). The auth model is split between a JWT bearer path and a scoped-API-key path — both routing through the same retrieval logic, which is exactly what makes black-box round-trips meaningful (this is the property the Seidr Go tester already exploits, [`tests/e2e/seidr/README.md:46-51`](tests/e2e/seidr/README.md)).

**Why this is the highest-leverage RF work:** it directly fills the gap `tests/PYRAMID.md` flags as the thinnest, highest-blast-radius layer. The negative-auth surface is already specified as a **12-endpoint × 11-attacker matrix** in [`tests/NEGATIVE_AUTH_PLAN.md:15-64`](tests/NEGATIVE_AUTH_PLAN.md) (E1–E12 × A1–A11), and that document is the natural backbone for the RF suite's negative cases.

**Critical boundary (do not get this wrong):** The 132-cell matrix is **assigned to Go** (`httptest`, sub-millisecond, in-process — pseudocode given at [`tests/NEGATIVE_AUTH_PLAN.md:66+`](tests/NEGATIVE_AUTH_PLAN.md)). RF must **not** absorb it. Re-running 132 rejection checks over `docker-compose` would be ~100–200× slower for zero new signal and would invite the flakiness `tests/FLAKY.md` exists to prevent. RF's negative coverage is limited to the handful of **story-level** negatives where multi-step context is load-bearing — expired token *mid-flow*, wrong-environment key on a promotion, replayed/duplicate promotion — the three negative paths the QA 60-day plan calls for.

---

### 4. Tier 2 — Browser E2E and the Shadow-DOM headline

**Tooling:** the [Robot Framework Browser library](https://robotframework-browser.org/) (Playwright-backed: Chromium/Firefox/WebKit, auto-waiting), **not** SeleniumLibrary.

**The load-bearing finding — the widget is pierceable because it is open.** `KeepSaveWidget` attaches its shadow root with `mode: 'open'`:

```ts
// frontend/src/embed/keepsave-widget.ts:45
this.attachShadow({ mode: 'open' });
```

Playwright's locators traverse open shadow roots **by default, with no special syntax** ([Playwright locators docs](https://playwright.dev/docs/locators)), and the Browser library inherits this. This is *necessary and sufficient*: had the widget used `mode: 'closed'`, default locators would not reach it, and the only workaround would be monkey-patching `Element.prototype.attachShadow` — confirmed by the Playwright maintainers ("there is nothing in Playwright to allow forcibly entering a closed shadow DOM root", [microsoft/playwright#23047](https://github.com/microsoft/playwright/issues/23047)).

**The widget's stable hooks are all real and reachable** (verified against [`frontend/src/embed/widget.ts`](frontend/src/embed/widget.ts)):

| Purpose | Selector | Verified at |
|---|---|---|
| Environment tabs | `.ks-tab[data-env="alpha\|uat\|prod"]` | `widget.ts:239-241` (`dataset: { env }`), read back `:394` |
| Action buttons (reveal/add/delete) | `[data-action="…"]` | `widget.ts:413` |
| Reveal/mask value | `.ks-secret-value` / `.ks-secret-mask` | `widget.ts:351-354` |
| Typed-confirm delete modal | `.ks-modal-overlay`, `[data-input="confirm-delete"]` | `widget.ts:496, 552` (modal); `:471` (`[data-input]`) |

> **Note — research was *right* on `data-env`, correcting an earlier doubt.** The env tabs render `dataset: { env }`, which the DOM exposes as the `data-env` attribute, so `.ks-tab[data-env="uat"]` is a valid Browser-library locator. (One caveat for suite authors: these `.ks-*` classes and `data-action`/`data-input` attributes are *implementation selectors*, not an advertised test API — see the open question on `data-testid` in §9.)

**Two firm shadow-DOM rules to bake into the suite:**

1. **Never use XPath for widget internals.** "Locating by XPath does not pierce shadow roots" (Playwright docs) — an XPath locator silently stops at the boundary. Use CSS / text / `getByRole` only.
2. **The open mode is now an implicit testability contract.** If anyone flips the widget to `mode: 'closed'` for hardening, the *entire* Playwright/Selenium E2E approach for the widget breaks. **Record this as an explicit constraint** ([`docs/EMBED_STATE.md`](docs/EMBED_STATE.md) or the widget's ADR / `docs/FOLLOWUPS.md`) so the change is a conscious, reviewed decision rather than a silent regression.

**Why Browser/Playwright over SeleniumLibrary for *this* widget specifically:** the widget combines *both* shadow DOM *and* cross-origin framing. SeleniumLibrary's WebDriver `getShadowRoot()` path is clunkier for nested shadow content (often forcing `Execute Javascript` to fetch the element first) and is weaker on cross-origin frames — friction on both axes for no upside. *(Confidence: medium — this rests on community reports rather than first-party docs; the open-vs-closed and XPath facts are high-confidence first-party.)*

---

### 5. The genuinely hard part: the cross-origin `postMessage` handshake

The widget's *defining security behavior* is not rendering — it's the ADR-0006 boot sequence and handshake. Verified against source:

- Boot: fetch `GET /api/v1/embed-config/:project_id` (unauthenticated — [`backend/internal/api/router.go:57,67`](backend/internal/api/router.go)); refuse if `embed_policy_enabled` is false or the parent origin isn't in `allowed_origins` ([`keepsave-widget.ts:6-13`](frontend/src/embed/keepsave-widget.ts) `EmbedConfigResponse`).
- Handshake: **strict origin equality**, silent drop on mismatch (to deny an attacker a probing oracle), and an outbound target origin that is **never `'*'`**:

```ts
// frontend/src/embed/auth.ts:54  — inbound: strict equality, silent drop
if (event.origin !== allowedOrigin) { /* warn + return */ }
// frontend/src/embed/auth.ts:77  — outbound: targeted, never '*'
window.parent.postMessage(request, allowedOrigin);
```

To exercise this **end-to-end** you must: (1) serve a host page on a **distinct second origin** from `api:8080`/`frontend:3000`; (2) register that origin in the project's allow-list via `PUT /api/v1/projects/:id/embed-config` ([`router.go:101`](backend/internal/api/router.go)); and (3) script the host page to answer the widget's `keepsave-auth-request` with a `keepsave-auth` token. Playwright/Browser handles cross-origin frames well (per-frame contexts; `frameLocator`; the Browser library's `>>>` frame-piercing combinator) — better than Selenium — **but the harness plumbing is the cost**, and it is the only genuinely hard piece of this whole effort.

**Keep the *negative*-origin behavior at the unit layer.** "Assert that nothing happened" (wrong origin → silently dropped, no DOM change) is an inherently weak, slow E2E test. The existing Vitest `auth.test.ts` already covers wildcard-origin refusal, masked-by-default, and `visibilitychange` re-masking on jsdom — that is where these belong. RF E2E should assert the **positive** path: a correctly-configured host completes the handshake and a scoped secret round-trips.

---

### 6. Explicit "RF must NOT test" list (enforce at PR review)

This is a hard boundary, not a guideline. RF asserts only **HTTP status codes, JSON response shapes, and audit-log API responses for multi-call scenarios**. It must not touch:

| Forbidden in RF | Why | Owned by |
|---|---|---|
| Crypto correctness (AES-GCM round-trip, ciphertext bytes, key/nonce length) | Black-box can't see encryption-at-rest; re-implementing duplicates and risks contradicting the gate | Go crypto unit + `go test -fuzz` ([`tests/PYRAMID.md:60,68`](tests/PYRAMID.md): crypto ≥90% line / ≥85% branch) |
| In-process middleware rejection (the 132-cell matrix) | Parametric, sub-ms `httptest`; 100–200× slower in RF for no new signal | Go ([`tests/NEGATIVE_AUTH_PLAN.md`](tests/NEGATIVE_AUTH_PLAN.md)) |
| Promotion business rules (env order, override policy, self-approval) | Pure Go logic | Go service tests |
| Frontend component behavior / DOM logic | Already Vitest; RF over the rendered DOM is fragile for little gain | Vitest (`widget.test.ts`, `PromotionWizard.test.tsx`) |
| Repository round-trips, audit-chain hash verification | In-process DB harness territory | Go repository tests |

**One nuance on the audit-log rule.** `CLAUDE.md` mandates that every state-mutating handler's test "MUST assert the audit row was written," and `docs/AUDIT_LOG_COVERAGE.md` requires that assertion to live in the **Go handler test**. RF must **not** be treated as satisfying that gate. RF *may* additionally assert audit *visibility* at the acceptance layer (i.e., `GET /audit-log` returns the expected event after a multi-step flow) — this **supplements** the Go gate as live documentation of the contract; it does not replace or weaken it.

---

### 7. Placement: integrated at `tests/robot/` (decisive)

**Recommendation: integrated — `tests/robot/` inside this repo.** Reject a separate repo and a top-level `/qa`.

The decisive precedent is `tests/e2e/seidr/`: a self-contained harness that ships its own [`docker-compose.yml`](tests/e2e/seidr/docker-compose.yml) (builds the backend from `context: ../../../backend` against an ephemeral `postgres:16-alpine`), an isolated Go module under `tester/`, and is run via `docker compose up --build --abort-on-container-exit --exit-code-from tester` ([`tests/e2e/seidr/README.md:24-29`](tests/e2e/seidr/README.md)). A RF suite at `tests/robot/` is the *same shape* with a Python runner.

| Option | Verdict | Reasoning |
|---|---|---|
| **(a) `tests/robot/` in-repo** | ✅ **Recommended** | Matches the Seidr precedent; `tests/` is already the QA home (PYRAMID/NEGATIVE_AUTH/FLAKY all live there); atomic API↔test PRs; one SHA-pinned CI; can dogfood the in-repo Python SDK via `pip install -e ../../sdks/python` |
| (b) top-level `/qa` | ❌ | Fragments the QA test home for no benefit |
| (c) separate repo | ❌ | Breaks atomic revert and API↔test PRs; duplicates governance/CODEOWNERS/CI-pinning; contradicts the Seidr precedent. Its *only* selling point — isolation — is already achieved by a dedicated `docker-compose.e2e.yml` + `requirements.txt` |

**The "standalone" property KeepSave actually values is process isolation, not repo isolation** — satisfied by a dedicated compose file and a pinned `requirements.txt`, exactly as Seidr isolates via its own `go.mod`.

**Suggested layout** (mirrors the official RF `tests/` + `resources/` split):

```
tests/robot/
  README.md                 # run command + what it proves / does NOT prove (Seidr-README style)
  requirements.txt          # PINNED: robotframework, robotframework-browser, robotframework-requests; -e ../../sdks/python (optional SDK dogfood)
  docker-compose.e2e.yml    # db + api + frontend (reuse root services) + a `robot` runner
  Dockerfile                # FROM marketsquare/robotframework-browser; pip install -r requirements.txt; rfbrowser init
  __init__.robot            # Suite Setup: poll /readyz, register+login bootstrap (mirrors seidr main.go)
  resources/keywords/       # auth / secrets / promotion / audit / ui (.resource files)
  resources/variables/      # ${BASE_URL}=http://api:8080  ${UI_URL}=http://frontend:3000
  tests/api/                # secret_roundtrip, negative_auth (story-level only), promotion
  tests/ui/                 # login_flow, promotion_wizard, embed_widget
  results/                  # output.xml/log.html/report.html — gitignored
```

The runner targets services by **compose DNS** (`http://api:8080`, `http://frontend:3000`), not host ports — port-conflict-free in CI, per `tests/FLAKY.md` discipline (no hard-coded ports, poll `/readyz` rather than sleep, no shared state between cases).

---

### 8. CI wiring — the load-bearing step

> **The gap that proves the point.** `tests/e2e/seidr/` is **not referenced anywhere in `.github/workflows/ci.yml`** (grep for `seidr|e2e|robot|compose` returns *nothing*). The harness exists but nothing runs it — so it can rot silently, violating PYRAMID's own definition-of-done: "It runs in CI. (Locally-only tests rot.)" ([`tests/PYRAMID.md:83`](tests/PYRAMID.md)). **Lesson for RF: adding the suite and gating on the suite are two distinct deliverables, and the second is the one that matters.**

CI today is a single least-privilege pipeline: top-level `permissions: contents: read`, every `uses:` pinned to a full commit SHA with a tag comment ([`.github/workflows/ci.yml:9-25`](.github/workflows/ci.yml)). A RF job is a clean additive fit:

```yaml
  e2e-robot:
    name: E2E (Robot Framework)
    runs-on: ubuntu-latest
    needs: [backend-build, frontend-build]   # only after images build
    permissions:
      contents: read                          # no elevation needed
    steps:
      - uses: actions/checkout@<pinned-sha>   # reuse the SHA already in this file
      - name: Run Robot E2E suite
        run: >
          docker compose -f tests/robot/docker-compose.e2e.yml up
          --build --abort-on-container-exit --exit-code-from robot
      - name: Upload Robot reports
        if: always()                          # publish log.html even on failure for triage
        uses: actions/upload-artifact@<pinned-sha>
        with:
          name: robot-e2e-report
          path: tests/robot/results/
```

**Trigger/latency policy** (this dissolves the only real argument for a separate repo): if RF E2E proves slow or flaky, scope **when** it runs, not **where it lives** — all three are one-repo CI-config changes:
1. Full gate on every PR (start here);
2. `needs:` + path/label filter so it only blocks PRs touching `backend/`, `frontend/`, or `tests/robot/`;
3. Scheduled nightly, PRs unblocked, promoted back to PR-gating after 20 stable runs (per `tests/FLAKY.md`'s "<99% pass over last 20 runs" definition).

**Opportunistic adjacent fix:** in the same change, wire the *existing* Seidr harness into `ci.yml` too (identical `docker compose … --exit-code-from tester` invocation), closing the un-gated gap and aligning with `docs/FOLLOWUPS.md` #8, which already assigns E2E-harness ownership to **"DevOps (image publishing) + QA (harness)."**

---

### 9. Minimum viable suite, governance, and open questions

**Minimum viable set (8–12 suites, one per named user story)** — one acceptance scenario apiece, readable by QA/PM, mapping to the ROLES_30_60_90 §6 QA-60-day deliverables:

1. Secret lifecycle (create/read/update/delete) + audit-visibility assertion after each
2. Promotion happy path Alpha→UAT with two distinct users + audit row confirmed
3. Promotion negative: expired token mid-flow
4. Promotion negative: wrong-environment key
5. Promotion negative: replayed/duplicate promotion
6. API-key expiry: short-TTL key → wait → 401
7. Promotion kill-switch: `KEEPSAVE_PROMOTIONS_ENABLED=false` → 503 on promote/approve, normal on reject/rollback/reads (per `CLAUDE.md` env-var table and `docs/RUNBOOK.md` §8)
8. Multi-party approval: self-approval attempt → 403 (ADR-0003 invariant, mirroring `NEGATIVE_AUTH_PLAN.md` case A10)

**Governance:** This is a **Type-2** change under `CLAUDE.md` §3.1 (new test tooling / CI job; no crypto/auth/promotion *code* change) — standard PR review, **no ADR required**. Register it in `docs/FOLLOWUPS.md` next to #8 as a tracked Phase-A QA item, add `tests/robot/` to CODEOWNERS under QA, and seed `tests/robot/tests/api/negative_auth.robot` directly from the E1–E12 × A1–A11 matrix so the two artifacts stay in lockstep. Ownership: **QA authors the keywords; Backend is on call for harness plumbing** — the `who-writes-the-spec` / `who-owns-the-mechanics` split `docs/ROLES.md` intends.

**Open questions (decide before building):**

| # | Question | Note |
|---|---|---|
| 1 | **Does CI want a third toolchain (Python RF + Playwright binaries) on top of Go + Node/Vitest?** Or write the same browser E2E directly in **Playwright-test (TS)** — same ecosystem as the existing Vite/Vitest frontend? | The central cost/benefit call. RF's keyword syntax helps **non-dev QA authors**; if the authoring audience is engineers already in TS, Playwright-test may be the better fit. *Which audience KeepSave optimizes for is unverified.* |
| 2 | Should the Seidr Go round-trip be **re-expressed as RF** (one E2E framework) or **kept in Go** (it mirrors Seidr's own Go provider contract)? | Recommendation: keep Seidr in Go, add RF as the *broader* acceptance layer. Consolidation is a viable alternative; team preference unknown. |
| 3 | Add dedicated **`data-testid`** hooks to the widget so refactors don't silently break E2E selectors? | The suite currently leans on `.ks-*` classes / `data-action` / `data-input`, which are implementation details, not an advertised test API. *Low-confidence that current selectors are stable enough long-term.* |
| 4 | **Exact pinned versions** of `robotframework` / `-browser` / `-requests`. | **To-pin at implementation time** — specific version numbers were *not verified against a live index* (proxy 403 on package docs). Low-confidence on numbers; high-confidence the libraries exist and are Playwright/Requests-backed. |
| 5 | Does the frontend container serve the widget bundle at a **stable path** an RF Browser test can load into a host page? | README shows `<script src=.../embed/keepsave-widget.js>` usage, but the served asset path under the frontend container build was **not confirmed end-to-end.** |

---

### 10. Confidence ledger

| Claim | Confidence | Basis |
|---|---|---|
| Widget uses `mode: 'open'` at `keepsave-widget.ts:45`; this is what makes it pierceable | **High** | Direct read of source; corroborated by `docs/system/08-embed-widget.md`, `THREAT_MODEL.md`, Playwright #23047 |
| Playwright/Browser pierces open shadow roots by default; XPath does not; closed is unsupported | **High** | First-party Playwright docs + maintainer thread #23047 |
| API is JSON + JWT/`X-API-Key`; RequestsLibrary fits 1:1 | **High** | Source (`router.go`, `auth.ts`); Seidr tester already does this round-trip |
| `api` package has 30 code / 3 test files, "NO handler-level integration tests" | **High** | [`tests/PYRAMID.md:15`](tests/PYRAMID.md) (direct read) |
| Negative-auth surface is a 12×11 Go-assigned matrix | **High** | [`tests/NEGATIVE_AUTH_PLAN.md`](tests/NEGATIVE_AUTH_PLAN.md) (direct read) |
| Frontend is Vitest+jsdom only; RF/Playwright/Selenium absent | **High** | [`frontend/package.json`](frontend/package.json) (direct read) |
| Seidr harness exists in-repo, self-contained compose; **not wired into CI** | **High** | Direct read of files; grep of `ci.yml` returns no match |
| Widget selectors (`.ks-tab[data-env]`, `[data-action]`, `[data-input]`, `.ks-secret-value/mask`, `.ks-modal-overlay`) are all real | **High** | Direct read of [`widget.ts`](frontend/src/embed/widget.ts) (lines cited) |
| SeleniumLibrary is materially worse for nested shadow + cross-origin | **Medium** | Community reports, not first-party docs |
| Specific pinned RF library versions | **Low** | Not verified against a live index (proxy 403) — mark **to-pin** |
| "Seidr README confirms all other user stories untested at acceptance level" | **Corrected** | Conclusion true (backed by PYRAMID.md); the README citation is wrong — README scope is the Seidr *runtime* |

---

Files referenced (absolute paths): `/home/user/KeepSave/frontend/src/embed/keepsave-widget.ts`, `/home/user/KeepSave/frontend/src/embed/auth.ts`, `/home/user/KeepSave/frontend/src/embed/widget.ts`, `/home/user/KeepSave/frontend/package.json`, `/home/user/KeepSave/tests/PYRAMID.md`, `/home/user/KeepSave/tests/NEGATIVE_AUTH_PLAN.md`, `/home/user/KeepSave/tests/e2e/seidr/docker-compose.yml`, `/home/user/KeepSave/tests/e2e/seidr/README.md`, `/home/user/KeepSave/backend/internal/api/router.go`, `/home/user/KeepSave/.github/workflows/ci.yml`.

---

<a id="appendix--fact-check-verdicts"></a>
## Appendix — fact-check verdicts

One load-bearing claim per research stream was independently fact-checked (default-skeptical, web-sourced). Spread: {"partly":10,"supported":4,"refuted":1,"unverifiable":1}.

| Stream | Verdict | Claim |
|--------|---------|-------|
| comp_pricing_a | partly | Doppler Team plan costs $21/user/month (monthly billing); SAML SSO is included at Team tier — not gated at Enterprise. |
| comp_pricing_b | partly | HCP Vault Secrets EOL: end-of-sale June 30 2025, end-of-life July 1 2026. Its per-secret SaaS pricing ($0.50/secret/mo, 25-secret free tier) no longer |
| pricing_model | partly | As of June 2026, Doppler charges per HUMAN seat (Team $21/user/mo) and does NOT charge for service accounts / machine identities; Free = 3 users + 3-d |
| revenue_expansion | partly | Infisical prices per-IDENTITY at $18/identity/month where a machine/agent identity counts the same as a human, explicitly so the bill 'scales with inf |
| icp_positioning | partly | The 'secrets-for-AI-agents' / non-human-identity category is real and fast-growing in 2026: KPMG 2026 frames NHIs as a top CISO priority (~80:1 over h |
| launch_plan | partly | Console.dev features 2-3 developer tools per week and 68% of readers sign up for featured tools — source: console.dev (verified via web search) |
| content_community | supported | GitGuardian State of Secrets Sprawl 2026: 29M new hardcoded secrets on public GitHub in 2025, up 34% YoY; AI-service secret leaks up 81% (source: blog |
| competitive_narrative | partly | KeepSave ships an embeddable <keepsave-widget> Web Component with Shadow DOM that has no direct analog among the 13 researched competitors (PATTERN_MA |
| seo | partly | AI-service credential leaks grew 81.5% in 2025; 24,008 unique secrets were found in MCP config files on public GitHub — a citable, current authority s |
| sem | partly | DevTools SaaS average CPC is $7–$9; cybersecurity/secrets management category terms run $16–$22; competitor evaluation queries run $50–$200 in 2026. S |
| geo | supported | The Princeton-led 'GEO: Generative Engine Optimization' paper (Aggarwal et al., arXiv 2311.09735, ACM KDD/SIGKDD 2024) introduced GEO-bench (~10,000 q |
| analytics | refuted | AI-referred sessions grew 527% YoY; AI platforms generated 1.13 billion referral visits in June 2025 (Yotpo / Adobe Analytics, 2025) |
| rf_fit | supported | KeepSave's <keepsave-widget> attaches its shadow root with mode:'open' at keepsave-widget.ts:45, which is what makes it pierceable by Playwright/Brows |
| rf_standalone | supported | `tests/e2e/seidr/` exists in-repo and is a self-contained harness: it has its own docker-compose.yml that builds the backend from context ../../../bac |
| rf_impl | unverifiable | "k" |
| rf_vs_stack | partly | "The seidr Go E2E harness (tests/e2e/seidr/) exercises exactly one scenario: register + login + create project + store secret + mint API key + fetch s |

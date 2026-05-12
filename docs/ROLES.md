# KeepSave — Project Roles & Operating Model

This document defines the roles needed to manage and continue developing KeepSave. The design is biased toward **critical thinking** (challenge every assumption, especially security-related) and **systematic thinking** (decisions are traceable, reversible where possible, and grounded in the threat model).

KeepSave handles other people's secrets. That single fact drives the entire org design: every role has an explicit security responsibility, and no role can ship a change to PROD alone.

---

## 1. Operating Principles

These apply to every role and are the basis for performance reviews and hiring loops.

1. **Threat model first.** Every feature epic begins with a delta to `docs/THREAT_MODEL.md`. If you can't describe the new attack surface, you're not ready to design.
2. **ADRs for irreversible decisions.** Encryption scheme, key hierarchy, auth model, data retention — write it down in `docs/adr/NNNN-*.md` with alternatives considered and the rejection rationale.
3. **Blast radius before convenience.** Always ask: if this change is wrong, what is the worst outcome? Who is affected? Can it be rolled back? If the answer is "PROD secrets leak and we can't recover," the change does not ship without a second pair of eyes and a rollback plan.
4. **No silent fallbacks.** Code paths that "try harder" on failure (retry, fallback to cache, default to allow) must be explicit and reviewed. Security defaults are deny.
5. **Audit-log first, feature second.** If an action is not in the audit log, it didn't happen — and the feature isn't done.
6. **Reversibility budget.** Each engineer carries a budget for irreversible actions (DB migrations, key rotations, schema breaking changes). Spending it requires written sign-off.

---

## 2. Core Roles

Roles are listed with: **mandate**, **owned artifacts**, **critical-thinking expectations**, and **systematic-thinking expectations**.

### 2.1 Tech Lead / Staff Engineer
- **Mandate:** End-to-end technical coherence across backend, frontend, crypto, and infra. Final reviewer for ADRs.
- **Owned artifacts:** `docs/adr/`, architecture diagrams, RFC template, Roadmap technical sequencing.
- **Critical thinking:** Reject features whose threat model isn't worked out. Challenge every "we'll fix it later" — write the follow-up into `docs/FOLLOWUPS.md` with a date.
- **Systematic thinking:** Maintain a dependency map of components (crypto → repository → service → API → frontend → embed SDK). New work is plotted on this map before estimation. Quarterly architectural review.

### 2.2 Security Engineer *(non-negotiable for this product)*
- **Mandate:** Own the threat model, key hierarchy, crypto correctness, and pentest cadence. Veto power on anything touching `internal/crypto`, `internal/auth`, or promotion flows.
- **Owned artifacts:** `docs/THREAT_MODEL.md`, `docs/PENTEST_CHECKLIST.md`, `SECURITY_AUDIT.md`, key rotation runbook.
- **Critical thinking:** For every PR touching secrets, ask: (a) does this widen the trust boundary? (b) is plaintext ever logged, returned in errors, or held in memory longer than needed? (c) can a downgraded/forged token reach this code path?
- **Systematic thinking:** STRIDE pass on each new endpoint. Pentest checklist re-run before each minor release. Quarterly key-rotation drill.

### 2.3 Backend Engineer (Go)
- **Mandate:** API handlers, services, repositories, promotion engine, migrations.
- **Owned artifacts:** `backend/internal/**`, `backend/migrations/`, Go test suites.
- **Critical thinking:** Treat every error as a security signal first, UX signal second. Don't return raw DB or crypto errors to clients. Question whether a new endpoint should exist at all — fewer surfaces = fewer attacks.
- **Systematic thinking:** Table-driven tests for every branch in promotion logic. Each new endpoint comes with: handler test, service test, repo test, audit-log assertion, and a negative-auth test.

### 2.4 Frontend Engineer (React + Embed SDK)
- **Mandate:** Dashboard UX, embeddable `<keepsave-widget>`, embed SDK security posture.
- **Owned artifacts:** `frontend/src/**`, embed widget contract, postMessage protocol spec.
- **Critical thinking:** The widget runs on third-party origins. Question every DOM read, every event listener, every storage call. Default to no cross-origin trust. Plaintext secrets must never persist in browser storage.
- **Systematic thinking:** Maintain a state diagram for the widget (unauthenticated → authenticated → viewing → revealed). Each transition has an audit-log requirement. Visual regression tests on the dashboard for security-sensitive states (revealed secret, promotion confirmation).

### 2.5 DevOps / Platform Engineer
- **Mandate:** CI/CD, infra-as-code, deployment of KeepSave itself, secret management for the KeepSave deployment (chicken-and-egg solved via KMS).
- **Owned artifacts:** `docker-compose.yml`, `helm/`, `scripts/`, CI workflows, deployment runbook.
- **Critical thinking:** The deployment of KeepSave is itself a secrets problem. Where does `MASTER_KEY` live? Who can read it? What is the blast radius if a CI runner is compromised?
- **Systematic thinking:** Every environment (dev, staging, prod) has a documented secret-source map. CI pipelines have least-privilege tokens. Rollback path is tested before each release.

### 2.6 QA / Test Engineer
- **Mandate:** Test strategy across unit, integration, E2E, and security/negative tests.
- **Owned artifacts:** `tests/`, test matrices, regression suite, fuzz harness for crypto edges.
- **Critical thinking:** Spend most effort on negative paths: expired tokens, wrong-environment keys, replayed promotions, malformed payloads, race conditions on promotion. Happy-path coverage is the cheap part.
- **Systematic thinking:** Test pyramid documented; new features cannot merge without entries at all relevant layers. Track flaky tests as bugs, not annoyances.

### 2.7 Product Manager
- **Mandate:** Prioritization, stakeholder alignment, roadmap discipline.
- **Owned artifacts:** `Roadmap.md`, phase changelogs, customer feedback log.
- **Critical thinking:** Ask "what are we choosing not to build, and why?" for every quarter. Push back on features that increase trust surface without commensurate user value.
- **Systematic thinking:** Each phase has explicit exit criteria (e.g., "all promotion endpoints have audit-log assertions"). No phase is "done" by date — only by criteria.

### 2.8 UX / UI Designer
- **Mandate:** Dashboard flows, widget design, security-sensitive UI states.
- **Owned artifacts:** Design system, UX specs for revealed-secret, promotion-confirm, error states.
- **Critical thinking:** Bad security UX is a security bug. Confirmation dialogs that users learn to dismiss are worse than no dialog. Error messages must inform without leaking.
- **Systematic thinking:** State inventory for every screen (loading, empty, error, denied, success). Sign-off from Security Engineer for any UI showing plaintext secrets.

### 2.9 Technical Writer / Developer Advocate
- **Mandate:** API docs, integration guides for AI Agents, embed widget docs, security policy public page.
- **Owned artifacts:** `README.md`, `docs/*_INTEGRATION.md`, public security posture page.
- **Critical thinking:** Examples in docs must follow the same security rules as production code (no plaintext in shell history, no echo of secrets). Audit every code sample.
- **Systematic thinking:** Docs version-locked to API version. Every breaking change has a migration guide.

---

## 3. Decision-Making Process

This is the spine that turns roles into systematic outcomes.

### 3.1 Three Kinds of Decisions

| Class | Examples | Required Process |
|-------|----------|------------------|
| **Type-1 (irreversible / high-blast)** | Encryption scheme, schema breaking change, removing audit fields, PROD key rotation | ADR + Security Engineer sign-off + Tech Lead sign-off |
| **Type-2 (reversible / contained)** | New endpoint, new UI component, dependency upgrade | RFC if non-trivial; standard PR review otherwise |
| **Type-3 (local / trivial)** | Refactor within a package, doc edit | Standard PR review |

Misclassification (treating a Type-1 as Type-2) is itself a bug and goes into the post-mortem queue.

### 3.2 RFC / ADR Lifecycle

1. **Problem statement** — what is broken or missing, and for whom.
2. **Constraints** — including threat-model constraints.
3. **Options** — at least two, with honest tradeoffs.
4. **Decision** — and explicit *rejection rationale* for the others.
5. **Rollback plan** — what we do if this turns out wrong.
6. **Open questions** — written down, not pretended away.

### 3.3 Post-mortems
Blameless, mandatory for any incident touching customer secrets or auth. Output is action items with owners and due dates, tracked in `docs/FOLLOWUPS.md`.

---

## 4. Team Composition by Phase

The roadmap goes through phases; the team should grow against them, not ahead of them.

### Phase A — MVP hardening (current → Phase 4-ish on Roadmap)
- 1 Tech Lead
- 1 Backend Engineer (Go)
- 1 Frontend Engineer
- **Security Engineer: part-time / fractional, but real veto power**
- DevOps: part-time
- QA: shared with backend/frontend

### Phase B — Multi-tenant / approval workflows
- + 1 Backend Engineer
- + Dedicated DevOps / Platform
- + Dedicated QA
- + Product Manager
- Security Engineer goes full-time

### Phase C — Embed SDK GA, public launch
- + 1 Frontend / Embed SDK specialist
- + UX Designer
- + Technical Writer
- On-call rotation formalized

The temptation will be to hire ahead of phase B. Resist it — every new person on a secrets product is a new trust grant.

---

## 5. Hiring Filters for Critical & Systematic Thinking

Use these in interview loops, regardless of role:

1. **"Tell me about a time you talked your team *out* of building something."** — filters for critical thinking that survives social pressure.
2. **"Walk me through a system you designed end-to-end. Where is it weakest today?"** — strong candidates know the weak spots without prompting.
3. **Threat-model exercise:** present a one-page feature spec; ask the candidate to enumerate attacker goals, trust boundaries, and mitigations. Score on coverage *and* on what they explicitly chose not to mitigate (and why).
4. **Incident retro:** give a fake post-mortem; ask what's missing. Look for root-cause depth vs. "add more tests."
5. **Reversibility question:** "Which of these decisions would you not make on a Friday afternoon, and why?"

---

## 6. Cross-Cutting Rituals

- **Weekly threat-model review** (30 min): Security Engineer + Tech Lead walk new PRs against current threat model.
- **Monthly ADR review:** revisit decisions older than 90 days; mark superseded or still-valid.
- **Quarterly pentest checklist run** against staging.
- **Per-release rollback drill** in staging before tagging.
- **Brown-bag rotation:** each engineer presents one part of the system per quarter — forces shared understanding and surfaces hidden assumptions.

---

## 7. Anti-patterns to Avoid

- "We'll add the audit log later." — No. The audit log is the feature.
- "It's behind auth, so we don't need to validate input." — Defense in depth, always.
- "Let's just rotate the key in PROD quickly." — Rotation is a rehearsed drill, never an improvisation.
- Roles defined by titles instead of owned artifacts. If a role can't point to files or processes it owns, the role isn't real.
- Hiring a Security Engineer "later." On a secrets product, security is not a phase — it's the substrate.

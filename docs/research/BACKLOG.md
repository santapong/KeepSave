# Post-Initiative Backlog

- **Initiative closed:** 2026-05-15 (research branch `claude/research-auth-competitors-H8VmQ` merged via PR #48)
- **Purpose:** canonical post-Day-90 tracker for everything the research initiative surfaced but did not itself implement.
- **Owner:** acting PM (interim Tech Lead).
- **Cross-references:** `docs/FOLLOWUPS.md` (Phase-A items), `docs/audits/` (audit findings), `docs/adr/0005`-`0015` (proposed ADRs), `docs/research/EXTERNAL_REVIEW_PREP.md` (pentest scope).

This document captures **what the initiative produced + what comes next.** Each entry has: owner, priority, dependency, and trigger-to-close.

---

## Section 1 — ADR sign-offs pending (Type-1 gate)

All 11 ADRs from this initiative are at status `proposed`. Per `docs/ROLES.md`, Type-1 changes require **Security Engineer veto + Tech Lead sign-off** before merge to `main`. Two implementation PRs (#49, #50) shipped under sponsor-override authorization; sign-off is still required for the ADRs themselves to flip from `proposed` to `accepted`.

| ADR | Topic | Type | Veto holder | Status |
|---|---|---|---|---|
| 0005 | RequireProjectAccess middleware | Type-1 | Security Engineer | `proposed` — closes 9 of 11 P1 IDORs in one PR |
| 0006 | Embed widget origin allow-list | Type-1 | Security Engineer | `proposed` — implementation landed in PR #50 (sponsor override) |
| 0007 | Approver ≠ requester DB invariant | Type-1 | Security Engineer + Tech Lead | `proposed` — closes FU 0d + audit P2 |
| 0008 | RS256 + JWKS rotation | Type-1 | Security Engineer | `proposed` |
| 0009 | Default `ks_` API key expiration | Type-1 | Security Engineer | `proposed` — closes FU 0k |
| 0010 | MCP gateway command-execution hardening | Type-1 | Security Engineer | `proposed` — closes audit P1 A03-F1 (RCE primitive) |
| 0011 | Graceful shutdown + DB context timeouts | Type-1 | Tech Lead | `proposed` — prereq for 0012 |
| 0012 | KMS auto-unseal | Type-1 | Security Engineer | `proposed` — depends on 0011; closes FU 1 |
| 0013 | Webhook emission with atomic SSRF guard | Type-1 | Security Engineer | `proposed` — closes Infisical Cand. 1 |
| 0014 | Audit-log taxonomy extension | Type-1 | Security Engineer | `proposed` — adds `role.changed`, `settings.changed`, `actor_type` |
| 0015 | `safego` helper + per-use API-key audit emission | Type-1 | Security Engineer | `proposed` — closes FU 0l |

**Action:** Tech Lead schedules ADR review sessions. Suggested batching: review 0011 + 0012 together (dependency); review 0005 + 0007 + 0009 together (auth surface); review 0006 + 0010 together (RCE-class hardening); 0008 + 0014 + 0015 individually.

---

## Section 2 — Implementation queue

Order by severity × leverage. Each implementation lives on a **separate branch off `main`**, not on the research branch. Each is a Type-1 PR that requires ADR sign-off first (or explicit sponsor override).

### P0 — must implement (active production exposure)

| Item | What it closes | File:line touchpoints | Effort | Trigger to close |
|---|---|---|---|---|
| **ADR-0005 impl** | 9 of 11 P1 IDORs from `SECURITY_AUDIT_2026-05-15.md` (A01-F1 through F10) | New middleware in `backend/internal/api/middleware.go`; mount on every `/projects/:id/*` route group; ~6 handlers updated | Medium (1 PR, ~10 files) | PR merged + IDOR re-test passes |
| **ADR-0010 impl** | Audit P1 A03-F1 RCE primitive; 6 critical goroutine-no-recover findings | `backend/internal/api/handlers_mcp*.go` + new `runtime/safego.go` (co-owned with 0011/0015); interpreter allowlist; sandbox exec; bounded worker pool | Large (1 PR, ~15 files) | PR merged + RCE re-test passes |

### P1 — should implement (Phase A close-out)

| Item | What it closes | Dependency | Effort |
|---|---|---|---|
| **ADR-0007 impl** | FU 0d + audit A04-F1 approver=requester unenforced | None | Small (1 PR, ~5 files: migration + service guard + test) |
| **ADR-0009 impl** | FU 0k convergent finding (Vault + SPIFFE + GitHub) | UX picker (separate Frontend Engineer + UX Designer ticket) | Small (1 PR, ~5 files) |
| **ADR-0011 impl** | 3 critical SPOFs (no graceful shutdown, 182 DB sites no context, Slowloris) | None (foundational) | Large (1 PR, ~20 files + repo-layer ctx propagation) |
| **ADR-0012 impl** | FU 1 KMS adapters unwired | **Depends on ADR-0011 impl** (drain semantics) | Medium (1 PR, ~6 files) |
| **ADR-0013 impl** | Infisical Cand. 1 + audit A10-F1 webhook SSRF + SPOF webhook retry-body-bug | None | Large (1 PR, ~12 files; 7-part decision) |
| **ADR-0015 impl** | FU 0l `c.MustGet` 58-site refactor + audit Risk 11 | Shares `safego.go` with ADR-0011 impl | Medium (1 PR, refactor 58 sites + audit emission middleware) |

### P2 — defensive in depth (no active exploit)

| Item | What it closes | Why P2 |
|---|---|---|
| **ADR-0008 impl** | Empty JWKS stub at `handlers_oauth.go:216-218` | No customer is currently consuming the JWKS endpoint; HS256 is functional today. Prepares for future federation. |
| **ADR-0014 impl** | Audit-log taxonomy gaps (`role.changed`, `settings.changed`, `actor_type`) | Operational improvement; not security-critical. Reserves namespace for 0012/0013 audit events. |

---

## Section 3 — Phase-A FU items not addressed by ADRs

These are tracked in `docs/FOLLOWUPS.md` but do not have a corresponding ADR. Most are Type-2 or Type-3 — owner-driven, no Security Engineer veto required.

| FU | Topic | Owner | Notes |
|---|---|---|---|
| **0a** | Error responses leak DB / crypto internals | Backend Engineer | Standard `httperror` wrapping; mostly mechanical |
| **0b** | Embed widget accepts auth from any origin | Frontend Engineer | **Closed by PR #50** (ADR-0006 impl) once merged |
| **0c** | No handler-level negative-auth tests | QA + Backend Engineer | Post-Option-C "Option E" candidate; closes part of audit A01 chain |
| **0d** | Approver-cannot-be-requester invariant | Backend + Security Engineer | **Closed by ADR-0007 impl** |
| **0e** | Promotion feature flag / kill switch | Backend Engineer | **Closed 2026-06-09** as Type-2 (runtime env flag, not architectural — per this row's recommendation): `KEEPSAVE_PROMOTIONS_ENABLED` + `internal/api/promotion_gate.go`; see `docs/FOLLOWUPS.md` "Closed (2026-06-09)" |
| **0f** | CI workflow lacked explicit `permissions:` block | DevOps | Independent; touches `.github/workflows/*.yml` |
| **0g** | CODEOWNERS + commit-message convention | Tech Lead | Independent; touches `.github/CODEOWNERS` |
| **0h** | Dashboard JWT storage key-name inconsistency | Frontend Engineer | **Closed by PR #49** (Task B) once merged |
| **0i** | Auto-hide timer on revealed secrets | Frontend + UX | **Closed by PR #49** (Task C) once merged |
| **0j** | Destructive actions use `window.confirm` | Frontend + UX | **Closed by PR #49** (Task D — `TypedConfirmModal`) once merged |
| **0k** | Default expiration on `ks_` API keys | Backend + Security Engineer | **Closed by ADR-0009 impl** |
| **0l** | `c.MustGet` 58-site refactor | Backend Engineer | **Closed by ADR-0015 impl** |
| **FU 1** | AWS / GCP KMS adapters wired | DevOps + Backend | **Folded into ADR-0012 impl** |
| **FU 2** | Backup tamper-detection test | QA | Independent |
| **FU 3** | DEK rotation API | Backend | Independent; possibly future ADR |
| **FU 6** | Phase 15 service unit tests | Backend | Independent |
| **FU 7** | Nonce-collision monitoring per DEK | Backend | Independent; `safego` helper from ADR-0011 may be helpful |
| **FU 8** | Seidr-runtime boot in E2E harness | DevOps + QA | Independent |
| **FU 9** | Audit-log assertion coverage gaps | Backend | Adjacent to ADR-0014 + 0015 |
| **FU 10** | FIPS-mode evaluation | PM / Tech Lead | Decision only; no implementation yet |

---

## Section 4 — Operational backlog (this session surfaced)

| Item | Owner | Priority | Notes |
|---|---|---|---|
| **CI infrastructure repair** | DevOps | **P0** | All 3 session PRs (#48, #49, #50) blocked on infrastructure-class CI failures completing in 2-3 seconds (bootstrap/runner config). Real `go test` and `vitest` runs pass locally. Likely fix lives in `.github/workflows/*.yml` or runner config. Until fixed, all PRs need `--admin` override to merge. |
| **Migration coexistence cleanup** | Backend | P1 | `backend/migrations/007_phase15_ai_intelligence.sql` (base-dir) and `backend/migrations/{postgres,mysql,sqlite}/007_embed_origin_allowlist.sql` (PR #50) both claim number `007`. No active-code-path collision per `migrate.go` (subdir wins when present), but operationally confusing. Rename one. |
| **THREAT_MODEL.md drift** | Security Engineer | P2 | Line-number references in dossiers + ADRs drifted from `82/85` to `86/89` and `83/84` to `87/88` during the audit-delta commit (`4286d0a`). 4 reviewers caught + corrected drift. Suggest pinning STRIDE rows by ID rather than line number. |
| **Infisical dossier "Python-only SDK" claim** | Research Lead / future SR re-pass | P3 | Factual error caught by 1Password Connect P1 reviewer (commit `cc523d3`): KeepSave ships `sdks/go/keepsave.go` (749 LoC) and `sdks/nodejs/src/index.ts` (577 LoC), not Python-only. Infisical dossier is accepted; correction needs a future SR re-pass. |
| **BEYOND.md §2.6 sub-bullet for Teleport Cand. 2** | Research Lead | P3 | Teleport reviewer (commit `cc523d3`) flagged that session-recording-for-AI-agents needs its own trigger anchor under §2.6, not §2.5. Required to make Teleport Cand. 2's §10 trigger contract satisfiable. |
| **Documentation: dead-UI audit doc accessibility** | Tech Writer | P3 | The Agent B implementation worktree branched from `main` couldn't see `docs/audits/FRONTEND_DEAD_UI.md` (it lives on the research branch). Once #48 merges this resolves itself. Until then, audit references in PR #49 are forward refs. |

---

## Section 5 — Research extensions (lower priority)

| Item | Owner | When |
|---|---|---|
| **P2 dossiers** (Okta, Auth0, AWS Cognito, Azure Entra, AWS SM, GCP SM, Bitwarden SM, FusionAuth, Authelia, OAuth2-proxy, ArgoCD/Spinnaker/Octopus) | Research Lead | Open-ended; pick when a customer signal forces specific vendor knowledge |
| **BEYOND memo v2** | Research Lead | After ADRs 0005-0015 land in production — incorporate implementation learnings |
| **Pattern matrix v3** | Research Lead | Once P2 dossiers exist + ADRs are implemented + measurements collected |
| **External pentest engagement** | Tech Lead + Security Engineer | Per `docs/research/EXTERNAL_REVIEW_PREP.md`; recommended after ADRs 0005/0006/0010 implementations land |
| **Customer-trigger watch — ROADMAP_NOT #2 (SSO)** | Product Manager | Standing. Trigger: "two customers ask in the same quarter, or one enterprise prospect makes it a deal-blocker." Dossier set ready: Keycloak (P0), Zitadel (P1), Authentik (P1). |
| **Customer-trigger watch — BEYOND §2.5 (SPIFFE)** | Product Manager | Standing. Trigger: MCP integration pulls forward agent-identity needs. Dossier set ready: SPIFFE (P0), Teleport (P1). |

---

## Section 6 — Recommended sprint sequencing

### Sprint 1 (next 30 days) — security risk reduction
1. **CI infrastructure repair** (DevOps) — unblocks everything
2. **ADR-0005 impl** (Backend + Security Engineer review) — closes 9 P1 IDORs
3. **ADR-0010 impl** (Backend + Security Engineer review) — closes RCE primitive
4. **Sign-off remaining ADRs in batch** (Security Engineer + Tech Lead)

### Sprint 2 (days 30-60) — Phase A complete
1. **ADR-0007 impl** (Backend + Security Engineer) — closes FU 0d
2. **ADR-0009 impl** (Backend + Security Engineer) — closes FU 0k
3. **ADR-0011 impl** (Backend + DevOps) — graceful shutdown
4. **ADR-0012 impl** (DevOps + Backend) — KMS auto-unseal, depends on 0011
5. **ADR-0015 impl** (Backend) — `safego` + audit emission
6. **Phase-A FU sweep**: 0a, 0c, 0e, 0f, 0g

### Sprint 3 (days 60-90) — Phase B prep
1. **ADR-0006 impl review** (already in #50) — Security Engineer final pass
2. **ADR-0013 impl** (Backend + Security Engineer) — webhook + SSRF
3. **ADR-0008 impl** (Backend + Security Engineer) — RS256 + JWKS (no urgency)
4. **ADR-0014 impl** (Backend) — audit-log taxonomy
5. **External pentest engagement** scheduled (Tech Lead + Security Engineer)

### Sprint 4+ — Operational + Phase B
1. Customer-trigger watch escalations (Product Manager)
2. P2 dossier backfill (Research Lead, demand-driven)
3. BEYOND memo v2 (Research Lead, after implementations land)
4. Pattern matrix v3 (Research Lead)

---

## Verification — backlog closure criteria

| Section | Closed when |
|---|---|
| §1 ADR sign-offs | All 11 ADRs at `accepted` or `rejected` (each with documented rationale) |
| §2 Implementation queue | All P0 + P1 items have merged PRs on `main`; P2 items have either merged PRs or explicit deferral entries in this doc |
| §3 Phase-A FUs | Each entry either closed (linked to merged PR) or moved to a Phase-B section in `docs/FOLLOWUPS.md` with documented trigger |
| §4 Operational | CI green; migration numbers deconflicted; STRIDE rows pinned by ID; SDK-factual-correction landed; BEYOND.md §2.6 sub-bullet added |
| §5 Research extensions | These remain standing — they don't "close," they evolve with the product |

---

## Standing reminders

- **This document is canonical for post-Day-90 work.** Do not duplicate items into `docs/FOLLOWUPS.md` — instead, FOLLOWUPS.md tracks Phase-A items and this doc tracks the post-research backlog.
- **Type-1 governance still applies.** Every ADR impl PR needs Security Engineer + Tech Lead sign-off OR explicit sponsor override (and the override must be documented in the PR's commit message + this backlog).
- **Update this doc when work is picked up.** Move items from §1/§2/§3 into "completed" status with a link to the merging PR.

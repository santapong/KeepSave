# Veto-List Audit (Security Engineer 30-day)

The Security Engineer has veto power over PRs touching `internal/crypto`, `internal/auth`, and the promotion engine (`docs/ROLES.md` §2.2). This audit walks the last 90 days of commits in those paths and asks one question per commit: **did the change get a security review before merge?** If the commit message has no reference to threat model, ADR, or security sign-off, it's a candidate for retroactive review.

Output is in three sections: (1) per-commit triage, (2) findings summary, (3) process change so this never has to be done by archaeology again.

---

## Section 1 — Commit triage

Format: `<sha> | <subject> | <referenced security artifact?> | <verdict>`

Verdict legend:
- ✅ Cleared — references a threat-model section, ADR, or `SECURITY_AUDIT.md` line; no further action.
- ⚠️ Retro-review — no security artifact referenced; needs Security Engineer to read the diff and confirm or open a follow-up.
- 🚫 Block — review reveals a concrete issue; open an issue immediately.

| SHA       | Subject                                                       | Artifact ref | Verdict      |
|-----------|----------------------------------------------------------------|--------------|--------------|
| `0f1d518` | fix(backend): unbreak lint, tests, govulncheck                | govulncheck  | ✅ Cleared   |
| `380c666` | feat(crypto): pluggable MasterKeyProvider interface           | (none)       | ⚠️ Retro-review — touches `internal/crypto`. Verify against ADR-0004; the file `keyprovider/env.go:40-42` returning `ErrUnsupported` is correct, but the AWS/GCP adapters need a separate review when wired. |
| `df85048` | feat: Phases 7-12 — observability, SDKs, enterprise, security, AI agents | "security" in subject only | ⚠️ Retro-review — broad commit; needs file-by-file review of any crypto/auth-adjacent diff. |
| `2d29fe7` | feat: Phase 5 hardening and operations                        | "hardening"  | ⚠️ Retro-review — security work claimed; no THREAT_MODEL update at the time. |
| `363db2a` | feat: Phase 2 environment promotion engine                    | (none)       | ⚠️ Retro-review — **promotion engine** is the highest-blast routine path. Review against ADR-0003 and verify the requester-can-self-approve invariant (`docs/FOLLOWUPS.md` item §5). |
| `189b468` | feat: Phase 1 foundation — encrypted secret storage           | (none)       | ⚠️ Retro-review — backfilled by ADRs 0001 & 0004, but the original commit had no recorded review. Cleared *by* ADR-0001/0004 backfill, no further action. |

Earlier commits (>90 days, listed for completeness):
- `1067b7c` (docs): adds the original threat model, runbook, pentest checklist → not a security-critical code commit; ✅.
- The Phase 5/12 commits include security work; ADR-0001 / 0002 / 0003 / 0004 retroactively cover the underlying decisions.

## Section 2 — Findings

1. **No commits in the last 90 days touching `internal/crypto`, `internal/auth`, or the promotion path reference a threat-model section, ADR, or `SECURITY_AUDIT.md` line.** This is the root cause: there is no convention for cross-linking, so reviewers don't have a place to point. The ADR backfill (ADRs 0001-0004) partially fixes this for the existing decisions.

2. **Promotion-engine commit `363db2a` is the highest-priority retro-review.** It introduced the promotion path which ADR-0003 now describes. Two specific things must be verified by reading the diff:
   - Whether requester-cannot-self-approve is enforced at the DB layer or only in service code (open follow-up in ADR-0003).
   - Whether the diff redacts secret plaintext (claimed in `THREAT_MODEL.md` §3, must be verified, not asserted).

3. **The `MasterKeyProvider` pluggability commit `380c666` is acceptable but the AWS/GCP KMS adapters in `kms_aws.go` / `kms_gcp.go` (not yet wired to `main.go` per `FOLLOWUPS.md` §1) need a security review *before* the wiring PR lands.** Pre-empting the future PR with a written review checklist saves time later.

4. **`df85048` (Phases 7-12, large commit)** must be split for review purposes. Security reviews multi-thousand-line PRs poorly. Spawn a follow-up task to map the file-by-file diff against current files; anything in `crypto/`, `auth/`, or `service/promotion_service.go` is in-scope for retro-review.

5. **No `SECURITY.md` (root-level)** today. Adding it is Security 90-day (vulnerability disclosure policy). The current scattered references (`SECURITY_AUDIT.md`, `docs/THREAT_MODEL.md`, `docs/PENTEST_CHECKLIST.md`) are correct artifacts but undiscoverable to outside reporters.

## Section 3 — Process change (so this is never archaeology again)

The retroactive audit is expensive. Three changes make it cheap going forward:

### 3a. Commit-message convention

Every commit touching `backend/internal/crypto/**`, `backend/internal/auth/**`, or the promotion engine MUST include in the body one of:
- `Refs ADR-NNNN` — implements / extends an existing ADR.
- `Refs THREAT_MODEL §N.M` — addresses a documented threat.
- `Refs SECURITY_AUDIT v<vers>` — closes an audited item.
- `Security: <one sentence>` — for changes too small to ADR but in scope.

PRs without one of these in any commit on protected paths are blocked at review.

### 3b. CODEOWNERS for protected paths

Add (or extend) `.github/CODEOWNERS`:
```
backend/internal/crypto/         @security-eng
backend/internal/auth/           @security-eng
backend/internal/service/promotion_service.go    @security-eng
backend/internal/api/handlers_promotion.go       @security-eng
docs/adr/                        @tech-lead @security-eng
docs/THREAT_MODEL.md             @security-eng
```

The Security Engineer's review is mandatory by GitHub branch protection, not by a convention people can forget.

### 3c. Weekly threat-model review meeting

30 minutes, Tech Lead + Security Engineer (per `docs/ROLES.md` §6). Standing agenda:
1. New PRs touching protected paths.
2. Open ADRs requiring decision.
3. Top 3 items from `docs/FOLLOWUPS.md` in scope.
4. Anything in the audit log that looks unusual.

Output: one line in `docs/FOLLOWUPS.md` per meeting if any item moved.

---

## Output — retro-reviews to schedule

| Item                                                                                   | Owner            | Due                 |
|----------------------------------------------------------------------------------------|------------------|---------------------|
| Read `363db2a` diff, verify requester-self-approval invariant + plaintext redaction in diff endpoint. | Security + Backend | 30 days             |
| File-by-file map of `df85048` against current paths; retro-review crypto/auth deltas.   | Security         | 30 days             |
| Pre-write review checklist for AWS/GCP KMS adapter wiring PR.                          | Security         | Before adapter PR (≤ 30 days). |
| Add CODEOWNERS file with §3b contents.                                                  | Tech Lead        | 30 days             |
| Add commit-message convention to `CONTRIBUTING.md` (or create it).                     | Tech Lead        | 30 days             |
| Stand up weekly threat-model review (calendar invite + first 3 meetings booked).        | Security + Tech Lead | 30 days         |

These go into `docs/FOLLOWUPS.md` as named items.

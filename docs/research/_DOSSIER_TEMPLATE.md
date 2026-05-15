# Competitor Dossier — <Vendor Name>

> Copy this file to `docs/research/competitors/<vendor-slug>.md` and fill in. All 13 sections are required for P0 dossiers. P1 may collapse §3–5. P2 is a single paragraph. Word caps: P0 ≤ 2500; P1 ≤ 400; P2 ≤ 100.

## 1. Header

- **Vendor:** <name>
- **Category:** identity provider | self-hosted auth | machine identity | secrets management | approval workflow | edge auth
- **License:** <OSS license or commercial>
- **Last updated:** YYYY-MM-DD
- **Analyst:** <agent role>
- **Reviewer:** <Security Reviewer agent>
- **Status:** draft | reviewed | accepted | rejected
- **Priority:** P0 | P1 | P2

## 2. One-paragraph overview

What they do, who uses them, why we care. Three to five sentences. No marketing language.

## 3. Architecture summary

Components, data flow, trust boundaries. Diagram optional. State which components KeepSave would interact with vs which we'd ignore.

## 4. Security model

Auth primitives, crypto choices, key custody, threat-model assumptions they publish. Cite their security docs with retrieval dates.

## 5. KeepSave-comparable surface

Explicit mapping table. Their X corresponds to our Y at `file:line`. If a feature has no analog, say so. Required file:line citations to KeepSave.

| Their concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| | | |

## 6. Adapt candidates

Numbered list of specific patterns we could borrow. Each is one discrete idea, not "they're well-designed."

1. **<Candidate name>** — one-paragraph description of the pattern.
2. ...

## 7. Pros / cons of adapting

For each candidate above. Cons must include operational + security cost, not just engineering effort.

### Candidate 1: <name>
- **Pros:** ...
- **Cons (operational):** ...
- **Cons (security):** ...

### Candidate 2: ...

## 8. Validation evidence

Links: official docs (version + retrieval date), RFCs, public security audits, CVE history (last 24 months), maintainer post-mortems. **Blog posts alone do not pass review.** At least one of: RFC reference, public audit, CVE, maintainer post-mortem.

- <reference 1>
- <reference 2>

## 9. Threat-model implications

Which STRIDE category in `docs/THREAT_MODEL.md` the candidate touches. Does adopting **widen or narrow** our trust boundary? If it widens, name the new entity now in the boundary.

## 10. Verdict

One of:

- `adopt-now` — adopt during Phase A or B; ADR required.
- `adopt-when-trigger-fires` — adopt when a specific trigger occurs. **Quote the trigger from `docs/ROADMAP_NOT.md` or `docs/FOLLOWUPS.md` verbatim.**
- `reject` — state reason.
- `note-only` — P2 only.

Security Reviewer veto applies to this section for any dossier touching `internal/auth`, `internal/crypto`, or promotion.

## 11. Rollback if adopted

Concrete: what migration backs out, what data does NOT survive the rollback, what runbook step is required. Mirrors `docs/adr/0000-template.md` §Rollback plan.

If true rollback is impossible (e.g., we'd re-encrypt secrets under a new scheme), say so and describe the forward-fix path instead.

## 12. Won't-break-our-system claim

Explicit: which KeepSave tests/invariants stay green if this is adopted; how the analyst verified that. Acceptable verification: read code at `file:line` and trace through; ran locally with a test fixture; cross-checked against a public audit report.

- **Invariant 1:** <what stays green> — verified by: <how>
- **Invariant 2:** ...

Security Reviewer veto applies to this section.

## 13. References

Full URL list with retrieval dates. RFCs, security audits, CVEs, source repos.

- <url 1> (retrieved YYYY-MM-DD)
- <url 2> (retrieved YYYY-MM-DD)

# ADR-NNNN: <Short, decision-shaped title>

- **Status:** Proposed | Accepted | Rejected | Superseded by ADR-NNNN
- **Date:** YYYY-MM-DD
- **Authors:** name (role)
- **Reviewers required:** Tech Lead; Security Engineer (if touches crypto / auth / promotion)
- **Supersedes:** none | ADR-NNNN
- **Related:** ADR-NNNN, `docs/THREAT_MODEL.md` §X, etc.

---

## Context

What is the situation that forces a decision? Two or three short paragraphs. Include:

- The user-facing or technical problem in plain language.
- Constraints we cannot violate (security, performance, customer commitments).
- Threat-model implications, if any. If this widens a trust boundary, say so explicitly.

Avoid: marketing language, justifications for a foregone conclusion, mention of who is annoyed.

## Options considered

At least two. *Always.* "We just picked X because it's standard" is not an ADR — it's an excuse.

### Option A — <name>

- **How it works:** one paragraph.
- **Pros:** bullets.
- **Cons:** bullets, including security and operational cost.

### Option B — <name>

(same shape)

### Option C — <name> *(if applicable)*

(same shape)

## Decision

State the chosen option in one sentence. Then 1-2 paragraphs explaining *why* this option won — referencing the cons of the rejected options, not just the pros of this one.

If the decision is conditional ("we choose A but only until X is true"), state the trigger that would force a re-decision.

## Rejection rationale

For each rejected option, one sentence on why it lost. This is the part future readers will care about most: it prevents the question being re-litigated every quarter.

## Consequences

What this decision commits us to. Include:

- **Operational:** new runbook entries, monitoring, on-call burden.
- **Security:** new trust boundaries, new threat-model entries needed.
- **Migration:** what data or config needs to change.
- **Reversibility:** how hard is it to undo. If it's hard, say so plainly — that informs how seriously the rollback plan needs to be designed.

## Rollback plan

If this decision turns out wrong, what do we do? Specific. Not "we'll figure it out" — that's the same as no plan.

If a true rollback is impossible (e.g., once we re-encrypt secrets under a new scheme, the old ciphertext can't be reconstructed), say so and describe the forward-fix path instead.

## Open questions

List unresolved questions, with an owner and a due date. These don't block acceptance — they're a contract for follow-up work. Empty list means you haven't thought hard enough.

## References

- File paths and line numbers in the codebase that this ADR describes.
- External docs (RFCs, NIST publications, vendor advisories).
- Related issues / PRs.

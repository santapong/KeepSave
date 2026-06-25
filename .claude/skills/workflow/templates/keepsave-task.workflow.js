// KeepSave task workflow — starter template for the `/workflow` skill.
//
// Executes the AI-Assisted Development Life Cycle (docs/ADLC.md) as gated
// Workflow phases, using the patterns in docs/HARNESS_ENGINEERING.md:
//   Understand → Classify (gate) → Build → Adversarial Verify → Review.
//
// HOW TO USE
//   1. Pass the task as `args` when invoking the Workflow tool:
//        Workflow({ scriptPath: ".../keepsave-task.workflow.js",
//                   args: { task: "Add an endpoint to list a project's promotion history" } })
//   2. Specialize the `TODO` blocks below for your task. The defaults let the
//      Understand phase DISCOVER the work-list and security claims, so the
//      template runs end-to-end unedited — override when you know better.
//   3. Keep the gate phases intact. They are the reason this is a KeepSave
//      workflow and not a generic fan-out.
//
// NOTE: plain JS, not TypeScript. No Date.now()/Math.random() (they break
// resume) — stamp times after the run; vary randomness by index.

export const meta = {
  name: 'keepsave-task',
  description: 'Run a KeepSave change through the ADLC gates: understand, classify, build, adversarially verify, review.',
  phases: [
    { title: 'Understand' },
    { title: 'Classify' },
    { title: 'Build' },
    { title: 'Verify' },
    { title: 'Review' },
  ],
}

// ── KeepSave guardrails injected into every code-touching agent prompt ──────
const GUARDRAILS = `
KeepSave is a secrets product. Obey these without exception:
- Never log, return in an error body, persist to browser storage, or commit to
  a test fixture any plaintext secret value.
- Use the httperror package for client responses — never return err.Error().
  (docs/ERROR_HANDLING_STANDARD.md)
- Every state-mutating handler MUST emit an audit event from
  docs/AUDIT_LOG_COVERAGE.md, and its test MUST assert the row was written.
- Add a negative-auth test for every new surface (tests/NEGATIVE_AUTH_PLAN.md).
- Work on the feature branch only; never edit main.
- If a gate cannot be satisfied, STOP and report the blocker. Never route around it.
`.trim()

// ── Schemas (validated agent output — no parsing needed) ────────────────────
const UNDERSTAND_SCHEMA = {
  type: 'object',
  required: ['subsystem', 'trustBoundary', 'decisionType', 'threatSurface', 'workItems', 'securityClaims'],
  properties: {
    subsystem: { type: 'string', description: 'crypto | auth | promotion | api | repository | frontend | embed | infra | docs' },
    trustBoundary: { type: 'string', description: 'Which boundary in docs/ARCHITECTURE.md the change touches (1/2/3 or none)' },
    decisionType: { type: 'string', enum: ['Type-1', 'Type-2', 'Type-3'] },
    escalationTrigger: { type: 'string', description: 'What would bump this to the next class up' },
    threatSurface: { type: 'string', description: 'STRIDE delta for docs/THREAT_MODEL.md, or "no boundary moved"' },
    workItems: {
      type: 'array',
      description: 'Independent units of implementation work',
      items: {
        type: 'object',
        required: ['path', 'change'],
        properties: { path: { type: 'string' }, change: { type: 'string' } },
      },
    },
    securityClaims: {
      type: 'array',
      description: 'Claims the change makes that MUST be adversarially verified (e.g. "no plaintext leaks", "query is tenant-scoped", "audit row written")',
      items: { type: 'string' },
    },
  },
}

const BUILD_SCHEMA = {
  type: 'object',
  required: ['item', 'summary', 'auditEvent'],
  properties: {
    item: { type: 'string' },
    summary: { type: 'string' },
    filesTouched: { type: 'array', items: { type: 'string' } },
    auditEvent: { type: ['string', 'null'], description: 'Audit event emitted, or null if not a state-mutating handler' },
    notes: { type: 'string' },
  },
}

const VERDICT_SCHEMA = {
  type: 'object',
  required: ['claim', 'lens', 'refuted', 'evidence'],
  properties: {
    claim: { type: 'string' },
    lens: { type: 'string' },
    refuted: { type: 'boolean', description: 'true if the claim could NOT be upheld; default true under uncertainty' },
    evidence: { type: 'string' },
  },
}

const REVIEW_SCHEMA = {
  type: 'object',
  required: ['approved', 'findings'],
  properties: {
    approved: { type: 'boolean' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        required: ['severity', 'title'],
        properties: {
          severity: { type: 'string', enum: ['blocker', 'high', 'medium', 'low'] },
          title: { type: 'string' },
          file: { type: 'string' },
          detail: { type: 'string' },
        },
      },
    },
  },
}

// ── Verification lenses (docs/HARNESS_ENGINEERING.md §5) ─────────────────────
// Diversity beats redundancy: each verifier attacks a claim from a distinct angle.
const LENSES = ['plaintext-leak', 'cross-tenant', 'downgraded-token', 'audit-row-present']

// ════════════════════════════════════════════════════════════════════════════

const TASK = (args && args.task) || 'TODO: describe the KeepSave task here'

// ── Stage 0-2: Understand + classify + threat surface ───────────────────────
phase('Understand')
const map = await agent(
  `You are scoping a KeepSave change for the AI-Assisted Development Life Cycle (docs/ADLC.md).\n\n` +
  `TASK: ${TASK}\n\n` +
  `Read the relevant code. Identify the subsystem and trust boundary (docs/ARCHITECTURE.md), ` +
  `classify the decision (Type-1/2/3 per docs/ROLES.md §3.1 — when unsure, the next class up), ` +
  `draft the threat-model delta (docs/THREAT_MODEL.md), enumerate the independent work items, ` +
  `and list every security CLAIM the change makes that must be adversarially refuted.\n\n` +
  `Check docs/FOLLOWUPS.md — prefer tracked work. Do not write code in this phase.`,
  { phase: 'Understand', label: 'scope', schema: UNDERSTAND_SCHEMA }
)

// ── Stage 1 GATE: Type-1 stops here. The human owns the decision. ───────────
phase('Classify')
log(`Decision class: ${map.decisionType} · subsystem: ${map.subsystem} · trust boundary: ${map.trustBoundary}`)
if (map.decisionType === 'Type-1') {
  log('⛔ Type-1 detected — implementation is gated behind an Accepted ADR + Security/Tech-Lead sign-off (ADLC §1.1, §3).')
  return {
    status: 'blocked-type-1',
    task: TASK,
    map,
    nextStep: 'Draft docs/adr/NNNN-*.md from the template; a human accepts it BEFORE any build phase runs.',
  }
}

// TODO (optional): override the discovered work-list / claims when you know better.
const workItems = map.workItems
const claims = map.securityClaims

// ── Stage 5: Build — pipeline over independent work items ───────────────────
// pipeline() = no barrier: an item starts verifying as soon as it finishes building.
// If items edit overlapping files, add { isolation: 'worktree' } to the build agent.
phase('Build')
const results = await pipeline(
  workItems,
  (item) => agent(
    `${GUARDRAILS}\n\n` +
    `Implement this unit of the task "${TASK}" on the feature branch:\n` +
    `  path: ${item.path}\n  change: ${item.change}\n\n` +
    `Follow CLAUDE.md coding conventions. Add tests at every relevant layer ` +
    `(tests/PYRAMID.md), including the audit-row assertion and a negative-auth cell.`,
    { phase: 'Build', label: `build:${item.path}`, schema: BUILD_SCHEMA }
  ),
  // ── Stage 6: Adversarial verify — one item's claims, refuted by N lenses ──
  (built, item) => parallel(
    claims.flatMap((claim) =>
      LENSES.map((lens) => () =>
        agent(
          `Adversarially verify, through the "${lens}" lens, this claim about the change to ` +
          `${item.path}:\n  "${claim}"\n\n` +
          `Your job is to REFUTE it. Read the diff and tests. Set refuted=true if you can ` +
          `break it OR if the evidence is ambiguous (default-deny). Set refuted=false only ` +
          `if you are confident the claim holds.`,
          { phase: 'Verify', label: `verify:${lens}`, schema: VERDICT_SCHEMA }
        )
      )
    )
  ).then((verdicts) => ({
    item,
    built,
    // A claim survives only if a MAJORITY of its lenses clear it.
    verdicts: verdicts.filter(Boolean),
  }))
)

// Collapse verification: any claim a majority of lenses refuted sends the item back.
const verified = results.filter(Boolean)
const failed = verified.filter((r) => {
  const total = r.verdicts.length || 1
  const refuted = r.verdicts.filter((v) => v.refuted).length
  return refuted * 2 >= total // majority refuted ⇒ claim not upheld
})
if (failed.length) {
  log(`⛔ ${failed.length}/${verified.length} item(s) failed adversarial verification — return to Build (ADLC Stage 5).`)
}

// ── Stage 7: Review — correctness + (sensitive diffs) security review ────────
phase('Review')
const sensitive = ['crypto', 'auth', 'promotion'].includes(map.subsystem)
const reviews = await parallel([
  () => agent(
    `Run a correctness review of the changes for task "${TASK}". ` +
    `Report blocker/high/medium/low findings.`,
    { phase: 'Review', label: 'code-review', schema: REVIEW_SCHEMA }
  ),
  ...(sensitive ? [
    () => agent(
      `Run a SECURITY review of the changes for task "${TASK}". This diff touches ${map.subsystem} ` +
      `— a Security-Engineer veto surface (docs/ROLES.md §2.2). Check: no plaintext leak, no ` +
      `err.Error() to clients, audit row written, query tenant-scoped, no token downgrade path. ` +
      `Default to NOT approved if any check is unmet.`,
      { phase: 'Review', label: 'security-review', schema: REVIEW_SCHEMA }
    ),
  ] : []),
])

// ── Report (driver turns this into a draft PR — ADLC Stages 8-9) ────────────
return {
  status: failed.length ? 'needs-rework' : 'ready-for-pr',
  task: TASK,
  decisionType: map.decisionType,
  subsystem: map.subsystem,
  threatDelta: map.threatSurface,
  built: verified.map((r) => r.built),
  verificationFailures: failed.map((r) => r.item),
  reviews: reviews.filter(Boolean),
  securityReviewRequired: sensitive,
  reminder: 'Open a DRAFT PR. Security-Engineer sign-off required before merging any crypto/auth/promotion diff. Update THREAT_MODEL/ARCHITECTURE if a boundary moved; tick FOLLOWUPS.',
}

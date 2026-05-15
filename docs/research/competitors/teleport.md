# Competitor Dossier — Teleport

## 1. Header

- **Vendor:** Teleport (goteleport.com)
- **Category:** machine identity (CA + audit + session recording, one binary)
- **License:** AGPLv3 Community + commercial Enterprise
- **Last updated:** 2026-05-15
- **Analyst:** Machine Identity Analyst
- **Reviewer:** Security Reviewer (accepted-with-changes 2026-05-15)
- **Status:** accepted-with-changes
- **Priority:** P1

## 5. KeepSave-comparable surface

Teleport overlaps SPIFFE (`docs/research/competitors/spiffe.md`) on identity issuance but is **product-shaped** (web UI, audit dashboards, session recording) where SPIFFE is **spec-shaped**. Carriers: X.509+SSH+JWT (Teleport) vs X509-SVID+JWT-SVID (SPIFFE) — same idea, different envelope.

| Teleport concept | KeepSave analog | KeepSave file:line |
|---|---|---|
| Short-lived cert, embedded roles | Static `ks_` key, no TTL at issuance | `auth/apikey.go:11-26`; `models/models.go:51-61` |
| `tbot` Machine ID JWT | None — see SPIFFE Cand. 1 | `api/validation.go:33-38` |
| Session recording (replayable) | Mutation-only audit; no payload trail | `models/models.go:63-72` |
| Auth Service (CA bundled) | Not a CA — see SPIFFE §5 not-adopted | — |

## 6. Adapt candidates

1. **Short-lived token with audit metadata** — overlaps SPIFFE Cand. 1; Teleport is the **production-tested reference UX**. Adopt pattern, not binary.
2. **Session recording for AI-agent secret access** (NOVEL) — capture request/response when an agent fetches a secret; store as audit-log child rows. Payload-layer extension of BEYOND §2.6. `note-only`; analyst recommends BEYOND.md owner add a §2.6 sub-bullet.
3. **CA-in-a-box** — `reject`. SPIFFE primitives suffice; CVE history confirms cost.

## 8. Validation evidence

- **Doyensec audit of Teleport (2022)** — `https://goteleport.com/resources/audits/` (2026-05-15). **Non-vendor, load-bearing** for §10.
- **CVE-2024-7752** (SSH agent hijack, CVSS 9.1), **CVE-2023-43662** (SAML bypass) — RCE-class history; bundled CA+proxy is CVE-prone (informs Cand. 3).
- **RFC 5280** (X.509), **RFC 7519** (JWT) — envelopes.
- Teleport docs (vendor): `https://goteleport.com/docs/` (2026-05-15).

## 9. Threat-model implications

Same STRIDE rows as SPIFFE: `docs/THREAT_MODEL.md` §2 row S (l.83), row T (l.84). Cand. 1 narrows the time axis; no new entity. Cand. 2 narrows Repudiation (Findings new v1.2.0 §1) at payload granularity; boundary unchanged. Cand. 3 would **widen** the boundary by placing a CA inside KeepSave's domain — declined.

## 10. Verdict

- **Cand. 1:** `adopt-when-trigger-fires`. Trigger quoted verbatim from `docs/research/BEYOND.md:50-56`:

  > *"### 2.5 SPIFFE-shaped workload identity for AI agents — Hypothesis: Replace long-lived API keys … with SVIDs … short-lived, attested. … Phase: B if the MCP integration path … pulls this forward."*

  When fired, **Teleport is the reference UX**; SPIFFE is the spec.
- **Cand. 2:** `adopt-when-trigger-fires`, same trigger; extends §2.6.
- **Cand. 3:** `reject` — too heavyweight; SPIFFE primitives suffice.

Security Reviewer veto applies (touches `internal/auth`).

## Security Reviewer notes

**Verdict:** accepted-with-changes. Three follow-ups; **Cand. 2 ADR blocked until FU-A lands.**

**Spot-checks (Read, 2026-05-15).** `apikey.go:11-26`, `models.go:51-61`, `validation.go:33-38`, `models.go:63-72` confirmed. CVE-2024-7752 + CVE-2023-43662 verified — supports Cand. 3 reject. Row S/T cited 83/84 (actual 87/88; inherited from SPIFFE P0). §10 `BEYOND.md:50-56` verbatim for Cand. 1.

**Cand. 2 (NOVEL session-recording): CONFIRMED `adopt-when-trigger-fires`, anchor CHANGED.**

- **FU-A (BLOCKING for ADR): anchor §2.5 → §2.6.** §2.5 governs *identity-issuance*; recording is *payload-audit*. No shared firing condition — a forensic customer ≠ MCP customer pulling SPIFFE forward. Line 40 concedes it (Cand. 2 → Repudiation = §2.6) yet §10 attaches to §2.5. Re-anchor with own clause: "customer raises forensic-replay / regulatory session-evidence ask, OR post-breach review finds gaps unresolvable by mutation-only audit." Until §2.6 carries that sub-bullet, §10 verbatim contract for Cand. 2 fails.

- **FU-B: §7 cons required.** P1 collapses §3-5 only; §7 missing. Cand. 2 creates a **new high-value target** (recorded plaintext reads); analyst caveat insufficient. §7 must cover: store encryption + KMS parity; retention + right-to-erase; replay audit (`audit.replay_viewed`); blast-radius vs. mutation-only.

- **FU-C: BEYOND.md PR.** Analyst recommended §2.6 sub-bullet but did not write it — separate PR by Research Lead; track in `FOLLOWUPS.md`.

**Cand. 1 + 3:** accepted; consistent with SPIFFE P0.

# 5. Security model

Part of the [system documentation](README.md). Source reconciled October 4, 2026.

The [threat model](../THREAT_MODEL.md), [ADR0028](../adr/0028-core-identity-and-vault-release.md)
and [ADR0029](../adr/0029-harness-neutral-platform.md) describe the controls and
review gates. Independent Security Engineer/Tech Lead signatures remain pending;
local tests and source publication do not supply those reviews.

## Credential custody and trust domains

AES-256-GCM seals values with fresh nonces and versioned project keys. An external
wrapping key protects project keys; Vault Transit is the production reference.
Current/history/promotion ciphertext keeps its required key dependencies.
Encryption and audit formats are unchanged. Database encryption cannot contain a
trusted API/worker process with custody, and metadata is not all encrypted.

Ordinary vault clients receive values they are permitted to read. Controlled
GitHub tools use a different boundary: only the trusted broker obtains the App
private key/installation token and makes authenticated typed GitHub requests.
The runner/connector and model receive permitted results, never that token.
Provider adapters accept stored identifiers and structured paths, not arbitrary
URLs or caller HTTP headers. Social GitHub sign-in is a separate credential flow.

## Current identity and permission checks

Human JWT use checks signature, sid/jti, token hash, current user/session, expiry
and revocation. Sessions last at most 24 hours with no human refresh flow. Social
flows retain stable subjects, provider-bound one-time state, PKCE and Google's
nonce/signature checks. Email collisions require explicit authenticated linking;
link initiation requires authentication within ten minutes. Signup/workspace
ownership never grants a global operator role.

Personal ownership or current organization membership supplies project roles.
Viewer sees permitted metadata, editor reads/writes credentials, promoter approves
eligible protected promotion, and admin manages configuration. API-key/agent
permissions intersect stored parent/project/environment/key scopes and expiry.
A secret reference authorizes each dependency independently. Shared authority
barriers make stored revocation/membership changes visible at admission.
Offboarding epochs prevent old grants from working after rejoin while preserving
unrelated organizations, personal projects and global sessions.

Account-method removal requires successful authentication through a remaining
method. Password recovery atomically changes the password, consumes the proof,
revokes sessions/link proofs and retained delegated keys/leases/agent issuance,
and writes audit/outbox. A missing revocation port refuses partial recovery.
Contact/recovery proofs are hashed, purpose/account-bound and bounded; their
15-minute lifetime differs from the invitation proof's 24-hour lifetime.
Delivery material is ephemeral encrypted custody; jobs carry IDs only. SMTP
requires authenticated certificate-verified STARTTLS. Accepted does not mean
mailbox-delivered; uncertain sends are not blindly replayed.

## Delegated tools and result authority

MCP public OAuth uses S256, exact callbacks/canonical resource/issuer, hash-only
opaque credentials, atomic 60-second codes, ten-minute access and rotating
families bounded by eight hours and the parent human session. Replay revokes the
family. Present Origin must match configured authority; authenticated native
requests may omit it. No dynamic registration or weaker fallback is enabled.

Portable profiles bind exact artifacts and limits; separate runs belong to
registered client delegations. Approval cannot be self-issued. Authority checks
intersect current actor/membership/epoch, parent lineage, binding, profile/package
approval, immutable admitted repository/commit and budgets. Admission is recorded
before credential use. Result retrieval checks current authority again; retained
operation handles do not bypass revocation. Encrypted results expire no later
than their run and are excluded from backups/audit exports/logs/outbox payloads.

The separate runner reference uses rootless Podman, no IP network, read-only root,
bounded scratch/CPU/memory/processes, seccomp and an attempt-specific Unix relay.
Its enrolled supervisor key/container-engine socket stay outside the connector.
Broker mTLS checks a verified client chain and exact enrolled fingerprint. Host
preflight currently refuses missing CPU delegation; actual isolation remains
unqualified. No permissive fallback or device-attestation claim exists.

## Limits and unavailable surfaces

Revocation denies admissions after its commit; already admitted external calls
may finish, and returned content cannot be recalled. Native client interruption
is separate from explicit durable run/operation cancellation. Local instructions,
reported digests and administrator settings neither grant permission nor prove a
device obeyed them. Self-hosting does not make a harness's model local.

Failed authoritative state denies access. Required audit failure rolls back local
mutations. Historical corruption is surfaced, not skipped. Recovery excludes
authority and rejects stale/live undelete attempts. New flags default off; legacy
API-host MCP execution/builds, old OAuth issuance, unfinished AI/SSO/compliance and
nondurable webhook automation remain unavailable. See the
[acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
for tested scenarios, retained advisories and real-provider/host/release gates.

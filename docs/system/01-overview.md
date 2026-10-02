# 1. System overview

Part of the [system documentation](README.md), reconciled 2026-10-02.

KeepSave is an encrypted project credential vault with environment promotion,
scoped access and recovery. The current bounded backend release adds dependable
identity, revocable sessions, explicit team workspaces and PostgreSQL journal
adapters. It is an unreleased candidate; see the
[acceptance ledger](../validation/2026-10-01-core-release/ACCEPTANCE.md).

A person registers or signs in, creates a named workspace explicitly, and creates
or attaches an authorized personal project. Registration itself creates no
project, sample secret or global permission. Workspaces use current membership
for project authority, including for the original project owner after assignment.
Admin manages configuration, editor reads/writes credentials, promoter approves
eligible promotions and viewer inspects permitted metadata. API keys and leases
also remain inside their parent project/environment/key scope and expiry.

The trusted API uses Go/Gin, policy ports, AES-256-GCM and PostgreSQL. A separate
trusted worker performs opted-in backup maintenance. These trusted processes can
access decrypted values; an authorized vault API read returns plaintext to the
caller. Database-at-rest encryption is not control-host isolation.

Core local mutation, immutable revision, required audit and durable outbox share
one transaction. Secret and project DELETE retain tombstones; history/key
versions remain while needed. Restoration appends a new revision with an
expected-current-revision condition. Encrypted recovery starts with verification
and an isolated fresh database; selected live restore uses a metadata diff and
never restores source sessions, grants or approval authority.

The accepted future journey is repository review through Codex: one approved
GitHub repository and resolved commit, a ten-minute run, a vetted connector on a
separate rootless runner, and a broker that makes authenticated GitHub requests
without exposing the token. Standards-based MCP/OAuth is M2, broker/runner M3,
private skills/profiles M4 and measured self-hosted team operation M5. These
remain future acceptance programs, not delivered capabilities.

See [architecture](../ARCHITECTURE.md) for data flow, module ownership and the
ordered plan, [security](05-security.md) for enforcement limits and
[core ADR0028](../adr/0028-core-identity-and-vault-release.md) for settled decisions.

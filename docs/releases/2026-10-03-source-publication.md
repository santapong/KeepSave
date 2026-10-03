# Harness-neutral source publication — October 3, 2026

The owner explicitly requested committing the prepared candidate and pushing it
to `main`, plus an operator preparation checklist in README. This is source
publication of an unreleased candidate, not a production release, capability
activation, operator credential configuration or deployment. No new release tag
is part of this request.

## Publication scope and authority

Preserve the validated implementation based on develop3878e69 and its three
existing planning-document edits. Commit on the implementation feature branch,
integrate through `develop`, then merge that integration history into `main`.
Use ordinary fast-forward-safe pushes; do not rewrite remote history or change
branch protection. Separate landing checkout/deployment source and installed
Hermes state stay outside this publication.

The owner's October3 main-publication instruction overrides the normal
feature/develop/PR publication routing for this bounded source publication.
It does not establish independent Security Engineer/Tech Lead signatures,
remote CI success, a reviewed production release or a successful rollback drill.
[CLAUDE](../../CLAUDE.md), [ROLES](../ROLES.md) and [ADLC](../ADLC.md) retain those
normal review and deployment requirements. No governance file is weakened or
reviewer signature invented. If GitHub rejects the ordinary push, preserve the
candidate and use a reviewed web PR rather than bypassing protection.

## Evidence and remaining work

The October2 [acceptance ledger](../validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
records the executed PostgreSQL/router, recovery, protocol, frontend/container and
two-process journeys. Its working-tree language is dated historical evidence.
The code/build-source manifest was rechecked on October3: all659 files still
match the recorded digest; only publication/setup documentation changes are
required for this task. These checks do not establish current remote CI.

The [README preparation checklist](../../README.md#what-the-operator-needs-to-prepare)
and [setup guide](../design/2026-10-02-harness-neutral-platform/SETUP.md) identify
operator prerequisites. Real Google/GitHub/SMTP, exact Codex/Hermes, disposable
GitHub App, runner isolation and production recovery/failure exercises are still
pending. Single-file instruction packages remain the implemented pilot subset;
reference-file skill trees remain follow-on work. New platform flags default off.

The checked runtime scan retains the API timezone-data advisory and unimported
OpenPGP module records; see the [image receipt](../validation/2026-10-02-harness-neutral-platform/OPERATIONS.md).
No blanket clean-security, availability or capacity claim follows from publication.

Successful publication must be confirmed by actual remote branch read-back.
The resulting revision and observed CI status belong in the publication receipt
and current project checkpoint, rather than replacing the dated test receipts.

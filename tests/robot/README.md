# KeepSave Robot Framework fixtures

Source audit: October 4, 2026. This is the retained black-box HTTP/widget test tier,
not current production/provider acceptance. Pinned dependency comments record a
June 28, 2026 dry-run; a dry-run parses keywords without invoking the API/browser.
See the [test pyramid](../PYRAMID.md), [current ledger](../../docs/validation/2026-10-02-harness-neutral-platform/ACCEPTANCE.md)
and [documentation hub](../../docs/README.md).

## Actual scope

API fixtures register/login, create a personal project, store a synthetic marker,
mint an environment-scoped read key and fetch the value. Story-level negatives
cover missing/garbage credentials and basic safe error text. They do not implement
a complete tenancy/race matrix or required transaction audit assertions. Browser
fixtures check tabs and masked values inside the open Shadow Root; they require
a separately prepared authenticated host page and bundle.

Crypto correctness belongs to Go crypto/fuzz tests; current authority, audit
rollback and concurrency use actual-router PostgreSQL fixtures. Component/DOM
logic uses Vitest. Robot does not start a real Google/GitHub/SMTP provider, native
harness, separate connector host or GitHub App.

## Current execution blockers

The retained Compose healthcheck uses `CMD-SHELL`/`wget`, but the current API image
is distroless without a shell or wget. Its service-health dependency therefore
cannot establish readiness. `Create Project` in `resources/keepsave.resource`
expects top-level `id`; current core handler returns `{project:{id}}`. Repair
these fixtures and rerun on the exact candidate before advertising this suite as
a passing current gate. These are source-audit findings, not a new execution.

For the currently recorded disposable database gate, use:

```bash
bash scripts/test-platform-postgres.sh
```

From the repository root, parse fixtures in an isolated Python environment:

```bash
python3 -m venv /tmp/keepsave-robot-venv
/tmp/keepsave-robot-venv/bin/pip install -r tests/robot/requirements.txt
/tmp/keepsave-robot-venv/bin/robot --dryrun --include api tests/robot/tests
```

This may install pinned dependencies; it does not qualify a live stack. After
repair, `robot --include api` targets an explicitly configured disposable API via
`KEEPSAVE_URL`; browser runs additionally need `rfbrowser init` and a real
`KEEPSAVE_WIDGET_HOST` URL serving the bundle. The default localhost 3000 example
is historical, not the current frontend 3002 default or an automatically built host.

Keep fixtures synthetic, poll real readiness rather than sleeping blindly and
clean only their owned resources. No production URL/secret belongs in Robot
output or fixtures. Shadow DOM is styling/testability, not host-page isolation;
see [widget integration](../../frontend/src/embed/INTEGRATION.md) and
[branding](../../docs/BRANDING.md).

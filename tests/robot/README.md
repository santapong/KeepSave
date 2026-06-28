# KeepSave — Robot Framework acceptance suite

The **black-box acceptance tier** (the tip of the test pyramid) for KeepSave, sitting just
above the Go E2E harness (`tests/e2e/seidr/`). It exercises real HTTP round-trips against a
running stack and — at the browser tier — the `<keepsave-widget>` embed.

Design rationale, scope, and the standalone-vs-integrated decision: see
[`docs/research/GTM_AND_TESTING_STRATEGY_2026-06.md` §Part 4](../../docs/research/GTM_AND_TESTING_STRATEGY_2026-06.md).
This suite is **integrated** in-repo (not a separate repo); "standalone" is achieved by
process isolation — a dedicated `docker-compose.e2e.yml` + pinned `requirements.txt`,
exactly as Seidr isolates via its own `go.mod`. Classified **Type-2** (new test tooling, no
crypto/auth/promotion code change).

## What this proves
- **API tier** (`tests/01_api_happy_path.robot`, `02_negative_auth.robot`): a secret stored
  via JWT round-trips through a scoped per-environment API key; story-level negative-auth
  (no token / garbage token / no API key → 401) and that error bodies don't leak internals.
- **Browser tier** (`tests/03_embed_widget.robot`, tagged `browser`): the embed widget renders
  inside its **open** Shadow DOM and masks values by default.

## What this does NOT prove (hard boundary — enforce at PR review)
RF asserts only HTTP status codes, JSON shapes, and audit-log *visibility* for multi-step flows.
It must **not** absorb:
- Crypto correctness / ciphertext bytes → Go crypto unit + `go test -fuzz`.
- The exhaustive 12×11 negative-auth matrix → Go `httptest` (`tests/NEGATIVE_AUTH_PLAN.md`).
- The CLAUDE.md audit-row gate → that assertion lives in the **Go handler test**; RF only
  *supplements* it with acceptance-layer visibility checks.
- Frontend component logic → Vitest. Promotion business rules → Go service tests.

## Run it

### Against a stack you already have (fastest)
```bash
pip install -r tests/robot/requirements.txt
export KEEPSAVE_URL=http://localhost:8080
robot --include api --outputdir tests/robot/results tests/robot/tests
```

### Self-contained via Docker (CI shape)
```bash
docker compose -f tests/robot/docker-compose.e2e.yml up \
  --build --abort-on-container-exit --exit-code-from robot
```

### Browser tier (extra setup)
Needs the frontend serving a host page with the widget bundle, plus the Playwright browsers:
```bash
pip install robotframework-browser && rfbrowser init
export KEEPSAVE_WIDGET_HOST=http://localhost:3000/embed/example.html
robot --include browser tests/robot/tests/03_embed_widget.robot
```

## Validate without a stack
`robot --dryrun` parses every suite and resolves every keyword (no HTTP calls):
```bash
robot --dryrun --include api tests/robot/tests
```

## Layout
```
tests/robot/
  resources/keepsave.resource   # keywords + variables (the API contract)
  tests/01_api_happy_path.robot  # canonical round-trip
  tests/02_negative_auth.robot   # story-level negatives only
  tests/03_embed_widget.robot    # browser tier (tagged `browser`)
  docker-compose.e2e.yml         # db + api + robot runner (mirrors seidr)
  Dockerfile                     # slim API-tier runner
  requirements.txt               # PINNED deps
```

## Conventions
- Compose DNS, never host ports (`http://api:8080`); poll `/readyz`, never `sleep`
  (`tests/FLAKY.md`).
- **Test data only.** Never commit a real secret value to a fixture (KeepSave invariant).
- Widget locators use CSS only — **XPath does not pierce shadow roots**. The widget's
  `mode:'open'` shadow root is an implicit testability contract; flipping it to `'closed'`
  breaks this tier (record any such change in `docs/EMBED_STATE.md`).

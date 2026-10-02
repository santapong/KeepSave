# Event Horizon login + GitHub / Google — local validation

Checked 2026-09-28, isolated branch `feat/landing-event-horizon-20260928`, based on `9e2b3ba`. Canonical `/mnt/data/company/apps/KeepSave` remained clean on `develop`. Changes are uncommitted and not deployed. The accepted landing design is preserved; the login uses its amber/violet art and the original KeepSave mark, adapting the selected login draft to include provider buttons.

## Evidence

Logs, selected draft, actual desktop/mobile screenshots and preview process files: `/mnt/data/keepsave-login-design-2026-09-28/`. The private preview environment and disposable database in that directory are not deliverables to commit or publish.

- Frontend: **105 tests across 16 files passed**. Includes existing password sign-in and landing tests, provider availability/error recovery, same-tab PKCE, callback cancellation/expiry/provider mismatch, link authorization headers, unexpected redirect rejection, storage denial, and one callback exchange under React StrictMode.
- Frontend lint, application build and widget build passed. Existing large-chunk and Node deprecation notices remain advisory.
- Backend `go test ./...` and `go vet ./...` passed using cached `golang:1.25` Docker tooling. Race tests passed for config, service, repository and API; the API test added afterward also passed with the race detector.
- New backend tests use real embedded SQLite migrations and isolated provider HTTP fixtures. GitHub code exchange checks callback/PKCE, verified email, stable identity, email-collision rejection, linking ownership, replay and concurrent one-time consumption. Google tests use signed RSA tokens and the actual go-oidc verifier; wrong issuer/audience/expiry/nonce/signature/authorized party, missing subject and unverified email fail. No Google/GitHub account credentials are used.
- Audit outcomes checked; a forced login-audit failure prevents JWT issuance. Identity persistence and audit append are separate transactions: an outcome-audit failure after persistence may leave a created/linked identity, with the preceding verification-intent audit recorded. Retrying sign-in can recover; this is not a claim of cross-table atomic audit + identity persistence.
- API checks cover public provider metadata, malformed and disabled-provider requests, anonymous link denial, agent-token denial and human-JWT routing. Backend errors do not serialize provider responses.
- Disposable PostgreSQL 16 integration: all 15 migrations, nullable and user-bound flow insertion/consumption/replay, stable identity resolution and ownership conflict passed. Its container/network were removed afterward.
- MySQL migration and explicit foreign keys/case-sensitive subject/PKCE storage were reviewed, but no MySQL server was run. The MySQL flow-insert branch follows [the documented INSERT…SELECT target-table restriction](https://dev.mysql.com/doc/refman/8.4/en/insert-select.html); live MySQL acceptance remains pending.
- nginx configuration syntax passed in a disposable official nginx container. Its access log excludes callback query strings. Frontend referrer metadata and nginx/Vercel headers use `no-referrer`.

## Browser acceptance actually exercised

Visible Codex in-app Chromium browser, current local login at `http://127.0.0.1:4651/login`:

- Desktop 1440×960; mobile 390×844 screenshot; 320px reflow checked. No horizontal document overflow at these widths. Artwork and both official provider marks loaded.
- Password show/hide, keyboard navigation to account creation, mobile footer and disabled-provider explanation inspected.
- Existing password registration API created a disposable account in the isolated preview database; actual browser password sign-in opened the workspace, Account showed both provider setup states, and sign-out returned to login. The disposable user was removed afterward.
- Latest login console error list was empty. Viewport override reset and login left open, signed out.
- No real external provider consent, live Google JWKS retrieval/rotation, actual GitHub/Google callback, real linked account UAT, Firefox, Safari, production hosting or remote CI was tested. Provider HTTP fixtures and signed token checks are not proof of a completed live social login.

## Repeat the checks

From `frontend`, use `NODE_OPTIONS=--no-experimental-webstorage npm test`, `npm run lint`, and `npm run build:all` on this host (Node 26). Backend commands: `go test ./...`, `go test -race ./internal/config ./internal/service ./internal/repository ./internal/api`, and `go vet ./...`. On this host Go was run through `golang:1.25` with CGO enabled and the existing `keepsave-go-mod` / `keepsave-go-build` cache volumes, limited to two CPUs.

Optional PostgreSQL test expects an isolated Docker service named `keepsave-social-postgres-test`, database/user `keepsave_social_test`, password `local-test-only`, on its own private Docker network with no host port published. Wait for TCP readiness (`pg_isready -h 127.0.0.1` inside the container). Run `KEEPSAVE_SOCIAL_POSTGRES_TEST=1 go test ./internal/repository -run TestSocialAuthPostgresIntegration -v` from a Go container on that network. The test uses a fixed disposable hostname/database; never substitute a real vault DSN. Remove only the test container and its network after completion.

## Setup and remaining review

[Provider setup guide](../../SOCIAL_LOGIN_SETUP.md) includes exact local callbacks, backend environment names, GitHub/Google console steps, account-linking behavior, and live verification steps. Both providers are currently disabled because the owner confirmed the OAuth apps do not exist yet.

[Design/RFC](../../design/2026-09-28-social-login.md) and the threat-model delta record the added trust boundary. Human Security/Tech Lead review remains an integration gate under the repository workflow. These local checks do not substitute for that review or authorize merging, pushing or deploying.

# KeepSave GitHub and Google sign-in setup

![KeepSave — Your secrets. In the right orbit.](assets/keepsave-header.svg)

Status reconciled 2026-10-04: provider adapters, browser flows, revocable sessions and method/proof services exist in the published source candidate. The canonical project is `/mnt/data/company/apps/KeepSave`. Neither real provider app configuration nor real consent/sign-in UAT has been performed. The buttons correctly remain unavailable when a provider is unconfigured. Independent Security/Tech Lead review and deployment acceptance remain release gates.

KeepSave permits open registration using passwords or a verified Google/GitHub identity. Every new account starts without organization membership or global administrator authority. A developer creates or joins a separately authorized organization; signing in does not grant access to another team's projects.

## Canonical application origin

The agreed full application host is **`https://app.keepsave.draveniq.dev`**. The existing static landing at **`https://keepsave.draveniq.dev`** remains separate. The full app host is a deployment target, not a claim that the backend has been published.

| Environment | Frontend origin | GitHub callback | Google callback |
| --- | --- | --- | --- |
| Production app target | `https://app.keepsave.draveniq.dev` | `https://app.keepsave.draveniq.dev/auth/callback/github` | `https://app.keepsave.draveniq.dev/auth/callback/google` |
| Local development example | `http://127.0.0.1:4651` | `http://127.0.0.1:4651/auth/callback/github` | `http://127.0.0.1:4651/auth/callback/google` |
| Default development Compose | `http://localhost:3002` | `http://localhost:3002/auth/callback/github` | `http://localhost:3002/auth/callback/google` |

Callbacks serve the frontend SPA, which posts the code to the API. Register exact origins and callbacks; do not mix `localhost` with `127.0.0.1`. Browser flow proofs live in the starting origin's tab. Use separate provider apps for development and production. The marketing host must not be registered as the application callback.

## GitHub OAuth app

1. Open [GitHub's OAuth app registration](https://github.com/settings/applications/new).
2. Use an application name identifying the environment, the frontend origin as the homepage, and the exact GitHub callback above.
3. Generate a client secret and store its client ID/secret in the backend secret manager as `GITHUB_CLIENT_ID` and `GITHUB_CLIENT_SECRET`.
4. Keep the sign-in app dedicated to identity: KeepSave requests `read:user user:email`, validates the numeric user ID, and requires the primary email to be verified. Private primary emails are obtained from GitHub's authenticated email endpoint.

S256 PKCE, random state, exact callback binding, and a fresh authenticated identity lookup protect the code flow. Provider tokens are used transiently for identity checks and are never returned to the browser or saved as vault credentials. Follow [GitHub's official code-flow requirements](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps).

**This OAuth app is separate from the GitHub App credential broker candidate.** Signing in requests no repository access and creates no provider binding, repository grant, or harness run. The separate broker pilot needs a separately configured GitHub App installation with explicit repository permissions.

## Google OAuth client

1. Open the [Google Cloud console](https://console.cloud.google.com/) and choose the KeepSave project.
2. Complete Google Auth Platform branding and audience settings. For an external test client, use Testing and explicitly enroll the human test accounts.
3. Create a **Web application** OAuth client. Register the exact Google callback above under authorized redirect URIs. This server code-exchange flow does not use a JavaScript SDK origin; any origin configured should still match the application host.
4. Store `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET` in the backend secret manager. Keep requested scopes to `openid email`.
5. Before public rollout, complete the applicable production branding, verified-domain, privacy-policy, audience, and publishing requirements shown in the console.

The backend verifies the Google issuer, signature, client audience, expiry, flow nonce, and verified email using go-oidc. It does not request Google Drive access or offline refresh access. See [Google's official OpenID Connect flow](https://developers.google.com/identity/openid-connect/openid-connect).

## Backend configuration

For the production app target, set these backend-only variables in the private launch environment:

```dotenv
SOCIAL_AUTH_ORIGIN=https://app.keepsave.draveniq.dev
GITHUB_CLIENT_ID=<GitHub client ID>
GITHUB_CLIENT_SECRET=<GitHub client secret>
GOOGLE_CLIENT_ID=<Google client ID>
GOOGLE_CLIENT_SECRET=<Google client secret>
```

Use the selected loopback origin for a development instance. Both values for a provider may remain empty to disable it. Incomplete provider credentials fail startup. Enabled providers require a valid canonical `SOCIAL_AUTH_ORIGIN`; production requires HTTPS, while development HTTP is restricted to loopback.

Do not put client secrets in frontend configuration, `VITE_` variables, chat, source control, or screenshots. Retain the existing vault recovery/key material and signing configuration. The process reads its environment at startup; applying configuration requires an explicitly authorized restart/recreation of that instance. No restart, provider setup, or production deployment was performed by this implementation.

Serve `/auth/callback/*` through the SPA fallback. Prefer frontend/API routing on the same app origin. If they are separated, set `VITE_API_BASE_URL`, allow only the application origin in CORS, and include the API origin in `connect-src`. Avoid callback query logging in proxies, analytics, and CDNs. The frontend removes the callback query before exchanging its code and uses `no-referrer`.

## Account identity and canonical email migration

Identity ownership is `(provider, subject)`, not an email assertion. A matching provider identity reopens its stored account. A new identity creates an account only when its canonical email is unused. Email collision requires signing in using the existing method, then connecting the provider from Account.

Migration **020_identity_sessions** adds case-insensitive trimmed email uniqueness, linking-session references, and operator grants. It does not merge accounts or rewrite missing identity history. Existing canonical collisions stop migration; an operator must investigate and resolve the actual account ownership before retrying. A read-only collision-count preflight is:

```sql
SELECT COUNT(*) AS collision_groups
FROM (
  SELECT LOWER(TRIM(email))
  FROM users
  GROUP BY LOWER(TRIM(email))
  HAVING COUNT(*) > 1
) AS collisions;
```

Password signup does not verify email ownership. Self-claimed email is never a global-admin credential. No signup, social callback, or migration automatically grants global administrator access, converts an email allowlist, or attaches a user to an existing organization. Social-only accounts initially have no usable password. The default-off identity slice now inventories/removes login methods and provides SMTP-gated password recovery; real delivery/method acceptance is pending. Removing a method requires an actual recent successful authentication through a remaining method, not merely an old signed-in tab. See the section below; no automatic method conversion occurs.

## Browser sessions and cutover

Password registration/login and social sign-in preserve **`{user, token}`** and **Bearer authorization**. Each successful sign-in creates a fresh stored session with **`sid = jti`**, an exact token hash, and a **24-hour maximum**. There is no refresh-cookie or refresh-token redesign. Session tokens remain scoped to browser tabs; SDKs retain a separate API-key credential when signing out a human session.

Every protected human request checks authoritative database status, stored ownership, token hash, expiry, and revocation. Database unavailability denies the request. Logout/revocation commits with its audit event and, on PostgreSQL, an identity outbox event. A failed commit must not report successful revocation or silently clear an otherwise active browser identity. A protected 401 clears local identity and pending provider proofs; switching identities also clears these proofs and SDK secret caches.

Cut over all API replicas together. Startup enables mandatory stored-session validation, and **legacy human JWTs without an explicit human type and valid `sid` are rejected**. Existing users must sign in again. Existing session rows are not converted into new authority. Agent tokens and API keys retain their separate scoped authorization paths; they are not human sessions. Do not roll back to an old binary that ignores session revocation.

Connecting a provider requires the **same starting user and session** at completion, and authentication within the preceding **ten minutes**. A changed session, revoked/expired session, stale authentication, changed provider, invalid PKCE proof, or consumed flow is denied. Sign in again before linking if the recent-authentication window has elapsed. The provider identity cannot be moved from another user.

Account lists the most recent 100 owned sessions using metadata only. Revocation blocks admissions after its database commit; already-dispatched work and previously returned data cannot be recalled.

## Operator-controlled global administration

`platform_admin_grants` binds reviewed authority to a stable existing **user UUID**. `KEEPSAVE_PLATFORM_ADMIN_EMAILS` is deprecated and is not used by the application's operator gate. There is no public endpoint for granting global administration, and scoped agent tokens cannot inherit a human operator's grant.

Use the private operator binary with the instance's existing database/key configuration after separately verifying the intended user UUID:

```sh
keepsave-operator --action grant --user-id "$REVIEWED_USER_ID" --reason "Approved operator account"
keepsave-operator --action revoke --user-id "$REVIEWED_USER_ID" --reason "Operator access withdrawn"
```

The command identifies the local operator, verifies the audit chain, and records the stored grant/revocation transactionally. It does not migrate the database, create users, trust an email allowlist, or accept raw encryption keys as command arguments. Use synthetic identities in CI; granting a real operator remains an intentional operator action.

## Management API contract

Paths below are under `/api/v1`; frontend resource types are generated from the core OpenAPI contract.

| Method / path | Authority | Result |
| --- | --- | --- |
| `GET /auth/providers` | Public | `{github, google}` booleans; availability only |
| `POST /auth/social/:provider/start` | Public | `{code_challenge}` → `{authorization_url, state}` |
| `POST /auth/social/:provider/complete` | One-time state/PKCE proof | `{code,state,code_verifier}` → `{user,token}` |
| `GET /account/connections` | Active human session | `{connected,available}` |
| `POST /account/connections/:provider/start` | Active, recently authenticated human session | Session-bound provider flow |
| `POST /account/connections/:provider/complete` | Same active, recently authenticated session + proof | `{linked:true}` |
| `GET /account/sessions` | Active human session | `{sessions:[{id,current,status,created_at,expires_at,ip_address,user_agent}]}` |
| `DELETE /account/sessions/:sessionId` | Active human session, owned target | 204 after committed revocation; foreign target 404 |
| `POST /auth/logout` | Active human session | 204 after committed current-session revocation |

Only `github` and `google` are valid providers. Start/complete share a ten-per-minute per-IP limiter with burst ten. Invalid/expired/revoked human sessions return 401; authoritative session database failure returns 503. Listing and session controls use `Cache-Control: no-store`. Session responses contain no credential or token hash.

## Login methods, verified contacts and recovery

The default-off PostgreSQL identity slice adds the following management routes
under `/api/v1`. The sole [OpenAPI source](../backend/internal/api/openapi/core.json)
defines request bodies; these routes do not expose proof strings.

| Route | Behavior |
|---|---|
| `GET /account/methods` | Inventory the caller's current login methods |
| `DELETE /account/methods/:method` | Refuse the last method; require recent successful remaining-method authentication |
| `GET /account/contacts` | Current caller contact/verification metadata |
| `POST /account/contact-proofs` and `POST /account/contact-proofs/:proofId/confirm` | Request/consume a purpose/account-bound contact proof |
| `GET /account/proofs/:proofId` and `POST /account/proofs/:proofId/resend` | Owned proof status; explicit resend invalidates the old proof |
| `POST /auth/recovery/request` and `POST /auth/recovery/confirm` | Enumeration-resistant initiation and proof-bound password recovery |

Contact/recovery proofs use 256-bit random values, hashed verification storage,
15-minute expiry and bounded attempts. Invitation proofs use a separate 24-hour
lifetime. Browser links carry the proof in a fragment cleared before submitting
it in a nonlogged body. Jobs carry delivery IDs only; necessary proof delivery
material is short-lived Vault-encrypted ciphertext.

Authenticated SMTP requires certificate-verified STARTTLS and no plaintext
fallback. Configure `KEEPSAVE_APPLICATION_ORIGIN`,
`KEEPSAVE_IDENTITY_ENABLED` and the private SMTP fields, then record a real
installation-specific delivery exercise before setting `KEEPSAVE_SMTP_ACCEPTED`.
A queued job or configured address is not accepted delivery. Report **SMTP
accepted**, not delivered; uncertain sends are not automatically replayed.

Successful recovery holds current user authority and atomically updates the
password, consumes the proof, revokes browser/linking sessions, expires retained
API keys, revokes related leases and denylists persisted agent issuance. Delegated
OAuth subsequently fails its revoked browser-parent check. A missing revocation
dependency or failed audit/transaction refuses recovery rather than partially
resetting the account. Unrelated accounts retain their authority. Reauthenticate
and create fresh authorized credentials afterward.

Verified-contact invitations and scoped organization offboarding use separate
current-authority checks. A verified email is not operator authority, device
attestation or permission to join an arbitrary organization. See
[setup](design/2026-10-02-harness-neutral-platform/SETUP.md) and
[runbook](RUNBOOK.md) for the bounded team flow.

## Real-provider acceptance exercise — pending

1. Record the application origin, provider app/client identifiers, configured callbacks, backend build, migration version, and browser version without secrets.
2. Confirm unconfigured providers stay disabled, then configure each separate development client privately and restart only the authorized disposable instance.
3. Complete actual Google and GitHub consent in the same browser tab. Verify the requested scopes and a new identity's empty account boundary. Sign out and confirm repeat login uses the same stored user.
4. Register/login with a password, then connect each provider within ten minutes from Account. Sign out and confirm provider login reopens that exact account. Attempt email collision through an unlinked identity and confirm there is no automatic merge.
5. Exercise cancellation, callback mismatch, cross-tab callbacks, expired flow, reused code/state, swapped provider, wrong-session completion, and stale linking authentication.
6. Create two human sessions. List/revoke the other session; its next protected request must return 401 while the current one remains usable. Confirm current-session logout and a failed-database logout outcome.
7. Record the synthetic and real-provider results separately. Synthetic HTTP/JWKS fixtures, structural checks, and a configured button do not establish successful provider UAT.

Use the accepted Field Twist mark and KeepSave branding in the provider consent applications while preserving recognizable native provider logos. Provider-console settings and publication requirements must be verified on the actual selected installation.

Google/GitHub app creation and secrets remain operator setup work. No external consent flow or repository broker connection is claimed complete.

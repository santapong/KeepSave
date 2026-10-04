# Event Horizon login and external identity — RFC

> **Identity scope, clarified 2026-10-04.** The initial no-session-change/no-unlink description below is historical, and the appended core checkpoint records its later replacement. Subsequent safe login-method management and delegated-authority changes are in the [harness-neutral checkpoint](2026-10-02-harness-neutral-platform/README.md). Current operator setup and real-provider acceptance requirements remain in [SOCIAL_LOGIN_SETUP](../SOCIAL_LOGIN_SETUP.md); synthetic fixtures do not qualify either provider for production.

2026-09-28. Explicit owner request: apply the proposed login now and support GitHub and Google. Owner confirmed neither OAuth app exists and requested setup steps. No live provider configuration or production action is authorized by this implementation record.

Type-2 additive external identity adapters and UI, with security review required before integration. Existing JWT signing/expiry, password checks, agent API keys and ownership middleware remain unchanged. No breaking schema, crypto primitive or key hierarchy change. The new trust surface is Internet/provider → backend identity exchange → existing human session. This supersedes the login concept's earlier omission of social providers.

## Design and execution plan

Inline in the existing isolated feature worktree. Implement backend provider adapters and additive migrations, then the selected login design and callback/account-connections UI. Verify provider HTTP contracts with isolated test servers, signed Google token validation, state/PKCE expiry/replay/provider/mode/user binding, verified email, stable identity, collision rejection, account linking authorization, audit rows, frontend pending/error/callback behavior, and real browser reflow. Existing frontend suite, backend tests/race checks, lint/vet and app/widget builds are gates for the local handoff. Owner visual acceptance and human Security/Tech Lead integration review remain separate.

OAuth authorization code flow uses S256 PKCE for both providers. A per-flow verifier lives only in sessionStorage and must survive navigation in the same tab. The backend stores only its challenge, a hash of random state, provider, optional linking user, and a ten-minute expiry. Atomic conditional deletion prevents callback replay. The provider redirects to the configured frontend `/auth/callback/{provider}`; the page removes the query immediately and sends code/state/verifier in a POST body. Backend secrets and provider tokens never enter frontend code, redirects, logs or persistent identity rows. Browser storage denial aborts sign-in rather than dropping flow binding.

Google ID tokens are verified by maintained go-oidc using pinned Google issuer/JWKS, client audience, expiry, signature and the flow nonce; email must be verified. GitHub identity uses numeric user ID and a primary verified email from its authenticated API. Minimum scopes: Google `openid email`; GitHub `read:user user:email`. No repository or Drive access.

Identity key is `(provider, subject)`, never email. A new identity can create a new user only if no case-insensitive email collision exists. Collision returns a generic instruction to sign in using the existing method and connect the provider in Account. Linking requires an existing JWT at both start and completion, bound to that same user in the stored flow. It cannot move an identity already owned by another user. Social-only users receive a non-password sentinel, not a generated usable password. No unlink flow is added, avoiding accidental lockout.

Only explicitly configured providers are offered as available. Empty config leaves them unavailable; incomplete/unsafe config fails startup. One canonical frontend origin supplies exact callback URLs, HTTPS except loopback development. No request-controlled return URL or provider endpoint. Network calls have deadlines, body limits and no redirects. Public auth routes receive a dedicated rate limit.

## Alternatives and rollback

Rejected email auto-linking: unverified password registrations and reassigned email identities can expose existing vaults. Rejected frontend-only buttons: they would imply authentication without a backend. Keep existing email/password login; deploys can disable either provider by clearing both of its credentials, leaving additive identity tables intact. Do not remove tables containing linked identities as an incident workaround.

Provider apps/credentials and real consent/account-login verification are pending owner setup. No agent review substitutes for required human security approval before integration. No merge, push or deployment is part of this request.

Sources checked 2026-09-28: [GitHub code flow and PKCE](https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps), [Google OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect), [go-oidc](https://github.com/coreos/go-oidc), [Google button guidance](https://developers.google.com/identity/branding-guidelines).

## Superseding core identity checkpoint — 2026-10-02

The original Type-2/JWT-unchanged description above is historical. The approved core release adds Type-1 identity/session and authorization changes covered by ADR-0028, with independent Security/Tech Lead review still pending. The application target is `https://app.keepsave.draveniq.dev`; the static marketing landing remains separate. Open registration is intentional, and neither email signup nor social sign-in grants organization membership or global administration.

Migration020 enforces canonical trimmed/case-insensitive email uniqueness and refuses existing collisions without merging identities. Password and social identity/session/audit writes share a transaction; PostgreSQL also records safe identity outbox events. Every new human sign-in receives a fresh 24-hour Bearer session with `sid=jti` and a stored token hash. Mandatory database checks deny revoked, expired, missing, and unavailable authority. Coordinated cutover requires all old human JWTs to reauthenticate; old binaries cannot safely honor the new revocation contract.

Account linking binds the starting session as well as user/provider/PKCE, requires authentication within ten minutes, and checks the session again transactionally before linking. Account exposes metadata-only session listing, owned-session revocation, and committed current-session logout. Failed logout cannot imply server revocation. Operator authority now comes from reviewed stable user-ID grants installed through a private operator command, never self-claimed email or an automatically migrated allowlist. Agent/API-key identities remain separate.

The GitHub identity OAuth app remains separate from the future GitHub App broker installation. Synthetic verification is local implementation evidence; actual Google/GitHub app setup, human consent/UAT, required independent review, and deployment remain unperformed. Current setup, session contracts, migration preflight, cutover, operator procedure, and UAT checklist are maintained in [SOCIAL_LOGIN_SETUP.md](../SOCIAL_LOGIN_SETUP.md). Official GitHub/Google code-flow references were rechecked 2026-10-02.

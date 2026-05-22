# Grovernance Platform integration

The Grovernance Platform (https://github.com/santapong/grovernance-platfrom) is
KeepSave's downstream deploy-governance gate. It uses KeepSave's OAuth
infrastructure as its identity provider: every actor token that reaches
Grovernance's `/authorize` endpoint is resolved against KeepSave's
`GET /api/v1/oauth/userinfo`.

This document pins the contract between the two services.

## /userinfo response shape

### Human users (user tokens minted via `authorization_code` or `password`-equivalent grants)

```json
{
  "sub":        "<user-uuid>",
  "email":      "alice@example.com",
  "created_at": "2026-01-12T08:45:11.000Z",
  "scopes":     ["read", "promote"],
  "groups":     ["platform-ops:admin", "deploy-stage:promoter"]
}
```

- `sub` — KeepSave user UUID. Stable across token rotation.
- `groups` — Organisation memberships, formatted as `"<org-slug>:<role>"`.
  Always present; empty array when the user has no memberships.
- `scopes` — Token's authorised scopes.

### Service accounts (client_credentials grant)

```json
{
  "sub":        "<oauth-client-uuid>",
  "token_type": "service_account",
  "scopes":     ["deploy:svc-x"]
}
```

- `sub` — OAuth client ID acting as the service-account identity.
- `token_type` — Stable discriminator the consumer branches on. Was
  `"client_credentials"` historically; renamed to `"service_account"` to make
  the *role* of the token (machine principal) explicit, distinct from the
  *grant type* used to mint it.
- `scopes` — Token's authorised scopes; used by Grovernance to check
  `AllowedServiceAccounts` membership when needed.

Grovernance's adapter accepts both `"service_account"` and the legacy
`"client_credentials"` value to keep older KeepSave deployments compatible.

## Group format rationale

`<org-slug>:<role>` is a flat, opaque string from KeepSave's perspective —
KeepSave does not know what Grovernance does with it. Grovernance services
encode `ProdDeployerGroups` as the same `<slug>:<role>` strings, e.g.
`ProdDeployerGroups: ["platform-ops:admin"]`. Membership match is a plain set
intersection; KeepSave's role taxonomy (`viewer | editor | admin | promoter`)
is exposed as-is.

## What this PR added

- `OrganizationRepository.ListMembershipGroupsByUserID` — returns the
  `[]string` of `"slug:role"` for a user.
- `OAuthService.GetUserInfo` — fetches memberships via the org repo and
  includes them on the response.
- `OAuthService` constructor now takes `*repository.OrganizationRepository`
  (previously two repos — `oauthRepo`, `userRepo`).

## Verifying the contract

Running locally:

```bash
# Mint a user token via /api/v1/auth/login or the authorization_code flow,
# then call /userinfo:
curl -sH "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/oauth/userinfo | jq

# Expected: response includes a `groups` array (possibly empty).
```

Grovernance's IdentityResolver test
(`internal/adapter/outbound/keepsave/identity_test.go`) covers the consumer
side using `httptest`.

# Negative-Auth Test Plan (QA + Backend 30-day)

Authentication middleware exists and rejects bad requests in code (`backend/internal/api/middleware.go:59-120`), but **no test verifies it**. A middleware regression would land green. This plan enumerates the missing negative tests by endpoint.

This is QA 30-day work item §2 + Backend 30-day work item §2 from `docs/ROLES_30_60_90.md`.

---

## Matrix: endpoint × attacker case

For each endpoint and each attacker case, a test must exist that asserts the *correct* error response (status + sanitized body per `ERROR_HANDLING_STANDARD.md`).

### Endpoints in scope (state-mutating + sensitive reads)

| ID  | Method | Path                                                       |
|-----|--------|------------------------------------------------------------|
| E1  | POST   | `/api/v1/projects/:id/secrets`                              |
| E2  | PUT    | `/api/v1/projects/:id/secrets/:secret_id`                   |
| E3  | DELETE | `/api/v1/projects/:id/secrets/:secret_id`                   |
| E4  | GET    | `/api/v1/projects/:id/secrets/:secret_id` (plaintext reveal)|
| E5  | POST   | `/api/v1/projects`                                          |
| E6  | PUT    | `/api/v1/projects/:id`                                      |
| E7  | DELETE | `/api/v1/projects/:id`                                      |
| E8  | POST   | `/api/v1/projects/:id/api-keys`                             |
| E9  | DELETE | `/api/v1/projects/:id/api-keys/:key_id`                     |
| E10 | POST   | `/api/v1/projects/:id/promote`                              |
| E11 | POST   | `/api/v1/projects/:id/promotions/:promotion_id/approve`     |
| E12 | POST   | `/api/v1/projects/:id/promotions/:promotion_id/rollback`    |

### Attacker cases

| Case | Scenario                                                                 | Expected response                          |
|------|--------------------------------------------------------------------------|--------------------------------------------|
| A1   | No `Authorization` header and no `X-API-Key`                              | 401 `UNAUTHORIZED`                          |
| A2   | Malformed `Authorization: Bearer ...` (truncated JWT, wrong segments)     | 401 `UNAUTHORIZED`                          |
| A3   | Expired JWT (valid signature, `exp` in past)                              | 401 `UNAUTHORIZED`                          |
| A4   | JWT signed with wrong secret                                              | 401 `UNAUTHORIZED`                          |
| A5   | Valid JWT but for a user who has no membership of target project          | 403 `FORBIDDEN`                             |
| A6   | API key valid but scoped to a different project ID                        | 403 `FORBIDDEN`                             |
| A7   | API key valid but `environment` scope does not match the targeted env      | 403 `FORBIDDEN`                             |
| A8   | API key valid but `scopes[]` does not include the required scope           | 403 `FORBIDDEN`                             |
| A9   | API key valid but deleted (revoked) between issuance and request           | 401 `UNAUTHORIZED`                          |
| A10  | Approver-equals-requester on E11 (promotion approve)                      | 403 `FORBIDDEN` (invariant — see ADR-0003)   |
| A11  | Rate-limit exceeded on the auth endpoint                                  | 429 `RATE_LIMITED`                          |

## Coverage matrix

`✗` = test required and missing today. The whole matrix is `✗` today; this is the work.

|     | E1 | E2 | E3 | E4 | E5 | E6 | E7 | E8 | E9 | E10 | E11 | E12 |
|-----|----|----|----|----|----|----|----|----|----|-----|-----|-----|
| A1  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A2  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A3  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A4  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A5  | ✗  | ✗  | ✗  | ✗  | —  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A6  | ✗  | ✗  | ✗  | ✗  | —  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A7  | ✗  | ✗  | ✗  | ✗  | —  | —  | —  | —  | —  | ✗   | —   | —   |
| A8  | ✗  | ✗  | ✗  | ✗  | —  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A9  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗  | ✗   | ✗   | ✗   |
| A10 | —  | —  | —  | —  | —  | —  | —  | —  | —  | —   | ✗   | —   |
| A11 | ✗  | —  | —  | —  | —  | —  | —  | —  | —  | —   | —   | —   |

(`—` = not applicable to this endpoint.)

## Test shape (Go)

A single test helper makes this not-painful. Pseudocode:

```go
type authTest struct {
    name     string
    setup    func(t *testing.T, fx *fixture)
    request  func(fx *fixture) *http.Request
    wantCode int
    wantCode string  // body code, e.g. "UNAUTHORIZED"
}

func runAuthMatrix(t *testing.T, endpoint string, cases []authTest) {
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            fx := newFixture(t)
            if tc.setup != nil { tc.setup(t, fx) }
            w := httptest.NewRecorder()
            fx.handler.ServeHTTP(w, tc.request(fx))
            require.Equal(t, tc.wantCode, w.Code)
            // assert sanitized body
            var body struct{ Error struct{ Code string `json:"code"` } `json:"error"` }
            require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
            require.Equal(t, tc.wantCode, body.Error.Code)
        })
    }
}
```

Each endpoint then exposes a slice of `authTest` cases. The matrix above translates directly into table entries.

## Priority of implementation

Day 1-3:
- E10 (promote) all cases — highest blast radius.
- E11 (approve) including A10 (the invariant). A10 may require a code change if not enforced at the DB layer; that's a Backend item.

Day 3-7:
- E1/E2/E3 (secret CRUD) all cases.

Day 7-14:
- E5/E6/E7 (project CRUD); E8/E9 (API key CRUD); E12 (rollback).

Day 14-21:
- E4 (read) all cases; A11 (rate-limit) across endpoints. The read path needs the audit/sampling decision (see `AUDIT_LOG_COVERAGE.md` §"Reads") before testing.

Day 21-30:
- Coverage gates wired in CI: PR blocked if a new state-mutating endpoint lacks at least A1+A3+A5+A9 coverage.

## Definition of done

- All `✗` cells become `✓` cells with a test file:line ref.
- A CI presence-check ensures the matrix doesn't regress: new endpoints without negative-auth tests fail the build.
- This file is updated in the same PR as any new endpoint or attacker case.

## References

- `backend/internal/api/middleware.go:59-120`
- `docs/ERROR_HANDLING_STANDARD.md` (the body shape these tests assert)
- ADR-0003 §Open Questions (A10 invariant)
- `docs/THREAT_MODEL.md` v1.2.0 §"Findings new" §2

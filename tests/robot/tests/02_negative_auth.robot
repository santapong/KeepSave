*** Settings ***
Documentation     KeepSave acceptance — STORY-LEVEL negative auth only.
...               The exhaustive 12x11 endpoint x attacker matrix stays in Go
...               (tests/NEGATIVE_AUTH_PLAN.md, in-process httptest, sub-ms). Robot must NOT
...               re-run that matrix over HTTP — it covers only black-box acceptance negatives
...               where the round-trip itself is the thing under test.
Resource          ../resources/keepsave.resource
Suite Setup       Connect To KeepSave
Force Tags        api    negauth


*** Test Cases ***
Unauthenticated Project Create Is Rejected
    ${body}=    Create Dictionary    name=should-not-be-created
    ${resp}=    POST On Session    ${SESSION}    /api/v1/projects    json=${body}    expected_status=any
    Should Be Equal As Integers    ${resp.status_code}    401

Garbage Bearer Token Is Rejected
    ${h}=    Create Dictionary    Authorization=Bearer not-a-real-token
    ${resp}=    GET On Session    ${SESSION}    /api/v1/projects    headers=${h}    expected_status=any
    Should Be Equal As Integers    ${resp.status_code}    401

Secret Read Without Any Credentials Is Rejected
    ${dummy}=    Set Variable    00000000-0000-0000-0000-000000000000
    ${resp}=    GET On Session    ${SESSION}    /api/v1/projects/${dummy}/secrets    expected_status=any
    Should Be Equal As Integers    ${resp.status_code}    401

Error Body Does Not Leak Internal Detail
    [Documentation]    Acceptance-layer guard for docs/ERROR_HANDLING_STANDARD.md: rejected
    ...                requests must not echo raw driver/runtime strings. Supplements (never
    ...                replaces) the Go AST gate in internal/api/error_leak_test.go.
    ${h}=    Create Dictionary    Authorization=Bearer not-a-real-token
    ${resp}=    GET On Session    ${SESSION}    /api/v1/projects    headers=${h}    expected_status=any
    Should Not Contain    ${resp.text}    panic
    Should Not Contain    ${resp.text}    sql:
    Should Not Contain    ${resp.text}    goroutine

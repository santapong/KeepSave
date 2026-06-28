*** Settings ***
Documentation     Reusable keywords + variables for KeepSave acceptance tests.
...               Targets a RUNNING KeepSave API (set KEEPSAVE_URL; default localhost:8080).
...               The exact request/response contract here is taken from the working
...               Go E2E client at tests/e2e/seidr/tester/main.go and the handlers.
Library           RequestsLibrary
Library           Collections
Library           String


*** Variables ***
${BASE_URL}       %{KEEPSAVE_URL=http://localhost:8080}
${SESSION}        keepsave
# A fixed test identity: register is ignored on re-runs, login still succeeds
# (same pattern as the Seidr Go tester).
${TEST_EMAIL}     rf-e2e@keepsave.local
${TEST_PASSWORD}    RobotFw-E2E-Pass-7zq!
# TEST DATA ONLY. Never put a real secret value in a fixture (KeepSave invariant,
# docs/HARNESS_ENGINEERING.md §6 / CLAUDE.md audit + no-plaintext rules).
${SECRET_KEY}     RF_E2E_MARKER
${SECRET_VALUE}    rf-plaintext-that-must-round-trip


*** Keywords ***
Connect To KeepSave
    [Documentation]    Open the HTTP session and block until /readyz returns 200.
    Create Session    ${SESSION}    ${BASE_URL}
    Wait Until Keyword Succeeds    60x    1s    KeepSave Is Ready

KeepSave Is Ready
    ${resp}=    GET On Session    ${SESSION}    /readyz    expected_status=any
    Should Be Equal As Integers    ${resp.status_code}    200

Register And Login
    [Documentation]    Register (ignored if the user exists) then login. Returns a JWT.
    [Arguments]    ${email}=${TEST_EMAIL}    ${password}=${TEST_PASSWORD}
    ${creds}=    Create Dictionary    email=${email}    password=${password}
    POST On Session    ${SESSION}    /api/v1/auth/register    json=${creds}    expected_status=any
    ${resp}=    POST On Session    ${SESSION}    /api/v1/auth/login    json=${creds}    expected_status=200
    ${body}=    Set Variable    ${resp.json()}
    ${token}=    Set Variable    ${body}[token]
    Should Not Be Empty    ${token}    /auth/login returned an empty token
    RETURN    ${token}

Bearer Headers
    [Arguments]    ${token}
    ${h}=    Create Dictionary    Authorization=Bearer ${token}
    RETURN    ${h}

Create Project
    [Arguments]    ${token}    ${name}
    ${h}=    Bearer Headers    ${token}
    ${body}=    Create Dictionary    name=${name}    description=Robot Framework acceptance project
    ${resp}=    POST On Session    ${SESSION}    /api/v1/projects    json=${body}    headers=${h}    expected_status=any
    Should Be True    ${resp.status_code} in [200, 201]    project create failed: ${resp.status_code} ${resp.text}
    ${id}=    Set Variable    ${resp.json()}[id]
    Should Not Be Empty    ${id}    project create returned an empty id
    RETURN    ${id}

Store Secret
    [Arguments]    ${token}    ${project_id}    ${key}    ${value}    ${environment}=alpha
    ${h}=    Bearer Headers    ${token}
    ${body}=    Create Dictionary    key=${key}    value=${value}    environment=${environment}
    ${resp}=    POST On Session    ${SESSION}    /api/v1/projects/${project_id}/secrets    json=${body}    headers=${h}    expected_status=any
    Should Be True    ${resp.status_code} in [200, 201]    store secret failed: ${resp.status_code} ${resp.text}

Create Scoped Api Key
    [Documentation]    Mint a per-project, per-environment, scoped API key. Returns the raw key.
    [Arguments]    ${token}    ${project_id}    ${environment}=alpha    ${scope}=read
    ${h}=    Bearer Headers    ${token}
    ${scopes}=    Create List    ${scope}
    ${body}=    Create Dictionary    name=rf-agent    project_id=${project_id}    scopes=${scopes}    environment=${environment}
    ${resp}=    POST On Session    ${SESSION}    /api/v1/api-keys    json=${body}    headers=${h}    expected_status=any
    Should Be True    ${resp.status_code} in [200, 201]    api-key create failed: ${resp.status_code} ${resp.text}
    ${json}=    Set Variable    ${resp.json()}
    # Response may carry the secret under raw_key or key.
    ${raw}=    Get From Dictionary    ${json}    raw_key    ${EMPTY}
    IF    '${raw}' == '${EMPTY}'
        ${raw}=    Get From Dictionary    ${json}    key    ${EMPTY}
    END
    Should Not Be Empty    ${raw}    api-key response had neither raw_key nor key
    RETURN    ${raw}

Read Secrets As Agent
    [Documentation]    Read secrets for an environment using a scoped API key (X-API-Key).
    [Arguments]    ${raw_key}    ${project_id}    ${environment}=alpha
    ${h}=    Create Dictionary    X-API-Key=${raw_key}
    ${params}=    Create Dictionary    environment=${environment}
    ${resp}=    GET On Session    ${SESSION}    /api/v1/projects/${project_id}/secrets    params=${params}    headers=${h}    expected_status=200
    RETURN    ${resp.json()}

Secret Value From Response
    [Documentation]    Extract a key's value whether the body is {secrets:[...]} or a bare array.
    [Arguments]    ${json}    ${key}
    ${secrets}=    Evaluate    $json['secrets'] if isinstance($json, dict) and 'secrets' in $json else $json
    ${value}=    Evaluate    next((s['value'] for s in $secrets if s.get('key') == $key), None)
    RETURN    ${value}

Promote
    [Documentation]    Promote selected keys between environments (alpha->uat->prod). Returns the response.
    [Arguments]    ${token}    ${project_id}    ${source}    ${target}    @{keys}
    ${h}=    Bearer Headers    ${token}
    ${keylist}=    Create List    @{keys}
    ${body}=    Create Dictionary    source_environment=${source}    target_environment=${target}    keys=${keylist}    notes=RF acceptance test
    ${resp}=    POST On Session    ${SESSION}    /api/v1/projects/${project_id}/promote    json=${body}    headers=${h}    expected_status=any
    RETURN    ${resp}

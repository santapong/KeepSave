*** Settings ***
Documentation     KeepSave acceptance — the canonical round-trip, human-readable.
...               A secret stored via a JWT is read back through a scoped per-environment
...               API key and the plaintext matches. This is the Seidr Go E2E expressed at
...               the Robot Framework acceptance tier (the "tip of the pyramid").
Resource          ../resources/keepsave.resource
Suite Setup       Connect To KeepSave
Force Tags        api    smoke


*** Test Cases ***
Secret Round-Trips Through A Scoped API Key
    ${token}=    Register And Login
    ${project_id}=    Create Project    ${token}    rf-happy-path
    Store Secret    ${token}    ${project_id}    ${SECRET_KEY}    ${SECRET_VALUE}    alpha
    ${raw_key}=    Create Scoped Api Key    ${token}    ${project_id}    alpha    read
    ${json}=    Read Secrets As Agent    ${raw_key}    ${project_id}    alpha
    ${value}=    Secret Value From Response    ${json}    ${SECRET_KEY}
    Should Be Equal    ${value}    ${SECRET_VALUE}    secret plaintext did not round-trip

Stored Secret Is Listed For Its Environment
    ${token}=    Register And Login
    ${project_id}=    Create Project    ${token}    rf-list-check
    Store Secret    ${token}    ${project_id}    ${SECRET_KEY}    ${SECRET_VALUE}    alpha
    ${raw_key}=    Create Scoped Api Key    ${token}    ${project_id}    alpha    read
    ${json}=    Read Secrets As Agent    ${raw_key}    ${project_id}    alpha
    ${value}=    Secret Value From Response    ${json}    ${SECRET_KEY}
    Should Not Be Equal    ${value}    ${None}    stored key was not present in the listing

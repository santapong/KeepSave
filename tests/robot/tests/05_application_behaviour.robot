*** Settings ***
Documentation     Real API behaviour using an owned disposable project; no provider or load claims.
Library           ../libraries/KeepSaveUX.py
Suite Setup       Prepare Synthetic Project
Suite Teardown    Clean Up Synthetic Project
Test Timeout      60s
Test Tags         application-api    application-behaviour

*** Test Cases ***
Fixture Refuses Remote Targets Before Any Request
    Remote Fixture Is Rejected Before Network Access

Protected Reads Require Authentication
    Unauthenticated Reads Are Denied

Batch Reads Respect Environment Boundaries And Validate Input
    Batch Reads Preserve Environment And Missing Keys

Logout Denies Subsequent Use Of That Session
    Revoked Session Cannot Read Projects

Bounded Serial Reads Remain Successful
    Serial Read Probe

*** Settings ***
Documentation     Actual browser UX checks on an explicitly configured synthetic loopback instance.
Library           Browser    timeout=15s
Library           ../libraries/KeepSaveUX.py
Suite Setup       Start UX Browser
Suite Teardown    Finish UX Browser
Test Timeout      90s
Test Tags         browser    application-ux

*** Variables ***
${ORIGIN}         %{KEEPSAVE_UI_URL=http://127.0.0.1:4663}
${CHROMIUM}       %{KEEPSAVE_CHROMIUM=}
@{CHROMIUM_ARGS}    --disable-gpu    --renderer-process-limit=2

*** Keywords ***
Start UX Browser
    Prepare Synthetic Project
    New Browser    chromium    executablePath=${CHROMIUM}    headless=${True}    chromiumSandbox=${True}    args=${CHROMIUM_ARGS}
    New Context    viewport={'width':1280,'height':900}    reducedMotion=reduce
    New Page    ${ORIGIN}

Finish UX Browser
    Run Keyword And Continue On Failure    Close Browser
    Clean Up Synthetic Project

No Page Overflow
    ${fits}=    Evaluate JavaScript    ${None}    () => document.documentElement.scrollWidth <= innerWidth
    Should Be True    ${fits}

Navigation Has Scrolled
    ${top}=    Get Property    .ks-navigation    scrollTop
    Should Be True    ${top} > 0

*** Test Cases ***
Landing Story Supports Keyboard Chapters And Independent Selection
    Click    [aria-label="Example vault environment"] >> button:has-text("PROD")
    Get Element Count    [role="table"][aria-label="PROD example secrets"]    ==    1
    Click    id=ks-chapter-1
    Press Keys    id=ks-chapter-1    End
    Get Attribute    id=ks-chapter-2    aria-selected    ==    true
    Press Keys    id=ks-chapter-2    Home
    Get Attribute    id=ks-chapter-0    aria-selected    ==    true
    Get Element Count    [role="table"][aria-label="PROD example secrets"]    ==    1
    Take Screenshot    filename=landing-desktop

Landing Selection And Nested Accents Use The Warm Palette
    Click    [aria-label="Example vault environment"] >> button:has-text("UAT")
    ${uat}=    Evaluate JavaScript    .ks-env-uat    e => getComputedStyle(e).backgroundColor
    Should Be Equal    ${uat}    rgb(244, 191, 118)
    ${border}=    Evaluate JavaScript    .ks-env-picker button[aria-pressed="true"]    e => getComputedStyle(e).borderTopColor
    Should Be Equal    ${border}    ${uat}
    Hover    .ks-env-picker button[aria-pressed="true"]
    ${check}=    Evaluate JavaScript    .ks-env-picker button[aria-pressed="true"] svg    e => getComputedStyle(e).color
    Should Be Equal    ${check}    ${uat}
    ${mask}=    Evaluate JavaScript    .ks-masked >> nth=0    e => getComputedStyle(e).color
    Should Be Equal    ${mask}    rgb(181, 178, 184)
    Take Screenshot    filename=landing-uat-warm
    Click    id=ks-chapter-1
    ${tool}=    Evaluate JavaScript    .ks-tool-icon    e => getComputedStyle(e).color
    Should Be Equal    ${tool}    ${uat}
    ${scope}=    Evaluate JavaScript    .ks-scope-chip    e => getComputedStyle(e).color
    Should Be Equal    ${scope}    ${uat}
    Take Screenshot    filename=landing-scope-warm
    Click    id=ks-chapter-0

Mobile Landing Menu And FAQ Work Without Page Overflow
    Set Viewport Size    390    844
    Click    button[aria-label="Open navigation"]
    Press Keys    body    Escape
    Get Element Count    nav[aria-label="Mobile navigation"]    ==    0
    Click    summary:has-text("Can I self-host KeepSave?")
    Get Property    details:has(summary:has-text("Can I self-host KeepSave?"))    open    ==    ${True}
    No Page Overflow
    Take Screenshot    filename=landing-mobile
    Set Viewport Size    1280    900

Unconfigured Social Providers Are Clearly Unavailable
    Go To    ${ORIGIN}/login
    Get Property    button:has-text("Continue with GitHub")    disabled    ==    ${True}
    Get Property    button:has-text("Continue with Google")    disabled    ==    ${True}
    Get Text    body    *=    awaiting setup

Developer Documentation Is Searchable And Shareable
    Sign In To Synthetic Workspace
    Go To    ${ORIGIN}/help#developer-guide
    Wait For Elements State    id=docs-search    visible
    Get Text    id=docs-article >> h2    ==    Run KeepSave locally
    Fill Text    id=docs-search    no-match-robot-acceptance
    Get Text    .cz-docs    *=    No guides match
    Fill Text    id=docs-search    troubleshooting
    Click    nav[aria-label="Documentation sections"] >> button:has-text("Troubleshoot a request")
    Get Url    *=    troubleshooting
    Get Text    id=docs-article >> h2    ==    Troubleshoot a request
    Fill Text    id=docs-search    ${EMPTY}
    Take Screenshot    filename=docs-desktop

Short Sidebar Has No Horizontal Bar And Retains Wheel And Keyboard Scrolling
    Set Viewport Size    1280    600
    ${fits}=    Evaluate JavaScript    .ks-navigation    n => n.scrollWidth <= n.clientWidth && n.scrollHeight > n.clientHeight
    Should Be True    ${fits}
    ${chrome}=    Evaluate JavaScript    .ks-navigation    n => getComputedStyle(n).scrollbarWidth
    Should Be Equal    ${chrome}    none
    Focus    .ks-navigation a[aria-label="Projects"]
    Hover    .ks-navigation
    Mouse Wheel    0    600
    Wait Until Keyword Succeeds    5x    200ms    Navigation Has Scrolled
    Focus    .ks-navigation a[aria-label="Projects"]
    FOR    ${index}    IN RANGE    4
        Keyboard Key    press    Tab
    END
    ${focused}=    Evaluate JavaScript    ${None}    () => document.activeElement.getAttribute('aria-label')
    Should Be Equal    ${focused}    Administration
    Navigation Has Scrolled
    ${visible}=    Evaluate JavaScript    .ks-navigation    n => { const a = document.activeElement.getBoundingClientRect(); const b = n.getBoundingClientRect(); return a.top >= b.top && a.bottom <= b.bottom; }
    Should Be True    ${visible}
    Take Screenshot    filename=sidebar-short-dark
    Emulate Media    forcedColors=active
    ${contrastChrome}=    Evaluate JavaScript    .ks-navigation    n => getComputedStyle(n).scrollbarWidth
    Should Be Equal    ${contrastChrome}    auto
    Emulate Media    forcedColors=none
    Set Viewport Size    1280    900

Mobile Documentation Remains Reachable In Both Themes
    Set Viewport Size    390    844
    Select Options By    .cz-docs-mobile select    value    getting-started
    Get Text    id=docs-article >> h2    ==    Your first secret
    No Page Overflow
    Click    button[aria-label="Open navigation"]
    Click    .ks-mobile-drawer >> button[aria-label="Switch to light mode"]
    Press Keys    body    Escape
    Get Attribute    html    data-theme    ==    light
    No Page Overflow
    Take Screenshot    filename=docs-mobile-light
    Set Viewport Size    1280    900

Theme Switch Preserves Draft And Saved Secret Survives Reload
    Open Synthetic Project
    Click    button:has-text("Add Secret")
    Fill Text    id=secret-key    ROBOT_UI_MARKER
    Fill Text    id=secret-value    synthetic-robot-ui-value
    Click    button[aria-label="Switch to dark mode"]
    Get Property    id=secret-key    value    ==    ROBOT_UI_MARKER
    Get Property    id=secret-value    value    ==    synthetic-robot-ui-value
    Click    button:text-is("Save")
    Wait For Elements State    tr:has-text("ROBOT_UI_MARKER")    visible
    Reload
    Wait For Elements State    tr:has-text("ROBOT_UI_MARKER")    visible
    Click    tr:has-text("ROBOT_UI_MARKER") >> button[title="Reveal value"]
    Get Text    tr:has-text("ROBOT_UI_MARKER")    *=    synthetic-robot-ui-value
    Click    tr:has-text("ROBOT_UI_MARKER") >> button[title="Hide value"]
    Click    tr:has-text("ROBOT_UI_MARKER") >> button:has-text("History")
    Get Text    [aria-labelledby="secret-history-title"]    *=    Revision 1
    Take Screenshot    filename=vault-history-dark

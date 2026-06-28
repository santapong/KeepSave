*** Settings ***
Documentation     KeepSave acceptance (BROWSER tier) — the <keepsave-widget> embed smoke test.
...               The widget attaches its shadow root with mode:'open'
...               (frontend/src/embed/keepsave-widget.ts:45), so the Browser library
...               (Playwright) pierces it with ordinary CSS locators — no special syntax.
...               RULES (from the RF plan): use CSS/text/role locators only — XPath does NOT
...               pierce shadow roots; assert the POSITIVE path only (negative-origin handling
...               stays in the Vitest unit tests).
...
...               Prereqs: the frontend stack running + a host page serving the widget bundle,
...               and `rfbrowser init` (installs the Playwright browsers) once.
...               Run only this tier:    robot --include browser tests/robot/tests/03_embed_widget.robot
...               It is TAGGED `browser` and excluded from the API-only dry-run / CI gate.
Library           Browser
Suite Setup       New Browser    chromium    headless=${True}
Suite Teardown    Close Browser
Force Tags        browser    embed


*** Variables ***
${HOST_PAGE_URL}    %{KEEPSAVE_WIDGET_HOST=http://localhost:3000/embed/example.html}


*** Test Cases ***
Widget Renders Environment Tabs Inside The Open Shadow DOM
    [Documentation]    Proves Playwright reaches widget internals through the open shadow root.
    New Page    ${HOST_PAGE_URL}
    Wait For Elements State    .ks-tab[data-env="alpha"]    visible    timeout=10s
    Get Element Count    .ks-tab    >=    1

Secret Values Are Masked Until Revealed
    [Documentation]    The default state must mask values (no plaintext on screen at rest).
    New Page    ${HOST_PAGE_URL}
    Wait For Elements State    .ks-secret-mask    visible    timeout=10s

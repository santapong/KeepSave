# UX State Inventory (UX interim — Phase A)

Every dashboard screen has multiple states: loading, empty, partial, error, denied, success. Today many of those states are implicit or missing — the user sees a blank panel or a stack-trace toast. This inventory documents what exists, what's missing, and where the security-sensitive states need designer + Security review.

UX Designer is not yet hired in Phase A; interim owner is Frontend Engineer + Tech Lead. This is the UX 30-day work item from `docs/ROLES_30_60_90.md`.

---

## Inventory format

For each screen:
- **Path / component file**
- **States that exist** (loading / empty / error / success / denied / revealed / editing)
- **Gaps** (states that are missing or broken)
- **Security-sensitive flags** (does the screen show plaintext, allow destructive actions, etc.)

## Screens

### 1. Login
- **File:** `frontend/src/pages/LoginPage.tsx`
- **Exists:** initial form, loading on submit, generic error.
- **Gaps:** no rate-limit feedback ("too many attempts" vs. "wrong password"); no lockout state; "forgot password" is not in scope but the path should at least be present (or absent on purpose) — verify.
- **Security flags:** error messages must NOT distinguish "user doesn't exist" from "wrong password" (verify against `auth_service.go` — currently wraps `sql.ErrNoRows` as "invalid credentials"; UX must not undo that).

### 2. Projects list
- **File:** `frontend/src/pages/ProjectsPage.tsx`
- **Exists:** loading; populated list; project create; project delete (with ConfirmDialog at `ProjectsPage.tsx:388-404`).
- **Gaps:** no empty state ("no projects yet — create your first") with explicit affordance; no denied state (what if API key scope hides the list?); no error state distinct from "empty".
- **Security flags:** delete confirms with a name-typing requirement? Verify; current ConfirmDialog may be just a button click.

### 3. Project detail / Secrets panel
- **File:** `frontend/src/components/SecretsPanel.tsx`
- **Exists:** loading; populated; masked (••••••••) by default; reveal-on-click; edit-inline; delete (with `window.confirm`); copy-to-clipboard with 2s highlight.
- **Gaps (highest priority):**
  - **No auto-hide timer on revealed secret** (`SecretsPanel.tsx:72`). Required by `docs/EMBED_STATE.md` for the widget; the dashboard panel should match.
  - **Edit state shows plaintext in `<input>`** without confirmation. Spec needs: visual indicator that we're in edit mode + confirm before save.
  - **`window.confirm` for delete** is the browser native dialog (`SecretsPanel.tsx:131`). Users learn to dismiss it. Replace with named-confirmation modal (must type the secret key to confirm).
  - **No error state** for delete failure other than a toast.
  - **No denied state** for read-only access (e.g., when API key has only `read` scope).
- **Security flags:** plaintext reveal + edit + copy are all sensitive paths. **Mandatory Security Engineer review** for the redesign.

### 4. Environment switcher
- **File:** inside `SecretsPanel.tsx` and `ProjectDetailPage.tsx`
- **Exists:** tab UI for alpha/uat/prod.
- **Gaps:** no visual distinction between environments (PROD should look different from alpha — color, label, icon — to reduce "I thought I was in alpha" errors); switching environment clears `revealed` set already (`widget.ts:268-274`) — confirm dashboard does the same.
- **Security flags:** PROD tab is the riskiest UI surface. Treat the color / typography / micro-confirmations as a security feature.

### 5. Promotion wizard
- **File:** `frontend/src/components/PromotionsList.tsx` (and related promotion flow components)
- **Exists:** list of promotions with statuses (`pending`, `completed`, `rolled-back`); approve / reject buttons; rollback button.
- **Gaps:**
  - **No diff preview before approval.** The user clicks "approve" without seeing what's in the promotion. This is a security-critical UX gap.
  - **Approve / reject buttons have no confirmation** (`PromotionsList.tsx:66-74, 76-84`). Approve is a destructive action (causes PROD writes); add a name-typing confirm.
  - **Rollback uses `window.confirm`** (`:87`). Same problem as delete — replace with named modal.
  - **No "approved by" or "rolled-back by" attribution displayed** — users can't see who acted.
- **Security flags:** entire flow. Designer must collaborate with Backend to define what a "promotion preview" looks like (which keys change, which add, which delete — never values).

### 6. API keys panel
- **File:** `frontend/src/components/ProjectAPIKeysPanel.tsx`
- **Exists:** create new key; new raw key displayed once in success card with copy button (2s highlight at `:115`); delete with `window.confirm` (`:97`).
- **Gaps:**
  - **New raw key never auto-clears** from the UI. If user navigates away and comes back, the card is gone — but until then the credential sits in the DOM. Add a "dismiss" button + auto-clear after 60s.
  - **No "I have copied this" checkbox** before letting the user proceed. Today they can dismiss the card without copying, losing the credential.
  - **Delete `window.confirm`** — replace with named modal that requires typing the key label.
- **Security flags:** the raw key shown is a credential. Treat like the revealed-secret state.

### 7. Help / docs link page
- **File:** `frontend/src/pages/HelpPage.tsx`
- **Exists:** static content.
- **Gaps:** **bug** — reads `localStorage.getItem('jwt')` while the rest of the app uses `localStorage.getItem('keepsave_token')`. Likely returns null in production. Fix in a separate PR (out of UX scope but worth flagging).
- **Security flags:** none (read-only).

### 8. Settings / profile
- **File:** TBD (probably exists; needs explicit inventory)
- **Status:** **not audited yet.** Designer interim should add a row when this screen is opened for redesign.

### 9. 401 / 403 / 404 error pages
- **File:** TBD
- **Status:** unclear whether dedicated error-state pages exist. Verify. If we lean on default error toasts everywhere, design real 401/403/404 components with the standard "what to do next" affordance.

## Cross-cutting UX rules (the contract)

These apply to every screen above.

1. **Plaintext secrets MUST auto-hide.** No "revealed forever" state.
2. **Destructive actions MUST require typed confirmation** (project / secret / API key delete; promotion approve to PROD; rollback). `window.confirm` is forbidden because users learn to dismiss it.
3. **Error messages MUST NOT echo internal state** (DB error text, crypto error text, JWT internals). Follow `docs/ERROR_HANDLING_STANDARD.md` — UI shows the `code` and `message` only.
4. **Loading states MUST be present** on every async operation. Spinner < 200ms is unnecessary; > 200ms is required.
5. **Empty states MUST tell the user what to do next**, not just say "nothing here".
6. **Denied states MUST be distinct from empty**. "You can't see this" is different from "there is nothing".
7. **PROD environment MUST be visually distinct** from non-PROD. Color, label, icon — not just the word.
8. **Copy-to-clipboard MUST give visual feedback** (already done at 2s highlight; keep that).
9. **Page-visibility-hidden MUST hide revealed secrets immediately.** No "I tabbed away with the secret still on screen".

## Process

This file is the working spec. Each row's gaps become an issue with label `ux-30day`. Frontend Engineer + interim Designer (Tech Lead) pair on prioritization; Security Engineer review mandatory for #3, #5, #6.

Once a real UX Designer is hired, this doc transitions to a design-spec format with linked Figma frames.

## References

- `docs/EMBED_STATE.md` (the widget's parallel rules)
- `docs/ERROR_HANDLING_STANDARD.md` (error message constraints)
- `docs/THREAT_MODEL.md` v1.2.0 §4 (embed widget findings)

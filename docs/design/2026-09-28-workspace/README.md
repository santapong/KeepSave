# Event Horizon workspace and orbit-vault icon

2026-09-28. Owner request: update the icon and redesign the workspace now. This extends the selected Event Horizon direction; no renewed direction-selection gate is needed. DravenIQ provides product-profile restraint and hierarchy, while KeepSave keeps its own amber/violet identity, Geist typography and name.

Type-2 frontend presentation/navigation change, implemented inline in the existing isolated worktree. No new backend/auth/crypto/promotion behavior or trust boundary. Preserve the earlier social-login work and the accepted public page structure. The shared icon changes across public/auth/workspace surfaces under the explicit icon-update request.

Design: a clearer orbital vault mark, flat near-black and warm-white themes, grouped sidebar navigation, functional collapse/mobile drawer, stable command search, focused project directory with grid/list and sorting, real project counts, contextual creation/import actions, and a compact Store → Connect → Promote explanation. Carry the surfaces through project details, environment controls, dialogs and errors. Do not invent activity, security scores, customers or secret counts. Existing backend projects supply all displayed project data.

References: reuse the captured Event Horizon landing/login direction and the prior product research recorded in `../2026-09-28-event-horizon/README.md`. Apply the established principles of strong primary actions, restrained chrome and concrete workflow explanation, not a fresh competitor clone. The former icon is retained here as `previous-icon.svg`; the new editable SVG is `frontend/public/keepsave.svg`.

Validation plan: existing frontend tests/lint/app and widget builds; focused filter/view/sort and navigation checks; actual isolated local account with clearly labeled sample projects to inspect populated and empty states; keyboard, mobile drawer, light/dark and creation/detail flows; responsive reflow. Keep real secrets masked. No production data, external provider setup, deployment, commit, merge or push is part of this request.

Implemented and locally checked: see `../../validation/2026-09-28-workspace/README.md` for the exact checks, sample-data scope, artifacts and limits. New-project actions remain in the page header and empty state; the redundant creation tile was removed so populated cards stay compact.

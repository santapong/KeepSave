# KeepSave Orbit icon family

Implemented locally on 2026-09-28 in `KeepSave-landing`, continuing the accepted Event Horizon direction. The owner requested black-hole/singularity inspiration across frontend icons, while keeping their meanings easy to understand and retaining KeepSave's identity.

## References and decisions

- [DravenIQ theme skill](/home/santapong/.codex/skills/draveniq-theme/SKILL.md): adopted the 24-unit grid, 1.75-unit rounded strokes, accessible labels and restrained repetition. KeepSave retains its own orbital vault mark and violet/amber palette; no DravenIQ logo substitution.
- [NASA: Anatomy of a Black Hole](https://science.nasa.gov/universe/black-holes/anatomy/), consulted 2026-09-28: orbital apertures, a dark central core and curved light paths informed product symbols. These are stylized brand details, not physical representations of a singularity.
- [Carbon: Icon usage](https://carbondesignsystem.com/elements/icons/usage/), consulted 2026-09-28: recognizable silhouettes, consistent sizing, alignment and readable contrast informed the controls. NASA imagery and Carbon icon assets were not copied into the SVG family.

## Implementation

`frontend/src/components/icons/artwork.ts` contains original editable SVG geometry for 91 named icons. `index.tsx` supplies a shared React adapter with currentColor, className, size, stroke width, ref and title support. All former Lucide imports in `frontend/src` now use this family. The existing package dependency is retained; the frontend source no longer imports it.

Product objects carry the strongest identity: folders and application windows contain an orbital aperture, keys use a compact core, MCP Hub uses connected orbital nodes, and agents keep a familiar robot silhouette. Utilities such as search, arrows, add, close, delete, copy and settings stay recognizable at small sizes. Success, warning and destructive icons inherit their status color. Navigation and project objects may use an amber secondary stroke, with a currentColor fallback in forced-colors mode.

The family covers landing, authentication, workspace navigation, project controls, connection pages, dashboard panels, dialogs, selectors and notifications. Text-glyph action arrows were also replaced. The generic application placeholder now uses the KeepSave application symbol. Uploaded application images and user-chosen emoji are preserved; GitHub and Google retain their provider marks. The legacy `KsMark` delegates to the canonical `EhMark` so brand surfaces share one logo.

Icons accompanying text are decorative by default. Standalone meaningful icons accept `title` or an ARIA label. Icon-only controls still require a meaningful name on the control. Icons add no animation, remote asset request or font dependency.

## Review

- [Standalone gallery](catalog.html): all 91 named exports at 16 and 32 pixels, with a theme switch. The gallery illustrates optional amber accents on product symbols; runtime color follows each surface.
- Development preview: `http://127.0.0.1:4651/icon-preview.html`. This HTML review entry is not included in the configured application production build.
- [Verification record](../../validation/2026-09-28-light-icons/README.md).

Future icons should preserve the familiar object/action first, use sparse orbital details only when legible, and be checked at actual UI sizes in both themes. Some named exports intentionally share artwork (`Key`/`KeyRound`, `Lock`/`LockKeyhole`, `CheckCircle`/`CheckCircle2`).

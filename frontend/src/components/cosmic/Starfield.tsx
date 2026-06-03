/**
 * Event Horizon — fixed, behind-everything starfield + nebula backdrop.
 *
 * Mount once near the root of a screen. Pure CSS (`.cz-cosmos` in
 * styles/cosmic.css): drifts on a slow loop, fades to a soft gradient in
 * the light theme, and is disabled under `prefers-reduced-motion` and
 * `data-motion="off"`.
 */
export function Starfield() {
  return <div className="cz-cosmos" aria-hidden="true" />;
}

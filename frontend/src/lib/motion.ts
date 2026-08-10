/**
 * Single source of truth for "should this screen animate?".
 *
 * Honours both the OS-level `prefers-reduced-motion` setting and the app's
 * own `data-motion="off"` switch on <html>.
 *
 * `matchMedia` is feature-detected rather than assumed: it is absent in
 * jsdom (so any component using it blows up under test) and in a few
 * embedded webviews. Missing support is treated as "animate", since the
 * media query is an opt-out, not an opt-in.
 */
export function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || typeof document === 'undefined') return true;
  if (document.documentElement.dataset.motion === 'off') return true;
  if (typeof window.matchMedia !== 'function') return false;
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

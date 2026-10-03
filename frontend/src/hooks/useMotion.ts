import { useSyncExternalStore } from 'react';

const EVENT = 'keepsave-motion-change';
const QUERY = '(prefers-reduced-motion: reduce)';

function subscribe(callback: () => void) {
  const media = window.matchMedia?.(QUERY);
  media?.addEventListener('change', callback);
  window.addEventListener(EVENT, callback);
  window.addEventListener('storage', callback);
  const observer = new MutationObserver(callback);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-motion'] });
  return () => {
    media?.removeEventListener('change', callback);
    window.removeEventListener(EVENT, callback);
    window.removeEventListener('storage', callback);
    observer.disconnect();
  };
}

function snapshot() {
  let paused = document.documentElement.dataset.motion === 'off';
  try { paused ||= localStorage.getItem('keepsave_motion') === 'off'; } catch { /* Storage is optional. */ }
  return paused || !!window.matchMedia?.(QUERY).matches;
}

export function useMotion() {
  const paused = useSyncExternalStore(subscribe, snapshot, () => true);
  const reduced = !!window.matchMedia?.(QUERY).matches;
  function toggle() {
    const value = paused ? 'on' : 'off';
    document.documentElement.dataset.motion = value;
    try { localStorage.setItem('keepsave_motion', value); } catch { /* Session preference still works. */ }
    window.dispatchEvent(new Event(EVENT));
  }
  return { paused, reduced, toggle };
}

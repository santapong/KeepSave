import { useEffect, useSyncExternalStore } from 'react';

type Theme = 'light' | 'dark';
const STORAGE_KEY = 'keepsave_theme';
const CHANGE_EVENT = 'keepsave-theme-change';

function readTheme(): Theme {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved === 'light' || saved === 'dark') return saved;
  } catch { /* The current page still supports themes when storage is unavailable. */ }
  return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark';
}

function subscribe(listener: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === STORAGE_KEY || event.key === null) listener();
  };
  window.addEventListener(CHANGE_EVENT, listener);
  window.addEventListener('storage', onStorage);
  return () => {
    window.removeEventListener(CHANGE_EVENT, listener);
    window.removeEventListener('storage', onStorage);
  };
}

function toggle() {
  const next = readTheme() === 'dark' ? 'light' : 'dark';
  document.documentElement.dataset.theme = next;
  try { localStorage.setItem(STORAGE_KEY, next); } catch { /* Keep the in-page preference. */ }
  window.dispatchEvent(new Event(CHANGE_EVENT));
}

export function useTheme() {
  const theme = useSyncExternalStore(subscribe, readTheme, () => 'dark' as const);
  useEffect(() => { document.documentElement.dataset.theme = theme; }, [theme]);
  return { theme, toggle };
}

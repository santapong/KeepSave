import { useState, useCallback } from 'react';

export function useSidebar() {
  const [collapsed, setCollapsed] = useState(() =>
    localStorage.getItem('keepsave_sidebar_collapsed') === 'true'
  );

  const toggle = useCallback(() => {
    setCollapsed(prev => {
      const next = !prev;
      localStorage.setItem('keepsave_sidebar_collapsed', String(next));
      return next;
    });
  }, []);

  return { collapsed, toggle };
}

import { cn } from '@/lib/utils';

interface EhMarkProps {
  className?: string;
}

/**
 * EhMark — a 26px mini event horizon used as the KeepSave logo mark in
 * the sidebar wordmark and login card. Pure CSS (`.cz-eh-mark`).
 */
export function EhMark({ className }: EhMarkProps) {
  return <span className={cn('cz-eh-mark', className)} aria-hidden="true" />;
}

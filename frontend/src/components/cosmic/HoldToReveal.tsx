import { useRef, useState, useEffect, type PointerEvent, type KeyboardEvent } from 'react';
import { cn } from '@/lib/utils';

interface HoldToRevealProps {
  /** The secret value shown once revealed. */
  value: string;
  /** Number of placeholder bricks shown while hidden. */
  bricks?: number;
  /** Hold duration before the value is revealed, in ms. */
  holdMs?: number;
  className?: string;
}

type State = 'idle' | 'holding' | 'open';

/**
 * HoldToReveal — a secret value rendered as brick blocks. Press and
 * hold (pointer or Enter/Space) starts a timer; the bricks glow and a
 * "decrypting" scan shimmer runs; on timeout the value is revealed.
 * Releasing or leaving before the timeout cancels. Pressing again while
 * open hides it. The value is never revealed on hover.
 */
export function HoldToReveal({ value, bricks = 12, holdMs = 720, className }: HoldToRevealProps) {
  const [state, setState] = useState<State>('idle');
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timer.current) clearTimeout(timer.current);
    };
  }, []);

  function begin() {
    if (state === 'open') {
      setState('idle');
      return;
    }
    if (state === 'holding') return; // ignore key auto-repeat
    setState('holding');
    timer.current = setTimeout(() => setState('open'), holdMs);
  }

  function cancel() {
    if (timer.current) clearTimeout(timer.current);
    setState((s) => (s === 'holding' ? 'idle' : s));
  }

  function onPointerDown(e: PointerEvent<HTMLSpanElement>) {
    e.preventDefault();
    begin();
  }

  function onKeyDown(e: KeyboardEvent<HTMLSpanElement>) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      begin();
    }
  }

  function onKeyUp(e: KeyboardEvent<HTMLSpanElement>) {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      cancel();
    }
  }

  const hint = state === 'open' ? 'hide' : state === 'holding' ? 'decrypting…' : 'hold to reveal';

  return (
    <span
      className={cn('cz-reveal', state === 'holding' && 'cz-holding', state === 'open' && 'cz-open', className)}
      onPointerDown={onPointerDown}
      onPointerUp={cancel}
      onPointerLeave={cancel}
      onKeyDown={onKeyDown}
      onKeyUp={onKeyUp}
      onBlur={cancel}
      role="button"
      tabIndex={0}
      aria-label={state === 'open' ? 'Hide secret value' : 'Press and hold to reveal secret value'}
      title={state === 'open' ? 'Press to hide' : 'Press and hold to reveal'}
    >
      <span className="cz-bricks">
        {state === 'open' ? (
          <span className="cz-val">{value}</span>
        ) : (
          Array.from({ length: bricks }).map((_, i) => <span key={i} className="cz-brick" />)
        )}
      </span>
      <span className="cz-hint">{hint}</span>
    </span>
  );
}

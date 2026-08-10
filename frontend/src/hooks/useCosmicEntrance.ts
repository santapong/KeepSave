import { useLayoutEffect, type RefObject } from 'react';
import { animate, stagger } from 'animejs';
import { prefersReducedMotion as motionOff } from '../lib/motion';


/**
 * Staggered entrance for the cosmic auth screens, driven by anime.js.
 *
 * The hidden start state is applied here in JS rather than in CSS, and
 * deliberately so: if anime.js ever fails to load, or this effect never
 * runs, nothing was hidden in the first place and the screen degrades to
 * a plain static render instead of a blank one. A CSS `opacity: 0` rule
 * would fail the other way.
 *
 * useLayoutEffect, not useEffect, so the elements are hidden before the
 * browser paints — otherwise the content flashes in and then hides.
 *
 * @param ref      container to search within
 * @param selector elements to reveal, animated in DOM order
 */
export function useCosmicEntrance(ref: RefObject<HTMLElement | null>, selector: string) {
  useLayoutEffect(() => {
    const root = ref.current;
    if (!root) return;
    if (motionOff()) return;

    const els = Array.from(root.querySelectorAll<HTMLElement>(selector));
    if (!els.length) return;

    // Hide up front to avoid a flash-then-hide on first paint.
    for (const el of els) el.style.opacity = '0';

    const clear = () => {
      for (const el of els) {
        el.style.opacity = '';
        el.style.transform = '';
        el.style.filter = '';
      }
    };

    const DURATION = 900;
    const STEP = 70;
    const START = 120;

    const animation = animate(els, {
      opacity: [0, 1],
      translateY: [18, 0],
      filter: ['blur(6px)', 'blur(0px)'],
      duration: DURATION,
      delay: stagger(STEP, { start: START }),
      ease: 'out(3)',
      onComplete: clear,
    });

    // Safety net. anime.js v4 drives these through the Web Animations API,
    // which *composites over* the inline style rather than replacing it —
    // so the element's resting value underneath is still opacity:0. If the
    // WAAPI animation is ever dropped (finished and collected, a re-render,
    // an interrupted transition) the content would silently vanish. Clearing
    // on a timer as well guarantees the resting state is visible even if
    // onComplete never fires.
    const settle = window.setTimeout(clear, START + STEP * els.length + DURATION + 250);

    return () => {
      window.clearTimeout(settle);
      animation?.pause?.();
      clear();
    };
  }, [ref, selector]);
}

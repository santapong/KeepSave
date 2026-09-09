/** One lightweight vector asset for the favicon, wordmark, and product preview. */
export function EhMark({ size = 32, className = '' }: { size?: number; className?: string }) {
  return <img src="/keepsave.svg?v=event-horizon" width={size} height={size}
    className={`cz-brand-icon ${className}`} alt="" aria-hidden="true" draggable={false} />;
}

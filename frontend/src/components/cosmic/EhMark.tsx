/** Field Twist's open core, in Gargantua champagne/copper or contrasting bronze. */
export function EhMark({ size = 32, className = '' }: { size?: number; className?: string }) {
  return <span className={`cz-brand-icon ${className}`} style={{ width: size, height: size }} aria-hidden="true">
    <img className="cz-mark-dark" src="/keepsave.svg?v=gargantua-field-twist-v3" width={size} height={size}
      alt="" draggable={false} />
    <img className="cz-mark-light" src="/keepsave-light.svg?v=gargantua-field-twist-v3" width={size} height={size}
      alt="" draggable={false} />
  </span>;
}

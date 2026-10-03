import { EhMark } from './EhMark';

/** Legacy component name delegates to the canonical KeepSave mark. */
export function KsMark({ size = 26, className }: { size?: number; className?: string }) {
  return <EhMark size={size} className={className} />;
}

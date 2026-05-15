import { useState, useEffect, useRef } from 'react';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';

interface TypedConfirmModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  description: string;
  /**
   * The exact string the user must type to enable the confirm button.
   * Comparison is case-sensitive.
   */
  confirmPhrase: string;
  /** Label of the destructive button. Defaults to "Confirm". */
  confirmLabel?: string;
  /** Whether the confirm button should use destructive styling. Defaults to true. */
  destructive?: boolean;
  /** Optional hint shown above the input describing what to type. */
  inputHint?: string;
  /** Called when the user confirms with a matching phrase. */
  onConfirm: () => void;
}

/**
 * Typed-confirmation modal for destructive actions.
 *
 * Replaces `window.confirm` for actions that cannot be undone (e.g. project /
 * secret / API-key deletion, rotate-all). The user must type a specific phrase
 * (often the resource name) before the confirm button enables; this prevents
 * "muscle memory" confirmation of irreversible operations.
 *
 * Closes FU 0j (docs/FOLLOWUPS.md).
 */
export function TypedConfirmModal({
  open,
  onOpenChange,
  title,
  description,
  confirmPhrase,
  confirmLabel = 'Confirm',
  destructive = true,
  inputHint,
  onConfirm,
}: TypedConfirmModalProps) {
  const [typed, setTyped] = useState('');
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Reset the typed phrase whenever the modal is reopened.
  useEffect(() => {
    if (open) {
      setTyped('');
      // Defer focus to next tick so Radix has mounted the input.
      setTimeout(() => inputRef.current?.focus(), 30);
    }
  }, [open]);

  const matches = typed === confirmPhrase;

  function handleSubmit() {
    if (!matches) return;
    onConfirm();
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <div className="space-y-2">
          <Label className="text-xs">
            {inputHint ?? (
              <>
                Type <code className="px-1 py-0.5 rounded bg-muted font-mono text-xs">{confirmPhrase}</code>{' '}
                to confirm
              </>
            )}
          </Label>
          <Input
            ref={inputRef}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && matches) {
                e.preventDefault();
                handleSubmit();
              }
            }}
            placeholder={confirmPhrase}
            autoComplete="off"
            spellCheck={false}
            className="font-mono"
          />
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant={destructive ? 'destructive' : 'default'}
            disabled={!matches}
            onClick={handleSubmit}
          >
            {confirmLabel}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

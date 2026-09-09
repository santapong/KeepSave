import { useState } from 'react';
import { MessageSquarePlus } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from '@/components/ui/dialog';
import { submitFeedback, FeedbackError, type FeedbackCategory } from '@/api/feedback';

const CATEGORIES: { value: FeedbackCategory; label: string }[] = [
  { value: 'bug', label: 'Bug' },
  { value: 'idea', label: 'Idea' },
  { value: 'other', label: 'Other' },
];

const MAX_MESSAGE = 4000;

// Floating feedback launcher, fixed at the bottom-right on every authenticated
// screen. Opens a small dialog with a 3-way category selector and a textarea;
// submissions are filed as GitHub issues by the backend (feature-flagged).
export function FeedbackButton() {
  const [open, setOpen] = useState(false);
  const [category, setCategory] = useState<FeedbackCategory>('bug');
  const [message, setMessage] = useState('');
  const [pending, setPending] = useState(false);
  const { toast } = useToast();

  const reset = () => {
    setCategory('bug');
    setMessage('');
  };

  const handleSubmit = async () => {
    const trimmed = message.trim();
    if (!trimmed || pending) return;
    setPending(true);
    try {
      await submitFeedback({ category, message: trimmed, page_url: window.location.href });
      toast({
        variant: 'success',
        title: 'Thanks for the feedback!',
        description: 'We filed it for the team.',
      });
      setOpen(false);
      reset();
    } catch (err) {
      const status = err instanceof FeedbackError ? err.status : 0;
      if (status === 503) {
        toast({ variant: 'destructive', title: "Feedback isn't configured on this server" });
      } else if (status === 429) {
        toast({ variant: 'destructive', title: 'Too many submissions — try again in a minute' });
      } else {
        toast({
          variant: 'destructive',
          title: 'Could not send feedback',
          description: 'Please try again.',
        });
      }
    } finally {
      setPending(false);
    }
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label="Give feedback"
        title="Give feedback"
        className="fixed bottom-6 right-6 z-[190] w-12 h-12 rounded-full bg-primary text-primary-foreground border-none cursor-pointer flex items-center justify-center shadow-lg hover:bg-primary/90 transition-colors"
      >
        <MessageSquarePlus className="h-5 w-5" />
      </button>

      <Dialog
        open={open}
        onOpenChange={(o) => {
          setOpen(o);
          if (!o) reset();
        }}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Send feedback</DialogTitle>
            <DialogDescription>
              Tell us what&apos;s working or what&apos;s broken. This opens an issue for the team.
            </DialogDescription>
          </DialogHeader>

          <div className="flex gap-2">
            {CATEGORIES.map((c) => (
              <button
                key={c.value}
                type="button"
                onClick={() => setCategory(c.value)}
                aria-pressed={category === c.value}
                className={cn(
                  'flex-1 rounded-md border px-3 py-2 text-sm transition-colors',
                  category === c.value
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'border-input bg-background hover:bg-accent hover:text-accent-foreground'
                )}
              >
                {c.label}
              </button>
            ))}
          </div>

          <div>
            <Textarea
              value={message}
              onChange={(e) => setMessage(e.target.value.slice(0, MAX_MESSAGE))}
              maxLength={MAX_MESSAGE}
              rows={5}
              placeholder="Describe your feedback…"
              aria-label="Feedback message"
            />
            <div className="mt-1 text-right text-xs text-muted-foreground">
              {message.length}/{MAX_MESSAGE}
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={() => setOpen(false)} disabled={pending}>
              Cancel
            </Button>
            <Button onClick={handleSubmit} disabled={pending || !message.trim()}>
              {pending ? 'Sending…' : 'Send feedback'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}

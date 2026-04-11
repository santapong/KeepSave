import { useState } from 'react';
import { promoteDiff, promote } from '../api/client';
import type { DiffEntry } from '../types';
import { cn } from '@/lib/utils';
import { useToast } from '@/hooks/useToast';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/ui/select';
import { CheckCircle2, AlertTriangle } from 'lucide-react';

interface PromotionWizardProps {
  projectId: string;
}

type Step = 'configure' | 'review' | 'done';

const PATHS = [
  { source: 'alpha', target: 'uat', label: 'Alpha \u2192 UAT' },
  { source: 'uat', target: 'prod', label: 'UAT \u2192 PROD' },
];

const STEPS: { key: Step; label: string }[] = [
  { key: 'configure', label: 'Configure' },
  { key: 'review', label: 'Review' },
  { key: 'done', label: 'Done' },
];

function stepIndex(step: Step): number {
  return STEPS.findIndex((s) => s.key === step);
}

export function PromotionWizard({ projectId }: PromotionWizardProps) {
  const [step, setStep] = useState<Step>('configure');
  const [pathIdx, setPathIdx] = useState(0);
  const [overridePolicy, setOverridePolicy] = useState('skip');
  const [notes, setNotes] = useState('');
  const [diff, setDiff] = useState<DiffEntry[]>([]);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<string>('');
  const { toast } = useToast();

  const path = PATHS[pathIdx];
  const currentStepIdx = stepIndex(step);

  function resetWizard() {
    setStep('configure');
    setResult('');
    setDiff([]);
    setSelectedKeys(new Set());
    setNotes('');
    setError('');
  }

  async function handlePreview() {
    setLoading(true);
    setError('');
    try {
      const entries = await promoteDiff(projectId, path.source, path.target);
      setDiff(entries);
      setSelectedKeys(new Set(entries.filter((e) => e.action !== 'no_change').map((e) => e.key)));
      setStep('review');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load diff');
    } finally {
      setLoading(false);
    }
  }

  async function handlePromote() {
    setLoading(true);
    setError('');
    try {
      const keys = selectedKeys.size > 0 ? Array.from(selectedKeys) : undefined;
      const promotion = await promote(
        projectId,
        path.source,
        path.target,
        overridePolicy,
        keys,
        notes || undefined
      );
      const msg =
        promotion.status === 'pending'
          ? 'Promotion request created. PROD promotions require approval.'
          : 'Promotion completed successfully!';
      setResult(msg);
      setStep('done');
      toast({ title: 'Promotion', description: msg });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Promotion failed');
    } finally {
      setLoading(false);
    }
  }

  function toggleKey(key: string) {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  /* ---------- Step Indicator ---------- */
  function renderStepIndicator() {
    return (
      <div className="flex items-start justify-center mb-7">
        {STEPS.map((s, idx) => {
          const isCompleted = idx < currentStepIdx;
          const isCurrent = idx === currentStepIdx;
          return (
            <div key={s.key} className="flex items-center">
              {idx > 0 && (
                <div
                  className={cn(
                    'w-12 h-0.5 mb-[18px]',
                    idx <= currentStepIdx ? 'bg-primary' : 'bg-border'
                  )}
                />
              )}
              <div className="flex flex-col items-center min-w-[64px]">
                <div
                  className={cn(
                    'w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold shrink-0',
                    isCompleted && 'bg-primary text-white',
                    isCurrent && 'bg-transparent border-2 border-primary text-primary',
                    !isCompleted && !isCurrent && 'bg-border text-muted-foreground'
                  )}
                >
                  {isCompleted ? '\u2713' : idx + 1}
                </div>
                <span
                  className={cn(
                    'text-[11px] font-semibold mt-1.5',
                    isCompleted || isCurrent ? 'text-foreground' : 'text-muted-foreground'
                  )}
                >
                  {s.label}
                </span>
              </div>
            </div>
          );
        })}
      </div>
    );
  }

  /* ---------- Done Step ---------- */
  if (step === 'done') {
    return (
      <Card>
        <CardContent className="p-6">
          {renderStepIndicator()}
          <div className="text-center py-6">
            <div className="w-12 h-12 rounded-full bg-green-500/15 flex items-center justify-center mx-auto mb-4">
              <CheckCircle2 className="h-6 w-6 text-green-500" />
            </div>
            <p className="text-sm font-semibold text-muted-foreground mb-1.5">
              {path.source.toUpperCase()} &rarr; {path.target.toUpperCase()}
            </p>
            <h3 className="text-lg text-green-500 font-semibold mb-5">
              {result}
            </h3>
            <Button onClick={resetWizard}>
              Start New Promotion
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  /* ---------- Configure & Review ---------- */
  return (
    <div>
      {error && (
        <div className="bg-destructive/10 text-destructive rounded-md px-3 py-2 text-sm mb-4">
          {error}
        </div>
      )}

      {step === 'configure' && (
        <Card>
          <CardContent className="p-6">
            {renderStepIndicator()}
            <h3 className="text-base font-semibold mb-4">Configure Promotion</h3>

            <div className="mb-4">
              <Label className="block mb-1.5">Promotion Path</Label>
              <div className="flex gap-2">
                {PATHS.map((p, idx) => (
                  <Button
                    key={idx}
                    onClick={() => setPathIdx(idx)}
                    variant={pathIdx === idx ? 'default' : 'outline'}
                    size="sm"
                    className="font-semibold"
                  >
                    {p.label}
                  </Button>
                ))}
              </div>
            </div>

            {pathIdx === 1 && (
              <div className="bg-amber-500/10 border border-amber-500/30 rounded-md px-3.5 py-2.5 text-sm text-amber-500 mb-4 flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                PROD promotions require multi-party approval before secrets are applied.
              </div>
            )}

            <div className="mb-4">
              <Label className="block mb-1.5">Override Policy</Label>
              <Select value={overridePolicy} onValueChange={setOverridePolicy}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="skip">Skip - Don&apos;t overwrite existing keys</SelectItem>
                  <SelectItem value="overwrite">Overwrite - Replace all values</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <div className="mb-4">
              <Label className="block mb-1.5">Notes (optional)</Label>
              <Textarea
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="Reason for promotion..."
                className="min-h-[60px] resize-y"
              />
            </div>

            <Button onClick={handlePreview} disabled={loading}>
              {loading ? 'Loading diff...' : 'Preview Changes'}
            </Button>
          </CardContent>
        </Card>
      )}

      {step === 'review' && (
        <Card>
          <CardContent className="p-6">
            {renderStepIndicator()}
            <h3 className="text-base font-semibold mb-1">
              Review Changes: {path.label}
            </h3>
            <p className="text-sm text-muted-foreground mb-4">
              Select which keys to promote. Override policy: <strong>{overridePolicy}</strong>
            </p>

            {pathIdx === 1 && (
              <div className="bg-amber-500/10 border border-amber-500/30 rounded-md px-3.5 py-2.5 text-sm text-amber-500 mb-4 flex items-center gap-2">
                <AlertTriangle className="h-4 w-4 shrink-0" />
                PROD promotions require multi-party approval before secrets are applied.
              </div>
            )}

            {diff.length === 0 ? (
              <p className="text-muted-foreground">No differences found.</p>
            ) : (
              <Card>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead className="w-10"></TableHead>
                      <TableHead>Key</TableHead>
                      <TableHead>Action</TableHead>
                      <TableHead>Source</TableHead>
                      <TableHead>Target</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diff.map((entry) => (
                      <TableRow
                        key={entry.key}
                        className={cn(
                          entry.action === 'no_change' && 'opacity-50',
                          entry.action === 'add' && 'bg-green-500/5',
                          entry.action === 'update' && 'bg-amber-500/5'
                        )}
                      >
                        <TableCell>
                          <input
                            type="checkbox"
                            checked={selectedKeys.has(entry.key)}
                            onChange={() => toggleKey(entry.key)}
                            disabled={entry.action === 'no_change'}
                            className="rounded"
                          />
                        </TableCell>
                        <TableCell>
                          <code className="text-sm font-semibold">{entry.key}</code>
                        </TableCell>
                        <TableCell>
                          <Badge
                            variant="secondary"
                            className={cn(
                              'text-[11px] uppercase font-semibold',
                              entry.action === 'add' && 'bg-green-500/15 text-green-500',
                              entry.action === 'update' && 'bg-amber-500/15 text-amber-500',
                              entry.action === 'no_change' && 'bg-muted text-muted-foreground'
                            )}
                          >
                            {entry.action}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <code className="text-xs">{entry.source_exists ? '\u2022\u2022\u2022\u2022\u2022\u2022' : '-'}</code>
                        </TableCell>
                        <TableCell>
                          <code className="text-xs">{entry.target_exists ? '\u2022\u2022\u2022\u2022\u2022\u2022' : '-'}</code>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Card>
            )}

            <div className="flex gap-2 mt-4">
              <Button variant="outline" onClick={() => setStep('configure')}>
                Back
              </Button>
              <Button
                onClick={handlePromote}
                disabled={loading || selectedKeys.size === 0}
                className={cn(
                  path.target === 'prod' && 'bg-amber-500 hover:bg-amber-600'
                )}
              >
                {loading
                  ? 'Promoting...'
                  : path.target === 'prod'
                    ? 'Request PROD Promotion'
                    : `Promote to ${path.target.toUpperCase()}`}
              </Button>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

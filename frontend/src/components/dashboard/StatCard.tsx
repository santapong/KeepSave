import type { ReactNode } from 'react';
import { TrendingUp, TrendingDown } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

interface StatCardProps {
  label: string;
  value: string | number;
  icon?: ReactNode;
  trend?: { value: number; positive: boolean };
  color?: string;
  className?: string;
}

export function StatCard({
  label,
  value,
  icon,
  trend,
  color,
  className,
}: StatCardProps) {
  return (
    <Card className={cn('overflow-hidden', className)}>
      <CardContent className="p-4">
        <div className="flex items-start justify-between">
          {icon && (
            <div className="text-muted-foreground">{icon}</div>
          )}
          {trend && (
            <div
              className={cn(
                'flex items-center gap-1 text-xs font-medium',
                trend.positive ? 'text-green-600' : 'text-red-600',
              )}
            >
              {trend.positive ? (
                <TrendingUp className="h-3 w-3" />
              ) : (
                <TrendingDown className="h-3 w-3" />
              )}
              <span>
                {trend.positive ? '+' : ''}
                {trend.value}%
              </span>
            </div>
          )}
        </div>

        <div className="mt-3">
          <p className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            {label}
          </p>
          <p className={cn('mt-1 text-2xl font-bold', color)}>{value}</p>
        </div>
      </CardContent>
    </Card>
  );
}

import { Link, useLocation } from 'react-router-dom';
import { ChevronRight, Home } from 'lucide-react';
import { cn } from '@/lib/utils';

const ROUTE_LABELS: Record<string, string> = {
  '': 'Projects',
  'organizations': 'Organizations',
  'templates': 'Templates',
  'mcp-hub': 'MCP Hub',
  'oauth-clients': 'OAuth',
  'applications': 'Applications',
  'admin': 'Dashboard',
  'help': 'Docs',
  'projects': 'Projects',
  'settings': 'Settings',
};

interface Breadcrumb {
  label: string;
  path: string;
}

export function Breadcrumbs() {
  const location = useLocation();

  const segments = location.pathname.split('/').filter(Boolean);

  const breadcrumbs: Breadcrumb[] = [
    { label: 'Home', path: '/' },
  ];

  let currentPath = '';
  for (const segment of segments) {
    currentPath += `/${segment}`;
    const label = ROUTE_LABELS[segment] || decodeURIComponent(segment);
    breadcrumbs.push({ label, path: currentPath });
  }

  // If we're at root, just show "Home > Projects"
  if (segments.length === 0) {
    breadcrumbs.push({ label: 'Projects', path: '/' });
  }

  return (
    <nav aria-label="Breadcrumb" className="flex items-center gap-1 text-sm text-muted-foreground">
      {breadcrumbs.map((crumb, index) => {
        const isLast = index === breadcrumbs.length - 1;
        const isFirst = index === 0;

        return (
          <span key={crumb.path + index} className="flex items-center gap-1">
            {index > 0 && (
              <ChevronRight className="h-3.5 w-3.5 shrink-0 text-muted-foreground/50" />
            )}
            {isLast ? (
              <span className={cn(
                'font-medium',
                isLast && 'text-foreground'
              )}>
                {isFirst ? (
                  <Home className="h-3.5 w-3.5" />
                ) : (
                  crumb.label
                )}
              </span>
            ) : (
              <Link
                to={crumb.path}
                className="hover:text-foreground transition-colors"
              >
                {isFirst ? (
                  <Home className="h-3.5 w-3.5" />
                ) : (
                  crumb.label
                )}
              </Link>
            )}
          </span>
        );
      })}
    </nav>
  );
}

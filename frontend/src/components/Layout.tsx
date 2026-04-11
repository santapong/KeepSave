import { type ReactNode, useState } from 'react';
import { cn } from '@/lib/utils';
import { Menu, X } from 'lucide-react';
import { useSidebar } from '@/hooks/useSidebar';
import { Sidebar } from './Sidebar';
import { Breadcrumbs } from './Breadcrumbs';
import type { User } from '../types';

interface LayoutProps {
  user: User | null;
  onLogout: () => void;
  children: ReactNode;
}

export function Layout({ user, onLogout, children }: LayoutProps) {
  const { collapsed, toggle } = useSidebar();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <div className="flex h-screen overflow-hidden">
      {/* Mobile sidebar overlay */}
      {mobileOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 sm:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      {/* Sidebar - hidden on mobile unless toggled */}
      <div
        className={cn(
          'sm:relative sm:block',
          mobileOpen
            ? 'fixed inset-y-0 left-0 z-50'
            : 'hidden'
        )}
      >
        <Sidebar
          user={user}
          collapsed={collapsed}
          onToggle={toggle}
          onLogout={onLogout}
        />
      </div>

      {/* Main content area */}
      <div className="flex flex-col flex-1 overflow-hidden">
        {/* Top header bar */}
        <header className="flex items-center justify-between h-14 border-b border-border bg-background px-4 shrink-0">
          <div className="flex items-center gap-3">
            {/* Mobile menu button */}
            <button
              onClick={() => setMobileOpen(prev => !prev)}
              className="flex items-center justify-center h-8 w-8 rounded-md hover:bg-accent text-muted-foreground sm:hidden"
            >
              {mobileOpen ? (
                <X className="h-5 w-5" />
              ) : (
                <Menu className="h-5 w-5" />
              )}
            </button>
            <Breadcrumbs />
          </div>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-y-auto p-6">
          {children}
        </main>
      </div>
    </div>
  );
}

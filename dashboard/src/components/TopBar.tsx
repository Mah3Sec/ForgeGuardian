import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { Bell, Settings, Moon, Sun, ChevronDown, Calendar, Search, FolderOpen } from 'lucide-react';
import { getDashboardStats } from '../lib/api';
import { useUIStore } from '../store/ui';

// Dispatches the same synthetic keydown the global Cmd/Ctrl+K listener in
// App.tsx already handles, rather than duplicating open-state wiring here —
// this button is a discoverable, clickable entry point to that one listener.
function openCommandPalette() {
  const isMac = navigator.platform.toLowerCase().includes('mac');
  window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: isMac, ctrlKey: !isMac }));
}

export function DashboardHeader() {
  const navigate = useUIStore(s => s.navigate);
  const theme = useUIStore(s => s.theme);
  const toggleTheme = useUIStore(s => s.toggleTheme);
  const { data } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: getDashboardStats,
    refetchInterval: 30_000,
    retry: 3,
  });

  type IconBtn = { icon: React.ReactNode; onClick: () => void; title: string };
  const iconBtns: IconBtn[] = [
    {
      icon: theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />,
      onClick: toggleTheme,
      title: theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme',
    },
    { icon: <Bell size={15} />,     onClick: () => navigate('/alerts'),   title: 'Alerts' },
    { icon: <Settings size={15} />, onClick: () => navigate('/settings'), title: 'Settings' },
  ];

  return (
    <div className="flex items-center justify-between gap-3 px-4 sm:px-6 pt-5 pb-4 border-b border-border-color bg-bg-base shrink-0 flex-wrap">
      <div>
        <h1 className="text-[1.4rem] font-bold text-text-primary m-0">
          Dashboard
        </h1>
        <p className="text-[0.78rem] text-text-secondary mt-1 mb-0">
          Overview of your supply chain security posture
        </p>
      </div>

      <div className="flex items-center gap-2.5 flex-wrap">
        {/* Projects — real link to the Projects page, not a fake filter.
            No per-project scoping exists yet server-side; a decorative
            dropdown with hardcoded names that changed nothing used to sit
            here — replaced with an honest link to the real project list. */}
        <button
          onClick={() => navigate('/projects')}
          className="hidden md:flex items-center gap-1.5 bg-surface border border-border-color rounded-md text-text-primary text-[0.78rem] py-1.5 px-2.5 hover:bg-surface-muted"
        >
          <FolderOpen size={13} className="text-text-muted" />
          All Projects
        </button>

        {/* Command palette */}
        <button
          onClick={openCommandPalette}
          title="Open command palette (⌘K)"
          className="hidden sm:flex items-center gap-1.5 bg-surface border border-border-color rounded-md px-2.5 py-1.5 text-[0.78rem] text-text-secondary hover:bg-surface-muted hover:text-text-primary"
        >
          <Search size={13} />
          <span className="text-text-muted font-mono text-[0.68rem]">⌘K</span>
        </button>

        {/* Date display */}
        <div className="hidden sm:flex items-center gap-1.5 bg-surface border border-border-color rounded-md px-2.5 py-1.5 text-[0.78rem] text-text-primary select-none">
          <Calendar size={13} className="text-text-muted" />
          {data?.last_updated
            ? new Date(data.last_updated).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
            : 'Last 7 days'}
          <ChevronDown size={12} className="text-text-muted" />
        </div>

        {/* Icon buttons */}
        {iconBtns.map((btn, i) => (
          <button
            key={i}
            onClick={btn.onClick}
            title={btn.title}
            className="bg-surface border border-border-color rounded-md p-1.5 text-text-secondary flex items-center justify-center hover:bg-surface-muted hover:text-text-primary"
          >
            {btn.icon}
          </button>
        ))}
      </div>
    </div>
  );
}

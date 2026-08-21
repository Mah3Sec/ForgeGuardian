import {
  Shield, Search, RefreshCw, HardDrive, Bot, Activity, GitBranch, Bell,
  FolderOpen, Package, FileText, ListFilter, KeyRound, Download, Webhook,
  GitMerge, LayoutDashboard, ChevronLeft, ChevronRight, FileCheck, PenTool,
  BookOpen, LogOut, Network, Puzzle, Server, Globe2, Building2, Sparkles, Cpu,
  Settings, ClipboardList, Terminal,
} from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useUIStore } from '../store/ui';
import { getAuthStatus } from '../lib/api';
import { cn } from './ui/utils';
import { WorkspaceSwitcher } from './WorkspaceSwitcher';

export interface NavItem {
  label: string;
  icon: React.ElementType;
  path: string;
}

export interface NavSection {
  section: string;
  items: NavItem[];
}

export const STANDALONE: NavItem = { label: 'Dashboard', icon: LayoutDashboard, path: '/dashboard' };

export const NAV_SECTIONS: NavSection[] = [
  {
    section: 'ANALYZE',
    items: [
      { label: 'Scan Now',            icon: Search,       path: '/scan' },
      { label: 'Scan Sessions',       icon: ClipboardList, path: '/sessions' },
      { label: 'Recursive Scanning',  icon: RefreshCw,    path: '/recursive' },
      { label: 'System Audit',        icon: HardDrive,    path: '/audit' },
      { label: 'AI Security',         icon: Bot,          path: '/ai-security' },
      { label: 'AI Advisory',         icon: Sparkles,     path: '/advisory' },
      { label: 'AI Patch Agent',      icon: Cpu,          path: '/agents' },
    ],
  },
  {
    section: 'MONITOR',
    items: [
      { label: 'Live Monitoring',     icon: Activity,     path: '/monitor' },
      { label: 'Log Monitor',         icon: Terminal,     path: '/logs' },
      { label: 'Attack Surface',      icon: Network,      path: '/attack-surface' },
      { label: 'Dependency Drift',    icon: GitBranch,    path: '/drift' },
      { label: 'Alerts',             icon: Bell,          path: '/alerts' },
    ],
  },
  {
    section: 'INVENTORY',
    items: [
      { label: 'Projects',            icon: FolderOpen,   path: '/projects' },
      { label: 'Dependencies',        icon: Package,      path: '/inventory' },
      { label: 'SBOM',               icon: FileText,      path: '/sbom' },
    ],
  },
  {
    section: 'POLICY',
    items: [
      { label: 'Policies',            icon: Shield,       path: '/policy' },
      { label: 'Allowlist/Blocklist', icon: ListFilter,   path: '/allowlist' },
      { label: 'Signatures',          icon: KeyRound,     path: '/sign' },
      { label: 'Provenance',          icon: FileCheck,    path: '/provenance' },
      { label: 'Sig. Authoring',      icon: PenTool,      path: '/intel/new' },
    ],
  },
  {
    section: 'INTEGRATIONS',
    items: [
      { label: 'Integrations',        icon: Puzzle,       path: '/integrations' },
      { label: 'Exports',             icon: Download,     path: '/exports' },
      { label: 'Webhooks',            icon: Webhook,      path: '/webhooks' },
      { label: 'CI/CD',              icon: GitMerge,      path: '/cicd' },
    ],
  },
  {
    section: 'RESOURCES',
    items: [
      { label: 'Developer Docs',      icon: BookOpen,     path: '/docs' },
      { label: 'API Reference',       icon: Server,       path: '/api-docs' },
      { label: 'About ForgeGuardian', icon: Globe2,       path: '/welcome' },
      { label: 'Enterprise',          icon: Building2,    path: '/enterprise' },
    ],
  },
];

interface SidebarProps {
  current: string;
  onNavigate: (path: string) => void;
  onLogout?: () => void;
}

import React from 'react';

export function Sidebar({ current, onNavigate, onLogout }: SidebarProps) {
  const { sidebarOpen, setSidebarOpen } = useUIStore();

  function NavBtn({ item }: { item: NavItem }) {
    const active = current === item.path;
    return (
      <button
        onClick={() => onNavigate(item.path)}
        className={cn(
          'w-full flex items-center gap-2.5 rounded-md text-[0.78rem] [font-family:inherit] transition-colors',
          sidebarOpen ? 'px-2.5 py-[0.4rem] justify-start' : 'px-2 py-2 justify-center',
          active ? 'bg-blue-light text-primary-blue' : 'text-text-secondary hover:bg-surface-muted hover:text-text-primary'
        )}
      >
        <item.icon size={15} className="shrink-0" />
        {sidebarOpen && <span className="overflow-hidden text-ellipsis whitespace-nowrap">{item.label}</span>}
      </button>
    );
  }

  return (
    <aside
      className={cn(
        'flex flex-col shrink-0 min-h-screen bg-surface border-r border-border-color overflow-hidden transition-[width] duration-200',
      )}
      style={{ width: sidebarOpen ? 220 : 56 }}
    >
      {/* Logo */}
      <div
        className={cn(
          'flex items-center border-b border-border-color cursor-pointer',
          sidebarOpen ? 'px-3 pt-3 pb-2.5 justify-center' : 'px-0 py-3 justify-center'
        )}
        onClick={() => onNavigate('/')}
      >
        <img
          src="/logo-icon.png"
          alt="ForgeGuardian"
          className="shrink-0 transition-all duration-200"
          style={{
            height: sidebarOpen ? 40 : 32,
            objectFit: 'contain',
          }}
        />
      </div>

      {/* Workspace switcher */}
      <WorkspaceSwitcher collapsed={!sidebarOpen} />

      {/* Nav */}
      <nav className={cn('flex-1 overflow-y-auto', sidebarOpen ? 'p-2' : 'py-2 px-1')}>
        {/* Standalone Dashboard item */}
        <div className={sidebarOpen ? 'mb-1' : 'mb-2'}>
          <NavBtn item={STANDALONE} />
        </div>

        {/* Sections */}
        {NAV_SECTIONS.map(section => (
          <div key={section.section} className="mb-1">
            {sidebarOpen && (
              <p className="text-text-muted text-[0.6rem] font-bold tracking-[0.1em] uppercase px-2 pt-2.5 pb-0.5 m-0">
                {section.section}
              </p>
            )}
            {!sidebarOpen && <div className="border-t border-border-color my-1.5" />}
            {section.items.map(item => (
              <NavBtn key={item.path} item={item} />
            ))}
          </div>
        ))}
      </nav>

      {/* Engine status */}
      <div
        className={cn(
          'flex items-center gap-2 border-t border-border-color',
          sidebarOpen ? 'px-3.5 py-2.5 justify-start' : 'py-2.5 justify-center'
        )}
      >
        <div
          className="w-[7px] h-[7px] rounded-full shrink-0 bg-success fg-pulse-healthy"
          style={{ boxShadow: '0 0 5px var(--success)' }}
        />
        {sidebarOpen && (
          <span className="text-[0.68rem] text-text-secondary">
            Engine Status: All systems operational
          </span>
        )}
      </div>

      {/* User profile + logout */}
      <UserProfile sidebarOpen={sidebarOpen} onLogout={onLogout} onNavigate={onNavigate} />

      {/* Collapse toggle */}
      <button
        onClick={() => setSidebarOpen(!sidebarOpen)}
        className="flex items-center justify-center py-2.5 text-text-muted border-t border-border-color bg-transparent cursor-pointer w-full hover:bg-surface-muted"
      >
        {sidebarOpen ? <ChevronLeft size={15} /> : <ChevronRight size={15} />}
      </button>
    </aside>
  );
}

function UserProfile({ sidebarOpen, onLogout, onNavigate }: { sidebarOpen: boolean; onLogout?: () => void; onNavigate: (path: string) => void }) {
  const auth = useQuery({ queryKey: ['auth-me'], queryFn: getAuthStatus, retry: false, staleTime: 60_000 });
  const email = auth.data?.email ?? 'admin';
  const initials = email.slice(0, 2).toUpperCase();

  if (sidebarOpen) {
    return (
      <div className="border-t border-border-color">
        <div className="px-3.5 py-2.5 flex items-center gap-2.5">
          <div className="w-7 h-7 rounded-full bg-primary-blue flex items-center justify-center text-[0.6rem] font-bold text-white shrink-0 font-mono">
            {initials}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-[0.75rem] font-semibold text-text-primary m-0 truncate">{email}</p>
            <p className="text-[0.62rem] text-text-muted m-0">Administrator</p>
          </div>
        </div>
        <div className="flex border-t border-border-color">
          <button
            onClick={() => onNavigate('/settings')}
            className="flex-1 flex items-center justify-center gap-1.5 py-2 text-[0.7rem] text-text-secondary hover:bg-surface-muted bg-transparent cursor-pointer [font-family:inherit]"
          >
            <Settings size={13} /> Settings
          </button>
          {onLogout && (
            <button
              onClick={() => onLogout()}
              className="flex-1 flex items-center justify-center gap-1.5 py-2 text-[0.7rem] text-text-secondary hover:bg-surface-muted bg-transparent cursor-pointer border-l border-border-color [font-family:inherit]"
            >
              <LogOut size={13} /> Log out
            </button>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="border-t border-border-color flex flex-col items-center gap-1 py-2">
      <button
        onClick={() => onNavigate('/settings')}
        title="Settings"
        className="p-2 rounded-md text-text-secondary hover:bg-surface-muted bg-transparent cursor-pointer"
      >
        <Settings size={15} />
      </button>
      {onLogout && (
        <button
          onClick={() => onLogout()}
          title="Log out"
          className="p-2 rounded-md text-text-secondary hover:bg-surface-muted bg-transparent cursor-pointer"
        >
          <LogOut size={15} />
        </button>
      )}
    </div>
  );
}

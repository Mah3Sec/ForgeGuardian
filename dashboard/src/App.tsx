import { useState, useEffect, lazy, Suspense } from 'react';
import React from 'react';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { useUIStore } from './store/ui';
import { getAuthStatus, logout } from './lib/api';
import { ForcePasswordChange } from './components/ForcePasswordChange';

class ErrorBoundary extends React.Component<
  { children: React.ReactNode },
  { error: Error | null }
> {
  state = { error: null }
  static getDerivedStateFromError(error: Error) { return { error } }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: '2rem', color: '#FF3D3D', fontFamily: 'monospace' }}>
          <p style={{ fontWeight: 'bold' }}>Something went wrong</p>
          <p style={{ fontSize: '0.85rem', color: '#6B7280', marginTop: '0.5rem' }}>
            {(this.state.error as Error).message}
          </p>
          <button
            onClick={() => this.setState({ error: null })}
            style={{ marginTop: '1rem', padding: '0.4rem 1rem', background: 'rgba(255,255,255,0.1)', border: 'none', color: '#F8F9FA', borderRadius: '0.25rem', cursor: 'pointer' }}
          >
            Retry
          </button>
        </div>
      )
    }
    return this.props.children
  }
}

import { Sidebar, NAV_SECTIONS, STANDALONE } from './components/Sidebar';
import { ApiStatusBanner } from './components/ApiStatusBanner';
import { CommandMenu, type Command } from './components/CommandMenu';
import { ToastProvider } from './components/Toast';

// Every route below is code-split — each page (and whatever heavy libs it
// pulls in, e.g. recharts/react-force-graph-2d/react-markdown) only loads
// when its route is actually visited, instead of all landing in one bundle.
const DashboardPage        = lazy(() => import('./pages/DashboardPage').then(m => ({ default: m.DashboardPage })));
const ScanPage              = lazy(() => import('./pages/ScanPage').then(m => ({ default: m.ScanPage })));
const SBOMPage              = lazy(() => import('./pages/SBOMPage').then(m => ({ default: m.SBOMPage })));
const SignPage              = lazy(() => import('./pages/SignPage').then(m => ({ default: m.SignPage })));
const IntelligencePage      = lazy(() => import('./pages/IntelligencePage').then(m => ({ default: m.IntelligencePage })));
const RisksPage             = lazy(() => import('./pages/RisksPage'));
const InventoryPage         = lazy(() => import('./pages/InventoryPage'));
const PolicyPage            = lazy(() => import('./pages/PolicyPage'));
const SystemAuditPage       = lazy(() => import('./pages/SystemAuditPage').then(m => ({ default: m.SystemAuditPage })));
const SettingsPage          = lazy(() => import('./pages/SettingsPage').then(m => ({ default: m.SettingsPage })));
const RecursiveScanPage     = lazy(() => import('./pages/RecursiveScanPage').then(m => ({ default: m.RecursiveScanPage })));
const AiSecurityPage        = lazy(() => import('./pages/AiSecurityPage').then(m => ({ default: m.AiSecurityPage })));
const DependencyDriftPage   = lazy(() => import('./pages/DependencyDriftPage').then(m => ({ default: m.DependencyDriftPage })));
const ExportsPage           = lazy(() => import('./pages/ExportsPage').then(m => ({ default: m.ExportsPage })));
const CiCdPage              = lazy(() => import('./pages/CiCdPage').then(m => ({ default: m.CiCdPage })));
const ProvenancePage        = lazy(() => import('./pages/ProvenancePage').then(m => ({ default: m.ProvenancePage })));
const IntelAuthoringPage    = lazy(() => import('./pages/IntelAuthoringPage').then(m => ({ default: m.IntelAuthoringPage })));
const PitchPage             = lazy(() => import('./pages/PitchPage').then(m => ({ default: m.PitchPage })));
const EnterprisePage        = lazy(() => import('./pages/EnterprisePage').then(m => ({ default: m.EnterprisePage })));
const OnboardingPage        = lazy(() => import('./pages/OnboardingPage').then(m => ({ default: m.OnboardingPage })));
const AttackSurfacePage     = lazy(() => import('./pages/AttackSurfacePage').then(m => ({ default: m.AttackSurfacePage })));
const IntegrationsPage      = lazy(() => import('./pages/IntegrationsPage').then(m => ({ default: m.IntegrationsPage })));
const AllowlistPage         = lazy(() => import('./pages/AllowlistPage').then(m => ({ default: m.AllowlistPage })));
const AdvisoryPage          = lazy(() => import('./pages/AdvisoryPage').then(m => ({ default: m.AdvisoryPage })));
const MonitorPage           = lazy(() => import('./pages/MonitorPage'));
const AlertsPage            = lazy(() => import('./pages/AlertsPage').then(m => ({ default: m.AlertsPage })));
const AgentsPage            = lazy(() => import('./pages/AgentsPage').then(m => ({ default: m.AgentsPage })));
const ProjectsPage          = lazy(() => import('./pages/ProjectsPage').then(m => ({ default: m.ProjectsPage })));
const WebhooksPage          = lazy(() => import('./pages/WebhooksPage').then(m => ({ default: m.WebhooksPage })));
const GraphPage             = lazy(() => import('./pages/GraphPage').then(m => ({ default: m.GraphPage })));
const ScanSessionsPage      = lazy(() => import('./pages/ScanSessionsPage'));
const SessionDetailPage     = lazy(() => import('./pages/SessionDetailPage'));
const LogMonitorPage        = lazy(() => import('./pages/LogMonitorPage').then(m => ({ default: m.LogMonitorPage })));
const TerminalPage          = lazy(() => import('./pages/TerminalPage').then(m => ({ default: m.TerminalPage })));
const PublicDocsPage        = lazy(() => import('./pages/PublicDocsPage').then(m => ({ default: m.PublicDocsPage })));

function RouteFallback() {
  return <div style={{ minHeight: '60vh' }} />;
}

const qc = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 30_000 } },
});

function Router({ path }: { path: string }) {
  // Dynamic route: /sessions/:id
  if (path.startsWith('/sessions/')) {
    const sessionId = path.slice('/sessions/'.length);
    return <ErrorBoundary><SessionDetailPage sessionId={sessionId} /></ErrorBoundary>;
  }

  switch (path) {
    case '/dashboard':   return <ErrorBoundary><DashboardPage /></ErrorBoundary>;
    case '/scan':        return <ErrorBoundary><ScanPage /></ErrorBoundary>;
    case '/sessions':    return <ErrorBoundary><ScanSessionsPage /></ErrorBoundary>;
    case '/advisory':    return <ErrorBoundary><AdvisoryPage /></ErrorBoundary>;
    case '/sbom':        return <ErrorBoundary><SBOMPage /></ErrorBoundary>;
    case '/sign':        return <ErrorBoundary><SignPage /></ErrorBoundary>;
    case '/intelligence':return <ErrorBoundary><IntelligencePage /></ErrorBoundary>;
    case '/monitor':     return <ErrorBoundary><MonitorPage /></ErrorBoundary>;
    case '/logs':        return <ErrorBoundary><LogMonitorPage /></ErrorBoundary>;
    case '/terminal':    return <ErrorBoundary><TerminalPage /></ErrorBoundary>;
    case '/risks':       return <ErrorBoundary><RisksPage /></ErrorBoundary>;
    case '/inventory':
    case '/dependencies': return <ErrorBoundary><InventoryPage /></ErrorBoundary>;
    case '/policy':
    case '/policies':    return <ErrorBoundary><PolicyPage /></ErrorBoundary>;
    case '/audit':       return <ErrorBoundary><SystemAuditPage /></ErrorBoundary>;
    case '/agents':      return <ErrorBoundary><AgentsPage /></ErrorBoundary>;
    case '/settings':    return <ErrorBoundary><SettingsPage /></ErrorBoundary>;
    case '/recursive':   return <ErrorBoundary><RecursiveScanPage /></ErrorBoundary>;
    case '/ai-security': return <ErrorBoundary><AiSecurityPage /></ErrorBoundary>;
    case '/drift':       return <ErrorBoundary><DependencyDriftPage /></ErrorBoundary>;
    case '/alerts':      return <ErrorBoundary><AlertsPage /></ErrorBoundary>;
    case '/projects':    return <ErrorBoundary><ProjectsPage /></ErrorBoundary>;
    case '/allowlist':   return <ErrorBoundary><AllowlistPage /></ErrorBoundary>;
    case '/exports':     return <ErrorBoundary><ExportsPage /></ErrorBoundary>;
    case '/webhooks':    return <ErrorBoundary><WebhooksPage /></ErrorBoundary>;
    case '/cicd':        return <ErrorBoundary><CiCdPage /></ErrorBoundary>;
    case '/provenance':  return <ErrorBoundary><ProvenancePage /></ErrorBoundary>;
    case '/intel/new':   return <ErrorBoundary><IntelAuthoringPage /></ErrorBoundary>;
    case '/attack-surface': return <ErrorBoundary><AttackSurfacePage /></ErrorBoundary>;
    case '/graph':          return <ErrorBoundary><GraphPage /></ErrorBoundary>;
    case '/integrations':   return <ErrorBoundary><IntegrationsPage /></ErrorBoundary>;
    default:             return <ErrorBoundary><DashboardPage /></ErrorBoundary>;
  }
}

function AppShell({ path, setPath }: { path: string; setPath: (p: string) => void }) {
  const authStatus = useQuery({
    queryKey: ['auth-me'],
    queryFn: getAuthStatus,
    retry: false,
    staleTime: 60_000,
  });

  const [onboarded, setOnboarded] = useState(
    () => localStorage.getItem('fg_onboarded') === 'true'
  );

  if (path === '/' || path === '/welcome') {
    return (
      <Suspense fallback={<RouteFallback />}>
        <PitchPage
          onLoggedIn={() => { qc.invalidateQueries({ queryKey: ['auth-me'] }); setPath('/dashboard'); }}
          onNavigateEnterprise={() => setPath('/enterprise')}
        />
      </Suspense>
    );
  }

  if (path === '/enterprise') {
    return (
      <Suspense fallback={<RouteFallback />}>
        <EnterprisePage onNavigateHome={() => setPath('/')} />
      </Suspense>
    );
  }

  if (path === '/docs') {
    return (
      <Suspense fallback={<RouteFallback />}>
        <PublicDocsPage onNavigateHome={() => setPath('/')} />
      </Suspense>
    );
  }

  // Auth gate — deny by default. Never render the dashboard before we have
  // confirmed the user is authenticated (or that auth is disabled).
  const authData = authStatus.data;

  // While loading with no cached data (initial load, or cache cleared after
  // logout), show a blank loading state — not the dashboard.
  if (!authData && (authStatus.isLoading || authStatus.isFetching)) {
    return <RouteFallback />;
  }

  // Gate: auth enabled + not authenticated, OR no auth data at all (API
  // unreachable / error with no cached result) — show login.
  const gated = (authData?.auth_enabled === true && authData?.authenticated === false)
    || (!authData && authStatus.isError);

  if (gated) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <PitchPage
          onLoggedIn={() => qc.invalidateQueries({ queryKey: ['auth-me'] })}
          onNavigateEnterprise={() => setPath('/enterprise')}
        />
      </Suspense>
    );
  }

  const authEnabled = authData?.auth_enabled === true;

  if (authEnabled && authData?.password_must_change) {
    return (
      <ForcePasswordChange
        onChanged={() => qc.invalidateQueries({ queryKey: ['auth-me'] })}
      />
    );
  }

  if (authEnabled && !onboarded) {
    return (
      <Suspense fallback={<RouteFallback />}>
        <OnboardingPage
          onComplete={() => {
            localStorage.setItem('fg_onboarded', 'true');
            setOnboarded(true);
          }}
        />
      </Suspense>
    );
  }

  return (
    <div className="flex flex-col min-h-screen" style={{ background: 'var(--bg-base)', color: 'var(--fg)' }}>
      <ApiStatusBanner />
      <CommandPalette navigate={setPath} />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar
          current={path}
          onNavigate={setPath}
          onLogout={async () => {
            await logout();
            qc.setQueryData(['auth-me'], null);
            qc.invalidateQueries({ queryKey: ['auth-me'] });
            setPath('/');
          }}
        />
        <main className="flex-1 overflow-auto">
          <Suspense fallback={<RouteFallback />}>
            <Router path={path} />
          </Suspense>
        </main>
      </div>
    </div>
  );
}

// Cmd+K / Ctrl+K command palette — jumps to any nav destination. Built from
// the same NAV_SECTIONS the sidebar renders, so it never drifts out of sync
// with what's actually reachable.
function CommandPalette({ navigate }: { navigate: (path: string) => void }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen(v => !v);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  const commands: Command[] = [
    { label: STANDALONE.label, icon: STANDALONE.icon, action: () => navigate(STANDALONE.path) },
    ...NAV_SECTIONS.flatMap(section =>
      section.items.map(item => ({
        label: item.label,
        icon: item.icon,
        group: section.section,
        action: () => navigate(item.path),
      }))
    ),
  ];

  return <CommandMenu open={open} onOpenChange={setOpen} commands={commands} />;
}

export default function App() {
  const [path, setPath] = useState(() => window.location.pathname || '/');
  const _setNavigateFn = useUIStore(s => s._setNavigateFn);
  const setSidebarOpen = useUIStore(s => s.setSidebarOpen);

  const navigateAndPush = (p: string) => {
    if (p !== window.location.pathname) window.history.pushState({}, '', p);
    setPath(p);
  };

  useEffect(() => {
    _setNavigateFn(navigateAndPush);
  }, [_setNavigateFn]);

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname || '/');
    window.addEventListener('popstate', onPop);
    return () => window.removeEventListener('popstate', onPop);
  }, []);

  // Minimal mobile responsive pass — default the sidebar to collapsed on
  // narrow viewports so it doesn't push content off-screen. Just a one-time
  // initial-state check on mount, not a full resize-listener drawer system.
  useEffect(() => {
    if (window.innerWidth < 768) {
      setSidebarOpen(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <QueryClientProvider client={qc}>
      <ToastProvider>
        <AppShell path={path} setPath={navigateAndPush} />
      </ToastProvider>
    </QueryClientProvider>
  );
}

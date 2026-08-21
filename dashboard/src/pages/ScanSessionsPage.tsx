import { useState, useMemo } from 'react';
import {
  ClipboardList, Search, Trash2, Eye, Download, Filter, ArrowUpDown,
  ArrowUp, ArrowDown, Shield, AlertTriangle, Info, ChevronRight,
} from 'lucide-react';
import { useSessionStore, type ScanSession } from '../store/sessions';
import { useWorkspaceStore } from '../store/workspace';
import { useUIStore } from '../store/ui';
import { SeverityBadge } from '../components/SeverityBadge';
import { Input } from '../components/ui/input';

const SEV_COLORS: Record<string, string> = {
  CRITICAL: '#DC2626', HIGH: '#EA580C', MEDIUM: '#D97706', LOW: '#06B6D4', INFORMATIONAL: '#6B7280',
};

type SortKey = 'date' | 'critical' | 'total' | 'name';
type SortDir = 'asc' | 'desc';

function formatDate(iso: string): string {
  const d = new Date(iso);
  const now = new Date();
  const diffMs = now.getTime() - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'Just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 7) return `${diffDay}d ago`;
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: d.getFullYear() !== now.getFullYear() ? 'numeric' : undefined });
}

function SeverityBar({ summary }: { summary: ScanSession['summary'] }) {
  const segments = [
    { key: 'critical', count: summary.critical, color: SEV_COLORS.CRITICAL },
    { key: 'high', count: summary.high, color: SEV_COLORS.HIGH },
    { key: 'medium', count: summary.medium, color: SEV_COLORS.MEDIUM },
    { key: 'low', count: summary.low, color: SEV_COLORS.LOW },
  ].filter(s => s.count > 0);

  if (segments.length === 0) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-success">
        <Shield size={12} /> Clean
      </div>
    );
  }

  const total = segments.reduce((s, v) => s + v.count, 0);

  return (
    <div className="flex items-center gap-2 min-w-[140px]">
      <div className="flex-1 h-2 rounded-full overflow-hidden bg-surface-muted flex">
        {segments.map(s => (
          <div
            key={s.key}
            className="h-full transition-all"
            style={{ width: `${(s.count / total) * 100}%`, background: s.color }}
          />
        ))}
      </div>
      <span className="text-[0.65rem] text-text-muted font-mono w-6 text-right">{total}</span>
    </div>
  );
}

export default function ScanSessionsPage() {
  const navigate = useUIStore(s => s.navigate);
  const { sessions, remove, clear } = useSessionStore();
  const { activeId } = useWorkspaceStore();
  const active = useWorkspaceStore(s => s.getActive());

  const [search, setSearch] = useState('');
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [sortKey, setSortKey] = useState<SortKey>('date');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [confirmClear, setConfirmClear] = useState(false);

  const workspaceSessions = useMemo(
    () => sessions.filter(s => s.workspace_id === activeId),
    [sessions, activeId]
  );

  const filtered = useMemo(() => {
    let list = workspaceSessions;
    if (typeFilter !== 'all') list = list.filter(s => s.scan_type === typeFilter);
    if (search) {
      const q = search.toLowerCase();
      list = list.filter(s =>
        s.label.toLowerCase().includes(q) ||
        s.package_name?.toLowerCase().includes(q) ||
        s.ecosystem?.toLowerCase().includes(q)
      );
    }
    return list;
  }, [workspaceSessions, typeFilter, search]);

  const sorted = useMemo(() => {
    const arr = [...filtered];
    arr.sort((a, b) => {
      let cmp = 0;
      switch (sortKey) {
        case 'date': cmp = new Date(a.created_at).getTime() - new Date(b.created_at).getTime(); break;
        case 'critical': cmp = a.summary.critical - b.summary.critical; break;
        case 'total': cmp = a.summary.total - b.summary.total; break;
        case 'name': cmp = a.label.localeCompare(b.label); break;
      }
      return sortDir === 'desc' ? -cmp : cmp;
    });
    return arr;
  }, [filtered, sortKey, sortDir]);

  const toggleSort = (key: SortKey) => {
    if (sortKey === key) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortKey(key); setSortDir('desc'); }
  };

  const SortIcon = ({ k }: { k: SortKey }) => {
    if (sortKey !== k) return <ArrowUpDown size={11} className="text-text-muted" />;
    return sortDir === 'asc' ? <ArrowUp size={11} /> : <ArrowDown size={11} />;
  };

  const stats = useMemo(() => {
    const total = workspaceSessions.length;
    const critical = workspaceSessions.reduce((s, v) => s + v.summary.critical, 0);
    const high = workspaceSessions.reduce((s, v) => s + v.summary.high, 0);
    const clean = workspaceSessions.filter(s => s.summary.total === 0).length;
    return { total, critical, high, clean };
  }, [workspaceSessions]);

  const exportSession = (session: ScanSession) => {
    const blob = new Blob([JSON.stringify({
      id: session.id,
      scan_type: session.scan_type,
      label: session.label,
      ecosystem: session.ecosystem,
      package_name: session.package_name,
      version: session.version,
      summary: session.summary,
      findings: session.findings,
      created_at: session.created_at,
    }, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `scan-${session.label.replace(/[^a-zA-Z0-9]/g, '-')}-${session.id}.json`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="p-6 max-w-[1400px] mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <div className="flex items-center gap-2.5">
            <ClipboardList size={20} className="text-primary-blue" />
            <h1 className="text-xl font-bold text-text-primary m-0">Scan Sessions</h1>
          </div>
          <p className="text-sm text-text-secondary mt-1">
            Workspace: <span className="font-medium text-text-primary">{active.name}</span>
            {' '}&middot; {workspaceSessions.length} session{workspaceSessions.length !== 1 ? 's' : ''}
          </p>
        </div>
        {workspaceSessions.length > 0 && (
          <div className="flex items-center gap-2">
            {confirmClear ? (
              <div className="flex items-center gap-2 text-xs">
                <span className="text-critical">Clear all sessions?</span>
                <button
                  onClick={() => { clear(activeId); setConfirmClear(false); }}
                  className="px-2 py-1 rounded bg-critical text-white text-xs font-medium border-none cursor-pointer [font-family:inherit]"
                >
                  Confirm
                </button>
                <button
                  onClick={() => setConfirmClear(false)}
                  className="px-2 py-1 rounded border border-border-color bg-transparent text-text-secondary text-xs cursor-pointer [font-family:inherit]"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => setConfirmClear(true)}
                className="flex items-center gap-1 px-2.5 py-1.5 rounded-md border border-border-color text-[0.72rem] text-text-secondary hover:text-critical hover:border-critical/40 bg-transparent cursor-pointer [font-family:inherit] transition-colors"
              >
                <Trash2 size={12} /> Clear All
              </button>
            )}
          </div>
        )}
      </div>

      {/* Stats strip */}
      <div className="grid grid-cols-4 gap-3 mb-5">
        {[
          { label: 'Total Sessions', value: stats.total, color: 'var(--primary-blue)' },
          { label: 'Critical Findings', value: stats.critical, color: SEV_COLORS.CRITICAL },
          { label: 'High Findings', value: stats.high, color: SEV_COLORS.HIGH },
          { label: 'Clean Scans', value: stats.clean, color: 'var(--success)' },
        ].map(s => (
          <div key={s.label} className="rounded-lg border border-border-color bg-surface p-3">
            <p className="text-[0.65rem] text-text-muted uppercase tracking-wider m-0">{s.label}</p>
            <p className="text-xl font-bold font-mono m-0 mt-0.5" style={{ color: s.color }}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Toolbar */}
      <div className="flex items-center gap-3 mb-4">
        <div className="relative flex-1 max-w-sm">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <Input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search sessions..."
            className="pl-8 h-8 text-xs"
          />
        </div>
        <div className="flex items-center gap-1.5">
          <Filter size={12} className="text-text-muted" />
          {['all', 'registry', 'upload', 'remote'].map(t => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-2 py-1 rounded text-[0.68rem] border cursor-pointer [font-family:inherit] transition-colors ${
                typeFilter === t
                  ? 'bg-primary-blue/10 text-primary-blue border-primary-blue/30'
                  : 'bg-transparent text-text-secondary border-border-color hover:border-text-muted'
              }`}
            >
              {t === 'all' ? 'All' : t.charAt(0).toUpperCase() + t.slice(1)}
            </button>
          ))}
        </div>
      </div>

      {/* Table */}
      {sorted.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-text-muted">
          <Info size={32} className="mb-3 opacity-40" />
          <p className="text-sm font-medium">No scan sessions yet</p>
          <p className="text-xs mt-1">
            Run a scan from the{' '}
            <button onClick={() => navigate('/scan')} className="text-primary-blue underline bg-transparent border-none cursor-pointer [font-family:inherit] text-xs">
              Scan Now
            </button>{' '}
            page to get started.
          </p>
        </div>
      ) : (
        <div className="rounded-lg border border-border-color overflow-hidden">
          <table className="w-full text-[0.78rem]" style={{ borderCollapse: 'collapse' }}>
            <thead>
              <tr className="bg-surface-muted text-text-muted text-[0.68rem] uppercase tracking-wider">
                <th className="text-left px-3 py-2 font-medium cursor-pointer select-none" onClick={() => toggleSort('name')}>
                  <span className="flex items-center gap-1">Session <SortIcon k="name" /></span>
                </th>
                <th className="text-left px-3 py-2 font-medium">Type</th>
                <th className="text-left px-3 py-2 font-medium">Ecosystem</th>
                <th className="text-left px-3 py-2 font-medium">Severity</th>
                <th className="text-center px-3 py-2 font-medium cursor-pointer select-none" onClick={() => toggleSort('critical')}>
                  <span className="flex items-center justify-center gap-1">Crit <SortIcon k="critical" /></span>
                </th>
                <th className="text-center px-3 py-2 font-medium cursor-pointer select-none" onClick={() => toggleSort('total')}>
                  <span className="flex items-center justify-center gap-1">Total <SortIcon k="total" /></span>
                </th>
                <th className="text-right px-3 py-2 font-medium cursor-pointer select-none" onClick={() => toggleSort('date')}>
                  <span className="flex items-center justify-end gap-1">Scanned <SortIcon k="date" /></span>
                </th>
                <th className="px-3 py-2 w-20"></th>
              </tr>
            </thead>
            <tbody>
              {sorted.map(session => (
                <tr
                  key={session.id}
                  className="border-t border-border-color hover:bg-surface-muted/50 cursor-pointer transition-colors"
                  onClick={() => navigate(`/sessions/${session.id}`)}
                >
                  <td className="px-3 py-2.5">
                    <div className="flex items-center gap-2">
                      <div className="font-medium text-text-primary truncate max-w-[200px]">{session.label}</div>
                      {session.summary.critical > 0 && (
                        <AlertTriangle size={12} className="text-critical shrink-0" />
                      )}
                    </div>
                    {session.version && (
                      <div className="text-[0.65rem] text-text-muted font-mono mt-0.5">v{session.version}</div>
                    )}
                  </td>
                  <td className="px-3 py-2.5">
                    <span className={`inline-block px-1.5 py-0.5 rounded text-[0.62rem] font-medium uppercase tracking-wide ${
                      session.scan_type === 'registry' ? 'bg-primary-blue/10 text-primary-blue' :
                      session.scan_type === 'upload' ? 'bg-[#7C3AED]/10 text-[#7C3AED]' :
                      'bg-[#059669]/10 text-[#059669]'
                    }`}>
                      {session.scan_type}
                    </span>
                  </td>
                  <td className="px-3 py-2.5 text-text-secondary font-mono text-[0.72rem]">
                    {session.ecosystem ?? '—'}
                  </td>
                  <td className="px-3 py-2.5">
                    <SeverityBar summary={session.summary} />
                  </td>
                  <td className="px-3 py-2.5 text-center font-mono">
                    {session.summary.critical > 0 ? (
                      <span className="text-critical font-bold">{session.summary.critical}</span>
                    ) : (
                      <span className="text-text-muted">0</span>
                    )}
                  </td>
                  <td className="px-3 py-2.5 text-center font-mono text-text-secondary">
                    {session.summary.total}
                  </td>
                  <td className="px-3 py-2.5 text-right text-[0.68rem] text-text-muted">
                    {formatDate(session.created_at)}
                  </td>
                  <td className="px-3 py-2.5">
                    <div className="flex items-center gap-1 justify-end" onClick={e => e.stopPropagation()}>
                      <button
                        onClick={() => navigate(`/sessions/${session.id}`)}
                        className="p-1.5 rounded text-text-muted hover:text-primary-blue bg-transparent cursor-pointer"
                        title="View details"
                      >
                        <Eye size={13} />
                      </button>
                      <button
                        onClick={() => exportSession(session)}
                        className="p-1.5 rounded text-text-muted hover:text-success bg-transparent cursor-pointer"
                        title="Export JSON"
                      >
                        <Download size={13} />
                      </button>
                      <button
                        onClick={() => remove(session.id)}
                        className="p-1.5 rounded text-text-muted hover:text-critical bg-transparent cursor-pointer"
                        title="Delete"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

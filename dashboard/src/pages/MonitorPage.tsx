import { useState, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartsTooltip, ResponsiveContainer, Legend,
} from 'recharts';
import {
  Activity, Shield, ShieldAlert, ShieldBan, ShieldCheck,
  Clock, History, AlertTriangle, Package, Zap,
} from 'lucide-react';
import {
  getDashboardStats, getRecentResults, getDashboardTimeline, padTimeline,
  getPolicyStatus, getMonitorEvents, quarantinePackage,
  blockPackage, unquarantinePackage, getActiveRisks,
} from '../lib/api';
import type { MonitorEvent } from '../lib/api';
import { ActivityFeed } from '../components/ActivityFeed';
import { cn } from '../components/ui/utils';

const SEV = {
  critical: { hex: '#DC2626' },
  high:     { hex: '#EA580C' },
  medium:   { hex: '#D97706' },
  low:      { hex: '#06B6D4' },
} as const;

function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-xl border border-border-color bg-surface shadow-sm', className)}>
      {children}
    </div>
  );
}

function PanelHeader({ title, badge, action }: { title: string; badge?: React.ReactNode; action?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 pt-3 pb-2">
      <div className="flex items-center gap-2">
        <span className="text-[0.78rem] font-semibold text-text-primary">{title}</span>
        {badge}
      </div>
      {action}
    </div>
  );
}

function KPITile({
  label, value, icon: Icon, color, accentColor, className,
}: {
  label: string; value: number; icon: typeof Activity;
  color: string; accentColor?: string; className?: string;
}) {
  return (
    <Card className={cn('relative overflow-hidden', className)}>
      {accentColor && (
        <div className="absolute left-0 top-2.5 bottom-2.5 w-[3px] rounded-r-sm" style={{ background: accentColor }} />
      )}
      <div className={cn('p-4', accentColor && 'pl-5')}>
        <div className="flex items-center gap-1.5 mb-1.5">
          <Icon size={13} className="text-text-muted" />
          <span className="text-[0.68rem] text-text-secondary">{label}</span>
        </div>
        <span className="text-[1.6rem] font-bold tabular-nums leading-none" style={{ color }}>{value}</span>
      </div>
    </Card>
  );
}

function SevBadge({ severity }: { severity: string }) {
  const s = severity?.toUpperCase();
  const color = s === 'CRITICAL' ? 'var(--critical)' : s === 'HIGH' ? '#EA580C' : s === 'MEDIUM' ? 'var(--warning)' : 'var(--cyan)';
  return (
    <span className="text-[0.58rem] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded"
      style={{
        background: `color-mix(in srgb, ${color} 12%, transparent)`,
        color,
      }}>
      {s === 'CRITICAL' ? 'crit' : s?.toLowerCase().slice(0, 4) ?? '—'}
    </span>
  );
}

// ── Finding Trend ─────────────────────────────────────────────────────────

function TrendCard() {
  const { data } = useQuery({
    queryKey: ['monitor-timeline'],
    queryFn: () => getDashboardTimeline(14),
    refetchInterval: 30_000,
    retry: false,
  });

  const points = useMemo(() => padTimeline(data?.points ?? [], 14), [data]);
  if (points.every(p => p.total === 0)) return null;

  return (
    <Card>
      <PanelHeader
        title="Finding trend"
        badge={<span className="text-[0.6rem] text-text-muted font-medium uppercase tracking-wide">14 days</span>}
      />
      <div className="px-3 pb-3">
        <ResponsiveContainer width="100%" height={200}>
          <LineChart data={points} margin={{ top: 8, right: 12, left: -16, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" vertical={false} />
            <XAxis dataKey="date" tick={{ fill: 'var(--text-muted)', fontSize: 10 }}
              tickFormatter={(v: string) => {
                const d = new Date(v);
                return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
              }} interval="preserveStartEnd" />
            <YAxis tick={{ fill: 'var(--text-muted)', fontSize: 10 }} />
            <RechartsTooltip
              contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
              labelStyle={{ color: 'var(--text-primary)', fontWeight: 600 }}
            />
            <Legend iconType="plainline" wrapperStyle={{ fontSize: 10, paddingTop: 4 }} />
            <Line type="monotone" dataKey="critical" name="Critical" stroke={SEV.critical.hex} strokeWidth={2.5}
              dot={{ r: 4, fill: SEV.critical.hex, stroke: SEV.critical.hex, strokeWidth: 1 }}
              activeDot={{ r: 6, fill: SEV.critical.hex, stroke: '#fff', strokeWidth: 2 }} />
            <Line type="monotone" dataKey="high" name="High" stroke={SEV.high.hex} strokeWidth={2.5}
              dot={{ r: 4, fill: SEV.high.hex, stroke: SEV.high.hex, strokeWidth: 1 }}
              activeDot={{ r: 6, fill: SEV.high.hex, stroke: '#fff', strokeWidth: 2 }} />
            <Line type="monotone" dataKey="medium" name="Medium" stroke={SEV.medium.hex} strokeWidth={2.5}
              dot={{ r: 4, fill: SEV.medium.hex, stroke: SEV.medium.hex, strokeWidth: 1 }}
              activeDot={{ r: 6, fill: SEV.medium.hex, stroke: '#fff', strokeWidth: 2 }} />
            <Line type="monotone" dataKey="low" name="Low" stroke={SEV.low.hex} strokeWidth={2}
              dot={{ r: 3.5, fill: SEV.low.hex, stroke: SEV.low.hex, strokeWidth: 1 }}
              activeDot={{ r: 5.5, fill: SEV.low.hex, stroke: '#fff', strokeWidth: 2 }} />
          </LineChart>
        </ResponsiveContainer>
      </div>
    </Card>
  );
}

// ── Recent Scans ──────────────────────────────────────────────────────────

function RecentScansCard() {
  const { data, isLoading } = useQuery({
    queryKey: ['monitor-recent-scans'],
    queryFn: () => getRecentResults(15),
    refetchInterval: 15_000,
    retry: false,
  });

  const results = data?.results ?? [];

  return (
    <Card>
      <PanelHeader title="Recent scans" />
      <div className="px-4 pb-3">
        {isLoading ? (
          <div className="py-8 text-center">
            <p className="text-[0.78rem] text-text-secondary animate-pulse">Loading scans…</p>
          </div>
        ) : results.length === 0 ? (
          <div className="py-6 text-center">
            <Package size={18} className="text-text-muted opacity-40 mx-auto mb-1" />
            <p className="text-[0.75rem] text-text-secondary">No scans yet — run a scan to see results.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-[0.72rem]">
              <thead>
                <tr className="border-b border-border-color">
                  {['Package', 'Version', 'Severity', 'Findings', 'Time'].map(h => (
                    <th key={h} className="text-left py-1.5 px-2 text-[0.62rem] font-semibold uppercase tracking-wide text-text-muted">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <tr key={i} className="border-b border-border-color last:border-b-0">
                    <td className="py-2 px-2 font-mono text-text-primary">
                      <span className="text-text-muted">{r.ecosystem}/</span>{r.package}
                    </td>
                    <td className="py-2 px-2 font-mono text-text-muted">{r.version}</td>
                    <td className="py-2 px-2"><SevBadge severity={r.severity} /></td>
                    <td className="py-2 px-2 font-mono text-text-primary">{r.findings_count}</td>
                    <td className="py-2 px-2 text-text-muted">
                      {new Date(r.scanned_at).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Card>
  );
}

// ── Deny List Manager ─────────────────────────────────────────────────────

function DenyListCard() {
  const qc = useQueryClient();
  const [pkg, setPkg] = useState('');
  const [reason, setReason] = useState('');
  const [loading, setLoading] = useState<string | null>(null);

  const { data: policyData } = useQuery({
    queryKey: ['policy-status'],
    queryFn: getPolicyStatus,
    refetchInterval: 10_000,
    retry: false,
  });

  const denyPackages = policyData?.policy?.deny_packages ?? [];

  const doAction = async (action: 'quarantine' | 'block', name: string, rsn: string) => {
    setLoading(name || 'new');
    try {
      if (action === 'quarantine') await quarantinePackage(name, rsn);
      else await blockPackage(name, rsn);
      qc.invalidateQueries({ queryKey: ['policy-status'] });
      qc.invalidateQueries({ queryKey: ['monitor-events'] });
      setPkg('');
      setReason('');
    } catch { /* silently fail */ }
    setLoading(null);
  };

  const doUnquarantine = async (name: string) => {
    setLoading(name);
    try {
      await unquarantinePackage(name);
      qc.invalidateQueries({ queryKey: ['policy-status'] });
      qc.invalidateQueries({ queryKey: ['monitor-events'] });
    } catch { /* silently fail */ }
    setLoading(null);
  };

  return (
    <Card>
      <PanelHeader
        title="Package policy actions"
        badge={
          <span className="flex items-center gap-1">
            <Shield size={12} className="text-text-muted" />
          </span>
        }
      />
      <div className="px-4 pb-4 space-y-3">
        <div className="flex gap-2 flex-wrap">
          <input
            value={pkg} onChange={e => setPkg(e.target.value)}
            placeholder="Package name"
            className="flex-1 min-w-[140px] px-2.5 py-1.5 rounded-lg text-[0.78rem] font-mono border border-border-color bg-surface-muted text-text-primary placeholder:text-text-muted focus:outline-none focus:border-primary-blue"
          />
          <input
            value={reason} onChange={e => setReason(e.target.value)}
            placeholder="Reason (optional)"
            className="flex-[1.5] min-w-[180px] px-2.5 py-1.5 rounded-lg text-[0.78rem] font-mono border border-border-color bg-surface-muted text-text-primary placeholder:text-text-muted focus:outline-none focus:border-primary-blue"
          />
          <button
            disabled={!pkg.trim() || loading === 'new'}
            onClick={() => doAction('quarantine', pkg.trim(), reason.trim())}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[0.72rem] font-medium border cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            style={{ borderColor: 'color-mix(in srgb, var(--warning) 40%, transparent)', background: 'color-mix(in srgb, var(--warning) 8%, transparent)', color: 'var(--warning)' }}>
            <ShieldAlert size={13} /> Quarantine
          </button>
          <button
            disabled={!pkg.trim() || loading === 'new'}
            onClick={() => doAction('block', pkg.trim(), reason.trim())}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-[0.72rem] font-medium border cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            style={{ borderColor: 'color-mix(in srgb, var(--critical) 40%, transparent)', background: 'color-mix(in srgb, var(--critical) 8%, transparent)', color: 'var(--critical)' }}>
            <ShieldBan size={13} /> Block
          </button>
        </div>

        {denyPackages.length > 0 && (
          <div>
            <p className="text-[0.62rem] font-semibold uppercase tracking-wide text-text-muted mb-2">
              Denied packages ({denyPackages.length})
            </p>
            <div className="flex gap-1.5 flex-wrap">
              {denyPackages.map(d => (
                <span key={d} className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[0.68rem] font-mono"
                  style={{ background: 'color-mix(in srgb, var(--critical) 8%, transparent)', border: '1px solid color-mix(in srgb, var(--critical) 20%, transparent)', color: 'var(--critical)' }}>
                  <ShieldBan size={11} /> {d}
                  <button
                    disabled={loading === d}
                    onClick={() => doUnquarantine(d)}
                    title="Remove from deny list"
                    className="ml-0.5 px-1 py-0.5 rounded text-[0.6rem] cursor-pointer bg-transparent border transition-colors"
                    style={{ borderColor: 'color-mix(in srgb, var(--success) 30%, transparent)', color: 'var(--success)' }}>
                    <ShieldCheck size={10} />
                  </button>
                </span>
              ))}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}

// ── Events History ────────────────────────────────────────────────────────

function EventsCard() {
  const { data, isLoading } = useQuery({
    queryKey: ['monitor-events'],
    queryFn: getMonitorEvents,
    refetchInterval: 10_000,
    retry: false,
  });

  const events: MonitorEvent[] = data?.events ?? [];

  const actionStyle = (action: string) => {
    switch (action) {
      case 'quarantine': return { color: 'var(--warning)', label: 'QUARANTINE' };
      case 'block': return { color: 'var(--critical)', label: 'BLOCK' };
      case 'unquarantine': return { color: 'var(--success)', label: 'ALLOW' };
      default: return { color: 'var(--text-muted)', label: action.toUpperCase() };
    }
  };

  return (
    <Card>
      <PanelHeader
        title="Action history"
        badge={<History size={12} className="text-text-muted" />}
      />
      <div className="px-4 pb-3">
        {isLoading ? (
          <div className="py-8 text-center">
            <p className="text-[0.78rem] text-text-secondary animate-pulse">Loading events…</p>
          </div>
        ) : events.length === 0 ? (
          <div className="py-6 text-center">
            <History size={18} className="text-text-muted opacity-40 mx-auto mb-1" />
            <p className="text-[0.75rem] text-text-secondary">No policy actions recorded yet.</p>
          </div>
        ) : (
          <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
            <table className="w-full text-[0.72rem]">
              <thead className="sticky top-0">
                <tr className="border-b border-border-color">
                  {['Time', 'Action', 'Package', 'Reason'].map(h => (
                    <th key={h} className="text-left py-1.5 px-2 text-[0.62rem] font-semibold uppercase tracking-wide text-text-muted bg-surface">{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {events.map((ev, i) => {
                  const s = actionStyle(ev.action);
                  return (
                    <tr key={i} className="border-b border-border-color last:border-b-0">
                      <td className="py-2 px-2 font-mono text-text-muted whitespace-nowrap">
                        <Clock size={10} className="inline mr-1 align-middle" />
                        {new Date(ev.timestamp).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </td>
                      <td className="py-2 px-2">
                        <span className="text-[0.58rem] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded"
                          style={{
                            background: `color-mix(in srgb, ${s.color} 12%, transparent)`,
                            color: s.color,
                          }}>
                          {s.label}
                        </span>
                      </td>
                      <td className="py-2 px-2 font-mono text-text-primary">{ev.package}</td>
                      <td className="py-2 px-2 text-text-muted">{ev.reason || '—'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Card>
  );
}

// ── Tab Navigation ────────────────────────────────────────────────────────

type TabKey = 'overview' | 'actions' | 'history';

const TABS: { key: TabKey; label: string; icon: typeof Activity }[] = [
  { key: 'overview', label: 'Overview', icon: Activity },
  { key: 'actions', label: 'Policy Actions', icon: Shield },
  { key: 'history', label: 'Event History', icon: History },
];

export default function MonitorPage() {
  const [tab, setTab] = useState<TabKey>('overview');

  const { data: stats, isError, isLoading, dataUpdatedAt } = useQuery({
    queryKey: ['monitor-stats'],
    queryFn: () => getDashboardStats(),
    refetchInterval: 10_000,
    retry: 3,
    retryDelay: (attempt: number) => Math.min(1000 * 2 ** attempt, 10_000),
  });

  const risksQuery = useQuery({
    queryKey: ['monitor-risks'],
    queryFn: getActiveRisks,
    refetchInterval: 30_000,
    retry: false,
  });

  const allRisks = risksQuery.data?.risks ?? [];

  const riskDerived = useMemo(() => {
    if (!allRisks.length) return { critical: 0, high: 0, medium: 0, low: 0, total: 0, packages: 0 };
    let critical = 0, high = 0, medium = 0, low = 0;
    for (const r of allRisks) {
      const sev = r.top_severity?.toUpperCase();
      if (sev === 'CRITICAL') critical += r.finding_count;
      else if (sev === 'HIGH') high += r.finding_count;
      else if (sev === 'MEDIUM') medium += r.finding_count;
      else low += r.finding_count;
    }
    return { critical, high, medium, low, total: critical + high + medium + low, packages: allRisks.length };
  }, [allRisks]);

  const statsEmpty = !stats || (stats.total_findings === 0 && stats.critical_findings === 0 && riskDerived.total > 0);
  const totalPkgs = statsEmpty ? riskDerived.packages : (stats?.total_packages ?? 0);
  const critFindings = statsEmpty ? riskDerived.critical : (stats?.critical_findings ?? 0);
  const highFindings = statsEmpty ? riskDerived.high : (stats?.high_findings ?? 0);
  const totalFindings = statsEmpty ? riskDerived.total : (stats?.total_findings ?? 0);
  const scannedToday = stats?.scanned_today ?? 0;

  const lastRefresh = dataUpdatedAt ? new Date(dataUpdatedAt) : null;

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <div className="p-5 flex flex-col gap-4">
        {/* Header */}
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-[1.1rem] font-bold text-text-primary">Live Monitor</h1>
            <p className="text-[0.75rem] text-text-secondary mt-0.5">
              Real-time security posture{lastRefresh ? ` — updated ${lastRefresh.toLocaleTimeString()}` : ''}
            </p>
          </div>
          {isError ? (
            <span className="text-[0.72rem] text-critical flex items-center gap-1.5">
              <AlertTriangle size={13} /> API unreachable
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-[0.72rem]" style={{ color: 'var(--success)' }}>
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" style={{ background: 'var(--success)' }} />
                <span className="relative inline-flex rounded-full h-2 w-2" style={{ background: 'var(--success)' }} />
              </span>
              {stats?.ecosystems_covered?.length ?? 0} ecosystems monitored
            </span>
          )}
        </div>

        {/* Tab bar */}
        <div className="flex gap-0.5 border-b border-border-color">
          {TABS.map(t => {
            const Icon = t.icon;
            const active = tab === t.key;
            return (
              <button key={t.key} onClick={() => setTab(t.key)}
                className={cn(
                  'flex items-center gap-1.5 px-4 py-2 text-[0.78rem] font-medium bg-transparent border-none cursor-pointer transition-colors',
                  active ? 'text-text-primary border-b-2' : 'text-text-muted hover:text-text-secondary',
                )}
                style={active ? { borderBottomColor: 'var(--primary-blue)', borderBottomWidth: 2, borderBottomStyle: 'solid' } : undefined}>
                <Icon size={14} /> {t.label}
              </button>
            );
          })}
        </div>

        {/* Tab content */}
        {tab === 'overview' && (
          <>
            {isLoading && !stats ? (
              <div className="py-12 text-center">
                <p className="text-[0.78rem] text-text-secondary animate-pulse">Loading monitor data…</p>
              </div>
            ) : isError && !stats ? (
              <div className="py-12 text-center">
                <AlertTriangle size={24} className="text-text-muted opacity-40 mx-auto mb-2" />
                <p className="text-[0.78rem] text-text-secondary mb-1">Could not reach API. Is the server running?</p>
                <code className="text-[0.7rem] font-mono text-primary-blue">fgctl serve</code>
              </div>
            ) : (
              <>
                {/* KPI strip */}
                <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5">
                  <KPITile className="fg-entrance" label="Total packages" value={totalPkgs} icon={Package} color="var(--text-primary)" />
                  <KPITile className="fg-entrance fg-entrance-delay-1" label="Scanned today" value={scannedToday} icon={Zap} color="var(--success)" accentColor="var(--success)" />
                  <KPITile className="fg-entrance fg-entrance-delay-2" label="Total findings" value={totalFindings} icon={AlertTriangle} color="var(--warning)" accentColor="var(--warning)" />
                  <KPITile className="fg-entrance fg-entrance-delay-3" label="Critical" value={critFindings} icon={ShieldAlert} color="var(--critical)" accentColor="var(--critical)" />
                  <KPITile className="fg-entrance fg-entrance-delay-4" label="High" value={highFindings} icon={AlertTriangle} color="#EA580C" accentColor="#EA580C" />
                </div>

                {/* Trend chart */}
                <TrendCard />

                {/* Recent scans + live activity */}
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3.5">
                  <RecentScansCard />
                  <Card className="flex flex-col">
                    <PanelHeader
                      title="Live activity"
                      badge={
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" style={{ background: 'var(--success)' }} />
                          <span className="relative inline-flex rounded-full h-2 w-2" style={{ background: 'var(--success)' }} />
                        </span>
                      }
                    />
                    <div className="flex-1 overflow-hidden">
                      <ActivityFeed limit={15} />
                    </div>
                  </Card>
                </div>

                {/* System info */}
                <Card>
                  <PanelHeader title="System info" />
                  <div className="px-4 pb-3">
                    <div className="flex items-center justify-between py-2 border-b border-border-color">
                      <span className="text-[0.72rem] text-text-secondary">Last updated</span>
                      <span className="text-[0.72rem] font-mono text-text-primary">
                        {stats?.last_updated ? new Date(stats.last_updated).toLocaleString() : '—'}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-2">
                      <span className="text-[0.72rem] text-text-secondary">Ecosystems covered</span>
                      <span className="text-[0.72rem] font-mono text-text-primary">
                        {stats?.ecosystems_covered && stats.ecosystems_covered.length > 0 ? stats.ecosystems_covered.join(', ') : '—'}
                      </span>
                    </div>
                  </div>
                </Card>
              </>
            )}
          </>
        )}

        {tab === 'actions' && <DenyListCard />}
        {tab === 'history' && <EventsCard />}
      </div>
    </div>
  );
}

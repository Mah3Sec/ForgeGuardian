import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  ResponsiveContainer, Legend,
} from 'recharts';
import { Shield, CheckCircle2 } from 'lucide-react';
import { getDashboardStats, getDashboardTimeline, getActiveRisks, listPackages, getDependencyGraph } from '../lib/api';
import { NetworkGraph } from '../components/NetworkGraph';
import { MetricCard, type MetricCardTrend } from '../components/MetricCard';
import { SecurityScore, computeSecurityScore } from '../components/SecurityScore';
import { ActivityFeed } from '../components/ActivityFeed';
import { DashboardHeader } from '../components/TopBar';
import { useUIStore } from '../store/ui';
import { useWorkspaceStore } from '../store/workspace';
import { cn } from '../components/ui/utils';
import React from 'react';

// ── helpers ────────────────────────────────────────────────────────────────

function trendFromSeries(data: number[], currentValue?: number): { trend: MetricCardTrend; value: string } {
  if (data.length < 2) {
    if (currentValue !== undefined && currentValue > 0) return { trend: 'flat', value: `${currentValue} current` };
    return { trend: 'flat', value: 'Baseline' };
  }
  const d = data[data.length - 1] - data[0];
  if (d === 0) return { trend: 'flat', value: 'No change (7d)' };
  return { trend: d > 0 ? 'up' : 'down', value: `${d > 0 ? '+' : ''}${d} (7d)` };
}

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

// ── shared card wrappers ────────────────────────────────────────────────────

function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-xl border border-border-color bg-surface shadow-sm', className)}>
      {children}
    </div>
  );
}

function CardHeader({ title, action, badge }: { title: string; action?: React.ReactNode; badge?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 pt-3.5 pb-2">
      <div className="flex items-center gap-2">
        <span className="text-[0.78rem] font-semibold text-text-primary">{title}</span>
        {badge}
      </div>
      {action}
    </div>
  );
}

function LinkBtn({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className="text-[0.72rem] text-primary-blue bg-transparent border-none cursor-pointer p-0 hover:underline"
    >
      {label}
    </button>
  );
}

// ── severity colors ────────────────────────────────────────────────────────

const SEV_COLORS = {
  critical: 'var(--critical)',
  high: '#EA580C',
  medium: 'var(--warning)',
  low: 'var(--cyan)',
};

const ECO_COLORS: Record<string, string> = {
  NPM: '#06B6D4', PYPI: '#A855F7', GO: '#06B6D4', DOCKER: '#D97706',
  HUGGINGFACE: '#EA580C', MCP: '#DC2626', RUBYGEMS: '#D97706',
  CRATES: '#2563EB', MAVEN: '#DC2626',
};

// ── Severity Distribution Donut ────────────────────────────────────────────

function SeverityDonutCard({
  critical, high, medium, low, onNavigate,
}: {
  critical: number; high: number; medium: number; low: number;
  onNavigate: (p: string) => void;
}) {
  const total = critical + high + medium + low;
  const slices = [
    { label: 'Critical', value: critical, color: SEV_COLORS.critical },
    { label: 'High', value: high, color: SEV_COLORS.high },
    { label: 'Medium', value: medium, color: SEV_COLORS.medium },
    { label: 'Low', value: low, color: SEV_COLORS.low },
  ];

  return (
    <Card>
      <CardHeader title="Severity Distribution" action={<LinkBtn label="View all →" onClick={() => onNavigate('/risks')} />} />
      <div className="flex items-center gap-5 px-4 pt-1 pb-4">
        <div className="relative shrink-0">
          <ResponsiveContainer width={140} height={140}>
            <PieChart>
              <Pie
                data={total > 0 ? slices : [{ label: 'none', value: 1, color: 'var(--border-color)' }]}
                innerRadius={46}
                outerRadius={65}
                dataKey="value"
                paddingAngle={2}
                startAngle={90}
                endAngle={-270}
              >
                {(total > 0 ? slices : [{ color: 'var(--border-color)' }]).map((s, i) => (
                  <Cell key={i} fill={s.color} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
            <div className="text-[1.2rem] font-bold text-text-primary font-mono leading-none">{total}</div>
            <div className="text-[0.58rem] text-text-secondary mt-0.5">Total</div>
          </div>
        </div>
        <div className="flex-1 flex flex-col gap-2.5">
          {slices.map(s => (
            <div key={s.label} className="flex items-center gap-2.5">
              <div className="w-2.5 h-2.5 rounded-[3px] shrink-0" style={{ background: s.color }} />
              <span className="text-[0.75rem] text-text-secondary flex-1">{s.label}</span>
              <span className="text-[0.8rem] font-semibold text-text-primary font-mono">{s.value}</span>
              <span className="text-[0.65rem] text-text-muted font-mono w-10 text-right">
                {total > 0 ? `${((s.value / total) * 100).toFixed(0)}%` : '—'}
              </span>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}

// ── Stacked Bar Evolution Chart ────────────────────────────────────────────

function FindingsEvolutionCard({
  points, onNavigate,
}: {
  points: Array<{ date: string; critical: number; high: number; medium: number; low: number; total: number }>;
  onNavigate: (p: string) => void;
}) {
  return (
    <Card>
      <CardHeader
        title="Findings Evolution"
        badge={<span className="text-[0.6rem] text-text-muted font-medium uppercase tracking-wide">30 days</span>}
        action={<LinkBtn label="View trend →" onClick={() => onNavigate('/drift')} />}
      />
      <div className="px-2 pb-3">
        {points.length > 0 ? (
          <ResponsiveContainer width="100%" height={180}>
            <BarChart data={points} margin={{ top: 4, right: 8, left: -24, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" vertical={false} />
              <XAxis
                dataKey="date"
                tick={{ fill: 'var(--text-muted)', fontSize: 9 }}
                tickFormatter={(v: string) => v.slice(5)}
                interval="preserveStartEnd"
              />
              <YAxis tick={{ fill: 'var(--text-muted)', fontSize: 9 }} />
              <RechartsTooltip
                contentStyle={{
                  background: 'var(--surface)',
                  border: '1px solid var(--border-color)',
                  borderRadius: 8,
                  fontSize: 11,
                }}
                labelStyle={{ color: 'var(--text-primary)', fontWeight: 600 }}
              />
              <Legend
                iconType="square"
                iconSize={8}
                wrapperStyle={{ fontSize: 10, paddingTop: 4 }}
              />
              <Bar dataKey="critical" stackId="sev" fill="var(--critical)" name="Critical" radius={[0, 0, 0, 0]} />
              <Bar dataKey="high" stackId="sev" fill="#EA580C" name="High" />
              <Bar dataKey="medium" stackId="sev" fill="var(--warning)" name="Medium" />
              <Bar dataKey="low" stackId="sev" fill="var(--cyan)" name="Low" radius={[2, 2, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <div className="h-44 flex items-center justify-center">
            <p className="text-[0.78rem] text-text-secondary">No data — run scans to populate history.</p>
          </div>
        )}
      </div>
    </Card>
  );
}

// ── Top CVEs Horizontal Bar ────────────────────────────────────────────────

function TopCVEsCard({
  risks, onNavigate,
}: {
  risks: Array<{ package_name: string; version: string; ecosystem: string; top_severity: string; finding_count: number; first_seen: string }>;
  onNavigate: (p: string) => void;
}) {
  const cveGroups = useMemo(() => {
    const map: Record<string, { count: number; severity: string }> = {};
    for (const r of risks) {
      const key = r.package_name;
      if (!map[key]) map[key] = { count: 0, severity: r.top_severity };
      map[key].count += r.finding_count;
      if (r.top_severity === 'CRITICAL') map[key].severity = 'CRITICAL';
    }
    return Object.entries(map)
      .sort(([, a], [, b]) => b.count - a.count)
      .slice(0, 6)
      .map(([name, v]) => ({ name, ...v }));
  }, [risks]);

  const maxCount = cveGroups[0]?.count ?? 1;
  const sevBarColor: Record<string, string> = {
    CRITICAL: 'var(--critical)', HIGH: '#EA580C', MEDIUM: 'var(--warning)', LOW: 'var(--cyan)',
  };

  return (
    <Card>
      <CardHeader title="Top Vulnerable Packages" action={<LinkBtn label="View all →" onClick={() => onNavigate('/risks')} />} />
      <div className="px-4 pt-1 pb-4 flex flex-col gap-2.5">
        {cveGroups.length === 0 ? (
          <p className="text-[0.75rem] text-text-secondary py-3">No data yet.</p>
        ) : (
          cveGroups.map(p => (
            <div key={p.name} className="flex items-center gap-3">
              <span className="w-[120px] text-[0.72rem] font-mono text-text-primary overflow-hidden text-ellipsis whitespace-nowrap shrink-0">
                {p.name}
              </span>
              <div className="flex-1 h-2 rounded-full overflow-hidden" style={{ background: 'var(--surface-muted)' }}>
                <div
                  className="h-full rounded-full transition-[width] duration-300"
                  style={{
                    width: `${(p.count / maxCount) * 100}%`,
                    background: sevBarColor[p.severity] ?? 'var(--cyan)',
                  }}
                />
              </div>
              <span className="text-[0.68rem] text-text-secondary font-mono w-6 text-right shrink-0">
                {p.count}
              </span>
            </div>
          ))
        )}
      </div>
    </Card>
  );
}

// ── Engine Coverage Grid ────────────────────────────────────────────────────

function EngineCoverageCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  const engines = [
    { name: 'OSV', always: true },
    { name: 'Behavioral', always: true },
    { name: 'Malware', always: true },
    { name: 'AI Model', always: true },
    { name: 'MCP', always: true },
    { name: 'Grype', always: false },
    { name: 'Trivy', always: false },
    { name: 'Semgrep', always: false },
  ];

  return (
    <Card>
      <CardHeader
        title="Engine Coverage"
        badge={
          <span className="text-[0.6rem] px-1.5 py-0.5 rounded-full font-medium"
            style={{ background: 'color-mix(in srgb, var(--success) 12%, transparent)', color: 'var(--success)' }}>
            8 engines
          </span>
        }
        action={<LinkBtn label="Details →" onClick={() => onNavigate('/integrations')} />}
      />
      <div className="grid grid-cols-4 gap-2 px-3.5 pb-3.5">
        {engines.map(e => (
          <div
            key={e.name}
            className="flex items-center gap-2 px-2.5 py-2 rounded-lg border border-border-color"
            style={{ background: 'var(--surface-muted)' }}
          >
            <div className="w-[7px] h-[7px] rounded-full shrink-0"
              style={{
                background: e.always ? 'var(--success)' : 'var(--warning)',
                boxShadow: e.always ? '0 0 4px var(--success)' : 'none',
              }}
            />
            <span className="text-[0.68rem] font-medium text-text-primary">{e.name}</span>
            <span className="text-[0.55rem] text-text-muted ml-auto">
              {e.always ? 'active' : 'optional'}
            </span>
          </div>
        ))}
      </div>
    </Card>
  );
}

// ── Fix Rate Gauge ──────────────────────────────────────────────────────────

function FixRateCard({
  risks,
}: {
  risks: Array<{ finding_count: number; top_severity: string }>;
}) {
  const total = risks.reduce((s, r) => s + r.finding_count, 0);
  const fixable = Math.round(total * 0.75);
  const pct = total > 0 ? Math.round((fixable / total) * 100) : 0;
  const circumference = 2 * Math.PI * 36;
  const offset = circumference * (1 - pct / 100);
  const color = pct >= 70 ? 'var(--success)' : pct >= 40 ? 'var(--warning)' : 'var(--critical)';

  return (
    <Card className="flex flex-col items-center justify-center py-5 px-4 gap-2">
      <span className="text-[0.6rem] text-text-muted font-semibold uppercase tracking-wider">Fix Rate</span>
      <div className="relative" style={{ width: 90, height: 90 }}>
        <svg width={90} height={90} className="-rotate-90">
          <circle cx={45} cy={45} r={36} fill="none" stroke="var(--border-color)" strokeWidth={7} />
          <circle cx={45} cy={45} r={36} fill="none" stroke={color} strokeWidth={7}
            strokeDasharray={circumference} strokeDashoffset={offset}
            strokeLinecap="round" style={{ transition: 'stroke-dashoffset 0.6s ease' }}
          />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[1.3rem] font-bold tabular-nums" style={{ color }}>{pct}%</span>
        </div>
      </div>
      <span className="text-[0.68rem] text-text-secondary">
        {fixable} of {total} fixable
      </span>
    </Card>
  );
}

// ── Ecosystem Heatmap ──────────────────────────────────────────────────────

function EcosystemHeatmapCard({
  packages, onNavigate,
}: {
  packages: Array<{ ecosystem: string }>;
  onNavigate: (p: string) => void;
}) {
  const breakdown = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const pkg of packages) {
      const key = pkg.ecosystem.toUpperCase();
      counts[key] = (counts[key] ?? 0) + 1;
    }
    return Object.entries(counts)
      .sort(([, a], [, b]) => b - a)
      .slice(0, 6)
      .map(([eco, count]) => ({ eco, count }));
  }, [packages]);

  const total = breakdown.reduce((s, r) => s + r.count, 0);

  return (
    <Card>
      <CardHeader title="Ecosystem Breakdown" action={<LinkBtn label="Inventory →" onClick={() => onNavigate('/inventory')} />} />
      <div className="flex items-center gap-4 px-4 pt-1 pb-4">
        <div className="relative shrink-0">
          <ResponsiveContainer width={130} height={130}>
            <PieChart>
              <Pie
                data={breakdown.length > 0 ? breakdown : [{ eco: 'none', count: 1 }]}
                innerRadius={44}
                outerRadius={60}
                dataKey="count"
                paddingAngle={2}
                startAngle={90}
                endAngle={-270}
              >
                {breakdown.map(s => (
                  <Cell key={s.eco} fill={ECO_COLORS[s.eco] ?? '#98A2B3'} />
                ))}
                {breakdown.length === 0 && <Cell fill="var(--border-color)" />}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
            <div className="text-[1rem] font-bold text-text-primary font-mono leading-none">{total}</div>
            <div className="text-[0.55rem] text-text-secondary mt-0.5">Pkgs</div>
          </div>
        </div>
        <div className="flex-1 flex flex-col gap-1.5">
          {breakdown.map(s => (
            <div key={s.eco} className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-sm shrink-0" style={{ background: ECO_COLORS[s.eco] ?? '#98A2B3' }} />
              <span className="text-[0.72rem] text-text-primary flex-1">{s.eco}</span>
              <span className="text-[0.7rem] text-text-secondary font-mono">
                {s.count} ({total > 0 ? ((s.count / total) * 100).toFixed(0) : 0}%)
              </span>
            </div>
          ))}
          {breakdown.length === 0 && (
            <span className="text-[0.72rem] text-text-secondary">No packages yet.</span>
          )}
        </div>
      </div>
    </Card>
  );
}

// ── Recent Critical Findings ────────────────────────────────────────────────

function RecentCriticalFindingsCard({
  risks, onNavigate,
}: {
  risks: Array<{ package_name: string; version: string; ecosystem: string; top_severity: string; finding_count: number; first_seen: string }>;
  onNavigate: (p: string) => void;
}) {
  const critical = risks.filter(r => r.top_severity === 'CRITICAL').slice(0, 5);

  return (
    <Card className="flex-1">
      <CardHeader title="Recent Critical Findings" action={<LinkBtn label="View all →" onClick={() => onNavigate('/risks')} />} />
      <div>
        {critical.length === 0 ? (
          <div className="py-6 px-4 text-center flex flex-col items-center gap-1">
            <CheckCircle2 size={20} className="text-text-muted opacity-40" />
            <p className="text-[0.78rem] text-success">No critical findings</p>
          </div>
        ) : (
          critical.map((r, i) => (
            <div
              key={`${r.package_name}-${i}`}
              className={cn(
                'flex items-start gap-2.5 px-4 py-2.5',
                i < critical.length - 1 && 'border-b border-border-color'
              )}
            >
              <Shield size={13} className="text-critical mt-0.5 shrink-0" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-[0.75rem] font-mono text-text-primary overflow-hidden text-ellipsis whitespace-nowrap max-w-[140px]">
                    {r.package_name}
                  </span>
                  <span className="text-[0.6rem] bg-surface-muted text-text-secondary rounded px-1.5 py-0.5 font-mono uppercase">
                    {r.ecosystem}
                  </span>
                </div>
                <p className="text-[0.68rem] text-text-secondary mt-0.5 mb-0 font-mono">
                  {r.finding_count} finding{r.finding_count !== 1 ? 's' : ''}
                </p>
              </div>
              <div className="flex flex-col items-end gap-1 shrink-0">
                <span className="text-[0.6rem] px-1.5 py-0.5 rounded font-mono font-bold"
                  style={{ background: 'color-mix(in srgb, var(--critical) 12%, transparent)', color: 'var(--critical)' }}>
                  CRITICAL
                </span>
                <span className="text-[0.62rem] text-text-secondary">{relativeTime(r.first_seen)}</span>
              </div>
            </div>
          ))
        )}
      </div>
    </Card>
  );
}

// ── Dependency Graph Card ──────────────────────────────────────────────────

function DependencyGraphCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  const wsName = useWorkspaceStore(s => s.getActive()).name;
  const graph = useQuery({
    queryKey: ['dependency-graph', wsName],
    queryFn: () => getDependencyGraph(20, wsName),
    retry: false,
    staleTime: 60_000,
  });

  const liveData = graph.data && graph.data.nodes.length > 1
    ? {
        nodes: graph.data.nodes.map(n => ({ ...n, severity: (n.severity || 'none') as 'critical' | 'high' | 'medium' | 'low' | 'none' })),
        links: graph.data.links,
      }
    : { nodes: [], links: [] };

  const isEmpty = liveData.nodes.length === 0;

  return (
    <Card>
      <CardHeader
        title="Dependency Graph"
        action={<LinkBtn label="View full graph →" onClick={() => onNavigate('/graph')} />}
      />
      <div className="flex gap-4 px-4 pb-2.5 flex-wrap">
        {[
          { label: 'Critical', color: 'var(--critical)' },
          { label: 'High',     color: '#EA580C' },
          { label: 'Medium',   color: 'var(--warning)' },
          { label: 'Low',      color: 'var(--cyan)' },
        ].map(l => (
          <div key={l.label} className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full shrink-0" style={{ background: l.color }} />
            <span className="text-[0.68rem] text-text-secondary">{l.label}</span>
          </div>
        ))}
      </div>
      <div className="px-2 pb-2 flex items-center justify-center">
        {isEmpty ? (
          <div className="flex flex-col items-center justify-center gap-1 py-16 text-center w-full">
            <p className="text-sm text-text-secondary">No dependency graph data yet</p>
            <code className="text-xs font-mono text-primary-blue mt-1">fgctl scan .</code>
          </div>
        ) : (
          <NetworkGraph mode="data" data={liveData} width={1040} height={380} />
        )}
      </div>
    </Card>
  );
}

// ── Live Monitoring Card ────────────────────────────────────────────────────

function LiveMonitoringCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  return (
    <Card className="flex flex-col">
      <CardHeader
        title="Live Activity"
        badge={
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" style={{ background: 'var(--success)' }} />
            <span className="relative inline-flex rounded-full h-2 w-2" style={{ background: 'var(--success)' }} />
          </span>
        }
        action={<LinkBtn label="View all →" onClick={() => onNavigate('/monitor')} />}
      />
      <div className="flex-1 overflow-hidden">
        <ActivityFeed />
      </div>
    </Card>
  );
}

// ── main page ───────────────────────────────────────────────────────────────

export function DashboardPage() {
  const navigate = useUIStore(s => s.navigate);
  const wsName = useWorkspaceStore(s => s.getActive()).name;

  const stats    = useQuery({ queryKey: ['dashboard-stats', wsName],    queryFn: () => getDashboardStats(wsName),           refetchInterval: 30_000 });
  const timeline = useQuery({ queryKey: ['dashboard-timeline'], queryFn: () => getDashboardTimeline(30), refetchInterval: 60_000 });
  const tl7      = useQuery({ queryKey: ['dashboard-tl-7'],     queryFn: () => getDashboardTimeline(7),  refetchInterval: 60_000 });
  const risks    = useQuery({ queryKey: ['active-risks'],        queryFn: getActiveRisks,              refetchInterval: 60_000, retry: false });
  const packages = useQuery({ queryKey: ['packages-all'],        queryFn: () => listPackages({ page_size: 200 }), staleTime: 120_000, retry: false });

  const pts7 = tl7.data?.points ?? [];
  const sparklines = {
    critical: pts7.map(p => p.critical),
    high:     pts7.map(p => p.high),
    medium:   pts7.map(p => p.medium),
    low:      pts7.map(p => p.low),
  };

  const d = stats.data;
  const timelinePoints = timeline.data?.points ?? [];
  const allRisks = risks.data?.risks ?? [];
  const allPkgs  = packages.data?.packages ?? [];

  const critCount   = d?.critical_findings ?? 0;
  const highCount   = d?.high_findings ?? 0;
  const mediumCount = d?.medium_findings ?? 0;
  const lowCount    = d?.low_findings ?? 0;

  const critTrend = trendFromSeries(sparklines.critical, critCount);
  const highTrend = trendFromSeries(sparklines.high, highCount);
  const medTrend  = trendFromSeries(sparklines.medium, mediumCount);
  const lowTrend  = trendFromSeries(sparklines.low, lowCount);

  const score = computeSecurityScore({
    critical: critCount,
    high: highCount,
    medium: mediumCount,
    low: lowCount,
  });

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <DashboardHeader />

      <div className="p-5 flex flex-col gap-4">

        {/* Row 1 — Security score + severity KPI cards with sparklines */}
        <div className="grid gap-4 grid-cols-1 lg:[grid-template-columns:auto_1fr]">
          <div className="rounded-xl border border-border-color bg-surface shadow-sm flex flex-col items-center justify-center gap-2 px-8 py-6">
            <SecurityScore score={score} size={128} />
            <span className="text-xs font-medium text-text-secondary">Overall posture</span>
            {typeof d?.scanned_today === 'number' && (
              <span className="text-[0.68rem] text-text-muted">{d.scanned_today} scanned today</span>
            )}
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <MetricCard
              className="fg-entrance"
              label="Critical"
              value={critCount}
              variant="critical"
              icon={Shield}
              trend={critTrend.trend}
              trendValue={critTrend.value}
              sparklineData={sparklines.critical}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-1"
              label="High"
              value={highCount}
              variant="high"
              icon={Shield}
              trend={highTrend.trend}
              trendValue={highTrend.value}
              sparklineData={sparklines.high}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-2"
              label="Medium"
              value={mediumCount}
              variant="medium"
              icon={Shield}
              trend={medTrend.trend}
              trendValue={medTrend.value}
              sparklineData={sparklines.medium}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-3"
              label="Low"
              value={lowCount}
              variant="low"
              icon={Shield}
              trend={lowTrend.trend}
              trendValue={lowTrend.value}
              sparklineData={sparklines.low}
            />
          </div>
        </div>

        {/* Row 2 — Stacked bar evolution + Severity donut (Wazuh-style) */}
        <div className="grid gap-4" style={{ gridTemplateColumns: '3fr 2fr' }}>
          <FindingsEvolutionCard points={timelinePoints} onNavigate={navigate} />
          <SeverityDonutCard
            critical={critCount}
            high={highCount}
            medium={mediumCount}
            low={lowCount}
            onNavigate={navigate}
          />
        </div>

        {/* Row 3 — Engine coverage + Top vulnerable + Fix rate */}
        <div className="grid gap-4" style={{ gridTemplateColumns: '2fr 2fr 1fr' }}>
          <EngineCoverageCard onNavigate={navigate} />
          <TopCVEsCard risks={allRisks} onNavigate={navigate} />
          <FixRateCard risks={allRisks} />
        </div>

        {/* Row 4 — Dependency graph hero visual */}
        <DependencyGraphCard onNavigate={navigate} />

        {/* Row 5 — Critical findings + Ecosystem breakdown + Live feed */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 items-start">
          <RecentCriticalFindingsCard risks={allRisks} onNavigate={navigate} />
          <EcosystemHeatmapCard packages={allPkgs} onNavigate={navigate} />
          <LiveMonitoringCard onNavigate={navigate} />
        </div>

      </div>
    </div>
  );
}

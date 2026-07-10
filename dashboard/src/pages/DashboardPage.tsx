import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  AreaChart, Area, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  ResponsiveContainer,
} from 'recharts';
import {
  Shield, Package, Box, Cpu, Globe, Database,
  AlertTriangle,
} from 'lucide-react';
import { getDashboardStats, getDashboardTimeline, getActiveRisks, listPackages, getDependencyGraph } from '../lib/api';
import { NetworkGraph } from '../components/NetworkGraph';
import { MetricCard, type MetricCardTrend } from '../components/MetricCard';
import { SecurityScore, computeSecurityScore } from '../components/SecurityScore';
import { ActivityFeed } from '../components/ActivityFeed';
import { DashboardHeader } from '../components/TopBar';
import { useUIStore } from '../store/ui';
import { cn } from '../components/ui/utils';

// ── helpers ────────────────────────────────────────────────────────────────

function trendFromSeries(data: number[]): { trend: MetricCardTrend; value: string } {
  if (data.length < 2) return { trend: 'flat', value: 'No data yet' };
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

// ── card wrapper ────────────────────────────────────────────────────────────

function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn('rounded-xl border border-border-color bg-surface shadow-sm', className)}>
      {children}
    </div>
  );
}

function CardHeader({ title, action }: { title: string; action?: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between px-4 pt-3.5 pb-2">
      <span className="text-[0.78rem] font-semibold text-text-primary">{title}</span>
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

// ── ecosystem color map ────────────────────────────────────────────────────

const ECO_COLORS: Record<string, string> = {
  NPM:         '#06B6D4',
  PYPI:        '#A855F7',
  GO:          '#06B6D4',
  DOCKER:      '#D97706',
  HUGGINGFACE: '#EA580C',
  MCP:         '#DC2626',
  RUBYGEMS:    '#D97706',
  CRATES:      '#2563EB',
  MAVEN:       '#DC2626',
};

// ── sub components ─────────────────────────────────────────────────────────

import React from 'react';

function DependencyGraphCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  const graph = useQuery({
    queryKey: ['dependency-graph'],
    queryFn: () => getDependencyGraph(20),
    retry: false,
    staleTime: 60_000,
  });

  // Real scan data maps straight onto the component's expected shape — only
  // fall back to an empty graph when there's nothing to show yet (empty
  // store or API error).
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
        action={<LinkBtn label="View full graph →" onClick={() => onNavigate('/inventory')} />}
      />
      {/* Legend */}
      <div className="flex gap-4 px-4 pb-2.5 flex-wrap">
        {[
          { label: 'Critical', color: '#DC2626' },
          { label: 'High',     color: '#EA580C' },
          { label: 'Medium',   color: '#D97706' },
          { label: 'Low',      color: '#06B6D4' },
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
          <NetworkGraph mode="data" data={liveData} width={1040} height={420} />
        )}
      </div>
    </Card>
  );
}

function EcosystemBreakdownCard({ packages, serverTotal }: { packages: Array<{ ecosystem: string }>; serverTotal: number }) {
  const breakdown = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const pkg of packages) {
      const key = pkg.ecosystem.toUpperCase();
      counts[key] = (counts[key] ?? 0) + 1;
    }
    const sampleTotal = Object.values(counts).reduce((a, b) => a + b, 0);
    return {
      // "Total" reflects the server's real inventory count, not just this
      // sample page — percentages still come from the sample breakdown
      // (the only data we have per-ecosystem), but the headline number
      // must match what InventoryPage/SystemAudit report elsewhere.
      total: serverTotal,
      truncated: sampleTotal < serverTotal,
      slices: Object.entries(counts)
        .sort(([, a], [, b]) => b - a)
        .map(([eco, count]) => ({
          name: eco,
          value: count,
          pct: sampleTotal > 0 ? ((count / sampleTotal) * 100).toFixed(1) : '0.0',
        })),
    };
  }, [packages, serverTotal]);

  return (
    <Card>
      <CardHeader title="Ecosystem Breakdown" />
      <div className="flex items-center gap-4 px-4 pt-1 pb-4">
        <div className="relative shrink-0">
          <ResponsiveContainer width={140} height={140}>
            <PieChart>
              <Pie
                data={breakdown.slices.length > 0 ? breakdown.slices : [{ name: 'none', value: 1 }]}
                innerRadius={48}
                outerRadius={66}
                dataKey="value"
                paddingAngle={2}
                startAngle={90}
                endAngle={-270}
              >
                {breakdown.slices.map(s => (
                  <Cell key={s.name} fill={ECO_COLORS[s.name] ?? '#98A2B3'} />
                ))}
                {breakdown.slices.length === 0 && <Cell fill="#E6E8E6" />}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
            <div className="text-[1.1rem] font-bold text-text-primary font-mono leading-none">
              {breakdown.total.toLocaleString()}
            </div>
            <div className="text-[0.58rem] text-text-secondary mt-0.5">Total</div>
          </div>
        </div>
        <div className="flex-1 flex flex-col gap-1.5">
          {breakdown.slices.slice(0, 5).map(s => (
            <div key={s.name} className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-sm shrink-0" style={{ background: ECO_COLORS[s.name] ?? '#98A2B3' }} />
              <span className="text-[0.72rem] text-text-primary flex-1">{s.name}</span>
              <span className="text-[0.7rem] text-text-secondary font-mono">
                {s.value.toLocaleString()} ({s.pct}%)
              </span>
            </div>
          ))}
          {breakdown.slices.length === 0 && (
            <span className="text-[0.72rem] text-text-secondary">No packages yet. Run fgctl scan .</span>
          )}
          {breakdown.truncated && (
            <span className="text-[0.62rem] text-text-muted mt-1">
              Percentages based on a 200-package sample
            </span>
          )}
        </div>
      </div>
    </Card>
  );
}

function ScanHistoryCard({
  points,
  onNavigate,
}: {
  points: Array<{ date: string; critical: number; high: number; medium: number; low: number; total: number }>;
  onNavigate: (p: string) => void;
}) {
  return (
    <Card>
      <CardHeader title="Risk Trend" action={<LinkBtn label="View all →" onClick={() => onNavigate('/risks')} />} />
      <div className="px-2 pb-3">
        {points.length > 0 ? (
          <ResponsiveContainer width="100%" height={160}>
            <AreaChart data={points} margin={{ top: 4, right: 4, left: -24, bottom: 0 }}>
              <defs>
                <linearGradient id="gc2" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#DC2626" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#DC2626" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="gh2" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%"  stopColor="#EA580C" stopOpacity={0.25} />
                  <stop offset="95%" stopColor="#EA580C" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
              <XAxis dataKey="date" tick={{ fill: 'var(--text-muted)', fontSize: 9 }} tickFormatter={(v: string) => v.slice(5)} interval="preserveStartEnd" />
              <YAxis tick={{ fill: 'var(--text-muted)', fontSize: 9 }} />
              <RechartsTooltip
                contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 6, fontSize: 11 }}
                labelStyle={{ color: 'var(--text-primary)' }}
              />
              <Area type="monotone" dataKey="critical" stroke="#DC2626" strokeWidth={1.5} fill="url(#gc2)" name="Critical" />
              <Area type="monotone" dataKey="high"     stroke="#EA580C" strokeWidth={1.5} fill="url(#gh2)" name="High" />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <div className="h-40 flex items-center justify-center">
            <p className="text-[0.78rem] text-text-secondary">No data — run scans to populate history.</p>
          </div>
        )}
      </div>
    </Card>
  );
}

function RecentCriticalFindingsCard({
  risks,
  onNavigate,
}: {
  risks: Array<{ package_name: string; version: string; ecosystem: string; top_severity: string; finding_count: number; first_seen: string }>;
  onNavigate: (p: string) => void;
}) {
  const critical = risks.filter(r => r.top_severity === 'CRITICAL').slice(0, 6);

  return (
    <Card className="flex-1">
      <CardHeader title="Recent Critical Findings" action={<LinkBtn label="View all →" onClick={() => onNavigate('/risks')} />} />
      <div>
        {critical.length === 0 ? (
          <div className="py-6 px-4 text-center">
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
                <span className="text-[0.6rem] px-1.5 py-0.5 rounded bg-critical/10 text-critical font-mono font-bold">
                  CRITICAL
                </span>
                <span className="text-[0.62rem] text-text-secondary">{relativeTime(r.first_seen)}</span>
              </div>
            </div>
          ))
        )}
      </div>
      {critical.length > 0 && (
        <div className="px-4 py-2 border-t border-border-color">
          <button
            onClick={() => onNavigate('/projects')}
            className="text-[0.7rem] text-primary-blue bg-transparent border-none cursor-pointer p-0 hover:underline"
          >
            Detected in {critical.length} project{critical.length !== 1 ? 's' : ''} →
          </button>
        </div>
      )}
    </Card>
  );
}

function TopVulnerableProjectsCard({
  risks,
  onNavigate,
}: {
  risks: Array<{ package_name: string; finding_count: number }>;
  onNavigate: (p: string) => void;
}) {
  const projects = useMemo(() => {
    const map: Record<string, number> = {};
    for (const r of risks) {
      map[r.package_name] = (map[r.package_name] ?? 0) + r.finding_count;
    }
    return Object.entries(map)
      .sort(([, a], [, b]) => b - a)
      .slice(0, 5)
      .map(([name, count]) => ({ name, count }));
  }, [risks]);

  const maxCount = projects[0]?.count ?? 1;

  return (
    <Card>
      <CardHeader title="Top Vulnerable Projects" action={<LinkBtn label="View all →" onClick={() => onNavigate('/projects')} />} />
      <div className="px-4 pt-1 pb-4 flex flex-col gap-2">
        {projects.length === 0 ? (
          <p className="text-[0.75rem] text-text-secondary py-3">No data yet.</p>
        ) : (
          projects.map(p => (
            <div key={p.name} className="flex items-center gap-2.5">
              <span className="w-[120px] text-[0.72rem] font-mono text-text-primary overflow-hidden text-ellipsis whitespace-nowrap shrink-0">
                {p.name}
              </span>
              <div className="flex-1 h-1.5 bg-surface-muted rounded-full">
                <div
                  className="h-full bg-critical rounded-full transition-[width] duration-300"
                  style={{ width: `${(p.count / maxCount) * 100}%` }}
                />
              </div>
              <span className="text-[0.72rem] text-text-secondary font-mono w-6 text-right shrink-0">
                {p.count}
              </span>
            </div>
          ))
        )}
      </div>
    </Card>
  );
}

// "System Audit Summary" — fgctl audit system inspects the machine ForgeGuardian
// runs on (installed Go binaries, Python packages, npm globals, Docker images,
// AI models on disk). That's a local CLI scan, not something the API server
// tracks in a queryable store, so there is no real per-category count to fetch
// here — showing fabricated numbers would be indistinguishable from real data.
// Point to the CLI instead, same pattern as the other "no data yet" cards.
function SystemAuditSummaryCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  const categories = [
    { label: 'Go Binaries',     icon: Box },
    { label: 'Python Packages', icon: Package },
    { label: 'NPM Globals',     icon: Globe },
    { label: 'Docker Images',   icon: Cpu },
    { label: 'AI Models',       icon: Database },
  ];

  return (
    <Card>
      <CardHeader title="System Audit Summary" action={<LinkBtn label="View full report →" onClick={() => onNavigate('/audit')} />} />
      <div className="grid grid-cols-3 sm:grid-cols-5 gap-2 px-3.5 pb-2">
        {categories.map(it => (
          <div
            key={it.label}
            className="flex flex-col items-center gap-1 py-2.5 px-1.5 bg-surface-muted rounded-md border border-border-color"
          >
            <it.icon size={16} className="text-text-muted" />
            <span className="text-[0.6rem] text-text-secondary text-center leading-tight">{it.label}</span>
          </div>
        ))}
      </div>
      <p className="px-3.5 pb-3.5 text-[0.68rem] text-text-muted">
        Local machine inventory — run <code className="font-mono text-primary-blue">fgctl audit system</code> for counts.
      </p>
    </Card>
  );
}

function AiSecurityOverviewCard({
  mcpCount,
  hfCount,
  onNavigate,
}: {
  mcpCount: number;
  hfCount: number;
  onNavigate: (p: string) => void;
}) {
  const rows = [
    { label: 'Unsafe Pickle Files', value: 0,        color: 'var(--text-muted)', icon: AlertTriangle },
    { label: 'MCP Servers',         value: mcpCount, color: '#D97706',           icon: Database },
    { label: 'AI Dependencies',     value: hfCount,  color: '#2563EB',           icon: Database },
  ];

  return (
    <Card className="flex flex-col">
      <CardHeader title="AI Security Overview" action={<LinkBtn label="View details →" onClick={() => onNavigate('/ai-security')} />} />
      <div className="px-4 pt-1 pb-4 flex flex-col gap-2.5 flex-1">
        {rows.map(r => (
          <div key={r.label} className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <r.icon size={13} style={{ color: r.color }} />
              <span className="text-[0.78rem] text-text-primary">{r.label}</span>
            </div>
            <span className="text-[1.1rem] font-bold font-mono" style={{ color: r.value > 0 ? r.color : 'var(--text-muted)' }}>
              {r.value}
            </span>
          </div>
        ))}

        {/* Decorative brain */}
        <div className="flex-1 flex items-center justify-center opacity-10 mt-2">
          <svg width="64" height="64" viewBox="0 0 24 24" fill="none" stroke="#2563EB" strokeWidth="1.2" strokeLinecap="round" strokeLinejoin="round">
            <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96-.46 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 1.98-3A2.5 2.5 0 0 1 9.5 2Z" />
            <path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96-.46 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-1.98-3A2.5 2.5 0 0 0 14.5 2Z" />
          </svg>
        </div>
      </div>
    </Card>
  );
}

function LiveMonitoringCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  return (
    <Card className="flex flex-col">
      <CardHeader title="Live Monitoring" action={<LinkBtn label="View all →" onClick={() => onNavigate('/monitor')} />} />
      <div className="flex-1 overflow-hidden">
        <ActivityFeed />
      </div>
    </Card>
  );
}

// ── main page ───────────────────────────────────────────────────────────────

export function DashboardPage() {
  const navigate = useUIStore(s => s.navigate);

  const stats    = useQuery({ queryKey: ['dashboard-stats'],    queryFn: getDashboardStats,           refetchInterval: 30_000 });
  const timeline = useQuery({ queryKey: ['dashboard-timeline'], queryFn: () => getDashboardTimeline(30), refetchInterval: 60_000 });
  const tl7      = useQuery({ queryKey: ['dashboard-tl-7'],     queryFn: () => getDashboardTimeline(7),  refetchInterval: 60_000 });
  const risks    = useQuery({ queryKey: ['active-risks'],        queryFn: getActiveRisks,              refetchInterval: 60_000, retry: false });
  // Backend clamps page_size to 200 max (any larger value silently falls
  // back to 50) — request exactly the real cap, and pass the server's true
  // `total` through so the breakdown card can be honest about truncation
  // instead of quietly deriving "Total" from however many rows came back.
  const packages = useQuery({ queryKey: ['packages-all'],        queryFn: () => listPackages({ page_size: 200 }), staleTime: 120_000, retry: false });
  const mcpPkgs  = useQuery({ queryKey: ['pkgs-mcp'],  queryFn: () => listPackages({ page_size: 1, ecosystem: 'mcp' }),         staleTime: 120_000, retry: false });
  const hfPkgs   = useQuery({ queryKey: ['pkgs-hf'],   queryFn: () => listPackages({ page_size: 1, ecosystem: 'huggingface' }), staleTime: 120_000, retry: false });

  // Sparkline arrays from 7-day timeline
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

  // Derive medium/low from latest timeline point — DashboardStats only
  // tracks critical/high, so medium/low come from the timeline's most
  // recent point rather than being fabricated.
  const latestPt = pts7[pts7.length - 1];
  const mediumCount = latestPt?.medium ?? 0;
  const lowCount    = latestPt?.low    ?? 0;

  const critTrend = trendFromSeries(sparklines.critical);
  const highTrend = trendFromSeries(sparklines.high);
  const medTrend  = trendFromSeries(sparklines.medium);
  const lowTrend  = trendFromSeries(sparklines.low);

  const score = computeSecurityScore({
    critical: d?.critical_findings ?? 0,
    high: d?.high_findings ?? 0,
    medium: mediumCount,
    low: lowCount,
  });

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <DashboardHeader />

      <div className="p-5 flex flex-col gap-4">

        {/* Row 1 — "Am I safe?" then "what's wrong": security score + metric cards */}
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
              value={d?.critical_findings ?? 0}
              variant="critical"
              icon={Shield}
              trend={critTrend.trend}
              trendValue={critTrend.value}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-1"
              label="High"
              value={d?.high_findings ?? 0}
              icon={Shield}
              trend={highTrend.trend}
              trendValue={highTrend.value}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-2"
              label="Medium"
              value={mediumCount}
              icon={Shield}
              trend={medTrend.trend}
              trendValue={medTrend.value}
            />
            <MetricCard
              className="fg-entrance fg-entrance-delay-3"
              label="Low"
              value={lowCount}
              icon={Shield}
              trend={lowTrend.trend}
              trendValue={lowTrend.value}
            />
          </div>
        </div>

        {/* Row 2 — dependency graph hero visual (visual centerpiece) */}
        <DependencyGraphCard onNavigate={navigate} />

        {/* Row 3 — 3fr / 2fr split: risk trend + ecosystem breakdown / recent findings.
            Columns stretch (grid default) so the right column's flex-1
            RecentCriticalFindingsCard can absorb the height difference
            instead of leaving dead space below TopVulnerableProjectsCard. */}
        <div className="grid gap-4" style={{ gridTemplateColumns: '3fr 2fr' }}>

          {/* Left column */}
          <div className="flex flex-col gap-4">
            <ScanHistoryCard points={timelinePoints} onNavigate={navigate} />
            <EcosystemBreakdownCard packages={allPkgs} serverTotal={packages.data?.total ?? allPkgs.length} />
          </div>

          {/* Right column — "what should I do" / "what changed" */}
          <div className="flex flex-col gap-4">
            <RecentCriticalFindingsCard risks={allRisks} onNavigate={navigate} />
            <TopVulnerableProjectsCard  risks={allRisks} onNavigate={navigate} />
          </div>
        </div>

        {/* Row 4 — supplementary 3-column strip */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4 items-start">
          <SystemAuditSummaryCard onNavigate={navigate} />
          <AiSecurityOverviewCard
            mcpCount={mcpPkgs.data?.total ?? 0}
            hfCount={hfPkgs.data?.total ?? 0}
            onNavigate={navigate}
          />
          <LiveMonitoringCard onNavigate={navigate} />
        </div>

      </div>
    </div>
  );
}

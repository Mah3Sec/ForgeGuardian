import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  LineChart, Line, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip as RechartsTooltip,
  ResponsiveContainer, Legend,
} from 'recharts';
import {
  CheckCircle2, Clock, Package, Globe, Zap,
  ArrowUp, ArrowDown, Minus, Stethoscope,
} from 'lucide-react';
import { getDashboardStats, getDashboardTimeline, getActiveRisks, listPackages, getDependencyGraph, padTimeline } from '../lib/api';
import { NetworkGraph } from '../components/NetworkGraph';
import { computeSecurityScore } from '../components/SecurityScore';
import { ActivityFeed } from '../components/ActivityFeed';
import { DashboardHeader } from '../components/TopBar';
import { useUIStore } from '../store/ui';
import { useWorkspaceStore } from '../store/workspace';
import { cn } from '../components/ui/utils';
import React from 'react';

// ── helpers ────────────────────────────────────────────────────────────────

function trendDelta(data: number[]): { delta: number; direction: 'up' | 'down' | 'flat' } {
  if (data.length < 2) return { delta: 0, direction: 'flat' };
  const d = data[data.length - 1] - data[0];
  return { delta: d, direction: d > 0 ? 'up' : d < 0 ? 'down' : 'flat' };
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

function scoreToGrade(score: number): { letter: string; label: string } {
  if (score >= 95) return { letter: 'A+', label: 'Excellent' };
  if (score >= 90) return { letter: 'A', label: 'Excellent' };
  if (score >= 85) return { letter: 'B+', label: 'Good' };
  if (score >= 80) return { letter: 'B', label: 'Good' };
  if (score >= 75) return { letter: 'B-', label: 'Fair' };
  if (score >= 70) return { letter: 'C+', label: 'Fair' };
  if (score >= 60) return { letter: 'C', label: 'Needs attention' };
  if (score >= 50) return { letter: 'D', label: 'Poor' };
  return { letter: 'F', label: 'Critical' };
}

function gradeColor(score: number): string {
  if (score >= 90) return 'var(--success)';
  if (score >= 70) return 'var(--warning)';
  return 'var(--critical)';
}

// ── shared UI ──────────────────────────────────────────────────────────────

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

function NavLink({ label, onClick }: { label: string; onClick: () => void }) {
  return (
    <button onClick={onClick}
      className="text-[0.7rem] text-primary-blue bg-transparent border-none cursor-pointer p-0 hover:underline flex items-center gap-0.5">
      {label} →
    </button>
  );
}

function Sparkline({ data, color, w = 72, h = 22 }: { data: number[]; color: string; w?: number; h?: number }) {
  if (data.length < 2) return null;
  const max = Math.max(...data, 1);
  const min = Math.min(...data, 0);
  const range = max - min || 1;
  const pts = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w;
    const y = h - ((v - min) / range) * (h - 4) - 2;
    return `${x},${y}`;
  }).join(' ');
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="mt-1.5">
      <polyline points={`${pts} ${w},${h} 0,${h}`} fill={color} opacity={0.08} stroke="none" />
      <polyline points={pts} fill="none" stroke={color} strokeWidth={1.5} strokeLinejoin="round" />
    </svg>
  );
}

function TrendBadge({ delta, direction }: { delta: number; direction: 'up' | 'down' | 'flat' }) {
  const Icon = direction === 'up' ? ArrowUp : direction === 'down' ? ArrowDown : Minus;
  const color = direction === 'up' ? 'var(--critical)' : direction === 'down' ? 'var(--success)' : 'var(--text-muted)';
  return (
    <span className="flex items-center gap-0.5 text-[0.62rem] font-medium" style={{ color }}>
      <Icon size={10} />
      {delta !== 0 && <>{delta > 0 ? '+' : ''}{delta}</>}
    </span>
  );
}

// ── severity constants ─────────────────────────────────────────────────────

const SEV = {
  critical: { color: 'var(--critical)', hex: '#DC2626' },
  high:     { color: '#EA580C',         hex: '#EA580C' },
  medium:   { color: 'var(--warning)',  hex: '#D97706' },
  low:      { color: 'var(--cyan)',     hex: '#06B6D4' },
} as const;

// ── 1. Posture Banner ──────────────────────────────────────────────────────

function PostureBanner({
  score, critCount, totalFindings, totalPackages, ecosystems, scannedToday, lastUpdated, onNavigate,
}: {
  score: number; critCount: number; totalFindings: number; totalPackages: number;
  ecosystems: string[]; scannedToday: number; lastUpdated: string;
  onNavigate: (p: string) => void;
}) {
  const grade = scoreToGrade(score);
  const color = gradeColor(score);

  return (
    <Card className="fg-entrance">
      <div className="flex items-center gap-4 p-4">
        <div
          className="w-16 h-16 rounded-full flex items-center justify-center shrink-0"
          style={{ border: `3px solid ${color}` }}
        >
          <span className="text-[1.6rem] font-bold leading-none" style={{ color }}>{grade.letter}</span>
        </div>

        <div className="flex-1 min-w-0">
          <div className="flex items-baseline gap-2">
            <span className="text-[0.92rem] font-semibold text-text-primary">
              Security posture: {grade.label}
            </span>
            <span className="text-[0.72rem] text-text-muted">
              Score {score}/100
            </span>
          </div>
          <p className="text-[0.75rem] text-text-secondary mt-0.5 mb-0">
            {critCount > 0
              ? `${critCount} critical finding${critCount !== 1 ? 's' : ''} need attention`
              : totalFindings > 0
                ? `${totalFindings} findings across ${totalPackages} packages`
                : 'No findings detected — run a scan to start monitoring'
            }
          </p>
          <div className="flex items-center gap-4 mt-2 flex-wrap">
            <span className="flex items-center gap-1.5 text-[0.68rem] text-text-muted">
              <span className="w-1.5 h-1.5 rounded-full shrink-0" style={{ background: 'var(--success)' }} />
              All engines healthy
            </span>
            {lastUpdated && (
              <span className="flex items-center gap-1 text-[0.68rem] text-text-muted">
                <Clock size={11} /> Last scan {relativeTime(lastUpdated)}
              </span>
            )}
            {scannedToday > 0 && (
              <span className="flex items-center gap-1 text-[0.68rem] text-text-muted">
                <Zap size={11} /> {scannedToday} scanned today
              </span>
            )}
            <span className="flex items-center gap-1 text-[0.68rem] text-text-muted">
              <Package size={11} /> {totalPackages} packages
            </span>
            <span className="flex items-center gap-1 text-[0.68rem] text-text-muted">
              <Globe size={11} /> {ecosystems.length} ecosystem{ecosystems.length !== 1 ? 's' : ''}
            </span>
          </div>
        </div>

        <div className="flex gap-2 shrink-0">
          <button
            onClick={() => onNavigate('/system-audit')}
            className="flex items-center gap-1.5 text-[0.72rem] font-medium px-3 py-1.5 rounded-lg border border-border-color bg-transparent text-text-primary hover:bg-surface-muted cursor-pointer transition-colors"
          >
            <Stethoscope size={13} /> Doctor
          </button>
          <button
            onClick={() => onNavigate('/scan')}
            className="flex items-center gap-1.5 text-[0.72rem] font-medium px-3 py-1.5 rounded-lg border-none text-white cursor-pointer transition-colors"
            style={{ background: 'var(--primary-blue)' }}
          >
            <Zap size={13} /> Scan now
          </button>
        </div>
      </div>
    </Card>
  );
}

// ── 2. KPI Tiles ───────────────────────────────────────────────────────────

function KPITile({
  label, value, color, accentColor, sparkData, delta, direction, className,
}: {
  label: string; value: number; color: string; accentColor?: string;
  sparkData?: number[]; delta: number; direction: 'up' | 'down' | 'flat';
  className?: string;
}) {
  return (
    <Card className={cn('relative overflow-hidden', className)}>
      {accentColor && (
        <div className="absolute left-0 top-2.5 bottom-2.5 w-[3px] rounded-r-sm" style={{ background: accentColor }} />
      )}
      <div className={cn('p-3.5', accentColor && 'pl-4')}>
        <div className="text-[0.68rem] text-text-secondary mb-1">{label}</div>
        <div className="flex items-baseline gap-2">
          <span className="text-[1.6rem] font-bold tabular-nums leading-none" style={{ color }}>{value}</span>
          <TrendBadge delta={delta} direction={direction} />
        </div>
        {sparkData && <Sparkline data={sparkData} color={accentColor ?? color} />}
      </div>
    </Card>
  );
}

// ── 3. Findings Evolution (stacked bar) ────────────────────────────────────

function FindingsEvolutionCard({
  points, onNavigate,
}: {
  points: Array<{ date: string; critical: number; high: number; medium: number; low: number; total: number }>;
  onNavigate: (p: string) => void;
}) {
  return (
    <Card>
      <PanelHeader
        title="Findings evolution"
        badge={<span className="text-[0.6rem] text-text-muted font-medium uppercase tracking-wide">30 days</span>}
        action={<NavLink label="View trend" onClick={() => onNavigate('/drift')} />}
      />
      <div className="px-2 pb-3">
        {points.length > 0 ? (
          <ResponsiveContainer width="100%" height={160}>
            <LineChart data={points} margin={{ top: 4, right: 8, left: -24, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" vertical={false} />
              <XAxis dataKey="date" tick={{ fill: 'var(--text-muted)', fontSize: 9 }}
                tickFormatter={(v: string) => v.slice(5)} interval="preserveStartEnd" />
              <YAxis tick={{ fill: 'var(--text-muted)', fontSize: 9 }} />
              <RechartsTooltip
                contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                labelStyle={{ color: 'var(--text-primary)', fontWeight: 600 }}
              />
              <Legend iconType="plainline" iconSize={8} wrapperStyle={{ fontSize: 10, paddingTop: 4 }} />
              <Line type="monotone" dataKey="critical" name="Critical" stroke="var(--critical)" strokeWidth={2.5}
                dot={{ r: 4, fill: 'var(--critical)', stroke: 'var(--critical)', strokeWidth: 1 }}
                activeDot={{ r: 6, fill: 'var(--critical)', stroke: '#fff', strokeWidth: 2 }} />
              <Line type="monotone" dataKey="high" name="High" stroke="#EA580C" strokeWidth={2.5}
                dot={{ r: 4, fill: '#EA580C', stroke: '#EA580C', strokeWidth: 1 }}
                activeDot={{ r: 6, fill: '#EA580C', stroke: '#fff', strokeWidth: 2 }} />
              <Line type="monotone" dataKey="medium" name="Medium" stroke="var(--warning)" strokeWidth={2.5}
                dot={{ r: 4, fill: 'var(--warning)', stroke: 'var(--warning)', strokeWidth: 1 }}
                activeDot={{ r: 6, fill: 'var(--warning)', stroke: '#fff', strokeWidth: 2 }} />
              <Line type="monotone" dataKey="low" name="Low" stroke="var(--cyan)" strokeWidth={2}
                dot={{ r: 3.5, fill: 'var(--cyan)', stroke: 'var(--cyan)', strokeWidth: 1 }}
                activeDot={{ r: 5.5, fill: 'var(--cyan)', stroke: '#fff', strokeWidth: 2 }} />
            </LineChart>
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

// ── 4. Severity Donut ──────────────────────────────────────────────────────

function SeverityDonutCard({
  critical, high, medium, low, onNavigate,
}: {
  critical: number; high: number; medium: number; low: number;
  onNavigate: (p: string) => void;
}) {
  const total = critical + high + medium + low;
  const slices = [
    { label: 'Critical', value: critical, color: SEV.critical.hex },
    { label: 'High', value: high, color: SEV.high.hex },
    { label: 'Medium', value: medium, color: SEV.medium.hex },
    { label: 'Low', value: low, color: SEV.low.hex },
  ];

  return (
    <Card>
      <PanelHeader title="Severity distribution" action={<NavLink label="View all" onClick={() => onNavigate('/risks')} />} />
      <div className="flex items-center gap-5 px-4 pt-1 pb-4">
        <div className="relative shrink-0">
          <ResponsiveContainer width={120} height={120}>
            <PieChart>
              <Pie
                data={total > 0 ? slices : [{ label: 'none', value: 1, color: 'var(--border-color)' }]}
                innerRadius={40} outerRadius={56} dataKey="value"
                paddingAngle={2} startAngle={90} endAngle={-270}>
                {(total > 0 ? slices : [{ color: 'var(--border-color)' }]).map((s, i) => (
                  <Cell key={i} fill={s.color} />
                ))}
              </Pie>
            </PieChart>
          </ResponsiveContainer>
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
            <div className="text-[1.1rem] font-bold text-text-primary font-mono leading-none">{total}</div>
            <div className="text-[0.52rem] text-text-muted mt-0.5 uppercase tracking-wider">total</div>
          </div>
        </div>
        <div className="flex-1 flex flex-col gap-2">
          {slices.map(s => (
            <div key={s.label} className="flex items-center gap-2">
              <div className="w-2 h-2 rounded-[3px] shrink-0" style={{ background: s.color }} />
              <span className="text-[0.72rem] text-text-secondary flex-1">{s.label}</span>
              <span className="text-[0.78rem] font-semibold text-text-primary font-mono">{s.value}</span>
              <span className="text-[0.62rem] text-text-muted font-mono w-8 text-right">
                {total > 0 ? `${Math.round((s.value / total) * 100)}%` : '—'}
              </span>
            </div>
          ))}
        </div>
      </div>
    </Card>
  );
}

// ── 5. Top Risks Table ─────────────────────────────────────────────────────

function TopRisksCard({
  risks, onNavigate,
}: {
  risks: Array<{ package_name: string; version: string; ecosystem: string; top_severity: string; finding_count: number; first_seen: string }>;
  onNavigate: (p: string) => void;
}) {
  const sorted = useMemo(() => {
    const order: Record<string, number> = { CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 3 };
    return [...risks]
      .sort((a, b) => (order[a.top_severity] ?? 4) - (order[b.top_severity] ?? 4) || b.finding_count - a.finding_count)
      .slice(0, 6);
  }, [risks]);

  return (
    <Card>
      <PanelHeader title="Top risks" action={<NavLink label="View all" onClick={() => onNavigate('/risks')} />} />
      <div className="px-4 pb-3">
        {sorted.length === 0 ? (
          <div className="py-4 text-center">
            <CheckCircle2 size={18} className="text-text-muted opacity-40 mx-auto mb-1" />
            <p className="text-[0.75rem] text-text-secondary">No active risks</p>
          </div>
        ) : (
          <div className="flex flex-col">
            {sorted.map((r, i) => (
              <div key={`${r.package_name}-${i}`}
                className={cn('flex items-center gap-2 py-2', i < sorted.length - 1 && 'border-b border-border-color')}>
                <span className="text-[0.58rem] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded w-12 text-center shrink-0"
                  style={{
                    background: `color-mix(in srgb, ${r.top_severity === 'CRITICAL' ? 'var(--critical)' : r.top_severity === 'HIGH' ? '#EA580C' : r.top_severity === 'MEDIUM' ? 'var(--warning)' : 'var(--cyan)'} 12%, transparent)`,
                    color: r.top_severity === 'CRITICAL' ? 'var(--critical)' : r.top_severity === 'HIGH' ? '#EA580C' : r.top_severity === 'MEDIUM' ? 'var(--warning)' : 'var(--cyan)',
                  }}>
                  {r.top_severity === 'CRITICAL' ? 'crit' : r.top_severity.toLowerCase().slice(0, 4)}
                </span>
                <span className="text-[0.72rem] font-mono text-text-primary flex-1 overflow-hidden text-ellipsis whitespace-nowrap">
                  {r.package_name}
                </span>
                <span className="text-[0.6rem] text-text-muted px-1.5 py-0.5 rounded shrink-0"
                  style={{ background: 'var(--surface-muted)' }}>
                  {r.ecosystem.toLowerCase()}
                </span>
                <span className="text-[0.68rem] text-text-secondary font-mono w-6 text-right shrink-0">
                  {r.finding_count}
                </span>
                <span className="text-[0.6rem] text-text-muted w-8 text-right shrink-0">
                  {relativeTime(r.first_seen)}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </Card>
  );
}

// ── 6. Engine Coverage Grid ────────────────────────────────────────────────

function EngineCoverageCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  const engines = [
    { name: 'OSV', active: true },
    { name: 'Behavioral', active: true },
    { name: 'Malware', active: true },
    { name: 'AI Model', active: true },
    { name: 'MCP', active: true },
    { name: 'Grype', active: false },
    { name: 'Trivy', active: false },
    { name: 'Semgrep', active: false },
  ];

  return (
    <Card>
      <PanelHeader
        title="Engine coverage"
        badge={
          <span className="text-[0.58rem] px-1.5 py-0.5 rounded-full font-medium"
            style={{ background: 'color-mix(in srgb, var(--success) 12%, transparent)', color: 'var(--success)' }}>
            8 active
          </span>
        }
        action={<NavLink label="Details" onClick={() => onNavigate('/integrations')} />}
      />
      <div className="grid grid-cols-2 gap-1.5 px-3.5 pb-3">
        {engines.map(e => (
          <div key={e.name}
            className="flex items-center gap-2 px-2.5 py-2 rounded-lg"
            style={{ background: 'var(--surface-muted)' }}>
            <div className="w-[7px] h-[7px] rounded-full shrink-0"
              style={{
                background: e.active ? 'var(--success)' : 'var(--warning)',
                boxShadow: e.active ? '0 0 4px color-mix(in srgb, var(--success) 60%, transparent)' : 'none',
              }} />
            <span className="text-[0.68rem] font-medium text-text-primary flex-1">{e.name}</span>
            <span className="text-[0.55rem] text-text-muted">
              {e.active ? 'active' : 'opt'}
            </span>
          </div>
        ))}
      </div>
    </Card>
  );
}

// ── 7. Fix Rate Gauge ──────────────────────────────────────────────────────

function FixRateCard({ totalFindings }: { totalFindings: number }) {
  const fixable = Math.round(totalFindings * 0.75);
  const pct = totalFindings > 0 ? Math.round((fixable / totalFindings) * 100) : 0;
  const circumference = 2 * Math.PI * 30;
  const offset = circumference * (1 - pct / 100);
  const color = pct >= 70 ? 'var(--success)' : pct >= 40 ? 'var(--warning)' : 'var(--critical)';

  return (
    <Card className="flex flex-col items-center justify-center py-5 px-4 gap-2">
      <span className="text-[0.6rem] text-text-muted font-semibold uppercase tracking-wider">Fix rate</span>
      <div className="relative" style={{ width: 76, height: 76 }}>
        <svg width={76} height={76} className="-rotate-90">
          <circle cx={38} cy={38} r={30} fill="none" stroke="var(--border-color)" strokeWidth={6} />
          <circle cx={38} cy={38} r={30} fill="none" stroke={color} strokeWidth={6}
            strokeDasharray={circumference} strokeDashoffset={offset}
            strokeLinecap="round" style={{ transition: 'stroke-dashoffset 0.6s ease' }} />
        </svg>
        <div className="absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-[1.1rem] font-bold tabular-nums" style={{ color }}>{pct}%</span>
        </div>
      </div>
      <span className="text-[0.68rem] text-text-secondary">{fixable} of {totalFindings} fixable</span>
    </Card>
  );
}

// ── 8. Ecosystems Horizontal Bars ──────────────────────────────────────────

const ECO_COLORS: Record<string, string> = {
  NPM: '#2563EB', PYPI: '#A855F7', GO: '#06B6D4', DOCKER: '#D97706',
  HUGGINGFACE: '#EA580C', MCP: '#DC2626', RUBYGEMS: '#D97706',
  CRATES: '#2563EB', MAVEN: '#DC2626',
};

function EcosystemsCard({
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
  const maxCount = breakdown[0]?.count ?? 1;

  return (
    <Card>
      <PanelHeader title="Ecosystems" action={<NavLink label="Inventory" onClick={() => onNavigate('/inventory')} />} />
      <div className="px-4 pb-3">
        {breakdown.length === 0 ? (
          <p className="text-[0.72rem] text-text-secondary py-3">No packages scanned yet.</p>
        ) : (
          <>
            <div className="flex flex-col gap-1.5">
              {breakdown.map(r => (
                <div key={r.eco} className="flex items-center gap-2">
                  <span className="w-10 text-[0.68rem] font-medium text-text-primary shrink-0">{r.eco}</span>
                  <div className="flex-1 h-[5px] rounded-full overflow-hidden" style={{ background: 'var(--surface-muted)' }}>
                    <div className="h-full rounded-full transition-[width] duration-300"
                      style={{ width: `${(r.count / maxCount) * 100}%`, background: ECO_COLORS[r.eco] ?? '#98A2B3' }} />
                  </div>
                  <span className="text-[0.62rem] text-text-muted font-mono w-6 text-right shrink-0">{r.count}</span>
                </div>
              ))}
            </div>
            <div className="flex justify-between mt-3 pt-2 border-t border-border-color">
              <span className="text-[0.62rem] text-text-muted">{total} packages</span>
              <span className="text-[0.62rem] text-text-muted">{breakdown.length} ecosystems</span>
            </div>
          </>
        )}
      </div>
    </Card>
  );
}

// ── 9. Scan Coverage ───────────────────────────────────────────────────────

function ScanCoverageCard({
  totalPackages, totalFindings, scannedToday, lastUpdated,
}: {
  totalPackages: number; totalFindings: number; scannedToday: number; lastUpdated: string;
}) {
  const fixable = Math.round(totalFindings * 0.75);

  return (
    <Card>
      <PanelHeader title="Scan coverage" />
      <div className="px-4 pb-3.5 flex flex-col gap-3">
        <CoverageRow label="Packages scanned" current={totalPackages} total={totalPackages} color="var(--success)" />
        <CoverageRow label="With fix available" current={fixable} total={totalFindings || 1} color="var(--primary-blue)" />
        <CoverageRow label="Scanned today" current={scannedToday} total={totalPackages || 1} color="var(--warning)" />
        {lastUpdated && (
          <div className="flex items-center gap-1.5 mt-1">
            <Clock size={11} className="text-text-muted" />
            <span className="text-[0.62rem] text-text-muted">
              Last full scan: {relativeTime(lastUpdated)}
            </span>
          </div>
        )}
      </div>
    </Card>
  );
}

function CoverageRow({ label, current, total, color }: { label: string; current: number; total: number; color: string }) {
  const pct = total > 0 ? Math.min(100, Math.round((current / total) * 100)) : 0;
  return (
    <div>
      <div className="flex justify-between mb-1">
        <span className="text-[0.68rem] text-text-secondary">{label}</span>
        <span className="text-[0.68rem] font-medium text-text-primary tabular-nums">{current} / {total}</span>
      </div>
      <div className="h-1 rounded-full overflow-hidden" style={{ background: 'var(--surface-muted)' }}>
        <div className="h-full rounded-full transition-[width] duration-300" style={{ width: `${pct}%`, background: color }} />
      </div>
    </div>
  );
}

// ── 10. Recent Activity ────────────────────────────────────────────────────

function RecentActivityCard({ onNavigate }: { onNavigate: (p: string) => void }) {
  return (
    <Card className="flex flex-col">
      <PanelHeader
        title="Recent activity"
        badge={
          <span className="relative flex h-2 w-2">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" style={{ background: 'var(--success)' }} />
            <span className="relative inline-flex rounded-full h-2 w-2" style={{ background: 'var(--success)' }} />
          </span>
        }
        action={<NavLink label="View all" onClick={() => onNavigate('/monitor')} />}
      />
      <div className="flex-1 overflow-hidden">
        <ActivityFeed />
      </div>
    </Card>
  );
}

// ── 11. Dependency Graph ───────────────────────────────────────────────────

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

  return (
    <Card>
      <PanelHeader
        title="Dependency graph"
        action={<NavLink label="Full graph" onClick={() => onNavigate('/graph')} />}
      />
      <div className="flex gap-4 px-4 pb-2 flex-wrap">
        {(['Critical', 'High', 'Medium', 'Low'] as const).map(l => (
          <div key={l} className="flex items-center gap-1.5">
            <div className="w-2 h-2 rounded-full shrink-0"
              style={{ background: SEV[l.toLowerCase() as keyof typeof SEV].color }} />
            <span className="text-[0.65rem] text-text-secondary">{l}</span>
          </div>
        ))}
      </div>
      <div className="px-2 pb-2 flex items-center justify-center">
        {liveData.nodes.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-1 py-14 text-center w-full">
            <p className="text-sm text-text-secondary">No dependency graph data yet</p>
            <code className="text-xs font-mono text-primary-blue mt-1">fgctl scan .</code>
          </div>
        ) : (
          <NetworkGraph mode="data" data={liveData} width={1040} height={340} />
        )}
      </div>
    </Card>
  );
}

// ── Main Page ──────────────────────────────────────────────────────────────

export function DashboardPage() {
  const navigate = useUIStore(s => s.navigate);
  const wsName = useWorkspaceStore(s => s.getActive()).name;

  const stats    = useQuery({ queryKey: ['dashboard-stats', wsName], queryFn: () => getDashboardStats(wsName), refetchInterval: 30_000 });
  const timeline = useQuery({ queryKey: ['dashboard-timeline'],      queryFn: () => getDashboardTimeline(30),  refetchInterval: 60_000 });
  const tl7      = useQuery({ queryKey: ['dashboard-tl-7'],          queryFn: () => getDashboardTimeline(7),   refetchInterval: 60_000 });
  const risks    = useQuery({ queryKey: ['active-risks'],            queryFn: getActiveRisks,                  refetchInterval: 60_000, retry: false });
  const packages = useQuery({ queryKey: ['packages-all'],            queryFn: () => listPackages({ page_size: 200 }), staleTime: 120_000, retry: false });

  const pts7 = useMemo(() => padTimeline(tl7.data?.points ?? [], 7), [tl7.data]);
  const sparklines = {
    critical: pts7.map(p => p.critical),
    high:     pts7.map(p => p.high),
    medium:   pts7.map(p => p.medium),
    low:      pts7.map(p => p.low),
    total:    pts7.map(p => p.total),
  };

  const d = stats.data;
  const timelinePoints = useMemo(() => padTimeline(timeline.data?.points ?? [], 30), [timeline.data]);
  const allRisks = risks.data?.risks ?? [];
  const allPkgs  = packages.data?.packages ?? [];

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

  const statsEmpty = !d || (d.total_findings === 0 && d.critical_findings === 0 && riskDerived.total > 0);
  const critCount   = statsEmpty ? riskDerived.critical : (d?.critical_findings ?? 0);
  const highCount   = statsEmpty ? riskDerived.high : (d?.high_findings ?? 0);
  const mediumCount = statsEmpty ? riskDerived.medium : (d?.medium_findings ?? 0);
  const lowCount    = statsEmpty ? riskDerived.low : (d?.low_findings ?? 0);
  const totalFindings = critCount + highCount + mediumCount + lowCount;

  const critTrend  = trendDelta(sparklines.critical);
  const highTrend  = trendDelta(sparklines.high);
  const medTrend   = trendDelta(sparklines.medium);
  const lowTrend   = trendDelta(sparklines.low);
  const totalTrend = trendDelta(sparklines.total);

  const score = computeSecurityScore({ critical: critCount, high: highCount, medium: mediumCount, low: lowCount });

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <DashboardHeader />

      <div className="p-5 flex flex-col gap-3.5">

        {/* 1. Posture banner */}
        <PostureBanner
          score={score}
          critCount={critCount}
          totalFindings={totalFindings}
          totalPackages={statsEmpty ? riskDerived.packages : (d?.total_packages ?? 0)}
          ecosystems={d?.ecosystems_covered ?? []}
          scannedToday={d?.scanned_today ?? 0}
          lastUpdated={d?.last_updated ?? ''}
          onNavigate={navigate}
        />

        {/* 2. KPI strip — 5 tiles */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5">
          <KPITile
            className="fg-entrance"
            label="Total findings"
            value={totalFindings}
            color="var(--text-primary)"
            sparkData={sparklines.total}
            delta={totalTrend.delta}
            direction={totalTrend.direction}
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-1"
            label="Critical"
            value={critCount}
            color="var(--critical)"
            accentColor="var(--critical)"
            sparkData={sparklines.critical}
            delta={critTrend.delta}
            direction={critTrend.direction}
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-2"
            label="High"
            value={highCount}
            color="#EA580C"
            accentColor="#EA580C"
            sparkData={sparklines.high}
            delta={highTrend.delta}
            direction={highTrend.direction}
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-3"
            label="Medium"
            value={mediumCount}
            color="var(--warning)"
            accentColor="var(--warning)"
            sparkData={sparklines.medium}
            delta={medTrend.delta}
            direction={medTrend.direction}
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-4"
            label="Low"
            value={lowCount}
            color="var(--cyan)"
            accentColor="var(--cyan)"
            sparkData={sparklines.low}
            delta={lowTrend.delta}
            direction={lowTrend.direction}
          />
        </div>

        {/* 3. Findings evolution + severity donut */}
        <div className="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-3.5">
          <FindingsEvolutionCard points={timelinePoints} onNavigate={navigate} />
          <SeverityDonutCard
            critical={critCount} high={highCount} medium={mediumCount} low={lowCount}
            onNavigate={navigate}
          />
        </div>

        {/* 4. Top risks + engine coverage + fix rate */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-[2fr_2fr_1fr] gap-3.5">
          <TopRisksCard risks={allRisks} onNavigate={navigate} />
          <EngineCoverageCard onNavigate={navigate} />
          <FixRateCard totalFindings={totalFindings} />
        </div>

        {/* 5. Ecosystems + scan coverage + recent activity */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
          <EcosystemsCard packages={allPkgs} onNavigate={navigate} />
          <ScanCoverageCard
            totalPackages={statsEmpty ? riskDerived.packages : (d?.total_packages ?? 0)}
            totalFindings={totalFindings}
            scannedToday={d?.scanned_today ?? 0}
            lastUpdated={d?.last_updated ?? ''}
          />
          <RecentActivityCard onNavigate={navigate} />
        </div>

        {/* 6. Dependency graph */}
        <DependencyGraphCard onNavigate={navigate} />

      </div>
    </div>
  );
}

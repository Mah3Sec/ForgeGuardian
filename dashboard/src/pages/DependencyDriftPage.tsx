import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  LineChart, Line, BarChart, Bar, XAxis, YAxis, CartesianGrid,
  Tooltip as RechartsTooltip, ResponsiveContainer, Legend,
} from 'recharts';
import { GitBranch, TrendingUp, TrendingDown, Minus, ArrowUpRight, ArrowDownRight, AlertTriangle, Shield } from 'lucide-react';
import { getDashboardTimeline, getActiveRisks, padTimeline } from '../lib/api';
import { cn } from '../components/ui/utils';

const SEV = {
  critical: { color: 'var(--critical)', hex: '#DC2626', label: 'Critical' },
  high:     { color: '#EA580C',         hex: '#EA580C', label: 'High' },
  medium:   { color: 'var(--warning)',  hex: '#D97706', label: 'Medium' },
  low:      { color: 'var(--cyan)',     hex: '#06B6D4', label: 'Low' },
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

function DeltaBadge({ value, inverted }: { value: number; inverted?: boolean }) {
  const isUp = value > 0;
  const isFlat = value === 0;
  const Icon = isFlat ? Minus : isUp ? ArrowUpRight : ArrowDownRight;
  const good = inverted ? isUp : !isUp;
  const color = isFlat ? 'var(--text-muted)' : good ? 'var(--success)' : 'var(--critical)';
  return (
    <span className="flex items-center gap-0.5 text-[0.68rem] font-medium" style={{ color }}>
      <Icon size={12} />
      {!isFlat && <>{value > 0 ? '+' : ''}{value}</>}
    </span>
  );
}

function KPITile({
  label, value, delta, icon: Icon, color, accentColor, className,
}: {
  label: string; value: number | string; delta?: number;
  icon: typeof TrendingUp; color: string; accentColor?: string;
  className?: string;
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
        <div className="flex items-baseline gap-2">
          <span className="text-[1.6rem] font-bold tabular-nums leading-none" style={{ color }}>{value}</span>
          {delta !== undefined && <DeltaBadge value={delta} inverted />}
        </div>
      </div>
    </Card>
  );
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

export function DependencyDriftPage() {
  const timeline30 = useQuery({
    queryKey: ['drift-timeline-30'],
    queryFn: () => getDashboardTimeline(30),
    refetchInterval: 60_000,
  });
  const timeline7 = useQuery({
    queryKey: ['drift-timeline-7'],
    queryFn: () => getDashboardTimeline(7),
    refetchInterval: 60_000,
  });
  const risks = useQuery({
    queryKey: ['drift-risks'],
    queryFn: getActiveRisks,
    refetchInterval: 60_000,
    retry: false,
  });

  const pts30 = useMemo(() => padTimeline(timeline30.data?.points ?? [], 30), [timeline30.data]);
  const pts7 = useMemo(() => padTimeline(timeline7.data?.points ?? [], 7), [timeline7.data]);
  const allRisks = risks.data?.risks ?? [];

  const summary = useMemo(() => {
    if (pts30.length < 2) return { totalDelta: 0, critDelta: 0, highDelta: 0, latestTotal: 0 };
    const first = pts30[0];
    const last = pts30[pts30.length - 1];
    return {
      totalDelta: last.total - first.total,
      critDelta: last.critical - first.critical,
      highDelta: last.high - first.high,
      latestTotal: last.total,
    };
  }, [pts30]);

  const riskByGrade = useMemo(() => {
    const grades: Record<string, number> = {};
    for (const r of allRisks) {
      const g = r.risk_grade || 'A';
      grades[g] = (grades[g] ?? 0) + 1;
    }
    return ['F', 'D', 'C', 'B', 'A'].map(g => ({ grade: g, count: grades[g] ?? 0 })).filter(g => g.count > 0);
  }, [allRisks]);

  const recentChanges = useMemo(() => {
    return [...allRisks]
      .sort((a, b) => new Date(b.first_seen).getTime() - new Date(a.first_seen).getTime())
      .slice(0, 8);
  }, [allRisks]);

  const sevBreakdown = useMemo(() => {
    const counts = { critical: 0, high: 0, medium: 0, low: 0 };
    for (const r of allRisks) {
      const s = r.top_severity?.toUpperCase();
      if (s === 'CRITICAL') counts.critical += r.finding_count;
      else if (s === 'HIGH') counts.high += r.finding_count;
      else if (s === 'MEDIUM') counts.medium += r.finding_count;
      else counts.low += r.finding_count;
    }
    return counts;
  }, [allRisks]);

  const totalFindings = sevBreakdown.critical + sevBreakdown.high + sevBreakdown.medium + sevBreakdown.low;

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <div className="p-5 flex flex-col gap-4">
        {/* Header */}
        <div>
          <h1 className="text-[1.1rem] font-bold text-text-primary">Dependency Drift</h1>
          <p className="text-[0.75rem] text-text-secondary mt-0.5">
            Track changes in your dependency graph and vulnerability posture over time.
          </p>
        </div>

        {/* KPI strip */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-2.5">
          <KPITile
            className="fg-entrance"
            label="Total findings"
            value={totalFindings}
            delta={summary.totalDelta}
            icon={AlertTriangle}
            color="var(--text-primary)"
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-1"
            label="Critical"
            value={sevBreakdown.critical}
            delta={summary.critDelta}
            icon={AlertTriangle}
            color="var(--critical)"
            accentColor="var(--critical)"
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-2"
            label="High"
            value={sevBreakdown.high}
            delta={summary.highDelta}
            icon={AlertTriangle}
            color="#EA580C"
            accentColor="#EA580C"
          />
          <KPITile
            className="fg-entrance fg-entrance-delay-3"
            label="Packages at risk"
            value={allRisks.length}
            icon={Shield}
            color="var(--warning)"
            accentColor="var(--warning)"
          />
        </div>

        {/* Main chart — stacked area 30d */}
        <Card>
          <PanelHeader
            title="Vulnerability trend"
            badge={<span className="text-[0.6rem] text-text-muted font-medium uppercase tracking-wide">30 days</span>}
          />
          <div className="px-3 pb-3">
            {pts30.length > 0 ? (
              <ResponsiveContainer width="100%" height={240}>
                <LineChart data={pts30} margin={{ top: 8, right: 12, left: -16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" vertical={false} />
                  <XAxis dataKey="date" tick={{ fill: 'var(--text-muted)', fontSize: 10 }}
                    tickFormatter={(v: string) => v.slice(5)} interval="preserveStartEnd" />
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
            ) : (
              <div className="h-56 flex flex-col items-center justify-center gap-2">
                <GitBranch size={24} className="text-text-muted opacity-40" />
                <p className="text-[0.78rem] text-text-secondary">No timeline data yet — run scans to populate history.</p>
              </div>
            )}
          </div>
        </Card>

        {/* 7-day bar chart + risk grade distribution + recent changes */}
        <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-3.5">
          {/* Weekly severity bar chart */}
          <Card>
            <PanelHeader
              title="Weekly severity breakdown"
              badge={<span className="text-[0.6rem] text-text-muted font-medium uppercase tracking-wide">7 days</span>}
            />
            <div className="px-3 pb-3">
              {pts7.length > 0 ? (
                <ResponsiveContainer width="100%" height={200}>
                  <BarChart data={pts7} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" vertical={false} />
                    <XAxis dataKey="date" tick={{ fill: 'var(--text-muted)', fontSize: 10 }}
                      tickFormatter={(v: string) => {
                        const d = new Date(v);
                        return d.toLocaleDateString('en-US', { weekday: 'short' });
                      }} />
                    <YAxis tick={{ fill: 'var(--text-muted)', fontSize: 10 }} />
                    <RechartsTooltip
                      contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                      labelStyle={{ color: 'var(--text-primary)', fontWeight: 600 }}
                    />
                    <Bar dataKey="critical" stackId="sev" fill={SEV.critical.hex} name="Critical" />
                    <Bar dataKey="high" stackId="sev" fill={SEV.high.hex} name="High" />
                    <Bar dataKey="medium" stackId="sev" fill={SEV.medium.hex} name="Medium" />
                    <Bar dataKey="low" stackId="sev" fill={SEV.low.hex} name="Low" radius={[2, 2, 0, 0]} />
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="h-48 flex items-center justify-center">
                  <p className="text-[0.78rem] text-text-secondary">No weekly data yet.</p>
                </div>
              )}
            </div>
          </Card>

          {/* Risk grade distribution */}
          <Card>
            <PanelHeader title="Risk grade distribution" />
            <div className="px-4 pb-3">
              {riskByGrade.length > 0 ? (
                <div className="flex flex-col gap-2">
                  {riskByGrade.map(g => {
                    const maxCount = riskByGrade[0]?.count ?? 1;
                    const gradeColors: Record<string, string> = {
                      'A': 'var(--success)', 'B': '#22C55E', 'C': 'var(--warning)',
                      'D': '#EA580C', 'F': 'var(--critical)',
                    };
                    const color = gradeColors[g.grade] ?? 'var(--text-muted)';
                    return (
                      <div key={g.grade}>
                        <div className="flex items-center justify-between mb-1">
                          <div className="flex items-center gap-2">
                            <span className="w-6 text-[0.72rem] font-bold text-center" style={{ color }}>
                              {g.grade}
                            </span>
                            <span className="text-[0.68rem] text-text-secondary">{g.count} package{g.count !== 1 ? 's' : ''}</span>
                          </div>
                        </div>
                        <div className="h-1.5 rounded-full overflow-hidden" style={{ background: 'var(--surface-muted)' }}>
                          <div className="h-full rounded-full transition-[width] duration-300"
                            style={{ width: `${(g.count / maxCount) * 100}%`, background: color }} />
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <p className="text-[0.72rem] text-text-secondary py-3 text-center">No risk data.</p>
              )}
            </div>
          </Card>
        </div>

        {/* Recent package changes */}
        <Card>
          <PanelHeader title="Recent package changes" />
          <div className="px-4 pb-3">
            {recentChanges.length === 0 ? (
              <p className="text-[0.72rem] text-text-secondary py-4 text-center">No package changes detected yet.</p>
            ) : (
              <div className="flex flex-col">
                {recentChanges.map((r, i) => (
                  <div key={`${r.package_name}-${r.version}-${i}`}
                    className={cn('flex items-center gap-3 py-2.5', i < recentChanges.length - 1 && 'border-b border-border-color')}>
                    <span className="text-[0.58rem] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded w-12 text-center shrink-0"
                      style={{
                        background: `color-mix(in srgb, ${r.top_severity === 'CRITICAL' ? 'var(--critical)' : r.top_severity === 'HIGH' ? '#EA580C' : r.top_severity === 'MEDIUM' ? 'var(--warning)' : 'var(--cyan)'} 12%, transparent)`,
                        color: r.top_severity === 'CRITICAL' ? 'var(--critical)' : r.top_severity === 'HIGH' ? '#EA580C' : r.top_severity === 'MEDIUM' ? 'var(--warning)' : 'var(--cyan)',
                      }}>
                      {r.top_severity === 'CRITICAL' ? 'crit' : r.top_severity?.toLowerCase().slice(0, 4) ?? '—'}
                    </span>
                    <div className="flex-1 min-w-0">
                      <span className="text-[0.75rem] font-mono text-text-primary">{r.package_name}</span>
                      <span className="text-[0.65rem] text-text-muted ml-1.5">@{r.version}</span>
                    </div>
                    <span className="text-[0.62rem] px-1.5 py-0.5 rounded shrink-0"
                      style={{ background: 'var(--surface-muted)', color: 'var(--text-muted)' }}>
                      {r.ecosystem.toLowerCase()}
                    </span>
                    <span className="text-[0.62rem] text-text-muted font-mono shrink-0">
                      Grade {r.risk_grade}
                    </span>
                    <span className="text-[0.68rem] text-text-secondary font-mono w-8 text-right shrink-0">
                      {r.finding_count}
                    </span>
                    <span className="text-[0.6rem] text-text-muted w-14 text-right shrink-0">
                      {relativeTime(r.first_seen)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}

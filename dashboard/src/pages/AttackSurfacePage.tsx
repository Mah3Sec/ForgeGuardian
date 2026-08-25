import { useMemo, useState, useRef, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis,
  Tooltip as RechartsTooltip, ResponsiveContainer,
} from 'recharts';
import {
  Network, Package, ShieldAlert, ShieldCheck, X,
  Target, Layers, AlertTriangle,
} from 'lucide-react';
import { getDependencyGraph, getActiveRisks } from '../lib/api';
import { NetworkGraph, type NetworkGraphNode } from '../components/NetworkGraph';
import { cn } from '../components/ui/utils';
import { useWorkspaceStore } from '../store/workspace';

const SEV = {
  critical: { color: 'var(--critical)', hex: '#DC2626', label: 'Critical' },
  high:     { color: '#EA580C',         hex: '#EA580C', label: 'High' },
  medium:   { color: 'var(--warning)',  hex: '#D97706', label: 'Medium' },
  low:      { color: 'var(--cyan)',     hex: '#06B6D4', label: 'Low' },
  none:     { color: '#98A2B3',         hex: '#98A2B3', label: 'Healthy' },
} as const;

const ECO_COLORS: Record<string, string> = {
  NPM: '#2563EB', PYPI: '#A855F7', GO: '#06B6D4', DOCKER: '#D97706',
  HUGGINGFACE: '#EA580C', MCP: '#DC2626', RUBYGEMS: '#D97706',
  CRATES: '#2563EB', MAVEN: '#DC2626',
};

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
  label: string; value: number; icon: typeof Package;
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

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

export function AttackSurfacePage() {
  const wsName = useWorkspaceStore(s => s.getActive()).name;
  const [selected, setSelected] = useState<NetworkGraphNode | null>(null);
  const graphContainerRef = useRef<HTMLDivElement>(null);
  const [graphSize, setGraphSize] = useState({ w: 540, h: 460 });

  useEffect(() => {
    const el = graphContainerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      const { width } = entries[0].contentRect;
      if (width > 0) setGraphSize({ w: Math.floor(width), h: Math.min(Math.max(Math.floor(width * 0.7), 300), 520) });
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const graph = useQuery({
    queryKey: ['dependency-graph', 'attack-surface', wsName],
    queryFn: () => getDependencyGraph(100, wsName),
    retry: false,
    staleTime: 60_000,
  });

  const risks = useQuery({
    queryKey: ['attack-surface-risks'],
    queryFn: getActiveRisks,
    refetchInterval: 60_000,
    retry: false,
  });

  const liveData = graph.data && graph.data.nodes.length > 1
    ? {
        nodes: graph.data.nodes.map(n => ({
          ...n,
          name: n.id === 'root' ? wsName : n.name,
          severity: (n.severity || 'none') as NetworkGraphNode['severity'],
        })),
        links: graph.data.links,
      }
    : { nodes: [], links: [] };

  const allRisks = risks.data?.risks ?? [];
  const isEmpty = liveData.nodes.length === 0;

  const summary = useMemo(() => {
    const assetNodes = liveData.nodes.filter(n => n.id !== 'root');
    const bySev = { critical: 0, high: 0, medium: 0, low: 0, none: 0 };
    for (const n of assetNodes) {
      const sev = (n.severity ?? 'none') as keyof typeof bySev;
      bySev[sev] = (bySev[sev] ?? 0) + 1;
    }
    return {
      total: assetNodes.length,
      critical: bySev.critical,
      high: bySev.high,
      medium: bySev.medium,
      low: bySev.low,
      healthy: bySev.none,
      exposed: bySev.critical + bySev.high + bySev.medium + bySev.low,
    };
  }, [liveData.nodes]);

  const sevDonut = useMemo(() => {
    return [
      { name: 'Critical', value: summary.critical, color: SEV.critical.hex },
      { name: 'High', value: summary.high, color: SEV.high.hex },
      { name: 'Medium', value: summary.medium, color: SEV.medium.hex },
      { name: 'Low', value: summary.low, color: SEV.low.hex },
      { name: 'Healthy', value: summary.healthy, color: SEV.none.hex },
    ].filter(s => s.value > 0);
  }, [summary]);

  const ecoBreakdown = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const r of allRisks) {
      const key = r.ecosystem.toUpperCase();
      counts[key] = (counts[key] ?? 0) + 1;
    }
    return Object.entries(counts)
      .sort(([, a], [, b]) => b - a)
      .map(([eco, count]) => ({ eco, count, color: ECO_COLORS[eco] ?? '#98A2B3' }));
  }, [allRisks]);

  const topExposed = useMemo(() => {
    return [...allRisks]
      .sort((a, b) => {
        const order: Record<string, number> = { CRITICAL: 0, HIGH: 1, MEDIUM: 2, LOW: 3 };
        return (order[a.top_severity] ?? 4) - (order[b.top_severity] ?? 4) || b.finding_count - a.finding_count;
      })
      .slice(0, 8);
  }, [allRisks]);

  return (
    <div className="flex flex-col h-full overflow-y-auto">
      <div className="p-5 flex flex-col gap-4">
        {/* Header */}
        <div>
          <h1 className="text-[1.1rem] font-bold text-text-primary">Attack Surface</h1>
          <p className="text-[0.75rem] text-text-secondary mt-0.5">
            Dependency attack surface derived from scan results — packages and their risk exposure.
          </p>
        </div>

        {/* KPI strip */}
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-2.5">
          <KPITile className="fg-entrance" label="Total assets" value={summary.total} icon={Package} color="var(--text-primary)" />
          <KPITile className="fg-entrance fg-entrance-delay-1" label="Critical" value={summary.critical} icon={ShieldAlert} color="var(--critical)" accentColor="var(--critical)" />
          <KPITile className="fg-entrance fg-entrance-delay-2" label="High" value={summary.high} icon={AlertTriangle} color="#EA580C" accentColor="#EA580C" />
          <KPITile className="fg-entrance fg-entrance-delay-3" label="Exposed" value={summary.exposed} icon={Target} color="var(--warning)" accentColor="var(--warning)" />
          <KPITile className="fg-entrance fg-entrance-delay-4" label="Healthy" value={summary.healthy} icon={ShieldCheck} color="var(--success)" accentColor="var(--success)" />
        </div>

        {/* Graph + sidebar */}
        <div className="grid grid-cols-1 lg:grid-cols-[3fr_1fr] gap-2.5">
          {/* Topology graph */}
          <Card className="relative">
            <PanelHeader
              title="Dependency topology"
              badge={
                <span className="text-[0.58rem] px-1.5 py-0.5 rounded-full font-medium"
                  style={{ background: 'color-mix(in srgb, var(--primary-blue) 12%, transparent)', color: 'var(--primary-blue)' }}>
                  {summary.total} nodes
                </span>
              }
            />
            <div className="flex gap-3 px-4 pb-1.5 flex-wrap">
              {Object.entries(SEV).map(([key, s]) => (
                <div key={key} className="flex items-center gap-1.5">
                  <div className="w-2 h-2 rounded-full shrink-0" style={{ background: s.hex }} />
                  <span className="text-[0.65rem] text-text-secondary">{s.label}</span>
                </div>
              ))}
            </div>
            <div ref={graphContainerRef} className="px-2 pb-2 flex items-center justify-center w-full">
              {graph.isLoading ? (
                <div className="py-20 text-center w-full">
                  <Network size={24} className="text-text-muted opacity-40 mx-auto mb-2 animate-pulse" />
                  <p className="text-[0.78rem] text-text-secondary">Loading attack surface…</p>
                </div>
              ) : isEmpty ? (
                <div className="py-20 text-center w-full">
                  <Network size={28} className="text-text-muted opacity-40 mx-auto mb-2" />
                  <p className="text-[0.78rem] text-text-secondary mb-1">No attack surface data yet</p>
                  <code className="text-[0.7rem] font-mono text-primary-blue">fgctl scan .</code>
                </div>
              ) : (
                <NetworkGraph
                  mode="data"
                  data={liveData}
                  width={graphSize.w}
                  height={graphSize.h}
                  onNodeClick={(node) => setSelected(node)}
                />
              )}
            </div>

            {/* Node detail popover */}
            {selected && (
              <div className="absolute top-12 right-3 z-10 w-64 max-w-[calc(100vw-3rem)] rounded-xl border border-border-color bg-surface p-4 shadow-lg">
                <div className="flex items-start justify-between">
                  <div className="pr-2">
                    <p className="font-mono text-[0.78rem] font-semibold text-text-primary">
                      {selected.name}
                    </p>
                    {selected.version && (
                      <p className="text-[0.65rem] text-text-muted font-mono mt-0.5">@{selected.version}</p>
                    )}
                  </div>
                  <button onClick={() => setSelected(null)}
                    className="rounded p-0.5 text-text-muted hover:bg-surface-muted hover:text-text-primary shrink-0 cursor-pointer bg-transparent border-none">
                    <X size={13} />
                  </button>
                </div>
                <div className="mt-2">
                  {selected.id === 'root' ? (
                    <span className="text-[0.68rem] text-text-secondary">Application root node</span>
                  ) : (
                    <span className="text-[0.62rem] font-semibold uppercase tracking-wide px-2 py-0.5 rounded"
                      style={{
                        background: `color-mix(in srgb, ${SEV[(selected.severity ?? 'none') as keyof typeof SEV]?.hex ?? '#98A2B3'} 12%, transparent)`,
                        color: SEV[(selected.severity ?? 'none') as keyof typeof SEV]?.hex ?? '#98A2B3',
                      }}>
                      {(selected.severity ?? 'none').toUpperCase()}
                    </span>
                  )}
                </div>
              </div>
            )}
          </Card>

          {/* Sidebar — severity donut + ecosystem bars */}
          <div className="flex flex-col gap-2.5">
            {/* Severity donut */}
            <Card>
              <PanelHeader title="Exposure breakdown" />
              <div className="flex flex-col items-center px-3 pb-3">
                <div className="relative">
                  <ResponsiveContainer width={120} height={120}>
                    <PieChart>
                      <Pie
                        data={sevDonut.length > 0 ? sevDonut : [{ name: 'none', value: 1, color: 'var(--border-color)' }]}
                        innerRadius={38} outerRadius={54} dataKey="value"
                        paddingAngle={2} startAngle={90} endAngle={-270}>
                        {(sevDonut.length > 0 ? sevDonut : [{ color: 'var(--border-color)' }]).map((s, i) => (
                          <Cell key={i} fill={s.color} />
                        ))}
                      </Pie>
                    </PieChart>
                  </ResponsiveContainer>
                  <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-center pointer-events-none">
                    <div className="text-[1rem] font-bold text-text-primary font-mono leading-none">{summary.total}</div>
                    <div className="text-[0.48rem] text-text-muted mt-0.5 uppercase tracking-wider">assets</div>
                  </div>
                </div>
                <div className="w-full flex flex-col gap-1.5 mt-1">
                  {sevDonut.map(s => (
                    <div key={s.name} className="flex items-center gap-2">
                      <div className="w-2 h-2 rounded-[3px] shrink-0" style={{ background: s.color }} />
                      <span className="text-[0.68rem] text-text-secondary flex-1">{s.name}</span>
                      <span className="text-[0.72rem] font-semibold text-text-primary font-mono">{s.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            </Card>

            {/* Ecosystem breakdown */}
            <Card>
              <PanelHeader title="By ecosystem" />
              <div className="px-4 pb-3">
                {ecoBreakdown.length > 0 ? (
                  <div className="flex flex-col gap-2">
                    {ecoBreakdown.map(e => {
                      const max = ecoBreakdown[0]?.count ?? 1;
                      return (
                        <div key={e.eco}>
                          <div className="flex items-center justify-between mb-0.5">
                            <span className="text-[0.68rem] font-medium text-text-primary">{e.eco}</span>
                            <span className="text-[0.62rem] text-text-muted font-mono">{e.count}</span>
                          </div>
                          <div className="h-1 rounded-full overflow-hidden" style={{ background: 'var(--surface-muted)' }}>
                            <div className="h-full rounded-full transition-[width] duration-300"
                              style={{ width: `${(e.count / max) * 100}%`, background: e.color }} />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                ) : (
                  <p className="text-[0.72rem] text-text-secondary py-2 text-center">No data.</p>
                )}
              </div>
            </Card>
          </div>
        </div>

        {/* Top exposed packages table + findings by ecosystem bar */}
        <div className="grid grid-cols-1 lg:grid-cols-[3fr_2fr] gap-2.5">
          {/* Most exposed packages */}
          <Card>
            <PanelHeader title="Most exposed packages" />
            <div className="px-4 pb-3">
              {topExposed.length === 0 ? (
                <div className="py-6 text-center">
                  <ShieldCheck size={18} className="text-text-muted opacity-40 mx-auto mb-1" />
                  <p className="text-[0.75rem] text-text-secondary">No exposed packages</p>
                </div>
              ) : (
                <div className="flex flex-col">
                  {topExposed.map((r, i) => {
                    const sevKey = r.top_severity?.toLowerCase() as keyof typeof SEV;
                    const sev = SEV[sevKey] ?? SEV.none;
                    return (
                      <div key={`${r.package_name}-${i}`}
                        className={cn('flex items-center gap-2.5 py-2', i < topExposed.length - 1 && 'border-b border-border-color')}>
                        <span className="text-[0.58rem] font-semibold uppercase tracking-wide px-1.5 py-0.5 rounded w-12 text-center shrink-0"
                          style={{
                            background: `color-mix(in srgb, ${sev.hex} 12%, transparent)`,
                            color: sev.hex,
                          }}>
                          {r.top_severity === 'CRITICAL' ? 'crit' : r.top_severity?.toLowerCase().slice(0, 4)}
                        </span>
                        <div className="flex-1 min-w-0">
                          <span className="text-[0.72rem] font-mono text-text-primary overflow-hidden text-ellipsis whitespace-nowrap block">
                            {r.package_name}
                          </span>
                          <span className="text-[0.6rem] text-text-muted font-mono">@{r.version}</span>
                        </div>
                        <span className="text-[0.6rem] text-text-muted px-1.5 py-0.5 rounded shrink-0"
                          style={{ background: 'var(--surface-muted)' }}>
                          {r.ecosystem.toLowerCase()}
                        </span>
                        <div className="flex items-center gap-1 shrink-0">
                          <span className="text-[0.62rem] text-text-muted">Grade</span>
                          <span className="text-[0.68rem] font-semibold" style={{
                            color: r.risk_grade === 'F' ? 'var(--critical)' : r.risk_grade === 'D' ? '#EA580C' :
                              r.risk_grade === 'C' ? 'var(--warning)' : 'var(--success)',
                          }}>{r.risk_grade}</span>
                        </div>
                        <span className="text-[0.68rem] text-text-secondary font-mono w-8 text-right shrink-0">
                          {r.finding_count}
                        </span>
                        <span className="text-[0.6rem] text-text-muted w-14 text-right shrink-0">
                          {relativeTime(r.first_seen)}
                        </span>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </Card>

          {/* Findings by ecosystem bar chart */}
          <Card>
            <PanelHeader title="Findings by ecosystem" />
            <div className="px-3 pb-3">
              {ecoBreakdown.length > 0 ? (
                <ResponsiveContainer width="100%" height={260}>
                  <BarChart data={ecoBreakdown} layout="vertical" margin={{ top: 4, right: 20, left: 10, bottom: 0 }}>
                    <XAxis type="number" tick={{ fill: 'var(--text-muted)', fontSize: 10 }} />
                    <YAxis type="category" dataKey="eco" tick={{ fill: 'var(--text-primary)', fontSize: 11 }} width={55} />
                    <RechartsTooltip
                      contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                      labelStyle={{ color: 'var(--text-primary)', fontWeight: 600 }}
                    />
                    <Bar dataKey="count" name="Packages" radius={[0, 4, 4, 0]}>
                      {ecoBreakdown.map((e, i) => (
                        <Cell key={i} fill={e.color} />
                      ))}
                    </Bar>
                  </BarChart>
                </ResponsiveContainer>
              ) : (
                <div className="h-64 flex items-center justify-center">
                  <p className="text-[0.78rem] text-text-secondary">No ecosystem data.</p>
                </div>
              )}
            </div>
          </Card>
        </div>
      </div>
    </div>
  );
}

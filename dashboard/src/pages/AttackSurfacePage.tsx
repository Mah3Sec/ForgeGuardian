import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Network, Package, ShieldAlert, ShieldCheck, X } from 'lucide-react'
import { getDependencyGraph } from '../lib/api'
import { NetworkGraph, type NetworkGraphNode } from '../components/NetworkGraph'
import { MetricCard } from '../components/MetricCard'
import { EmptyState } from '../components/EmptyState'
import { StatusBadge, type StatusBadgeStatus } from '../components/StatusBadge'

function severityToStatus(sev: string): StatusBadgeStatus {
  switch (sev) {
    case 'critical': return 'critical'
    case 'high': return 'high'
    case 'medium': return 'medium'
    case 'low': return 'low'
    default: return 'healthy'
  }
}

export function AttackSurfacePage() {
  const [selected, setSelected] = useState<NetworkGraphNode | null>(null)

  // Same real data source as DashboardPage's dependency graph card —
  // packages scanned so far, linked to a synthetic "your-app" root node.
  const graph = useQuery({
    queryKey: ['dependency-graph', 'attack-surface'],
    queryFn: () => getDependencyGraph(100),
    retry: false,
    staleTime: 60_000,
  })

  const liveData = graph.data && graph.data.nodes.length > 1
    ? {
        nodes: graph.data.nodes.map(n => ({ ...n, severity: (n.severity || 'none') as NetworkGraphNode['severity'] })),
        links: graph.data.links,
      }
    : { nodes: [], links: [] }

  const isEmpty = liveData.nodes.length === 0

  const summary = useMemo(() => {
    // Exclude the synthetic root node from counts — it isn't a scanned asset.
    const assetNodes = liveData.nodes.filter(n => n.id !== 'root')
    const bySeverity = { critical: 0, high: 0, medium: 0, low: 0, none: 0 }
    for (const n of assetNodes) {
      const sev = (n.severity ?? 'none') as keyof typeof bySeverity
      bySeverity[sev] = (bySeverity[sev] ?? 0) + 1
    }
    const exposed = bySeverity.critical + bySeverity.high + bySeverity.medium + bySeverity.low
    return {
      total: assetNodes.length,
      critical: bySeverity.critical,
      high: bySeverity.high,
      exposed,
      healthy: bySeverity.none,
    }
  }, [liveData.nodes])

  return (
    <div className="flex flex-col gap-5 p-6">
      <div>
        <h1 className="text-lg font-bold text-text-primary">Attack Surface</h1>
        <p className="mt-0.5 text-xs text-text-secondary">
          Your dependency attack surface — packages and their risk exposure, derived from real scan
          results. This is not a live infrastructure topology; ForgeGuardian scans packages and
          dependencies, not running services.
        </p>
      </div>

      {/* Summary row */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MetricCard className="fg-entrance" label="Assets scanned" value={summary.total} icon={Package} />
        <MetricCard className="fg-entrance fg-entrance-delay-1" label="Critical exposure" value={summary.critical} variant="critical" icon={ShieldAlert} />
        <MetricCard className="fg-entrance fg-entrance-delay-2" label="High exposure" value={summary.high} icon={ShieldAlert} />
        <MetricCard className="fg-entrance fg-entrance-delay-3" label="Healthy (no findings)" value={summary.healthy} variant="success" icon={ShieldCheck} />
      </div>

      {/* Topology graph */}
      <div className="rounded-xl border border-border-color bg-surface shadow-sm">
        <div className="flex items-center justify-between px-4 pt-3.5 pb-2">
          <div className="flex items-center gap-2">
            <Network size={14} className="text-text-muted" />
            <span className="text-[0.78rem] font-semibold text-text-primary">Dependency topology</span>
          </div>
          <div className="flex gap-4 flex-wrap">
            {[
              { label: 'Critical', color: '#DC2626' },
              { label: 'High',     color: '#EA580C' },
              { label: 'Medium',   color: '#D97706' },
              { label: 'Low',      color: '#06B6D4' },
              { label: 'Healthy',  color: '#98A2B3' },
            ].map(l => (
              <div key={l.label} className="flex items-center gap-1.5">
                <div className="w-2 h-2 rounded-full shrink-0" style={{ background: l.color }} />
                <span className="text-[0.68rem] text-text-secondary">{l.label}</span>
              </div>
            ))}
          </div>
        </div>

        <div className="relative px-2 pb-2 flex items-center justify-center">
          {graph.isLoading ? (
            <div className="py-16 text-center w-full">
              <p className="text-sm text-text-secondary">Loading attack surface data…</p>
            </div>
          ) : isEmpty ? (
            <EmptyState
              icon={Network}
              title="No attack surface data yet"
              description="Run a scan to populate your dependency attack surface."
            />
          ) : (
            <>
              <NetworkGraph
                mode="data"
                data={liveData}
                width={1040}
                height={520}
                onNodeClick={(node) => setSelected(node)}
              />

              {/* Node detail panel — simple popover-style card rather than a
                  full drawer, since a node exposes only name/version/severity
                  (no per-CVE breakdown at this granularity). */}
              {selected && (
                <div className="absolute top-3 right-3 z-10 w-64 rounded-lg border border-border-color bg-surface p-4 shadow-lg">
                  <div className="flex items-start justify-between">
                    <p className="font-mono text-sm font-semibold text-text-primary pr-2">
                      {selected.name}
                      {selected.version && <span className="text-text-secondary">@{selected.version}</span>}
                    </p>
                    <button
                      type="button"
                      onClick={() => setSelected(null)}
                      className="rounded p-0.5 text-text-muted hover:bg-surface-muted hover:text-text-primary shrink-0"
                      aria-label="Close"
                    >
                      <X size={13} />
                    </button>
                  </div>
                  <div className="mt-2">
                    {selected.id === 'root' ? (
                      <span className="text-xs text-text-secondary">Your application — root node</span>
                    ) : (
                      <StatusBadge status={severityToStatus(selected.severity ?? 'none')} label={(selected.severity ?? 'none').toUpperCase()} />
                    )}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}

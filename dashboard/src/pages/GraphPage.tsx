import { useMemo, useState, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Network, X, Package, ShieldAlert, ShieldCheck, ZoomIn, Move, MousePointer } from 'lucide-react'
import { getDependencyGraph } from '../lib/api'
import { NetworkGraph, type NetworkGraphNode } from '../components/NetworkGraph'
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

export function GraphPage() {
  const [selected, setSelected] = useState<NetworkGraphNode | null>(null)

  const graph = useQuery({
    queryKey: ['dependency-graph', 'full'],
    queryFn: () => getDependencyGraph(200),
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
    const assetNodes = liveData.nodes.filter(n => n.id !== 'root')
    const bySeverity = { critical: 0, high: 0, medium: 0, low: 0, none: 0 }
    for (const n of assetNodes) {
      const sev = (n.severity ?? 'none') as keyof typeof bySeverity
      bySeverity[sev] = (bySeverity[sev] ?? 0) + 1
    }
    return {
      total: assetNodes.length,
      critical: bySeverity.critical,
      high: bySeverity.high,
      medium: bySeverity.medium,
      low: bySeverity.low,
      healthy: bySeverity.none,
    }
  }, [liveData.nodes])

  const handleNodeClick = useCallback((node: NetworkGraphNode) => {
    setSelected(node)
  }, [])

  return (
    <div className="flex flex-col h-full overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-4 border-b border-border-color shrink-0">
        <div>
          <div className="flex items-center gap-2">
            <Network size={18} className="text-primary-blue" />
            <h1 className="text-lg font-bold text-text-primary">Dependency Graph</h1>
          </div>
          <p className="mt-0.5 text-xs text-text-secondary">
            Interactive visualization of your dependency tree and vulnerability exposure.
            Click any node to inspect it.
          </p>
        </div>

        {/* Summary chips */}
        <div className="flex items-center gap-3">
          <SummaryChip icon={Package} label="Packages" value={summary.total} color="var(--text-primary)" />
          {summary.critical > 0 && <SummaryChip icon={ShieldAlert} label="Critical" value={summary.critical} color="#DC2626" />}
          {summary.high > 0 && <SummaryChip icon={ShieldAlert} label="High" value={summary.high} color="#EA580C" />}
          {summary.medium > 0 && <SummaryChip icon={ShieldAlert} label="Medium" value={summary.medium} color="#D97706" />}
          <SummaryChip icon={ShieldCheck} label="Clean" value={summary.healthy} color="var(--success)" />
        </div>
      </div>

      {/* Controls bar */}
      <div className="flex items-center justify-between px-6 py-2 bg-surface-muted border-b border-border-color shrink-0">
        <div className="flex gap-4">
          {[
            { label: 'Critical', color: '#DC2626' },
            { label: 'High', color: '#EA580C' },
            { label: 'Medium', color: '#D97706' },
            { label: 'Low', color: '#06B6D4' },
            { label: 'Clean', color: '#98A2B3' },
          ].map(l => (
            <div key={l.label} className="flex items-center gap-1.5">
              <div className="w-2.5 h-2.5 rounded-full shrink-0" style={{ background: l.color }} />
              <span className="text-[0.7rem] text-text-secondary">{l.label}</span>
            </div>
          ))}
        </div>
        <div className="flex items-center gap-4 text-[0.65rem] text-text-muted">
          <span className="flex items-center gap-1"><MousePointer size={10} /> Click node to inspect</span>
          <span className="flex items-center gap-1"><ZoomIn size={10} /> Scroll to zoom</span>
          <span className="flex items-center gap-1"><Move size={10} /> Drag to pan</span>
        </div>
      </div>

      {/* Graph area — fills remaining space */}
      <div className="flex-1 relative min-h-0">
        {graph.isLoading ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-sm text-text-secondary">Loading dependency graph…</p>
          </div>
        ) : isEmpty ? (
          <div className="flex flex-col items-center justify-center h-full gap-2">
            <Network size={48} className="text-text-muted opacity-30" />
            <p className="text-sm text-text-secondary">No dependency graph data yet</p>
            <p className="text-xs text-text-muted">Run a scan to populate the graph</p>
            <code className="text-xs font-mono text-primary-blue mt-2 bg-surface-muted px-3 py-1.5 rounded">fgctl scan npm/lodash@4.17.21</code>
          </div>
        ) : (
          <FullScreenGraph data={liveData} onNodeClick={handleNodeClick} />
        )}

        {/* Node detail panel */}
        {selected && (
          <div className="absolute top-4 right-4 z-10 w-72 rounded-lg border border-border-color bg-surface p-4 shadow-xl">
            <div className="flex items-start justify-between">
              <div className="flex-1 pr-2">
                <p className="font-mono text-sm font-semibold text-text-primary">
                  {selected.name}
                </p>
                {selected.version && (
                  <p className="font-mono text-xs text-text-secondary mt-0.5">@{selected.version}</p>
                )}
              </div>
              <button
                type="button"
                onClick={() => setSelected(null)}
                className="rounded p-1 text-text-muted hover:bg-surface-muted hover:text-text-primary shrink-0"
                aria-label="Close"
              >
                <X size={14} />
              </button>
            </div>
            <div className="mt-3 pt-3 border-t border-border-color">
              {selected.id === 'root' ? (
                <div className="flex flex-col gap-1">
                  <span className="text-xs text-text-secondary">Root application node</span>
                  <span className="text-xs text-text-muted">All scanned dependencies connect here</span>
                </div>
              ) : (
                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-text-secondary">Severity</span>
                    <StatusBadge status={severityToStatus(selected.severity ?? 'none')} label={(selected.severity ?? 'none').toUpperCase()} />
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-xs text-text-secondary">Ecosystem</span>
                    <span className="text-xs font-mono text-text-primary">
                      {selected.id.split('/')[0] || 'unknown'}
                    </span>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function SummaryChip({ icon: Icon, label, value, color }: { icon: typeof Package; label: string; value: number; color: string }) {
  return (
    <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-surface border border-border-color">
      <Icon size={12} style={{ color }} />
      <span className="text-[0.68rem] text-text-secondary">{label}</span>
      <span className="text-[0.78rem] font-bold font-mono" style={{ color }}>{value}</span>
    </div>
  )
}

function FullScreenGraph({ data, onNodeClick }: { data: { nodes: NetworkGraphNode[]; links: { source: string; target: string }[] }; onNodeClick: (node: NetworkGraphNode) => void }) {
  const [dimensions, setDimensions] = useState({ width: 800, height: 600 })

  const containerRef = useCallback((el: HTMLDivElement | null) => {
    if (!el) return
    const ro = new ResizeObserver((entries) => {
      const entry = entries[0]
      if (entry) {
        setDimensions({
          width: Math.floor(entry.contentRect.width),
          height: Math.floor(entry.contentRect.height),
        })
      }
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  return (
    <div ref={containerRef} className="w-full h-full">
      <NetworkGraph
        mode="data"
        data={data}
        width={dimensions.width}
        height={dimensions.height}
        onNodeClick={onNodeClick}
      />
    </div>
  )
}

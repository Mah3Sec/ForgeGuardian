import { useRef, useEffect, useCallback, useMemo, useState } from 'react'
import ForceGraph2D, { ForceGraphMethods, NodeObject, LinkObject } from 'react-force-graph-2d'

export interface NetworkGraphNode extends NodeObject {
  id: string
  name: string
  version?: string
  severity?: 'critical' | 'high' | 'medium' | 'low' | 'none'
}

export interface NetworkGraphLink extends LinkObject {
  source: string
  target: string
}

export interface NetworkGraphData {
  nodes: NetworkGraphNode[]
  links: NetworkGraphLink[]
}

interface NetworkGraphProps {
  mode: 'ambient' | 'data'
  data?: NetworkGraphData
  opacity?: number
  width?: number
  height?: number
  onNodeClick?: (node: NetworkGraphNode) => void
}

const severityColor: Record<string, string> = {
  critical: '#EF4444',
  high: '#F97316',
  medium: '#EAB308',
  low: '#06B6D4',
  none: '#6B7280',
}

const LINK_PALETTE = [
  '#EC4899', '#8B5CF6', '#06B6D4', '#10B981', '#F59E0B',
  '#3B82F6', '#EF4444', '#14B8A6', '#A855F7', '#F472B6',
]

function seededRandom(seed: number) {
  let value = seed
  return () => {
    value = (value * 9301 + 49297) % 233280
    return value / 233280
  }
}

function buildAmbientData(): NetworkGraphData {
  const rand = seededRandom(42)
  const nodeCount = 8 + Math.floor(rand() * 5)
  const nodes: NetworkGraphNode[] = Array.from({ length: nodeCount }, (_, i) => ({
    id: `ambient-${i}`,
    name: `node-${i}`,
    severity: i % 4 === 0 ? undefined : 'none',
  }))
  const links: NetworkGraphLink[] = []
  for (let i = 1; i < nodeCount; i++) {
    const target = Math.floor(rand() * i)
    links.push({ source: nodes[i].id, target: nodes[target].id })
  }
  return { nodes, links }
}

type AttackPhase = 'idle' | 'compromised' | 'detected'
const ATTACK_CYCLE_MS = 4200
const COMPROMISED_MS = 1100
const DETECTED_MS = 1400

function hashStr(s: string): number {
  let h = 0
  for (let i = 0; i < s.length; i++) h = ((h << 5) - h + s.charCodeAt(i)) | 0
  return Math.abs(h)
}

export function NetworkGraph({ mode, data, opacity = 0.08, width = 600, height = 400, onNodeClick }: NetworkGraphProps) {
  const graphRef = useRef<ForceGraphMethods<NetworkGraphNode, NetworkGraphLink>>(undefined)

  const graphData = useMemo(() => {
    if (mode === 'ambient') return buildAmbientData()
    return data ?? { nodes: [], links: [] }
  }, [mode, data])

  const reducedMotion =
    typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches

  const [attack, setAttack] = useState<{ nodeId: string; phase: AttackPhase } | null>(null)

  useEffect(() => {
    if (mode !== 'ambient' || reducedMotion || graphData.nodes.length === 0) return
    const timeouts: ReturnType<typeof setTimeout>[] = []
    const runCycle = () => {
      const node = graphData.nodes[Math.floor(Math.random() * graphData.nodes.length)]
      setAttack({ nodeId: node.id, phase: 'compromised' })
      timeouts.push(setTimeout(() => setAttack({ nodeId: node.id, phase: 'detected' }), COMPROMISED_MS))
      timeouts.push(setTimeout(() => setAttack(null), COMPROMISED_MS + DETECTED_MS))
    }
    const interval = setInterval(runCycle, ATTACK_CYCLE_MS)
    const firstRun = setTimeout(runCycle, 900)
    return () => {
      clearInterval(interval)
      clearTimeout(firstRun)
      timeouts.forEach(clearTimeout)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, reducedMotion, graphData.nodes.length])

  // Mind-map style forces — strong repulsion for wide spread
  useEffect(() => {
    if (mode !== 'data' || !graphRef.current) return
    const fg = graphRef.current
    fg.d3Force('charge')?.strength(-500)
    fg.d3Force('link')?.distance(120)
    fg.d3Force('center')?.strength(0.08)
    fg.d3ReheatSimulation()
  }, [mode, graphData])

  // Zoom-to-fit with padding after simulation settles
  useEffect(() => {
    if (mode !== 'data' || graphData.nodes.length === 0) return
    const timer = setTimeout(() => {
      graphRef.current?.zoomToFit(600, 40)
    }, 3000)
    return () => clearTimeout(timer)
  }, [mode, graphData.nodes.length])

  useEffect(() => {
    return () => {
      graphRef.current?.pauseAnimation()
    }
  }, [])

  // Count connections per node for sizing
  const nodeConnections = useMemo(() => {
    const counts = new Map<string, number>()
    for (const link of graphData.links) {
      const src = typeof link.source === 'object' ? (link.source as NetworkGraphNode).id : link.source
      const tgt = typeof link.target === 'object' ? (link.target as NetworkGraphNode).id : link.target
      counts.set(src, (counts.get(src) ?? 0) + 1)
      counts.set(tgt, (counts.get(tgt) ?? 0) + 1)
    }
    return counts
  }, [graphData.links])

  const nodeColor = useCallback(
    (node: NetworkGraphNode) => {
      if (mode === 'ambient') {
        if (attack?.nodeId === node.id) {
          return attack.phase === 'compromised' ? '#DC2626' : '#2563EB'
        }
        const hash = node.id.charCodeAt(node.id.length - 1)
        return hash % 3 === 0 ? '#2563EB' : '#98A2B3'
      }
      return severityColor[node.severity ?? 'none'] ?? severityColor.none
    },
    [mode, attack]
  )

  const nodeVal = useCallback(
    (node: NetworkGraphNode) => (mode === 'ambient' && attack?.nodeId === node.id ? 3 : 1),
    [mode, attack]
  )

  // Mind-map style node painting — varied sizes, glow halos
  const paintDataNode = useCallback(
    (node: NetworkGraphNode, ctx: CanvasRenderingContext2D, globalScale: number) => {
      if (node.x == null || node.y == null || !isFinite(node.x) || !isFinite(node.y)) return
      const label = node.name
      const isRoot = node.id === 'root'
      const sev = node.severity ?? 'none'
      const color = severityColor[sev] ?? severityColor.none

      const conns = nodeConnections.get(node.id) ?? 1
      const baseR = isRoot ? 18 : sev === 'critical' ? 13 : sev === 'high' ? 11 : sev === 'medium' ? 9 : 7
      const r = baseR + Math.min(conns * 1.5, 8)

      const fontSize = Math.max(12 / globalScale, 4)

      // Outer glow — all nodes get one, severity scales intensity
      const glowR = r + (isRoot ? 14 : sev === 'critical' ? 12 : sev === 'high' ? 10 : 6)
      const grad = ctx.createRadialGradient(node.x!, node.y!, r * 0.5, node.x!, node.y!, glowR)
      const glowColor = isRoot ? '37,99,235' : sev === 'critical' ? '239,68,68' : sev === 'high' ? '249,115,22' : sev === 'medium' ? '234,179,8' : sev === 'low' ? '6,182,212' : '107,114,128'
      grad.addColorStop(0, `rgba(${glowColor},0.3)`)
      grad.addColorStop(1, `rgba(${glowColor},0)`)
      ctx.beginPath()
      ctx.arc(node.x!, node.y!, glowR, 0, 2 * Math.PI)
      ctx.fillStyle = grad
      ctx.fill()

      // Core circle
      ctx.beginPath()
      ctx.arc(node.x!, node.y!, r, 0, 2 * Math.PI)
      ctx.fillStyle = isRoot ? '#3B82F6' : color
      ctx.fill()

      // Bright rim
      ctx.strokeStyle = isRoot ? 'rgba(96,165,250,0.6)' : `${color}88`
      ctx.lineWidth = (isRoot ? 2.5 : 1.5) / globalScale
      ctx.stroke()

      // Label
      ctx.font = `${isRoot ? 'bold ' : '600 '}${fontSize}px Inter, system-ui, sans-serif`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'top'

      const isDark = document.documentElement.classList.contains('dark') ||
        document.documentElement.getAttribute('data-theme') === 'dark' ||
        (!document.documentElement.getAttribute('data-theme') && window.matchMedia('(prefers-color-scheme: dark)').matches)

      // Text outline for readability
      ctx.strokeStyle = isDark ? 'rgba(0,0,0,0.85)' : 'rgba(255,255,255,0.9)'
      ctx.lineWidth = 3 / globalScale
      ctx.lineJoin = 'round'
      ctx.strokeText(label, node.x!, node.y! + r + 4)
      ctx.fillStyle = isDark ? '#F3F4F6' : '#111827'
      ctx.fillText(label, node.x!, node.y! + r + 4)
    },
    [nodeConnections]
  )

  // Vibrant link colors — each link gets a distinct color from the palette based on hash
  const linkColor = useCallback(
    (link: NetworkGraphLink) => {
      const srcId = typeof link.source === 'object' ? (link.source as NetworkGraphNode).id : link.source
      const tgtId = typeof link.target === 'object' ? (link.target as NetworkGraphNode).id : link.target
      const idx = hashStr(srcId + tgtId) % LINK_PALETTE.length
      return LINK_PALETTE[idx] + '66'
    },
    []
  )

  // Vibrant link colors for arrows too
  const linkArrowColor = useCallback(
    (link: NetworkGraphLink) => {
      const srcId = typeof link.source === 'object' ? (link.source as NetworkGraphNode).id : link.source
      const tgtId = typeof link.target === 'object' ? (link.target as NetworkGraphNode).id : link.target
      const idx = hashStr(srcId + tgtId) % LINK_PALETTE.length
      return LINK_PALETTE[idx] + '88'
    },
    []
  )

  // Custom link painting for curved, vibrant lines
  const paintLink = useCallback(
    (link: LinkObject, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const src = link.source as NetworkGraphNode
      const tgt = link.target as NetworkGraphNode
      if (src?.x == null || src?.y == null || tgt?.x == null || tgt?.y == null) return
      if (!isFinite(src.x) || !isFinite(src.y) || !isFinite(tgt.x) || !isFinite(tgt.y)) return

      const srcId = typeof link.source === 'object' ? (link.source as NetworkGraphNode).id : String(link.source)
      const tgtId = typeof link.target === 'object' ? (link.target as NetworkGraphNode).id : String(link.target)
      const idx = hashStr(srcId + tgtId) % LINK_PALETTE.length
      const color = LINK_PALETTE[idx]

      ctx.beginPath()
      ctx.strokeStyle = color + '55'
      ctx.lineWidth = 1.5 / globalScale
      ctx.moveTo(src.x!, src.y!)
      ctx.lineTo(tgt.x!, tgt.y!)
      ctx.stroke()
    },
    []
  )

  if (mode === 'ambient') {
    return (
      <div
        className="pointer-events-none select-none"
        style={{ opacity, width, height }}
        aria-hidden="true"
      >
        <ForceGraph2D
          ref={graphRef}
          graphData={graphData}
          width={width}
          height={height}
          backgroundColor="transparent"
          nodeColor={nodeColor}
          nodeVal={nodeVal}
          nodeRelSize={4}
          linkColor={() => '#98A2B3'}
          linkWidth={0.5}
          d3AlphaDecay={0.005}
          d3VelocityDecay={0.35}
          cooldownTime={reducedMotion ? 0 : 30000}
          enableZoomInteraction={false}
          enablePanInteraction={false}
          enableNodeDrag={false}
        />
      </div>
    )
  }

  const isDark = typeof document !== 'undefined' && (
    document.documentElement.classList.contains('dark') ||
    document.documentElement.getAttribute('data-theme') === 'dark' ||
    (!document.documentElement.getAttribute('data-theme') && window.matchMedia('(prefers-color-scheme: dark)').matches)
  )
  const bgColor = isDark ? '#0F1114' : '#FAFBFC'

  return (
    <div className="rounded-xl overflow-hidden">
      <ForceGraph2D
        ref={graphRef}
        graphData={graphData}
        width={width}
        height={height}
        backgroundColor={bgColor}
        nodeColor={nodeColor}
        nodeLabel={(node) => `${(node as NetworkGraphNode).name}${(node as NetworkGraphNode).version ? `@${(node as NetworkGraphNode).version}` : ''}`}
        nodeCanvasObject={paintDataNode}
        nodeCanvasObjectMode={() => 'replace'}
        linkCanvasObject={paintLink}
        linkCanvasObjectMode={() => 'replace'}
        d3AlphaDecay={0.015}
        d3VelocityDecay={0.25}
        cooldownTime={4000}
        d3AlphaMin={0.001}
        enableZoomInteraction
        enablePanInteraction
        onNodeClick={onNodeClick ? (node) => onNodeClick(node as NetworkGraphNode) : undefined}
        onEngineStop={() => graphRef.current?.zoomToFit(600, 40)}
      />
    </div>
  )
}

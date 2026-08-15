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

// Light-theme severity palette — mirrors StatusBadge's color mapping so a
// node's color always means the same thing across the dashboard.
const severityColor: Record<string, string> = {
  critical: '#DC2626',
  high: '#EA580C',
  medium: '#D97706',
  low: '#06B6D4',
  none: '#98A2B3',
}

// Deterministic pseudo-random generator (no external dep) so the ambient
// layout is stable across re-renders within a session instead of jumping
// around on every mount.
function seededRandom(seed: number) {
  let value = seed
  return () => {
    value = (value * 9301 + 49297) % 233280
    return value / 233280
  }
}

function buildAmbientData(): NetworkGraphData {
  const rand = seededRandom(42)
  const nodeCount = 8 + Math.floor(rand() * 5) // 8-12 nodes
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

// Ambient "attack narrative" cycle: a node briefly goes red (compromised),
// then blue-with-ring (detected/contained), then fades back to neutral.
// Purely a visual cue for the marketing hero — communicates
// detect → respond without any text. Off entirely under reduced-motion.
type AttackPhase = 'idle' | 'compromised' | 'detected'
const ATTACK_CYCLE_MS = 4200
const COMPROMISED_MS = 1100
const DETECTED_MS = 1400

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
    let timeouts: ReturnType<typeof setTimeout>[] = []

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

  useEffect(() => {
    return () => {
      graphRef.current?.pauseAnimation()
    }
  }, [])

  const nodeColor = useCallback(
    (node: NetworkGraphNode) => {
      if (mode === 'ambient') {
        if (attack?.nodeId === node.id) {
          return attack.phase === 'compromised' ? '#DC2626' : '#2563EB'
        }
        // A few nodes tinted primary-blue, the rest neutral gray.
        const hash = node.id.charCodeAt(node.id.length - 1)
        return hash % 3 === 0 ? '#2563EB' : '#98A2B3'
      }
      return severityColor[node.severity ?? 'none'] ?? severityColor.none
    },
    [mode, attack]
  )

  // Attacked node renders larger + with a soft ring so the compromise/detect
  // cycle actually reads at 8-10% ambient opacity, not just a subtle tint shift.
  const nodeVal = useCallback(
    (node: NetworkGraphNode) => (mode === 'ambient' && attack?.nodeId === node.id ? 3 : 1),
    [mode, attack]
  )

  const paintDataNode = useCallback(
    (node: NetworkGraphNode, ctx: CanvasRenderingContext2D, globalScale: number) => {
      const label = node.name
      const fontSize = 12 / globalScale
      const r = 6
      const color = severityColor[node.severity ?? 'none'] ?? severityColor.none

      ctx.beginPath()
      ctx.arc(node.x!, node.y!, r, 0, 2 * Math.PI)
      ctx.fillStyle = color
      ctx.fill()
      ctx.strokeStyle = `${color}66`
      ctx.lineWidth = 2 / globalScale
      ctx.stroke()

      ctx.font = `${fontSize}px JetBrains Mono, monospace`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillStyle = '#111315'
      ctx.fillText(label, node.x!, node.y! + r + fontSize)
    },
    []
  )

  if (mode === 'ambient') {
    // Ambient mode: extremely low-opacity, non-interactive decorative
    // backdrop. A perpetual "never settle" simulation is achievable via a
    // very low d3AlphaDecay + long cooldownTime, but on low-end devices an
    // endless simulation loop can be a CPU/battery drain for a purely
    // decorative element — we opt for a low alpha decay (slow, gentle drift)
    // combined with a bounded (not infinite) cooldown, which reads as
    // "settling slowly" without running forever. This is the simpler,
    // safer choice per the "don't over-engineer" guidance.
    //
    // prefers-reduced-motion: the simulation still runs a continuous JS
    // animation loop while cooling down (up to `cooldownTime`), so honor the
    // OS-level reduced-motion preference by skipping the animation loop
    // entirely (cooldownTime: 0 renders the initial layout statically) —
    // this also disables the attack-cycle effect above (see useEffect guard).
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

  return (
    <div className="rounded-xl border border-border-color bg-surface overflow-hidden">
      <ForceGraph2D
        ref={graphRef}
        graphData={graphData}
        width={width}
        height={height}
        backgroundColor="#FFFFFF"
        nodeColor={nodeColor}
        nodeLabel={(node) => `${(node as NetworkGraphNode).name}${(node as NetworkGraphNode).version ? `@${(node as NetworkGraphNode).version}` : ''}`}
        nodeCanvasObject={paintDataNode}
        nodeCanvasObjectMode={() => 'replace'}
        linkColor={() => '#E6E8E6'}
        linkWidth={1}
        enableZoomInteraction
        enablePanInteraction
        onNodeClick={onNodeClick ? (node) => onNodeClick(node as NetworkGraphNode) : undefined}
      />
    </div>
  )
}

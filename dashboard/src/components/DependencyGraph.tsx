import { useRef, useEffect, useCallback } from 'react'
import ForceGraph2D, { ForceGraphMethods, NodeObject, LinkObject } from 'react-force-graph-2d'

interface GraphNode extends NodeObject {
  id: string
  name: string
  version: string
  severity: 'critical' | 'high' | 'medium' | 'low' | 'none'
}

interface GraphLink extends LinkObject {
  source: string
  target: string
}

interface GraphData {
  nodes: GraphNode[]
  links: GraphLink[]
}

interface DependencyGraphProps {
  data?: GraphData
  width?: number
  height?: number
}

const severityColor: Record<string, string> = {
  critical: '#FF3D3D',
  high:     '#FF6B35',
  medium:   '#FFAB40',
  low:      '#60A5FA',
  none:     '#374151',
}

const defaultData: GraphData = {
  nodes: [
    { id: 'root', name: 'your-app', version: '1.0.0', severity: 'none' },
    { id: 'lodash', name: 'lodash', version: '4.17.20', severity: 'critical' },
    { id: 'axios', name: 'axios', version: '0.21.1', severity: 'high' },
    { id: 'express', name: 'express', version: '4.18.2', severity: 'none' },
    { id: 'follow-redirects', name: 'follow-redirects', version: '1.14.0', severity: 'medium' },
    { id: 'qs', name: 'qs', version: '6.5.2', severity: 'low' },
  ],
  links: [
    { source: 'root', target: 'lodash' },
    { source: 'root', target: 'axios' },
    { source: 'root', target: 'express' },
    { source: 'axios', target: 'follow-redirects' },
    { source: 'express', target: 'qs' },
  ],
}

export function DependencyGraph({ data = defaultData, width = 600, height = 400 }: DependencyGraphProps) {
  const graphRef = useRef<ForceGraphMethods<GraphNode, GraphLink>>(undefined)

  useEffect(() => {
    return () => {
      graphRef.current?.pauseAnimation()
    }
  }, [])

  const nodeColor = useCallback((node: GraphNode) => severityColor[node.severity] ?? severityColor.none, [])

  const nodeLabel = useCallback((node: GraphNode) => `${node.name}@${node.version}`, [])

  const paintNode = useCallback((node: GraphNode, ctx: CanvasRenderingContext2D, globalScale: number) => {
    const label = node.name
    const fontSize = 12 / globalScale
    const r = 6
    const color = severityColor[node.severity] ?? severityColor.none

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
    ctx.fillStyle = '#F8F9FA'
    ctx.fillText(label, node.x!, node.y! + r + fontSize)
  }, [])

  return (
    <div className="rounded-lg border border-border bg-card overflow-hidden">
      <ForceGraph2D
        ref={graphRef}
        graphData={data}
        width={width}
        height={height}
        backgroundColor="#141618"
        nodeColor={nodeColor}
        nodeLabel={nodeLabel}
        nodeCanvasObject={paintNode}
        nodeCanvasObjectMode={() => 'replace'}
        linkColor={() => '#374151'}
        linkWidth={1}
        enableZoomInteraction
        enablePanInteraction
      />
    </div>
  )
}

import { cn } from './ui/utils'

export type StatusBadgeStatus = 'critical' | 'high' | 'medium' | 'low' | 'healthy' | 'warning' | 'info'

interface StatusBadgeProps {
  status: StatusBadgeStatus
  label?: string
  className?: string
}

// Color mapping — always paired with a text label (never color-only) to meet
// the accessibility requirement in the design spec: color-blind users must
// be able to distinguish severities from the label text alone.
const statusConfig: Record<StatusBadgeStatus, { color: string; defaultLabel: string }> = {
  critical: { color: 'var(--critical)', defaultLabel: 'Critical' },
  // Distinct amber/orange between critical red and warning amber, per spec.
  high: { color: '#EA580C', defaultLabel: 'High' },
  medium: { color: 'var(--warning)', defaultLabel: 'Medium' },
  warning: { color: 'var(--warning)', defaultLabel: 'Warning' },
  low: { color: 'var(--cyan)', defaultLabel: 'Low' },
  info: { color: 'var(--cyan)', defaultLabel: 'Info' },
  healthy: { color: 'var(--success)', defaultLabel: 'Healthy' },
}

export function StatusBadge({ status, label, className }: StatusBadgeProps) {
  const config = statusConfig[status] ?? statusConfig.info
  const text = label ?? config.defaultLabel

  return (
    <span className={cn('inline-flex items-center gap-1.5 text-sm font-medium', className)}>
      <span
        className="inline-block h-2 w-2 shrink-0 rounded-full"
        style={{ backgroundColor: config.color }}
        aria-hidden="true"
      />
      <span style={{ color: config.color }}>{text}</span>
    </span>
  )
}

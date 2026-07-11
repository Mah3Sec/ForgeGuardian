import type { ElementType } from 'react'
import { ArrowUp, ArrowDown, Minus } from 'lucide-react'
import { cn } from './ui/utils'

export type MetricCardTrend = 'up' | 'down' | 'flat'
export type MetricCardVariant = 'default' | 'critical' | 'success'

interface MetricCardProps {
  label: string
  value: string | number
  trend?: MetricCardTrend
  trendValue?: string
  variant?: MetricCardVariant
  icon?: ElementType
  className?: string
}

const variantColor: Record<MetricCardVariant, string> = {
  default: 'var(--text-primary)',
  critical: 'var(--critical)',
  success: 'var(--success)',
}

const trendIcon: Record<MetricCardTrend, ElementType> = {
  up: ArrowUp,
  down: ArrowDown,
  flat: Minus,
}

// For most security metrics (e.g. finding counts) a downward trend is good;
// callers control color via `variant`, so trend color stays neutral here —
// we only pick the icon shape based on direction, not implied good/bad.
const trendColor: Record<MetricCardTrend, string> = {
  up: 'var(--critical)',
  down: 'var(--success)',
  flat: 'var(--text-muted)',
}

export function MetricCard({ label, value, trend, trendValue, variant = 'default', icon: Icon, className }: MetricCardProps) {
  const TrendIcon = trend ? trendIcon[trend] : null

  return (
    <div
      className={cn(
        'rounded-xl border border-border-color bg-surface p-5 shadow-sm',
        className
      )}
    >
      <div className="flex items-start justify-between">
        <span className="text-sm font-medium text-text-secondary">{label}</span>
        {Icon && <Icon size={18} className="text-text-muted" />}
      </div>
      <div className="mt-2 flex items-baseline gap-2">
        <span className="text-3xl font-bold tabular-nums" style={{ color: variantColor[variant] }}>
          {value}
        </span>
        {trend && TrendIcon && (
          <span className="flex items-center gap-0.5 text-xs font-medium" style={{ color: trendColor[trend] }}>
            <TrendIcon size={12} />
            {trendValue}
          </span>
        )}
      </div>
    </div>
  )
}

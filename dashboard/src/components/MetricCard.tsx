import type { ElementType } from 'react'
import { ArrowUp, ArrowDown, Minus } from 'lucide-react'
import { cn } from './ui/utils'

export type MetricCardTrend = 'up' | 'down' | 'flat'
export type MetricCardVariant = 'default' | 'critical' | 'high' | 'medium' | 'low' | 'success'

interface MetricCardProps {
  label: string
  value: string | number
  trend?: MetricCardTrend
  trendValue?: string
  variant?: MetricCardVariant
  icon?: ElementType
  className?: string
  sparklineData?: number[]
}

const variantStyles: Record<MetricCardVariant, { color: string; accent: string; bg: string }> = {
  default:  { color: 'var(--text-primary)', accent: 'var(--border-color)', bg: 'transparent' },
  critical: { color: 'var(--critical)',      accent: 'var(--critical)',     bg: 'color-mix(in srgb, var(--critical) 6%, transparent)' },
  high:     { color: '#EA580C',              accent: '#EA580C',            bg: 'color-mix(in srgb, #EA580C 6%, transparent)' },
  medium:   { color: 'var(--warning)',       accent: 'var(--warning)',     bg: 'color-mix(in srgb, var(--warning) 6%, transparent)' },
  low:      { color: 'var(--cyan)',          accent: 'var(--cyan)',        bg: 'color-mix(in srgb, var(--cyan) 6%, transparent)' },
  success:  { color: 'var(--success)',       accent: 'var(--success)',     bg: 'color-mix(in srgb, var(--success) 6%, transparent)' },
}

const trendIcon: Record<MetricCardTrend, ElementType> = {
  up: ArrowUp,
  down: ArrowDown,
  flat: Minus,
}

const trendColor: Record<MetricCardTrend, string> = {
  up: 'var(--critical)',
  down: 'var(--success)',
  flat: 'var(--text-muted)',
}

function Sparkline({ data, color }: { data: number[]; color: string }) {
  if (data.length < 2) return null
  const max = Math.max(...data, 1)
  const min = Math.min(...data, 0)
  const range = max - min || 1
  const h = 28
  const w = 80
  const points = data.map((v, i) => {
    const x = (i / (data.length - 1)) * w
    const y = h - ((v - min) / range) * (h - 4) - 2
    return `${x},${y}`
  }).join(' ')
  const fillPoints = `${points} ${w},${h} 0,${h}`

  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} preserveAspectRatio="none" className="mt-2">
      <polyline points={fillPoints} fill={color} opacity={0.1} stroke="none" />
      <polyline points={points} fill="none" stroke={color} strokeWidth={1.5} strokeLinejoin="round" />
    </svg>
  )
}

export function MetricCard({ label, value, trend, trendValue, variant = 'default', icon: Icon, className, sparklineData }: MetricCardProps) {
  const TrendIcon = trend ? trendIcon[trend] : null
  const styles = variantStyles[variant]

  return (
    <div
      className={cn(
        'rounded-xl border border-border-color bg-surface shadow-sm relative overflow-hidden',
        className
      )}
      style={{ background: styles.bg }}
    >
      {/* Severity accent bar */}
      {variant !== 'default' && (
        <div
          className="absolute left-0 top-3 bottom-3 w-[3px] rounded-r-sm"
          style={{ background: styles.accent }}
        />
      )}

      <div className="p-4 pl-5">
        <div className="flex items-start justify-between">
          <span className="text-[0.78rem] font-medium text-text-secondary">{label}</span>
          {Icon && <Icon size={16} className="text-text-muted" />}
        </div>
        <div className="mt-1.5 flex items-baseline gap-2">
          <span className="text-[1.75rem] font-bold tabular-nums leading-none" style={{ color: styles.color }}>
            {value}
          </span>
        </div>
        {trend && TrendIcon && (
          <span className="flex items-center gap-0.5 text-[0.68rem] font-medium mt-1" style={{ color: trendColor[trend] }}>
            <TrendIcon size={11} />
            {trendValue}
          </span>
        )}
        {sparklineData && sparklineData.length >= 2 && (
          <Sparkline data={sparklineData} color={styles.accent} />
        )}
      </div>
    </div>
  )
}

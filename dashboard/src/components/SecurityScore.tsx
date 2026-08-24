interface SecurityScoreProps {
  score: number // 0-100
  size?: number
}

interface SeveritySummary {
  critical: number
  high: number
  medium: number
  low: number
}

/**
 * Deterministic security score formula with diminishing returns.
 *
 * Uses sqrt-based scaling so each additional finding contributes less than the
 * previous one. This keeps the score informative across both small projects
 * (a single critical drops from 100 to ~85) and large dependency trees (hundreds
 * of medium findings don't immediately floor the score to 0).
 *
 * Each severity has a weight and a cap on its total contribution:
 *   - critical: weight 15, cap 45 (3+ criticals max out this band)
 *   - high:     weight 10, cap 32 (10+ highs max out)
 *   - medium:   weight 4,  cap 15 (14+ mediums max out)
 *   - low:      weight 2,  cap 8  (16+ lows max out)
 *
 * Total max penalty = 100, so a project with severe issues across all bands
 * reaches 0 while a clean project stays at 100.
 */
export function computeSecurityScore(summary: SeveritySummary): number {
  const band = (count: number, weight: number, cap: number) =>
    Math.min(Math.sqrt(count) * weight, cap)
  const penalty =
    band(summary.critical, 15, 45) +
    band(summary.high, 10, 32) +
    band(summary.medium, 4, 15) +
    band(summary.low, 2, 8)
  return Math.max(0, Math.min(100, Math.round(100 - penalty)))
}

// Score band thresholds — a real product decision, not arbitrary:
//   >= 90: "success" — effectively production-ready, only minor/low findings
//   70-89: "warning" — needs attention before shipping, but not on fire
//   < 70:  "critical" — significant unresolved risk
function bandColor(score: number): string {
  if (score >= 90) return 'var(--success)'
  if (score >= 70) return 'var(--warning)'
  return 'var(--critical)'
}

export function SecurityScore({ score, size = 120 }: SecurityScoreProps) {
  const clamped = Math.max(0, Math.min(100, score))
  const strokeWidth = size * 0.08
  const radius = (size - strokeWidth) / 2
  const circumference = 2 * Math.PI * radius
  const offset = circumference * (1 - clamped / 100)
  const color = bandColor(clamped)

  return (
    <div
      className="relative inline-flex items-center justify-center"
      style={{ width: size, height: size }}
      role="img"
      aria-label={`Security score: ${clamped} out of 100`}
    >
      <svg width={size} height={size} className="-rotate-90">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke="var(--border-color)"
          strokeWidth={strokeWidth}
        />
        <circle
          cx={size / 2}
          cy={size / 2}
          r={radius}
          fill="none"
          stroke={color}
          strokeWidth={strokeWidth}
          strokeDasharray={circumference}
          strokeDashoffset={offset}
          strokeLinecap="round"
          style={{ transition: 'stroke-dashoffset 0.5s ease' }}
        />
      </svg>
      <div className="absolute inset-0 flex flex-col items-center justify-center">
        <span className="text-2xl font-bold tabular-nums" style={{ color }}>
          {Math.round(clamped)}
        </span>
        <span className="text-[10px] font-medium uppercase tracking-wide text-text-muted">
          Score
        </span>
      </div>
    </div>
  )
}

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
 * Deterministic security score formula.
 *
 * Starts at 100 (perfect) and subtracts a weighted penalty per finding,
 * floored at 0. Weights reflect roughly how much each severity level should
 * move the needle on an overall "is this safe to ship" score:
 *   - critical: -15 each (a single critical is a real blocker)
 *   - high:     -8 each
 *   - medium:   -3 each
 *   - low:      -1 each
 *
 * This is intentionally simple and linear rather than logarithmic/curved —
 * it's easy to explain to a user ("you have 2 criticals, that's -30 points")
 * and easy to keep stable as scan coverage grows. Revisit if in practice
 * large dependency trees with many low-severity findings unfairly tank the
 * score — a log-dampened low/medium term would be the next iteration.
 */
export function computeSecurityScore(summary: SeveritySummary): number {
  const penalty =
    summary.critical * 15 + summary.high * 8 + summary.medium * 3 + summary.low * 1
  return Math.max(0, Math.min(100, 100 - penalty))
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

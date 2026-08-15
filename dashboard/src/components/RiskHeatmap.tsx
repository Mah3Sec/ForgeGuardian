import type { RiskItem } from '../types/api'

const GRADE_COLORS: Record<string, string> = {
  A: '#00FF87',
  B: '#4ADE80',
  C: '#FFAB40',
  D: '#FF8C00',
  F: '#FF3D3D',
}

interface EcosystemRisk {
  ecosystem: string
  grade: string
  score: number
  packageCount: number
}

function computeEcosystemRisks(risks: RiskItem[]): EcosystemRisk[] {
  const map = new Map<string, { totalScore: number; count: number }>()
  for (const r of risks) {
    const existing = map.get(r.ecosystem) ?? { totalScore: 0, count: 0 }
    map.set(r.ecosystem, {
      totalScore: existing.totalScore + r.risk_score,
      count: existing.count + 1,
    })
  }
  return Array.from(map.entries())
    .map(([eco, { totalScore, count }]) => {
      const avg = Math.round(totalScore / count)
      return {
        ecosystem: eco,
        score: avg,
        grade: scoreGrade(avg),
        packageCount: count,
      }
    })
    .sort((a, b) => b.score - a.score)
}

function scoreGrade(score: number): string {
  if (score <= 20) return 'A'
  if (score <= 40) return 'B'
  if (score <= 60) return 'C'
  if (score <= 80) return 'D'
  return 'F'
}

interface Props {
  risks: RiskItem[]
}

export function RiskHeatmap({ risks }: Props) {
  const ecosystems = computeEcosystemRisks(risks)

  if (ecosystems.length === 0) {
    return (
      <div style={{
        background: 'var(--bg-surface)',
        border: '1px solid rgba(255,255,255,0.07)',
        borderRadius: '0.5rem',
        padding: '1rem',
      }}>
        <p className="text-xs font-semibold mb-3" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          Ecosystem Risk
        </p>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>No risk data yet.</p>
      </div>
    )
  }

  return (
    <div style={{
      background: 'var(--bg-surface)',
      border: '1px solid rgba(255,255,255,0.07)',
      borderRadius: '0.5rem',
      padding: '1rem',
    }}>
      <p className="text-xs font-semibold mb-3" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        Ecosystem Risk
      </p>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(120px, 1fr))', gap: '0.5rem' }}>
        {ecosystems.map(eco => (
          <div
            key={eco.ecosystem}
            style={{
              background: 'var(--bg-elevated)',
              border: `1px solid ${GRADE_COLORS[eco.grade]}33`,
              borderRadius: '0.375rem',
              padding: '0.625rem 0.75rem',
              display: 'flex',
              flexDirection: 'column',
              gap: '0.25rem',
            }}
          >
            <span className="text-xs font-mono" style={{ color: 'var(--fg)', textTransform: 'uppercase' }}>
              {eco.ecosystem}
            </span>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.375rem' }}>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '1.125rem',
                  fontWeight: 700,
                  color: GRADE_COLORS[eco.grade],
                  lineHeight: 1,
                }}
              >
                {eco.grade}
              </span>
              <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                {eco.packageCount} pkg{eco.packageCount !== 1 ? 's' : ''}
              </span>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}

import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import * as Dialog from '@radix-ui/react-dialog'
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip as RechartsTooltip } from 'recharts'
import { AlertTriangle, X, ArrowRight } from 'lucide-react'
import { getActiveRisks } from '../lib/api'
import type { RiskItem } from '../types/api'
import { StatusBadge, type StatusBadgeStatus } from '../components/StatusBadge'
import { EmptyState } from '../components/EmptyState'
import { LoadingState } from '../components/LoadingState'
import { Table, TableHeader, TableBody, TableRow, TableHead, TableCell } from '../components/ui/table'
import { useUIStore } from '../store/ui'

// Mirrors the Go backend's internal/policy/policy.go severityOrd ordering —
// same threshold approach used by ScanPage.tsx's severity filter, reused
// here instead of reinventing an exact-match filter.
const SEVERITY_ORDER: Record<string, number> = {
  CRITICAL: 4,
  HIGH: 3,
  MEDIUM: 2,
  LOW: 1,
  INFORMATIONAL: 0,
}

type SeverityFilter = 'all' | 'critical' | 'high' | 'medium'

const SEVERITY_FILTER_THRESHOLD: Record<SeverityFilter, number> = {
  all: -1,
  critical: SEVERITY_ORDER.CRITICAL,
  high: SEVERITY_ORDER.HIGH,
  medium: SEVERITY_ORDER.MEDIUM,
}

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp']

function severityToStatus(sev: string): StatusBadgeStatus {
  switch (sev) {
    case 'CRITICAL': return 'critical'
    case 'HIGH': return 'high'
    case 'MEDIUM': return 'medium'
    case 'LOW': return 'low'
    default: return 'info'
  }
}

const SEV_SUMMARY_COLOR: Record<string, string> = {
  CRITICAL: 'var(--critical)',
  HIGH: '#EA580C',
  MEDIUM: 'var(--warning)',
  LOW: 'var(--cyan)',
}

function formatDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '—'
  const diffMs = Date.now() - d.getTime()
  const days = Math.floor(diffMs / 86400000)
  const abs = d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
  if (days <= 0) return `Today (${abs})`
  if (days === 1) return `Yesterday (${abs})`
  if (days < 30) return `${days}d ago (${abs})`
  return abs
}

// Detail drawer — RiskItem is package-level aggregate risk data (grade +
// score + finding_count + top_severity), NOT individual findings with
// titles/descriptions. The drawer is honest about that granularity rather
// than fabricating a per-CVE breakdown the backend doesn't provide.
function RiskDetailDrawer({ item, onClose }: { item: RiskItem | null; onClose: () => void }) {
  const navigate = useUIStore(s => s.navigate)

  return (
    <Dialog.Root open={!!item} onOpenChange={(open) => { if (!open) onClose() }}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/30" />
        <Dialog.Content
          className="fixed right-0 top-0 z-50 h-full w-full max-w-md overflow-y-auto border-l border-border-color bg-surface shadow-lg focus:outline-none"
          aria-describedby={undefined}
        >
          {item && (
            <div className="flex h-full flex-col">
              <div className="flex items-center justify-between border-b border-border-color px-5 py-4">
                <Dialog.Title className="text-sm font-semibold text-text-primary">
                  Risk detail
                </Dialog.Title>
                <Dialog.Close asChild>
                  <button
                    type="button"
                    className="rounded-md p-1 text-text-muted hover:bg-surface-muted hover:text-text-primary"
                    aria-label="Close"
                  >
                    <X size={16} />
                  </button>
                </Dialog.Close>
              </div>

              <div className="flex-1 space-y-5 px-5 py-5">
                <div>
                  <p className="font-mono text-base font-semibold text-text-primary">
                    {item.package_name}
                    <span className="ml-1 text-sm text-text-secondary">@{item.version}</span>
                  </p>
                  <span className="mt-1 inline-block rounded bg-surface-muted px-1.5 py-0.5 font-mono text-xs uppercase text-text-secondary">
                    {item.ecosystem}
                  </span>
                </div>

                {/* Risk score gauge */}
                <div className="rounded-lg border border-border-color p-4" style={{ textAlign: 'center' }}>
                  <svg viewBox="0 0 120 70" width="160" style={{ margin: '0 auto', display: 'block' }}>
                    <path d="M 10 60 A 50 50 0 0 1 110 60" fill="none" stroke="rgba(255,255,255,0.08)" strokeWidth="8" strokeLinecap="round" />
                    <path
                      d="M 10 60 A 50 50 0 0 1 110 60"
                      fill="none"
                      stroke={item.risk_score >= 70 ? 'var(--critical)' : item.risk_score >= 40 ? '#EA580C' : item.risk_score >= 20 ? 'var(--warning)' : 'var(--cyan)'}
                      strokeWidth="8"
                      strokeLinecap="round"
                      strokeDasharray={`${(item.risk_score / 100) * 157} 157`}
                    />
                    <text x="60" y="52" textAnchor="middle" fill="var(--fg)" fontSize="22" fontWeight="bold" fontFamily="var(--font-mono)">{item.risk_score}</text>
                    <text x="60" y="65" textAnchor="middle" fill="var(--color-muted)" fontSize="9" fontFamily="var(--font-mono)">/100</text>
                  </svg>
                  <p className="mt-1 font-mono text-sm text-text-secondary">
                    Grade: <span className="text-lg font-bold text-text-primary">{item.risk_grade}</span>
                  </p>
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-lg border border-border-color p-3">
                    <p className="text-xs text-text-secondary">Top severity</p>
                    <div className="mt-1.5">
                      <StatusBadge status={severityToStatus(item.top_severity)} label={item.top_severity} />
                    </div>
                  </div>
                  <div className="rounded-lg border border-border-color p-3">
                    <p className="text-xs text-text-secondary">Findings</p>
                    <p className="mt-1 font-mono text-2xl font-bold text-text-primary">{item.finding_count}</p>
                  </div>
                </div>

                <div>
                  <p className="text-xs text-text-secondary">First seen</p>
                  <p className="mt-1 text-sm text-text-primary">{formatDate(item.first_seen)}</p>
                </div>

                <p className="text-xs text-text-muted">
                  Run a full scan to see per-finding detail (CVE IDs,
                  descriptions, fix versions) for {item.package_name}.
                </p>
              </div>

              <div className="border-t border-border-color px-5 py-4">
                <button
                  type="button"
                  onClick={() => { navigate('/scan'); onClose() }}
                  className="flex w-full items-center justify-center gap-2 rounded-md bg-primary-blue px-4 py-2 text-sm font-medium text-white hover:opacity-90"
                >
                  Run full scan
                  <ArrowRight size={14} />
                </button>
              </div>
            </div>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

export default function RisksPage() {
  const [ecoFilter, setEcoFilter] = useState('')
  const [sevFilter, setSevFilter] = useState<SeverityFilter>('all')
  const [selected, setSelected] = useState<RiskItem | null>(null)

  const risks = useQuery({ queryKey: ['active-risks'], queryFn: getActiveRisks, refetchInterval: 30_000, retry: false })

  const items: RiskItem[] = useMemo(() => {
    const threshold = SEVERITY_FILTER_THRESHOLD[sevFilter]
    return (risks.data?.risks ?? []).filter(r => {
      if (ecoFilter && r.ecosystem !== ecoFilter) return false
      if ((SEVERITY_ORDER[r.top_severity] ?? 0) < threshold) return false
      return true
    })
  }, [risks.data, ecoFilter, sevFilter])

  const allRisks = risks.data?.risks ?? []
  const countBySev = (sev: string) => allRisks.filter(r => r.top_severity === sev).length
  const isFiltered = ecoFilter !== '' || sevFilter !== 'all'

  return (
    <div className="flex flex-col gap-5 p-6">
      <div>
        <h1 className="text-lg font-bold text-text-primary">Findings</h1>
        <p className="mt-0.5 text-xs text-text-secondary">
          Aggregated security risk across your scanned packages — click a row for detail
        </p>
      </div>

      {/* Severity summary cards */}
      <div className="grid grid-cols-4 gap-3">
        {(['CRITICAL', 'HIGH', 'MEDIUM', 'LOW'] as const).map(sev => (
          <div key={sev} className="rounded-xl border border-border-color bg-surface p-4 shadow-sm">
            <p className="text-xs text-text-secondary">{sev}</p>
            {risks.isLoading
              ? <div className="mt-1 h-7 w-10 animate-pulse rounded bg-surface-muted" />
              : <p className="mt-0.5 font-mono text-2xl font-bold" style={{ color: SEV_SUMMARY_COLOR[sev] }}>
                  {countBySev(sev)}
                </p>
            }
          </div>
        ))}
      </div>

      {/* Severity distribution + risk grade distribution */}
      {allRisks.length > 0 && (() => {
        const sevCounts = [
          { name: 'Critical', value: countBySev('CRITICAL'), color: 'var(--critical)' },
          { name: 'High', value: countBySev('HIGH'), color: '#EA580C' },
          { name: 'Medium', value: countBySev('MEDIUM'), color: 'var(--warning)' },
          { name: 'Low', value: countBySev('LOW'), color: 'var(--cyan)' },
        ].filter(d => d.value > 0);
        const gradeCounts = ['A', 'B', 'C', 'D', 'F'].map(g => ({
          name: g,
          value: allRisks.filter(r => r.risk_grade === g).length,
        })).filter(d => d.value > 0);
        const GRADE_COLORS: Record<string, string> = { A: '#22c55e', B: '#60A5FA', C: '#F59E0B', D: '#EA580C', F: '#FF3D3D' };
        return (
          <div className="grid grid-cols-2 gap-3">
            <div className="rounded-xl border border-border-color bg-surface p-4 shadow-sm">
              <p className="text-xs font-semibold uppercase text-text-secondary" style={{ letterSpacing: '0.05em' }}>Severity Distribution</p>
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginTop: 8 }}>
                <ResponsiveContainer width="50%" height={120}>
                  <PieChart>
                    <Pie data={sevCounts} dataKey="value" cx="50%" cy="50%" innerRadius={30} outerRadius={48} paddingAngle={2} strokeWidth={0}>
                      {sevCounts.map((d, i) => <Cell key={i} fill={d.color} />)}
                    </Pie>
                    <RechartsTooltip contentStyle={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: '0.75rem', color: 'var(--fg)' }} />
                  </PieChart>
                </ResponsiveContainer>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1 }}>
                  {sevCounts.map(d => (
                    <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.75rem' }}>
                      <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
                      <span className="text-text-secondary" style={{ flex: 1 }}>{d.name}</span>
                      <span className="font-mono font-semibold text-text-primary">{d.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <div className="rounded-xl border border-border-color bg-surface p-4 shadow-sm">
              <p className="text-xs font-semibold uppercase text-text-secondary" style={{ letterSpacing: '0.05em' }}>Risk Grade Distribution</p>
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginTop: 8 }}>
                <ResponsiveContainer width="50%" height={120}>
                  <PieChart>
                    <Pie data={gradeCounts} dataKey="value" cx="50%" cy="50%" innerRadius={30} outerRadius={48} paddingAngle={2} strokeWidth={0}>
                      {gradeCounts.map((d, i) => <Cell key={i} fill={GRADE_COLORS[d.name] ?? '#666'} />)}
                    </Pie>
                    <RechartsTooltip contentStyle={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: '0.75rem', color: 'var(--fg)' }} />
                  </PieChart>
                </ResponsiveContainer>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 4, flex: 1 }}>
                  {gradeCounts.map(d => (
                    <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.75rem' }}>
                      <span style={{ width: 8, height: 8, borderRadius: '50%', background: GRADE_COLORS[d.name] ?? '#666', flexShrink: 0 }} />
                      <span className="text-text-secondary" style={{ flex: 1 }}>Grade {d.name}</span>
                      <span className="font-mono font-semibold text-text-primary">{d.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>
        );
      })()}

      {/* Filters + table */}
      <div className="overflow-hidden rounded-xl border border-border-color bg-surface shadow-sm">
        <div className="flex items-center gap-3 border-b border-border-color px-4 py-3">
          <AlertTriangle size={14} className="text-warning" />
          <span className="text-xs font-semibold text-text-primary">{items.length} finding{items.length !== 1 ? 's' : ''}</span>
          <div className="ml-auto flex gap-2">
            <select
              value={ecoFilter}
              onChange={e => setEcoFilter(e.target.value)}
              className="rounded-md border border-border-color bg-bg-base px-2 py-1 text-xs text-text-primary [font-family:inherit]"
            >
              <option value="">All ecosystems</option>
              {ECOSYSTEMS.map(e => <option key={e} value={e}>{e}</option>)}
            </select>
            <select
              value={sevFilter}
              onChange={e => setSevFilter(e.target.value as SeverityFilter)}
              className="rounded-md border border-border-color bg-bg-base px-2 py-1 text-xs text-text-primary [font-family:inherit]"
            >
              <option value="all">All severities</option>
              <option value="critical">Critical+</option>
              <option value="high">High+</option>
              <option value="medium">Medium+</option>
            </select>
          </div>
        </div>

        {risks.isLoading ? (
          <LoadingState rows={6} variant="table" className="px-4 py-3" />
        ) : items.length === 0 ? (
          isFiltered ? (
            <EmptyState
              icon={AlertTriangle}
              title="No findings match these filters"
              description="Try widening the severity or ecosystem filter."
              action={{ label: 'Clear filters', onClick: () => { setEcoFilter(''); setSevFilter('all') } }}
            />
          ) : (
            <EmptyState
              icon={AlertTriangle}
              title="No active findings"
              description="Run fgctl scan . to populate this view with real scan results."
            />
          )
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Severity</TableHead>
                <TableHead>Finding</TableHead>
                <TableHead>Asset</TableHead>
                <TableHead>Risk</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map(item => (
                <TableRow
                  key={`${item.ecosystem}/${item.package_name}@${item.version}`}
                  onClick={() => setSelected(item)}
                  className="cursor-pointer"
                >
                  <TableCell>
                    <StatusBadge status={severityToStatus(item.top_severity)} label={item.top_severity} />
                  </TableCell>
                  <TableCell>
                    <span className="text-sm text-text-primary">
                      {item.package_name} — {item.finding_count} finding{item.finding_count !== 1 ? 's' : ''} found
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-xs uppercase text-text-secondary">{item.ecosystem}</span>
                    <span className="ml-1 font-mono text-sm text-text-primary">{item.package_name}@{item.version}</span>
                  </TableCell>
                  <TableCell>
                    <span className="font-mono text-sm font-semibold text-text-primary">{item.risk_score}</span>
                    <span className="text-xs text-text-muted">/100</span>
                  </TableCell>
                  <TableCell>
                    {/* No per-finding resolved/open workflow state exists server-side —
                        "Detected" is an honest label; it does not imply a tracked
                        open/resolved lifecycle that the backend doesn't have. */}
                    <span className="rounded-full bg-surface-muted px-2 py-0.5 text-xs font-medium text-text-secondary">
                      Detected
                    </span>
                  </TableCell>
                  <TableCell>
                    <span className="text-xs text-text-secondary">{formatDate(item.first_seen)}</span>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      <RiskDetailDrawer item={selected} onClose={() => setSelected(null)} />
    </div>
  )
}

import { useState } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { getDashboardStats, getRecentResults, getDashboardTimeline, getPolicyStatus, getMonitorEvents, quarantinePackage, blockPackage, unquarantinePackage } from '../lib/api'
import type { MonitorEvent } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Badge } from '../components/ui/badge'
import { Skeleton } from '../components/ui/skeleton'
import { ActivityFeed } from '../components/ActivityFeed'
import { SeverityBadge } from '../components/SeverityBadge'
import { ShieldAlert, ShieldBan, ShieldCheck, Clock, History, Activity, Shield } from 'lucide-react'
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip as RechartsTooltip } from 'recharts'

function StatCard({ label, value, variant }: { label: string; value: number; variant?: 'critical' | 'high' | 'medium' | 'low' | 'safe' | 'default' }) {
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">{label}</span>
          {variant && variant !== 'default' && (
            <Badge variant={variant}>{variant.toUpperCase()}</Badge>
          )}
        </div>
        <p className="mt-2 text-3xl font-bold font-mono">{value.toLocaleString()}</p>
      </CardContent>
    </Card>
  )
}

function StatCardSkeleton() {
  return (
    <Card>
      <CardContent className="pt-6">
        <Skeleton className="h-4 w-24 mb-3" />
        <Skeleton className="h-9 w-16" />
      </CardContent>
    </Card>
  )
}

function TrendChart() {
  const { data } = useQuery({
    queryKey: ['monitor-timeline'],
    queryFn: () => getDashboardTimeline(14),
    refetchInterval: 30_000,
    retry: false,
  })

  const points = data?.points ?? []
  if (points.length === 0) return null

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Finding Trend (14d)</CardTitle>
      </CardHeader>
      <CardContent>
        <ResponsiveContainer width="100%" height={160}>
          <AreaChart data={points} margin={{ top: 4, right: 8, left: 0, bottom: 4 }}>
            <defs>
              <linearGradient id="critGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--color-critical)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--color-critical)" stopOpacity={0} />
              </linearGradient>
              <linearGradient id="highGrad" x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--color-high)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--color-high)" stopOpacity={0} />
              </linearGradient>
            </defs>
            <XAxis dataKey="date" tick={{ fontSize: 10, fill: 'var(--color-muted)' }} axisLine={false} tickLine={false}
              tickFormatter={(v: string) => new Date(v).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })} />
            <YAxis hide />
            <RechartsTooltip
              contentStyle={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: '0.75rem', color: 'var(--fg)' }}
            />
            <Area type="monotone" dataKey="critical" stroke="var(--color-critical)" fill="url(#critGrad)" strokeWidth={2} />
            <Area type="monotone" dataKey="high" stroke="var(--color-high)" fill="url(#highGrad)" strokeWidth={1.5} />
            <Area type="monotone" dataKey="medium" stroke="var(--color-medium)" fill="none" strokeWidth={1} strokeDasharray="3 3" />
          </AreaChart>
        </ResponsiveContainer>
      </CardContent>
    </Card>
  )
}

function RecentScansTable() {
  const { data, isLoading } = useQuery({
    queryKey: ['monitor-recent-scans'],
    queryFn: () => getRecentResults(15),
    refetchInterval: 15_000,
    retry: false,
  })

  const results = data?.results ?? []

  if (isLoading) {
    return (
      <Card>
        <CardHeader><CardTitle className="text-sm font-medium">Recent Scans</CardTitle></CardHeader>
        <CardContent>
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-6 w-full" />)}
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium">Recent Scans</CardTitle>
      </CardHeader>
      <CardContent>
        {results.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">No scans yet — run a scan to see results here.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
                  {['Package', 'Version', 'Severity', 'Findings', 'Time'].map(h => (
                    <th key={h} className="text-left py-1.5 px-2 font-mono text-xs uppercase" style={{ color: 'var(--color-muted)' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                    <td className="py-1.5 px-2 font-mono text-xs" style={{ color: 'var(--fg)' }}>
                      <span style={{ color: 'var(--color-muted)', marginRight: 4 }}>{r.ecosystem}/</span>{r.package}
                    </td>
                    <td className="py-1.5 px-2 font-mono text-xs" style={{ color: 'var(--color-muted)' }}>{r.version}</td>
                    <td className="py-1.5 px-2"><SeverityBadge severity={r.severity} /></td>
                    <td className="py-1.5 px-2 font-mono text-xs" style={{ color: 'var(--fg)' }}>{r.findings_count}</td>
                    <td className="py-1.5 px-2 text-xs" style={{ color: 'var(--color-muted)' }}>
                      {new Date(r.scanned_at).toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ── Deny List Manager ────────────────────────────────────────────────────

function DenyListManager() {
  const qc = useQueryClient()
  const [pkg, setPkg] = useState('')
  const [reason, setReason] = useState('')
  const [loading, setLoading] = useState<string | null>(null)

  const { data: policyData } = useQuery({
    queryKey: ['policy-status'],
    queryFn: getPolicyStatus,
    refetchInterval: 10_000,
    retry: false,
  })

  const denyPackages = policyData?.policy?.deny_packages ?? []

  const doAction = async (action: 'quarantine' | 'block', name: string, rsn: string) => {
    setLoading(name || 'new')
    try {
      if (action === 'quarantine') await quarantinePackage(name, rsn)
      else await blockPackage(name, rsn)
      qc.invalidateQueries({ queryKey: ['policy-status'] })
      qc.invalidateQueries({ queryKey: ['monitor-events'] })
      setPkg('')
      setReason('')
    } catch { /* silently fail */ }
    setLoading(null)
  }

  const doUnquarantine = async (name: string) => {
    setLoading(name)
    try {
      await unquarantinePackage(name)
      qc.invalidateQueries({ queryKey: ['policy-status'] })
      qc.invalidateQueries({ queryKey: ['monitor-events'] })
    } catch { /* silently fail */ }
    setLoading(null)
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <Shield size={14} /> Package Policy Actions
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <input
            value={pkg}
            onChange={e => setPkg(e.target.value)}
            placeholder="Package name"
            style={{
              flex: 1, minWidth: 140, padding: '6px 10px', borderRadius: 4,
              background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.1)',
              color: 'var(--fg)', fontFamily: 'var(--font-mono)', fontSize: '0.78rem',
            }}
          />
          <input
            value={reason}
            onChange={e => setReason(e.target.value)}
            placeholder="Reason (optional)"
            style={{
              flex: 1.5, minWidth: 180, padding: '6px 10px', borderRadius: 4,
              background: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.1)',
              color: 'var(--fg)', fontFamily: 'var(--font-mono)', fontSize: '0.78rem',
            }}
          />
          <button
            disabled={!pkg.trim() || loading === 'new'}
            onClick={() => doAction('quarantine', pkg.trim(), reason.trim())}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 4, padding: '6px 12px', borderRadius: 4,
              border: '1px solid rgba(255,165,0,0.3)', background: 'rgba(255,165,0,0.1)',
              color: '#FFA500', fontSize: '0.78rem', fontFamily: 'var(--font-mono)',
              cursor: !pkg.trim() ? 'not-allowed' : 'pointer', opacity: !pkg.trim() ? 0.4 : 1,
            }}
          >
            <ShieldAlert size={13} /> Quarantine
          </button>
          <button
            disabled={!pkg.trim() || loading === 'new'}
            onClick={() => doAction('block', pkg.trim(), reason.trim())}
            style={{
              display: 'inline-flex', alignItems: 'center', gap: 4, padding: '6px 12px', borderRadius: 4,
              border: '1px solid rgba(255,61,61,0.3)', background: 'rgba(255,61,61,0.1)',
              color: '#FF3D3D', fontSize: '0.78rem', fontFamily: 'var(--font-mono)',
              cursor: !pkg.trim() ? 'not-allowed' : 'pointer', opacity: !pkg.trim() ? 0.4 : 1,
            }}
          >
            <ShieldBan size={13} /> Block
          </button>
        </div>

        {denyPackages.length > 0 && (
          <div>
            <p className="text-xs font-mono uppercase mb-2" style={{ color: 'var(--color-muted)' }}>Denied Packages ({denyPackages.length})</p>
            <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
              {denyPackages.map(d => (
                <span key={d} style={{
                  display: 'inline-flex', alignItems: 'center', gap: 6, padding: '3px 10px', borderRadius: 4,
                  background: 'rgba(255,61,61,0.08)', border: '1px solid rgba(255,61,61,0.2)',
                  fontSize: '0.75rem', fontFamily: 'var(--font-mono)', color: '#FF3D3D',
                }}>
                  <ShieldBan size={11} /> {d}
                  <button
                    disabled={loading === d}
                    onClick={() => doUnquarantine(d)}
                    title="Remove from deny list"
                    style={{
                      marginLeft: 2, padding: '1px 4px', borderRadius: 3,
                      background: 'rgba(0,255,135,0.1)', border: '1px solid rgba(0,255,135,0.3)',
                      color: '#00FF87', fontSize: '0.65rem', cursor: loading === d ? 'wait' : 'pointer',
                      lineHeight: 1,
                    }}
                  >
                    <ShieldCheck size={10} />
                  </button>
                </span>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ── Events History ───────────────────────────────────────────────────────

function EventsHistory() {
  const { data, isLoading } = useQuery({
    queryKey: ['monitor-events'],
    queryFn: getMonitorEvents,
    refetchInterval: 10_000,
    retry: false,
  })

  const events: MonitorEvent[] = data?.events ?? []

  const actionStyle = (action: string) => {
    switch (action) {
      case 'quarantine': return { color: '#FFA500', bg: 'rgba(255,165,0,0.08)', icon: <ShieldAlert size={12} /> }
      case 'block': return { color: '#FF3D3D', bg: 'rgba(255,61,61,0.08)', icon: <ShieldBan size={12} /> }
      case 'unquarantine': return { color: '#00FF87', bg: 'rgba(0,255,135,0.08)', icon: <ShieldCheck size={12} /> }
      default: return { color: 'var(--color-muted)', bg: 'rgba(255,255,255,0.04)', icon: <Activity size={12} /> }
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-sm font-medium flex items-center gap-2">
          <History size={14} /> Action History
        </CardTitle>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
          </div>
        ) : events.length === 0 ? (
          <p className="text-sm text-muted-foreground text-center py-8">
            No policy actions recorded yet. Quarantine or block a package to see events here.
          </p>
        ) : (
          <div className="overflow-x-auto" style={{ maxHeight: 400, overflowY: 'auto' }}>
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
                  {['Time', 'Action', 'Package', 'Reason'].map(h => (
                    <th key={h} className="text-left py-1.5 px-2 font-mono text-xs uppercase sticky top-0" style={{ color: 'var(--color-muted)', background: 'var(--surface)' }}>{h}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {events.map((ev, i) => {
                  const s = actionStyle(ev.action)
                  return (
                    <tr key={i} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                      <td className="py-1.5 px-2 text-xs font-mono" style={{ color: 'var(--color-muted)', whiteSpace: 'nowrap' }}>
                        <Clock size={10} style={{ display: 'inline', marginRight: 4, verticalAlign: 'middle' }} />
                        {new Date(ev.timestamp).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                      </td>
                      <td className="py-1.5 px-2">
                        <span style={{
                          display: 'inline-flex', alignItems: 'center', gap: 4, padding: '2px 8px', borderRadius: 4,
                          background: s.bg, color: s.color, fontSize: '0.72rem', fontFamily: 'var(--font-mono)',
                          textTransform: 'uppercase', fontWeight: 600,
                        }}>
                          {s.icon} {ev.action}
                        </span>
                      </td>
                      <td className="py-1.5 px-2 font-mono text-xs" style={{ color: 'var(--fg)' }}>{ev.package}</td>
                      <td className="py-1.5 px-2 text-xs" style={{ color: 'var(--color-muted)' }}>{ev.reason || '—'}</td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ── Tab Navigation ───────────────────────────────────────────────────────

type TabKey = 'overview' | 'actions' | 'history'

const TABS: { key: TabKey; label: string; icon: typeof Activity }[] = [
  { key: 'overview', label: 'Overview', icon: Activity },
  { key: 'actions', label: 'Policy Actions', icon: Shield },
  { key: 'history', label: 'Event History', icon: History },
]

export default function MonitorPage() {
  const [tab, setTab] = useState<TabKey>('overview')

  const { data: stats, isError, isLoading, failureCount, dataUpdatedAt } = useQuery({
    queryKey: ['monitor-stats'],
    queryFn: () => getDashboardStats(),
    refetchInterval: 10_000,
    retry: 3,
    retryDelay: (attempt: number) => Math.min(1000 * 2 ** attempt, 10_000),
  })

  const lastRefresh = dataUpdatedAt ? new Date(dataUpdatedAt) : null

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-foreground">Live Monitor</h1>
          <p className="text-sm text-muted-foreground mt-0.5">
            Refreshes every 10s{lastRefresh ? ` — last updated ${lastRefresh.toLocaleTimeString()}` : ''}
          </p>
        </div>
        {isError && (
          <span style={{ color: isLoading ? 'var(--color-warn)' : 'var(--color-critical)' }}>
            {isLoading && failureCount > 0 ? 'Reconnecting...' : 'API unreachable'}
          </span>
        )}
        {!isError && stats && (
          <span className="flex items-center gap-1.5 text-xs text-[#00FF87]">
            <span className="h-2 w-2 rounded-full bg-[#00FF87] animate-pulse" />
            {stats.ecosystems_covered.length} ecosystem{stats.ecosystems_covered.length !== 1 ? 's' : ''} monitored
          </span>
        )}
      </div>

      {/* Tab bar */}
      <div style={{ display: 'flex', gap: 2, borderBottom: '1px solid rgba(255,255,255,0.08)', paddingBottom: 0 }}>
        {TABS.map(t => {
          const Icon = t.icon
          const active = tab === t.key
          return (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: 6,
                padding: '8px 16px', fontSize: '0.82rem', fontFamily: 'var(--font-mono)',
                background: 'none', border: 'none', cursor: 'pointer',
                color: active ? 'var(--fg)' : 'var(--color-muted)',
                borderBottom: active ? '2px solid var(--fg)' : '2px solid transparent',
                fontWeight: active ? 600 : 400,
                transition: 'color 0.15s, border-color 0.15s',
              }}
            >
              <Icon size={14} /> {t.label}
            </button>
          )
        })}
      </div>

      {/* Tab content */}
      {tab === 'overview' && (
        <>
          {isLoading ? (
            <>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                {Array.from({ length: 4 }).map((_, i) => <StatCardSkeleton key={i} />)}
              </div>
              <div className="grid grid-cols-2 gap-4">
                {Array.from({ length: 2 }).map((_, i) => <StatCardSkeleton key={i} />)}
              </div>
            </>
          ) : isError ? (
            <div className="flex flex-col items-center justify-center h-48 gap-3 text-center">
              <p className="text-muted-foreground text-sm">Could not reach API. Is the server running?</p>
              <code className="text-xs px-3 py-1.5 rounded font-mono" style={{ background: 'rgba(255,255,255,0.06)' }}>fgctl serve</code>
            </div>
          ) : stats ? (
            <>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <StatCard label="Total Packages" value={stats.total_packages} variant="default" />
                <StatCard label="Scanned Today" value={stats.scanned_today} variant="safe" />
                <StatCard label="Critical Findings" value={stats.critical_findings} variant="critical" />
                <StatCard label="High Findings" value={stats.high_findings} variant="high" />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <StatCard label="Total Findings" value={stats.total_findings} variant="default" />
                <StatCard label="Total Versions" value={stats.total_versions} variant="default" />
              </div>

              <TrendChart />

              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                <RecentScansTable />
                <Card>
                  <CardHeader>
                    <CardTitle className="text-sm font-medium">Live Activity</CardTitle>
                  </CardHeader>
                  <CardContent>
                    <ActivityFeed limit={15} />
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm font-medium">System Info</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="space-y-2 text-sm">
                    <div className="flex items-center justify-between py-2 border-b border-border">
                      <span className="text-muted-foreground">Last updated</span>
                      <span className="font-mono text-foreground">
                        {stats.last_updated ? new Date(stats.last_updated).toLocaleString() : '—'}
                      </span>
                    </div>
                    <div className="flex items-center justify-between py-2">
                      <span className="text-muted-foreground">Ecosystems covered</span>
                      <span className="font-mono text-foreground">
                        {stats.ecosystems_covered.length > 0 ? stats.ecosystems_covered.join(', ') : '—'}
                      </span>
                    </div>
                  </div>
                </CardContent>
              </Card>
            </>
          ) : null}
        </>
      )}

      {tab === 'actions' && <DenyListManager />}

      {tab === 'history' && <EventsHistory />}
    </div>
  )
}

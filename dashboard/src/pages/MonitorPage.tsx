import { useQuery } from '@tanstack/react-query'
import { getDashboardStats } from '../lib/api'
import { Card, CardContent, CardHeader, CardTitle } from '../components/ui/card'
import { Badge } from '../components/ui/badge'
import { Skeleton } from '../components/ui/skeleton'

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

export default function MonitorPage() {
  const { data: stats, isError, isLoading, failureCount, dataUpdatedAt } = useQuery({
    queryKey: ['monitor-stats'],
    queryFn: getDashboardStats,
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
          <code className="text-xs px-3 py-1.5 rounded font-mono" style={{ background: 'rgba(255,255,255,0.06)' }}>make up</code>
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

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-medium">Recent Activity</CardTitle>
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
    </div>
  )
}

import { useQuery } from '@tanstack/react-query'
import { getDashboardActivity } from '../lib/api'
import type { ActivityEvent } from '../types/api'
import { Skeleton } from './ui/skeleton'

function severityDot(severity: string) {
  switch (severity) {
    case 'CRITICAL': return '#FF3D3D'
    case 'HIGH':     return '#FF8C00'
    case 'MEDIUM':   return '#FFAB40'
    case 'LOW':      return '#60A5FA'
    default:         return '#6B7280'
  }
}

function relativeTime(iso: string): string {
  const diff = Math.floor((Date.now() - new Date(iso).getTime()) / 1000)
  if (diff < 60)  return `${diff}s ago`
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}

interface Props {
  limit?: number
  className?: string
}

export function ActivityFeed({ limit = 20, className }: Props) {
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard-activity', limit],
    queryFn: () => getDashboardActivity(limit),
    refetchInterval: 15_000,
    retry: false,
  })

  const events: ActivityEvent[] = data?.events ?? []

  return (
    <div
      className={className}
      style={{
        background: 'var(--bg-surface)',
        border: '1px solid rgba(255,255,255,0.07)',
        borderRadius: '0.5rem',
        padding: '1rem',
        display: 'flex',
        flexDirection: 'column',
        gap: '0.25rem',
        overflowY: 'auto',
        maxHeight: '320px',
      }}
    >
      <p className="text-xs font-semibold mb-2" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
        Live Activity
      </p>

      {isLoading && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
          {[...Array(5)].map((_, i) => <Skeleton key={i} className="h-4 w-full" />)}
        </div>
      )}

      {!isLoading && events.length === 0 && (
        <p className="text-xs" style={{ color: 'var(--color-muted)', padding: '1rem 0' }}>
          No activity yet — run a scan to see events here.
        </p>
      )}

      {events.map(event => (
        <div
          key={event.id}
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: '0.5rem',
            padding: '0.35rem 0',
            borderBottom: '1px solid rgba(255,255,255,0.04)',
          }}
        >
          <span
            style={{
              width: 7,
              height: 7,
              borderRadius: '50%',
              background: severityDot(event.severity),
              flexShrink: 0,
              marginTop: 5,
            }}
          />
          <div style={{ flex: 1, minWidth: 0 }}>
            <p className="text-xs" style={{ color: 'var(--fg)', lineHeight: 1.4, wordBreak: 'break-word' }}>
              {event.message}
            </p>
          </div>
          <span className="text-xs" style={{ color: 'var(--color-muted)', flexShrink: 0, marginLeft: '0.25rem' }}>
            {relativeTime(event.occurred_at)}
          </span>
        </div>
      ))}
    </div>
  )
}

import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getHealth } from '../lib/api'

export function ApiStatusBanner() {
  const [dismissed, setDismissed] = useState(false)

  const { isError, isLoading, failureCount } = useQuery({
    queryKey: ['api-health'],
    queryFn: getHealth,
    retry: 3,
    retryDelay: (attempt: number) => Math.min(1000 * 2 ** attempt, 10_000),
    refetchInterval: 15_000,
    refetchOnWindowFocus: true,
  })

  const isReconnecting = isLoading && failureCount > 0

  if (isLoading || !isError || dismissed) return null

  return (
    <div
      role="alert"
      style={{
        background: '#2a1a0a',
        borderBottom: '1px solid #FFAB40',
        padding: '10px 24px',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: '12px',
        fontSize: '13px',
        color: '#FFAB40',
        fontFamily: 'JetBrains Mono, monospace',
      }}
    >
      <span>
        {isReconnecting
          ? <strong style={{ color: 'var(--color-warn)' }}>[RECONNECTING...]</strong>
          : <strong>[WARN]</strong>
        }{' '}ForgeGuardian API offline — start the backend with:{' '}
        <code
          style={{
            background: '#1a1200',
            padding: '2px 6px',
            borderRadius: '3px',
            border: '1px solid #FFAB40',
            color: '#FFD580',
          }}
        >
          make api
        </code>
        {' '}or{' '}
        <code
          style={{
            background: '#1a1200',
            padding: '2px 6px',
            borderRadius: '3px',
            border: '1px solid #FFAB40',
            color: '#FFD580',
          }}
        >
          docker compose -f docker-compose.minimal.yml up -d
        </code>
      </span>
      <button
        onClick={() => setDismissed(true)}
        aria-label="Dismiss"
        style={{
          background: 'transparent',
          border: 'none',
          color: '#FFAB40',
          cursor: 'pointer',
          fontSize: '16px',
          padding: '0 4px',
          lineHeight: 1,
          flexShrink: 0,
        }}
      >
        ✕
      </button>
    </div>
  )
}

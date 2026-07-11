import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Bell, X } from 'lucide-react';
import { listAlerts, dismissAlert } from '../lib/api';
import type { Severity } from '../types/api';

const SEV_COLORS: Record<string, string> = {
  CRITICAL:      'var(--color-critical)',
  HIGH:          'var(--color-high)',
  MEDIUM:        'var(--color-medium)',
  LOW:           'var(--color-low)',
  INFORMATIONAL: 'var(--color-info)',
};

function relativeTime(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

export function AlertsPage() {
  const qc = useQueryClient();
  const [sevFilter, setSevFilter] = useState<string>('ALL');
  const [showDismissed, setShowDismissed] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ['alerts', sevFilter, showDismissed],
    queryFn: () => listAlerts({
      page_size: 100,
      severity: sevFilter === 'ALL' ? undefined : sevFilter,
      dismissed: showDismissed ? undefined : false,
    }),
    refetchInterval: 15_000,
  });

  const dismiss = useMutation({
    mutationFn: (id: number) => dismissAlert(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['alerts'] }),
  });

  const alerts = data?.alerts ?? [];
  const severities = ['ALL', 'CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFORMATIONAL'];

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Alerts</h1>
          <p className="text-sm mt-1" style={{ color: 'var(--color-muted)' }}>
            Security alerts from all scan engines.
            {data?.total !== undefined && <span> ({data.total} total)</span>}
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
          <label style={{ fontSize: '0.72rem', color: 'var(--color-muted)', display: 'flex', alignItems: 'center', gap: 4, cursor: 'pointer' }}>
            <input type="checkbox" checked={showDismissed} onChange={e => setShowDismissed(e.target.checked)} />
            show dismissed
          </label>
          <Bell size={18} style={{ color: 'var(--color-warn)' }} />
        </div>
      </div>

      {/* Severity filter */}
      <div style={{ display: 'flex', gap: '0.375rem', flexWrap: 'wrap' }}>
        {severities.map(s => (
          <button key={s} onClick={() => setSevFilter(s)} style={{
            padding: '0.25rem 0.625rem',
            borderRadius: '0.25rem',
            fontSize: '0.7rem',
            fontFamily: 'var(--font-mono)',
            fontWeight: 600,
            background: sevFilter === s ? (SEV_COLORS[s] ?? 'var(--color-indigo)') : 'var(--surface)',
            color: sevFilter === s ? '#fff' : 'var(--color-muted)',
            border: '1px solid rgba(255,255,255,0.08)',
            cursor: 'pointer',
          }}>
            {s}
          </button>
        ))}
      </div>

      {/* Alert list */}
      <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        {isLoading && (
          <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>Loading alerts…</div>
        )}
        {!isLoading && alerts.length === 0 && (
          <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>No alerts. Run a scan to generate findings.</div>
        )}
        {alerts.map((al, i) => (
          <div key={al.id} style={{
            display: 'flex', alignItems: 'flex-start', gap: '0.75rem',
            padding: '0.75rem 1rem',
            borderBottom: i < alerts.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none',
            opacity: al.dismissed ? 0.45 : 1,
          }}>
            <div style={{
              width: 8, height: 8, borderRadius: '50%',
              background: SEV_COLORS[al.severity as Severity] ?? 'var(--color-muted)',
              marginTop: 5, flexShrink: 0,
            }} />
            <div style={{ flex: 1, minWidth: 0 }}>
              <p style={{ fontSize: '0.8rem', color: 'var(--fg)', margin: 0 }}>{al.message}</p>
              {al.package_name && (
                <p style={{ fontSize: '0.7rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', margin: '0.15rem 0 0' }}>
                  {al.ecosystem ? `${al.ecosystem}/` : ''}{al.package_name}{al.version ? `@${al.version}` : ''}
                </p>
              )}
            </div>
            <span style={{ fontSize: '0.68rem', color: 'var(--color-muted)', whiteSpace: 'nowrap', flexShrink: 0 }}>
              {relativeTime(al.occurred_at)}
            </span>
            {!al.dismissed && (
              <button
                onClick={() => dismiss.mutate(al.id)}
                title="Dismiss"
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-muted)', flexShrink: 0, padding: 2 }}
              >
                <X size={12} />
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

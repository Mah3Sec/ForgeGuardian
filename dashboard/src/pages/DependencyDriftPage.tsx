import { useQuery } from '@tanstack/react-query';
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';
import { getDashboardTimeline } from '../lib/api';
import { GitBranch } from 'lucide-react';

export function DependencyDriftPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['dashboard-timeline-drift'],
    queryFn: () => getDashboardTimeline(30),
    refetchInterval: 60_000,
  });

  const pts = data?.points ?? [];

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>
        Dependency Drift
      </h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Track changes in your dependency graph and vulnerability posture over the last 30 days.
      </p>

      <div className="rounded-lg p-5" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div className="flex items-center gap-2 mb-4">
          <GitBranch size={15} style={{ color: 'var(--color-indigo)' }} />
          <span className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>
            VULNERABILITY TREND — 30 DAYS
          </span>
        </div>
        {isLoading ? (
          <div style={{ height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>
            Loading...
          </div>
        ) : (
          <ResponsiveContainer width="100%" height={220}>
            <AreaChart data={pts}>
              <defs>
                <linearGradient id="driftTotal" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-indigo)" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="var(--color-indigo)" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="rgba(255,255,255,0.05)" />
              <XAxis dataKey="date" tick={{ fill: 'var(--color-muted)', fontSize: 10 }} tickLine={false} />
              <YAxis tick={{ fill: 'var(--color-muted)', fontSize: 10 }} tickLine={false} axisLine={false} />
              <Tooltip
                contentStyle={{ background: 'var(--surface-2)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: 12 }}
                labelStyle={{ color: 'var(--fg)' }}
              />
              <Area type="monotone" dataKey="total" name="Total" stroke="var(--color-indigo)" fill="url(#driftTotal)" strokeWidth={2} />
              <Area type="monotone" dataKey="critical" name="Critical" stroke="var(--color-critical)" fill="none" strokeWidth={1.5} />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="rounded-lg p-4 space-y-2" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>HOW THIS DATA IS POPULATED</p>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
          There's no dedicated drift command — this timeline is built from your regular scan history
          (<code style={{ color: 'var(--color-safe)' }}>GET /api/v1/dashboard/timeline</code>). Run scans
          periodically to populate it:
        </p>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan .</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl monitor --watch</code>
      </div>
    </div>
  );
}

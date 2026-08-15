import { useQuery } from '@tanstack/react-query';
import { Terminal, Package, FileCode, Beer, Gem, Container, Binary, AlertCircle, Activity, ShieldAlert, Clock } from 'lucide-react';
import { getAuditStats, getDashboardStats, getHealth } from '../lib/api';
import { StatCard } from '../components/StatCard';
import { Skeleton } from '../components/ui/skeleton';
import { MetricCard } from '../components/MetricCard';
import { ActivityFeed } from '../components/ActivityFeed';
import { cn } from '../components/ui/utils';

const ECOSYSTEMS = [
  { label: 'npm (global)',   icon: Package,  hint: 'npm -g list' },
  { label: 'Python (pip)',   icon: FileCode,  hint: 'pip list' },
  { label: 'Homebrew',       icon: Beer,      hint: 'brew list' },
  { label: 'Ruby gems',      icon: Gem,       hint: 'gem list' },
  { label: 'Cargo bins',     icon: Binary,    hint: 'cargo install --list' },
  { label: 'Go binaries',    icon: Binary,    hint: '$GOPATH/bin' },
  { label: 'Docker images',  icon: Container, hint: 'docker images' },
];

// Continuous monitoring status — hits /healthz (always-on liveness probe,
// independent of DATABASE_URL) so "Monitoring active" reflects real API
// reachability rather than misreporting "offline" whenever no DB is configured.
function MonitoringStatusHeader() {
  const health = useQuery({
    queryKey: ['api-health', 'audit-monitor'],
    queryFn: getHealth,
    retry: 3,
    retryDelay: (attempt: number) => Math.min(1000 * 2 ** attempt, 10_000),
    refetchInterval: 15_000,
    refetchOnWindowFocus: true,
  });

  const active = !health.isError && !health.isLoading;
  const reconnecting = health.isLoading && health.failureCount > 0;

  return (
    <div className="rounded-lg p-4 flex items-center gap-3" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
      <span
        className={cn('w-2.5 h-2.5 rounded-full shrink-0', active && 'fg-pulse-healthy')}
        style={{
          background: active ? 'var(--color-safe)' : 'var(--color-warn)',
          boxShadow: active ? '0 0 6px var(--color-safe)' : undefined,
        }}
      />
      <div>
        <p className="text-sm font-mono font-bold" style={{ color: active ? 'var(--color-safe)' : 'var(--color-warn)' }}>
          {active ? '● Monitoring active' : reconnecting ? '● Reconnecting…' : '● Monitoring inactive — API unreachable'}
        </p>
        <p className="text-xs mt-0.5" style={{ color: 'var(--color-muted)' }}>
          {active
            ? 'ForgeGuardian API is reachable — stats below refresh automatically.'
            : 'Could not reach the ForgeGuardian API. Start it with `make api` or the docker-compose stack.'}
        </p>
      </div>
    </div>
  );
}

// Stats row — every number here maps to a real backend field. "events/min"
// from the design spec implies a live event stream ForgeGuardian doesn't
// have, so it is omitted entirely rather than fabricated.
function MonitoringStatsRow() {
  const stats = useQuery({
    queryKey: ['dashboard-stats', 'audit-monitor'],
    queryFn: getDashboardStats,
    refetchInterval: 30_000,
    retry: false,
  });

  const d = stats.data;

  return (
    <div className="grid grid-cols-3 gap-3">
      <MetricCard
        label="Threats detected"
        value={(d?.critical_findings ?? 0) + (d?.high_findings ?? 0)}
        variant="critical"
        icon={ShieldAlert}
      />
      <MetricCard
        label="Assets monitored"
        value={d?.total_packages ?? 0}
        icon={Package}
      />
      <MetricCard
        label="Last scan"
        value={d?.last_updated ? new Date(d.last_updated).toLocaleTimeString() : 'N/A'}
        icon={Clock}
      />
    </div>
  );
}

export function SystemAuditPage() {
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['audit-stats'],
    queryFn: getAuditStats,
    retry: false,
  });

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>
        System Audit
      </h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Audit globally installed packages across all package managers for known vulnerabilities.
      </p>

      {/* Continuous monitoring — real API-reachability status + real stats */}
      <div className="space-y-3">
        <h2 className="text-sm font-mono flex items-center gap-2" style={{ color: 'var(--color-muted)' }}>
          <Activity size={14} />
          CONTINUOUS MONITORING
        </h2>
        <MonitoringStatusHeader />
        <MonitoringStatsRow />
        <ActivityFeed />
      </div>

      {/* Signature store stats */}
      {isLoading ? (
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          {[...Array(4)].map((_, i) => <Skeleton key={i} className="h-[110px] w-full" />)}
        </div>
      ) : isError ? (
        <div
          className="rounded-lg p-4 flex items-center gap-3"
          style={{ background: 'var(--surface)', border: '1px solid rgba(255,61,61,0.2)' }}
        >
          <AlertCircle size={16} style={{ color: 'var(--color-critical)' }} />
          <span className="text-sm" style={{ color: 'var(--color-critical)' }}>{(error as Error).message}</span>
        </div>
      ) : data ? (
        <div>
          <h2 className="text-sm font-mono mb-3" style={{ color: 'var(--color-muted)' }}>
            SIGNATURE STORE
          </h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <StatCard label="Total Signatures" value={data.total_signatures} color="var(--color-safe)" />
            <StatCard label="Malware Patterns" value={data.malware_patterns} color="var(--color-critical)" />
            <StatCard label="Typosquat Targets" value={data.typosquat_targets} color="var(--color-warn)" />
            <StatCard label="Behavioral Rules" value={data.behavioral_rules} color="var(--color-low)" />
            <StatCard label="Blocklist Entries" value={data.blocklist_entries} color="var(--color-critical)" />
            <StatCard label="MCP Injection Rules" value={data.mcp_injection_rules} color="var(--color-indigo)" />
          </div>
          <div className="flex items-center gap-3 mt-3 flex-wrap">
            {data.ecosystems_covered.map(e => (
              <span key={e} className="text-xs font-mono px-2 py-0.5 rounded"
                style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--color-muted)' }}>
                {e}
              </span>
            ))}
            {data.signatures_updated && (
              <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                Updated: {new Date(data.signatures_updated).toLocaleDateString()}
              </span>
            )}
          </div>
        </div>
      ) : null}

      {/* CLI hint */}
      <div
        className="rounded-lg p-4 flex items-start gap-3"
        style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}
      >
        <Terminal size={16} style={{ color: 'var(--color-safe)', marginTop: 2 }} />
        <div>
          <p className="text-sm font-mono font-bold" style={{ color: 'var(--fg)' }}>
            Run from the CLI to refresh the signature store stats above:
          </p>
          <code
            className="text-xs mt-1 block"
            style={{ color: 'var(--color-safe)' }}
          >
            fgctl intel update
          </code>
          <p className="text-xs mt-2" style={{ color: 'var(--color-muted)' }}>
            For a full local audit of installed packages on this machine (npm/pip/brew/gem/cargo/go/docker), run:
          </p>
          <code
            className="text-xs mt-0.5 block"
            style={{ color: 'var(--color-safe)' }}
          >
            fgctl audit system
          </code>
        </div>
      </div>

      {/* Ecosystem cards */}
      <div>
        <h2 className="text-sm font-mono mb-3" style={{ color: 'var(--color-muted)' }}>
          SCANNED ECOSYSTEMS
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
          {ECOSYSTEMS.map(eco => (
            <div
              key={eco.label}
              className="rounded-lg p-4 flex items-center gap-3"
              style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}
            >
              <eco.icon size={20} style={{ color: 'var(--color-muted)' }} />
              <div>
                <p className="text-sm font-mono font-medium" style={{ color: 'var(--fg)' }}>
                  {eco.label}
                </p>
                <p className="text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
                  {eco.hint}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* PATH check note */}
      <div
        className="rounded-lg p-4"
        style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}
      >
        <p className="text-xs font-mono font-bold mb-1" style={{ color: 'var(--color-muted)' }}>
          PATH SECURITY CHECK
        </p>
        <p className="text-sm" style={{ color: 'var(--fg)' }}>
          ForgeGuardian also checks for world-writable directories in your <code>$PATH</code> that could enable hijacking attacks.
        </p>
      </div>
    </div>
  );
}

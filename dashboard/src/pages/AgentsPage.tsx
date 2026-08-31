import { Bot, Radio, Trash2 } from 'lucide-react';
import { useAgentStream } from '../hooks/useAgentStream';

const TYPE_COLOR: Record<string, string> = {
  start: 'var(--color-indigo)',
  step:  'var(--color-info)',
  patch: 'var(--color-safe)',
  done:  'var(--color-safe)',
  error: 'var(--color-critical)',
};

function relTime(s: string) {
  const diff = Date.now() - new Date(s).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return 'just now';
  if (m < 60) return `${m}m ago`;
  return `${Math.floor(m / 60)}h ago`;
}

export function AgentsPage() {
  const { events, connected, error, clear } = useAgentStream();
  // events are newest-first (useAgentStream prepends) and the log spans
  // every session ever seen this page load, so "active" must be scoped to
  // the most recent session_id — otherwise an old session's terminal
  // 'done'/'error' event permanently masks a brand-new session's 'start'.
  const latestSessionId = events.find(e => e.session_id)?.session_id;
  const latestSessionEvents = latestSessionId
    ? events.filter(e => e.session_id === latestSessionId)
    : events;
  const active = latestSessionEvents.some(e => e.type === 'start') &&
                 !latestSessionEvents.some(e => e.type === 'done' || e.type === 'error');

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>AI Patch Agents</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>
            Live feed of autonomous patch agent sessions.
          </p>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <span style={{
            width: 8, height: 8, borderRadius: '50%',
            background: connected ? 'var(--color-safe)' : 'var(--color-critical)',
            display: 'inline-block',
          }} />
          <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)' }}>
            {connected ? 'LIVE' : 'DISCONNECTED'}
          </span>
        </div>
      </div>

      {/* Session status */}
      <div className="rounded-lg p-5" style={{ background: 'var(--surface)', border: `1px solid ${active ? 'rgba(0,255,135,0.2)' : 'rgba(255,255,255,0.06)'}` }}>
        <div className="flex items-center gap-3 mb-2">
          {active
            ? <Radio size={16} style={{ color: 'var(--color-safe)' }} />
            : <Bot size={16} style={{ color: 'var(--color-muted)' }} />}
          <span className="text-sm font-mono font-bold" style={{ color: active ? 'var(--color-safe)' : 'var(--color-muted)' }}>
            {active ? 'AGENT SESSION RUNNING' : 'NO ACTIVE AGENT SESSION'}
          </span>
        </div>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
          {active
            ? 'Patch agent is active. Events stream below in real time.'
            : 'Start a session from the CLI. Events will appear here automatically.'}
        </p>
        {error && <p className="text-xs mt-2" style={{ color: 'var(--color-critical)' }}>{error}</p>}
      </div>

      {/* Live event feed */}
      {events.length > 0 && (
        <div className="rounded-lg overflow-hidden" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <div style={{ padding: '0.5rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span style={{ fontSize: '0.7rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', fontWeight: 600 }}>
              AGENT EVENT LOG ({events.length})
            </span>
            <button onClick={clear} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-muted)', display: 'flex', alignItems: 'center', gap: 4 }}>
              <Trash2 size={12} />
              <span style={{ fontSize: '0.7rem', fontFamily: 'var(--font-mono)' }}>clear</span>
            </button>
          </div>
          <div style={{ maxHeight: '320px', overflowY: 'auto' }}>
            {events.map((ev, i) => (
              <div key={i} style={{ padding: '0.5rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.03)', display: 'flex', gap: '0.75rem', alignItems: 'flex-start' }}>
                <span style={{
                  fontSize: '0.65rem', fontWeight: 700, fontFamily: 'var(--font-mono)',
                  color: TYPE_COLOR[ev.type] ?? 'var(--fg)',
                  minWidth: 52, paddingTop: 2, textTransform: 'uppercase',
                }}>{ev.type}</span>
                <div style={{ flex: 1 }}>
                  <p style={{ fontSize: '0.78rem', color: 'var(--fg)' }}>{ev.message}</p>
                  {ev.package && (
                    <p style={{ fontSize: '0.7rem', color: 'var(--color-muted)', marginTop: 2 }}>
                      {ev.package}{ev.version ? `@${ev.version}` : ''}
                    </p>
                  )}
                </div>
                <span style={{ fontSize: '0.65rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', whiteSpace: 'nowrap' }}>
                  {relTime(ev.occurred_at)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* CLI hint */}
      <div className="rounded-lg p-4 space-y-2" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>START A PATCH SESSION</p>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl patch .                      # patch current project</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl patch . --severity=high      # high+ findings only</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl patch . --dry-run            # preview proposed changes</code>
        <p className="text-xs mt-1" style={{ color: 'var(--color-muted)' }}>Requires an AI provider API key. See Settings for configuration.</p>
      </div>
    </div>
  );
}

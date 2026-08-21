import { useState, useMemo, useEffect, useRef } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  Terminal, Search, Filter, Download, Pause, Play, Trash2,
  AlertTriangle, Info, AlertCircle, Bug, ChevronDown,
} from 'lucide-react';
import { getServerLogs, getDashboardActivity, type LogEntry } from '../lib/api';
import { Input } from '../components/ui/input';
import type { ActivityEvent } from '../types/api';

const LEVEL_COLORS: Record<string, string> = {
  DEBUG: '#06B6D4',
  INFO: '#22C55E',
  WARN: '#D97706',
  ERROR: '#DC2626',
};

const LEVEL_ICONS: Record<string, typeof Info> = {
  DEBUG: Bug,
  INFO: Info,
  WARN: AlertTriangle,
  ERROR: AlertCircle,
};

function formatTime(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  } catch { return iso; }
}

function formatDate(iso: string): string {
  try {
    const d = new Date(iso);
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  } catch { return ''; }
}

type CombinedEntry = {
  id: string;
  time: string;
  level: string;
  message: string;
  source: 'server' | 'activity';
  fields?: Record<string, unknown>;
};

export function LogMonitorPage() {
  const [search, setSearch] = useState('');
  const [levelFilter, setLevelFilter] = useState<string>('all');
  const [paused, setPaused] = useState(false);
  const [autoScroll, setAutoScroll] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  const serverLogs = useQuery({
    queryKey: ['server-logs'],
    queryFn: () => getServerLogs(300),
    refetchInterval: paused ? false : 3000,
    retry: 1,
  });

  const activityLogs = useQuery({
    queryKey: ['activity-logs'],
    queryFn: () => getDashboardActivity(50),
    refetchInterval: paused ? false : 5000,
    retry: 1,
  });

  const combined = useMemo(() => {
    const entries: CombinedEntry[] = [];

    if (serverLogs.data?.logs) {
      for (let i = 0; i < serverLogs.data.logs.length; i++) {
        const e = serverLogs.data.logs[i];
        entries.push({
          id: `srv-${i}-${e.time}`,
          time: e.time,
          level: e.level,
          message: e.msg,
          source: 'server',
          fields: e.fields,
        });
      }
    }

    if (activityLogs.data?.events) {
      for (const ev of activityLogs.data.events) {
        entries.push({
          id: `act-${ev.id}`,
          time: ev.occurred_at,
          level: ev.severity === 'CRITICAL' || ev.severity === 'HIGH' ? 'ERROR' :
                 ev.severity === 'MEDIUM' ? 'WARN' : 'INFO',
          message: ev.message,
          source: 'activity',
          fields: { type: ev.type, package: ev.package_name, ecosystem: ev.ecosystem },
        });
      }
    }

    entries.sort((a, b) => new Date(a.time).getTime() - new Date(b.time).getTime());
    return entries;
  }, [serverLogs.data, activityLogs.data]);

  const filtered = useMemo(() => {
    let list = combined;
    if (levelFilter !== 'all') {
      list = list.filter(e => e.level === levelFilter);
    }
    if (search) {
      const q = search.toLowerCase();
      list = list.filter(e =>
        e.message.toLowerCase().includes(q) ||
        JSON.stringify(e.fields || {}).toLowerCase().includes(q)
      );
    }
    return list;
  }, [combined, levelFilter, search]);

  const stats = useMemo(() => ({
    total: combined.length,
    error: combined.filter(e => e.level === 'ERROR').length,
    warn: combined.filter(e => e.level === 'WARN').length,
    info: combined.filter(e => e.level === 'INFO').length,
  }), [combined]);

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [filtered, autoScroll]);

  const handleScroll = () => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  };

  const exportLogs = () => {
    const text = filtered.map(e =>
      `[${e.time}] [${e.level}] ${e.message}${e.fields ? ' ' + JSON.stringify(e.fields) : ''}`
    ).join('\n');
    const blob = new Blob([text], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `forgeguardian-logs-${new Date().toISOString().slice(0, 10)}.log`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="flex flex-col h-[calc(100vh-2rem)] p-6 max-w-[1600px] mx-auto">
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <div>
          <div className="flex items-center gap-2.5">
            <Terminal size={20} className="text-primary-blue" />
            <h1 className="text-xl font-bold text-text-primary m-0">Log Monitor</h1>
            <span className={`flex items-center gap-1 px-2 py-0.5 rounded-full text-[0.62rem] font-medium ${
              paused ? 'bg-warning/10 text-warning' : 'bg-success/10 text-success'
            }`}>
              <span className={`w-1.5 h-1.5 rounded-full ${paused ? 'bg-warning' : 'bg-success animate-pulse'}`} />
              {paused ? 'Paused' : 'Live'}
            </span>
          </div>
          <p className="text-sm text-text-secondary mt-1">
            Server logs and scan activity events &middot; {combined.length} entries
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setPaused(!paused)}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[0.72rem] font-medium border cursor-pointer [font-family:inherit] transition-colors ${
              paused
                ? 'bg-success/10 text-success border-success/30 hover:bg-success/20'
                : 'bg-warning/10 text-warning border-warning/30 hover:bg-warning/20'
            }`}
          >
            {paused ? <Play size={13} /> : <Pause size={13} />}
            {paused ? 'Resume' : 'Pause'}
          </button>
          <button
            onClick={exportLogs}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-[0.72rem] text-text-secondary border border-border-color hover:text-primary-blue hover:border-primary-blue/40 bg-transparent cursor-pointer [font-family:inherit] transition-colors"
          >
            <Download size={13} /> Export
          </button>
        </div>
      </div>

      {/* Stats strip */}
      <div className="grid grid-cols-4 gap-3 mb-4">
        {[
          { label: 'Total Entries', value: stats.total, color: 'var(--primary-blue)' },
          { label: 'Errors', value: stats.error, color: LEVEL_COLORS.ERROR },
          { label: 'Warnings', value: stats.warn, color: LEVEL_COLORS.WARN },
          { label: 'Info', value: stats.info, color: LEVEL_COLORS.INFO },
        ].map(s => (
          <div key={s.label} className="rounded-lg border border-border-color bg-surface p-3">
            <p className="text-[0.62rem] text-text-muted uppercase tracking-wider m-0">{s.label}</p>
            <p className="text-xl font-bold font-mono m-0 mt-0.5" style={{ color: s.color }}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Toolbar */}
      <div className="flex items-center gap-3 mb-3">
        <div className="relative flex-1 max-w-sm">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <Input
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder="Search logs..."
            className="pl-8 h-8 text-xs"
          />
        </div>
        <div className="flex items-center gap-1.5">
          <Filter size={12} className="text-text-muted" />
          {['all', 'ERROR', 'WARN', 'INFO', 'DEBUG'].map(level => (
            <button
              key={level}
              onClick={() => setLevelFilter(level)}
              className={`px-2 py-1 rounded text-[0.68rem] border cursor-pointer [font-family:inherit] transition-colors ${
                levelFilter === level
                  ? 'bg-primary-blue/10 text-primary-blue border-primary-blue/30'
                  : 'bg-transparent text-text-secondary border-border-color hover:border-text-muted'
              }`}
            >
              {level === 'all' ? 'All' : level}
            </button>
          ))}
        </div>
      </div>

      {/* Log viewer */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-auto rounded-lg border border-border-color bg-[#0a0c0e] font-mono text-[0.72rem]"
      >
        {filtered.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-20 text-text-muted">
            <Terminal size={32} className="mb-3 opacity-40" />
            <p className="text-sm font-medium">No log entries</p>
            <p className="text-xs mt-1">
              {serverLogs.isError ? 'Server logs unavailable — API may be offline' : 'Waiting for log events...'}
            </p>
          </div>
        ) : (
          <table className="w-full" style={{ borderCollapse: 'collapse' }}>
            <tbody>
              {filtered.map(entry => {
                const Icon = LEVEL_ICONS[entry.level] || Info;
                const color = LEVEL_COLORS[entry.level] || '#6B7280';
                const isExpanded = expandedId === entry.id;
                const hasFields = entry.fields && Object.keys(entry.fields).length > 0;

                return (
                  <tr
                    key={entry.id}
                    className={`border-b border-white/5 hover:bg-white/[0.03] cursor-pointer transition-colors ${
                      entry.level === 'ERROR' ? 'bg-red-500/[0.04]' : ''
                    }`}
                    onClick={() => hasFields && setExpandedId(isExpanded ? null : entry.id)}
                  >
                    <td className="px-3 py-1.5 w-[70px] text-text-muted whitespace-nowrap align-top">
                      <span title={entry.time}>{formatTime(entry.time)}</span>
                    </td>
                    <td className="px-2 py-1.5 w-[60px] whitespace-nowrap align-top">
                      <span
                        className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[0.6rem] font-bold"
                        style={{ color, background: `${color}15` }}
                      >
                        <Icon size={10} />
                        {entry.level}
                      </span>
                    </td>
                    <td className="px-2 py-1.5 w-[50px] whitespace-nowrap align-top">
                      <span className={`text-[0.58rem] px-1 py-0.5 rounded ${
                        entry.source === 'server' ? 'bg-primary-blue/10 text-primary-blue' : 'bg-[#7C3AED]/10 text-[#7C3AED]'
                      }`}>
                        {entry.source === 'server' ? 'SRV' : 'ACT'}
                      </span>
                    </td>
                    <td className="px-2 py-1.5 text-[#E5E7EB] align-top">
                      <div>{entry.message}</div>
                      {isExpanded && hasFields && (
                        <pre className="mt-1.5 p-2 rounded bg-white/[0.04] text-[0.65rem] text-text-muted overflow-x-auto">
                          {JSON.stringify(entry.fields, null, 2)}
                        </pre>
                      )}
                      {hasFields && !isExpanded && (
                        <span className="text-text-muted text-[0.58rem] ml-2">
                          <ChevronDown size={9} className="inline" /> {Object.keys(entry.fields!).length} fields
                        </span>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between mt-2 text-[0.65rem] text-text-muted">
        <span>
          Showing {filtered.length} of {combined.length} entries
          {search && ` matching "${search}"`}
        </span>
        <span className="flex items-center gap-2">
          {!autoScroll && (
            <button
              onClick={() => {
                setAutoScroll(true);
                if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
              }}
              className="text-primary-blue hover:underline bg-transparent border-none cursor-pointer [font-family:inherit] text-[0.65rem]"
            >
              Jump to latest
            </button>
          )}
          <span>Refresh: {paused ? 'paused' : '3s'}</span>
        </span>
      </div>
    </div>
  );
}

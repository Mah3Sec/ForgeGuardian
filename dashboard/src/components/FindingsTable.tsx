import { useState, useMemo, useCallback } from 'react';
import { ChevronDown, ChevronRight, ArrowUpDown, ArrowUp, ArrowDown, ShieldBan, ShieldAlert, ShieldCheck } from 'lucide-react';
import type { Finding } from '../types/api';
import { SeverityBadge } from './SeverityBadge';
import { quarantinePackage, blockPackage, unquarantinePackage } from '../lib/api';

const SEV_ORDER: Record<string, number> = { CRITICAL: 4, HIGH: 3, MEDIUM: 2, LOW: 1, INFORMATIONAL: 0 };

type SortKey = 'severity' | 'id' | 'title' | 'source';
type SortDir = 'asc' | 'desc';

interface Props {
  findings: Finding[];
  maxRows?: number;
  showActions?: boolean;
  denyList?: string[];
  onPolicyChange?: () => void;
}

export function FindingsTable({ findings, maxRows, showActions, denyList, onPolicyChange }: Props) {
  const [visibleCount, setVisibleCount] = useState(50);
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [sortKey, setSortKey] = useState<SortKey>('severity');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const toggleSort = useCallback((key: SortKey) => {
    if (sortKey === key) {
      setSortDir(d => d === 'desc' ? 'asc' : 'desc');
    } else {
      setSortKey(key);
      setSortDir('desc');
    }
  }, [sortKey]);

  const sorted = useMemo(() => {
    const arr = [...findings];
    const dir = sortDir === 'desc' ? -1 : 1;
    arr.sort((a, b) => {
      switch (sortKey) {
        case 'severity': return dir * ((SEV_ORDER[a.severity] ?? 0) - (SEV_ORDER[b.severity] ?? 0));
        case 'id': return dir * a.id.localeCompare(b.id);
        case 'title': return dir * a.title.localeCompare(b.title);
        case 'source': return dir * a.source.localeCompare(b.source);
        default: return 0;
      }
    });
    return arr;
  }, [findings, sortKey, sortDir]);

  const rows = maxRows ? sorted.slice(0, maxRows) : sorted.slice(0, visibleCount);
  const hasMore = !maxRows && findings.length > visibleCount;

  const toggle = (uid: string) => {
    setExpanded(prev => {
      const next = new Set(prev);
      if (next.has(uid)) next.delete(uid); else next.add(uid);
      return next;
    });
  };

  if (rows.length === 0) {
    return (
      <div className="text-center py-8 text-sm" style={{ color: 'var(--color-muted)' }}>
        No findings
      </div>
    );
  }

  const SortIcon = ({ col }: { col: SortKey }) => {
    if (sortKey !== col) return <ArrowUpDown size={10} style={{ opacity: 0.3 }} />;
    return sortDir === 'desc' ? <ArrowDown size={10} /> : <ArrowUp size={10} />;
  };

  const headers: { key: SortKey; label: string }[] = [
    { key: 'severity', label: 'Severity' },
    { key: 'id', label: 'ID' },
    { key: 'title', label: 'Title' },
    { key: 'source', label: 'Source' },
  ];

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
            <th style={{ width: 28 }} />
            {headers.map(h => (
              <th
                key={h.key}
                className="text-left py-2 px-3 font-mono text-xs uppercase"
                style={{ color: 'var(--color-muted)', cursor: 'pointer', userSelect: 'none' }}
                onClick={() => toggleSort(h.key)}
              >
                <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4 }}>
                  {h.label} <SortIcon col={h.key} />
                </span>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((f, i) => {
            const uid = f.id + i;
            const isOpen = expanded.has(uid);
            return (
              <>
                <tr
                  key={uid}
                  style={{ borderBottom: isOpen ? 'none' : '1px solid rgba(255,255,255,0.04)', cursor: 'pointer' }}
                  className="hover:bg-white/[0.02] transition-colors"
                  onClick={() => toggle(uid)}
                >
                  <td className="py-2 pl-3" style={{ color: 'var(--color-muted)' }}>
                    {isOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                  </td>
                  <td className="py-2 px-3"><SeverityBadge severity={f.severity} /></td>
                  <td className="py-2 px-3 font-mono text-xs" style={{ color: 'var(--color-muted)' }}>{f.id}</td>
                  <td className="py-2 px-3" style={{ color: 'var(--fg)' }}>{f.title}</td>
                  <td className="py-2 px-3 text-xs" style={{ color: 'var(--color-muted)' }}>{f.source}</td>
                </tr>
                {isOpen && (
                  <tr key={uid + '-detail'} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                    <td colSpan={5} style={{ padding: 0 }}>
                      <div style={{
                        margin: '0 0.75rem 0.75rem 2.25rem',
                        padding: '0.75rem 1rem',
                        borderRadius: '0.375rem',
                        background: 'rgba(255,255,255,0.02)',
                        border: '1px solid rgba(255,255,255,0.06)',
                        fontSize: '0.78rem',
                      }}>
                        {f.description && (
                          <div style={{ marginBottom: '0.5rem' }}>
                            <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>Description</span>
                            <p style={{ color: 'var(--fg)', marginTop: 2, lineHeight: 1.5 }}>{f.description}</p>
                          </div>
                        )}
                        <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
                          {f.fixed_version && (
                            <div>
                              <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>Fix available</span>
                              <p style={{ color: 'var(--color-safe)', marginTop: 2, fontFamily: 'var(--font-mono)' }}>
                                Upgrade to {f.fixed_version}
                              </p>
                            </div>
                          )}
                          <div>
                            <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>Type</span>
                            <p style={{ color: 'var(--fg)', marginTop: 2, fontFamily: 'var(--font-mono)' }}>{f.type || '—'}</p>
                          </div>
                          <div>
                            <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>Engine</span>
                            <p style={{ color: 'var(--fg)', marginTop: 2, fontFamily: 'var(--font-mono)' }}>{f.source}</p>
                          </div>
                        </div>
                        {f.metadata && Object.keys(f.metadata).length > 0 && (() => {
                          const m = f.metadata!;
                          const knownKeys = ['artifact', 'package_name', 'ecosystem', 'file_path', 'location', 'file', 'path', 'urls', 'format', 'error'];
                          const shown = knownKeys.filter(k => m[k] != null);
                          const rest = Object.keys(m).filter(k => !knownKeys.includes(k));
                          return (
                            <div style={{ marginTop: '0.5rem' }}>
                              <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>Details</span>
                              <div style={{ display: 'flex', gap: '1.5rem', flexWrap: 'wrap', marginTop: 4 }}>
                                {shown.map(k => (
                                  <div key={k}>
                                    <span style={{ color: 'var(--color-muted)', fontSize: '0.6rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase' }}>{k.replace(/_/g, ' ')}</span>
                                    <p style={{ color: 'var(--fg)', marginTop: 1, fontFamily: 'var(--font-mono)', fontSize: '0.72rem', wordBreak: 'break-all' }}>
                                      {typeof m[k] === 'object' ? JSON.stringify(m[k]) : String(m[k])}
                                    </p>
                                  </div>
                                ))}
                              </div>
                              {rest.length > 0 && (
                                <pre style={{ marginTop: 6, color: 'var(--fg)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', whiteSpace: 'pre-wrap', wordBreak: 'break-all', opacity: 0.7 }}>
                                  {JSON.stringify(Object.fromEntries(rest.map(k => [k, m[k]])), null, 2)}
                                </pre>
                              )}
                            </div>
                          );
                        })()}
                        {showActions && (() => {
                          const pkgName = (f.metadata?.package_name as string) || (f.metadata?.artifact as string) || f.id.split('-')[0] || '';
                          if (!pkgName) return null;
                          const isDenied = denyList?.some(d => d.toLowerCase() === pkgName.toLowerCase());
                          const loading = actionLoading === pkgName;
                          const doAction = async (action: 'quarantine' | 'block' | 'unquarantine') => {
                            setActionLoading(pkgName);
                            try {
                              if (action === 'quarantine') await quarantinePackage(pkgName, f.title);
                              else if (action === 'block') await blockPackage(pkgName, f.title);
                              else await unquarantinePackage(pkgName);
                              onPolicyChange?.();
                            } catch { /* toast would be nice but not critical */ }
                            setActionLoading(null);
                          };
                          return (
                            <div style={{ marginTop: '0.75rem', paddingTop: '0.5rem', borderTop: '1px solid rgba(255,255,255,0.06)', display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                              <span style={{ color: 'var(--color-muted)', fontSize: '0.68rem', fontFamily: 'var(--font-mono)', textTransform: 'uppercase', marginRight: 4 }}>Actions</span>
                              {isDenied ? (
                                <button
                                  disabled={loading}
                                  onClick={(e) => { e.stopPropagation(); doAction('unquarantine'); }}
                                  style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 4, border: '1px solid rgba(0,255,135,0.3)', background: 'rgba(0,255,135,0.08)', color: '#00FF87', fontSize: '0.72rem', fontFamily: 'var(--font-mono)', cursor: loading ? 'wait' : 'pointer' }}
                                >
                                  <ShieldCheck size={12} /> Unquarantine
                                </button>
                              ) : (
                                <>
                                  <button
                                    disabled={loading}
                                    onClick={(e) => { e.stopPropagation(); doAction('quarantine'); }}
                                    style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 4, border: '1px solid rgba(255,165,0,0.3)', background: 'rgba(255,165,0,0.08)', color: '#FFA500', fontSize: '0.72rem', fontFamily: 'var(--font-mono)', cursor: loading ? 'wait' : 'pointer' }}
                                  >
                                    <ShieldAlert size={12} /> Quarantine
                                  </button>
                                  <button
                                    disabled={loading}
                                    onClick={(e) => { e.stopPropagation(); doAction('block'); }}
                                    style={{ display: 'inline-flex', alignItems: 'center', gap: 4, padding: '3px 10px', borderRadius: 4, border: '1px solid rgba(255,61,61,0.3)', background: 'rgba(255,61,61,0.08)', color: '#FF3D3D', fontSize: '0.72rem', fontFamily: 'var(--font-mono)', cursor: loading ? 'wait' : 'pointer' }}
                                  >
                                    <ShieldBan size={12} /> Block
                                  </button>
                                </>
                              )}
                            </div>
                          );
                        })()}
                      </div>
                    </td>
                  </tr>
                )}
              </>
            );
          })}
        </tbody>
      </table>
      {maxRows && findings.length > maxRows && (
        <p className="text-xs text-center py-2" style={{ color: 'var(--color-muted)' }}>
          + {findings.length - maxRows} more findings
        </p>
      )}
      {hasMore && (
        <div className="text-center py-3">
          <button
            onClick={() => setVisibleCount(c => c + 50)}
            className="text-xs px-4 py-1.5 rounded font-mono"
            style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.1)', cursor: 'pointer' }}
          >
            Load 50 more ({findings.length - visibleCount} remaining)
          </button>
        </div>
      )}
    </div>
  );
}

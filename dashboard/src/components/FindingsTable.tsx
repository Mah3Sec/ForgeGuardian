import { useState } from 'react';
import type { Finding } from '../types/api';
import { SeverityBadge } from './SeverityBadge';

interface Props {
  findings: Finding[];
  maxRows?: number;
}

export function FindingsTable({ findings, maxRows }: Props) {
  const [visibleCount, setVisibleCount] = useState(50);
  // When maxRows is provided (e.g. dashboard summary), use that fixed limit.
  // Otherwise use the load-more paginated visibleCount.
  const rows = maxRows ? findings.slice(0, maxRows) : findings.slice(0, visibleCount);
  const hasMore = !maxRows && findings.length > visibleCount;

  if (rows.length === 0) {
    return (
      <div className="text-center py-8 text-sm" style={{ color: 'var(--color-muted)' }}>
        No findings
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
            {['Severity', 'ID', 'Title', 'Source'].map(h => (
              <th key={h} className="text-left py-2 px-3 font-mono text-xs uppercase"
                  style={{ color: 'var(--color-muted)' }}>{h}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((f, i) => (
            <tr
              key={f.id + i}
              style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}
              className="hover:bg-white/[0.02] transition-colors"
            >
              <td className="py-2 px-3"><SeverityBadge severity={f.severity} /></td>
              <td className="py-2 px-3 font-mono text-xs" style={{ color: 'var(--color-muted)' }}>{f.id}</td>
              <td className="py-2 px-3" style={{ color: 'var(--fg)' }}>{f.title}</td>
              <td className="py-2 px-3 text-xs" style={{ color: 'var(--color-muted)' }}>{f.source}</td>
            </tr>
          ))}
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

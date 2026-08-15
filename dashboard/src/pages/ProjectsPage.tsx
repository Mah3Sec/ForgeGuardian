import { useQuery } from '@tanstack/react-query';
import { getActiveRisks } from '../lib/api';
import { FolderOpen } from 'lucide-react';

const GRADE_COLORS: Record<string, string> = {
  A: 'var(--color-safe)',
  B: 'var(--color-info)',
  C: 'var(--color-medium)',
  D: 'var(--color-warn)',
  F: 'var(--color-critical)',
};

export function ProjectsPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['active-risks'],
    queryFn: getActiveRisks,
    refetchInterval: 60_000,
  });

  // Group by package_name as proxy for project
  const projectMap = new Map<string, { grade: string; critical: number; total: number; ecosystem: string }>();
  for (const r of data?.risks ?? []) {
    const existing = projectMap.get(r.package_name);
    if (!existing) {
      projectMap.set(r.package_name, {
        grade: r.risk_grade,
        critical: r.top_severity === 'CRITICAL' ? r.finding_count : 0,
        total: r.finding_count,
        ecosystem: r.ecosystem,
      });
    } else {
      existing.total += r.finding_count;
      if (r.top_severity === 'CRITICAL') existing.critical += r.finding_count;
    }
  }
  const projects = Array.from(projectMap.entries())
    .sort(([, a], [, b]) => b.total - a.total);

  return (
    <div className="p-6 space-y-5">
      <div className="flex items-center gap-3">
        <FolderOpen size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Projects</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>
            Scanned packages and their risk posture.
          </p>
        </div>
      </div>

      <div className="rounded-lg overflow-hidden" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
              {['Project', 'Ecosystem', 'Risk Grade', 'Critical', 'Total Findings'].map(h => (
                <th key={h} style={{ padding: '0.625rem 0.875rem', textAlign: 'left', fontSize: '0.7rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {isLoading && (
              <tr><td colSpan={5} style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>Loading...</td></tr>
            )}
            {!isLoading && projects.length === 0 && (
              <tr><td colSpan={5} style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-muted)', fontSize: '0.8rem' }}>No projects scanned yet. Run <code style={{ color: 'var(--color-safe)' }}>fgctl scan .</code></td></tr>
            )}
            {projects.map(([name, p], i) => (
              <tr key={name} style={{ borderBottom: i < projects.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none' }}>
                <td style={{ padding: '0.625rem 0.875rem', fontSize: '0.8rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)' }}>{name}</td>
                <td style={{ padding: '0.625rem 0.875rem', fontSize: '0.72rem', color: 'var(--color-muted)' }}>{p.ecosystem}</td>
                <td style={{ padding: '0.625rem 0.875rem' }}>
                  <span style={{ fontSize: '0.75rem', fontWeight: 700, color: GRADE_COLORS[p.grade] ?? 'var(--fg)', fontFamily: 'var(--font-mono)' }}>{p.grade}</span>
                </td>
                <td style={{ padding: '0.625rem 0.875rem', fontSize: '0.8rem', color: p.critical > 0 ? 'var(--color-critical)' : 'var(--color-muted)', fontFamily: 'var(--font-mono)' }}>{p.critical}</td>
                <td style={{ padding: '0.625rem 0.875rem', fontSize: '0.8rem', color: 'var(--fg)', fontFamily: 'var(--font-mono)' }}>{p.total}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

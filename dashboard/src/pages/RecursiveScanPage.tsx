import { useState, useEffect, useRef } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Layers, AlertTriangle, CheckCircle } from 'lucide-react';
import { triggerScan, getJobStatus } from '../lib/api';
import type { ScanResult } from '../types/api';

// Scans run async — submit returns a job_id, poll until it settles.
// Bounded to 5 minutes (150 * 2s) so a stuck worker can't poll forever, and
// checks isMounted so navigating away stops polling instead of leaking a
// setTimeout loop tied to nothing.
const MAX_POLLS = 150;

async function scanAndAwait(eco: string, pkg: string, ver: string, isMounted: () => boolean): Promise<ScanResult> {
  const job = await triggerScan(eco, pkg, ver);
  let current = job;
  let polls = 0;
  while (current.status !== 'complete' && current.status !== 'failed') {
    if (!isMounted()) throw new Error('cancelled — navigated away');
    if (++polls > MAX_POLLS) throw new Error(`scan for ${pkg}@${ver} did not complete after 5 minutes`);
    await new Promise(r => setTimeout(r, 2_000));
    current = await getJobStatus(job.job_id);
  }
  if (current.status === 'failed') throw new Error(current.error || `scan failed for ${pkg}@${ver}`);
  return current.result!;
}

const SEV_COLOR: Record<string, string> = {
  CRITICAL: 'var(--color-critical)',
  HIGH: 'var(--color-high)',
  MEDIUM: 'var(--color-medium)',
  LOW: 'var(--color-low)',
  INFORMATIONAL: 'var(--color-info)',
};

export function RecursiveScanPage() {
  const [eco, setEco] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [ver, setVer] = useState('');
  const [results, setResults] = useState<ScanResult[]>([]);

  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);

  const { mutate, isPending, error } = useMutation({
    mutationFn: async () => {
      // Scan the root package plus any comma-separated extra packages
      const pkgs = pkg.split(',').map(p => p.trim()).filter(Boolean);
      const scans = await Promise.all(
        pkgs.map(p => scanAndAwait(eco, p, ver, () => mountedRef.current))
      );
      return scans;
    },
    onSuccess: (scans) => setResults(scans),
  });

  const totalFindings = results.reduce((s, r) => s + (r.findings?.length ?? 0), 0);
  const criticalCount = results.reduce((s, r) => s + (r.findings?.filter(f => f.severity === 'CRITICAL').length ?? 0), 0);

  const inputStyle = {
    background: 'var(--surface)',
    border: '1px solid rgba(255,255,255,0.1)',
    borderRadius: '0.375rem',
    color: 'var(--fg)',
    fontSize: '0.8rem',
    padding: '0.4rem 0.625rem',
    fontFamily: 'var(--font-mono)',
    outline: 'none',
  };

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Layers size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Recursive Scanning</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>
            Scan multiple packages at once — surface hidden vulnerabilities across transitive dependencies.
          </p>
        </div>
      </div>

      {/* Input form */}
      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div className="grid grid-cols-1 sm:grid-cols-[120px_1fr_140px] gap-3 items-end">
          <div>
            <label style={{ fontSize: '0.7rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', display: 'block', marginBottom: 4 }}>ECOSYSTEM</label>
            <select value={eco} onChange={e => setEco(e.target.value)} style={{ ...inputStyle, width: '100%' }}>
              {['npm','pypi','go','maven','crates','rubygems','huggingface','mcp'].map(e => (
                <option key={e} value={e}>{e}</option>
              ))}
            </select>
          </div>
          <div>
            <label style={{ fontSize: '0.7rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', display: 'block', marginBottom: 4 }}>PACKAGES (comma-separated)</label>
            <input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="lodash, express, axios" style={{ ...inputStyle, width: '100%' }} />
          </div>
          <div>
            <label style={{ fontSize: '0.7rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)', display: 'block', marginBottom: 4 }}>VERSION</label>
            <input value={ver} onChange={e => setVer(e.target.value)} placeholder="4.17.21" style={{ ...inputStyle, width: '100%' }} />
          </div>
        </div>
        <button
          onClick={() => mutate()}
          disabled={isPending || !pkg || !ver}
          style={{
            background: isPending ? 'rgba(99,102,241,0.3)' : 'rgba(99,102,241,0.8)',
            border: 'none', borderRadius: '0.375rem',
            color: '#fff', fontSize: '0.8rem', fontFamily: 'var(--font-mono)',
            padding: '0.5rem 1.25rem', cursor: isPending ? 'not-allowed' : 'pointer',
          }}
        >
          {isPending ? 'Scanning…' : 'Run Recursive Scan'}
        </button>
        {error && (
          <p style={{ fontSize: '0.75rem', color: 'var(--color-critical)' }}>
            {(error as Error).message}
          </p>
        )}
      </div>

      {/* Summary bar */}
      {results.length > 0 && (
        <div style={{ display: 'flex', gap: '1rem' }}>
          {[
            { label: 'Packages scanned', val: results.length, color: 'var(--color-indigo)' },
            { label: 'Total findings', val: totalFindings, color: 'var(--color-warn)' },
            { label: 'Critical', val: criticalCount, color: 'var(--color-critical)' },
          ].map(s => (
            <div key={s.label} className="rounded-lg p-4 flex-1 text-center" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
              <p style={{ fontSize: '1.5rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color: s.color }}>{s.val}</p>
              <p style={{ fontSize: '0.7rem', color: 'var(--color-muted)', marginTop: 2 }}>{s.label}</p>
            </div>
          ))}
        </div>
      )}

      {/* Per-package results */}
      {results.map((r, i) => (
        <div key={i} className="rounded-lg overflow-hidden" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <div style={{ padding: '0.625rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            {(r.findings?.length ?? 0) === 0
              ? <CheckCircle size={14} style={{ color: 'var(--color-safe)' }} />
              : <AlertTriangle size={14} style={{ color: 'var(--color-critical)' }} />}
            <span style={{ fontSize: '0.8rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', fontWeight: 600 }}>{r.package}</span>
            <span style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginLeft: 'auto' }}>{r.findings?.length ?? 0} findings</span>
          </div>
          {(r.findings ?? []).slice(0, 5).map((f, j) => (
            <div key={j} style={{ padding: '0.5rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.04)', display: 'flex', gap: '0.75rem', alignItems: 'flex-start' }}>
              <span style={{ fontSize: '0.65rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color: SEV_COLOR[f.severity] ?? 'var(--fg)', minWidth: 72, paddingTop: 2 }}>{f.severity}</span>
              <div>
                <p style={{ fontSize: '0.78rem', color: 'var(--fg)' }}>{f.title}</p>
                <p style={{ fontSize: '0.7rem', color: 'var(--color-muted)', marginTop: 2 }}>{f.source}</p>
              </div>
            </div>
          ))}
          {(r.findings?.length ?? 0) > 5 && (
            <p style={{ padding: '0.5rem 0.875rem', fontSize: '0.72rem', color: 'var(--color-muted)' }}>
              + {(r.findings?.length ?? 0) - 5} more — use <code style={{ color: 'var(--color-safe)' }}>fgctl scan</code> for full output
            </p>
          )}
        </div>
      ))}

      {/* CLI hint */}
      <div className="rounded-lg p-4 space-y-2" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>CLI EQUIVALENT</p>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan . --recursive</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan . --recursive --depth=all --format json</code>
      </div>
    </div>
  );
}

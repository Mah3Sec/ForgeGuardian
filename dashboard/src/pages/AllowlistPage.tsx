import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ListFilter, ShieldCheck, ShieldX, Plus, Trash2 } from 'lucide-react';
import { listAllowlist, addAllowlist, deleteAllowlist } from '../lib/api';

const inputStyle = {
  background: 'var(--surface-2, #1e2024)',
  border: '1px solid rgba(255,255,255,0.1)',
  borderRadius: '0.375rem',
  color: 'var(--fg)',
  fontSize: '0.8rem',
  padding: '0.4rem 0.625rem',
  fontFamily: 'var(--font-mono)',
  outline: 'none',
  width: '100%',
} as React.CSSProperties;

export function AllowlistPage() {
  const qc = useQueryClient();
  const [eco, setEco] = useState('');
  const [pkg, setPkg] = useState('');
  const [reason, setReason] = useState('');
  const [formErr, setFormErr] = useState('');

  const { data, isLoading } = useQuery({
    queryKey: ['allowlist'],
    queryFn: listAllowlist,
    staleTime: 30_000,
  });

  const add = useMutation({
    mutationFn: () => addAllowlist(eco, pkg, reason),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['allowlist'] });
      setPkg(''); setReason(''); setEco(''); setFormErr('');
    },
    onError: (e: Error) => setFormErr(e.message),
  });

  const remove = useMutation({
    mutationFn: (id: number) => deleteAllowlist(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['allowlist'] }),
  });

  const entries = data?.allowlist ?? [];

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <ListFilter size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Allowlist</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>
            Explicitly trusted packages that bypass policy enforcement.
          </p>
        </div>
      </div>

      {/* Add form */}
      <div className="rounded-lg p-4 space-y-3" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>ADD ENTRY</p>
        <div className="grid grid-cols-1 sm:grid-cols-[110px_1fr_1fr] gap-2.5">
          <div>
            <label style={{ fontSize: '0.68rem', color: 'var(--color-muted)', display: 'block', marginBottom: 3 }}>ECOSYSTEM</label>
            <input value={eco} onChange={e => setEco(e.target.value)} placeholder="npm" style={inputStyle} />
          </div>
          <div>
            <label style={{ fontSize: '0.68rem', color: 'var(--color-muted)', display: 'block', marginBottom: 3 }}>PACKAGE *</label>
            <input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="lodash" style={inputStyle} />
          </div>
          <div>
            <label style={{ fontSize: '0.68rem', color: 'var(--color-muted)', display: 'block', marginBottom: 3 }}>REASON</label>
            <input value={reason} onChange={e => setReason(e.target.value)} placeholder="internal fork — no CVE impact" style={inputStyle} />
          </div>
        </div>
        {formErr && <p style={{ fontSize: '0.72rem', color: 'var(--color-critical)' }}>{formErr}</p>}
        <button
          onClick={() => { if (!pkg) { setFormErr('Package is required'); return; } add.mutate(); }}
          disabled={add.isPending}
          style={{
            display: 'flex', alignItems: 'center', gap: 6,
            background: 'rgba(0,255,135,0.12)', border: '1px solid rgba(0,255,135,0.25)',
            borderRadius: '0.375rem', color: 'var(--color-safe)',
            fontSize: '0.78rem', fontFamily: 'var(--font-mono)',
            padding: '0.4rem 0.875rem', cursor: add.isPending ? 'not-allowed' : 'pointer',
          }}
        >
          <Plus size={13} />
          {add.isPending ? 'Adding…' : 'Add to Allowlist'}
        </button>
      </div>

      {/* Allowlist table */}
      <div className="rounded-lg overflow-hidden" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div style={{ padding: '0.5rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <ShieldCheck size={14} style={{ color: 'var(--color-safe)' }} />
          <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-safe)' }}>
            ALLOWLISTED ({entries.length})
          </span>
        </div>
        {isLoading && <div style={{ padding: '1.5rem', textAlign: 'center', fontSize: '0.8rem', color: 'var(--color-muted)' }}>Loading…</div>}
        {!isLoading && entries.length === 0 && (
          <div style={{ padding: '1.5rem', textAlign: 'center', fontSize: '0.8rem', color: 'var(--color-muted)' }}>
            No entries. Add packages above to bypass policy enforcement.
          </div>
        )}
        {entries.map((e, i) => (
          <div key={e.id} style={{
            display: 'flex', alignItems: 'center', gap: '0.75rem',
            padding: '0.5rem 0.875rem',
            borderBottom: i < entries.length - 1 ? '1px solid rgba(255,255,255,0.04)' : 'none',
          }}>
            <ShieldCheck size={12} style={{ color: 'var(--color-safe)', flexShrink: 0 }} />
            <code style={{ fontSize: '0.78rem', color: 'var(--fg)', fontFamily: 'var(--font-mono)', flex: 1 }}>
              {e.ecosystem ? `${e.ecosystem}/` : ''}{e.package}
            </code>
            <span style={{ fontSize: '0.7rem', color: 'var(--color-muted)', flex: 2 }}>{e.reason || '—'}</span>
            <span style={{ fontSize: '0.68rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)' }}>{e.added_by}</span>
            <button
              onClick={() => remove.mutate(e.id)}
              title="Remove"
              style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-critical)', padding: 2 }}
            >
              <Trash2 size={12} />
            </button>
          </div>
        ))}
      </div>

      {/* Blocklist from policy (read-only) */}
      <div className="rounded-lg p-4 space-y-2" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div className="flex items-center gap-2">
          <ShieldX size={14} style={{ color: 'var(--color-critical)' }} />
          <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-critical)' }}>BLOCKLIST</p>
        </div>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
          Blocked packages are managed via policy.yaml — use <code style={{ color: 'var(--color-safe)' }}>fgctl policy set deny=pkg@version</code>
        </p>
      </div>
    </div>
  );
}

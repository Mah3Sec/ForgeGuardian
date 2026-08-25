import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { signArtifact, verifyAttestation } from '../lib/api';
import { Key, CheckCircle2, XCircle, AlertCircle } from 'lucide-react';
import type { Attestation } from '../types/api';

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'];

export function SignPage() {
  const [ecosystem, setEcosystem] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [version, setVersion] = useState('');
  const [sha256, setSha256] = useState('');
  const [attestation, setAttestation] = useState<Attestation | null>(null);
  const [verifyInput, setVerifyInput] = useState('');
  const [verifySHA, setVerifySHA] = useState('');

  const sign = useMutation({
    mutationFn: () => signArtifact(sha256, ecosystem, pkg, version),
    onSuccess: (data) => setAttestation(data),
  });

  const verify = useMutation({
    mutationFn: () => {
      let att: Attestation;
      try {
        att = JSON.parse(verifyInput) as Attestation;
      } catch {
        return Promise.reject(new Error('Invalid JSON — paste a valid attestation object'));
      }
      return verifyAttestation(att, verifySHA);
    },
  });

  const copyAtt = () => attestation && navigator.clipboard.writeText(JSON.stringify(attestation, null, 2));

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Sign & Verify</h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Sigstore keyless signing with Rekor transparency log. Ephemeral ECDSA P-256 keypair — no long-lived secrets.
      </p>

      {/* Sign form */}
      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <h2 className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>SIGN ARTIFACT</h2>
        <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>ECOSYSTEM</label>
            <select value={ecosystem} onChange={e => setEcosystem(e.target.value)}
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }}>
              {ECOSYSTEMS.map(e => <option key={e}>{e}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>PACKAGE</label>
            <input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="name"
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>VERSION</label>
            <input value={version} onChange={e => setVersion(e.target.value)} placeholder="version"
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>SHA256</label>
            <input value={sha256} onChange={e => setSha256(e.target.value)} placeholder="hex hash"
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
          </div>
        </div>
        <button onClick={() => sign.mutate()} disabled={!sha256 || !pkg || !version || sign.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
          style={{ background: 'var(--color-safe)', color: '#0A0B0D' }}>
          <Key size={14} />{sign.isPending ? 'Signing…' : 'Sign'}
        </button>
        {sign.isError && (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-critical)' }}>
            <AlertCircle size={14} />{(sign.error as Error).message}
          </div>
        )}
      </div>

      {/* Attestation output */}
      {attestation && (
        <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(0,255,135,0.2)' }}>
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
            <div className="flex items-center gap-2">
              <CheckCircle2 size={14} color="var(--color-safe)" />
              <span className="text-xs font-mono" style={{ color: 'var(--color-safe)' }}>SIGNED</span>
              {attestation.rekor_log_id && (
                <span className="text-xs" style={{ color: 'var(--color-muted)' }}>Rekor: {attestation.rekor_log_id.slice(0, 16)}…</span>
              )}
            </div>
            <button onClick={copyAtt} className="text-xs px-2 py-1 rounded font-mono"
              style={{ background: 'rgba(0,255,135,0.1)', color: 'var(--color-safe)' }}>Copy JSON</button>
          </div>
          <pre className="p-4 text-xs overflow-auto max-h-60 font-mono" style={{ color: 'var(--fg)' }}>
            {JSON.stringify(attestation, null, 2)}
          </pre>
        </div>
      )}

      {/* Verify form */}
      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <h2 className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>VERIFY ATTESTATION</h2>
        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>ATTESTATION JSON</label>
          <textarea rows={4} value={verifyInput} onChange={e => setVerifyInput(e.target.value)}
            placeholder='Paste attestation JSON here…'
            className="w-full rounded px-3 py-2 text-xs font-mono resize-y"
            style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
        </div>
        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>EXPECTED SHA256</label>
          <input value={verifySHA} onChange={e => setVerifySHA(e.target.value)} placeholder="expected sha256 hex"
            className="w-full rounded px-3 py-2 text-sm font-mono"
            style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
        </div>
        <button onClick={() => verify.mutate()} disabled={!verifyInput || !verifySHA || verify.isPending}
          className="px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
          style={{ background: 'rgba(255,255,255,0.1)', color: 'var(--fg)' }}>
          {verify.isPending ? 'Verifying…' : 'Verify'}
        </button>
        {verify.isError && (
          <div className="flex items-center gap-2 p-3 rounded text-sm"
            style={{ background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)' }}>
            <XCircle size={14} />{(verify.error as Error).message}
          </div>
        )}
        {verify.data && (
          <div className={`flex items-center gap-2 p-3 rounded text-sm`}
            style={{
              background: verify.data.valid ? 'rgba(0,255,135,0.1)' : 'rgba(255,61,61,0.1)',
              color: verify.data.valid ? 'var(--color-safe)' : 'var(--color-critical)',
              border: `1px solid ${verify.data.valid ? 'rgba(0,255,135,0.2)' : 'rgba(255,61,61,0.2)'}`,
            }}>
            {verify.data.valid ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
            {verify.data.valid
              ? `Valid — SHA256 match: ${verify.data.sha256_match}, Rekor: ${verify.data.rekor_verified}`
              : `Invalid — ${verify.data.error}`}
          </div>
        )}
      </div>
    </div>
  );
}

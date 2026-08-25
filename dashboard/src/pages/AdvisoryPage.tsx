import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { generateAdvisory } from '../lib/api';
import { SeverityBadge } from '../components/SeverityBadge';
import { Zap, AlertCircle, CheckCircle2 } from 'lucide-react';
import type { Advisory } from '../types/api';

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'];

export function AdvisoryPage() {
  const [ecosystem, setEcosystem] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [version, setVersion] = useState('');
  const [advisory, setAdvisory] = useState<Advisory | null>(null);

  const gen = useMutation({
    mutationFn: () => generateAdvisory(ecosystem, pkg, version),
    onSuccess: (data) => setAdvisory(data),
  });

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>AI Security Advisory</h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Powered by Claude — generates a structured advisory with exploitability analysis and recommended actions.
      </p>

      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
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
            <input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="package name"
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>VERSION</label>
            <input value={version} onChange={e => setVersion(e.target.value)} placeholder="version"
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }} />
          </div>
        </div>
        <button onClick={() => gen.mutate()} disabled={!pkg || !version || gen.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
          style={{ background: 'var(--color-warning)', color: '#0A0B0D' }}>
          <Zap size={14} />
          {gen.isPending ? 'Generating advisory…' : 'Generate Advisory'}
        </button>
      </div>

      {gen.isError && (
        <div className="flex items-center gap-2 p-3 rounded text-sm"
          style={{ background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)' }}>
          <AlertCircle size={14} />{(gen.error as Error).message}
        </div>
      )}

      {advisory && (
        <div className="space-y-4">
          {/* Header */}
          <div className="rounded-lg p-5 space-y-3" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
            <div className="flex items-center gap-3">
              <SeverityBadge severity={advisory.severity} />
              <span className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>
                Confidence: {Math.round(advisory.confidence * 100)}%
              </span>
            </div>
            <h2 className="font-bold" style={{ color: 'var(--fg)' }}>
              {advisory.package.name}@{advisory.package.version} ({advisory.package.ecosystem})
            </h2>
          </div>

          {/* Advisory text */}
          <div className="rounded-lg p-5" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
            <h3 className="text-xs font-mono mb-2" style={{ color: 'var(--color-muted)' }}>ADVISORY</h3>
            <p className="text-sm leading-relaxed" style={{ color: 'var(--fg)' }}>{advisory.advisory}</p>
          </div>

          {/* Recommended action */}
          <div className="rounded-lg p-5" style={{ background: 'rgba(0,255,135,0.05)', border: '1px solid rgba(0,255,135,0.15)' }}>
            <div className="flex items-center gap-2 mb-2">
              <CheckCircle2 size={14} color="var(--color-safe)" />
              <h3 className="text-xs font-mono" style={{ color: 'var(--color-safe)' }}>RECOMMENDED ACTION</h3>
            </div>
            <p className="text-sm" style={{ color: 'var(--fg)' }}>{advisory.recommended_action}</p>
          </div>

          {/* Agentic risk */}
          {advisory.agentic_risk && (
            <div className="rounded-lg p-5" style={{ background: 'rgba(255,171,64,0.05)', border: '1px solid rgba(255,171,64,0.15)' }}>
              <h3 className="text-xs font-mono mb-2" style={{ color: 'var(--color-warning)' }}>AGENTIC ATTACK SURFACE</h3>
              <p className="text-sm" style={{ color: 'var(--fg)' }}>{advisory.agentic_risk}</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

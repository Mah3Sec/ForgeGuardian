import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { getSBOM } from '../lib/api';
import { FileText, Download, AlertCircle } from 'lucide-react';

const FORMATS = ['cyclonedx-json', 'cyclonedx-xml', 'spdx-json', 'spdx-tv'];
const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'];

export function SBOMPage() {
  const [ecosystem, setEcosystem] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [version, setVersion] = useState('');
  const [format, setFormat] = useState('cyclonedx-json');
  const [sbomText, setSbomText] = useState('');
  // What the currently-displayed sbomText was actually generated for — kept
  // separate from the live form fields so editing package/version/format
  // after generating doesn't relabel or rename-on-download stale content.
  const [generatedFor, setGeneratedFor] = useState<{ ecosystem: string; pkg: string; version: string; format: string } | null>(null);

  const gen = useMutation({
    mutationFn: () => getSBOM(ecosystem, pkg, version, format),
    onSuccess: (data) => {
      setSbomText(data);
      setGeneratedFor({ ecosystem, pkg, version, format });
    },
  });

  const download = () => {
    if (!generatedFor) return;
    const ext = generatedFor.format.includes('json') ? 'json' : generatedFor.format.includes('xml') ? 'xml' : 'txt';
    const blob = new Blob([sbomText], { type: 'application/octet-stream' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${generatedFor.pkg}-${generatedFor.version}-sbom.${ext}`;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>SBOM Generator</h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Generate CycloneDX or SPDX software bill of materials for any package.
      </p>

      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
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
            <input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="package"
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
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>FORMAT</label>
            <select value={format} onChange={e => setFormat(e.target.value)}
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)' }}>
              {FORMATS.map(f => <option key={f}>{f}</option>)}
            </select>
          </div>
        </div>
        <button onClick={() => gen.mutate()} disabled={!pkg || !version || gen.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
          style={{ background: 'var(--color-safe)', color: '#0A0B0D' }}>
          <FileText size={14} />
          {gen.isPending ? 'Generating…' : 'Generate SBOM'}
        </button>
      </div>

      {gen.isError && (
        <div className="flex items-center gap-2 p-3 rounded text-sm"
          style={{ background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)' }}>
          <AlertCircle size={14} />{(gen.error as Error).message}
        </div>
      )}

      {sbomText && generatedFor && (
        <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
            <span className="text-xs font-mono" style={{ color: 'var(--color-muted)' }}>
              {generatedFor.format.toUpperCase()} — {generatedFor.pkg}@{generatedFor.version}
            </span>
            <button onClick={download} className="flex items-center gap-1 text-xs px-3 py-1 rounded font-mono"
              style={{ background: 'rgba(0,255,135,0.1)', color: 'var(--color-safe)', border: '1px solid rgba(0,255,135,0.2)' }}>
              <Download size={12} /> Download
            </button>
          </div>
          <pre className="p-4 text-xs overflow-auto max-h-96 font-mono" style={{ color: 'var(--fg)' }}>
            {sbomText}
          </pre>
        </div>
      )}
    </div>
  );
}

import { Download, FileJson, FileCode, FileText } from 'lucide-react';
import { useUIStore } from '../store/ui';

export function ExportsPage() {
  const navigate = useUIStore(s => s.navigate);

  const formats = [
    {
      icon: FileJson,
      title: 'CycloneDX JSON',
      desc: 'Machine-readable SBOM in CycloneDX 1.5 JSON format. Compatible with Dependency-Track, OWASP tools, and most CI pipelines.',
      color: 'var(--color-indigo)',
      hint: 'fgctl sbom npm/express@4.18.2 --format cyclonedx-json',
    },
    {
      icon: FileCode,
      title: 'CycloneDX XML',
      desc: 'CycloneDX 1.5 XML format. Required by some enterprise security tools and legacy scanners.',
      color: 'var(--color-info)',
      hint: 'fgctl sbom npm/express@4.18.2 --format cyclonedx-xml',
    },
    {
      icon: FileText,
      title: 'SPDX',
      desc: 'Software Package Data Exchange format. ISO/IEC 5962:2021 standard. Required for US government supply chain compliance.',
      color: 'var(--color-warn)',
      hint: 'fgctl sbom npm/express@4.18.2 --format spdx-json',
    },
  ];

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <Download size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Exports</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>Export scan results, SBOMs, and reports in standard formats.</p>
        </div>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
        {formats.map(f => (
          <div
            key={f.title}
            className="rounded-lg p-4"
            style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', gap: '1rem', alignItems: 'flex-start' }}
          >
            <f.icon size={20} style={{ color: f.color, marginTop: 2, flexShrink: 0 }} />
            <div style={{ flex: 1 }}>
              <p style={{ fontSize: '0.875rem', fontWeight: 600, color: 'var(--fg)', marginBottom: '0.25rem' }}>{f.title}</p>
              <p style={{ fontSize: '0.75rem', color: 'var(--color-muted)', marginBottom: '0.5rem' }}>{f.desc}</p>
              <code style={{ fontSize: '0.72rem', color: 'var(--color-safe)', fontFamily: 'var(--font-mono)' }}>{f.hint}</code>
            </div>
            <button
              onClick={() => navigate('/sbom')}
              style={{
                background: 'rgba(99,102,241,0.15)',
                border: '1px solid rgba(99,102,241,0.3)',
                borderRadius: '0.375rem',
                color: 'var(--color-indigo)',
                fontSize: '0.72rem',
                fontFamily: 'var(--font-mono)',
                padding: '0.375rem 0.75rem',
                cursor: 'pointer',
                flexShrink: 0,
              }}
            >
              Generate →
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

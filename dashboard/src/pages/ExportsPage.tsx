import { useState } from 'react';
import { Download, FileJson, FileCode, FileText, FileSpreadsheet, CheckCircle } from 'lucide-react';
import { useUIStore } from '../store/ui';

const BASE = import.meta.env.VITE_API_URL ?? '';

async function downloadReport(format: 'json' | 'csv') {
  const res = await fetch(`${BASE}/api/v1/export/report?format=${format}`, {
    credentials: 'include',
  });
  if (!res.ok) throw new Error('Export failed');
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  const date = new Date().toISOString().slice(0, 10);
  a.download = `forgeguardian-report-${date}.${format}`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function ExportsPage() {
  const navigate = useUIStore(s => s.navigate);
  const [downloading, setDownloading] = useState<string | null>(null);
  const [done, setDone] = useState<string | null>(null);

  const handleDownload = async (format: 'json' | 'csv') => {
    setDownloading(format);
    setDone(null);
    try {
      await downloadReport(format);
      setDone(format);
      setTimeout(() => setDone(null), 3000);
    } finally {
      setDownloading(null);
    }
  };

  const reports = [
    {
      icon: FileJson,
      title: 'Scan Report (JSON)',
      desc: 'Full scan results with risk scores, severity breakdown, and timestamps. Machine-readable format for integration with other tools.',
      color: 'var(--color-info)',
      format: 'json' as const,
    },
    {
      icon: FileSpreadsheet,
      title: 'Scan Report (CSV)',
      desc: 'Spreadsheet-compatible scan results. Import into Excel, Google Sheets, or any data tool for custom analysis and reporting.',
      color: 'var(--color-safe)',
      format: 'csv' as const,
    },
  ];

  const sbomFormats = [
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
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Download size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Exports</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>Export scan results, SBOMs, and reports in standard formats.</p>
        </div>
      </div>

      <div>
        <h2 className="text-sm font-semibold mb-3" style={{ color: 'var(--fg)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>Scan Reports</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {reports.map(f => (
            <div
              key={f.title}
              className="rounded-lg p-4"
              style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', gap: '1rem', alignItems: 'flex-start' }}
            >
              <f.icon size={20} style={{ color: f.color, marginTop: 2, flexShrink: 0 }} />
              <div style={{ flex: 1 }}>
                <p style={{ fontSize: '0.875rem', fontWeight: 600, color: 'var(--fg)', marginBottom: '0.25rem' }}>{f.title}</p>
                <p style={{ fontSize: '0.75rem', color: 'var(--color-muted)' }}>{f.desc}</p>
              </div>
              <button
                onClick={() => handleDownload(f.format)}
                disabled={downloading === f.format}
                style={{
                  background: done === f.format ? 'rgba(34,197,94,0.15)' : 'rgba(99,102,241,0.15)',
                  border: `1px solid ${done === f.format ? 'rgba(34,197,94,0.3)' : 'rgba(99,102,241,0.3)'}`,
                  borderRadius: '0.375rem',
                  color: done === f.format ? 'var(--color-safe)' : 'var(--color-indigo)',
                  fontSize: '0.72rem',
                  fontFamily: 'var(--font-mono)',
                  padding: '0.375rem 0.75rem',
                  cursor: downloading ? 'wait' : 'pointer',
                  flexShrink: 0,
                  display: 'flex',
                  alignItems: 'center',
                  gap: '0.375rem',
                }}
              >
                {done === f.format ? <><CheckCircle size={12} /> Downloaded</> :
                 downloading === f.format ? 'Downloading...' :
                 <><Download size={12} /> Download</>}
              </button>
            </div>
          ))}
        </div>
      </div>

      <div>
        <h2 className="text-sm font-semibold mb-3" style={{ color: 'var(--fg)', letterSpacing: '0.05em', textTransform: 'uppercase' }}>SBOM Formats</h2>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
          {sbomFormats.map(f => (
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
    </div>
  );
}

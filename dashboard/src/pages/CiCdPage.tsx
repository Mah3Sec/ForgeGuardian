import { GitMerge } from 'lucide-react';

const GH_ACTIONS = `name: Security Scan
on: [push, pull_request]
jobs:
  forgeguardian:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install ForgeGuardian
        run: curl -sSfL https://forgeguardian.dev/install.sh | sh
      - name: Scan dependencies
        run: fgctl scan . --ci --fail-on=high --format sarif > fg-results.sarif
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: fg-results.sarif`;

const GITLAB_CI = `forgeguardian:
  image: alpine:latest
  script:
    - curl -sSfL https://forgeguardian.dev/install.sh | sh
    - fgctl scan . --ci --fail-on=high
  artifacts:
    reports:
      sast: fg-results.json`;

const MAKEFILE = `# In your Makefile:
security:
\tfgctl scan . --ci --fail-on=critical

security-full:
\tfgctl scan . --verbose --format sarif > results.sarif`;

export function CiCdPage() {
  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <GitMerge size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>CI/CD Integration</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>Integrate ForgeGuardian into your CI/CD pipelines for continuous supply chain security.</p>
        </div>
      </div>

      {[
        { title: 'GitHub Actions', lang: 'yaml', code: GH_ACTIONS },
        { title: 'GitLab CI',      lang: 'yaml', code: GITLAB_CI },
        { title: 'Makefile',       lang: 'make', code: MAKEFILE },
      ].map(s => (
        <div
          key={s.title}
          className="rounded-lg"
          style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)', overflow: 'hidden' }}
        >
          <div style={{ padding: '0.625rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--fg)' }}>{s.title}</span>
            <span style={{ fontSize: '0.65rem', color: 'var(--color-muted)', fontFamily: 'var(--font-mono)' }}>{s.lang}</span>
          </div>
          <pre
            style={{
              padding: '0.875rem',
              fontSize: '0.72rem',
              fontFamily: 'var(--font-mono)',
              color: 'var(--color-safe)',
              overflowX: 'auto',
              margin: 0,
              lineHeight: 1.6,
              whiteSpace: 'pre',
            }}
          >
            {s.code}
          </pre>
        </div>
      ))}

      <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold mb-2" style={{ color: 'var(--color-muted)' }}>CI-OPTIMIZED FLAGS</p>
        <div className="space-y-1">
          <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>{'--ci              # Quiet mode + SARIF output + fail-on=high'}</code>
          <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>{'--fail-on=critical # Only fail pipeline on critical findings'}</code>
          <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>{'--format sarif    # SARIF 2.1.0 for GitHub/GitLab code scanning'}</code>
          <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>{'--prod-only       # Skip dev/test dependencies'}</code>
        </div>
      </div>
    </div>
  );
}

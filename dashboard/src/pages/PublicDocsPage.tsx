import { useState } from 'react';
import { BookOpen, Terminal, Download, Settings, Shield, Activity, GitMerge, ChevronRight } from 'lucide-react';
import { CopyButton } from '../components/CopyButton';

const GITHUB_URL = 'https://github.com/mah3sec/forgeguardian';

interface Section {
  id: string;
  title: string;
  icon: React.ElementType;
}

const SECTIONS: Section[] = [
  { id: 'getting-started', title: 'Getting Started', icon: Download },
  { id: 'cli', title: 'CLI Reference', icon: Terminal },
  { id: 'dashboard', title: 'Dashboard', icon: Activity },
  { id: 'scanning', title: 'Scanning', icon: Shield },
  { id: 'docker', title: 'Docker Deployment', icon: Settings },
  { id: 'cicd', title: 'CI/CD Integration', icon: GitMerge },
];

function CodeBlock({ code, lang = 'bash' }: { code: string; lang?: string }) {
  return (
    <div className="relative group rounded-lg border border-border-color bg-[var(--bg-base)] overflow-hidden">
      <div className="flex items-center justify-between px-4 py-2 border-b border-border-color bg-surface-muted">
        <span className="text-[0.65rem] font-mono text-text-muted uppercase tracking-wider">{lang}</span>
        <CopyButton text={code} />
      </div>
      <pre className="p-4 overflow-x-auto text-[0.82rem] leading-relaxed font-mono text-text-primary">
        <code>{code}</code>
      </pre>
    </div>
  );
}

function SectionHeading({ id, title }: { id: string; title: string }) {
  return (
    <h2 id={id} className="text-xl font-bold text-text-primary mt-12 mb-4 scroll-mt-20 flex items-center gap-2">
      <span className="text-primary-blue">#</span> {title}
    </h2>
  );
}

export function PublicDocsPage({ onNavigateHome }: { onNavigateHome?: () => void }) {
  const [activeSection, setActiveSection] = useState('getting-started');

  const scrollTo = (id: string) => {
    setActiveSection(id);
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth' });
  };

  return (
    <div className="min-h-screen" style={{ background: 'var(--bg-base)', color: 'var(--fg)' }}>
      {/* Header */}
      <header className="border-b border-border-color bg-surface">
        <div className="max-w-6xl mx-auto px-6 py-4 flex items-center justify-between">
          <div className="flex items-center gap-3 cursor-pointer" onClick={onNavigateHome}>
            <img src="/logo-icon.png" alt="ForgeGuardian" className="h-8" />
            <span className="font-semibold text-text-primary">ForgeGuardian</span>
            <span className="text-text-muted">/ Docs</span>
          </div>
          <div className="flex items-center gap-4">
            <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer" className="text-[0.8rem] text-text-secondary hover:text-primary-blue transition-colors">
              GitHub
            </a>
          </div>
        </div>
      </header>

      <div className="max-w-6xl mx-auto px-6 flex gap-8">
        {/* Sidebar nav */}
        <nav className="hidden md:block w-56 shrink-0 pt-8 sticky top-0 h-screen overflow-y-auto">
          <p className="text-[0.65rem] font-bold text-text-muted uppercase tracking-wider mb-3 px-2">Documentation</p>
          {SECTIONS.map(s => (
            <button
              key={s.id}
              onClick={() => scrollTo(s.id)}
              className={`w-full flex items-center gap-2 px-3 py-2 rounded-md text-[0.8rem] text-left transition-colors mb-0.5 bg-transparent cursor-pointer border-none [font-family:inherit] ${
                activeSection === s.id
                  ? 'bg-blue-light text-primary-blue'
                  : 'text-text-secondary hover:bg-surface-muted hover:text-text-primary'
              }`}
            >
              <s.icon size={14} className="shrink-0" />
              {s.title}
            </button>
          ))}
        </nav>

        {/* Content */}
        <main className="flex-1 min-w-0 py-8 pb-24">
          <div className="flex items-center gap-2 text-[0.75rem] text-text-muted mb-6">
            <BookOpen size={14} />
            <span>ForgeGuardian Documentation</span>
            <ChevronRight size={12} />
            <span className="text-text-primary">User Guide</span>
          </div>

          <h1 className="text-3xl font-bold text-text-primary mb-2">ForgeGuardian Documentation</h1>
          <p className="text-text-secondary mb-8 text-[0.95rem] leading-relaxed max-w-2xl">
            Local-first, AI-native software supply chain security. Scan packages across nine ecosystems,
            generate SLSA Level 3 provenance, sign artifacts with Sigstore, and get AI-powered security advisories.
          </p>

          {/* Getting Started */}
          <SectionHeading id="getting-started" title="Getting Started" />

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">Install the CLI</h3>
          <p className="text-text-secondary text-[0.88rem] mb-3">
            One-liner install — auto-detects your OS and architecture, no compiler required:
          </p>
          <CodeBlock code="curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | bash" />

          <p className="text-text-secondary text-[0.88rem] mt-4 mb-3">
            Or install with Go:
          </p>
          <CodeBlock code="go install github.com/mah3sec/forgeguardian/cmd/fgctl@latest" />

          <h3 className="text-base font-semibold text-text-primary mt-8 mb-3">Run your first scan</h3>
          <p className="text-text-secondary text-[0.88rem] mb-3">
            No account, no config file required. Point it at any project directory:
          </p>
          <CodeBlock code="fgctl scan ." />
          <p className="text-text-muted text-[0.8rem] mt-2">
            Config, policy, and signatures live at <code className="px-1 py-0.5 bg-surface-muted rounded text-[0.78rem]">~/.forgeguardian/</code>
          </p>

          <h3 className="text-base font-semibold text-text-primary mt-8 mb-3">Scan a registry package</h3>
          <CodeBlock code={`fgctl scan lodash            # npm (default)\nfgctl scan requests --eco pypi  # PyPI\nfgctl scan gin --eco go         # Go module`} />

          {/* CLI Reference */}
          <SectionHeading id="cli" title="CLI Reference" />

          <div className="overflow-x-auto">
            <table className="w-full text-[0.84rem] mt-4">
              <thead>
                <tr className="border-b border-border-color">
                  <th className="text-left py-2.5 pr-4 text-text-muted font-semibold">Command</th>
                  <th className="text-left py-2.5 text-text-muted font-semibold">Description</th>
                </tr>
              </thead>
              <tbody className="font-mono">
                {[
                  ['fgctl scan <path|pkg>', 'Scan a local directory or registry package'],
                  ['fgctl scan --recursive', 'Recursively scan all subdirectories'],
                  ['fgctl sbom <path>', 'Generate SBOM (CycloneDX / SPDX)'],
                  ['fgctl sign <artifact>', 'Sign with Sigstore keyless signing'],
                  ['fgctl verify <artifact>', 'Verify artifact signature'],
                  ['fgctl advisory <path>', 'AI-powered security advisory'],
                  ['fgctl patch <path>', 'AI autonomous vulnerability patching'],
                  ['fgctl monitor <path>', 'Continuous real-time monitoring'],
                  ['fgctl audit', 'System security audit'],
                  ['fgctl policy apply <file>', 'Apply a policy-as-code file'],
                  ['fgctl intel new', 'Create a new detection signature'],
                  ['fgctl setup', 'Interactive first-time setup'],
                  ['fgctl doctor', 'Diagnose installation issues'],
                  ['fgctl version', 'Show version and build info'],
                ].map(([cmd, desc]) => (
                  <tr key={cmd} className="border-b border-border-color/50">
                    <td className="py-2.5 pr-4 text-primary-blue whitespace-nowrap">{cmd}</td>
                    <td className="py-2.5 text-text-secondary font-sans">{desc}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <h3 className="text-base font-semibold text-text-primary mt-8 mb-3">Common flags</h3>
          <CodeBlock code={`fgctl scan . --format json         # JSON output\nfgctl scan . --format sarif        # SARIF for GitHub/GitLab\nfgctl scan . --fail-on-high        # Exit 1 on high+ severity\nfgctl scan . --ci                  # CI mode (minimal output)\nfgctl scan . --eco npm             # Force ecosystem`} />

          {/* Dashboard */}
          <SectionHeading id="dashboard" title="Dashboard" />

          <p className="text-text-secondary text-[0.88rem] mb-4 leading-relaxed">
            The web dashboard provides a visual interface for all ForgeGuardian features.
            It runs alongside the Go backend and connects to the same scan engine as the CLI.
          </p>

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">Features</h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-6">
            {[
              ['Security posture score', 'Real-time overall score based on scan findings'],
              ['Scan sessions', 'History of all scans with detailed results and charts'],
              ['Live monitoring', 'Real-time file system and dependency change alerts'],
              ['Dependency graph', 'Interactive visualization of your dependency tree'],
              ['AI Advisory', 'AI-powered security analysis and recommendations'],
              ['AI Patch Agent', 'Automated vulnerability patching via AI'],
              ['Attack surface mapping', 'Comprehensive view of your security perimeter'],
              ['Policy management', 'Create, apply, and enforce security policies'],
              ['SBOM generation', 'Generate and export software bills of materials'],
              ['Alert management', 'Configure alerts and notification webhooks'],
              ['Workspace management', 'Organize scans across multiple projects'],
              ['Export & reports', 'Export findings as JSON, CSV, or HTML reports'],
            ].map(([title, desc]) => (
              <div key={title} className="rounded-lg border border-border-color bg-surface p-3.5">
                <p className="text-[0.82rem] font-semibold text-text-primary mb-0.5">{title}</p>
                <p className="text-[0.75rem] text-text-muted">{desc}</p>
              </div>
            ))}
          </div>

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">Default credentials</h3>
          <p className="text-text-secondary text-[0.88rem] mb-3">
            On first launch with auth enabled, set credentials via environment variables or <code className="px-1 py-0.5 bg-surface-muted rounded text-[0.78rem]">fgctl setup</code>:
          </p>
          <CodeBlock code={`FG_ADMIN_EMAIL=you@example.com\nFG_ADMIN_PASSWORD=your-password\nFG_SESSION_SECRET=$(openssl rand -hex 32)`} />

          {/* Scanning */}
          <SectionHeading id="scanning" title="Scanning" />

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">Scan engines</h3>
          <p className="text-text-secondary text-[0.88rem] mb-4">
            Every scan runs multiple detection engines concurrently. Five always run, three optional engines
            activate automatically when their tools are installed:
          </p>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mb-6">
            {[
              ['OSV', 'Known CVEs via osv.dev', true],
              ['Behavioral', 'Malicious install scripts, typosquatting', true],
              ['Malware', 'Byte/regex pattern matching', true],
              ['AI Model', 'HuggingFace weight safety', true],
              ['MCP', 'Prompt injection in tool descriptions', true],
              ['Grype', 'Deep CVE scan of artifact files', false],
              ['Trivy', 'Container + OS CVE scanning', false],
              ['Semgrep', 'SAST static analysis', false],
            ].map(([name, desc, always]) => (
              <div key={name as string} className="flex items-center gap-3 rounded-lg border border-border-color bg-surface px-4 py-3">
                <div className="flex-1 min-w-0">
                  <p className="text-[0.82rem] font-semibold font-mono text-text-primary">{name}</p>
                  <p className="text-[0.73rem] text-text-muted">{desc}</p>
                </div>
                <span className={`text-[0.65rem] px-2 py-0.5 rounded-full font-medium ${
                  always ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-400'
                }`}>
                  {always ? 'always runs' : 'optional'}
                </span>
              </div>
            ))}
          </div>

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">Supported ecosystems</h3>
          <div className="flex flex-wrap gap-2 mb-6">
            {['npm', 'PyPI', 'Go', 'Maven', 'RubyGems', 'crates.io', 'HuggingFace', 'MCP', 'OCI / Docker'].map(e => (
              <span key={e} className="px-3 py-1.5 rounded-md bg-surface border border-border-color text-[0.78rem] font-mono text-text-primary">{e}</span>
            ))}
          </div>

          {/* Docker */}
          <SectionHeading id="docker" title="Docker Deployment" />

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">One-command deployment</h3>
          <p className="text-text-secondary text-[0.88rem] mb-3">
            The Docker image bundles the Go backend, dashboard, and all scan engines:
          </p>
          <CodeBlock code={`docker run -d --name forgeguardian \\\n  -p 8080:8080 \\\n  -e FG_ADMIN_EMAIL=admin@example.com \\\n  -e FG_ADMIN_PASSWORD=changeme \\\n  -e FG_SESSION_SECRET=$(openssl rand -hex 32) \\\n  ghcr.io/mah3sec/forgeguardian:latest`} />

          <h3 className="text-base font-semibold text-text-primary mt-8 mb-3">Docker Compose (with PostgreSQL)</h3>
          <p className="text-text-secondary text-[0.88rem] mb-3">
            For persistent storage and production use:
          </p>
          <CodeBlock code={`git clone https://github.com/mah3sec/forgeguardian.git\ncd forgeguardian\ndocker compose up -d`} lang="bash" />
          <p className="text-text-muted text-[0.8rem] mt-2">
            Dashboard at <code className="px-1 py-0.5 bg-surface-muted rounded text-[0.78rem]">http://localhost:8080</code>
          </p>

          {/* CI/CD */}
          <SectionHeading id="cicd" title="CI/CD Integration" />

          <h3 className="text-base font-semibold text-text-primary mt-6 mb-3">GitHub Actions</h3>
          <CodeBlock lang="yaml" code={`name: Security Scan
on: [push, pull_request]

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install ForgeGuardian
        run: curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | bash

      - name: Scan
        run: fgctl scan . --ci --fail-on-high --format sarif > results.sarif

      - name: Upload SARIF
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif`} />

          <h3 className="text-base font-semibold text-text-primary mt-8 mb-3">GitLab CI</h3>
          <CodeBlock lang="yaml" code={`security-scan:
  image: ghcr.io/mah3sec/forgeguardian:latest
  script:
    - fgctl scan . --ci --fail-on-high
  artifacts:
    reports:
      sast: results.sarif`} />

          {/* Footer */}
          <div className="mt-16 pt-8 border-t border-border-color">
            <div className="flex items-center justify-between text-[0.78rem] text-text-muted">
              <span>&copy; 2026 ForgeGuardian — Apache 2.0 Licensed</span>
              <div className="flex gap-4">
                <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer" className="hover:text-primary-blue transition-colors">GitHub</a>
                <a href={`${GITHUB_URL}/blob/main/LICENSE`} target="_blank" rel="noopener noreferrer" className="hover:text-primary-blue transition-colors">License</a>
              </div>
            </div>
          </div>
        </main>
      </div>
    </div>
  );
}

import { useEffect, useRef, useState } from 'react';
import {
  Shield, Package, GitBranch, FileCheck,
  AlertCircle, CheckCircle, Eye, Bell, Timer, Github, ArrowRight,
  Globe, Server, Boxes, Database, Webhook,
} from 'lucide-react';
import { AuthPanel } from '../components/AuthPanel';
import { NetworkGraph } from '../components/NetworkGraph';
import { TopoBackground } from '../components/TopoBackground';
import { CopyButton } from '../components/CopyButton';
import { MetricCard } from '../components/MetricCard';
import { SecurityScore, computeSecurityScore } from '../components/SecurityScore';
import { Card } from '../components/ui/card';
import { Badge } from '../components/ui/badge';

const GITHUB_URL = 'https://github.com/mah3sec/forgeguardian';

const ENGINES = [
  { name: 'OSV',        desc: 'Known CVEs via osv.dev API',            always: true },
  { name: 'Behavioral', desc: 'Malicious install scripts, typosquatting', always: true },
  { name: 'Malware',    desc: 'Byte/regex pattern matching',            always: true },
  { name: 'AI Model',   desc: 'HuggingFace weight safety',              always: true },
  { name: 'MCP',        desc: 'Prompt injection in tool descriptions',  always: true },
  { name: 'Grype',      desc: 'Deep CVE scan of artifact files',        always: false },
  { name: 'Trivy',      desc: 'Container + OS CVE scanning',            always: false },
  { name: 'Semgrep',    desc: 'SAST static analysis',                   always: false },
];

const ECOSYSTEMS = ['npm', 'PyPI', 'Go', 'Maven', 'RubyGems', 'crates.io', 'HuggingFace', 'MCP', 'OCI / Docker'];

interface FreeProRow {
  feature: string;
  free: boolean;
  pro: boolean;
}

const FREE_PRO_ROWS: FreeProRow[] = [
  { feature: 'fgctl scan . — local project scan',                          free: true,  pro: true },
  { feature: '8 scan engines (OSV + Behavioral + Malware + AI Model + MCP)', free: true,  pro: true },
  { feature: 'SBOM generation (CycloneDX + SPDX)',                          free: true,  pro: true },
  { feature: 'Sigstore keyless signing + verification',                    free: true,  pro: true },
  { feature: 'Community signatures (contribute + use)',                    free: true,  pro: true },
  { feature: 'fgctl intel new/validate/test/update',                       free: true,  pro: true },
  { feature: 'Policy-as-code enforcement',                                 free: true,  pro: true },
  { feature: 'Self-hostable + airgap-compatible',                         free: true,  pro: true },
  { feature: 'Basic dashboard',                                           free: true,  pro: true },
  { feature: 'Dashboard: allowlist, advisory, monitor, alerts, agents, projects', free: true,  pro: true },
  { feature: 'fgctl advisory — AI security advisory',                    free: true,  pro: true },
  { feature: 'fgctl patch — AI autonomous patching',                     free: true,  pro: true },
  { feature: 'fgctl monitor — continuous monitoring',                     free: true,  pro: true },
  { feature: 'Dashboard: webhooks (config UI)',                          free: false, pro: true },
  { feature: 'Team management + RBAC',                                    free: false, pro: true },
  { feature: 'SLA + priority support',                                    free: false, pro: true },
  { feature: 'Cloud-hosted option',                                       free: false, pro: true },
];

const PROBLEMS = [
  {
    icon: Eye,
    title: 'Blind spots',
    desc: 'Dependencies come from nine different ecosystems. Without a single scan surface, most teams only see the packages they think to check.',
  },
  {
    icon: Bell,
    title: 'Alert fatigue',
    desc: 'A single-engine scanner produces one signal per package. Cross-referencing that against real exploitability takes a second tool, and a third.',
  },
  {
    icon: Timer,
    title: 'Slow response',
    desc: 'By the time a CVE is triaged, prioritized, and a fix is proposed by hand, the window to patch before exploitation has often already closed.',
  },
];

const HOW_IT_WORKS = [
  { step: '1', title: 'Connect',    desc: 'Point fgctl at a project, a registry package, or an uploaded archive — no account required.' },
  { step: '2', title: 'Scan',       desc: 'All available engines run concurrently: five built-in, three optional if installed.' },
  { step: '3', title: 'Analyze',    desc: 'Findings are normalized across engines and enriched with fix versions where known.' },
  { step: '4', title: 'Prioritize', desc: 'Severity, exploitability, and AI triage combine into a single grade per package.' },
  { step: '5', title: 'Remediate',  desc: 'Apply the suggested fix version, or let the AI patch agent propose a compatible upgrade.' },
];

const INTEGRATION_STEPS = [
  {
    step: '1',
    title: 'Install',
    detail: 'One-liner, no compiler required — auto-detects OS/arch, falls back to go install.',
    code: 'curl -sSfL https://raw.githubusercontent.com/mah3sec/forgeguardian/main/install.sh | sh',
    note: 'Installs fgctl (+ fg-agent, intel-agent) to ~/.local/bin.',
  },
  {
    step: '2',
    title: 'Scan',
    detail: 'No account, no config file required to run your first scan.',
    code: 'fgctl scan .',
    note: 'Config, policy, and signatures live at ~/.forgeguardian/{config,policy}.yaml and signatures.json — created on first run.',
  },
  {
    step: '3',
    title: 'Wire into CI',
    detail: 'Drop into any GitHub Actions workflow — fails the build on findings above your threshold.',
    code: 'fgctl scan . --ci --fail-on=high --format=sarif > results.sarif',
    note: 'SARIF output uploads directly to GitHub Code Scanning via github/codeql-action/upload-sarif.',
  },
  {
    step: '4',
    title: 'Connect the dashboard',
    detail: 'The dashboard and fgctl are two clients of the same API — point either at your server.',
    code: 'fgctl config set api_url=http://localhost:8080',
    note: 'Scans triggered from the CLI show up in the dashboard, and vice versa — same backend, same data.',
  },
];

const INTEGRATIONS = [
  { name: 'Grype',           desc: 'Deep CVE scan of artifact files — auto-detected if installed', icon: Boxes },
  { name: 'Trivy',           desc: 'Container + OS CVE scanning — auto-detected if installed',     icon: Server },
  { name: 'Semgrep',         desc: 'SAST static analysis — auto-detected if installed',             icon: FileCheck },
  { name: 'GitHub Actions',  desc: 'Drop-in CI workflow for scan-on-PR',                             icon: Github },
  { name: 'Slack / Discord', desc: 'Webhook alerts on new findings',                                 icon: Webhook },
  { name: 'Generic Webhook', desc: 'Push alerts to any HTTP endpoint',                                icon: Globe },
];

const TOPOLOGY = [
  { label: 'Internet',       icon: Globe },
  { label: 'API',            icon: Server },
  { label: 'Application',    icon: Boxes },
  { label: 'Infrastructure', icon: GitBranch },
  { label: 'Database',       icon: Database },
];

function TierMark({ ok }: { ok: boolean }) {
  return ok
    ? <CheckCircle size={14} className="text-success" />
    : <span className="font-mono text-[0.68rem] text-text-muted">—</span>;
}

interface PitchPageProps {
  onLoggedIn: () => void;
  onNavigateEnterprise: () => void;
}

export function PitchPage({ onLoggedIn, onNavigateEnterprise }: PitchPageProps) {
  const [scrolled, setScrolled] = useState(false);
  const loginRef = useRef<HTMLDivElement>(null);
  const featuresRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8);
    window.addEventListener('scroll', onScroll);
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  const scrollTo = (ref: React.RefObject<HTMLDivElement | null>) => {
    ref.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  // Illustrative-only example data for the risk intelligence preview —
  // labeled clearly since there's no logged-in real data on a marketing page.
  const illustrativeSummary = { critical: 1, high: 3, medium: 6, low: 4 };
  const illustrativeScore = computeSecurityScore(illustrativeSummary);

  return (
    <div className="min-h-screen overflow-x-hidden bg-bg-base text-text-primary">
      {/* 1. Sticky nav */}
      <nav
        className={
          'sticky top-0 z-40 flex items-center justify-between gap-2 border-b bg-surface/80 px-4 sm:px-6 py-3.5 backdrop-blur-md transition-colors ' +
          (scrolled ? 'border-border-color' : 'border-transparent')
        }
      >
        <div className="flex items-center gap-2 min-w-0 shrink">
          <img src="/logo.png" alt="ForgeGuardian" className="h-7 shrink-0" />
          <span className="font-mono text-[0.95rem] font-bold truncate">ForgeGuardian</span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noreferrer"
            className="flex items-center gap-1.5 rounded-md border border-border-color px-2.5 sm:px-3.5 py-1.5 text-sm font-medium text-text-primary hover:bg-surface-muted"
          >
            <Github size={14} /> <span className="hidden sm:inline">GitHub</span>
          </a>
          <button
            onClick={() => scrollTo(loginRef)}
            className="whitespace-nowrap rounded-md bg-primary-blue px-3 sm:px-4 py-1.5 text-sm font-medium text-white hover:opacity-90"
          >
            Sign in
          </button>
        </div>
      </nav>

      {/* 2. Hero */}
      <section className="relative overflow-hidden px-6 pt-20 pb-24 text-center">
        <TopoBackground className="text-text-primary" opacity={0.05} lines={12} />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center opacity-60">
          <NetworkGraph mode="ambient" opacity={0.1} width={1100} height={520} />
        </div>
        <div className="relative z-10 mx-auto max-w-3xl">
          <p className="mb-4 font-mono text-xs font-semibold uppercase tracking-widest text-primary-blue">
            Local-first · AI-native · open-core
          </p>
          <h1 className="text-4xl font-bold leading-tight tracking-tight text-text-primary sm:text-5xl">
            See threats before they become incidents.
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-base leading-relaxed text-text-secondary">
            ForgeGuardian scans every package you depend on — across nine ecosystems — with eight
            concurrent detection engines, generates SLSA Level 3 provenance, signs artifacts with
            Sigstore, and layers an AI reasoning pass on every scan, triage, and patch. The core
            stays free and open forever.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <button
              onClick={() => scrollTo(loginRef)}
              className="flex items-center gap-2 rounded-md bg-primary-blue px-5 py-2.5 text-sm font-semibold text-white hover:opacity-90"
            >
              Get started <ArrowRight size={14} />
            </button>
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 rounded-md border border-border-color bg-surface px-5 py-2.5 text-sm font-semibold text-text-primary hover:bg-surface-muted"
            >
              <Github size={14} /> View on GitHub
            </a>
            <button
              onClick={() => scrollTo(featuresRef)}
              className="rounded-md border border-border-color bg-surface px-5 py-2.5 text-sm font-semibold text-text-primary hover:bg-surface-muted"
            >
              Explore platform
            </button>
          </div>
          <p className="mt-6 text-xs text-text-muted">
            No account required for the CLI. Self-hostable and airgap-compatible.
          </p>
        </div>
      </section>

      {/* 3. Problem section */}
      <section className="border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">
            Your attack surface is always changing
          </h2>
          <div className="mt-10 grid gap-5 sm:grid-cols-3">
            {PROBLEMS.map(p => (
              <Card key={p.title} className="p-6">
                <p.icon size={20} className="text-primary-blue" />
                <h3 className="mt-3 text-sm font-semibold text-text-primary">{p.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-text-secondary">{p.desc}</p>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* 4. Product overview */}
      <section ref={featuresRef} className="px-6 py-20">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">
            One security layer across your stack
          </h2>
          <p className="mx-auto mt-3 max-w-2xl text-center text-sm text-text-secondary">
            Every scan runs all available engines concurrently — five always run, three optional
            engines activate automatically if installed.
          </p>
          <div className="mt-10 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            {ENGINES.map(e => (
              <Card key={e.name} className="p-4">
                <div className="flex items-center justify-between">
                  <span className="font-mono text-sm font-bold text-text-primary">{e.name}</span>
                  {e.always
                    ? <Badge variant="safe" className="text-[0.6rem]">always runs</Badge>
                    : <Badge variant="outline" className="text-[0.6rem]">optional</Badge>}
                </div>
                <p className="mt-2 text-xs leading-relaxed text-text-secondary">{e.desc}</p>
              </Card>
            ))}
          </div>

          <h3 className="mt-14 text-center text-lg font-semibold text-text-primary">
            9 ecosystems, one tool
          </h3>
          <div className="mt-5 flex flex-wrap justify-center gap-2">
            {ECOSYSTEMS.map(eco => (
              <span
                key={eco}
                className="rounded-md border border-border-color bg-surface-muted px-3.5 py-1.5 font-mono text-xs text-text-primary"
              >
                {eco}
              </span>
            ))}
          </div>
        </div>
      </section>

      {/* 5. Continuous monitoring */}
      <section className="overflow-hidden border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto grid max-w-5xl items-center gap-10 lg:grid-cols-2">
          <div>
            <h2 className="text-2xl font-bold text-text-primary">
              Watches your dependency graph continuously
            </h2>
            <p className="mt-4 text-sm leading-relaxed text-text-secondary">
              Every dependency is tracked as a node in a live graph. As new CVEs land upstream, the
              graph re-evaluates affected packages so nothing has to wait for the next manual scan.
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              <Badge variant="outline">SBOM-based tracking</Badge>
              <Badge variant="outline">Slack / Discord alerts</Badge>
            </div>
          </div>
          <div className="flex max-w-full justify-center overflow-hidden">
            <NetworkGraph mode="ambient" opacity={0.5} width={420} height={320} />
          </div>
        </div>
      </section>

      {/* 6. Risk intelligence */}
      <section className="px-6 py-20">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">Risk intelligence, at a glance</h2>
          <p className="mx-auto mt-3 max-w-2xl text-center text-sm text-text-secondary">
            Illustrative example — your real dashboard populates these from live scan data.
          </p>
          <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="flex flex-col items-center justify-center rounded-xl border border-border-color bg-surface p-5 shadow-sm">
              <SecurityScore score={illustrativeScore} size={92} />
              <span className="mt-2 text-xs font-medium text-text-secondary">Security score</span>
            </div>
            <MetricCard label="Critical issues" value={illustrativeSummary.critical} variant="critical" icon={AlertCircle} />
            <MetricCard label="High risk" value={illustrativeSummary.high} icon={Shield} />
            <MetricCard label="Assets monitored" value="~40" icon={Package} />
          </div>
        </div>
      </section>

      {/* 7. Attack surface diagram */}
      <section className="border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">Full attack surface visibility</h2>
          <div className="mt-12 flex flex-wrap items-center justify-center gap-3">
            {TOPOLOGY.map((t, i) => (
              <div key={t.label} className="flex items-center gap-3">
                <div className="flex flex-col items-center gap-2 rounded-xl border border-border-color bg-surface-muted px-6 py-5">
                  <t.icon size={20} className="text-primary-blue" />
                  <span className="text-xs font-medium text-text-primary">{t.label}</span>
                </div>
                {i < TOPOLOGY.length - 1 && <ArrowRight size={16} className="text-text-muted" />}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* 8. How it works */}
      <section className="px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">How it works</h2>
          <div className="mt-10 grid gap-6 sm:grid-cols-5">
            {HOW_IT_WORKS.map(s => (
              <div key={s.step} className="text-center">
                <div className="mx-auto flex h-9 w-9 items-center justify-center rounded-full bg-primary-blue font-mono text-sm font-bold text-white">
                  {s.step}
                </div>
                <h3 className="mt-3 text-sm font-semibold text-text-primary">{s.title}</h3>
                <p className="mt-1.5 text-xs leading-relaxed text-text-secondary">{s.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* 8b. Integration guide — concrete steps, not just marketing copy */}
      <section className="border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">Integrate it in minutes</h2>
          <p className="mx-auto mt-3 max-w-2xl text-center text-sm text-text-secondary">
            Real commands, real file paths — the CLI and dashboard are two clients of the same
            engine, not separate products.
          </p>
          <div className="mt-10 flex flex-col gap-4">
            {INTEGRATION_STEPS.map(s => (
              <div key={s.step} className="flex gap-4 rounded-xl border border-border-color bg-bg-base p-5">
                <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary-blue font-mono text-sm font-bold text-white">
                  {s.step}
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-semibold text-text-primary">{s.title}</h3>
                  <p className="mt-1 text-sm text-text-secondary">{s.detail}</p>
                  <div className="mt-3 flex items-start justify-between gap-3 rounded-md border border-border-color bg-surface-muted px-3 py-2">
                    <code className="text-xs font-mono text-text-primary break-all sm:break-normal sm:overflow-x-auto sm:whitespace-nowrap">{s.code}</code>
                    <CopyButton text={s.code} className="shrink-0" />
                  </div>
                  <p className="mt-2 text-xs text-text-muted">{s.note}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* 9. Integrations */}
      <section className="border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">Integrations</h2>
          <p className="mx-auto mt-3 max-w-2xl text-center text-sm text-text-secondary">
            Optional engines auto-detect if installed. CI and alerting integrations are first-class.
          </p>
          <div className="mt-10 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {INTEGRATIONS.map(i => (
              <Card key={i.name} className="flex items-start gap-3 p-4">
                <i.icon size={18} className="mt-0.5 shrink-0 text-primary-blue" />
                <div>
                  <div className="text-sm font-semibold text-text-primary">{i.name}</div>
                  <div className="mt-1 text-xs leading-relaxed text-text-secondary">{i.desc}</div>
                </div>
              </Card>
            ))}
          </div>
        </div>
      </section>

      {/* 10. Free vs Pro */}
      <section className="px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">Free vs Pro</h2>
          <p className="mx-auto mt-3 max-w-2xl text-center text-sm text-text-secondary">
            ForgeGuardian is open-core — the engine, CLI, scanner, and community tools are Apache 2.0,
            free forever. Pro adds AI-powered features and team capabilities.
          </p>
          <Card className="mt-8 overflow-hidden">
            <table className="w-full border-collapse text-sm">
              <thead>
                <tr className="border-b border-border-color">
                  <th className="px-4 py-3 text-left font-mono text-[0.7rem] uppercase text-text-secondary">Feature</th>
                  <th className="px-4 py-3 text-center font-mono text-[0.7rem] uppercase text-text-secondary">Community</th>
                  <th className="px-4 py-3 text-center font-mono text-[0.7rem] uppercase text-success">Pro</th>
                </tr>
              </thead>
              <tbody>
                {FREE_PRO_ROWS.map((row, i) => (
                  <tr key={row.feature} className={i < FREE_PRO_ROWS.length - 1 ? 'border-b border-border-color/60' : ''}>
                    <td className="px-4 py-2.5 text-text-primary">{row.feature}</td>
                    <td className="px-4 py-2.5 text-center"><TierMark ok={row.free} /></td>
                    <td className="px-4 py-2.5 text-center"><TierMark ok={row.pro} /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </Card>
          <p className="mt-4 text-center text-xs text-text-muted">
            Community signatures stay community-owned — revenue from Pro funds continued
            development. You'll never lose access to what you have today.
          </p>
        </div>
      </section>

      {/* 10b. Enterprise / consulting teaser */}
      <section className="relative overflow-hidden border-t border-border-color bg-surface px-6 py-20">
        <TopoBackground className="text-text-primary" opacity={0.04} lines={8} />
        <div className="relative z-10 mx-auto max-w-3xl text-center">
          <h2 className="text-2xl font-bold text-text-primary">Work with us directly</h2>
          <p className="mx-auto mt-3 max-w-2xl text-sm text-text-secondary">
            Beyond the open-source tool and Pro tier, we help companies deploy, run, and extend
            ForgeGuardian inside their own environment — setup, ongoing management, and custom
            development.
          </p>
          <div className="mt-8 flex justify-center">
            <button
              onClick={onNavigateEnterprise}
              className="flex items-center gap-2 rounded-md bg-primary-blue px-5 py-2.5 text-sm font-semibold text-white hover:opacity-90"
            >
              See enterprise services <ArrowRight size={14} />
            </button>
          </div>
        </div>
      </section>

      {/* 11. Final CTA */}
      <section className="border-t border-border-color bg-[#0B0D0F] px-6 py-20 text-center text-white">
        <h2 className="text-2xl font-bold">Make security continuously visible.</h2>
        <button
          onClick={() => scrollTo(loginRef)}
          className="mt-6 inline-flex items-center gap-2 rounded-md bg-primary-blue px-6 py-2.5 text-sm font-semibold text-white hover:opacity-90"
        >
          Get started <ArrowRight size={14} />
        </button>
      </section>

      {/* 13. Login section (split-screen) */}
      <section ref={loginRef} className="px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <AuthPanel onLoggedIn={onLoggedIn} />
        </div>
      </section>

      {/* 12. Footer */}
      <footer className="border-t border-border-color px-6 py-8 text-center text-xs text-text-muted">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3">
          <span>© {new Date().getFullYear()} ForgeGuardian — Apache 2.0 Licensed</span>
          <div className="flex gap-4">
            <a href="https://github.com/mah3sec/forgeguardian" target="_blank" rel="noreferrer" className="hover:text-text-primary">
              GitHub
            </a>
            <a href="https://github.com/mah3sec/forgeguardian#readme" target="_blank" rel="noreferrer" className="hover:text-text-primary">
              Docs
            </a>
            <a href="https://github.com/mah3sec/forgeguardian/blob/main/LICENSE" target="_blank" rel="noreferrer" className="hover:text-text-primary">
              License
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}

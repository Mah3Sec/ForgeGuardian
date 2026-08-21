import { ArrowRight, Github, Wrench, LifeBuoy, Blocks, MessageSquare, Search, Settings2 } from 'lucide-react';
import { TopoBackground } from '../components/TopoBackground';

const GITHUB_URL = 'https://github.com/mah3sec/forgeguardian';

const OFFERS = [
  {
    icon: Wrench,
    title: 'Setup',
    desc: 'Deploy ForgeGuardian into your environment, wire it into existing auth and CI, and configure policy-as-code for your ecosystems.',
  },
  {
    icon: LifeBuoy,
    title: 'Management',
    desc: 'Ongoing maintenance of self-hosted deployments — upgrades, monitoring, and troubleshooting so your team stays focused on shipping.',
  },
  {
    icon: Blocks,
    title: 'Custom development',
    desc: 'Custom integrations, workflow automation, and architecture support built on top of the open-core engine.',
  },
];

const ENGAGEMENT_STEPS = [
  { step: '1', icon: MessageSquare, title: 'Talk to us',    desc: 'Tell us about your environment, ecosystems, and what you need ForgeGuardian to do that it doesn’t out of the box today.' },
  { step: '2', icon: Search,        title: 'Scope',          desc: 'We map your ask onto the open-core engine’s actual capabilities — what’s a config change, what’s custom work.' },
  { step: '3', icon: Settings2,     title: 'Build & deploy', desc: 'We implement, deploy, and hand off — or keep managing it, based on what you need.' },
];

interface EnterprisePageProps {
  onNavigateHome: () => void;
}

export function EnterprisePage({ onNavigateHome }: EnterprisePageProps) {
  return (
    <div className="min-h-screen bg-bg-base text-text-primary">
      <nav className="sticky top-0 z-40 flex items-center justify-between gap-2 border-b border-border-color bg-surface/80 px-4 sm:px-6 py-3.5 backdrop-blur-md">
        <button onClick={onNavigateHome} className="flex items-center min-w-0 shrink bg-transparent border-none cursor-pointer">
          <img src="/logo-icon.png" alt="ForgeGuardian" className="shrink-0" style={{ height: 40, objectFit: 'contain' }} />
        </button>
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
            onClick={onNavigateHome}
            className="whitespace-nowrap rounded-md border border-border-color bg-surface px-3 sm:px-4 py-1.5 text-sm font-medium text-text-primary hover:bg-surface-muted"
          >
            <span className="sm:hidden">Home</span>
            <span className="hidden sm:inline">Back to home</span>
          </button>
        </div>
      </nav>

      {/* Hero */}
      <section className="relative overflow-hidden px-6 pt-20 pb-16 text-center">
        <TopoBackground className="text-text-primary" opacity={0.05} lines={10} />
        <div className="relative z-10 mx-auto max-w-3xl">
          <p className="mb-4 font-mono text-xs font-semibold uppercase tracking-widest text-primary-blue">
            Setup · Management · Custom development
          </p>
          <h1 className="text-4xl font-bold leading-tight tracking-tight text-text-primary sm:text-5xl">
            Work with us directly.
          </h1>
          <p className="mx-auto mt-5 max-w-2xl text-base leading-relaxed text-text-secondary">
            Beyond the open-source tool and the Pro tier, we help companies deploy, run, and extend
            ForgeGuardian inside their own environment — setup, ongoing management, and custom
            development on top of the open-core engine.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
            <a
              href={`${GITHUB_URL}/issues/new?title=Enterprise%20inquiry&labels=enterprise`}
              target="_blank"
              rel="noreferrer"
              className="flex items-center gap-2 rounded-md bg-primary-blue px-5 py-2.5 text-sm font-semibold text-white hover:opacity-90"
            >
              Talk to us <ArrowRight size={14} />
            </a>
          </div>
        </div>
      </section>

      {/* Offers */}
      <section className="border-t border-border-color bg-surface px-6 py-20">
        <div className="mx-auto max-w-5xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">What we help with</h2>
          <div className="mt-10 grid gap-5 sm:grid-cols-3">
            {OFFERS.map(o => (
              <div key={o.title} className="rounded-xl border border-border-color bg-bg-base p-6 shadow-sm">
                <o.icon size={20} className="text-primary-blue" />
                <h3 className="mt-3 text-sm font-semibold text-text-primary">{o.title}</h3>
                <p className="mt-2 text-sm leading-relaxed text-text-secondary">{o.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* How engagement works */}
      <section className="px-6 py-20">
        <div className="mx-auto max-w-4xl">
          <h2 className="text-center text-2xl font-bold text-text-primary">How it works</h2>
          <div className="mt-10 grid gap-6 sm:grid-cols-3">
            {ENGAGEMENT_STEPS.map(s => (
              <div key={s.step} className="text-center">
                <div className="mx-auto flex h-9 w-9 items-center justify-center rounded-full bg-primary-blue font-mono text-sm font-bold text-white">
                  {s.step}
                </div>
                <h3 className="mt-3 text-sm font-semibold text-text-primary">{s.title}</h3>
                <p className="mt-1.5 text-xs leading-relaxed text-text-secondary">{s.desc}</p>
              </div>
            ))}
          </div>
          <p className="mx-auto mt-10 max-w-xl text-center text-xs text-text-muted">
            This is a direct-engagement offering, not a self-serve product — there's no signup form.
            Open an issue and we'll scope it with you.
          </p>
        </div>
      </section>

      {/* Final CTA */}
      <section className="border-t border-border-color bg-[#0B0D0F] px-6 py-20 text-center text-white">
        <h2 className="text-2xl font-bold">Let's talk about your environment.</h2>
        <a
          href={`${GITHUB_URL}/issues/new?title=Enterprise%20inquiry&labels=enterprise`}
          target="_blank"
          rel="noreferrer"
          className="mt-6 inline-flex items-center gap-2 rounded-md bg-primary-blue px-6 py-2.5 text-sm font-semibold text-white hover:opacity-90"
        >
          Talk to us <ArrowRight size={14} />
        </a>
      </section>

      <footer className="border-t border-border-color px-6 py-8 text-center text-xs text-text-muted">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3">
          <span>© {new Date().getFullYear()} ForgeGuardian — Apache 2.0 Licensed</span>
          <div className="flex gap-4">
            <a href={GITHUB_URL} target="_blank" rel="noreferrer" className="hover:text-text-primary">GitHub</a>
            <button onClick={onNavigateHome} className="bg-transparent border-none cursor-pointer hover:text-text-primary">Home</button>
          </div>
        </div>
      </footer>
    </div>
  );
}

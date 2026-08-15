import { useMutation } from '@tanstack/react-query'
import { Puzzle, ShieldCheck, GitMerge, Webhook, CheckCircle2, Loader2, XCircle } from 'lucide-react'
import { testWebhook } from '../lib/api'
import { useUIStore } from '../store/ui'

type IntegrationStatus = 'available' | 'optional'

interface IntegrationCard {
  icon: React.ElementType
  name: string
  description: string
  status: IntegrationStatus
  statusLabel: string
}

// Scan engines — optional CLI tools auto-detected at scan time
// (see ScanPage.tsx's EngineStatusBar / result.engines). This page has no
// live scan result in context, so we describe them honestly as
// "optional — auto-detected" rather than fabricating a live
// installed/not-installed check with no data behind it.
const SCAN_ENGINES: IntegrationCard[] = [
  {
    icon: ShieldCheck,
    name: 'Grype',
    description: 'Vulnerability scanner for container images and filesystems — matches packages against known CVE databases.',
    status: 'optional',
    statusLabel: 'Optional — auto-detected',
  },
  {
    icon: ShieldCheck,
    name: 'Trivy',
    description: 'Comprehensive scanner covering OS packages, language dependencies, misconfigurations, and secrets.',
    status: 'optional',
    statusLabel: 'Optional — auto-detected',
  },
  {
    icon: ShieldCheck,
    name: 'Semgrep',
    description: 'Static analysis engine — pattern-based rules for finding insecure code and known vulnerable code paths.',
    status: 'optional',
    statusLabel: 'Optional — auto-detected',
  },
]

const CICD: IntegrationCard[] = [
  {
    icon: GitMerge,
    name: 'GitHub Actions',
    description: 'Run fgctl scan in CI, emit SARIF, and upload findings to the GitHub Security tab. See the CI/CD page for the exact workflow snippet.',
    status: 'available',
    statusLabel: 'Available',
  },
]

function IntegrationTile({ card }: { card: IntegrationCard }) {
  return (
    <div className="rounded-xl border border-border-color bg-surface p-4 shadow-sm flex flex-col gap-3">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <card.icon size={18} className="text-primary-blue" />
          <span className="text-sm font-semibold text-text-primary">{card.name}</span>
        </div>
        <span
          className="text-[0.65rem] font-medium px-2 py-0.5 rounded-full shrink-0"
          style={{
            background: card.status === 'available' ? 'var(--blue-light)' : 'var(--surface-muted)',
            color: card.status === 'available' ? 'var(--primary-blue)' : 'var(--text-secondary)',
          }}
        >
          {card.statusLabel}
        </span>
      </div>
      <p className="text-xs text-text-secondary leading-relaxed">{card.description}</p>
    </div>
  )
}

function NotificationsCard() {
  const navigate = useUIStore(s => s.navigate)
  const test = useMutation({ mutationFn: testWebhook })

  return (
    <div className="rounded-xl border border-border-color bg-surface p-4 shadow-sm flex flex-col gap-3">
      <div className="flex items-start justify-between">
        <div className="flex items-center gap-2.5">
          <Webhook size={18} className="text-primary-blue" />
          <span className="text-sm font-semibold text-text-primary">Slack / Discord / Generic webhook</span>
        </div>
        <span
          className="text-[0.65rem] font-medium px-2 py-0.5 rounded-full shrink-0"
          style={{ background: 'var(--surface-muted)', color: 'var(--text-secondary)' }}
        >
          Optional — configure locally
        </span>
      </div>
      <p className="text-xs text-text-secondary leading-relaxed">
        Push security alerts to a webhook URL you configure via <code className="text-[0.7rem]">fgctl config set notify.slack_webhook_url=&lt;url&gt;</code>
        {' '}(or the Discord / generic equivalents). This is a configured URL, not an OAuth-style connection —
        there is nothing to "connect" here beyond setting the URL.
      </p>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => test.mutate()}
          disabled={test.isPending}
          className="text-xs font-medium rounded-md border border-border-color px-3 py-1.5 text-text-primary hover:bg-surface-muted disabled:opacity-50 flex items-center gap-1.5"
        >
          {test.isPending && <Loader2 size={12} className="animate-spin" />}
          Test webhook endpoint
        </button>
        <button
          type="button"
          onClick={() => navigate('/webhooks')}
          className="text-xs text-primary-blue hover:underline"
        >
          Full webhook settings →
        </button>
      </div>
      {test.isSuccess && test.data.status === 'ok' && (
        <p className="text-xs text-success flex items-center gap-1.5">
          <CheckCircle2 size={12} /> {test.data.message}
        </p>
      )}
      {test.isSuccess && test.data.status === 'not_configured' && (
        <p className="text-xs text-text-secondary flex items-center gap-1.5">
          <Webhook size={12} /> {test.data.message}
        </p>
      )}
      {test.isSuccess && test.data.status === 'failed' && (
        <div className="text-xs text-critical flex flex-col gap-1">
          <span className="flex items-center gap-1.5"><XCircle size={12} /> Delivery failed:</span>
          {test.data.errors?.map((e, i) => <span key={i} className="ml-4">{e}</span>)}
        </div>
      )}
      {test.isError && (
        <p className="text-xs text-critical flex items-center gap-1.5">
          <XCircle size={12} /> {(test.error as Error).message}
        </p>
      )}
    </div>
  )
}

export function IntegrationsPage() {
  return (
    <div className="flex flex-col gap-5 p-6">
      <div className="flex items-center gap-3">
        <Puzzle size={20} className="text-primary-blue" />
        <div>
          <h1 className="text-lg font-bold text-text-primary">Integrations</h1>
          <p className="mt-0.5 text-xs text-text-secondary">
            Real integrations only — CLI tool auto-detection and locally-configured webhooks, not
            OAuth-style connected apps.
          </p>
        </div>
      </div>

      <div>
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted mb-2">Scan engines</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          {SCAN_ENGINES.map(c => <IntegrationTile key={c.name} card={c} />)}
        </div>
      </div>

      <div>
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted mb-2">CI/CD</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          {CICD.map(c => <IntegrationTile key={c.name} card={c} />)}
        </div>
      </div>

      <div>
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted mb-2">Notifications</h2>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <NotificationsCard />
        </div>
      </div>
    </div>
  )
}

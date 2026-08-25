import { useMutation } from '@tanstack/react-query';
import { Webhook, Bell, Slack, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { testWebhook } from '../lib/api';

export function WebhooksPage() {
  const test = useMutation({ mutationFn: testWebhook });

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Webhook size={20} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Webhooks</h1>
          <p className="text-sm mt-0.5" style={{ color: 'var(--color-muted)' }}>Configure outbound webhooks for security events and scan completions.</p>
        </div>
      </div>

      <div className="rounded-lg p-5" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <div className="flex items-center gap-2 mb-3">
          <Bell size={14} style={{ color: 'var(--color-warn)' }} />
          <span className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>CONFIGURED LOCALLY, NOT VIA THIS PAGE</span>
        </div>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
          Webhooks fire on scan completion, critical finding detection, and policy violations — configured
          on the machine running the ForgeGuardian API server, via <code style={{ color: 'var(--color-safe)' }}>fgctl config set</code>{' '}
          (writes to <code style={{ color: 'var(--color-safe)' }}>~/.forgeguardian/config.yaml</code>). There's no
          dashboard form for this yet — use the commands below, then verify with the test button.
        </p>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.625rem' }}>
        {[
          { icon: Slack,   label: 'Slack',            hint: 'fgctl config set notify.slack_webhook_url=https://hooks.slack.com/...' },
          { icon: Webhook, label: 'Generic Webhook',   hint: 'fgctl config set notify.webhook_url=https://your-endpoint.com/hook' },
          { icon: Bell,    label: 'Discord',           hint: 'fgctl config set notify.discord_webhook_url=https://discord.com/api/webhooks/...' },
        ].map(w => (
          <div
            key={w.label}
            className="rounded-lg p-4"
            style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', gap: '0.875rem' }}
          >
            <w.icon size={16} style={{ color: 'var(--color-indigo)', flexShrink: 0 }} />
            <div style={{ flex: 1 }}>
              <p style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--fg)' }}>{w.label}</p>
              <code style={{ fontSize: '0.68rem', color: 'var(--color-safe)', fontFamily: 'var(--font-mono)' }}>{w.hint}</code>
            </div>
          </div>
        ))}
        <div
          className="rounded-lg p-4"
          style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', gap: '0.875rem' }}
        >
          <Bell size={16} style={{ color: 'var(--color-indigo)', flexShrink: 0 }} />
          <div style={{ flex: 1 }}>
            <p style={{ fontSize: '0.8rem', fontWeight: 600, color: 'var(--fg)' }}>Severity threshold</p>
            <code style={{ fontSize: '0.68rem', color: 'var(--color-safe)', fontFamily: 'var(--font-mono)' }}>
              fgctl config set notify.on_severity=high
            </code>
          </div>
        </div>
      </div>

      <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold mb-3" style={{ color: 'var(--color-muted)' }}>TEST DELIVERY</p>
        <p className="text-xs mb-3" style={{ color: 'var(--color-muted)' }}>
          Hits the API server's <code style={{ color: 'var(--color-safe)' }}>POST /api/v1/webhooks/test</code> —
          it reads whatever's currently in <code style={{ color: 'var(--color-safe)' }}>config.yaml</code> on
          that machine and attempts a real delivery.
        </p>
        <button
          type="button"
          onClick={() => test.mutate()}
          disabled={test.isPending}
          className="text-xs font-medium rounded-md border px-3 py-1.5 flex items-center gap-1.5"
          style={{ borderColor: 'rgba(255,255,255,0.12)', color: 'var(--fg)', opacity: test.isPending ? 0.5 : 1 }}
        >
          {test.isPending && <Loader2 size={12} className="animate-spin" />}
          Test webhook endpoint
        </button>

        {test.isSuccess && test.data.status === 'ok' && (
          <p className="text-xs mt-3 flex items-center gap-1.5" style={{ color: 'var(--color-safe)' }}>
            <CheckCircle2 size={12} /> {test.data.message}
          </p>
        )}
        {test.isSuccess && test.data.status === 'not_configured' && (
          <p className="text-xs mt-3 flex items-center gap-1.5" style={{ color: 'var(--color-muted)' }}>
            <Webhook size={12} /> {test.data.message}
          </p>
        )}
        {test.isSuccess && test.data.status === 'failed' && (
          <div className="text-xs mt-3 flex flex-col gap-1" style={{ color: 'var(--color-critical)' }}>
            <span className="flex items-center gap-1.5"><XCircle size={12} /> Delivery failed:</span>
            {test.data.errors?.map((e, i) => <span key={i} className="ml-4">{e}</span>)}
          </div>
        )}
        {test.isError && (
          <p className="text-xs mt-3 flex items-center gap-1.5" style={{ color: 'var(--color-critical)' }}>
            <XCircle size={12} /> {(test.error as Error).message}
          </p>
        )}
      </div>
    </div>
  );
}

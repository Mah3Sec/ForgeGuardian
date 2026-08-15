import { useQuery } from '@tanstack/react-query'
import {
  Info, Server, ShieldCheck, LogOut, Webhook, KeyRound,
  Users, CreditCard, ScrollText, Clock,
} from 'lucide-react'
import { getAuthStatus, logout } from '../lib/api'
import { useUIStore } from '../store/ui'

function SectionCard({ title, icon: Icon, children }: { title: string; icon: React.ElementType; children: React.ReactNode }) {
  return (
    <div className="rounded-xl border border-border-color bg-surface p-5 shadow-sm">
      <div className="flex items-center gap-2 mb-3">
        <Icon size={14} className="text-text-muted" />
        <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">{title}</h2>
      </div>
      {children}
    </div>
  )
}

function ComingSoonBadge() {
  return (
    <span className="text-[0.65rem] font-medium px-2 py-0.5 rounded-full bg-surface-muted text-text-secondary shrink-0">
      Coming soon
    </span>
  )
}

// General — real API URL display, restyled to the new light-theme tokens.
function GeneralSection() {
  return (
    <SectionCard title="General" icon={Server}>
      <div>
        <label className="block text-xs font-medium text-text-secondary mb-1">API URL</label>
        <div className="rounded-md border border-border-color bg-bg-base px-3 py-2 text-sm font-mono text-text-primary">
          {import.meta.env.VITE_API_URL ?? 'http://localhost:8080'}
        </div>
        <p className="text-xs text-text-muted mt-1.5">
          Set <code className="font-mono">VITE_API_URL</code> in <code className="font-mono">.env</code> to change the API server address.
        </p>
      </div>
    </SectionCard>
  )
}

// Security — real auth system state via getAuthStatus(). Sidebar.tsx already
// has a logout button when auth is enabled; this section shows status and a
// convenience logout action, not a separate session-management UI (the
// backend is single-JWT-cookie with no session table).
function SecuritySection() {
  const auth = useQuery({ queryKey: ['auth-me', 'settings'], queryFn: getAuthStatus, retry: false })

  return (
    <SectionCard title="Security" icon={ShieldCheck}>
      {auth.isLoading ? (
        <p className="text-xs text-text-secondary">Checking auth status…</p>
      ) : auth.isError ? (
        <p className="text-xs text-critical">Could not reach the API to check auth status.</p>
      ) : (
        <div className="flex items-center justify-between">
          <div>
            <p className="text-sm text-text-primary">
              Login {auth.data?.auth_enabled ? 'enabled' : 'disabled'}
            </p>
            <p className="text-xs text-text-secondary mt-0.5">
              {auth.data?.auth_enabled
                ? auth.data.authenticated
                  ? `Signed in${auth.data.email ? ` as ${auth.data.email}` : ''}.`
                  : 'Not currently signed in.'
                : 'FG_API_KEY / session auth is not configured on this server — the dashboard is unauthenticated.'}
            </p>
          </div>
          {auth.data?.auth_enabled && auth.data.authenticated && (
            <button
              type="button"
              onClick={() => logout()}
              className="flex items-center gap-1.5 text-xs font-medium rounded-md border border-border-color px-3 py-1.5 text-text-primary hover:bg-surface-muted shrink-0"
            >
              <LogOut size={12} /> Log out
            </button>
          )}
        </div>
      )}
    </SectionCard>
  )
}

// Notifications — real webhook-test config already covered fully by
// WebhooksPage.tsx and the Integrations page's Notifications card; this
// section is a brief pointer, not a rebuild.
function NotificationsSection() {
  const navigate = useUIStore(s => s.navigate)
  return (
    <SectionCard title="Notifications" icon={Webhook}>
      <p className="text-sm text-text-primary">
        Configure Slack, Discord, or generic webhook alerts via <code className="font-mono text-xs">fgctl config set</code>.
      </p>
      <div className="flex gap-3 mt-2">
        <button type="button" onClick={() => navigate('/webhooks')} className="text-xs text-primary-blue hover:underline">
          Webhook settings →
        </button>
        <button type="button" onClick={() => navigate('/integrations')} className="text-xs text-primary-blue hover:underline">
          Integrations →
        </button>
      </div>
    </SectionCard>
  )
}

// API keys — there is no key-management CRUD backend (one shared static
// FG_API_KEY env var, no create/list/revoke endpoints). Show the current
// key's configured status only — never the secret value, never a fake
// "create new key" form with nowhere real to POST to.
function ApiKeysSection() {
  const configured = (import.meta.env.VITE_API_KEY ?? '') !== ''
  return (
    <SectionCard title="API keys" icon={KeyRound}>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm text-text-primary">Dashboard API key</p>
          <p className="text-xs text-text-secondary mt-0.5">
            ForgeGuardian uses a single shared static key (server-side <code className="font-mono">FG_API_KEY</code>,
            client-side <code className="font-mono">VITE_API_KEY</code>) — there is no multi-key create/list/revoke
            system yet.
          </p>
        </div>
        <span
          className="text-[0.65rem] font-medium px-2 py-0.5 rounded-full shrink-0"
          style={{
            background: configured ? 'var(--blue-light)' : 'var(--surface-muted)',
            color: configured ? 'var(--primary-blue)' : 'var(--text-secondary)',
          }}
        >
          {configured ? 'Configured' : 'Not configured'}
        </span>
      </div>
    </SectionCard>
  )
}

// Team, Billing, Audit Log — deliberate, spec-acknowledged placeholders.
// No multi-user backend, no billing system, no persisted audit trail exist
// yet — these are intentional, polished "coming soon" panels rather than
// fabricated sample data that could be mistaken for real functionality.
function PlaceholderSection({
  title,
  icon,
  reason,
}: {
  title: string
  icon: React.ElementType
  reason: string
}) {
  const Icon = icon
  return (
    <div className="rounded-xl border border-dashed border-border-color bg-surface-muted/40 p-5">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <Icon size={14} className="text-text-muted" />
          <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted">{title}</h2>
        </div>
        <ComingSoonBadge />
      </div>
      <p className="text-xs text-text-secondary">{reason}</p>
    </div>
  )
}

export function SettingsPage() {
  return (
    <div className="p-6 space-y-4 max-w-2xl">
      <h1 className="text-lg font-bold text-text-primary">Settings</h1>

      <GeneralSection />
      <SecuritySection />
      <NotificationsSection />
      <ApiKeysSection />

      <PlaceholderSection
        title="Team"
        icon={Users}
        reason="No multi-user backend yet — ForgeGuardian currently runs as a single-tenant, single-credential deployment. Team member management is planned for a future release."
      />
      <PlaceholderSection
        title="Billing"
        icon={CreditCard}
        reason="No billing system exists yet. ForgeGuardian's core is free and open source; Pro-tier billing infrastructure is not yet built."
      />
      <PlaceholderSection
        title="Audit Log"
        icon={ScrollText}
        reason="No persisted audit trail exists server-side yet. Recent activity (scans, findings) is visible on the System Audit page, but a durable, queryable audit log is not yet implemented."
      />

      {/* Version info */}
      <div className="rounded-xl border border-border-color bg-surface p-4 flex items-start gap-3 shadow-sm">
        <Info size={14} className="text-text-muted mt-0.5" />
        <div className="space-y-0.5">
          <p className="text-xs text-text-secondary">
            ForgeGuardian Dashboard · Local-first AI-native Supply Chain Security
          </p>
          <p className="text-xs text-text-secondary flex items-center gap-1">
            <Clock size={11} /> Run <code className="font-mono">fgctl version</code> for CLI version info.
          </p>
        </div>
      </div>
    </div>
  )
}

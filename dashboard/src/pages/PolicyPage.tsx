import { useState, useEffect } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Shield, CheckCircle, XCircle, AlertTriangle } from 'lucide-react'
import { getPolicyStatus, savePolicy } from '../lib/api'
import { Skeleton } from '../components/ui/skeleton'
import { Switch } from '../components/ui/switch'
import { TagInput } from '../components/TagInput'
import type { Policy } from '../types/api'

const DEFAULT_POLICY: Policy = {
  version: 1,
  fail_on: '',
  deny_packages: [],
  allow_licenses: [],
  max_package_age_days: 0,
  block_typosquatting: false,
  block_abandoned: false,
  require_signing: false,
}

const FAIL_ON_OPTIONS = ['', 'critical', 'high', 'medium', 'low']

export default function PolicyPage() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['policy-status'],
    queryFn: getPolicyStatus,
    refetchInterval: 30_000,
    retry: false,
  })

  const [form, setForm] = useState<Policy>(DEFAULT_POLICY)
  const [savedMsg, setSavedMsg] = useState('')

  // Seed local form state from the loaded policy (or defaults if unconfigured).
  useEffect(() => {
    if (data) {
      setForm(data.configured ? data.policy : DEFAULT_POLICY)
    }
  }, [data])

  const save = useMutation({
    mutationFn: () => savePolicy(form),
    onSuccess: (saved) => {
      setForm(saved)
      setSavedMsg('Policy saved.')
      qc.invalidateQueries({ queryKey: ['policy-status'] })
      setTimeout(() => setSavedMsg(''), 3000)
    },
  })

  const rules = [
    {
      label: 'fail_on severity',
      value: data?.policy.fail_on || 'not set',
      active: !!data?.policy.fail_on,
      description: 'Exit code 2 if findings meet this severity threshold',
    },
    {
      label: 'block_typosquatting',
      value: data?.policy.block_typosquatting ? 'enabled' : 'disabled',
      active: !!data?.policy.block_typosquatting,
      description: 'Fail scan if typosquatting is detected',
    },
    {
      label: 'deny_packages',
      value: (data?.policy.deny_packages?.length ?? 0) > 0
        ? `${data?.policy.deny_packages?.length} package(s) blocked`
        : 'none',
      active: (data?.policy.deny_packages?.length ?? 0) > 0,
      description: 'Explicitly blocked package names',
    },
  ]

  return (
    <div style={{ padding: '1.5rem', display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <h1 className="text-lg font-bold font-mono" style={{ color: 'var(--fg)' }}>Security Policy</h1>
        <p className="text-xs mt-0.5" style={{ color: 'var(--color-muted)' }}>
          Policy-as-code enforcement — edit below, or use <code style={{ color: 'var(--color-safe)' }}>fgctl policy set</code>
        </p>
      </div>

      {/* Status banner — three real states: unconfigured, not-yet-evaluated
          (violations is null — no aggregation query exists yet), or a real
          count. Never show a fabricated "no violations" all-clear. */}
      {!isLoading && (
        <div style={{
          background: !data?.configured
            ? 'rgba(255,255,255,0.04)'
            : data?.violations == null
              ? 'rgba(255,255,255,0.04)'
              : data.violations === 0 ? 'rgba(0,255,135,0.06)' : 'rgba(255,171,64,0.08)',
          border: `1px solid ${!data?.configured
            ? 'rgba(255,255,255,0.08)'
            : data?.violations == null
              ? 'rgba(255,255,255,0.08)'
              : data.violations === 0 ? 'rgba(0,255,135,0.2)' : 'rgba(255,171,64,0.3)'}`,
          borderRadius: '0.5rem',
          padding: '0.875rem 1rem',
          display: 'flex',
          alignItems: 'center',
          gap: '0.75rem',
        }}>
          {!data?.configured || data?.violations == null
            ? <AlertTriangle size={16} style={{ color: 'var(--color-muted)', flexShrink: 0 }} />
            : data.violations === 0
              ? <CheckCircle size={16} style={{ color: 'var(--color-safe)', flexShrink: 0 }} />
              : <AlertTriangle size={16} style={{ color: 'var(--color-warn)', flexShrink: 0 }} />
          }
          <div>
            <p className="text-sm font-semibold" style={{ color: 'var(--fg)' }}>
              {!data?.configured
                ? 'No policy configured yet — showing defaults'
                : data?.violations == null
                  ? 'Violation count not yet evaluated'
                  : data.violations === 0 ? 'No policy violations detected' : `${data.violations} policy violation(s)`}
            </p>
            <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
              Last evaluated: {data?.last_evaluated ? new Date(data.last_evaluated).toLocaleTimeString() : '—'}
            </p>
          </div>
        </div>
      )}

      {/* Policy rules (read-only summary) */}
      <div style={{ background: 'var(--bg-surface)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '0.5rem', overflow: 'hidden' }}>
        <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid rgba(255,255,255,0.07)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Shield size={14} style={{ color: 'var(--color-muted)' }} />
          <span className="text-xs font-semibold" style={{ color: 'var(--fg)' }}>Enforcement Rules</span>
        </div>
        {isLoading ? (
          <div style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-12 w-full" />)}
          </div>
        ) : (
          <div>
            {rules.map((rule, i) => (
              <div key={rule.label} style={{
                padding: '0.875rem 1rem',
                borderBottom: i < rules.length - 1 ? '1px solid rgba(255,255,255,0.05)' : 'none',
                display: 'flex',
                alignItems: 'center',
                gap: '1rem',
              }}>
                {rule.active
                  ? <CheckCircle size={14} style={{ color: 'var(--color-safe)', flexShrink: 0 }} />
                  : <XCircle size={14} style={{ color: 'var(--color-muted)', flexShrink: 0 }} />
                }
                <div style={{ flex: 1 }}>
                  <code className="text-xs" style={{ color: 'var(--fg)' }}>{rule.label}</code>
                  <p className="text-xs mt-0.5" style={{ color: 'var(--color-muted)' }}>{rule.description}</p>
                </div>
                <span className="text-xs font-mono px-2 py-0.5 rounded" style={{
                  background: rule.active ? 'rgba(0,255,135,0.1)' : 'rgba(255,255,255,0.05)',
                  color: rule.active ? 'var(--color-safe)' : 'var(--color-muted)',
                }}>
                  {rule.value}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Edit policy */}
      {!isLoading && (
        <div style={{ background: 'var(--bg-surface)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '0.5rem', padding: '1.25rem', display: 'flex', flexDirection: 'column', gap: '1rem' }}>
          <p className="text-xs font-semibold" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            Edit Policy
          </p>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>FAIL ON SEVERITY</label>
            <select
              value={form.fail_on}
              onChange={e => setForm(f => ({ ...f, fail_on: e.target.value }))}
              className="w-full rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)', maxWidth: 240 }}
            >
              {FAIL_ON_OPTIONS.map(o => <option key={o} value={o}>{o || '(not set)'}</option>)}
            </select>
          </div>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>MAX PACKAGE AGE (DAYS, 0 = unlimited)</label>
            <input
              type="number"
              min={0}
              value={form.max_package_age_days}
              onChange={e => setForm(f => ({ ...f, max_package_age_days: Number(e.target.value) || 0 }))}
              className="rounded px-3 py-2 text-sm font-mono"
              style={{ background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)', maxWidth: 160 }}
            />
          </div>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            {[
              { key: 'block_typosquatting' as const, label: 'Block typosquatting', description: 'Fail scan if typosquatting is detected' },
              { key: 'block_abandoned' as const, label: 'Block abandoned packages', description: 'Fail scan if a dependency is abandoned/unmaintained' },
              { key: 'require_signing' as const, label: 'Require signing', description: 'Fail scan if artifacts lack a valid Sigstore attestation' },
            ].map(({ key, label, description }) => (
              <div key={key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '1rem' }}>
                <div>
                  <p className="text-sm" style={{ color: 'var(--fg)' }}>{label}</p>
                  <p className="text-xs" style={{ color: 'var(--color-muted)' }}>{description}</p>
                </div>
                <Switch checked={form[key]} onCheckedChange={v => setForm(f => ({ ...f, [key]: v }))} />
              </div>
            ))}
          </div>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>DENIED PACKAGES</label>
            <TagInput
              value={form.deny_packages}
              onChange={v => setForm(f => ({ ...f, deny_packages: v }))}
              placeholder="package-name, press Enter"
              color="#FF3D3D"
            />
          </div>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>ALLOWED LICENSES</label>
            <TagInput
              value={form.allow_licenses}
              onChange={v => setForm(f => ({ ...f, allow_licenses: v }))}
              placeholder="MIT, Apache-2.0, …"
              color="#00FF87"
            />
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <button
              onClick={() => save.mutate()}
              disabled={save.isPending}
              className="px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
              style={{ background: 'var(--color-safe)', color: '#0A0B0D' }}
            >
              {save.isPending ? 'Saving…' : 'Save Policy'}
            </button>
            {savedMsg && (
              <span className="text-xs font-mono" style={{ color: 'var(--color-safe)' }}>{savedMsg}</span>
            )}
            {save.isError && (
              <span className="text-xs font-mono" style={{ color: 'var(--color-critical)' }}>
                {(save.error as Error).message}
              </span>
            )}
          </div>
        </div>
      )}

      {/* Denied packages (from server state) */}
      {!isLoading && (data?.policy.deny_packages?.length ?? 0) > 0 && (
        <div style={{ background: 'var(--bg-surface)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '0.5rem', padding: '1rem' }}>
          <p className="text-xs font-semibold mb-2" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
            Denied Packages
          </p>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
            {data?.policy.deny_packages?.map(pkg => (
              <span key={pkg} className="text-xs font-mono px-2.5 py-1 rounded"
                style={{ background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.25)' }}>
                {pkg}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

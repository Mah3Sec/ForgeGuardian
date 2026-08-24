import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Package, Search } from 'lucide-react'
import { listPackages, getDashboardStats } from '../lib/api'
import { Skeleton } from '../components/ui/skeleton'

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp']

export default function InventoryPage() {
  const [search, setSearch] = useState('')
  const [eco, setEco] = useState('')
  const [page, setPage] = useState(1)
  const PAGE_SIZE = 50

  const stats = useQuery({ queryKey: ['dashboard-stats'], queryFn: () => getDashboardStats(), retry: false })
  const packages = useQuery({
    queryKey: ['packages', eco, page],
    queryFn: () => listPackages({ page, page_size: PAGE_SIZE, ecosystem: eco || undefined }),
    retry: false,
  })

  const filtered = (packages.data?.packages ?? []).filter(p =>
    !search || p.name?.toLowerCase().includes(search.toLowerCase())
  )

  const total = packages.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <div style={{ padding: '1.5rem', display: 'flex', flexDirection: 'column', gap: '1.25rem' }}>
      <div>
        <h1 className="text-lg font-bold font-mono" style={{ color: 'var(--fg)' }}>Software Inventory</h1>
        <p className="text-xs mt-0.5" style={{ color: 'var(--color-muted)' }}>
          All known packages and dependencies across your scanned projects
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        {[
          { label: 'Total Packages', value: stats.data?.total_packages, color: 'var(--color-safe)' },
          { label: 'Total Versions', value: stats.data?.total_versions, color: '#60A5FA' },
          { label: 'Total Findings', value: stats.data?.total_findings, color: 'var(--color-warn)' },
          { label: 'Ecosystems', value: stats.data?.ecosystems_covered?.length, color: 'var(--fg)' },
        ].map(card => (
          <div key={card.label} style={{
            background: 'var(--bg-surface)',
            border: '1px solid rgba(255,255,255,0.07)',
            borderRadius: '0.5rem',
            padding: '0.875rem 1rem',
          }}>
            <p className="text-xs" style={{ color: 'var(--color-muted)' }}>{card.label}</p>
            {stats.isLoading
              ? <Skeleton className="h-7 w-12 mt-1" />
              : <p className="text-2xl font-bold font-mono mt-0.5" style={{ color: card.color }}>{card.value ?? '—'}</p>
            }
          </div>
        ))}
      </div>

      {/* Ecosystem breakdown */}
      <div style={{ background: 'var(--bg-surface)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '0.5rem', padding: '1rem' }}>
        <p className="text-xs font-semibold mb-2" style={{ color: 'var(--color-muted)', textTransform: 'uppercase', letterSpacing: '0.08em' }}>
          Ecosystem Coverage
        </p>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.5rem' }}>
          {(stats.data?.ecosystems_covered ?? ECOSYSTEMS).map(e => (
            <button
              key={e}
              onClick={() => setEco(prev => prev === e ? '' : e)}
              className="text-xs font-mono px-2.5 py-1 rounded"
              style={{
                background: eco === e ? 'rgba(0,255,135,0.12)' : 'var(--bg-elevated)',
                color: eco === e ? 'var(--color-safe)' : 'var(--fg)',
                border: eco === e ? '1px solid rgba(0,255,135,0.3)' : '1px solid rgba(255,255,255,0.07)',
                cursor: 'pointer',
              }}
            >
              {e}
            </button>
          ))}
        </div>
      </div>

      {/* Package table */}
      <div style={{ background: 'var(--bg-surface)', border: '1px solid rgba(255,255,255,0.07)', borderRadius: '0.5rem', overflow: 'hidden' }}>
        <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid rgba(255,255,255,0.07)', display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          <Package size={14} style={{ color: 'var(--color-muted)' }} />
          <span className="text-xs font-semibold" style={{ color: 'var(--fg)' }}>{total} packages</span>
          <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <div style={{ position: 'relative' }}>
              <Search size={12} style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', color: 'var(--color-muted)' }} />
              <input
                type="text"
                placeholder="Search packages..."
                value={search}
                onChange={e => setSearch(e.target.value)}
                className="text-xs rounded pl-6 pr-2 py-1"
                style={{ background: 'var(--bg-elevated)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.1)', width: 180, outline: 'none' }}
              />
            </div>
          </div>
        </div>

        {packages.isLoading ? (
          <div style={{ padding: '1rem', display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
            {[...Array(8)].map((_, i) => <Skeleton key={i} className="h-8 w-full" />)}
          </div>
        ) : filtered.length === 0 ? (
          <div style={{ padding: '3rem', textAlign: 'center' }}>
            <p className="text-sm" style={{ color: 'var(--fg)' }}>No packages found</p>
            <p className="text-xs mt-1" style={{ color: 'var(--color-muted)' }}>Scan a project to populate your inventory:</p>
            <code className="text-xs px-3 py-1.5 rounded font-mono mt-2 inline-block"
              style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--color-safe)' }}>
              fgctl scan .
            </code>
          </div>
        ) : (
          <>
            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.8125rem' }}>
                <thead>
                  <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.07)' }}>
                    {['Package', 'Ecosystem', 'Versions'].map(h => (
                      <th key={h} style={{ padding: '0.5rem 0.75rem', textAlign: 'left', color: 'var(--color-muted)', fontWeight: 500, fontSize: '0.75rem' }}>{h}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {filtered.map(pkg => (
                    <tr key={`${pkg.ecosystem}/${pkg.name}`} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                      <td style={{ padding: '0.55rem 0.75rem' }}>
                        <span className="font-mono text-sm" style={{ color: 'var(--fg)' }}>{pkg.name}</span>
                      </td>
                      <td style={{ padding: '0.55rem 0.75rem' }}>
                        <span className="text-xs font-mono px-1.5 py-0.5 rounded"
                          style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--color-muted)' }}>
                          {pkg.ecosystem}
                        </span>
                      </td>
                      <td style={{ padding: '0.55rem 0.75rem' }}>
                        <span className="text-xs" style={{ color: 'var(--color-muted)' }}>
                          {pkg.versions?.length ?? 1} version{(pkg.versions?.length ?? 1) !== 1 ? 's' : ''}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {totalPages > 1 && (
              <div style={{ padding: '0.75rem 1rem', borderTop: '1px solid rgba(255,255,255,0.07)', display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', alignItems: 'center' }}>
                <button
                  onClick={() => setPage(p => Math.max(1, p - 1))}
                  disabled={page === 1}
                  className="text-xs px-2.5 py-1 rounded"
                  style={{ background: 'var(--bg-elevated)', color: page === 1 ? 'var(--color-muted)' : 'var(--fg)', border: '1px solid rgba(255,255,255,0.1)', cursor: page === 1 ? 'default' : 'pointer' }}
                >
                  ← Prev
                </button>
                <span className="text-xs" style={{ color: 'var(--color-muted)' }}>{page} / {totalPages}</span>
                <button
                  onClick={() => setPage(p => Math.min(totalPages, p + 1))}
                  disabled={page === totalPages}
                  className="text-xs px-2.5 py-1 rounded"
                  style={{ background: 'var(--bg-elevated)', color: page === totalPages ? 'var(--color-muted)' : 'var(--fg)', border: '1px solid rgba(255,255,255,0.1)', cursor: page === totalPages ? 'default' : 'pointer' }}
                >
                  Next →
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}

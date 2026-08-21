import { useMemo, useState } from 'react';
import {
  ArrowLeft, FileJson, FileSpreadsheet, FileText,
  Shield, Clock, Target, Layers, GitBranch,
  ChevronRight, ChevronDown, Eye,
} from 'lucide-react';
import {
  PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis,
  Tooltip as RechartsTooltip, ResponsiveContainer,
  LineChart, Line, CartesianGrid, Treemap,
} from 'recharts';
import { useSessionStore, type ScanSession } from '../store/sessions';
import { useUIStore } from '../store/ui';
import { SeverityBadge } from '../components/SeverityBadge';
import { FindingsTable } from '../components/FindingsTable';
import type { Finding } from '../types/api';

const SEV_COLORS: Record<string, string> = {
  CRITICAL: '#DC2626', HIGH: '#EA580C', MEDIUM: '#D97706', LOW: '#06B6D4', INFORMATIONAL: '#6B7280',
};

const SEV_ORDER: Record<string, number> = { CRITICAL: 4, HIGH: 3, MEDIUM: 2, LOW: 1, INFORMATIONAL: 0 };

function formatDate(iso: string): string {
  return new Date(iso).toLocaleString('en-US', {
    month: 'short', day: 'numeric', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  });
}

// Mindmap-style category tree
function FindingsMindmap({ findings }: { findings: Finding[] }) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());

  const tree = useMemo(() => {
    const byType: Record<string, Record<string, Finding[]>> = {};
    for (const f of findings) {
      const type = f.type || 'Unknown';
      const sev = f.severity;
      if (!byType[type]) byType[type] = {};
      if (!byType[type][sev]) byType[type][sev] = [];
      byType[type][sev].push(f);
    }
    return Object.entries(byType)
      .map(([type, sevs]) => ({
        type,
        total: Object.values(sevs).reduce((s, v) => s + v.length, 0),
        severities: Object.entries(sevs)
          .sort(([a], [b]) => (SEV_ORDER[b] ?? 0) - (SEV_ORDER[a] ?? 0))
          .map(([sev, items]) => ({ sev, items })),
      }))
      .sort((a, b) => b.total - a.total);
  }, [findings]);

  const toggle = (type: string) => {
    setExpanded(prev => {
      const next = new Set(prev);
      next.has(type) ? next.delete(type) : next.add(type);
      return next;
    });
  };

  if (tree.length === 0) return null;

  return (
    <div className="rounded-lg border border-border-color bg-surface p-4">
      <h3 className="text-sm font-semibold text-text-primary m-0 mb-3 flex items-center gap-2">
        <GitBranch size={14} className="text-primary-blue" />
        Findings Mindmap
      </h3>
      <div className="relative pl-4">
        <div className="absolute left-[7px] top-0 bottom-0 w-px bg-border-color" />
        {tree.map(node => {
          const isOpen = expanded.has(node.type);
          return (
            <div key={node.type} className="mb-1.5">
              <button
                onClick={() => toggle(node.type)}
                className="flex items-center gap-1.5 px-2 py-1 rounded hover:bg-surface-muted bg-transparent cursor-pointer [font-family:inherit] text-left w-full transition-colors"
              >
                <div className="relative -ml-[calc(1rem+1px)] w-4 h-px bg-border-color" />
                {isOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                <span className="text-xs font-medium text-text-primary">{node.type}</span>
                <span className="text-[0.62rem] text-text-muted font-mono ml-auto">{node.total}</span>
              </button>
              {isOpen && (
                <div className="pl-8 relative">
                  <div className="absolute left-[calc(1.5rem+7px)] top-0 bottom-0 w-px bg-border-color/50" />
                  {node.severities.map(({ sev, items }) => (
                    <div key={sev} className="flex items-start gap-2 py-0.5 relative">
                      <div className="absolute left-[calc(-0.5rem+7px)] top-[0.6rem] w-3 h-px bg-border-color/50" />
                      <SeverityBadge severity={sev as Finding['severity']} />
                      <div className="flex flex-wrap gap-1">
                        {items.slice(0, 5).map((f, i) => (
                          <span key={i} className="text-[0.62rem] text-text-secondary bg-surface-muted px-1.5 py-0.5 rounded truncate max-w-[200px]">
                            {f.title || f.id}
                          </span>
                        ))}
                        {items.length > 5 && (
                          <span className="text-[0.62rem] text-text-muted">+{items.length - 5} more</span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// Treemap chart for vulnerability categories
function CategoryTreemap({ findings }: { findings: Finding[] }) {
  const data = useMemo(() => {
    const bySource: Record<string, number> = {};
    for (const f of findings) {
      const src = f.source || 'unknown';
      bySource[src] = (bySource[src] || 0) + 1;
    }
    return Object.entries(bySource).map(([name, size]) => ({ name, size }));
  }, [findings]);

  if (data.length === 0) return null;

  const COLORS_MAP = ['#2563EB', '#7C3AED', '#059669', '#D97706', '#DC2626', '#EC4899', '#06B6D4', '#8B5CF6'];

  return (
    <div className="rounded-lg border border-border-color bg-surface p-4">
      <h3 className="text-sm font-semibold text-text-primary m-0 mb-3 flex items-center gap-2">
        <Layers size={14} className="text-primary-blue" />
        Source Distribution
      </h3>
      <ResponsiveContainer width="100%" height={200}>
        <Treemap
          data={data}
          dataKey="size"
          nameKey="name"
          stroke="var(--border-color)"
          content={(({ x, y, width, height, name, index }: { x: number; y: number; width: number; height: number; name: string; index: number }) => {
            const w = Number(width) || 0;
            const h = Number(height) || 0;
            if (w < 30 || h < 20) return <g />;
            return (
              <g>
                <rect
                  x={x} y={y} width={w} height={h}
                  fill={COLORS_MAP[(index as number) % COLORS_MAP.length]}
                  rx={4}
                  opacity={0.85}
                />
                {w > 50 && h > 30 && (
                  <text
                    x={Number(x) + w / 2} y={Number(y) + h / 2}
                    textAnchor="middle" dominantBaseline="central"
                    fill="#fff" fontSize={10} fontFamily="monospace"
                  >
                    {String(name)}
                  </text>
                )}
              </g>
            );
          }) as any}
        />
      </ResponsiveContainer>
    </div>
  );
}

// Severity trend across sessions for the same package
function SeverityTrendChart({ sessions, currentId }: { sessions: ScanSession[]; currentId: string }) {
  const data = useMemo(() => {
    return sessions
      .slice()
      .sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())
      .map(s => ({
        date: new Date(s.created_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        critical: s.summary.critical,
        high: s.summary.high,
        medium: s.summary.medium,
        low: s.summary.low,
        total: s.summary.total,
        isCurrent: s.id === currentId,
      }));
  }, [sessions, currentId]);

  if (data.length < 2) return null;

  return (
    <div className="rounded-lg border border-border-color bg-surface p-4">
      <h3 className="text-sm font-semibold text-text-primary m-0 mb-3 flex items-center gap-2">
        <Target size={14} className="text-primary-blue" />
        Severity Trend (Same Package)
      </h3>
      <ResponsiveContainer width="100%" height={220}>
        <LineChart data={data}>
          <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
          <XAxis dataKey="date" tick={{ fontSize: 10, fill: 'var(--text-muted)' }} />
          <YAxis tick={{ fontSize: 10, fill: 'var(--text-muted)' }} />
          <RechartsTooltip
            contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
          />
          <Line type="monotone" dataKey="critical" stroke={SEV_COLORS.CRITICAL} strokeWidth={2} dot={{ r: 3 }} />
          <Line type="monotone" dataKey="high" stroke={SEV_COLORS.HIGH} strokeWidth={2} dot={{ r: 3 }} />
          <Line type="monotone" dataKey="medium" stroke={SEV_COLORS.MEDIUM} strokeWidth={1.5} dot={{ r: 2 }} />
          <Line type="monotone" dataKey="low" stroke={SEV_COLORS.LOW} strokeWidth={1.5} dot={{ r: 2 }} />
        </LineChart>
      </ResponsiveContainer>
      <div className="flex items-center gap-4 mt-2 justify-center">
        {Object.entries(SEV_COLORS).filter(([k]) => k !== 'INFORMATIONAL').map(([label, color]) => (
          <div key={label} className="flex items-center gap-1">
            <div className="w-2.5 h-2.5 rounded-full" style={{ background: color }} />
            <span className="text-[0.6rem] text-text-muted capitalize">{label.toLowerCase()}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function exportSessionJSON(session: ScanSession) {
  const blob = new Blob([JSON.stringify({
    id: session.id,
    scan_type: session.scan_type,
    label: session.label,
    ecosystem: session.ecosystem,
    package_name: session.package_name,
    version: session.version,
    summary: session.summary,
    findings: session.findings,
    result: session.result,
    project_result: session.project_result,
    created_at: session.created_at,
  }, null, 2)], { type: 'application/json' });
  downloadBlob(blob, `scan-${session.id}.json`);
}

function csvCell(s: string): string {
  const escaped = s.replace(/"/g, '""');
  return `"${escaped}"`;
}

function exportSessionCSV(session: ScanSession) {
  const header = 'ID,Severity,Type,Title,Source,Fixed Version,Description\n';
  const rows = session.findings.map(f =>
    [csvCell(f.id), csvCell(f.severity), csvCell(f.type || ''), csvCell(f.title || ''), csvCell(f.source || ''), csvCell(f.fixed_version || ''), csvCell((f.description || '').slice(0, 200))].join(',')
  ).join('\n');
  const blob = new Blob([header + rows], { type: 'text/csv' });
  downloadBlob(blob, `scan-${session.id}.csv`);
}

function esc(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

function exportSessionHTML(session: ScanSession) {
  const sevCounts = [
    { label: 'Critical', count: session.summary.critical, color: SEV_COLORS.CRITICAL },
    { label: 'High', count: session.summary.high, color: SEV_COLORS.HIGH },
    { label: 'Medium', count: session.summary.medium, color: SEV_COLORS.MEDIUM },
    { label: 'Low', count: session.summary.low, color: SEV_COLORS.LOW },
  ];
  const findingsRows = session.findings
    .sort((a, b) => (SEV_ORDER[b.severity] ?? 0) - (SEV_ORDER[a.severity] ?? 0))
    .map(f => `<tr>
      <td><span style="color:${SEV_COLORS[f.severity] || '#6B7280'};font-weight:600">${esc(f.severity)}</span></td>
      <td><code>${esc(f.id)}</code></td>
      <td>${esc(f.title || '')}</td>
      <td>${esc(f.source || '')}</td>
      <td>${esc(f.fixed_version || '—')}</td>
    </tr>`).join('\n');

  const html = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><title>ForgeGuardian Scan Report — ${esc(session.label)}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0B0D0F;color:#E5E7EB;padding:2rem}
h1{font-size:1.4rem;margin-bottom:0.5rem}
.meta{color:#9CA3AF;font-size:0.8rem;margin-bottom:1.5rem}
.cards{display:grid;grid-template-columns:repeat(4,1fr);gap:0.75rem;margin-bottom:2rem}
.card{background:#161A1E;border:1px solid #2A2F36;border-radius:8px;padding:1rem}
.card-label{font-size:0.65rem;text-transform:uppercase;color:#6B7280;letter-spacing:0.05em}
.card-value{font-size:1.5rem;font-weight:700;font-family:monospace;margin-top:0.25rem}
table{width:100%;border-collapse:collapse;font-size:0.8rem}
th{text-align:left;padding:0.5rem 0.75rem;border-bottom:1px solid #2A2F36;color:#6B7280;font-size:0.7rem;text-transform:uppercase}
td{padding:0.5rem 0.75rem;border-bottom:1px solid #1F2328}
code{background:#1F2328;padding:0.15rem 0.4rem;border-radius:4px;font-size:0.72rem}
.footer{margin-top:2rem;text-align:center;font-size:0.7rem;color:#4B5563}
</style></head><body>
<h1>ForgeGuardian Scan Report</h1>
<p class="meta">${esc(session.label)} &middot; ${esc(session.ecosystem || '')} &middot; ${formatDate(session.created_at)}</p>
<div class="cards">
${sevCounts.map(s => `<div class="card"><div class="card-label">${s.label}</div><div class="card-value" style="color:${s.color}">${s.count}</div></div>`).join('\n')}
</div>
<table>
<thead><tr><th>Severity</th><th>ID</th><th>Title</th><th>Source</th><th>Fix</th></tr></thead>
<tbody>${findingsRows}</tbody>
</table>
<div class="footer">Generated by ForgeGuardian &middot; ${new Date().toISOString()}</div>
</body></html>`;

  const blob = new Blob([html], { type: 'text/html' });
  downloadBlob(blob, `scan-report-${session.id}.html`);
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

export default function SessionDetailPage({ sessionId }: { sessionId: string }) {
  const navigate = useUIStore(s => s.navigate);
  const session = useSessionStore(s => s.get(sessionId));
  const allSessions = useSessionStore(s => s.sessions);

  const [activeTab, setActiveTab] = useState<'overview' | 'findings' | 'mindmap'>('overview');

  const relatedSessions = useMemo(() => {
    if (!session?.package_name) return [];
    return allSessions.filter(
      s => s.package_name === session.package_name && s.ecosystem === session.ecosystem
    );
  }, [allSessions, session]);

  if (!session) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-text-muted">
        <Shield size={32} className="mb-3 opacity-40" />
        <p className="text-sm font-medium">Session not found</p>
        <button
          onClick={() => navigate('/sessions')}
          className="mt-2 text-xs text-primary-blue underline bg-transparent border-none cursor-pointer [font-family:inherit]"
        >
          Back to sessions
        </button>
      </div>
    );
  }

  const { summary, findings } = session;

  const sevPieData = [
    { name: 'Critical', value: summary.critical, fill: SEV_COLORS.CRITICAL },
    { name: 'High', value: summary.high, fill: SEV_COLORS.HIGH },
    { name: 'Medium', value: summary.medium, fill: SEV_COLORS.MEDIUM },
    { name: 'Low', value: summary.low, fill: SEV_COLORS.LOW },
  ].filter(d => d.value > 0);

  const engineData = useMemo(() => {
    const byEngine: Record<string, number> = {};
    for (const f of findings) {
      const engine = f.source || 'unknown';
      byEngine[engine] = (byEngine[engine] || 0) + 1;
    }
    return Object.entries(byEngine)
      .map(([engine, count]) => ({ engine, count }))
      .sort((a, b) => b.count - a.count);
  }, [findings]);

  const categoryData = useMemo(() => {
    const byType: Record<string, number> = {};
    for (const f of findings) {
      const type = f.type || 'Unknown';
      byType[type] = (byType[type] || 0) + 1;
    }
    return Object.entries(byType)
      .map(([type, count]) => ({ type, count }))
      .sort((a, b) => b.count - a.count)
      .slice(0, 10);
  }, [findings]);

  const fixableCount = findings.filter(f => f.fixed_version).length;
  const uniqueSources = new Set(findings.map(f => f.source)).size;

  return (
    <div className="p-6 max-w-[1400px] mx-auto">
      {/* Header */}
      <div className="flex items-center gap-3 mb-1">
        <button
          onClick={() => navigate('/sessions')}
          className="p-1.5 rounded-md hover:bg-surface-muted bg-transparent cursor-pointer text-text-muted"
        >
          <ArrowLeft size={16} />
        </button>
        <div className="flex-1 min-w-0">
          <h1 className="text-lg font-bold text-text-primary m-0 truncate">{session.label}</h1>
          <div className="flex items-center gap-3 text-xs text-text-muted mt-0.5">
            <span className="flex items-center gap-1">
              <Clock size={11} />
              {formatDate(session.created_at)}
            </span>
            <span className={`px-1.5 py-0.5 rounded text-[0.62rem] font-medium uppercase ${
              session.scan_type === 'registry' ? 'bg-primary-blue/10 text-primary-blue' :
              session.scan_type === 'upload' ? 'bg-[#7C3AED]/10 text-[#7C3AED]' :
              'bg-[#059669]/10 text-[#059669]'
            }`}>
              {session.scan_type}
            </span>
            {session.ecosystem && <span className="font-mono">{session.ecosystem}</span>}
            {session.version && <span className="font-mono">v{session.version}</span>}
          </div>
        </div>

        {/* Export buttons */}
        <div className="flex items-center gap-1.5">
          <button
            onClick={() => exportSessionJSON(session)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-md border border-border-color text-[0.72rem] text-text-secondary hover:text-primary-blue hover:border-primary-blue/40 bg-transparent cursor-pointer [font-family:inherit] transition-colors"
            title="Export JSON"
          >
            <FileJson size={13} /> JSON
          </button>
          <button
            onClick={() => exportSessionCSV(session)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-md border border-border-color text-[0.72rem] text-text-secondary hover:text-success hover:border-success/40 bg-transparent cursor-pointer [font-family:inherit] transition-colors"
            title="Export CSV"
          >
            <FileSpreadsheet size={13} /> CSV
          </button>
          <button
            onClick={() => exportSessionHTML(session)}
            className="flex items-center gap-1 px-2.5 py-1.5 rounded-md border border-border-color text-[0.72rem] text-text-secondary hover:text-[#7C3AED] hover:border-[#7C3AED]/40 bg-transparent cursor-pointer [font-family:inherit] transition-colors"
            title="Export HTML Report"
          >
            <FileText size={13} /> Report
          </button>
        </div>
      </div>

      {/* Severity cards */}
      <div className="grid grid-cols-5 gap-3 mt-5 mb-5">
        {[
          { label: 'Critical', value: summary.critical, color: SEV_COLORS.CRITICAL },
          { label: 'High', value: summary.high, color: SEV_COLORS.HIGH },
          { label: 'Medium', value: summary.medium, color: SEV_COLORS.MEDIUM },
          { label: 'Low', value: summary.low, color: SEV_COLORS.LOW },
          { label: 'Total', value: summary.total, color: 'var(--text-primary)' },
        ].map(s => (
          <div key={s.label} className="rounded-lg border border-border-color bg-surface p-3">
            <p className="text-[0.62rem] text-text-muted uppercase tracking-wider m-0">{s.label}</p>
            <p className="text-2xl font-bold font-mono m-0 mt-0.5" style={{ color: s.color }}>{s.value}</p>
          </div>
        ))}
      </div>

      {/* Metrics strip */}
      <div className="grid grid-cols-3 gap-3 mb-5">
        <div className="rounded-lg border border-border-color bg-surface p-3 flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-success/10 flex items-center justify-center">
            <Shield size={14} className="text-success" />
          </div>
          <div>
            <p className="text-[0.62rem] text-text-muted uppercase m-0">Fixable</p>
            <p className="text-sm font-bold text-text-primary m-0">{fixableCount} / {findings.length}</p>
          </div>
        </div>
        <div className="rounded-lg border border-border-color bg-surface p-3 flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-primary-blue/10 flex items-center justify-center">
            <Layers size={14} className="text-primary-blue" />
          </div>
          <div>
            <p className="text-[0.62rem] text-text-muted uppercase m-0">Engines</p>
            <p className="text-sm font-bold text-text-primary m-0">{uniqueSources}</p>
          </div>
        </div>
        <div className="rounded-lg border border-border-color bg-surface p-3 flex items-center gap-3">
          <div className="w-8 h-8 rounded-full bg-[#7C3AED]/10 flex items-center justify-center">
            <Eye size={14} className="text-[#7C3AED]" />
          </div>
          <div>
            <p className="text-[0.62rem] text-text-muted uppercase m-0">Categories</p>
            <p className="text-sm font-bold text-text-primary m-0">{categoryData.length}</p>
          </div>
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex items-center gap-1 mb-5 border-b border-border-color">
        {(['overview', 'findings', 'mindmap'] as const).map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            className={`px-4 py-2 text-[0.78rem] font-medium border-b-2 -mb-px bg-transparent cursor-pointer [font-family:inherit] transition-colors ${
              activeTab === tab
                ? 'text-primary-blue border-primary-blue'
                : 'text-text-secondary border-transparent hover:text-text-primary'
            }`}
          >
            {tab === 'overview' ? 'Overview' : tab === 'findings' ? `Findings (${findings.length})` : 'Mindmap'}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {/* Severity distribution donut */}
          {sevPieData.length > 0 && (
            <div className="rounded-lg border border-border-color bg-surface p-4">
              <h3 className="text-sm font-semibold text-text-primary m-0 mb-3">Severity Distribution</h3>
              <ResponsiveContainer width="100%" height={220}>
                <PieChart>
                  <Pie
                    data={sevPieData}
                    cx="50%"
                    cy="50%"
                    innerRadius={55}
                    outerRadius={85}
                    dataKey="value"
                    paddingAngle={2}
                    stroke="none"
                  >
                    {sevPieData.map((d, i) => (
                      <Cell key={i} fill={d.fill} />
                    ))}
                  </Pie>
                  <RechartsTooltip
                    contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                  />
                </PieChart>
              </ResponsiveContainer>
              <div className="flex items-center justify-center gap-4 mt-1">
                {sevPieData.map(d => (
                  <div key={d.name} className="flex items-center gap-1.5">
                    <div className="w-2.5 h-2.5 rounded-full" style={{ background: d.fill }} />
                    <span className="text-[0.65rem] text-text-muted">{d.name}: {d.value}</span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Findings by Engine bar chart */}
          {engineData.length > 0 && (
            <div className="rounded-lg border border-border-color bg-surface p-4">
              <h3 className="text-sm font-semibold text-text-primary m-0 mb-3">Findings by Engine</h3>
              <ResponsiveContainer width="100%" height={220}>
                <BarChart data={engineData} layout="vertical" margin={{ left: 10, right: 20 }}>
                  <XAxis type="number" tick={{ fontSize: 10, fill: 'var(--text-muted)' }} />
                  <YAxis type="category" dataKey="engine" tick={{ fontSize: 10, fill: 'var(--text-secondary)' }} width={80} />
                  <RechartsTooltip
                    contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                  />
                  <Bar dataKey="count" fill="#2563EB" radius={[0, 4, 4, 0]} barSize={16} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* Findings by Category bar chart */}
          {categoryData.length > 0 && (
            <div className="rounded-lg border border-border-color bg-surface p-4">
              <h3 className="text-sm font-semibold text-text-primary m-0 mb-3">Findings by Category</h3>
              <ResponsiveContainer width="100%" height={220}>
                <BarChart data={categoryData} margin={{ left: 10, right: 20 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border-color)" />
                  <XAxis dataKey="type" tick={{ fontSize: 9, fill: 'var(--text-muted)' }} interval={0} angle={-30} textAnchor="end" height={50} />
                  <YAxis tick={{ fontSize: 10, fill: 'var(--text-muted)' }} />
                  <RechartsTooltip
                    contentStyle={{ background: 'var(--surface)', border: '1px solid var(--border-color)', borderRadius: 8, fontSize: 11 }}
                  />
                  <Bar dataKey="count" fill="#7C3AED" radius={[4, 4, 0, 0]} barSize={24} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* Source distribution treemap */}
          <CategoryTreemap findings={findings} />

          {/* Severity trend */}
          <div className="col-span-1 lg:col-span-2">
            <SeverityTrendChart sessions={relatedSessions} currentId={session.id} />
          </div>
        </div>
      )}

      {activeTab === 'findings' && (
        <div>
          {findings.length > 0 ? (
            <FindingsTable findings={findings} />
          ) : (
            <div className="flex flex-col items-center justify-center py-16 text-text-muted">
              <Shield size={28} className="mb-2 text-success opacity-60" />
              <p className="text-sm font-medium">No findings detected</p>
            </div>
          )}
        </div>
      )}

      {activeTab === 'mindmap' && (
        <div className="grid grid-cols-1 gap-4">
          <FindingsMindmap findings={findings} />

          {/* Engine effectiveness */}
          {session.result?.engines && session.result.engines.length > 0 && (
            <div className="rounded-lg border border-border-color bg-surface p-4">
              <h3 className="text-sm font-semibold text-text-primary m-0 mb-3 flex items-center gap-2">
                <Target size={14} className="text-primary-blue" />
                Engine Effectiveness
              </h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                {session.result.engines.map(eng => (
                  <div key={eng.engine} className="rounded-md border border-border-color p-3">
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-xs font-medium text-text-primary">{eng.engine}</span>
                      <span className={`text-[0.6rem] px-1 py-0.5 rounded ${
                        eng.status === 'ok' ? 'bg-success/10 text-success' : 'bg-surface-muted text-text-muted'
                      }`}>
                        {eng.status}
                      </span>
                    </div>
                    <p className="text-lg font-bold font-mono text-text-primary m-0">{eng.findings}</p>
                    <p className="text-[0.6rem] text-text-muted m-0">findings</p>
                    {eng.error && (
                      <p className="text-[0.6rem] text-critical mt-1 m-0 truncate">{eng.error}</p>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

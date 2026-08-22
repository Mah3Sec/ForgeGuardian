import { useState, useMemo, useRef, useEffect } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  PieChart, Pie, Cell, BarChart, Bar, XAxis, YAxis,
  Tooltip as RechartsTooltip, ResponsiveContainer,
} from 'recharts';
import { triggerScan, scanUpload, getJobStatus, triggerRemoteScan, getRemoteScanJobStatus, getRecentResults } from '../lib/api';
import { SeverityBadge } from '../components/SeverityBadge';
import { FindingsTable } from '../components/FindingsTable';
import { Search, Upload, AlertCircle, ShieldCheck, CheckCircle, XCircle, Loader, Server, Lock, Clock, FolderOpen, Package } from 'lucide-react';
import type { Finding, EngineStatus, ScanSummary } from '../types/api';
import { useSessionStore } from '../store/sessions';
import { useWorkspaceStore } from '../store/workspace';

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'];

// Mirrors the Go backend's internal/policy/policy.go severityOrd ordering.
const SEVERITY_ORDER: Record<string, number> = {
  CRITICAL: 4,
  HIGH: 3,
  MEDIUM: 2,
  LOW: 1,
  INFORMATIONAL: 0,
};

type SeverityFilter = 'all' | 'critical' | 'high' | 'medium';

const SEVERITY_FILTER_THRESHOLD: Record<SeverityFilter, number> = {
  all: -1,
  critical: SEVERITY_ORDER.CRITICAL,
  high: SEVERITY_ORDER.HIGH,
  medium: SEVERITY_ORDER.MEDIUM,
};

const inputStyle = {
  background: 'var(--bg-base)',
  color: 'var(--fg)',
  border: '1px solid rgba(255,255,255,0.12)',
  borderRadius: '0.375rem',
  padding: '0.5rem 0.75rem',
  fontSize: '0.875rem',
  fontFamily: 'var(--font-mono)',
  width: '100%',
  outline: 'none',
} as React.CSSProperties;

function EngineStatusBar({ engines }: { engines: EngineStatus[] }) {
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.375rem', marginTop: '0.5rem' }}>
      {engines.map(e => (
        <div key={e.engine} style={{
          display: 'flex', alignItems: 'center', gap: 4,
          padding: '0.2rem 0.5rem',
          borderRadius: '0.25rem',
          fontSize: '0.68rem',
          fontFamily: 'var(--font-mono)',
          background: e.status === 'ok' ? 'rgba(0,255,135,0.08)' : 'rgba(255,255,255,0.04)',
          border: `1px solid ${e.status === 'ok' ? 'rgba(0,255,135,0.2)' : 'rgba(255,255,255,0.08)'}`,
          color: e.status === 'ok' ? 'var(--color-safe)' : 'var(--color-muted)',
        }}>
          {e.status === 'ok'
            ? <CheckCircle size={10} />
            : <XCircle size={10} />}
          {e.engine}
          {e.findings > 0 && (
            <span style={{ color: 'var(--color-warn)', fontWeight: 700 }}>({e.findings})</span>
          )}
        </div>
      ))}
    </div>
  );
}

function ScanHistory({ onRescan }: { onRescan: (eco: string, name: string, ver: string) => void }) {
  const recent = useQuery({
    queryKey: ['recent-scans'],
    queryFn: () => getRecentResults(10),
    retry: false,
  });

  const results = recent.data?.results ?? [];
  const sevColor: Record<string, string> = {
    CRITICAL: 'var(--color-critical)', HIGH: 'var(--color-high)',
    MEDIUM: 'var(--color-medium)', LOW: '#60A5FA',
  };

  return (
    <div className="space-y-4">
      <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', padding: '2rem 0', gap: '1rem', textAlign: 'center' }}>
        <ShieldCheck size={40} style={{ color: 'var(--color-muted)' }} />
        <div>
          <p style={{ fontSize: '0.9rem', color: 'var(--fg)', fontWeight: 600 }}>No scan results yet</p>
          <p style={{ fontSize: '0.78rem', color: 'var(--color-muted)', marginTop: 4 }}>
            Pick a registry package above, or upload an archive — then hit Run Full Scan.
          </p>
        </div>
      </div>

      {results.length > 0 && (
        <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', gap: 8 }}>
            <Clock size={14} style={{ color: 'var(--color-muted)' }} />
            <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)' }}>
              RECENT SCANS
            </span>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full text-sm border-collapse">
              <thead>
                <tr style={{ borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
                  {['Package', 'Version', 'Ecosystem', 'Severity', 'Findings', 'Scanned'].map(h => (
                    <th key={h} className="text-left py-2 px-3 font-mono text-xs uppercase" style={{ color: 'var(--color-muted)' }}>{h}</th>
                  ))}
                  <th />
                </tr>
              </thead>
              <tbody>
                {results.map((r, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }} className="hover:bg-white/[0.02]">
                    <td className="py-2 px-3 font-mono" style={{ color: 'var(--fg)' }}>{r.package}</td>
                    <td className="py-2 px-3 font-mono text-xs" style={{ color: 'var(--color-muted)' }}>{r.version}</td>
                    <td className="py-2 px-3 text-xs uppercase" style={{ color: 'var(--color-muted)' }}>{r.ecosystem}</td>
                    <td className="py-2 px-3"><SeverityBadge severity={r.severity} /></td>
                    <td className="py-2 px-3 font-mono" style={{ color: sevColor[r.severity] ?? 'var(--fg)' }}>{r.findings_count}</td>
                    <td className="py-2 px-3 text-xs" style={{ color: 'var(--color-muted)' }}>
                      {new Date(r.scanned_at).toLocaleDateString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                    </td>
                    <td className="py-2 px-3">
                      <button
                        onClick={() => onRescan(r.ecosystem, r.package, r.version)}
                        style={{
                          fontSize: '0.68rem', fontFamily: 'var(--font-mono)',
                          padding: '0.2rem 0.5rem', borderRadius: '0.25rem',
                          background: 'rgba(99,102,241,0.15)', border: '1px solid rgba(99,102,241,0.3)',
                          color: 'var(--color-indigo)', cursor: 'pointer',
                        }}
                      >
                        Rescan
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

export function ScanPage() {
  const queryClient = useQueryClient();
  const [tab, setTab] = useState<'registry' | 'upload' | 'remote'>('registry');

  // Registry scan state
  const [ecosystem, setEcosystem] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [version, setVersion] = useState('');

  // Upload scan state
  const [dragOver, setDragOver] = useState(false);
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  // Remote (SSH) scan state — privateKey is held only in component state,
  // sent once per request, never written anywhere persistent on this side.
  const [remoteTarget, setRemoteTarget] = useState('');
  const [remotePort, setRemotePort] = useState('22');
  const [remotePrivateKey, setRemotePrivateKey] = useState('');
  const [remotePath, setRemotePath] = useState('');
  const [acceptNewHostKey, setAcceptNewHostKey] = useState(false);
  const [remoteJobId, setRemoteJobId] = useState<string | null>(null);

  const [grouped, setGrouped] = useState(true);
  const [jobId, setJobId] = useState<string | null>(null);
  const [uploadJobId, setUploadJobId] = useState<string | null>(null);

  // Client-side result filters — purely narrowing what's already in `result`.
  const [severityFilter, setSeverityFilter] = useState<SeverityFilter>('all');
  const [onlyFixable, setOnlyFixable] = useState(false);

  // Submitting a scan returns a job immediately; poll until it settles.
  const registryScan = useMutation({
    mutationFn: () => triggerScan(ecosystem, pkg, version),
    onSuccess: (job) => setJobId(job.job_id),
  });

  // Upload's result is a whole-project ProjectScanResult (localscanner walks
  // the archive's real manifests), same shape as a remote scan's result —
  // so it gets its own job id + poll using the same getRemoteScanJobStatus
  // path rather than sharing registryScan's single-package jobPoll.
  const uploadScan = useMutation({
    mutationFn: () => scanUpload(uploadFile!, uploadFile!.name),
    onSuccess: (job) => setUploadJobId(job.job_id),
  });

  const remoteScan = useMutation({
    mutationFn: () => {
      if (!remoteTarget.includes('@')) {
        throw new Error('Target must be in user@host format (e.g. deploy@10.0.4.12)');
      }
      return triggerRemoteScan({
        target: remoteTarget,
        privateKey: remotePrivateKey,
        port: remotePort ? Number(remotePort) : undefined,
        remotePath: remotePath || undefined,
        acceptNewHostKey,
      });
    },
    onSuccess: (job) => setRemoteJobId(job.job_id),
  });

  const jobPoll = useQuery({
    queryKey: ['scan-job', jobId],
    queryFn: () => getJobStatus(jobId!),
    enabled: !!jobId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'complete' || status === 'failed' ? false : 2_000;
    },
  });

  const uploadJobPoll = useQuery({
    queryKey: ['upload-scan-job', uploadJobId],
    queryFn: () => getRemoteScanJobStatus(uploadJobId!),
    enabled: !!uploadJobId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'complete' || status === 'failed' ? false : 2_000;
    },
  });

  const remoteJobPoll = useQuery({
    queryKey: ['remote-scan-job', remoteJobId],
    queryFn: () => getRemoteScanJobStatus(remoteJobId!),
    enabled: !!remoteJobId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'complete' || status === 'failed' ? false : 2_000;
    },
  });

  const job = jobPoll.data;
  const result = job?.status === 'complete' ? job.result ?? null : null;

  const uploadJob = uploadJobPoll.data;
  const uploadResult = uploadJob?.status === 'complete' ? uploadJob.result ?? null : null;
  const uploadIsPending = uploadScan.isPending
    || (!!uploadJobId && uploadJob?.status !== 'complete' && uploadJob?.status !== 'failed');
  const uploadError = uploadScan.error
    || (uploadJob?.status === 'failed' ? new Error(uploadJob.error || 'upload scan failed') : undefined);

  const remoteJob = remoteJobPoll.data;
  const remoteResult = remoteJob?.status === 'complete' ? remoteJob.result ?? null : null;
  const remoteIsPending = remoteScan.isPending
    || (!!remoteJobId && remoteJob?.status !== 'complete' && remoteJob?.status !== 'failed');
  const remoteError = remoteScan.error
    || (remoteJob?.status === 'failed' ? new Error(remoteJob.error || 'remote scan failed') : undefined);

  // Derived, filtered view of result.findings — never mutates `result` itself.
  const filteredFindings = useMemo(() => {
    if (!result) return [];
    const threshold = SEVERITY_FILTER_THRESHOLD[severityFilter];
    return result.findings.filter(f => {
      if ((SEVERITY_ORDER[f.severity] ?? 0) < threshold) return false;
      if (onlyFixable && !f.fixed_version) return false;
      return true;
    });
  }, [result, severityFilter, onlyFixable]);

  const groupedFindings = useMemo(() => {
    if (!grouped || !result) return null;
    const map = new Map<string, Finding[]>();
    for (const f of filteredFindings) {
      const key = f.source || 'unknown';
      if (!map.has(key)) map.set(key, []);
      map.get(key)!.push(f);
    }
    return map;
  }, [filteredFindings, grouped, result]);

  const isPending = registryScan.isPending
    || (!!jobId && job?.status !== 'complete' && job?.status !== 'failed');
  const error = registryScan.error
    || (job?.status === 'failed' ? new Error(job.error || 'scan failed') : undefined);

  // Auto-save scan results as sessions
  const saveSession = useSessionStore(s => s.save);
  const activeWorkspaceId = useWorkspaceStore(s => s.activeId);
  const savedRef = useRef<Set<string>>(new Set());

  const invalidateDashboard = () => {
    queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] });
    queryClient.invalidateQueries({ queryKey: ['dashboard-timeline'] });
    queryClient.invalidateQueries({ queryKey: ['monitor-stats'] });
    queryClient.invalidateQueries({ queryKey: ['risks'] });
    queryClient.invalidateQueries({ queryKey: ['recent-results'] });
  };

  useEffect(() => {
    if (result && jobId && !savedRef.current.has(jobId)) {
      savedRef.current.add(jobId);
      const emptySummary: ScanSummary = { critical: 0, high: 0, medium: 0, low: 0, informational: 0, total: 0, highest_sev: 'LOW' };
      saveSession({
        workspace_id: activeWorkspaceId,
        scan_type: 'registry',
        label: `${ecosystem}/${pkg}@${version || 'latest'}`,
        ecosystem,
        package_name: pkg,
        version: version || undefined,
        result,
        project_result: null,
        summary: result.summary || emptySummary,
        findings: result.findings || [],
      });
      invalidateDashboard();
    }
  }, [result, jobId, ecosystem, pkg, version, saveSession, activeWorkspaceId]);

  useEffect(() => {
    if (uploadResult && uploadJobId && !savedRef.current.has(uploadJobId)) {
      savedRef.current.add(uploadJobId);
      saveSession({
        workspace_id: activeWorkspaceId,
        scan_type: 'upload',
        label: uploadFile?.name || 'Uploaded archive',
        result: null,
        project_result: uploadResult,
        summary: uploadResult.summary,
        findings: uploadResult.results?.flatMap(r => r.findings || []) || [],
      });
      invalidateDashboard();
    }
  }, [uploadResult, uploadJobId, uploadFile, saveSession, activeWorkspaceId]);

  useEffect(() => {
    if (remoteResult && remoteJobId && !savedRef.current.has(remoteJobId)) {
      savedRef.current.add(remoteJobId);
      saveSession({
        workspace_id: activeWorkspaceId,
        scan_type: 'remote',
        label: remoteTarget || 'Remote scan',
        result: null,
        project_result: remoteResult,
        summary: remoteResult.summary,
        findings: remoteResult.results?.flatMap(r => r.findings || []) || [],
      });
      invalidateDashboard();
    }
  }, [remoteResult, remoteJobId, remoteTarget, saveSession, activeWorkspaceId]);

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const f = e.dataTransfer.files[0];
    if (f) setUploadFile(f);
  };

  const tabStyle = (active: boolean) => ({
    padding: '0.5rem 1.25rem',
    fontSize: '0.8rem',
    fontFamily: 'var(--font-mono)',
    fontWeight: 600,
    borderRadius: '0.375rem 0.375rem 0 0',
    border: 'none',
    cursor: 'pointer',
    background: active ? 'var(--surface)' : 'transparent',
    color: active ? 'var(--fg)' : 'var(--color-muted)',
    borderBottom: active ? '2px solid var(--color-safe)' : '2px solid transparent',
  } as React.CSSProperties);

  return (
    <div className="p-6 space-y-6 max-w-4xl">
      <div>
        <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Vulnerability Scanner</h1>
        <p className="text-sm mt-1" style={{ color: 'var(--color-muted)' }}>
          Full 8-engine scan — OSV · Grype · Semgrep · Trivy · Behavioral · Malware · AI-Model · MCP
        </p>
      </div>

      {/* Tabs */}
      <div style={{ borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', gap: '0.25rem' }}>
        <button style={tabStyle(tab === 'registry')} onClick={() => setTab('registry')}>
          <Search size={12} style={{ display: 'inline', marginRight: 6 }} />
          Registry Package
        </button>
        <button style={tabStyle(tab === 'upload')} onClick={() => setTab('upload')}>
          <Upload size={12} style={{ display: 'inline', marginRight: 6 }} />
          Upload Project
        </button>
        <button style={tabStyle(tab === 'remote')} onClick={() => setTab('remote')}>
          <Server size={12} style={{ display: 'inline', marginRight: 6 }} />
          Remote Host
        </button>
      </div>

      {/* Registry scan form */}
      {tab === 'registry' && (
        <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
            Downloads the package from its registry and runs all engines against the real files.
          </p>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div>
              <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>ECOSYSTEM</label>
              <select value={ecosystem} onChange={e => setEcosystem(e.target.value)} style={inputStyle}>
                {ECOSYSTEMS.map(e => <option key={e} value={e}>{e}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>PACKAGE</label>
              <input
                value={pkg}
                onChange={e => setPkg(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && pkg && version && !isPending && registryScan.mutate()}
                placeholder="e.g. lodash"
                style={inputStyle}
              />
            </div>
            <div>
              <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>VERSION</label>
              <input
                value={version}
                onChange={e => setVersion(e.target.value)}
                onKeyDown={e => e.key === 'Enter' && pkg && version && !isPending && registryScan.mutate()}
                placeholder="e.g. 4.17.21"
                style={inputStyle}
              />
            </div>
          </div>
          <button
            onClick={() => registryScan.mutate()}
            disabled={!pkg || !version || isPending}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '0.5rem 1.25rem', borderRadius: '0.375rem',
              background: isPending ? 'rgba(0,255,135,0.4)' : 'var(--color-safe)',
              color: '#0A0B0D', border: 'none', cursor: isPending ? 'not-allowed' : 'pointer',
              fontSize: '0.8rem', fontFamily: 'var(--font-mono)', fontWeight: 700,
            }}
          >
            {isPending ? <Loader size={14} className="animate-spin" /> : <Search size={14} />}
            {isPending ? 'Scanning…' : 'Run Full Scan'}
          </button>
        </div>
      )}

      {/* Upload form */}
      {tab === 'upload' && (
        <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
            Upload a project archive (.tar.gz, .zip) or a single file. All engines run against the extracted contents.
          </p>

          {/* Drop zone */}
          <div
            onDragOver={e => { e.preventDefault(); setDragOver(true); }}
            onDragLeave={() => setDragOver(false)}
            onDrop={handleDrop}
            onClick={() => fileRef.current?.click()}
            style={{
              border: `2px dashed ${dragOver ? 'var(--color-safe)' : 'rgba(255,255,255,0.12)'}`,
              borderRadius: '0.5rem',
              padding: '2rem',
              textAlign: 'center',
              cursor: 'pointer',
              background: dragOver ? 'rgba(0,255,135,0.04)' : 'transparent',
              transition: 'all 0.15s',
            }}
          >
            <input
              ref={fileRef}
              type="file"
              accept=".tar.gz,.tgz,.zip,.whl,.gem,.jar,.tar"
              style={{ display: 'none' }}
              onChange={e => { if (e.target.files?.[0]) setUploadFile(e.target.files[0]); }}
            />
            <Upload size={28} style={{ color: 'var(--color-muted)', margin: '0 auto 0.75rem' }} />
            {uploadFile ? (
              <div>
                <p style={{ fontSize: '0.85rem', color: 'var(--color-safe)', fontFamily: 'var(--font-mono)', fontWeight: 600 }}>
                  {uploadFile.name}
                </p>
                <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginTop: 4 }}>
                  {(uploadFile.size / 1024 / 1024).toFixed(2)} MB — click to change
                </p>
              </div>
            ) : (
              <div>
                <p style={{ fontSize: '0.85rem', color: 'var(--fg)' }}>Drop archive here or click to browse</p>
                <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginTop: 4 }}>
                  .tar.gz · .tgz · .zip · .jar · .gem · .whl (max 512 MB)
                </p>
              </div>
            )}
          </div>

          <button
            onClick={() => uploadScan.mutate()}
            disabled={!uploadFile || uploadIsPending}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '0.5rem 1.25rem', borderRadius: '0.375rem',
              background: uploadIsPending ? 'rgba(0,255,135,0.4)' : 'var(--color-safe)',
              color: '#0A0B0D', border: 'none', cursor: uploadIsPending ? 'not-allowed' : 'pointer',
              fontSize: '0.8rem', fontFamily: 'var(--font-mono)', fontWeight: 700,
            }}
          >
            {uploadIsPending ? <Loader size={14} className="animate-spin" /> : <Upload size={14} />}
            {uploadIsPending ? 'Scanning…' : 'Scan Uploaded File'}
          </button>

          {/* Upload scan error */}
          {uploadError && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '0.75rem 1rem', borderRadius: '0.375rem', background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)', fontSize: '0.8rem' }}>
              <AlertCircle size={14} />
              {(uploadError as Error).message}
            </div>
          )}

          {/* Upload scan results — grouped by manifest file, same shape as remote scan */}
          {uploadResult && (
            <div className="space-y-4">
              <div className="rounded-lg p-4" style={{ background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                  <FolderOpen size={14} style={{ color: 'var(--color-safe)' }} />
                  <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)', textTransform: 'uppercase' }}>
                    Scan Target
                  </span>
                </div>
                <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Source</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', fontWeight: 600, marginTop: 2 }}>
                      {uploadFile?.name ?? 'Uploaded archive'}
                    </p>
                  </div>
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Scan Type</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>Upload / Project Scan</p>
                  </div>
                  {uploadResult.root_dir && (
                    <div>
                      <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Scanned Directory</span>
                      <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{uploadResult.root_dir}</p>
                    </div>
                  )}
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Manifests Found</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{uploadResult.manifests.length}</p>
                  </div>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '0.75rem' }}>
                {[
                  { label: 'Critical', val: uploadResult.summary.critical, color: 'var(--color-critical)' },
                  { label: 'High',     val: uploadResult.summary.high,     color: 'var(--color-high)' },
                  { label: 'Medium',   val: uploadResult.summary.medium,   color: 'var(--color-medium)' },
                  { label: 'Low',      val: uploadResult.summary.low,      color: '#60A5FA' },
                  { label: 'Total',    val: uploadResult.summary.total,    color: 'var(--fg)' },
                ].map(({ label, val, color }) => (
                  <div key={label} style={{ borderRadius: '0.375rem', padding: '0.75rem', textAlign: 'center', background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <div style={{ fontSize: '1.75rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color }}>{val}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginTop: 2 }}>{label}</div>
                  </div>
                ))}
              </div>

              {uploadResult.missing_tools.length > 0 && (
                <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
                  Engines unavailable on the scanning server: {uploadResult.missing_tools.join(', ')}
                </p>
              )}

              <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
                {uploadResult.manifests.length} manifest file(s) found in {uploadFile?.name ?? 'uploaded archive'}
              </p>

              {uploadResult.results.map((r, i) => (
                r.findings.length === 0 ? null : (
                  <div key={i} className="rounded-lg" style={{ background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <div style={{ padding: '0.625rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', fontSize: '0.78rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)' }}>
                      {r.entry.ecosystem}/{r.entry.name}@{r.entry.version}
                      <span style={{ marginLeft: 8, fontSize: '0.68rem', color: 'var(--color-muted)' }}>{r.entry.file_path}</span>
                    </div>
                    <FindingsTable findings={r.findings} />
                  </div>
                )
              ))}

              {uploadResult.manifests.length === 0 && (
                <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-warn)', fontSize: '0.85rem' }}>
                  <AlertCircle size={24} style={{ margin: '0 auto 0.5rem' }} />
                  No recognized dependency manifests found in this archive.
                </div>
              )}

              {uploadResult.manifests.length > 0 && uploadResult.summary.total === 0 && (
                <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-safe)', fontSize: '0.85rem' }}>
                  <ShieldCheck size={24} style={{ margin: '0 auto 0.5rem' }} />
                  No findings across {uploadResult.manifests.length} manifest file(s).
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Remote (SSH) scan form */}
      {tab === 'remote' && (
        <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
            Connects over SSH, discovers dependency manifests (package.json, go.mod, etc.),
            pulls them to scan locally. Nothing is installed on the target — only read-only
            <code style={{ margin: '0 3px', color: 'var(--fg)' }}>find</code>/<code style={{ margin: '0 3px', color: 'var(--fg)' }}>cat</code> commands run remotely.
          </p>
          <div style={{ display: 'flex', alignItems: 'flex-start', gap: 8, padding: '0.625rem 0.75rem', borderRadius: '0.375rem', background: 'rgba(255,171,64,0.06)', border: '1px solid rgba(255,171,64,0.15)' }}>
            <Lock size={13} style={{ color: 'var(--color-warn)', marginTop: 1, flexShrink: 0 }} />
            <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
              The private key below is sent once for this scan and is never written to disk
              or stored by the server. Key-based auth only — no passwords.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <div style={{ gridColumn: 'span 2' }}>
              <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>TARGET (user@host)</label>
              <input
                value={remoteTarget}
                onChange={e => setRemoteTarget(e.target.value)}
                placeholder="e.g. deploy@10.0.4.12"
                style={inputStyle}
              />
            </div>
            <div>
              <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>PORT</label>
              <input
                value={remotePort}
                onChange={e => setRemotePort(e.target.value)}
                placeholder="22"
                style={inputStyle}
              />
            </div>
          </div>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>PRIVATE KEY (PEM)</label>
            <textarea
              value={remotePrivateKey}
              onChange={e => setRemotePrivateKey(e.target.value)}
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----&#10;...&#10;-----END OPENSSH PRIVATE KEY-----"
              rows={5}
              style={{ ...inputStyle, resize: 'vertical' as const, fontSize: '0.72rem' }}
              autoComplete="off"
              spellCheck={false}
            />
          </div>

          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>
              REMOTE PATH <span style={{ opacity: 0.6 }}>(optional — default: remote $HOME)</span>
            </label>
            <input
              value={remotePath}
              onChange={e => setRemotePath(e.target.value)}
              placeholder="/opt/app"
              style={inputStyle}
            />
          </div>

          <label style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.78rem', color: 'var(--color-muted)', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={acceptNewHostKey}
              onChange={e => setAcceptNewHostKey(e.target.checked)}
            />
            Trust this host's key if not already known to the server (does not persist it)
          </label>

          <button
            onClick={() => remoteScan.mutate()}
            disabled={!remoteTarget || !remotePrivateKey || remoteIsPending}
            style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '0.5rem 1.25rem', borderRadius: '0.375rem',
              background: remoteIsPending ? 'rgba(0,255,135,0.4)' : 'var(--color-safe)',
              color: '#0A0B0D', border: 'none', cursor: remoteIsPending ? 'not-allowed' : 'pointer',
              fontSize: '0.8rem', fontFamily: 'var(--font-mono)', fontWeight: 700,
            }}
          >
            {remoteIsPending ? <Loader size={14} className="animate-spin" /> : <Server size={14} />}
            {remoteIsPending ? 'Scanning…' : 'Scan Remote Host'}
          </button>

          {/* Remote scan error */}
          {remoteError && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '0.75rem 1rem', borderRadius: '0.375rem', background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)', fontSize: '0.8rem' }}>
              <AlertCircle size={14} />
              {(remoteError as Error).message}
            </div>
          )}

          {/* Remote scan results — grouped by manifest file */}
          {remoteResult && (
            <div className="space-y-4">
              <div className="rounded-lg p-4" style={{ background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
                  <Server size={14} style={{ color: 'var(--color-safe)' }} />
                  <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)', textTransform: 'uppercase' }}>
                    Scan Target
                  </span>
                </div>
                <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Remote Host</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', fontWeight: 600, marginTop: 2 }}>{remoteTarget}</p>
                  </div>
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Remote Path</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{remotePath || '~ (home)'}</p>
                  </div>
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Scan Type</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>Remote SSH Scan</p>
                  </div>
                  {remoteResult.root_dir && (
                    <div>
                      <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Scanned Directory</span>
                      <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{remoteResult.root_dir}</p>
                    </div>
                  )}
                  <div>
                    <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Manifests Found</span>
                    <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{remoteResult.manifests.length}</p>
                  </div>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '0.75rem' }}>
                {[
                  { label: 'Critical', val: remoteResult.summary.critical, color: 'var(--color-critical)' },
                  { label: 'High',     val: remoteResult.summary.high,     color: 'var(--color-high)' },
                  { label: 'Medium',   val: remoteResult.summary.medium,   color: 'var(--color-medium)' },
                  { label: 'Low',      val: remoteResult.summary.low,      color: '#60A5FA' },
                  { label: 'Total',    val: remoteResult.summary.total,    color: 'var(--fg)' },
                ].map(({ label, val, color }) => (
                  <div key={label} style={{ borderRadius: '0.375rem', padding: '0.75rem', textAlign: 'center', background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <div style={{ fontSize: '1.75rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color }}>{val}</div>
                    <div style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginTop: 2 }}>{label}</div>
                  </div>
                ))}
              </div>

              {remoteResult.missing_tools.length > 0 && (
                <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
                  Engines unavailable on the scanning server: {remoteResult.missing_tools.join(', ')}
                </p>
              )}

              <p style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
                {remoteTarget}:{remotePath || '~'} — {remoteResult.manifests.length} manifest file(s) scanned
              </p>

              {remoteResult.results.map((r, i) => (
                r.findings.length === 0 ? null : (
                  <div key={i} className="rounded-lg" style={{ background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <div style={{ padding: '0.625rem 0.875rem', borderBottom: '1px solid rgba(255,255,255,0.06)', fontSize: '0.78rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)' }}>
                      {r.entry.ecosystem}/{r.entry.name}@{r.entry.version}
                      <span style={{ marginLeft: 8, fontSize: '0.68rem', color: 'var(--color-muted)' }}>{r.entry.file_path}</span>
                    </div>
                    <FindingsTable findings={r.findings} />
                  </div>
                )
              ))}

              {remoteResult.summary.total === 0 && (
                <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-safe)', fontSize: '0.85rem' }}>
                  <ShieldCheck size={24} style={{ margin: '0 auto 0.5rem' }} />
                  No findings across {remoteResult.manifests.length} manifest file(s).
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* Error */}
      {error && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '0.75rem 1rem', borderRadius: '0.375rem', background: 'rgba(255,61,61,0.1)', color: 'var(--color-critical)', border: '1px solid rgba(255,61,61,0.2)', fontSize: '0.8rem' }}>
          <AlertCircle size={14} />
          {(error as Error).message}
        </div>
      )}

      {/* Empty state + scan history */}
      {!result && !isPending && !error && (
        <ScanHistory onRescan={(eco, name, ver) => { setEcosystem(eco); setPkg(name); setVersion(ver); setTab('registry'); }} />
      )}

      {/* Results */}
      {result && (
        <div className="space-y-4">

          {/* Scan target info */}
          <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginBottom: 8 }}>
              <Package size={14} style={{ color: 'var(--color-safe)' }} />
              <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)', textTransform: 'uppercase' }}>
                Scan Target
              </span>
            </div>
            <div style={{ display: 'flex', gap: '2rem', flexWrap: 'wrap' }}>
              <div>
                <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Package</span>
                <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', fontWeight: 600, marginTop: 2 }}>
                  {result.package}
                </p>
              </div>
              <div>
                <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Ecosystem</span>
                <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>{ecosystem}</p>
              </div>
              <div>
                <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Scan Type</span>
                <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2 }}>Registry Package</p>
              </div>
              {result.sha256 && (
                <div>
                  <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>SHA-256</span>
                  <p style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', color: 'var(--fg)', marginTop: 2, wordBreak: 'break-all' }}>{result.sha256}</p>
                </div>
              )}
              <div>
                <span style={{ fontSize: '0.6rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)', textTransform: 'uppercase' }}>Status</span>
                <p style={{ fontSize: '0.85rem', fontFamily: 'var(--font-mono)', color: result.downloaded ? 'var(--color-safe)' : 'var(--color-warn)', marginTop: 2 }}>
                  {result.downloaded ? 'Artifact downloaded' : 'Download failed — partial scan'}
                </p>
              </div>
            </div>
          </div>

          {/* Engine status bar */}
          {result.engines && result.engines.length > 0 && (
            <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)' }}>
                  ENGINES
                </span>
                <span style={{ fontSize: '0.7rem', color: result.downloaded ? 'var(--color-safe)' : 'var(--color-warn)' }}>
                  {result.downloaded ? '✓ artifact downloaded — full scan' : '⚠ download failed — OSV + behavioral only'}
                </span>
              </div>
              <EngineStatusBar engines={result.engines} />
            </div>
          )}

          {/* Summary cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(5, 1fr)', gap: '0.75rem' }}>
            {[
              { label: 'Critical', val: result.summary.critical, color: 'var(--color-critical)' },
              { label: 'High',     val: result.summary.high,     color: 'var(--color-high)' },
              { label: 'Medium',   val: result.summary.medium,   color: 'var(--color-medium)' },
              { label: 'Low',      val: result.summary.low,      color: '#60A5FA' },
              { label: 'Total',    val: result.summary.total,    color: 'var(--fg)' },
            ].map(({ label, val, color }) => (
              <div key={label} style={{ borderRadius: '0.375rem', padding: '0.75rem', textAlign: 'center', background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
                <div style={{ fontSize: '1.75rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color }}>{val}</div>
                <div style={{ fontSize: '0.72rem', color: 'var(--color-muted)', marginTop: 2 }}>{label}</div>
              </div>
            ))}
          </div>

          {/* Severity donut + Engine bar chart */}
          {(result.summary.total > 0 || (result.engines && result.engines.length > 0)) && (
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
              {/* Severity distribution donut */}
              {result.summary.total > 0 && (() => {
                const sevData = [
                  { name: 'Critical', value: result.summary.critical, color: 'var(--color-critical)' },
                  { name: 'High',     value: result.summary.high,     color: 'var(--color-high)' },
                  { name: 'Medium',   value: result.summary.medium,   color: 'var(--color-medium)' },
                  { name: 'Low',      value: result.summary.low,      color: '#60A5FA' },
                ].filter(d => d.value > 0);
                return (
                  <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)' }}>
                      SEVERITY DISTRIBUTION
                    </span>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginTop: '0.5rem' }}>
                      <ResponsiveContainer width="50%" height={140}>
                        <PieChart>
                          <Pie data={sevData} dataKey="value" cx="50%" cy="50%" innerRadius={35} outerRadius={55} paddingAngle={2} strokeWidth={0}>
                            {sevData.map((d, i) => <Cell key={i} fill={d.color} />)}
                          </Pie>
                          <RechartsTooltip
                            contentStyle={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: '0.75rem', color: 'var(--fg)' }}
                          />
                        </PieChart>
                      </ResponsiveContainer>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 6, flex: 1 }}>
                        {sevData.map(d => (
                          <div key={d.name} style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: '0.75rem' }}>
                            <span style={{ width: 8, height: 8, borderRadius: '50%', background: d.color, flexShrink: 0 }} />
                            <span style={{ color: 'var(--color-muted)', flex: 1 }}>{d.name}</span>
                            <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--fg)' }}>{d.value}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                );
              })()}

              {/* Findings by engine */}
              {result.engines && result.engines.length > 0 && (() => {
                const engineData = result.engines
                  .filter(e => e.status === 'ok')
                  .map(e => ({ name: e.engine, findings: e.findings }))
                  .sort((a, b) => b.findings - a.findings);
                if (engineData.length === 0) return null;
                return (
                  <div className="rounded-lg p-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
                    <span style={{ fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 600, color: 'var(--color-muted)' }}>
                      FINDINGS BY ENGINE
                    </span>
                    <ResponsiveContainer width="100%" height={140} style={{ marginTop: '0.5rem' }}>
                      <BarChart data={engineData} layout="vertical" margin={{ left: 0, right: 8, top: 4, bottom: 4 }}>
                        <XAxis type="number" hide />
                        <YAxis type="category" dataKey="name" width={70} tick={{ fontSize: 11, fill: 'var(--color-muted)', fontFamily: 'var(--font-mono)' }} axisLine={false} tickLine={false} />
                        <RechartsTooltip
                          contentStyle={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.1)', borderRadius: 6, fontSize: '0.75rem', color: 'var(--fg)' }}
                        />
                        <Bar dataKey="findings" fill="rgba(99,102,241,0.6)" radius={[0, 4, 4, 0]} barSize={16} />
                      </BarChart>
                    </ResponsiveContainer>
                  </div>
                );
              })()}
            </div>
          )}

          {/* Highest-severity informational banner — mirrors what --fail-on would act on */}
          {result.summary.highest_sev && (
            <div style={{
              display: 'flex', alignItems: 'center', gap: 8,
              padding: '0.625rem 1rem', borderRadius: '0.375rem',
              background: 'rgba(255,255,255,0.03)', border: '1px solid rgba(255,255,255,0.08)',
              fontSize: '0.78rem', color: 'var(--color-muted)',
            }}>
              <AlertCircle size={13} style={{ flexShrink: 0 }} />
              <span>
                This scan's highest severity: <SeverityBadge severity={result.summary.highest_sev} /> — would fail CI with{' '}
                <code style={{ color: 'var(--fg)' }}>
                  --fail-on={result.summary.highest_sev.toLowerCase()}
                </code>{' '}
                or lower.
              </span>
            </div>
          )}

          {/* Filters */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flexWrap: 'wrap' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
              <label className="text-xs font-mono" style={{ color: 'var(--color-muted)' }}>SEVERITY</label>
              <select
                value={severityFilter}
                onChange={e => setSeverityFilter(e.target.value as SeverityFilter)}
                style={{ ...inputStyle, width: 'auto', padding: '0.35rem 0.625rem', fontSize: '0.78rem' }}
              >
                <option value="all">All severities</option>
                <option value="critical">Critical+</option>
                <option value="high">High+</option>
                <option value="medium">Medium+</option>
              </select>
            </div>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', fontSize: '0.78rem', color: 'var(--color-muted)', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={onlyFixable}
                onChange={e => setOnlyFixable(e.target.checked)}
              />
              Only show fixable findings
            </label>
            {(severityFilter !== 'all' || onlyFixable) && (
              <span style={{ fontSize: '0.72rem', color: 'var(--color-muted)' }}>
                Showing {filteredFindings.length} of {result.findings.length} findings
              </span>
            )}
          </div>

          {/* Findings */}
          <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
            <div style={{ padding: '0.75rem 1rem', borderBottom: '1px solid rgba(255,255,255,0.06)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: '0.8rem', fontFamily: 'var(--font-mono)', color: 'var(--color-muted)' }}>
                FINDINGS — {result.package}
                {result.sha256 && <span style={{ marginLeft: 8, fontSize: '0.68rem', opacity: 0.5 }}>sha256:{result.sha256.slice(0, 12)}…</span>}
              </span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
                {result.summary.highest_sev && <SeverityBadge severity={result.summary.highest_sev} />}
                <button
                  onClick={() => setGrouped(g => !g)}
                  style={{ fontSize: '0.72rem', padding: '0.25rem 0.625rem', borderRadius: '0.25rem', background: 'rgba(255,255,255,0.06)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.1)', cursor: 'pointer', fontFamily: 'var(--font-mono)' }}
                >
                  {grouped ? 'Flat' : 'Grouped'}
                </button>
              </div>
            </div>
            {filteredFindings.length === 0 ? (
              <div style={{ padding: '2rem', textAlign: 'center', color: 'var(--color-safe)', fontSize: '0.85rem' }}>
                <ShieldCheck size={24} style={{ margin: '0 auto 0.5rem' }} />
                {result.findings.length === 0 ? 'No findings — package looks clean.' : 'No findings match the current filters.'}
              </div>
            ) : groupedFindings ? (
              <div>
                {Array.from(groupedFindings.entries()).map(([key, group]) => (
                  <div key={key} style={{ borderBottom: '1px solid rgba(255,255,255,0.04)' }}>
                    <div style={{ padding: '0.5rem 1rem', background: 'rgba(255,255,255,0.02)', fontSize: '0.72rem', fontFamily: 'var(--font-mono)', fontWeight: 700, color: 'var(--color-muted)' }}>
                      {key} <span style={{ color: 'var(--fg)' }}>({group.length})</span>
                    </div>
                    <FindingsTable findings={group} />
                  </div>
                ))}
              </div>
            ) : (
              <FindingsTable findings={filteredFindings} />
            )}
          </div>
        </div>
      )}
    </div>
  );
}

import { useState } from 'react';
import { Terminal, Server } from 'lucide-react';
import { CopyButton } from '../components/CopyButton';
import { cn } from '../components/ui/utils';

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

interface Endpoint {
  method: Method;
  path: string;
  description: string;
}

interface EndpointGroup {
  title: string;
  endpoints: Endpoint[];
}

// Derived directly from the route-table doc comment at the top of
// internal/api/main.go (and the route registrations further down that file)
// — every entry here corresponds to a real, wired-up handler. Grouped by URL
// prefix for readability; healthz/metrics get their own "System" group since
// they sit outside /api/v1.
const GROUPS: EndpointGroup[] = [
  {
    title: 'System',
    endpoints: [
      { method: 'GET', path: '/healthz', description: 'Liveness probe' },
      { method: 'GET', path: '/metrics', description: 'Prometheus metrics' },
    ],
  },
  {
    title: 'Packages',
    endpoints: [
      { method: 'GET', path: '/api/v1/packages', description: 'List packages (paginated)' },
      { method: 'GET', path: '/api/v1/packages/:ecosystem/:name', description: 'Package detail' },
      { method: 'GET', path: '/api/v1/packages/:ecosystem/:name/versions', description: 'List versions' },
    ],
  },
  {
    title: 'Scan',
    endpoints: [
      { method: 'POST', path: '/api/v1/scan', description: 'Trigger scan (async — returns job_id)' },
      { method: 'POST', path: '/api/v1/scan/upload', description: 'Trigger scan on uploaded archive (async)' },
      { method: 'POST', path: '/api/v1/scan/remote', description: 'Scan a remote host over SSH (async — manifests pulled, nothing installed remotely)' },
      { method: 'GET', path: '/api/v1/scan/:ecosystem/:name/:version', description: 'Get scan results' },
      { method: 'GET', path: '/api/v1/jobs/:id', description: 'Poll async scan job status/result' },
    ],
  },
  {
    title: 'Advisory',
    endpoints: [
      { method: 'POST', path: '/api/v1/advisory', description: 'Generate AI advisory' },
    ],
  },
  {
    title: 'SBOM',
    endpoints: [
      { method: 'GET', path: '/api/v1/sbom/:ecosystem/:name/:version', description: 'Get SBOM' },
    ],
  },
  {
    title: 'Sign & Verify',
    endpoints: [
      { method: 'POST', path: '/api/v1/sign', description: 'Sign artifact' },
      { method: 'POST', path: '/api/v1/verify', description: 'Verify attestation' },
    ],
  },
  {
    title: 'Provenance',
    endpoints: [
      { method: 'POST', path: '/api/v1/provenance', description: 'Generate SLSA provenance (no signing/persistence)' },
    ],
  },
  {
    title: 'Dashboard',
    endpoints: [
      { method: 'GET', path: '/api/v1/dashboard/stats', description: 'Dashboard summary stats' },
      { method: 'GET', path: '/api/v1/dashboard/recent', description: 'Recent scan activity' },
      { method: 'GET', path: '/api/v1/dashboard/timeline', description: 'Findings timeline' },
      { method: 'GET', path: '/api/v1/dashboard/graph', description: 'Dependency graph of recent scans' },
      { method: 'GET', path: '/api/v1/dashboard/activity', description: 'Recent activity feed' },
    ],
  },
  {
    title: 'Risks',
    endpoints: [
      { method: 'GET', path: '/api/v1/risks', description: 'Active risks' },
    ],
  },
  {
    title: 'Policy',
    endpoints: [
      { method: 'GET', path: '/api/v1/policy/status', description: 'Current policy config + status' },
      { method: 'PUT', path: '/api/v1/policy', description: 'Save policy configuration' },
    ],
  },
  {
    title: 'Intelligence',
    endpoints: [
      { method: 'GET', path: '/api/v1/intelligence/signatures', description: 'List detection signatures' },
      { method: 'POST', path: '/api/v1/intelligence/refresh', description: 'Trigger intel agent run' },
      { method: 'POST', path: '/api/v1/intelligence/signatures', description: 'Author a new detection signature (returns YAML)' },
      { method: 'POST', path: '/api/v1/intelligence/validate', description: 'Validate a signature YAML document' },
      { method: 'POST', path: '/api/v1/intelligence/test', description: 'Heuristic signature test against a live scan' },
    ],
  },
  {
    title: 'Audit',
    endpoints: [
      { method: 'GET', path: '/api/v1/audit/stats', description: 'Signature store runtime statistics' },
    ],
  },
  {
    title: 'Webhooks',
    endpoints: [
      { method: 'POST', path: '/api/v1/webhooks/test', description: 'Test a configured webhook endpoint' },
    ],
  },
  {
    title: 'Agent',
    endpoints: [
      { method: 'GET', path: '/api/v1/agent/stream', description: 'Stream live patch-agent events (SSE)' },
      { method: 'POST', path: '/api/v1/agent/events', description: 'Publish a patch-agent event' },
    ],
  },
  {
    title: 'Allowlist',
    endpoints: [
      { method: 'GET', path: '/api/v1/allowlist', description: 'List allowlist entries' },
      { method: 'POST', path: '/api/v1/allowlist', description: 'Add allowlist entry' },
      { method: 'DELETE', path: '/api/v1/allowlist/:id', description: 'Delete allowlist entry' },
      { method: 'GET', path: '/api/v1/allowlist/check', description: 'Check whether a package is allowlisted' },
    ],
  },
  {
    title: 'Alerts',
    endpoints: [
      { method: 'GET', path: '/api/v1/alerts', description: 'List alerts' },
      { method: 'POST', path: '/api/v1/alerts', description: 'Create alert' },
      { method: 'POST', path: '/api/v1/alerts/:id/dismiss', description: 'Dismiss alert' },
    ],
  },
  {
    title: 'Auth',
    endpoints: [
      { method: 'POST', path: '/api/v1/auth/login', description: 'Dashboard login (sets session cookie)' },
      { method: 'POST', path: '/api/v1/auth/logout', description: 'Dashboard logout (clears session cookie)' },
      { method: 'GET', path: '/api/v1/auth/me', description: 'Current dashboard session auth state' },
    ],
  },
];

const METHOD_COLOR: Record<Method, string> = {
  GET: 'var(--primary-blue)',
  POST: 'var(--success)',
  PUT: 'var(--warning)',
  DELETE: 'var(--critical)',
};

const METHOD_BG: Record<Method, string> = {
  GET: 'var(--blue-light)',
  POST: 'rgba(22,163,74,0.1)',
  PUT: 'rgba(217,119,6,0.1)',
  DELETE: 'rgba(220,38,38,0.1)',
};

function baseUrl() {
  return (import.meta.env.VITE_API_URL as string | undefined) ?? 'https://your-forgeguardian-instance';
}

function MethodBadge({ method }: { method: Method }) {
  return (
    <span
      className="inline-flex items-center justify-center rounded-md px-2 py-0.5 text-[0.68rem] font-bold font-mono shrink-0"
      style={{ color: METHOD_COLOR[method], background: METHOD_BG[method] }}
    >
      {method}
    </span>
  );
}

function EndpointRow({ endpoint }: { endpoint: Endpoint }) {
  const curl = `curl -X ${endpoint.method} ${baseUrl()}${endpoint.path}`;
  return (
    <div className="flex items-center gap-3 px-4 py-3 border-b border-border-color last:border-b-0">
      <MethodBadge method={endpoint.method} />
      <div className="flex-1 min-w-0">
        <code className="text-xs font-mono text-text-primary break-all">{endpoint.path}</code>
        <p className="text-xs text-text-secondary mt-0.5">{endpoint.description}</p>
      </div>
      <CopyButton text={curl} label="curl" className="shrink-0" />
    </div>
  );
}

function HeroExample() {
  const [tab, setTab] = useState<'request' | 'response'>('request');

  const curl = `curl -X POST ${baseUrl()}/api/v1/scan \\
  -H "Content-Type: application/json" \\
  -d '{"ecosystem":"npm","package":"lodash","version":"4.17.20"}'`;

  const requestBody = `{
  "ecosystem": "npm",
  "package": "lodash",
  "version": "4.17.20"
}`;

  const responseBody = `{
  "job_id": "b3f1c2a4-...",
  "status": "queued",
  "created_at": "2026-08-08T12:00:00Z",
  "updated_at": "2026-08-08T12:00:00Z"
}`;

  return (
    <div className="rounded-xl border border-border-color bg-surface shadow-sm overflow-hidden">
      <div className="flex items-center justify-between px-4 pt-4 pb-2">
        <div className="flex items-center gap-2.5">
          <MethodBadge method="POST" />
          <code className="text-sm font-mono text-text-primary">/api/v1/scan</code>
        </div>
        <span className="text-[0.65rem] font-medium px-2 py-0.5 rounded-full bg-surface-muted text-text-secondary">
          Worked example
        </span>
      </div>
      <p className="px-4 text-xs text-text-secondary">
        Triggers an async scan job and immediately returns a job_id — poll{' '}
        <code className="text-[0.72rem]">GET /api/v1/jobs/:id</code> until <code className="text-[0.72rem]">status</code> settles
        to <code className="text-[0.72rem]">complete</code> or <code className="text-[0.72rem]">failed</code>.
      </p>

      <div className="px-4 pt-3 flex items-center gap-2">
        <button
          type="button"
          onClick={() => setTab('request')}
          className={cn(
            'text-xs font-medium px-3 py-1.5 rounded-md focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none',
            tab === 'request' ? 'bg-blue-light text-primary-blue' : 'text-text-secondary hover:bg-surface-muted'
          )}
        >
          Request
        </button>
        <button
          type="button"
          onClick={() => setTab('response')}
          className={cn(
            'text-xs font-medium px-3 py-1.5 rounded-md focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none',
            tab === 'response' ? 'bg-blue-light text-primary-blue' : 'text-text-secondary hover:bg-surface-muted'
          )}
        >
          Response
        </button>
      </div>

      <div className="p-4 pt-3">
        {tab === 'request' ? (
          <div className="flex flex-col gap-2">
            <div className="rounded-lg border border-border-color bg-surface-muted overflow-hidden">
              <div className="flex items-center justify-end px-2 py-1 border-b border-border-color">
                <CopyButton text={curl} label="Copy curl" />
              </div>
              <pre className="fg-docs-pre p-3 text-xs font-mono text-text-primary overflow-x-auto">
                {curl}
              </pre>
            </div>
            <div className="rounded-lg border border-border-color bg-surface-muted overflow-hidden">
              <div className="flex items-center justify-end px-2 py-1 border-b border-border-color">
                <CopyButton text={requestBody} label="Copy body" />
              </div>
              <pre className="fg-docs-pre p-3 text-xs font-mono text-text-primary overflow-x-auto">
                {requestBody}
              </pre>
            </div>
          </div>
        ) : (
          <div className="rounded-lg border border-border-color bg-surface-muted overflow-hidden">
            <div className="flex items-center justify-end px-2 py-1 border-b border-border-color">
              <CopyButton text={responseBody} label="Copy" />
            </div>
            <pre className="fg-docs-pre p-3 text-xs font-mono text-text-primary overflow-x-auto">
              {responseBody}
            </pre>
          </div>
        )}
      </div>
    </div>
  );
}

export function ApiDocsPage() {
  return (
    <div className="flex flex-col gap-5 p-6">
      <div className="flex items-center gap-3">
        <Server size={20} className="text-primary-blue" />
        <div>
          <h1 className="text-lg font-bold text-text-primary">API Reference</h1>
          <p className="mt-0.5 text-xs text-text-secondary">
            Real routes from the ForgeGuardian REST API, grouped by area. Base URL defaults to{' '}
            <code className="text-[0.72rem]">http://localhost:8080</code> in local dev, or your{' '}
            <code className="text-[0.72rem]">VITE_API_URL</code> if set.
          </p>
        </div>
      </div>

      <div className="rounded-lg border border-border-color bg-surface-muted p-3 flex items-start gap-2.5">
        <Terminal size={14} className="text-text-muted mt-0.5 shrink-0" />
        <p className="text-xs text-text-secondary">
          Every path below has a copy-to-clipboard <code className="text-[0.72rem]">curl</code> snippet using the
          real method + path. Request/response bodies are only shown in full for the worked example — the
          other endpoints intentionally omit fabricated JSON bodies since we haven't verified every handler's
          exact wire shape.
        </p>
      </div>

      <HeroExample />

      {GROUPS.map(group => (
        <div key={group.title}>
          <h2 className="text-xs font-semibold uppercase tracking-wide text-text-muted mb-2">{group.title}</h2>
          <div className="rounded-xl border border-border-color bg-surface shadow-sm overflow-hidden">
            {group.endpoints.map(ep => (
              <EndpointRow key={`${ep.method}-${ep.path}`} endpoint={ep} />
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

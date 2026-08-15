// Typed API client for the ForgeGuardian REST API

const BASE = import.meta.env.VITE_API_URL ?? '';

// Optional API key — set VITE_API_KEY in .env to enable auth.
const API_KEY = import.meta.env.VITE_API_KEY ?? '';

function authHeaders(): Record<string, string> {
  return API_KEY ? { 'X-Api-Key': API_KEY } : {};
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 30_000)
  try {
    const res = await fetch(`${BASE}${path}`, {
      headers: { 'Content-Type': 'application/json', ...authHeaders(), ...options?.headers },
      signal: controller.signal,
      credentials: 'include',
      ...options,
    })
    clearTimeout(timeoutId)
    if (!res.ok) {
      const body = await res.text()
      throw new Error(`API ${res.status}: ${body}`)
    }
    // Validate Content-Type before attempting JSON parse.
    // Non-JSON responses (HTML error pages, proxy errors) cause "Unexpected token '<'".
    const contentType = res.headers.get('content-type') ?? ''
    if (!contentType.includes('application/json')) {
      const body = await res.text()
      throw new Error(
        `API ${res.status}: expected JSON but got ${contentType || 'unknown content-type'} — ` +
        `is the ForgeGuardian API server running? (${body.slice(0, 120)})`
      )
    }
    let data: unknown
    try { data = await res.json() }
    catch (e) { throw new Error(`API ${res.status}: response is not JSON`, { cause: e }) }
    return data as T
  } catch (err) {
    clearTimeout(timeoutId)
    if (err instanceof Error && err.name === 'AbortError') {
      throw new Error('API request timed out after 30s', { cause: err })
    }
    throw err
  }
}

// Health — always-on liveness probe, independent of DATABASE_URL. Use this
// (not getDashboardStats) for "is the API reachable" checks — dashboard/stats
// 503s whenever no DB is configured, which is a normal dev setup, not an
// outage, so using it for reachability produces false "API offline" reports.
export const getHealth = () =>
  request<{ status: string }>('/healthz');

// Dashboard
export const getDashboardStats = () =>
  request<import('../types/api').DashboardStats>('/api/v1/dashboard/stats');

export const getRecentResults = (limit = 20) =>
  request<{ limit: number; results: import('../types/api').RecentResult[] }>(
    `/api/v1/dashboard/recent?limit=${limit}`
  );

// Packages
export const listPackages = (params?: { page?: number; page_size?: number; ecosystem?: string }) => {
  const q = new URLSearchParams();
  if (params?.page) q.set('page', String(params.page));
  if (params?.page_size) q.set('page_size', String(params.page_size));
  if (params?.ecosystem) q.set('ecosystem', params.ecosystem);
  return request<{ packages: import('../types/api').PackageInfo[]; total: number }>(`/api/v1/packages?${q}`);
};

// Scan — both endpoints return a job immediately (202); poll getJobStatus
// until status is 'complete' or 'failed'. Scans can run well past any
// single HTTP request's timeout (grype/trivy/semgrep on a large artifact).
export const triggerScan = (ecosystem: string, pkg: string, version: string) =>
  request<import('../types/api').ScanJob>('/api/v1/scan', {
    method: 'POST',
    body: JSON.stringify({ ecosystem, package: pkg, version }),
  }).then(normalizeScanJob);

// The Go backend's scanner.ScanSummary struct has no json tags, so the raw
// wire shape is PascalCase (Critical/High/Medium/Low/Informational/Total/
// HighestSev) while types/api.ts's ScanSummary (and every dashboard
// component reading it, e.g. computeSecurityScore) expects lowercase
// snake_case keys (critical/high/.../highest_sev). Left unnormalized this
// silently produces `undefined` -> `NaN` throughout the UI. Normalize once
// here at the client boundary rather than special-casing every caller.
function normalizeScanJob(job: import('../types/api').ScanJob): import('../types/api').ScanJob {
  const raw = job.result as unknown as Record<string, unknown> | undefined;
  const rawSummary = raw?.summary as Record<string, unknown> | undefined;
  if (!rawSummary) return job;
  const pick = (lower: string, pascal: string) =>
    (rawSummary[lower] ?? rawSummary[pascal] ?? 0) as number;
  const normalized: import('../types/api').ScanSummary = {
    critical: pick('critical', 'Critical'),
    high: pick('high', 'High'),
    medium: pick('medium', 'Medium'),
    low: pick('low', 'Low'),
    informational: pick('informational', 'Informational'),
    total: pick('total', 'Total'),
    highest_sev: (rawSummary.highest_sev ?? rawSummary.HighestSev ?? 'INFORMATIONAL') as import('../types/api').Severity,
  };
  return { ...job, result: { ...job.result!, summary: normalized } };
}

export const getJobStatus = (jobId: string) =>
  request<import('../types/api').ScanJob>(`/api/v1/jobs/${jobId}`).then(normalizeScanJob);

// internal/localscanner.ProjectScanResult (and its nested ManifestEntry /
// LocalScanResult / ManifestFile) has no json tags — same PascalCase-wire
// problem as ScanSummary above, just one layer deeper. Normalize once here
// rather than special-casing every dashboard component that reads a remote
// scan's result.
function normalizeProjectScanResult(raw: Record<string, unknown>): import('../types/api').ProjectScanResult {
  const pick = <T,>(obj: Record<string, unknown>, lower: string, pascal: string, fallback: T): T =>
    (obj[lower] ?? obj[pascal] ?? fallback) as T;

  const rawResults = pick<unknown[]>(raw, 'results', 'Results', []);
  const results = rawResults.map((r) => {
    const rr = r as Record<string, unknown>;
    const rawEntry = pick<Record<string, unknown>>(rr, 'entry', 'Entry', {});
    return {
      entry: {
        ecosystem: pick(rawEntry, 'ecosystem', 'Ecosystem', ''),
        name: pick(rawEntry, 'name', 'Name', ''),
        version: pick(rawEntry, 'version', 'Version', ''),
        file_path: pick(rawEntry, 'file_path', 'FilePath', ''),
        line: pick(rawEntry, 'line', 'Line', 0),
        raw: pick(rawEntry, 'raw', 'Raw', ''),
        is_dev_dependency: pick(rawEntry, 'is_dev_dependency', 'IsDevDependency', false),
      },
      findings: pick(rr, 'findings', 'Findings', []),
      skipped: pick(rr, 'skipped', 'Skipped', false),
      err: pick<string | undefined>(rr, 'err', 'Err', undefined),
    };
  });

  const rawManifests = pick<unknown[]>(raw, 'manifests', 'Manifests', []);
  const manifests = rawManifests.map((m) => {
    const mm = m as Record<string, unknown>;
    return { path: pick(mm, 'path', 'Path', ''), ecosystem: pick(mm, 'ecosystem', 'Ecosystem', '') };
  });

  const rawSummary = pick<Record<string, unknown>>(raw, 'summary', 'Summary', {});
  const summary: import('../types/api').ScanSummary = {
    critical: pick(rawSummary, 'critical', 'Critical', 0),
    high: pick(rawSummary, 'high', 'High', 0),
    medium: pick(rawSummary, 'medium', 'Medium', 0),
    low: pick(rawSummary, 'low', 'Low', 0),
    informational: pick(rawSummary, 'informational', 'Informational', 0),
    total: pick(rawSummary, 'total', 'Total', 0),
    highest_sev: pick(rawSummary, 'highest_sev', 'HighestSev', 'INFORMATIONAL' as import('../types/api').Severity),
  };

  return {
    root_dir: pick(raw, 'root_dir', 'RootDir', ''),
    manifests,
    results,
    missing_tools: pick(raw, 'missing_tools', 'MissingTools', []),
    summary,
  };
}

function normalizeRemoteScanJob(job: import('../types/api').RemoteScanJob): import('../types/api').RemoteScanJob {
  if (!job.result) return job;
  return { ...job, result: normalizeProjectScanResult(job.result as unknown as Record<string, unknown>) };
}

// Remote (SSH) scan — connects to a target host, pulls dependency manifests,
// scans them. The private key is sent once for this request only; the
// server never persists it (see internal/api/handlers/remote_scan.go).
export const triggerRemoteScan = (opts: {
  target: string;
  privateKey: string;
  port?: number;
  remotePath?: string;
  acceptNewHostKey?: boolean;
  maxDepth?: number;
}) =>
  request<import('../types/api').RemoteScanJob>('/api/v1/scan/remote', {
    method: 'POST',
    body: JSON.stringify({
      target: opts.target,
      private_key: opts.privateKey,
      port: opts.port,
      remote_path: opts.remotePath,
      accept_new_host_key: opts.acceptNewHostKey,
      max_depth: opts.maxDepth,
    }),
  }).then(normalizeRemoteScanJob);

export const getRemoteScanJobStatus = (jobId: string) =>
  request<import('../types/api').RemoteScanJob>(`/api/v1/jobs/${jobId}`).then(normalizeRemoteScanJob);

// Upload's result is a whole-project ProjectScanResult (one entry per real
// dependency found in whatever manifests are inside the archive) — same
// shape and same normalization as a --remote scan, not a single-package
// ScanResult, since the backend now walks the archive with localscanner
// instead of scanning it as one opaque "local" artifact.
export async function scanUpload(file: File, name?: string): Promise<import('../types/api').RemoteScanJob> {
  const form = new FormData();
  form.append('file', file);
  if (name) form.append('name', name);
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 300_000); // 5 min for large files
  try {
    const res = await fetch(`${BASE}/api/v1/scan/upload`, {
      method: 'POST',
      headers: { ...authHeaders() },
      body: form,
      signal: controller.signal,
      credentials: 'include',
    });
    clearTimeout(timeoutId);
    if (!res.ok) {
      const body = await res.text();
      throw new Error(`API ${res.status}: ${body}`);
    }
    const job = await res.json();
    return normalizeRemoteScanJob(job);
  } catch (err) {
    clearTimeout(timeoutId);
    if (err instanceof Error && err.name === 'AbortError') throw new Error('Upload timed out after 5 minutes', { cause: err });
    throw err;
  }
}

function normalizeScanResult(result: import('../types/api').ScanResult): import('../types/api').ScanResult {
  return normalizeScanJob({ result } as import('../types/api').ScanJob).result!;
}

export const getScanResults = (ecosystem: string, pkg: string, version: string) =>
  request<import('../types/api').ScanResult>(`/api/v1/scan/${ecosystem}/${pkg}/${version}`).then(normalizeScanResult);

// Advisory
export const generateAdvisory = (
  ecosystem: string,
  pkg: string,
  version: string,
  findings: import('../types/api').Finding[] = []
) =>
  request<import('../types/api').Advisory>('/api/v1/advisory', {
    method: 'POST',
    body: JSON.stringify({ ecosystem, package: pkg, version, findings }),
  });

// SBOM — returns raw text (XML or JSON depending on format); uses AbortController for timeout
export const getSBOM = async (ecosystem: string, pkg: string, version: string, format = 'cyclonedx-json'): Promise<string> => {
  const controller = new AbortController()
  const timeoutId = setTimeout(() => controller.abort(), 30_000)
  try {
    const res = await fetch(`${BASE}/api/v1/sbom/${ecosystem}/${pkg}/${version}?format=${format}`, {
      signal: controller.signal,
      credentials: 'include',
      headers: { ...authHeaders() },
    })
    clearTimeout(timeoutId)
    if (!res.ok) {
      const body = await res.text()
      throw new Error(`API ${res.status}: ${body}`)
    }
    return res.text()
  } catch (err) {
    clearTimeout(timeoutId)
    if (err instanceof Error && err.name === 'AbortError') {
      throw new Error('API request timed out after 30s', { cause: err })
    }
    throw err
  }
}

// Sign + Verify
export const signArtifact = (sha256: string, ecosystem: string, pkg: string, version: string) =>
  request<import('../types/api').Attestation>('/api/v1/sign', {
    method: 'POST',
    body: JSON.stringify({ sha256, ecosystem, package: pkg, version }),
  });

// The backend returns HTTP 422 (not 200) when verification legitimately
// fails — that's a real, well-formed VerifyResult body, not a transport
// error, so this bypasses the shared request() helper (which throws on any
// non-2xx) and parses the body regardless of status. Otherwise a failed
// verification is indistinguishable from the API being unreachable, and
// verify.data.valid === false becomes unreachable dead code on the caller.
export const verifyAttestation = async (attestation: import('../types/api').Attestation, sha256: string) => {
  const res = await fetch(`${BASE}/api/v1/verify`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    credentials: 'include',
    body: JSON.stringify({ attestation, sha256 }),
  });
  const contentType = res.headers.get('content-type') ?? '';
  if (!contentType.includes('application/json')) {
    const text = await res.text();
    throw new Error(`API ${res.status}: expected JSON but got ${contentType || 'unknown content-type'} (${text.slice(0, 120)})`);
  }
  const data = await res.json();
  if (!res.ok && res.status !== 422) {
    throw new Error(`API ${res.status}: ${JSON.stringify(data)}`);
  }
  return data as import('../types/api').VerifyResult;
};

// Intelligence
export const listSignatures = (params?: { type?: string; ecosystem?: string }) => {
  const q = new URLSearchParams();
  if (params?.type) q.set('type', params.type);
  if (params?.ecosystem) q.set('ecosystem', params.ecosystem);
  return request<{
    total: number;
    updated_at: string | null;
    signatures: import('../types/api').DetectionSignature[];
  }>(`/api/v1/intelligence/signatures?${q}`);
};

export const refreshIntelligence = () =>
  request<{ message: string; before: number; added: number; total: number }>('/api/v1/intelligence/refresh', { method: 'POST' });

// Dashboard timeline
export interface TimelinePoint {
  date: string
  critical: number
  high: number
  medium: number
  low: number
  total: number
}

export const getDashboardTimeline = (days = 30) =>
  request<{ days: number; points: TimelinePoint[] }>(`/api/v1/dashboard/timeline?days=${days}`);

export const getDashboardActivity = (limit = 20) =>
  request<{ events: import('../types/api').ActivityEvent[] }>(`/api/v1/dashboard/activity?limit=${limit}`);

export const getActiveRisks = () =>
  request<{ risks: import('../types/api').RiskItem[] }>('/api/v1/risks');

export interface DependencyGraphData {
  nodes: Array<{ id: string; name: string; version: string; severity: string }>
  links: Array<{ source: string; target: string }>
}

export const getDependencyGraph = (limit = 20) =>
  request<DependencyGraphData>(`/api/v1/dashboard/graph?limit=${limit}`);

export const getPolicyStatus = () =>
  request<import('../types/api').PolicyStatus>('/api/v1/policy/status');

export const savePolicy = (policy: import('../types/api').Policy) =>
  request<import('../types/api').Policy>('/api/v1/policy', {
    method: 'PUT',
    body: JSON.stringify(policy),
  });

// Provenance
export const generateProvenance = (sha256: string, ecosystem: string, pkg: string, version: string) =>
  request<import('../types/api').Provenance>('/api/v1/provenance', {
    method: 'POST',
    body: JSON.stringify({ sha256, ecosystem, package: pkg, version }),
  });

// Signature authoring
export const generateSignature = (input: import('../types/api').SignatureInput) =>
  request<import('../types/api').GeneratedSignature>('/api/v1/intelligence/signatures', {
    method: 'POST',
    body: JSON.stringify(input),
  });

export const validateSignatureYaml = (yaml: string) =>
  request<import('../types/api').ValidationResult>('/api/v1/intelligence/validate', {
    method: 'POST',
    body: JSON.stringify({ yaml }),
  });

export const testSignature = (yaml: string, ecosystem: string, pkg: string, version: string) =>
  request<import('../types/api').SignatureTestResult>('/api/v1/intelligence/test', {
    method: 'POST',
    body: JSON.stringify({ yaml, ecosystem, package: pkg, version }),
  });

// Audit stats
export const getAuditStats = () =>
  request<import('../types/api').AuditStats>('/api/v1/audit/stats');

// Allowlist
export const listAllowlist = () =>
  request<{ allowlist: import('../types/api').AllowlistEntry[]; total: number }>('/api/v1/allowlist');

export const addAllowlist = (ecosystem: string, pkg: string, reason: string, addedBy?: string) =>
  request<import('../types/api').AllowlistEntry>('/api/v1/allowlist', {
    method: 'POST',
    body: JSON.stringify({ ecosystem, package: pkg, reason, added_by: addedBy ?? 'dashboard' }),
  });

export const deleteAllowlist = (id: number) =>
  request<{ ok: boolean }>(`/api/v1/allowlist/${id}`, { method: 'DELETE' });

export const checkAllowlist = (pkg: string, ecosystem: string) =>
  request<{ allowlisted: boolean; package: string; ecosystem: string }>(
    `/api/v1/allowlist/check?package=${encodeURIComponent(pkg)}&ecosystem=${encodeURIComponent(ecosystem)}`
  );

// Alerts
export const listAlerts = (params?: { page?: number; page_size?: number; severity?: string; dismissed?: boolean }) => {
  const q = new URLSearchParams();
  if (params?.page) q.set('page', String(params.page));
  if (params?.page_size) q.set('page_size', String(params.page_size));
  if (params?.severity) q.set('severity', params.severity);
  if (params?.dismissed !== undefined) q.set('dismissed', String(params.dismissed));
  return request<{ page: number; page_size: number; total: number; alerts: import('../types/api').AlertItem[] }>(
    `/api/v1/alerts?${q}`
  );
};

export const createAlert = (type: string, message: string, severity: string, packageName?: string, ecosystem?: string, version?: string) =>
  request<import('../types/api').AlertItem>('/api/v1/alerts', {
    method: 'POST',
    body: JSON.stringify({ type, message, severity, package_name: packageName, ecosystem, version }),
  });

export const dismissAlert = (id: number) =>
  request<{ ok: boolean }>(`/api/v1/alerts/${id}/dismiss`, { method: 'POST' });

// Auth — session cookie is httpOnly and set/cleared server-side; the client
// only ever inspects response status/body, never stores a token itself.
export const login = (email: string, password: string) =>
  request<{ ok: boolean }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ email, password }),
  });

export const logout = () =>
  request<{ ok: boolean }>('/api/v1/auth/logout', { method: 'POST' });

export const getAuthStatus = () =>
  request<{ auth_enabled: boolean; authenticated: boolean; email?: string }>('/api/v1/auth/me');

// Webhooks — POST /api/v1/webhooks/test. Real endpoint; actual delivery is
// configured locally via fgctl config (notify.slack_webhook_url etc.), this
// call only verifies the API route responds.
export const testWebhook = () =>
  request<{ message?: string; status: 'ok' | 'failed' | 'not_configured'; attempted?: string[]; errors?: string[] }>(
    '/api/v1/webhooks/test', { method: 'POST' },
  );

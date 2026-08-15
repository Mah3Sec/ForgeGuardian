// Types matching the Go API response shapes

export type Severity = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'INFORMATIONAL';
export type Ecosystem = 'npm' | 'pypi' | 'go' | 'rubygems' | 'crates' | 'maven' | 'huggingface' | 'mcp';

export interface Finding {
  id: string;
  severity: Severity;
  type: string;
  title: string;
  description: string;
  source: string;
  fixed_version?: string;
  metadata?: Record<string, unknown>;
}

export interface ScanSummary {
  critical: number;
  high: number;
  medium: number;
  low: number;
  informational: number;
  total: number;
  highest_sev: Severity;
}

export interface EngineStatus {
  engine: string;
  status: 'ok' | 'skipped';
  findings: number;
  error: string;
}

export interface ScanResult {
  package: string;
  sha256?: string;
  downloaded?: boolean;
  engines?: EngineStatus[];
  summary: ScanSummary;
  findings: Finding[];
}

// Scans run asynchronously — trigger/upload return a job immediately,
// then the caller polls GET /api/v1/jobs/:id until status settles.
export interface ScanJob {
  job_id: string;
  status: 'queued' | 'running' | 'complete' | 'failed';
  result?: ScanResult;
  error?: string;
  created_at: string;
  updated_at: string;
}

// Mirrors internal/localscanner.ManifestEntry — one dependency found in a
// manifest file. The Go struct has no json tags (PascalCase on the wire);
// normalizeRemoteScanJob() in lib/api.ts converts to this shape.
export interface ManifestEntry {
  ecosystem: string;
  name: string;
  version: string;
  file_path: string;
  line: number;
  raw: string;
  is_dev_dependency: boolean;
}

// Mirrors internal/localscanner.LocalScanResult.
export interface LocalScanResult {
  entry: ManifestEntry;
  findings: Finding[];
  skipped: boolean;
  err?: string;
}

// Mirrors internal/localscanner.ManifestFile.
export interface ManifestFile {
  path: string;
  ecosystem: string;
}

// Mirrors internal/localscanner.ProjectScanResult — the result shape for
// both `fgctl scan .` and a --remote scan, returned as a job's `result`.
export interface ProjectScanResult {
  root_dir: string;
  manifests: ManifestFile[];
  results: LocalScanResult[];
  missing_tools: string[];
  summary: ScanSummary;
}

// A remote (SSH) scan job — same async job/poll model as ScanJob, but its
// result is a ProjectScanResult (a whole project's findings) rather than a
// single package's ScanResult.
export interface RemoteScanJob {
  job_id: string;
  status: 'queued' | 'running' | 'complete' | 'failed';
  result?: ProjectScanResult;
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface Advisory {
  package: {
    ecosystem: string;
    name: string;
    version: string;
  };
  severity: Severity;
  confidence: number;
  advisory: string;
  recommended_action: string;
  agentic_risk?: string;
  patch_suggestion?: string;
  findings: Finding[];
  generated_at: string;
}

export interface DetectionSignature {
  id: string;
  type: string;
  ecosystem: string;
  target?: string;
  pattern?: string;
  package?: string;
  rule?: string;
  severity: string;
  title: string;
  description: string;
  source: string;
  cve?: string;
  created_at: string;
}

export interface DashboardStats {
  total_packages: number;
  total_versions: number;
  total_findings: number;
  critical_findings: number;
  high_findings: number;
  scanned_today: number;
  ecosystems_covered: string[];
  last_updated: string;
}

export interface RecentResult {
  package: string;
  version: string;
  ecosystem: string;
  severity: Severity;
  findings_count: number;
  scanned_at: string;
}

export interface PackageInfo {
  ecosystem: string;
  name: string;
  versions: string[];
  latest_version?: string;
}

export interface Attestation {
  package: { ecosystem: string; name: string; version: string };
  sha256: string;
  signature: string;
  public_key: string;
  rekor_log_id?: string;
  rekor_index?: number;
  rekor_url: string;
  signed_at: string;
}

export interface VerifyResult {
  valid: boolean;
  sha256_match: boolean;
  rekor_verified: boolean;
  error?: string;
}

export interface RiskScore {
  overall: number;
  grade: 'A' | 'B' | 'C' | 'D' | 'F';
  factors: {
    vulnerability: number;
    behavioral: number;
    supply_chain: number;
    maintenance: number;
  };
}

export interface ActivityEvent {
  id: string;
  type: 'scan_complete' | 'finding_critical' | 'policy_violation' | 'signature_match' | 'new_package';
  message: string;
  severity: Severity;
  package_name?: string;
  ecosystem?: string;
  occurred_at: string;
}

export interface RiskItem {
  package_name: string;
  version: string;
  ecosystem: string;
  risk_grade: string;
  risk_score: number;
  finding_count: number;
  top_severity: string;
  first_seen: string;
}

export interface AllowlistEntry {
  id: number;
  ecosystem: string;
  package: string;
  reason: string;
  added_by: string;
  created_at: string;
}

export interface AlertItem {
  id: number;
  type: string;
  message: string;
  severity: Severity;
  package_name?: string;
  ecosystem?: string;
  version?: string;
  dismissed: boolean;
  occurred_at: string;
}

export interface Policy {
  version: number;
  fail_on: string;
  deny_packages: string[];
  allow_licenses: string[];
  max_package_age_days: number;
  block_typosquatting: boolean;
  block_abandoned: boolean;
  require_signing: boolean;
}

export interface PolicyStatus {
  configured: boolean;
  violations: number | null;
  last_evaluated: string;
  policy: Policy;
}

// Provenance — SLSA v1 in-toto statement. The shape is stable but deeply
// nested and only ever rendered as pretty-printed JSON, so we keep it loose.
export type Provenance = Record<string, unknown>;

export type SignatureType =
  | 'blocklisted_package'
  | 'typosquatting_target'
  | 'behavioral_rule'
  | 'malware_pattern'
  | 'mcp_injection_pattern'
  | 'pickle_rule';

export interface SignatureInput {
  type: SignatureType;
  ecosystem: string;
  severity: string;
  name: string;
  description: string;
  author?: string;
  cve?: string;
  references?: string[];
  package?: string;
  target?: string;
  pattern?: string;
  rule?: string;
  tags?: string[];
}

export interface GeneratedSignature {
  id: string;
  yaml: string;
  filename: string;
  suggested_path: string;
}

export interface ValidationResult {
  valid: boolean;
  problems: string[];
}

export interface SignatureTestResult {
  matched: boolean;
  findings: Finding[];
  note: string;
}

export interface AuditStats {
  total_signatures: number;
  malware_patterns: number;
  typosquat_targets: number;
  behavioral_rules: number;
  blocklist_entries: number;
  mcp_injection_rules: number;
  ecosystems_covered: string[];
  signatures_updated: string;
}

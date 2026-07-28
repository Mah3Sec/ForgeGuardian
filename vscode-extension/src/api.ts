import * as vscode from 'vscode';
import * as https from 'https';
import * as http from 'http';
import * as url from 'url';

function cfg() {
  return vscode.workspace.getConfiguration('forgeguardian');
}

export function apiUrl(): string {
  return (cfg().get<string>('apiUrl') ?? 'http://localhost:8080').replace(/\/$/, '');
}

export interface ScanSummary {
  critical: number; high: number; medium: number; low: number;
  informational: number; total: number; highest_sev: string;
}

export interface Finding {
  id: string; severity: string; type: string;
  title: string; description: string; source: string;
}

export interface ScanResult {
  package: string;
  summary: ScanSummary;
  findings: Finding[];
  error?: string;
}

export interface Advisory {
  package: { ecosystem: string; name: string; version: string };
  severity: string; confidence: number;
  advisory: string; recommended_action: string;
  agentic_risk?: string;
}

export interface Signature {
  id: string; type: string; ecosystem: string;
  target?: string; package?: string; rule?: string;
  severity: string; title: string; source: string; cve?: string;
}

async function request<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const base = apiUrl();
  const parsed = url.parse(base + path);
  const isHTTPS = parsed.protocol === 'https:';
  const transport = isHTTPS ? https : http;

  return new Promise<T>((resolve, reject) => {
    const payload = body ? JSON.stringify(body) : undefined;
    const options: http.RequestOptions = {
      hostname: parsed.hostname,
      port: parsed.port ?? (isHTTPS ? 443 : 80),
      path: parsed.path,
      method,
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json',
        ...(payload ? { 'Content-Length': Buffer.byteLength(payload) } : {}),
      },
    };
    const req = transport.request(options, (res) => {
      let data = '';
      res.on('data', chunk => data += chunk);
      res.on('end', () => {
        try { resolve(JSON.parse(data) as T); }
        catch { reject(new Error(`JSON parse error: ${data.slice(0, 200)}`)); }
      });
    });
    req.on('error', reject);
    if (payload) req.write(payload);
    req.end();
  });
}

export function scanPackage(ecosystem: string, pkg: string, version: string) {
  return request<ScanResult>('/api/v1/scan', 'POST', { ecosystem, package: pkg, version });
}

export function generateAdvisory(ecosystem: string, pkg: string, version: string, findings: Finding[]) {
  return request<Advisory>('/api/v1/advisory', 'POST', { ecosystem, package: pkg, version, findings });
}

export function listSignatures(params?: { type?: string; ecosystem?: string }) {
  const q = new URLSearchParams();
  if (params?.type) q.set('type', params.type);
  if (params?.ecosystem) q.set('ecosystem', params.ecosystem);
  return request<{ total: number; signatures: Signature[] }>(`/api/v1/intelligence/signatures?${q}`);
}

export function refreshIntelligence() {
  return request<{ message: string }>('/api/v1/intelligence/refresh', 'POST');
}

export function healthz() {
  return request<{ status: string }>('/healthz');
}

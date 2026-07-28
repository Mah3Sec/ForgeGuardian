import * as vscode from 'vscode';
import * as api from './api';

const MANIFEST_FILES = new Set([
  'package.json',
  'requirements.txt',
  'go.mod',
  'Cargo.toml',
  'Gemfile',
  'pyproject.toml',
  'pom.xml',
]);

export function isManifestFile(fileName: string): boolean {
  const base = fileName.split('/').pop() ?? fileName;
  return MANIFEST_FILES.has(base);
}

export interface PackageRef {
  name: string;
  version: string;
  line: number;
  ecosystem: string;
}

export function parsePackagesFromDocument(doc: vscode.TextDocument): PackageRef[] {
  const base = doc.fileName.split('/').pop() ?? doc.fileName;
  const text = doc.getText();

  if (base === 'package.json') return parsePackageJSON(text);
  if (base === 'requirements.txt') return parseRequirements(text);
  if (base === 'go.mod') return parseGoMod(text);
  return [];
}

function parsePackageJSON(text: string): PackageRef[] {
  const refs: PackageRef[] = [];
  let parsed: Record<string, unknown>;
  try { parsed = JSON.parse(text); } catch { return refs; }

  const depSections = ['dependencies', 'devDependencies', 'peerDependencies'];
  const lines = text.split('\n');

  for (const section of depSections) {
    const deps = parsed[section] as Record<string, string> | undefined;
    if (!deps) continue;
    for (const [name, rawVer] of Object.entries(deps)) {
      const version = normalizeVersion(rawVer);
      if (!version) continue;
      const lineIdx = lines.findIndex(l => l.includes(`"${name}"`));
      refs.push({ name, version, line: Math.max(0, lineIdx), ecosystem: 'npm' });
    }
  }
  return refs;
}

function parseRequirements(text: string): PackageRef[] {
  const refs: PackageRef[] = [];
  const lines = text.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (!line || line.startsWith('#') || line.startsWith('-')) continue;
    const m = line.match(/^([A-Za-z0-9_.-]+)\s*[=<>!~]+\s*([^\s;#]+)/);
    if (m) {
      refs.push({ name: m[1], version: m[2].replace(/[^\d.]/g, ''), line: i, ecosystem: 'pypi' });
    }
  }
  return refs;
}

function parseGoMod(text: string): PackageRef[] {
  const refs: PackageRef[] = [];
  const lines = text.split('\n');
  let inRequire = false;
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    if (line.startsWith('require (')) { inRequire = true; continue; }
    if (inRequire && line === ')') { inRequire = false; continue; }
    const m = (inRequire || line.startsWith('require '))
      ? line.replace(/^require\s+/, '').match(/^([\w./\-]+)\s+(v[\d.]+[^\s]*)/)
      : null;
    if (m) {
      refs.push({ name: m[1], version: m[2].replace(/^v/, ''), line: i, ecosystem: 'go' });
    }
  }
  return refs;
}

function normalizeVersion(raw: string): string {
  return raw.replace(/^[^0-9]*/, '').split(/\s/)[0];
}

// Per-file debounce timers
const debounceMap = new Map<string, NodeJS.Timeout>();
const DEBOUNCE_MS = 500;

// Per-file scan cache: { findings[], fetchedAt }
interface CacheEntry { findings: api.Finding[]; fetchedAt: Date; pkgKey: string }
const scanCache = new Map<string, CacheEntry>();

export async function scanDocumentForDiagnostics(
  doc: vscode.TextDocument,
  collection: vscode.DiagnosticCollection
): Promise<void> {
  if (!isManifestFile(doc.fileName)) return;

  const key = doc.uri.toString();
  if (debounceMap.has(key)) clearTimeout(debounceMap.get(key)!);

  await new Promise<void>(resolve => {
    debounceMap.set(key, setTimeout(resolve, DEBOUNCE_MS));
  });
  debounceMap.delete(key);

  const refs = parsePackagesFromDocument(doc);
  if (refs.length === 0) {
    collection.delete(doc.uri);
    return;
  }

  const diagnostics: vscode.Diagnostic[] = [];

  for (const ref of refs) {
    const cacheKey = `${ref.ecosystem}/${ref.name}@${ref.version}`;
    let findings: api.Finding[];

    const cached = scanCache.get(cacheKey);
    const stale = !cached || (Date.now() - cached.fetchedAt.getTime()) > 5 * 60 * 1000;

    if (stale) {
      try {
        const result = await api.scanPackage(ref.ecosystem, ref.name, ref.version);
        findings = result.findings ?? [];
        scanCache.set(cacheKey, { findings, fetchedAt: new Date(), pkgKey: cacheKey });
      } catch {
        continue; // skip on API error — don't show stale diagnostics
      }
    } else {
      findings = cached.findings;
    }

    for (const finding of findings) {
      const range = new vscode.Range(ref.line, 0, ref.line, Number.MAX_SAFE_INTEGER);
      const diag = new vscode.Diagnostic(
        range,
        `${finding.title} — ${ref.name}@${ref.version} (${finding.source})`,
        severityToDiagnostic(finding.severity)
      );
      diag.source = 'ForgeGuardian';
      diag.code = finding.id;
      diagnostics.push(diag);
    }
  }

  collection.set(doc.uri, diagnostics);
}

function severityToDiagnostic(sev: string): vscode.DiagnosticSeverity {
  switch (sev.toUpperCase()) {
    case 'CRITICAL':
    case 'HIGH':   return vscode.DiagnosticSeverity.Error;
    case 'MEDIUM': return vscode.DiagnosticSeverity.Warning;
    default:       return vscode.DiagnosticSeverity.Information;
  }
}

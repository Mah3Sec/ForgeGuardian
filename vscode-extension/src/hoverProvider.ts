import * as vscode from 'vscode';
import * as api from './api';
import { parsePackagesFromDocument, PackageRef } from './diagnostics';

interface HoverCacheEntry {
  findings: api.Finding[];
  fetchedAt: Date;
}

const CACHE_TTL_MS = 5 * 60 * 1000;

export class PackageHoverProvider implements vscode.HoverProvider {
  private cache = new Map<string, HoverCacheEntry>();

  async provideHover(doc: vscode.TextDocument, position: vscode.Position): Promise<vscode.Hover | null> {
    const refs = parsePackagesFromDocument(doc);
    const ref = refs.find(r => r.line === position.line);
    if (!ref) return null;

    const cacheKey = `${ref.ecosystem}/${ref.name}@${ref.version}`;
    let findings: api.Finding[];

    const cached = this.cache.get(cacheKey);
    if (cached && Date.now() - cached.fetchedAt.getTime() < CACHE_TTL_MS) {
      findings = cached.findings;
    } else {
      try {
        const result = await api.scanPackage(ref.ecosystem, ref.name, ref.version);
        findings = result.findings ?? [];
        this.cache.set(cacheKey, { findings, fetchedAt: new Date() });
      } catch {
        return null;
      }
    }

    if (findings.length === 0) {
      return new vscode.Hover(new vscode.MarkdownString(
        `**ForgeGuardian** — \`${ref.name}@${ref.version}\`\n\n✅ No findings`
      ));
    }

    const top = findings
      .sort((a, b) => severityOrder(b.severity) - severityOrder(a.severity))
      .slice(0, 3);

    const lines: string[] = [
      `**ForgeGuardian** — \`${ref.name}@${ref.version}\`\n`,
      ...top.map(f => `- **[${f.severity}]** ${f.title} \`${f.id}\``),
    ];
    if (findings.length > 3) {
      lines.push(`\n…and ${findings.length - 3} more`);
    }
    lines.push(`\n[View Advisory](command:forgeguardian.generateAdvisory)`);

    const md = new vscode.MarkdownString(lines.join('\n'));
    md.isTrusted = true;
    return new vscode.Hover(md);
  }
}

function severityOrder(sev: string): number {
  switch (sev.toUpperCase()) {
    case 'CRITICAL': return 4;
    case 'HIGH':     return 3;
    case 'MEDIUM':   return 2;
    case 'LOW':      return 1;
    default:         return 0;
  }
}

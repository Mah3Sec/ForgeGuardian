import * as vscode from 'vscode';
import { parsePackagesFromDocument, isManifestFile } from './diagnostics';

interface ScanState {
  total: number;
  findings: number;
  critical: number;
  high: number;
}

// Populated externally after a scan completes (key = uri + line)
const lineState = new Map<string, ScanState>();

export function setLineState(uri: string, line: number, state: ScanState): void {
  lineState.set(`${uri}:${line}`, state);
}

export function clearLineStates(uri: string): void {
  for (const key of lineState.keys()) {
    if (key.startsWith(uri + ':')) lineState.delete(key);
  }
}

export class PackageCodeLensProvider implements vscode.CodeLensProvider {
  private _onDidChangeCodeLenses = new vscode.EventEmitter<void>();
  readonly onDidChangeCodeLenses = this._onDidChangeCodeLenses.event;

  refresh(): void {
    this._onDidChangeCodeLenses.fire();
  }

  provideCodeLenses(doc: vscode.TextDocument): vscode.CodeLens[] {
    if (!isManifestFile(doc.fileName)) return [];

    const refs = parsePackagesFromDocument(doc);
    return refs.map(ref => {
      const range = new vscode.Range(ref.line, 0, ref.line, 0);
      const stateKey = `${doc.uri.toString()}:${ref.line}`;
      const state = lineState.get(stateKey);

      let title: string;
      if (!state) {
        title = `\u{1F6E1} ForgeGuardian: Click to scan`;
      } else if (state.findings === 0) {
        title = `✓ No findings`;
      } else {
        const parts: string[] = [];
        if (state.critical) parts.push(`${state.critical} CRITICAL`);
        if (state.high) parts.push(`${state.high} HIGH`);
        if (!parts.length) parts.push(`${state.findings} findings`);
        title = `⚠ ${parts.join(', ')}`;
      }

      return new vscode.CodeLens(range, {
        title,
        command: 'forgeguardian.scanCurrentFile',
        arguments: [doc.uri, ref],
      });
    });
  }
}

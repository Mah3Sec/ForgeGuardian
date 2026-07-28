import * as vscode from 'vscode';
import type { Signature } from './api';

export class SignaturesProvider implements vscode.TreeDataProvider<SignatureItem> {
  private _onDidChange = new vscode.EventEmitter<SignatureItem | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private signatures: Signature[] = [];

  setSignatures(sigs: Signature[]) {
    this.signatures = sigs;
    this._onDidChange.fire(undefined);
  }

  getTreeItem(element: SignatureItem): vscode.TreeItem {
    return element;
  }

  getChildren(element?: SignatureItem): SignatureItem[] {
    if (element) return [];
    return this.signatures.map(s => new SignatureItem(s));
  }
}

export class SignatureItem extends vscode.TreeItem {
  constructor(public readonly sig: Signature) {
    const label = sig.package ?? sig.target ?? sig.rule?.slice(0, 30) ?? sig.id;
    super(label, vscode.TreeItemCollapsibleState.None);
    this.description = `[${sig.ecosystem}] ${sig.type}`;
    this.tooltip = sig.title;
    const iconName = sig.severity === 'critical' ? 'error'
      : sig.severity === 'high' ? 'warning' : 'info';
    this.iconPath = new vscode.ThemeIcon(iconName);
    this.contextValue = 'signature';
  }
}

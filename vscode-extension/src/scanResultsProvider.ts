import * as vscode from 'vscode';
import type { ScanResult, Finding } from './api';

export class ScanResultsProvider implements vscode.TreeDataProvider<ScanTreeItem> {
  private _onDidChange = new vscode.EventEmitter<ScanTreeItem | undefined>();
  readonly onDidChangeTreeData = this._onDidChange.event;

  private results: Map<string, ScanResult> = new Map();

  addResult(result: ScanResult) {
    this.results.set(result.package, result);
    this._onDidChange.fire(undefined);
  }

  clear() {
    this.results.clear();
    this._onDidChange.fire(undefined);
  }

  getTreeItem(element: ScanTreeItem): vscode.TreeItem {
    return element;
  }

  getChildren(element?: ScanTreeItem): ScanTreeItem[] {
    if (!element) {
      return Array.from(this.results.values()).map(r => new PackageItem(r));
    }
    if (element instanceof PackageItem) {
      return element.result.findings.map(f => new FindingItem(f));
    }
    return [];
  }
}

export type ScanTreeItem = PackageItem | FindingItem;

export class PackageItem extends vscode.TreeItem {
  constructor(public readonly result: ScanResult) {
    super(result.package, vscode.TreeItemCollapsibleState.Collapsed);
    const s = result.summary;
    this.description = s.total > 0
      ? `${s.critical}C ${s.high}H ${s.medium}M ${s.low}L`
      : 'Clean';
    this.iconPath = s.critical > 0
      ? new vscode.ThemeIcon('error', new vscode.ThemeColor('charts.red'))
      : s.high > 0
      ? new vscode.ThemeIcon('warning', new vscode.ThemeColor('charts.orange'))
      : new vscode.ThemeIcon('pass', new vscode.ThemeColor('charts.green'));
    this.tooltip = `Total: ${s.total} findings`;
    this.contextValue = 'packageResult';
  }
}

export class FindingItem extends vscode.TreeItem {
  constructor(public readonly finding: Finding) {
    super(finding.title, vscode.TreeItemCollapsibleState.None);
    this.description = finding.id;
    this.tooltip = finding.description;
    const iconName = finding.severity === 'CRITICAL' || finding.severity === 'HIGH'
      ? 'error' : finding.severity === 'MEDIUM' ? 'warning' : 'info';
    this.iconPath = new vscode.ThemeIcon(iconName);
    this.contextValue = 'finding';
  }
}

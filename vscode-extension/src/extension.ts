import * as vscode from 'vscode';
import * as api from './api';
import { ScanResultsProvider } from './scanResultsProvider';
import { SignaturesProvider } from './signaturesProvider';
import { scanDocumentForDiagnostics, isManifestFile, parsePackagesFromDocument } from './diagnostics';
import { PackageHoverProvider } from './hoverProvider';
import { PackageCodeLensProvider, setLineState, clearLineStates } from './codeLensProvider';

const MANIFEST_LANG_SELECTORS: vscode.DocumentSelector = [
  { language: 'json' },
  { language: 'python' },
  { language: 'go' },
  { language: 'toml' },
  { language: 'ruby' },
];

export function activate(context: vscode.ExtensionContext) {
  const scanProvider = new ScanResultsProvider();
  const sigProvider = new SignaturesProvider();

  // ── Diagnostics ──────────────────────────────────────────────────────────
  const diagnostics = vscode.languages.createDiagnosticCollection('forgeguardian');
  context.subscriptions.push(diagnostics);

  // ── Hover + CodeLens ─────────────────────────────────────────────────────
  const hoverProvider = new PackageHoverProvider();
  const codeLensProvider = new PackageCodeLensProvider();

  context.subscriptions.push(
    vscode.languages.registerHoverProvider(MANIFEST_LANG_SELECTORS, hoverProvider),
    vscode.languages.registerCodeLensProvider(MANIFEST_LANG_SELECTORS, codeLensProvider),
  );

  // ── Auto-scan on save ────────────────────────────────────────────────────
  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async doc => {
      const enabled = vscode.workspace.getConfiguration('forgeguardian').get<boolean>('autoScanOnSave');
      if (enabled && isManifestFile(doc.fileName)) {
        clearLineStates(doc.uri.toString());
        await scanDocumentForDiagnostics(doc, diagnostics);
        codeLensProvider.refresh();
      }
    })
  );

  context.subscriptions.push(
    vscode.window.registerTreeDataProvider('forgeguardian.scanResults', scanProvider),
    vscode.window.registerTreeDataProvider('forgeguardian.signatures', sigProvider),
  );

  // Preflight check
  checkApiConnectivity();

  // ── Commands ──────────────────────────────────────────────────────────────

  context.subscriptions.push(
    vscode.commands.registerCommand('forgeguardian.scanPackage', async () => {
      const input = await promptPackageInput();
      if (!input) return;
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title: `Scanning ${input.pkg}@${input.version}…`, cancellable: false },
        async () => {
          try {
            const result = await api.scanPackage(input.ecosystem, input.pkg, input.version);
            scanProvider.addResult(result);
            const { summary } = result;
            const msg = summary.total === 0
              ? `✅ ${input.pkg}@${input.version} — Clean`
              : `⚠️ ${input.pkg}@${input.version} — ${summary.critical}C ${summary.high}H ${summary.medium}M ${summary.low}L`;
            const action = summary.critical > 0 || summary.high > 0 ? 'View Advisory' : undefined;
            const choice = action
              ? await vscode.window.showWarningMessage(msg, action)
              : vscode.window.showInformationMessage(msg);
            if (choice === 'View Advisory') {
              await vscode.commands.executeCommand('forgeguardian.generateAdvisory');
            }
          } catch (e) {
            vscode.window.showErrorMessage(`Scan failed: ${(e as Error).message}`);
          }
        }
      );
    }),

    vscode.commands.registerCommand('forgeguardian.generateAdvisory', async () => {
      // AI Advisory is a Pro feature
      const action = await vscode.window.showInformationMessage(
        '🔒 AI Advisory is a ForgeGuardian Pro feature — coming soon.',
        'Join Pro Waitlist',
        'Learn More'
      );
      if (action === 'Join Pro Waitlist' || action === 'Learn More') {
        vscode.env.openExternal(vscode.Uri.parse('https://forgeguardian.dev/pro'));
      }
    }),

    vscode.commands.registerCommand('forgeguardian.generateSBOM', async () => {
      const input = await promptPackageInput();
      if (!input) return;
      const formats = ['cyclonedx-json', 'cyclonedx-xml', 'spdx-json', 'spdx-tv'];
      const format = await vscode.window.showQuickPick(formats, { placeHolder: 'Select SBOM format' });
      if (!format) return;
      const sbomUrl = `${api.apiUrl()}/api/v1/sbom/${input.ecosystem}/${input.pkg}/${input.version}?format=${format}`;
      vscode.env.openExternal(vscode.Uri.parse(sbomUrl));
    }),

    vscode.commands.registerCommand('forgeguardian.verifyAttestation', async () => {
      const attPath = await vscode.window.showOpenDialog({
        canSelectFiles: true,
        filters: { 'Attestation JSON': ['json'] },
        openLabel: 'Select Attestation',
      });
      if (!attPath?.[0]) return;
      const sha256 = await vscode.window.showInputBox({ prompt: 'Enter expected SHA256 of artifact' });
      if (!sha256) return;
      const dashUrl = `${api.apiUrl()}/api/v1/verify`;
      vscode.window.showInformationMessage(`Verify via API: POST ${dashUrl} with attestation + sha256`);
    }),

    vscode.commands.registerCommand('forgeguardian.openDashboard', () => {
      vscode.env.openExternal(vscode.Uri.parse(api.apiUrl().replace(':8080', ':5173')));
    }),

    vscode.commands.registerCommand('forgeguardian.scanCurrentFile', async () => {
      const doc = vscode.window.activeTextEditor?.document;
      if (!doc || !isManifestFile(doc.fileName)) {
        vscode.window.showInformationMessage('Open a package.json, requirements.txt, or go.mod to scan.');
        return;
      }
      clearLineStates(doc.uri.toString());
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Window, title: 'ForgeGuardian: Scanning…' },
        async () => {
          await scanDocumentForDiagnostics(doc, diagnostics);
          // Populate CodeLens state from diagnostic results
          const refs = parsePackagesFromDocument(doc);
          const fileDiags = diagnostics.get(doc.uri) ?? [];
          for (const ref of refs) {
            const refDiags = fileDiags.filter(d => d.range.start.line === ref.line);
            setLineState(doc.uri.toString(), ref.line, {
              total: refDiags.length,
              findings: refDiags.length,
              critical: refDiags.filter(d => d.severity === vscode.DiagnosticSeverity.Error && d.message.includes('CRITICAL')).length,
              high:     refDiags.filter(d => d.severity === vscode.DiagnosticSeverity.Error && d.message.includes('HIGH')).length,
            });
          }
          codeLensProvider.refresh();
        }
      );
    }),

    vscode.commands.registerCommand('forgeguardian.refreshSignatures', async () => {
      await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title: 'Refreshing intelligence signatures…' },
        async () => {
          try {
            await api.refreshIntelligence();
            const data = await api.listSignatures();
            sigProvider.setSignatures(data.signatures);
            vscode.window.showInformationMessage(`Intelligence updated — ${data.total} signatures loaded`);
          } catch (e) {
            vscode.window.showErrorMessage(`Refresh failed: ${(e as Error).message}`);
          }
        }
      );
    }),
  );

  // Load signatures on activation
  loadSignatures(sigProvider);
}

async function promptPackageInput() {
  const ecosystem = await vscode.window.showQuickPick(
    ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'],
    { placeHolder: 'Select ecosystem' }
  );
  if (!ecosystem) return null;
  const pkg = await vscode.window.showInputBox({ prompt: 'Package name', placeHolder: 'e.g. lodash' });
  if (!pkg) return null;
  const version = await vscode.window.showInputBox({ prompt: 'Version', placeHolder: 'e.g. 4.17.21' });
  if (!version) return null;
  return { ecosystem, pkg, version };
}

function showAdvisoryPanel(advisory: api.Advisory, context: vscode.ExtensionContext) {
  const panel = vscode.window.createWebviewPanel(
    'forgeguardianAdvisory',
    `Advisory: ${advisory.package.name}@${advisory.package.version}`,
    vscode.ViewColumn.Two,
    { enableScripts: false }
  );

  const sev = advisory.severity;
  const sevColor = sev === 'CRITICAL' ? '#FF3D3D' : sev === 'HIGH' ? '#FFAB40' : '#00FF87';

  panel.webview.html = `<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<style>
  body { font-family: -apple-system, sans-serif; padding: 20px; background: #0A0B0D; color: #F8F9FA; }
  h1 { font-family: monospace; color: #00FF87; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-weight: bold;
           background: ${sevColor}22; color: ${sevColor}; border: 1px solid ${sevColor}44; }
  .section { background: #141618; border-radius: 8px; padding: 16px; margin: 12px 0; border: 1px solid rgba(255,255,255,0.06); }
  .label { font-size: 11px; font-family: monospace; color: #6B7280; text-transform: uppercase; letter-spacing: 0.1em; margin-bottom: 8px; }
  p { line-height: 1.6; color: #d1d5db; }
  .action { background: rgba(0,255,135,0.05); border: 1px solid rgba(0,255,135,0.15); }
  .action p { color: #F8F9FA; }
</style>
</head>
<body>
<h1>${advisory.package.name} @ ${advisory.package.version}</h1>
<p><span class="badge">${advisory.severity}</span> &nbsp; Confidence: ${Math.round(advisory.confidence * 100)}%</p>

<div class="section">
  <div class="label">Advisory</div>
  <p>${advisory.advisory}</p>
</div>

<div class="section action">
  <div class="label" style="color:#00FF87">Recommended Action</div>
  <p>${advisory.recommended_action}</p>
</div>

${advisory.agentic_risk ? `<div class="section">
  <div class="label" style="color:#FFAB40">Agentic Attack Surface</div>
  <p>${advisory.agentic_risk}</p>
</div>` : ''}
</body>
</html>`;
}

async function checkApiConnectivity() {
  try {
    await api.healthz();
  } catch {
    vscode.window.showWarningMessage(
      'ForgeGuardian API not reachable at ' + api.apiUrl() + '. Start the server or update the URL in settings.',
      'Open Settings'
    ).then(choice => {
      if (choice === 'Open Settings') {
        vscode.commands.executeCommand('workbench.action.openSettings', 'forgeguardian.apiUrl');
      }
    });
  }
}

async function loadSignatures(provider: SignaturesProvider) {
  try {
    const data = await api.listSignatures();
    provider.setSignatures(data.signatures);
  } catch {
    // API may not be running yet — silently skip
  }
}

export function deactivate() {}

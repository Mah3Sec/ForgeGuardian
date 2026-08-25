import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { generateSignature, validateSignatureYaml, testSignature } from '../lib/api';
import { PenTool, CheckCircle2, XCircle, AlertCircle, FlaskConical } from 'lucide-react';
import type { SignatureType, GeneratedSignature } from '../types/api';

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp', '*'];
const SEVERITIES = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'INFORMATIONAL'];

const SIG_TYPES: { value: SignatureType; label: string; description: string }[] = [
  { value: 'blocklisted_package', label: 'blocklisted_package', description: 'A specific package version confirmed malicious' },
  { value: 'typosquatting_target', label: 'typosquatting_target', description: 'A popular package that attackers misspell to hijack' },
  { value: 'behavioral_rule', label: 'behavioral_rule', description: 'A dangerous install script pattern (regex)' },
  { value: 'malware_pattern', label: 'malware_pattern', description: 'A byte/regex pattern found in malware (regex)' },
  { value: 'mcp_injection_pattern', label: 'mcp_injection_pattern', description: 'A prompt injection pattern in MCP tool descriptions (regex)' },
  { value: 'pickle_rule', label: 'pickle_rule', description: 'An unsafe AI model configuration pattern' },
];

const inputStyle = {
  background: 'var(--bg-base)', color: 'var(--fg)', border: '1px solid rgba(255,255,255,0.12)',
};

function extraFieldLabel(type: SignatureType): { label: string; placeholder: string; multiline?: boolean } {
  switch (type) {
    case 'blocklisted_package': return { label: 'PACKAGE NAME', placeholder: 'evil-pkg' };
    case 'typosquatting_target': return { label: 'TARGET PACKAGE', placeholder: 'lodash' };
    case 'malware_pattern': return { label: 'REGEX PATTERN', placeholder: '(?i)eval\\(atob\\(' };
    case 'mcp_injection_pattern': return { label: 'REGEX PATTERN', placeholder: '(?i)ignore (all )?previous instructions' };
    case 'behavioral_rule': return { label: 'RULE TEXT', placeholder: 'postinstall script downloads and executes remote binary', multiline: true };
    case 'pickle_rule': return { label: 'RULE TEXT', placeholder: '__reduce__ used to invoke os.system', multiline: true };
  }
}

export function IntelAuthoringPage() {
  const [type, setType] = useState<SignatureType>('blocklisted_package');
  const [ecosystem, setEcosystem] = useState('npm');
  const [severity, setSeverity] = useState('CRITICAL');
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [author, setAuthor] = useState('');
  const [cve, setCve] = useState('');
  const [reference, setReference] = useState('');
  const [extraValue, setExtraValue] = useState('');

  const [generated, setGenerated] = useState<GeneratedSignature | null>(null);
  const [yamlText, setYamlText] = useState('');

  const [testEco, setTestEco] = useState('npm');
  const [testPkg, setTestPkg] = useState('');
  const [testVersion, setTestVersion] = useState('');

  const extraField = extraFieldLabel(type);

  const generate = useMutation({
    mutationFn: () =>
      generateSignature({
        type,
        ecosystem,
        severity,
        name,
        description,
        author: author || 'anonymous',
        cve,
        references: reference ? [reference] : [],
        package: type === 'blocklisted_package' ? extraValue : undefined,
        target: type === 'typosquatting_target' ? extraValue : undefined,
        pattern: type === 'malware_pattern' || type === 'mcp_injection_pattern' ? extraValue : undefined,
        rule: type === 'behavioral_rule' || type === 'pickle_rule' ? extraValue : undefined,
        tags: [],
      }),
    onSuccess: (data) => {
      setGenerated(data);
      setYamlText(data.yaml);
    },
  });

  const validate = useMutation({
    mutationFn: () => validateSignatureYaml(yamlText),
  });

  const test = useMutation({
    mutationFn: () => testSignature(yamlText, testEco, testPkg, testVersion),
  });

  const copyYAML = () => yamlText && navigator.clipboard.writeText(yamlText);

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>Signature Authoring</h1>
      <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
        Author, validate, and test new detection signatures for the ForgeGuardian intelligence store.
      </p>

      {/* Form */}
      <div className="rounded-lg p-5 space-y-4" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <h2 className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>NEW SIGNATURE</h2>

        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>TYPE</label>
          <select value={type} onChange={e => setType(e.target.value as SignatureType)}
            className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle}>
            {SIG_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
          </select>
          <p className="text-xs mt-1" style={{ color: 'var(--color-muted)' }}>
            {SIG_TYPES.find(t => t.value === type)?.description}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>ECOSYSTEM</label>
            <select value={ecosystem} onChange={e => setEcosystem(e.target.value)}
              className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle}>
              {ECOSYSTEMS.map(e => <option key={e}>{e}</option>)}
            </select>
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>SEVERITY</label>
            <select value={severity} onChange={e => setSeverity(e.target.value)}
              className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle}>
              {SEVERITIES.map(s => <option key={s}>{s}</option>)}
            </select>
          </div>
        </div>

        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>NAME</label>
          <input value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Backdoor in evil-pkg"
            className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
        </div>

        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>DESCRIPTION</label>
          <textarea rows={3} value={description} onChange={e => setDescription(e.target.value)}
            placeholder="What does it do / why is it dangerous?"
            className="w-full rounded px-3 py-2 text-sm font-mono resize-y" style={inputStyle} />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>AUTHOR</label>
            <input value={author} onChange={e => setAuthor(e.target.value)} placeholder="anonymous"
              className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
          </div>
          <div>
            <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>CVE (optional)</label>
            <input value={cve} onChange={e => setCve(e.target.value)} placeholder="CVE-YYYY-NNNNN"
              className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
          </div>
        </div>

        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>REFERENCE URL (optional)</label>
          <input value={reference} onChange={e => setReference(e.target.value)} placeholder="https://…"
            className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
        </div>

        <div>
          <label className="block text-xs font-mono mb-1" style={{ color: 'var(--color-muted)' }}>{extraField.label}</label>
          {extraField.multiline ? (
            <textarea rows={2} value={extraValue} onChange={e => setExtraValue(e.target.value)}
              placeholder={extraField.placeholder}
              className="w-full rounded px-3 py-2 text-sm font-mono resize-y" style={inputStyle} />
          ) : (
            <input value={extraValue} onChange={e => setExtraValue(e.target.value)}
              placeholder={extraField.placeholder}
              className="w-full rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
          )}
        </div>

        <button onClick={() => generate.mutate()} disabled={!name || !description || !extraValue || generate.isPending}
          className="flex items-center gap-2 px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
          style={{ background: 'var(--color-safe)', color: '#0A0B0D' }}>
          <PenTool size={14} />{generate.isPending ? 'Generating…' : 'Generate Signature'}
        </button>
        {generate.isError && (
          <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-critical)' }}>
            <AlertCircle size={14} />{(generate.error as Error).message}
          </div>
        )}
      </div>

      {/* Generated YAML */}
      {generated && (
        <div className="rounded-lg" style={{ background: 'var(--surface)', border: '1px solid rgba(0,255,135,0.2)' }}>
          <div className="flex items-center justify-between px-4 py-3" style={{ borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
            <div className="flex items-center gap-2">
              <CheckCircle2 size={14} color="var(--color-safe)" />
              <span className="text-xs font-mono" style={{ color: 'var(--color-safe)' }}>{generated.id}</span>
            </div>
            <button onClick={copyYAML} className="text-xs px-2 py-1 rounded font-mono"
              style={{ background: 'rgba(0,255,135,0.1)', color: 'var(--color-safe)' }}>Copy YAML</button>
          </div>
          <pre className="p-4 text-xs overflow-auto max-h-72 font-mono" style={{ background: 'var(--bg-elevated)', color: 'var(--fg)' }}>
            {yamlText}
          </pre>
          <div className="px-4 py-2 text-xs" style={{ color: 'var(--color-muted)', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
            Suggested path: <code style={{ color: 'var(--color-safe)' }}>{generated.suggested_path}</code>
          </div>
        </div>
      )}

      {/* Validate */}
      {generated && (
        <div className="rounded-lg p-5 space-y-3" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <h2 className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>VALIDATE</h2>
          <button onClick={() => validate.mutate()} disabled={!yamlText || validate.isPending}
            className="px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
            style={{ background: 'rgba(255,255,255,0.1)', color: 'var(--fg)' }}>
            {validate.isPending ? 'Validating…' : 'Validate'}
          </button>
          {validate.data && (
            <div className="flex items-start gap-2 p-3 rounded text-sm" style={{
              background: validate.data.valid ? 'rgba(0,255,135,0.1)' : 'rgba(255,61,61,0.1)',
              color: validate.data.valid ? 'var(--color-safe)' : 'var(--color-critical)',
              border: `1px solid ${validate.data.valid ? 'rgba(0,255,135,0.2)' : 'rgba(255,61,61,0.2)'}`,
            }}>
              {validate.data.valid ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
              {validate.data.valid ? (
                'Valid'
              ) : (
                <ul className="list-disc list-inside">
                  {validate.data.problems.map(p => <li key={p}>{p}</li>)}
                </ul>
              )}
            </div>
          )}
          {validate.isError && (
            <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-critical)' }}>
              <AlertCircle size={14} />{(validate.error as Error).message}
            </div>
          )}
        </div>
      )}

      {/* Test against a live package */}
      {generated && (
        <div className="rounded-lg p-5 space-y-3" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
          <h2 className="text-sm font-mono" style={{ color: 'var(--color-muted)' }}>TEST AGAINST A LIVE PACKAGE</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
            <select value={testEco} onChange={e => setTestEco(e.target.value)}
              className="rounded px-3 py-2 text-sm font-mono" style={inputStyle}>
              {ECOSYSTEMS.filter(e => e !== '*').map(e => <option key={e}>{e}</option>)}
            </select>
            <input value={testPkg} onChange={e => setTestPkg(e.target.value)} placeholder="package"
              className="rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
            <input value={testVersion} onChange={e => setTestVersion(e.target.value)} placeholder="version"
              className="rounded px-3 py-2 text-sm font-mono" style={inputStyle} />
          </div>
          <button onClick={() => test.mutate()} disabled={!testPkg || !testVersion || test.isPending}
            className="flex items-center gap-2 px-4 py-2 rounded text-sm font-mono font-bold disabled:opacity-50"
            style={{ background: 'rgba(255,255,255,0.1)', color: 'var(--fg)' }}>
            <FlaskConical size={14} />{test.isPending ? 'Testing…' : 'Test Signature'}
          </button>
          {test.data && (
            <div className="p-3 rounded text-sm" style={{
              background: test.data.matched ? 'rgba(255,171,64,0.1)' : 'rgba(255,255,255,0.05)',
              color: test.data.matched ? 'var(--color-warn)' : 'var(--color-muted)',
              border: `1px solid ${test.data.matched ? 'rgba(255,171,64,0.25)' : 'rgba(255,255,255,0.08)'}`,
            }}>
              <p>{test.data.matched ? `Matched — ${test.data.findings.length} finding(s)` : 'No match'}</p>
              <p className="text-xs mt-1" style={{ color: 'var(--color-muted)' }}>{test.data.note}</p>
            </div>
          )}
          {test.isError && (
            <div className="flex items-center gap-2 text-sm" style={{ color: 'var(--color-critical)' }}>
              <AlertCircle size={14} />{(test.error as Error).message}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

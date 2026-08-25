import { useMemo, useRef, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import {
  ArrowRight, Search, Upload, Terminal, ShieldCheck, AlertCircle, Loader,
} from 'lucide-react';
import { triggerScan, scanUpload, getJobStatus } from '../lib/api';
import { ProgressScan, type ProgressScanStatus } from '../components/ProgressScan';
import { SecurityScore, computeSecurityScore } from '../components/SecurityScore';
import { CopyButton } from '../components/CopyButton';
import { Input } from '../components/ui/input';
import { Button } from '../components/ui/button';
import { Card } from '../components/ui/card';

const ECOSYSTEMS = ['npm', 'pypi', 'go', 'rubygems', 'crates', 'maven', 'huggingface', 'mcp'];

type ConnectOption = 'registry' | 'upload' | 'cli';

interface OnboardingPageProps {
  onComplete: () => void;
}

// Cosmetic one-time onboarding shown after first successful login. The
// workspace name (step 1) and "has onboarded" flag are BOTH client-side
// localStorage only — there is no backend concept of a workspace or an
// onboarded account property. Steps 2-4 wire into the real scan pipeline
// (triggerScan/scanUpload + getJobStatus polling), the same pattern used in
// ScanPage.tsx, so the security score reveal in step 4 is computed from a
// genuine ScanResult whenever the user actually ran a scan through the UI.
export function OnboardingPage({ onComplete }: OnboardingPageProps) {
  const [step, setStep] = useState(1);
  const [workspaceName, setWorkspaceName] = useState('');
  const [connectOption, setConnectOption] = useState<ConnectOption | null>(null);

  // Registry scan state
  const [ecosystem, setEcosystem] = useState('npm');
  const [pkg, setPkg] = useState('');
  const [version, setVersion] = useState('');

  // Upload scan state
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const [jobId, setJobId] = useState<string | null>(null);

  const registryScan = useMutation({
    mutationFn: () => triggerScan(ecosystem, pkg, version),
    onSuccess: (job) => setJobId(job.job_id),
  });

  const uploadScan = useMutation({
    mutationFn: () => scanUpload(uploadFile!, uploadFile!.name),
    onSuccess: (job) => setJobId(job.job_id),
  });

  const jobPoll = useQuery({
    queryKey: ['onboarding-scan-job', jobId],
    queryFn: () => getJobStatus(jobId!),
    enabled: !!jobId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      return status === 'complete' || status === 'failed' ? false : 2_000;
    },
  });

  const job = jobPoll.data;
  const result = job?.status === 'complete' ? job.result ?? null : null;
  const isPending = registryScan.isPending || uploadScan.isPending
    || (!!jobId && job?.status !== 'complete' && job?.status !== 'failed');
  const error = registryScan.error || uploadScan.error
    || (job?.status === 'failed' ? new Error(job.error || 'scan failed') : undefined);

  const progressStatus: ProgressScanStatus = job?.status ?? (isPending ? 'running' : 'queued');

  const score = useMemo(() => (result ? computeSecurityScore(result.summary) : null), [result]);

  const finish = () => {
    localStorage.setItem('fg_workspace_name', workspaceName);
    onComplete();
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const f = e.dataTransfer.files[0];
    if (f) setUploadFile(f);
  };

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg-base px-6 py-12">
      <div className="w-full max-w-lg">
        {/* Step indicator */}
        <div className="mb-8 flex items-center justify-center gap-2">
          {[1, 2, 3, 4].map(n => (
            <div
              key={n}
              className="h-1.5 w-10 rounded-full"
              style={{ background: n <= step ? 'var(--primary-blue)' : 'var(--border-color)' }}
            />
          ))}
        </div>

        <Card className="p-8">
          {/* Step 1: Workspace name */}
          {step === 1 && (
            <div>
              <h1 className="text-xl font-bold text-text-primary">Name your workspace</h1>
              <p className="mt-2 text-sm text-text-secondary">
                A personal label shown in this browser only — it isn't sent anywhere or tied to
                your account.
              </p>
              <Input
                value={workspaceName}
                onChange={e => setWorkspaceName(e.target.value)}
                placeholder="e.g. My Team"
                className="mt-6"
                autoFocus
                onKeyDown={e => e.key === 'Enter' && workspaceName.trim() && setStep(2)}
              />
              <Button
                className="mt-6 w-full"
                disabled={!workspaceName.trim()}
                onClick={() => setStep(2)}
              >
                Continue <ArrowRight size={14} />
              </Button>
            </div>
          )}

          {/* Step 2: Connect environment */}
          {step === 2 && (
            <div>
              <h1 className="text-xl font-bold text-text-primary">Connect an environment</h1>
              <p className="mt-2 text-sm text-text-secondary">
                Pick how you want to run your first scan.
              </p>
              <div className="mt-6 flex flex-col gap-3">
                {([
                  { id: 'registry' as const, icon: Search, label: 'Scan a registry package', desc: 'Pull a real package from npm, PyPI, Go, and more.' },
                  { id: 'upload' as const, icon: Upload, label: 'Upload a project archive', desc: 'Drag in a .tar.gz, .zip, or single package file.' },
                  { id: 'cli' as const, icon: Terminal, label: 'Use the CLI', desc: 'Run fgctl locally against any project.' },
                ]).map(opt => (
                  <button
                    key={opt.id}
                    onClick={() => setConnectOption(opt.id)}
                    className={
                      'flex items-start gap-3 rounded-lg border p-4 text-left transition-colors ' +
                      (connectOption === opt.id
                        ? 'border-primary-blue bg-blue-light/40'
                        : 'border-border-color bg-surface hover:bg-surface-muted')
                    }
                  >
                    <opt.icon size={18} className="mt-0.5 shrink-0 text-primary-blue" />
                    <div>
                      <div className="text-sm font-semibold text-text-primary">{opt.label}</div>
                      <div className="mt-0.5 text-xs text-text-secondary">{opt.desc}</div>
                    </div>
                  </button>
                ))}
              </div>
              <Button
                className="mt-6 w-full"
                disabled={!connectOption}
                onClick={() => setStep(3)}
              >
                Continue <ArrowRight size={14} />
              </Button>
            </div>
          )}

          {/* Step 3: Initial scan */}
          {step === 3 && (
            <div>
              <h1 className="text-xl font-bold text-text-primary">Run your first scan</h1>

              {connectOption === 'registry' && (
                <div className="mt-6 space-y-4">
                  <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                    <select
                      value={ecosystem}
                      onChange={e => setEcosystem(e.target.value)}
                      className="rounded-md border border-border-color bg-background px-2 py-2 font-mono text-xs"
                    >
                      {ECOSYSTEMS.map(e => <option key={e} value={e}>{e}</option>)}
                    </select>
                    <Input value={pkg} onChange={e => setPkg(e.target.value)} placeholder="package" className="text-xs" />
                    <Input value={version} onChange={e => setVersion(e.target.value)} placeholder="version" className="text-xs" />
                  </div>
                  <Button
                    className="w-full"
                    disabled={!pkg || !version || isPending}
                    onClick={() => registryScan.mutate()}
                  >
                    {isPending ? <Loader size={14} className="animate-spin" /> : <Search size={14} />}
                    {isPending ? 'Scanning…' : 'Run scan'}
                  </Button>
                </div>
              )}

              {connectOption === 'upload' && (
                <div className="mt-6 space-y-4">
                  <div
                    onDragOver={e => { e.preventDefault(); setDragOver(true); }}
                    onDragLeave={() => setDragOver(false)}
                    onDrop={handleDrop}
                    onClick={() => fileRef.current?.click()}
                    className={
                      'cursor-pointer rounded-lg border-2 border-dashed p-6 text-center transition-colors ' +
                      (dragOver ? 'border-primary-blue bg-blue-light/30' : 'border-border-color')
                    }
                  >
                    <input
                      ref={fileRef}
                      type="file"
                      accept=".tar.gz,.tgz,.zip,.whl,.gem,.jar,.tar"
                      className="hidden"
                      onChange={e => { if (e.target.files?.[0]) setUploadFile(e.target.files[0]); }}
                    />
                    <Upload size={24} className="mx-auto mb-2 text-text-muted" />
                    {uploadFile ? (
                      <p className="text-sm font-medium text-text-primary">{uploadFile.name}</p>
                    ) : (
                      <p className="text-sm text-text-secondary">Drop archive here or click to browse</p>
                    )}
                  </div>
                  <Button
                    className="w-full"
                    disabled={!uploadFile || isPending}
                    onClick={() => uploadScan.mutate()}
                  >
                    {isPending ? <Loader size={14} className="animate-spin" /> : <Upload size={14} />}
                    {isPending ? 'Scanning…' : 'Scan uploaded file'}
                  </Button>
                </div>
              )}

              {connectOption === 'cli' && (
                <div className="mt-6 space-y-4">
                  <p className="text-sm text-text-secondary">
                    Run this from any project directory:
                  </p>
                  <div className="flex items-center justify-between gap-3 rounded-md border border-border-color bg-surface-muted px-4 py-3">
                    <code className="font-mono text-sm text-text-primary">fgctl scan .</code>
                    <CopyButton text="fgctl scan ." />
                  </div>
                  <p className="text-xs text-text-muted">
                    Results from CLI scans won't appear in this onboarding flow — head to the
                    dashboard to review them once the scan finishes.
                  </p>
                  <Button className="w-full" onClick={finish}>
                    Go to dashboard <ArrowRight size={14} />
                  </Button>
                </div>
              )}

              {connectOption !== 'cli' && (
                <>
                  {(isPending || job) && (
                    <div className="mt-6 rounded-lg border border-border-color p-4">
                      <ProgressScan status={progressStatus} />
                    </div>
                  )}
                  {error && (
                    <div className="mt-4 flex items-center gap-2 rounded-md border border-critical/20 bg-critical/10 px-3 py-2 text-xs text-critical">
                      <AlertCircle size={13} className="shrink-0" />
                      {(error as Error).message}
                    </div>
                  )}
                  {result && (
                    <Button className="mt-6 w-full" onClick={() => setStep(4)}>
                      See results <ArrowRight size={14} />
                    </Button>
                  )}
                </>
              )}
            </div>
          )}

          {/* Step 4: Security score reveal */}
          {step === 4 && result && score !== null && (
            <div className="text-center">
              <ShieldCheck size={28} className="mx-auto mb-3 text-success" />
              <h1 className="text-xl font-bold text-text-primary">Scan complete</h1>
              <div className="mt-6 flex justify-center">
                <SecurityScore score={score} size={140} />
              </div>
              <p className="mt-4 text-sm text-text-secondary">
                {result.summary.total} issue{result.summary.total === 1 ? '' : 's'} discovered in{' '}
                <span className="font-mono text-text-primary">{result.package}</span>.
              </p>
              <Button className="mt-8 w-full" onClick={finish}>
                Go to dashboard <ArrowRight size={14} />
              </Button>
            </div>
          )}
        </Card>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertCircle, Loader, ShieldCheck, Lock } from 'lucide-react';
import { login } from '../lib/api';
import { NetworkGraph } from './NetworkGraph';
import { TopoBackground } from './TopoBackground';
import { Input } from './ui/input';
import { Button } from './ui/button';

interface AuthPanelProps {
  onLoggedIn: () => void;
}

// Full-viewport split-screen auth experience: brand panel (left) + form
// panel (right). Reuses the exact `login()` call and error handling that
// existed in the old docked-card version — only the layout changed.
export function AuthPanel({ onLoggedIn }: AuthPanelProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const loginMutation = useMutation({
    mutationFn: () => login(email, password),
    onSuccess: () => onLoggedIn(),
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!email || !password || loginMutation.isPending) return;
    loginMutation.mutate();
  };

  return (
    <div className="grid min-h-[560px] grid-cols-1 overflow-hidden rounded-2xl border border-border-color bg-surface shadow-sm md:grid-cols-2">
      {/* Brand panel */}
      <div className="relative hidden flex-col justify-between overflow-hidden bg-[#0B0D0F] p-10 text-white md:flex">
        <TopoBackground className="text-white" opacity={0.05} lines={9} />
        <div className="pointer-events-none absolute inset-0">
          <NetworkGraph mode="ambient" opacity={0.18} width={640} height={560} />
        </div>
        <div className="relative z-10 flex items-center gap-2.5">
          <img src="/logo.png" alt="ForgeGuardian" className="h-8 drop-shadow-[0_0_8px_rgba(37,99,235,0.6)]" />
          <span className="font-mono text-lg font-bold">ForgeGuardian</span>
        </div>
        <div className="relative z-10">
          <p className="text-2xl font-semibold leading-snug">
            See threats before they become incidents.
          </p>
          <p className="mt-3 max-w-sm text-sm text-white/60">
            Local-first, AI-native software supply chain security across nine ecosystems —
            open-source at its core.
          </p>
        </div>
      </div>

      {/* Form panel */}
      <div className="flex flex-col justify-center p-8 md:p-12">
        <div className="mx-auto w-full max-w-sm">
          <div className="mb-6 flex items-center gap-2">
            <Lock size={16} className="text-success" />
            <h2 className="text-lg font-semibold text-text-primary">Sign in</h2>
          </div>
          <p className="mb-6 text-sm text-text-secondary">
            Administrator access to this ForgeGuardian instance.
          </p>

          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div>
              <label className="mb-1 block text-xs font-mono text-text-secondary">EMAIL</label>
              <Input
                type="email"
                value={email}
                onChange={e => setEmail(e.target.value)}
                placeholder="admin@example.com"
                autoComplete="email"
                required
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-mono text-text-secondary">PASSWORD</label>
              <Input
                type="password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                placeholder="••••••••"
                autoComplete="current-password"
                required
              />
            </div>

            {loginMutation.isError && (
              <div className="flex items-center gap-2 rounded-md border border-critical/20 bg-critical/10 px-3 py-2 text-xs text-critical">
                <AlertCircle size={13} className="shrink-0" />
                {(loginMutation.error as Error).message}
              </div>
            )}

            <Button
              type="submit"
              disabled={!email || !password || loginMutation.isPending}
              className="mt-1 bg-primary-blue font-mono font-bold text-white hover:bg-primary-blue/90"
            >
              {loginMutation.isPending ? <Loader size={14} className="animate-spin" /> : <ShieldCheck size={14} />}
              {loginMutation.isPending ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>

          <p className="mt-6 text-center text-[0.7rem] text-text-muted">
            Your workspace is protected with encrypted authentication.
          </p>
        </div>
      </div>
    </div>
  );
}

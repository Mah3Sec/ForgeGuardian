import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertCircle, Loader, KeyRound, ShieldAlert } from 'lucide-react';
import { changePassword } from '../lib/api';
import { Input } from './ui/input';
import { Button } from './ui/button';

interface ForcePasswordChangeProps {
  onChanged: () => void;
}

export function ForcePasswordChange({ onChanged }: ForcePasswordChangeProps) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const mutation = useMutation({
    mutationFn: () => changePassword(currentPassword, newPassword),
    onSuccess: () => onChanged(),
  });

  const mismatch = confirmPassword.length > 0 && newPassword !== confirmPassword;
  const tooShort = newPassword.length > 0 && newPassword.length < 8;
  const canSubmit =
    currentPassword.length > 0 &&
    newPassword.length >= 8 &&
    newPassword === confirmPassword &&
    !mutation.isPending;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    mutation.mutate();
  };

  return (
    <div
      className="flex min-h-screen items-center justify-center px-4"
      style={{ background: 'var(--bg-base)', color: 'var(--fg)' }}
    >
      <div className="w-full max-w-md rounded-2xl border border-border-color bg-surface p-8 shadow-sm">
        <div className="mb-2 flex items-center gap-2 text-warning">
          <ShieldAlert size={20} />
          <h2 className="text-lg font-semibold text-text-primary">
            Password Change Required
          </h2>
        </div>
        <p className="mb-6 text-sm text-text-secondary">
          You are using the default credentials. Please set a new password
          before continuing.
        </p>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div>
            <label className="mb-1 block text-xs font-mono text-text-secondary">
              CURRENT PASSWORD
            </label>
            <Input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              placeholder="••••••••"
              autoComplete="current-password"
              required
            />
          </div>

          <div>
            <label className="mb-1 block text-xs font-mono text-text-secondary">
              NEW PASSWORD
            </label>
            <Input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              placeholder="Minimum 8 characters"
              autoComplete="new-password"
              required
            />
            {tooShort && (
              <p className="mt-1 text-xs text-critical">
                Must be at least 8 characters
              </p>
            )}
          </div>

          <div>
            <label className="mb-1 block text-xs font-mono text-text-secondary">
              CONFIRM NEW PASSWORD
            </label>
            <Input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Re-enter new password"
              autoComplete="new-password"
              required
            />
            {mismatch && (
              <p className="mt-1 text-xs text-critical">
                Passwords do not match
              </p>
            )}
          </div>

          {mutation.isError && (
            <div className="flex items-center gap-2 rounded-md border border-critical/20 bg-critical/10 px-3 py-2 text-xs text-critical">
              <AlertCircle size={13} className="shrink-0" />
              {(mutation.error as Error).message}
            </div>
          )}

          <Button
            type="submit"
            disabled={!canSubmit}
            className="mt-1 bg-primary-blue font-mono font-bold text-white hover:bg-primary-blue/90"
          >
            {mutation.isPending ? (
              <Loader size={14} className="animate-spin" />
            ) : (
              <KeyRound size={14} />
            )}
            {mutation.isPending ? 'Updating...' : 'Set New Password'}
          </Button>
        </form>
      </div>
    </div>
  );
}

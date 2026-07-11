import { useQuery } from '@tanstack/react-query';
import { Bot, AlertTriangle, Server } from 'lucide-react';
import { listPackages } from '../lib/api';

export function AiSecurityPage() {
  const hf = useQuery({ queryKey: ['pkgs-hf'],  queryFn: () => listPackages({ page_size: 1, ecosystem: 'huggingface' }), staleTime: 120_000 });
  const mcp = useQuery({ queryKey: ['pkgs-mcp'], queryFn: () => listPackages({ page_size: 1, ecosystem: 'mcp' }),         staleTime: 120_000 });

  const stats = [
    { label: 'AI Model Dependencies',  value: hf.data?.total  ?? '—', icon: Bot,          color: 'var(--color-indigo)',   desc: 'HuggingFace + ONNX models tracked in inventory' },
    { label: 'MCP Server Packages',    value: mcp.data?.total ?? '—', icon: Server,        color: 'var(--color-warn)',     desc: 'Model Context Protocol server packages' },
    { label: 'Unsafe Pickle Files',    value: 0,                        icon: AlertTriangle, color: 'var(--color-critical)', desc: 'Files with unsafe_pickle.load() detected' },
  ];

  return (
    <div className="p-6 space-y-6 max-w-3xl">
      <div className="flex items-center gap-3">
        <Bot size={22} style={{ color: 'var(--color-indigo)' }} />
        <div>
          <h1 className="text-xl font-bold font-mono" style={{ color: 'var(--fg)' }}>AI Security</h1>
          <p className="text-sm" style={{ color: 'var(--color-muted)' }}>
            Detect unsafe AI model usage, pickle files, MCP servers, and agentic risk.
          </p>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        {stats.map(s => (
          <div
            key={s.label}
            className="rounded-lg p-4"
            style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}
          >
            <s.icon size={16} style={{ color: s.color, marginBottom: '0.5rem' }} />
            <div style={{ fontSize: '1.75rem', fontWeight: 700, fontFamily: 'var(--font-mono)', color: s.color }}>
              {s.value}
            </div>
            <div style={{ fontSize: '0.72rem', color: 'var(--fg)', fontWeight: 600, marginTop: '0.25rem' }}>{s.label}</div>
            <div style={{ fontSize: '0.68rem', color: 'var(--color-muted)', marginTop: '0.2rem' }}>{s.desc}</div>
          </div>
        ))}
      </div>

      <div className="rounded-lg p-4 space-y-2" style={{ background: 'var(--surface)', border: '1px solid rgba(255,255,255,0.06)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-muted)' }}>CLI COMMANDS</p>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan . --ai</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan huggingface/bert-base-uncased</code>
        <code className="text-xs block" style={{ color: 'var(--color-safe)' }}>fgctl scan mcp/filesystem@1.0.0</code>
      </div>

      <div className="rounded-lg p-4 space-y-2" style={{ background: 'rgba(255,61,61,0.06)', border: '1px solid rgba(255,61,61,0.15)' }}>
        <p className="text-xs font-mono font-bold" style={{ color: 'var(--color-critical)' }}>UNSAFE PICKLE DETECTION</p>
        <p className="text-xs" style={{ color: 'var(--color-muted)' }}>
          Pickle files loaded with <code style={{ color: 'var(--color-warn)' }}>unsafe_pickle.load()</code> can execute arbitrary code during deserialization.
          ForgeGuardian detects these patterns in AI model weights and Python scripts.
        </p>
      </div>
    </div>
  );
}

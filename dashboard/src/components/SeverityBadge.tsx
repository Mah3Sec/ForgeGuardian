import type { Severity } from '../types/api';

const colors: Record<Severity, string> = {
  CRITICAL: 'bg-red-600 text-white',
  HIGH:     'bg-orange-500 text-white',
  MEDIUM:   'bg-yellow-500 text-black',
  LOW:      'bg-blue-500 text-white',
  INFORMATIONAL: 'bg-gray-600 text-white',
};

export function SeverityBadge({ severity }: { severity: Severity | string }) {
  const cls = colors[(severity as Severity)] ?? 'bg-gray-700 text-white';
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-bold uppercase ${cls}`}>
      {severity}
    </span>
  );
}

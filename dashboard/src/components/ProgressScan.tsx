import { CheckCircle2, Circle, XCircle, Loader2 } from 'lucide-react'
import { cn } from './ui/utils'

export type ProgressScanStatus = 'queued' | 'running' | 'complete' | 'failed'

interface ProgressScanProps {
  status: ProgressScanStatus
  steps?: string[]
  className?: string
}

const DEFAULT_STEPS = [
  'Discovering assets',
  'Analyzing dependencies',
  'Detecting vulnerabilities',
  'Building security posture',
]

// Honesty note: `ScanJob.status` (dashboard/src/types/api.ts) only carries
// four coarse states — queued | running | complete | failed — there is no
// real sub-step/progress signal from the backend. We therefore do NOT fake
// granular progress: while `running`, only the first step is shown as
// "active" and the rest stay pending (not falsely checked) until the job
// actually reaches `complete`, at which point all steps flip to done at
// once. If the backend ever exposes real per-step status, this component
// should be updated to reflect it instead of this single active-step
// approximation.
export function ProgressScan({ status, steps = DEFAULT_STEPS, className }: ProgressScanProps) {
  if (status === 'failed') {
    return (
      <div className={cn('flex items-center gap-2 text-sm text-critical', className)}>
        <XCircle size={16} />
        <span>Scan failed</span>
      </div>
    )
  }

  return (
    <div className={cn('flex flex-col gap-3', className)}>
      {steps.map((step, i) => {
        const isDone = status === 'complete'
        const isActive = status === 'running' && i === 0 && !isDone
        const isQueued = status === 'queued'

        return (
          <div key={step} className="flex items-center gap-2">
            {isDone ? (
              <CheckCircle2 size={16} className="text-success" />
            ) : isActive ? (
              <Loader2 size={16} className="animate-spin text-primary-blue" />
            ) : (
              <Circle size={16} className="text-text-muted" />
            )}
            <span
              className={cn(
                'text-sm',
                isDone && 'text-text-primary',
                isActive && 'font-medium text-text-primary',
                (isQueued || (!isDone && !isActive)) && 'text-text-muted'
              )}
            >
              {step}
            </span>
          </div>
        )
      })}
    </div>
  )
}

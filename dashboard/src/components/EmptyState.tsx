import type { ElementType } from 'react'
import { Inbox } from 'lucide-react'
import { cn } from './ui/utils'
import { TopoBackground } from './TopoBackground'

interface EmptyStateProps {
  icon?: ElementType
  title: string
  description?: string
  action?: {
    label: string
    onClick: () => void
  }
  className?: string
}

export function EmptyState({ icon: Icon = Inbox, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn('relative flex flex-col items-center gap-4 py-16 px-6 text-center overflow-hidden', className)}>
      <TopoBackground className="text-text-primary" opacity={0.04} lines={6} />
      <Icon size={40} className="relative z-10 text-text-muted" />
      <div className="relative z-10">
        <p className="text-sm font-semibold text-text-primary">{title}</p>
        {description && (
          <p className="mt-1 text-[0.78rem] text-text-secondary">{description}</p>
        )}
      </div>
      {action && (
        <button
          type="button"
          onClick={action.onClick}
          className="relative z-10 rounded-md bg-primary-blue px-4 py-2 text-sm font-medium text-white hover:opacity-90"
        >
          {action.label}
        </button>
      )}
    </div>
  )
}

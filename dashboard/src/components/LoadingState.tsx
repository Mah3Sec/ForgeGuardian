import { Skeleton } from './ui/skeleton'
import { cn } from './ui/utils'

interface LoadingStateProps {
  rows?: number
  variant?: 'table' | 'card' | 'text'
  className?: string
}

function TableRowSkeleton() {
  return (
    <div className="flex items-center gap-4 border-b border-border-color px-4 py-3">
      <Skeleton className="h-4 w-1/4" />
      <Skeleton className="h-4 w-1/6" />
      <Skeleton className="h-4 w-1/6" />
      <Skeleton className="h-4 flex-1" />
    </div>
  )
}

function CardSkeleton() {
  return (
    <div className="rounded-xl border border-border-color bg-surface p-5">
      <Skeleton className="h-4 w-1/3" />
      <Skeleton className="mt-3 h-8 w-1/2" />
      <Skeleton className="mt-3 h-3 w-2/3" />
    </div>
  )
}

function TextSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="h-3 w-full" />
      <Skeleton className="h-3 w-11/12" />
      <Skeleton className="h-3 w-3/4" />
    </div>
  )
}

export function LoadingState({ rows = 3, variant = 'table', className }: LoadingStateProps) {
  const items = Array.from({ length: rows })

  return (
    <div className={cn('w-full', variant === 'card' && 'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3', className)}>
      {items.map((_, i) => {
        if (variant === 'card') return <CardSkeleton key={i} />
        if (variant === 'text') return <TextSkeleton key={i} />
        return <TableRowSkeleton key={i} />
      })}
    </div>
  )
}

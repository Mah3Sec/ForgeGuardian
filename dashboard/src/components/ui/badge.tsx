import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from './utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        destructive: 'border-transparent bg-destructive text-destructive-foreground',
        outline: 'text-foreground',
        critical: 'border-transparent bg-[#FF3D3D]/20 text-[#FF3D3D] border-[#FF3D3D]/30',
        high: 'border-transparent bg-orange-500/20 text-orange-400 border-orange-500/30',
        medium: 'border-transparent bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
        low: 'border-transparent bg-blue-500/20 text-blue-400 border-blue-500/30',
        safe: 'border-transparent bg-[#00FF87]/20 text-[#00FF87] border-[#00FF87]/30',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge, badgeVariants }

import * as React from 'react'
import { cn } from './utils'

export interface SwitchProps {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  disabled?: boolean
  className?: string
  id?: string
}

// Minimal custom toggle (checkbox styled as a pill) — avoids pulling in
// another Radix package for a single boolean control.
const Switch = React.forwardRef<HTMLButtonElement, SwitchProps>(
  ({ checked, onCheckedChange, disabled, className, id }, ref) => (
    <button
      ref={ref}
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        'relative inline-flex h-5 w-9 shrink-0 items-center rounded-full transition-colors',
        'disabled:cursor-not-allowed disabled:opacity-50',
        className
      )}
      style={{
        background: checked ? 'var(--color-safe)' : 'rgba(255,255,255,0.15)',
      }}
    >
      <span
        className="inline-block h-3.5 w-3.5 rounded-full bg-white transition-transform"
        style={{
          transform: checked ? 'translateX(19px)' : 'translateX(3px)',
        }}
      />
    </button>
  )
)
Switch.displayName = 'Switch'

export { Switch }

import { useState, useMemo, useEffect, type ElementType } from 'react'
import * as Dialog from '@radix-ui/react-dialog'
import { Search } from 'lucide-react'
import { cn } from './ui/utils'

export interface Command {
  label: string
  action: () => void
  icon?: ElementType
  group?: string
}

interface CommandMenuProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  commands: Command[]
}

const UNGROUPED = 'Commands'

export function CommandMenu({ open, onOpenChange, commands }: CommandMenuProps) {
  const [filter, setFilter] = useState('')

  useEffect(() => {
    if (!open) setFilter('')
  }, [open])

  const grouped = useMemo(() => {
    const q = filter.trim().toLowerCase()
    const filtered = q ? commands.filter((c) => c.label.toLowerCase().includes(q)) : commands
    const groups = new Map<string, Command[]>()
    for (const cmd of filtered) {
      const key = cmd.group ?? UNGROUPED
      if (!groups.has(key)) groups.set(key, [])
      groups.get(key)!.push(cmd)
    }
    return groups
  }, [commands, filter])

  const run = (cmd: Command) => {
    cmd.action()
    onOpenChange(false)
  }

  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-black/40" />
        <Dialog.Content
          className="fixed left-1/2 top-[20%] z-50 w-full max-w-md -translate-x-1/2 overflow-hidden rounded-xl border border-border-color bg-surface shadow-lg"
          onOpenAutoFocus={(e) => {
            // Let the input take focus naturally via autoFocus below.
            e.preventDefault()
            const input = (e.currentTarget as HTMLElement).querySelector('input')
            input?.focus()
          }}
        >
          <Dialog.Title className="sr-only">Command Menu</Dialog.Title>
          <div className="flex items-center gap-2 border-b border-border-color px-4 py-3">
            <Search size={16} className="text-text-muted" />
            <input
              autoFocus
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Type a command..."
              className="w-full bg-transparent text-sm text-text-primary outline-none placeholder:text-text-muted"
            />
          </div>
          <div className="max-h-80 overflow-y-auto py-2">
            {grouped.size === 0 && (
              <div className="px-4 py-6 text-center text-sm text-text-muted">No matching commands</div>
            )}
            {Array.from(grouped.entries()).map(([group, cmds]) => (
              <div key={group} className="mb-1">
                <div className="px-4 py-1 text-xs font-medium uppercase tracking-wide text-text-muted">
                  {group}
                </div>
                {cmds.map((cmd, i) => {
                  const Icon = cmd.icon
                  return (
                    <button
                      key={`${group}-${i}`}
                      type="button"
                      onClick={() => run(cmd)}
                      className={cn(
                        'flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-text-primary',
                        'hover:bg-surface-muted'
                      )}
                    >
                      {Icon && <Icon size={14} className="text-text-secondary" />}
                      {cmd.label}
                    </button>
                  )
                })}
              </div>
            ))}
          </div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}

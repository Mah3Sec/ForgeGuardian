import { useState, useEffect, useRef } from 'react';
import { ChevronDown, Plus, Trash2, Check, X } from 'lucide-react';
import { useWorkspaceStore, WORKSPACE_COLORS } from '../store/workspace';
import { cn } from './ui/utils';
import { Input } from './ui/input';

export function WorkspaceSwitcher({ collapsed }: { collapsed: boolean }) {
  const { workspaces, activeId, create, remove, setActive } = useWorkspaceStore();
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [selectedColor, setSelectedColor] = useState(WORKSPACE_COLORS[0]);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
        setCreating(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [open]);

  const active = workspaces.find(w => w.id === activeId) ?? workspaces[0];

  const handleCreate = () => {
    if (!newName.trim()) return;
    const ws = create(newName.trim(), selectedColor);
    setActive(ws.id);
    setNewName('');
    setCreating(false);
    setOpen(false);
  };

  if (collapsed) {
    return (
      <div ref={containerRef} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="relative w-full flex items-center justify-center py-2 bg-transparent cursor-pointer"
        title={active.name}
      >
        <div
          className="w-6 h-6 rounded flex items-center justify-center text-[0.55rem] font-bold text-white"
          style={{ background: active.color }}
        >
          {active.name.slice(0, 2).toUpperCase()}
        </div>
        {open && (
          <div className="absolute left-full top-0 ml-1 z-50 w-56 rounded-lg border border-border-color bg-surface shadow-lg py-1">
            {workspaces.map(w => (
              <button
                key={w.id}
                onClick={(e) => { e.stopPropagation(); setActive(w.id); setOpen(false); }}
                className={cn(
                  'w-full flex items-center gap-2 px-3 py-1.5 text-[0.75rem] text-left bg-transparent cursor-pointer [font-family:inherit]',
                  w.id === activeId ? 'text-primary-blue' : 'text-text-secondary hover:bg-surface-muted'
                )}
              >
                <div className="w-3 h-3 rounded-sm shrink-0" style={{ background: w.color }} />
                <span className="truncate flex-1">{w.name}</span>
                {w.id === activeId && <Check size={12} />}
              </button>
            ))}
          </div>
        )}
      </button>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="relative px-2.5 py-2">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center gap-2 px-2 py-1.5 rounded-md border border-border-color bg-transparent hover:bg-surface-muted cursor-pointer [font-family:inherit] transition-colors"
      >
        <div className="w-4 h-4 rounded-sm shrink-0" style={{ background: active.color }} />
        <span className="flex-1 text-left text-[0.75rem] text-text-primary truncate font-medium">
          {active.name}
        </span>
        <ChevronDown size={12} className={cn('text-text-muted transition-transform', open && 'rotate-180')} />
      </button>

      {open && (
        <div className="absolute left-2.5 right-2.5 top-full mt-1 z-50 rounded-lg border border-border-color bg-surface shadow-lg overflow-hidden">
          <div className="py-1">
            <p className="px-3 py-1 text-[0.6rem] font-bold text-text-muted tracking-wider uppercase">Workspaces</p>
            {workspaces.map(w => (
              <div key={w.id} className="group flex items-center">
                <button
                  onClick={() => { setActive(w.id); setOpen(false); }}
                  className={cn(
                    'flex-1 flex items-center gap-2 px-3 py-1.5 text-[0.75rem] text-left bg-transparent cursor-pointer [font-family:inherit]',
                    w.id === activeId ? 'text-primary-blue bg-blue-light' : 'text-text-secondary hover:bg-surface-muted'
                  )}
                >
                  <div className="w-3.5 h-3.5 rounded-sm shrink-0" style={{ background: w.color }} />
                  <span className="truncate flex-1">{w.name}</span>
                  {w.id === activeId && <Check size={12} />}
                </button>
                {workspaces.length > 1 && (
                  <button
                    onClick={(e) => { e.stopPropagation(); remove(w.id); }}
                    className="hidden group-hover:flex items-center justify-center w-7 h-7 mr-1 text-text-muted hover:text-critical bg-transparent cursor-pointer rounded"
                    title="Delete workspace"
                  >
                    <Trash2 size={11} />
                  </button>
                )}
              </div>
            ))}
          </div>

          <div className="border-t border-border-color px-3 py-2">
            {creating ? (
              <div className="flex flex-col gap-2">
                <Input
                  value={newName}
                  onChange={e => setNewName(e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && handleCreate()}
                  placeholder="Workspace name"
                  autoFocus
                  className="h-7 text-xs"
                />
                <div className="flex gap-1">
                  {WORKSPACE_COLORS.map(c => (
                    <button
                      key={c}
                      onClick={() => setSelectedColor(c)}
                      className={cn(
                        'w-5 h-5 rounded-full border-2 cursor-pointer transition-transform',
                        selectedColor === c ? 'border-text-primary scale-110' : 'border-transparent'
                      )}
                      style={{ background: c }}
                    />
                  ))}
                </div>
                <div className="flex gap-1.5">
                  <button
                    onClick={handleCreate}
                    disabled={!newName.trim()}
                    className="flex-1 flex items-center justify-center gap-1 h-7 rounded-md bg-primary-blue text-white text-xs font-medium cursor-pointer disabled:opacity-40 border-none [font-family:inherit]"
                  >
                    <Check size={12} /> Create
                  </button>
                  <button
                    onClick={() => { setCreating(false); setNewName(''); }}
                    className="flex items-center justify-center w-7 h-7 rounded-md border border-border-color bg-transparent text-text-muted hover:text-text-primary cursor-pointer"
                  >
                    <X size={12} />
                  </button>
                </div>
              </div>
            ) : (
              <button
                onClick={() => setCreating(true)}
                className="w-full flex items-center gap-1.5 text-[0.72rem] text-text-secondary hover:text-primary-blue bg-transparent cursor-pointer [font-family:inherit] py-0.5"
              >
                <Plus size={12} /> New Workspace
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

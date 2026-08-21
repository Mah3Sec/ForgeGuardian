import { create } from 'zustand';

export interface Workspace {
  id: string;
  name: string;
  color: string;
  created_at: string;
}

const STORAGE_KEY = 'fg_workspaces';
const ACTIVE_KEY = 'fg_active_workspace';

const COLORS = ['#2563EB', '#7C3AED', '#059669', '#D97706', '#DC2626', '#EC4899', '#06B6D4', '#8B5CF6'];

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function loadWorkspaces(): Workspace[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch { return []; }
}

function saveWorkspaces(ws: Workspace[]) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(ws));
}

function ensureDefault(workspaces: Workspace[]): Workspace[] {
  if (workspaces.length > 0) return workspaces;
  const def: Workspace = {
    id: generateId(),
    name: 'Default',
    color: COLORS[0],
    created_at: new Date().toISOString(),
  };
  saveWorkspaces([def]);
  localStorage.setItem(ACTIVE_KEY, def.id);
  return [def];
}

interface WorkspaceState {
  workspaces: Workspace[];
  activeId: string;
  create: (name: string, color?: string) => Workspace;
  rename: (id: string, name: string) => void;
  remove: (id: string) => void;
  setActive: (id: string) => void;
  getActive: () => Workspace;
}

const initial = ensureDefault(loadWorkspaces());
const storedActive = localStorage.getItem(ACTIVE_KEY);
const initialActive = initial.find(w => w.id === storedActive)?.id ?? initial[0].id;

export const useWorkspaceStore = create<WorkspaceState>((set, get) => ({
  workspaces: initial,
  activeId: initialActive,

  create: (name, color) => {
    const ws: Workspace = {
      id: generateId(),
      name,
      color: color ?? COLORS[get().workspaces.length % COLORS.length],
      created_at: new Date().toISOString(),
    };
    const next = [...get().workspaces, ws];
    saveWorkspaces(next);
    set({ workspaces: next });
    return ws;
  },

  rename: (id, name) => {
    const next = get().workspaces.map(w => w.id === id ? { ...w, name } : w);
    saveWorkspaces(next);
    set({ workspaces: next });
  },

  remove: (id) => {
    let next = get().workspaces.filter(w => w.id !== id);
    next = ensureDefault(next);
    saveWorkspaces(next);
    const activeId = next.find(w => w.id === get().activeId)?.id ?? next[0].id;
    localStorage.setItem(ACTIVE_KEY, activeId);
    set({ workspaces: next, activeId });
  },

  setActive: (id) => {
    localStorage.setItem(ACTIVE_KEY, id);
    set({ activeId: id });
  },

  getActive: () => {
    const { workspaces, activeId } = get();
    return workspaces.find(w => w.id === activeId) ?? workspaces[0];
  },
}));

export { COLORS as WORKSPACE_COLORS };

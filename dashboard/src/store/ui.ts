import { create } from 'zustand';

export type Theme = 'light' | 'dark';

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle('dark', theme === 'dark');
}

function getInitialTheme(): Theme {
  const stored = localStorage.getItem('fg_theme');
  return stored === 'light' ? 'light' : 'dark';
}

interface UIState {
  sidebarOpen: boolean;
  currentEcosystem: string;
  theme: Theme;
  navigate: (path: string) => void;
  _setNavigateFn: (fn: (path: string) => void) => void;
  setSidebarOpen: (v: boolean) => void;
  setCurrentEcosystem: (e: string) => void;
  toggleTheme: () => void;
}

const initialTheme = getInitialTheme();
applyTheme(initialTheme);

export const useUIStore = create<UIState>((set, get) => ({
  sidebarOpen: true,
  currentEcosystem: 'all',
  theme: initialTheme,
  navigate: (path) => console.warn('navigate not wired:', path),
  _setNavigateFn: (fn) => set({ navigate: fn }),
  setSidebarOpen: (v) => set({ sidebarOpen: v }),
  setCurrentEcosystem: (e) => set({ currentEcosystem: e }),
  toggleTheme: () => {
    const next: Theme = get().theme === 'dark' ? 'light' : 'dark';
    localStorage.setItem('fg_theme', next);
    applyTheme(next);
    set({ theme: next });
  },
}));

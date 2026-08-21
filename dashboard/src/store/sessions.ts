import { create } from 'zustand';
import type { ScanResult, ProjectScanResult, ScanSummary, Finding } from '../types/api';

export type ScanType = 'registry' | 'upload' | 'remote';

export interface ScanSession {
  id: string;
  workspace_id: string;
  scan_type: ScanType;
  label: string;
  ecosystem?: string;
  package_name?: string;
  version?: string;
  result: ScanResult | null;
  project_result: ProjectScanResult | null;
  summary: ScanSummary;
  findings: Finding[];
  created_at: string;
}

const STORAGE_KEY = 'fg_scan_sessions';
const MAX_SESSIONS = 200;

function generateId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function loadSessions(): ScanSession[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch { return []; }
}

function saveSessions(sessions: ScanSession[]) {
  const trimmed = sessions.slice(0, MAX_SESSIONS);
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(trimmed));
  } catch {
    // localStorage full — drop oldest half and retry
    const half = trimmed.slice(0, Math.floor(trimmed.length / 2));
    localStorage.setItem(STORAGE_KEY, JSON.stringify(half));
  }
}

interface SessionState {
  sessions: ScanSession[];
  save: (session: Omit<ScanSession, 'id' | 'created_at'>) => ScanSession;
  remove: (id: string) => void;
  clear: (workspaceId: string) => void;
  get: (id: string) => ScanSession | undefined;
  forWorkspace: (workspaceId: string) => ScanSession[];
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: loadSessions(),

  save: (input) => {
    const session: ScanSession = {
      ...input,
      id: generateId(),
      created_at: new Date().toISOString(),
    };
    const next = [session, ...get().sessions];
    saveSessions(next);
    set({ sessions: next });
    return session;
  },

  remove: (id) => {
    const next = get().sessions.filter(s => s.id !== id);
    saveSessions(next);
    set({ sessions: next });
  },

  clear: (workspaceId) => {
    const next = get().sessions.filter(s => s.workspace_id !== workspaceId);
    saveSessions(next);
    set({ sessions: next });
  },

  get: (id) => get().sessions.find(s => s.id === id),

  forWorkspace: (workspaceId) => get().sessions.filter(s => s.workspace_id === workspaceId),
}));

import { useEffect, useRef, useState } from 'react';

const BASE = import.meta.env.VITE_API_URL ?? '';
const API_KEY = import.meta.env.VITE_API_KEY ?? '';

export interface AgentEvent {
  session_id?: string;
  type: 'connected' | 'start' | 'step' | 'patch' | 'done' | 'error';
  message: string;
  package?: string;
  version?: string;
  occurred_at: string;
}

export function useAgentStream() {
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const esRef = useRef<EventSource | null>(null);

  useEffect(() => {
    const url = API_KEY
      ? `${BASE}/api/v1/agent/stream?api_key=${encodeURIComponent(API_KEY)}`
      : `${BASE}/api/v1/agent/stream`;

    const es = new EventSource(url);
    esRef.current = es;

    es.onopen = () => {
      setConnected(true);
      setError(null);
    };

    es.onmessage = (e) => {
      try {
        const ev: AgentEvent = JSON.parse(e.data);
        if (ev.type === 'connected') {
          setConnected(true);
          return;
        }
        setEvents(prev => [ev, ...prev].slice(0, 200)); // keep last 200
      } catch {
        // malformed event — ignore
      }
    };

    es.onerror = () => {
      setConnected(false);
      setError('Stream disconnected — retrying…');
    };

    return () => {
      es.close();
      esRef.current = null;
    };
  }, []);

  const clear = () => setEvents([]);

  return { events, connected, error, clear };
}

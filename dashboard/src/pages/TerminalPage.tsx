import { useState, useRef, useEffect, useCallback } from 'react'
import { useQuery } from '@tanstack/react-query'

const BASE = import.meta.env.VITE_API_URL ?? ''
const API_KEY = import.meta.env.VITE_API_KEY ?? ''
function authHeaders(): Record<string, string> {
  return API_KEY ? { 'X-Api-Key': API_KEY } : {}
}

interface OutputLine {
  type: 'prompt' | 'output' | 'error' | 'system'
  text: string
}

async function getCompletions() {
  const res = await fetch(`${BASE}/api/v1/terminal/completions`, {
    headers: authHeaders(),
    credentials: 'include',
  })
  if (!res.ok) throw new Error('Failed')
  return res.json() as Promise<{
    commands: string[]
    examples: { cmd: string; desc: string }[]
  }>
}

export function TerminalPage() {
  const [input, setInput] = useState('')
  const [lines, setLines] = useState<OutputLine[]>([
    { type: 'system', text: 'ForgeGuardian Terminal v1.0.0' },
    { type: 'system', text: 'Type "help" for available commands.' },
    { type: 'system', text: '' },
  ])
  const [cmdHistory, setCmdHistory] = useState<string[]>([])
  const [historyIdx, setHistoryIdx] = useState(-1)
  const [running, setRunning] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)

  useQuery({
    queryKey: ['terminal-completions'],
    queryFn: getCompletions,
    staleTime: 5 * 60_000,
    retry: false,
  })

  const scroll = useCallback(() => {
    requestAnimationFrame(() => {
      if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    })
  }, [])

  useEffect(scroll, [lines, scroll])

  const focusInput = () => {
    if (!running) inputRef.current?.focus()
  }

  useEffect(focusInput, [running])

  const exec = useCallback(async (raw: string) => {
    const cmd = raw.trim()
    if (!cmd) return

    setCmdHistory(prev => [...prev.filter(h => h !== cmd), cmd])
    setHistoryIdx(-1)

    setLines(prev => [...prev, { type: 'prompt', text: `$ fgctl ${cmd}` }])
    setRunning(true)

    if (cmd === 'clear') {
      setLines([])
      setRunning(false)
      return
    }

    const controller = new AbortController()
    abortRef.current = controller

    try {
      const res = await fetch(`${BASE}/api/v1/terminal/exec`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authHeaders() },
        credentials: 'include',
        body: JSON.stringify({ command: cmd }),
        signal: controller.signal,
      })

      if (!res.ok) {
        const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }))
        setLines(prev => [...prev, { type: 'error', text: err.error || res.statusText }])
        setRunning(false)
        return
      }

      const reader = res.body?.getReader()
      if (!reader) {
        setLines(prev => [...prev, { type: 'error', text: 'No response stream' }])
        setRunning(false)
        return
      }

      const decoder = new TextDecoder()
      let buffer = ''
      let isDoneEvent = false

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const segments = buffer.split('\n')
        buffer = segments.pop() || ''

        for (const seg of segments) {
          if (seg.startsWith('event: done')) {
            isDoneEvent = true
            continue
          }
          if (seg.startsWith('data: ')) {
            if (isDoneEvent) {
              isDoneEvent = false
              continue
            }
            const data = seg.slice(6)
            setLines(prev => [...prev, { type: 'output', text: data }])
          }
          if (seg === '') isDoneEvent = false
        }
      }

      setLines(prev => [...prev, { type: 'system', text: '' }])
    } catch (err: unknown) {
      const e = err as Error
      if (e.name === 'AbortError') {
        setLines(prev => [...prev, { type: 'error', text: '^C' }])
      } else {
        setLines(prev => [...prev, { type: 'error', text: e.message || 'Unknown error' }])
      }
    } finally {
      setRunning(false)
      abortRef.current = null
    }
  }, [])

  const handleKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !running) {
      exec(input)
      setInput('')
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (cmdHistory.length > 0) {
        const ni = historyIdx < cmdHistory.length - 1 ? historyIdx + 1 : historyIdx
        setHistoryIdx(ni)
        setInput(cmdHistory[cmdHistory.length - 1 - ni] || '')
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (historyIdx > 0) {
        const ni = historyIdx - 1
        setHistoryIdx(ni)
        setInput(cmdHistory[cmdHistory.length - 1 - ni] || '')
      } else {
        setHistoryIdx(-1)
        setInput('')
      }
    } else if (e.key === 'c' && e.ctrlKey && running) {
      abortRef.current?.abort()
    } else if (e.key === 'l' && e.ctrlKey) {
      e.preventDefault()
      setLines([])
    }
  }

  return (
    <div style={{
      display: 'flex',
      flexDirection: 'column',
      height: '100%',
      padding: '12px',
      background: '#0C0C0C',
      boxSizing: 'border-box',
    }}>
      {/* Terminal window */}
      <div style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        borderRadius: '8px',
        overflow: 'hidden',
        border: '1px solid #333',
        boxShadow: '0 8px 32px rgba(0,0,0,0.5)',
        minHeight: 0,
      }}>
        {/* Title bar */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          height: '36px',
          padding: '0 12px',
          background: '#1E1E1E',
          borderBottom: '1px solid #333',
          gap: '8px',
          flexShrink: 0,
          userSelect: 'none',
        }}>
          <div style={{ display: 'flex', gap: '6px' }}>
            <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#FF5F57' }} />
            <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#FFBD2E' }} />
            <div style={{ width: 12, height: 12, borderRadius: '50%', background: '#28C840' }} />
          </div>
          <div style={{
            flex: 1,
            textAlign: 'center',
            fontSize: '12px',
            fontFamily: "'Menlo', 'Monaco', 'Cascadia Code', 'Consolas', monospace",
            color: '#808080',
            letterSpacing: '0.02em',
          }}>
            fgctl — ForgeGuardian Terminal
          </div>
          <div style={{ width: 48 }} />
        </div>

        {/* Terminal body */}
        <div
          ref={scrollRef}
          onClick={focusInput}
          style={{
            flex: 1,
            overflow: 'auto',
            padding: '12px 14px',
            background: '#0C0C0C',
            fontFamily: "'Menlo', 'Monaco', 'Cascadia Code', 'Consolas', monospace",
            fontSize: '13px',
            lineHeight: '20px',
            cursor: 'text',
            minHeight: 0,
          }}
        >
          {lines.map((line, i) => (
            <div key={i} style={{ minHeight: '20px', whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
              {line.type === 'prompt' ? (
                <>
                  <span style={{ color: '#28C840' }}>❯</span>
                  <span style={{ color: '#E8E8E8' }}> {line.text.replace('$ fgctl ', '')}</span>
                </>
              ) : line.type === 'error' ? (
                <span style={{ color: '#FF5F57' }}>{line.text}</span>
              ) : line.type === 'system' ? (
                <span style={{ color: '#666' }}>{line.text}</span>
              ) : (
                <span style={{ color: '#CCCCCC' }}>{line.text}</span>
              )}
            </div>
          ))}

          {/* Active prompt */}
          <div style={{ display: 'flex', alignItems: 'center', minHeight: '20px' }}>
            <span style={{ color: running ? '#555' : '#28C840', marginRight: '6px' }}>
              {running ? '⏳' : '❯'}
            </span>
            <div style={{ position: 'relative', flex: 1, display: 'flex', alignItems: 'center' }}>
              <input
                ref={inputRef}
                type="text"
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={handleKey}
                disabled={running}
                autoFocus
                spellCheck={false}
                autoComplete="off"
                style={{
                  width: '100%',
                  background: 'transparent',
                  border: 'none',
                  outline: 'none',
                  color: '#E8E8E8',
                  fontFamily: 'inherit',
                  fontSize: 'inherit',
                  lineHeight: 'inherit',
                  padding: 0,
                  margin: 0,
                  caretColor: '#28C840',
                }}
              />
              {running && (
                <span style={{
                  color: '#666',
                  fontSize: '11px',
                  marginLeft: '8px',
                  whiteSpace: 'nowrap',
                }}>
                  running... (ctrl+c to stop)
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Status bar */}
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          height: '24px',
          padding: '0 12px',
          background: '#1A1A2E',
          borderTop: '1px solid #333',
          fontSize: '11px',
          fontFamily: "'Menlo', 'Monaco', 'Cascadia Code', 'Consolas', monospace",
          color: '#666',
          flexShrink: 0,
          userSelect: 'none',
        }}>
          <div style={{ display: 'flex', gap: '14px' }}>
            <span>
              <span style={{ color: '#28C840', marginRight: 4 }}>●</span>
              connected
            </span>
            <span>{cmdHistory.length} commands</span>
          </div>
          <div style={{ display: 'flex', gap: '14px' }}>
            <span>↑↓ history</span>
            <span>^L clear</span>
            <span>^C cancel</span>
          </div>
        </div>
      </div>
    </div>
  )
}

export default TerminalPage

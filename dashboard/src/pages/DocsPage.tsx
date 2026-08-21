import { useState, useEffect, useMemo, useRef, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeSlug from 'rehype-slug';
import { BookOpen, AlertCircle, Search, ChevronLeft, ChevronRight, List, X } from 'lucide-react';
import { CopyButton } from '../components/CopyButton';
import { LoadingState } from '../components/LoadingState';
import { EmptyState } from '../components/EmptyState';
import { cn } from '../components/ui/utils';

const DOCS = [
  { key: 'GUIDE', label: 'Guide', file: 'GUIDE.md' },
  { key: 'DOCS', label: 'Full Docs', file: 'DOCS.md' },
  { key: 'WORKFLOWS', label: 'Workflows', file: 'WORKFLOWS.md' },
];

interface Heading {
  id: string;
  text: string;
  level: 2 | 3;
}

// Extract h2/h3 headings from raw markdown so the "on this page" TOC can be
// built before/without waiting on a DOM pass. Slug logic mirrors rehype-slug's
// default (lowercase, spaces -> hyphens, strip non-alphanumerics) closely
// enough for our own heading set; rehype-slug is still used for the actual
// rendered anchor ids so the two stay in sync in the common case (plain-text
// headings, no inline code/links) — see slugify() below, shared by both.
function slugify(text: string): string {
  return text
    .toLowerCase()
    .trim()
    .replace(/[`*_~]/g, '')
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

function extractHeadings(markdown: string): Heading[] {
  const lines = markdown.split('\n');
  const headings: Heading[] = [];
  const seen: Record<string, number> = {};
  let inCodeFence = false;
  for (const line of lines) {
    if (/^```/.test(line.trim())) {
      inCodeFence = !inCodeFence;
      continue;
    }
    if (inCodeFence) continue;
    const m = /^(#{2,3})\s+(.+)$/.exec(line.trim());
    if (!m) continue;
    const level = m[1].length as 2 | 3;
    const text = m[2].replace(/#+$/, '').trim();
    let id = slugify(text);
    if (seen[id] !== undefined) {
      seen[id] += 1;
      id = `${id}-${seen[id]}`;
    } else {
      seen[id] = 0;
    }
    headings.push({ id, text, level });
  }
  return headings;
}

// Simple client-side "search this page" — substring match against the raw
// markdown text of the CURRENTLY LOADED doc only. This intentionally does
// NOT index all four docs; it is scoped to whichever doc is open, which is
// why the UI labels it "Search this page" rather than "Search documentation".
function useInPageSearch(content: string) {
  const [query, setQuery] = useState('');
  const matchCount = useMemo(() => {
    if (!query.trim()) return 0;
    const needle = query.trim().toLowerCase();
    const haystack = content.toLowerCase();
    if (!needle) return 0;
    let count = 0;
    let idx = haystack.indexOf(needle);
    while (idx !== -1) {
      count++;
      idx = haystack.indexOf(needle, idx + needle.length);
    }
    return count;
  }, [query, content]);
  return { query, setQuery, matchCount };
}

// Scrolls to the first case-insensitive text match of `query` within
// `container`, briefly highlighting it. Walks text nodes directly rather than
// mutating react-markdown's rendered DOM (avoids fighting React's reconciler
// with injected <mark> tags).
function scrollToFirstMatch(container: HTMLElement, query: string) {
  if (!query.trim()) return false;
  const needle = query.trim().toLowerCase();
  const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT);
  let node: Node | null;
  while ((node = walker.nextNode())) {
    const text = node.textContent ?? '';
    if (text.toLowerCase().includes(needle)) {
      const el = node.parentElement;
      if (el) {
        el.scrollIntoView({ behavior: 'smooth', block: 'center' });
        el.classList.add('fg-search-hit');
        setTimeout(() => el.classList.remove('fg-search-hit'), 1500);
        return true;
      }
    }
  }
  return false;
}

function CodeBlock({ className, children }: { className?: string; children?: React.ReactNode }) {
  const codeText = String(children ?? '').replace(/\n$/, '');
  const isBlock = /language-/.test(className ?? '') || codeText.includes('\n');

  if (!isBlock) {
    return <code className={className}>{children}</code>;
  }

  return (
    <div className="group relative">
      <pre>
        <code className={className}>{children}</code>
      </pre>
      <div className="absolute top-2 right-2 opacity-0 group-hover:opacity-100 transition-opacity">
        <CopyButton text={codeText} label="Copy" />
      </div>
    </div>
  );
}

export function DocsPage() {
  const [selected, setSelected] = useState(DOCS[0].key);
  const [content, setContent] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [tocOpen, setTocOpen] = useState(true);
  const contentRef = useRef<HTMLDivElement>(null);

  const { query, setQuery, matchCount } = useInPageSearch(content);

  useEffect(() => {
    const doc = DOCS.find(d => d.key === selected);
    if (!doc) return;
    setLoading(true);
    setError('');
    setQuery('');
    fetch(`/docs/${doc.file}`)
      .then(res => {
        if (!res.ok) throw new Error(`Failed to load ${doc.file}: ${res.status}`);
        return res.text();
      })
      .then(text => setContent(text))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected]);

  const headings = useMemo(() => extractHeadings(content), [content]);

  const activeIndex = DOCS.findIndex(d => d.key === selected);
  const prevDoc = activeIndex > 0 ? DOCS[activeIndex - 1] : null;
  const nextDoc = activeIndex < DOCS.length - 1 ? DOCS[activeIndex + 1] : null;

  const handleSearchSubmit = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return;
    if (!contentRef.current) return;
    scrollToFirstMatch(contentRef.current, query);
  }, [query]);

  const jumpToHeading = (id: string) => {
    const el = document.getElementById(id);
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
  };

  return (
    <div className="flex flex-col lg:flex-row h-full min-h-0">
      {/* ── Left nav — full-width top bar on mobile/tablet, side column at lg+ ── */}
      <div className="w-full lg:w-56 shrink-0 border-b lg:border-b-0 lg:border-r border-border-color flex flex-col overflow-y-auto p-4 lg:h-full">
        <div className="flex items-center gap-2 mb-4">
          <BookOpen size={16} className="text-primary-blue" />
          <h1 className="text-sm font-bold text-text-primary">Developer Docs</h1>
        </div>

        {/* Search this page */}
        <div className="relative mb-4">
          <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-text-muted" />
          <input
            type="text"
            value={query}
            onChange={e => setQuery(e.target.value)}
            onKeyDown={handleSearchSubmit}
            placeholder="Search this page…"
            aria-label="Search this page"
            className="w-full rounded-md border border-border-color bg-surface pl-7 pr-2 py-1.5 text-xs text-text-primary placeholder:text-text-muted focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none"
          />
        </div>
        {query.trim() && (
          <p className="text-[0.68rem] text-text-secondary mb-3 -mt-2">
            {matchCount > 0
              ? `${matchCount} match${matchCount !== 1 ? 'es' : ''} — press Enter to jump`
              : 'No matches on this page'}
          </p>
        )}

        <nav className="flex flex-row lg:flex-col gap-1 flex-wrap">
          {DOCS.map(doc => (
            <button
              key={doc.key}
              onClick={() => setSelected(doc.key)}
              className={cn(
                'text-left text-sm px-3 py-2 rounded-md transition-colors focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none',
                selected === doc.key
                  ? 'bg-blue-light text-primary-blue font-medium'
                  : 'text-text-secondary hover:bg-surface-muted hover:text-text-primary'
              )}
            >
              {doc.label}
            </button>
          ))}
        </nav>
      </div>

      {/* ── Center content ──────────────────────────────────────────────── */}
      <div className="flex-1 min-w-0 overflow-y-auto overflow-x-hidden p-6">
        {/* Breadcrumbs */}
        <div className="flex items-center justify-between mb-4">
          <p className="text-xs text-text-secondary">
            <span>Docs</span>
            <span className="mx-1.5 text-text-muted">/</span>
            <span className="text-text-primary font-medium">{DOCS[activeIndex]?.label}</span>
          </p>
          <button
            type="button"
            onClick={() => setTocOpen(v => !v)}
            className="lg:hidden inline-flex items-center gap-1.5 text-xs text-text-secondary hover:text-text-primary rounded-md px-2 py-1 focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none"
          >
            <List size={13} /> On this page
          </button>
        </div>

        <div className="rounded-xl border border-border-color bg-surface p-6 max-w-3xl">
          {loading ? (
            <LoadingState variant="text" rows={6} />
          ) : error ? (
            <EmptyState icon={AlertCircle} title="Failed to load document" description={error} />
          ) : (
            <div className="fg-docs" ref={contentRef}>
              <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                rehypePlugins={[rehypeSlug]}
                components={{
                  code: CodeBlock,
                }}
              >
                {content}
              </ReactMarkdown>
            </div>
          )}
        </div>

        {/* Prev/Next */}
        {!loading && !error && (
          <div className="flex items-center justify-between mt-6 max-w-3xl">
            {prevDoc ? (
              <button
                onClick={() => setSelected(prevDoc.key)}
                className="inline-flex items-center gap-1.5 rounded-md border border-border-color bg-surface px-3 py-2 text-xs font-medium text-text-secondary hover:bg-surface-muted hover:text-text-primary focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none"
              >
                <ChevronLeft size={13} /> {prevDoc.label}
              </button>
            ) : <span />}
            {nextDoc ? (
              <button
                onClick={() => setSelected(nextDoc.key)}
                className="inline-flex items-center gap-1.5 rounded-md border border-border-color bg-surface px-3 py-2 text-xs font-medium text-text-secondary hover:bg-surface-muted hover:text-text-primary focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none"
              >
                {nextDoc.label} <ChevronRight size={13} />
              </button>
            ) : <span />}
          </div>
        )}
      </div>

      {/* ── Right "on this page" TOC ────────────────────────────────────── */}
      {tocOpen && headings.length > 0 && !loading && !error && (
        <div className="w-52 shrink-0 border-l border-border-color overflow-y-auto p-4 hidden lg:block">
          <div className="flex items-center justify-between mb-2">
            <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-text-muted">On this page</p>
            <button
              type="button"
              onClick={() => setTocOpen(false)}
              className="text-text-muted hover:text-text-primary lg:hidden"
              aria-label="Close table of contents"
            >
              <X size={13} />
            </button>
          </div>
          <ul className="flex flex-col gap-1">
            {headings.map(h => (
              <li key={h.id}>
                <button
                  onClick={() => jumpToHeading(h.id)}
                  className={cn(
                    'text-left text-xs w-full rounded-md px-2 py-1 text-text-secondary hover:bg-surface-muted hover:text-text-primary focus-visible:ring-2 focus-visible:ring-primary-blue focus-visible:outline-none',
                    h.level === 3 && 'pl-4'
                  )}
                >
                  {h.text}
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

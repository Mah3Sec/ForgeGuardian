# ForgeGuardian Dashboard — Design System

> Extracted from the current codebase (`dashboard/src/index.css`,
> `tailwind.config.js`, `components/ui/*`). This documents what exists and
> is actually used — not a target to redesign toward. Update this file
> when tokens/components change; don't let it drift into aspiration.

## Theme tokens

Light is `:root`, dark is `.dark` (toggled on `<html>` via
`store/ui.ts`'s `theme` state, persisted to `localStorage` as `fg_theme`,
defaults to dark for new users — see `index.html`'s anti-flash inline
script).

| Token | Light | Dark | Use |
|---|---|---|---|
| `--bg-base` | `#FAFAF8` | `#0B0D0F` | Page background |
| `--surface` | `#FFFFFF` | `#111418` | Card/panel background |
| `--surface-muted` | `#F5F6F4` | `#161A1F` | Nested/inset panels, hover states |
| `--border-color` | `#E6E8E6` | `#252A31` | All borders |
| `--text-primary` | `#111315` | `#F5F7FA` | Headings, primary text |
| `--text-secondary` | `#667085` | `#8B95A5` | Body/description text |
| `--text-muted` | `#98A2B3` | `#6B7280` | Labels, timestamps, placeholders |
| `--primary-blue` | `#2563EB` | `#4F8CFF` | Primary actions, active nav, links |
| `--blue-light` | `#DBEAFE` | `#1E3A5F` | Active-state background (nav, tabs) |
| `--cyan` | `#06B6D4` | `#22D3EE` | Secondary accent (low severity, info) |
| `--success` | `#16A34A` | `#22C55E` | Safe/healthy/pass states |
| `--warning` | `#D97706` | `#F59E0B` | Medium severity, caution |
| `--critical` | `#DC2626` | `#F87171` | Critical severity, errors |
| `--font-mono` | `'JetBrains Mono', 'IBM Plex Mono', 'Fira Code', 'Consolas', monospace` | same | Data, code, CLI snippets |

Sans-serif body font is `'Inter', system-ui, sans-serif` (set on `body`,
not a CSS var).

**Severity color convention** (used consistently across `SeverityBadge`,
`StatusBadge`, `NetworkGraph`, dashboard cards):
critical → `--critical` · high → `#EA580C` (orange, not a root token —
defined inline per-component) · medium → `--warning` · low → `--cyan`.

### shadcn/ui HSL tokens

A second, parallel token set (`--background`, `--foreground`, `--card`,
`--primary`, `--border`, `--ring`, etc., all HSL triples) backs the
`components/ui/*` primitives (Button, Badge, Card, Input, Select, Switch,
Table, Tabs, Textarea, Tooltip) via Tailwind's `hsl(var(--x))` pattern in
`tailwind.config.js`. These are kept in sync with the hex tokens above —
same colors, different representation, because shadcn's variant system
expects HSL. When changing a color, update **both** the hex var and its
HSL counterpart in `index.css`.

### Legacy aliases — do not use in new code

`index.css` defines a block of backward-compat vars (`--fg`, `--color-safe`,
`--color-muted`, `--color-indigo`, `--bg-elevated`, etc.) that just alias
the tokens above. They exist because older pages use inline
`style={{color: 'var(--fg)'}}` instead of Tailwind utility classes. New
components should use the Tailwind utilities directly (`text-text-primary`,
`bg-surface`, `border-border-color` — see `tailwind.config.js`'s `colors`
block for the full utility-name mapping), not these aliases. The alias
block will be removed once every page is migrated; don't add new reliance
on it.

## Typography scale

No formal type-scale tokens — sizes are ad-hoc Tailwind arbitrary values
per component (`text-[0.68rem]`, `text-[0.78rem]`, `text-[1.1rem]`, etc.).
Observed conventions, not enforced:

- Page titles: `text-lg` to `text-xl font-bold`
- Section/card headers: `text-sm font-semibold`
- Body/description text: `text-sm` or `text-xs`, `text-text-secondary`
- Metadata/labels: `text-[0.6rem]` to `text-[0.7rem]`, uppercase,
  `text-text-muted`, often with `tracking-wide`
- Data/numbers: `font-mono font-bold`, sized up for emphasis
  (`text-[1.25rem]` to `text-2xl` for hero metrics)

## Spacing & radius

- Card/panel radius: `rounded-xl` (12px, matches `--radius: 0.75rem`)
  for top-level cards, `rounded-md` (6px) for buttons/inputs/badges
- Card padding: `p-4` to `p-6` depending on density
- Grid gaps: `gap-3` (dense stat rows) to `gap-4` (card grids)
- Page padding: `p-5` or `p-6`

## Motion

Two reusable keyframe utilities in `index.css`, both respect
`prefers-reduced-motion`:

- `.fg-entrance` (+`.fg-entrance-delay-{1-5}`) — 480ms fade-up-in,
  staggered by 60ms increments. Used on metric-card rows for a subtle
  cascade on load.
- `.fg-pulse-healthy` — 3s slow opacity pulse. Used on the sidebar's
  "engine status" green dot to read as alive, not static.

No animation library — plain CSS `@keyframes`. Don't reach for
Framer Motion or similar for small effects; follow this pattern instead.

## Components

### `components/ui/*` — shadcn-style primitives

Generic, unopinionated, variant-driven via `class-variance-authority`.
Use these for anything generic (buttons, badges, form inputs, tabs,
tooltips) instead of hand-rolling inline-styled elements.

- **Button** (`button.tsx`) — variants `default | destructive | outline |
  secondary | ghost | link`, sizes `default | sm | lg | icon`. Includes a
  `hover:-translate-y-0.5` micro-lift on all variants.
- **Badge** (`badge.tsx`) — variants `default | secondary | destructive |
  outline` plus severity-specific `critical | high | medium | low | safe`
  (hardcoded hex colors, not theme-token-driven — see note below).
- **Card** (`card.tsx`) — `Card`/`CardHeader`/`CardTitle`/
  `CardDescription`/`CardContent`/`CardFooter`, theme-token-backed
  (`bg-card`, `border-border`).
- Input, Select, Switch, Table, Tabs, Textarea, Tooltip — standard
  shadcn wrappers, all theme-token-backed.

**Known inconsistency**: `Badge`'s severity variants
(`critical`/`high`/`medium`/`low`/`safe`) use hardcoded hex values
(`#FF3D3D`, `#00FF87`, etc.) instead of the theme tokens above, so they
don't shift with light/dark mode the way the rest of the app does. Worth
fixing if touching this component, not fixed here since it's cosmetic
and out of this doc's scope.

### `components/*` — app-specific composed components

- **TopoBackground** — reusable low-opacity SVG contour-line texture
  (deterministic sine-perturbed paths, no `Math.random`). Takes
  `opacity` (default 0.06) and `lines` (default 10) props. Use for
  hero sections, empty states, auth screens — anywhere the spec calls
  for subtle brand texture behind sparse content. **Do not** use on
  dense data views (dashboard cards) — contour lines compete with real
  content there.
- **EmptyState** — icon + title + description + optional action button,
  with a `TopoBackground` baked in at low opacity. The standard "no data
  yet" pattern — use this instead of a one-off empty div.
- **MetricCard** — label + value + icon + trend indicator (up/down/flat
  with colored delta text). The standard stat-tile.
- **SecurityScore** — circular gauge (0-100), color interpolates
  critical→warning→success by score band.
- **SeverityBadge** / **StatusBadge** — small pill badges for
  CRITICAL/HIGH/MEDIUM/LOW/INFORMATIONAL and job-status states
  respectively. Prefer these over ad-hoc colored spans.
- **Sidebar** — the app nav shell. Exports `NAV_SECTIONS`/`STANDALONE`
  as the single source of truth for what's reachable — the command
  palette (`App.tsx`'s `CommandPalette`) is built from these, so adding
  a nav item here automatically makes it Cmd+K-searchable.
- **CommandMenu** / **Toast** — Cmd+K palette and toast-notification
  system, both mounted globally in `App.tsx`.
- **CopyButton** — copy-to-clipboard button with a checkmark
  micro-confirmation. Use for every code snippet / hash / JSON block
  that a user would plausibly copy — don't hand-roll clipboard calls.
- **NetworkGraph** — force-directed graph (via `react-force-graph-2d`),
  two modes: `ambient` (decorative, non-interactive, low-opacity,
  deterministic layout) for marketing pages, `data` (interactive,
  severity-colored nodes) for real dependency graphs.

## Layout patterns

- **Responsive grids**: always specify a mobile-first breakpoint on
  multi-column stat/card grids — `grid-cols-2 sm:grid-cols-4`, not a bare
  `grid-cols-4`. (A recurring bug class this session was exactly this
  omission — grids that looked fine at desktop width clipped content on
  tablet/mobile.)
- **Sidebar**: collapses to an icon-only rail below 768px (auto-collapsed
  on mount via `App.tsx`'s viewport check), full-width nav items above.
- **Page shell**: every real dashboard page renders inside `App.tsx`'s
  `AppShell` (`Sidebar` + `<main>`), except the three top-level
  bypass routes (`/welcome`, `/enterprise`, and the auth-gate/onboarding
  states) which render full-bleed with their own nav.

## What's intentionally NOT standardized

- Exact `text-[...]` pixel/rem sizes — copied per-component, not tokenized.
- Inline `style={{}}` objects still exist throughout older pages
  (`AiSecurityPage`, `IntegrationsPage`, etc.) using the legacy CSS-var
  aliases. New code should use Tailwind utility classes against the
  tokens in the table above instead — don't add more inline-style pages,
  but don't mass-migrate existing ones as a side effect of unrelated work
  either.

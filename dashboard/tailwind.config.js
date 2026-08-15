/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: [
    './index.html',
    './src/**/*.{ts,tsx}',
    './src/components/ui/**/*.{ts,tsx}',
  ],
  theme: {
    extend: {
      colors: {
        // Legacy dark-theme aliases — now point at the new CSS vars so any
        // remaining `bg-safe`/`bg-critical` utility usages stay coherent.
        safe: 'var(--success)',
        warn: 'var(--warning)',
        critical: 'var(--critical)',
        surface: 'var(--surface)',
        elevated: 'var(--surface-muted)',
        // New semantic tokens for the "Technical Security OS" design system —
        // use these Tailwind utilities (bg-primary-blue, text-text-secondary,
        // border-border-color, etc.) in new components going forward instead
        // of inline style={{}} objects.
        'bg-base': 'var(--bg-base)',
        'surface-muted': 'var(--surface-muted)',
        'border-color': 'var(--border-color)',
        'text-primary': 'var(--text-primary)',
        'text-secondary': 'var(--text-secondary)',
        'text-muted': 'var(--text-muted)',
        'primary-blue': 'var(--primary-blue)',
        'blue-light': 'var(--blue-light)',
        cyan: 'var(--cyan)',
        success: 'var(--success)',
        warning: 'var(--warning)',
        // shadcn/ui semantic colors (reference CSS variables)
        border: 'hsl(var(--border))',
        input: 'hsl(var(--input))',
        ring: 'hsl(var(--ring))',
        background: 'hsl(var(--background))',
        foreground: 'hsl(var(--foreground))',
        primary: {
          DEFAULT: 'hsl(var(--primary))',
          foreground: 'hsl(var(--primary-foreground))',
        },
        secondary: {
          DEFAULT: 'hsl(var(--secondary))',
          foreground: 'hsl(var(--secondary-foreground))',
        },
        destructive: {
          DEFAULT: 'hsl(var(--destructive))',
          foreground: 'hsl(var(--destructive-foreground))',
        },
        muted: {
          DEFAULT: 'hsl(var(--muted))',
          foreground: 'hsl(var(--muted-foreground))',
        },
        accent: {
          DEFAULT: 'hsl(var(--accent))',
          foreground: 'hsl(var(--accent-foreground))',
        },
        card: {
          DEFAULT: 'hsl(var(--card))',
          foreground: 'hsl(var(--card-foreground))',
        },
      },
      borderRadius: {
        lg: 'var(--radius)',
        md: 'calc(var(--radius) - 2px)',
        sm: 'calc(var(--radius) - 4px)',
      },
      fontFamily: {
        mono: ['JetBrains Mono', 'IBM Plex Mono', 'Fira Code', 'Consolas', 'monospace'],
      },
    },
  },
  plugins: [],
}

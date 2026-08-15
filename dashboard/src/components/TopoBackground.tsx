// Subtle contour-line brand texture. Pure SVG, no canvas/animation cost —
// a handful of overlapping sine-perturbed paths at very low opacity so it
// reads as texture, not content. Deterministic (no Math.random) so it's
// stable across re-renders and SSR-safe.
interface TopoBackgroundProps {
  className?: string;
  opacity?: number;
  lines?: number;
}

function contourPath(seed: number, width: number, height: number): string {
  const points: string[] = [];
  const amplitude = height * 0.06;
  const baseY = (seed / 10) * height;
  const freq = 0.004 + (seed % 3) * 0.0015;
  const phase = seed * 1.7;
  for (let x = 0; x <= width; x += width / 24) {
    const y = baseY + Math.sin(x * freq + phase) * amplitude + Math.sin(x * freq * 2.3 + phase) * (amplitude * 0.4);
    points.push(`${x.toFixed(1)},${y.toFixed(1)}`);
  }
  return `M${points.join(' L')}`;
}

export function TopoBackground({ className, opacity = 0.06, lines = 10 }: TopoBackgroundProps) {
  const width = 1600;
  const height = 900;
  return (
    <svg
      className={className}
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      aria-hidden="true"
      style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', opacity }}
    >
      {Array.from({ length: lines }, (_, i) => (
        <path
          key={i}
          d={contourPath(i, width, height)}
          fill="none"
          stroke="currentColor"
          strokeWidth={1}
        />
      ))}
    </svg>
  );
}

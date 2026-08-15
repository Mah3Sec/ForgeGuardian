import React from 'react';
import { LineChart, Line, ResponsiveContainer } from 'recharts';

interface StatCardProps {
  label: string;
  value: number | string;
  color?: string;
  sub?: string;
  icon?: React.ReactNode;
  delta?: string;
  deltaPositive?: boolean;
  sparklineData?: number[];
  sparklineColor?: string;
}

export function StatCard({
  label,
  value,
  color = 'var(--color-safe)',
  sub,
  icon,
  delta,
  deltaPositive,
  sparklineData,
  sparklineColor,
}: StatCardProps) {
  const sparkColor = sparklineColor ?? color;
  const sparkPoints = (sparklineData ?? []).map((v, i) => ({ i, v }));

  return (
    <div
      style={{
        background: 'var(--surface)',
        border: '1px solid rgba(255,255,255,0.06)',
        borderRadius: '0.5rem',
        padding: '1rem',
        display: 'flex',
        flexDirection: 'column',
        gap: '0.25rem',
        position: 'relative',
        minHeight: 110,
      }}
    >
      {icon && (
        <div style={{ position: 'absolute', top: '0.75rem', right: '0.75rem', color, opacity: 0.8 }}>
          {icon}
        </div>
      )}

      <span
        style={{
          color: 'var(--color-muted)',
          fontSize: '0.7rem',
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          fontFamily: 'var(--font-mono)',
        }}
      >
        {label}
      </span>

      <span
        style={{
          color,
          fontSize: '2rem',
          fontWeight: 700,
          fontFamily: 'var(--font-mono)',
          lineHeight: 1.1,
        }}
      >
        {typeof value === 'number' ? value.toLocaleString() : value}
      </span>

      {delta && (
        <span
          style={{
            fontSize: '0.68rem',
            color: deltaPositive ? 'var(--color-info)' : 'var(--color-critical)',
            fontFamily: 'var(--font-mono)',
          }}
        >
          {delta}
        </span>
      )}

      {sub && !delta && (
        <span style={{ fontSize: '0.7rem', color: 'var(--color-muted)' }}>{sub}</span>
      )}

      {sparkPoints.length > 1 && (
        <div style={{ marginTop: 'auto', height: 32, width: '100%' }}>
          <ResponsiveContainer width="100%" height={32}>
            <LineChart data={sparkPoints}>
              <Line
                type="monotone"
                dataKey="v"
                stroke={sparkColor}
                strokeWidth={1.5}
                dot={false}
                isAnimationActive={false}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}

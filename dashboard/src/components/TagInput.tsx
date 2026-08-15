import { useState, type KeyboardEvent } from 'react';
import { X } from 'lucide-react';

interface TagInputProps {
  value: string[];
  onChange: (v: string[]) => void;
  placeholder?: string;
  color?: string;
}

export function TagInput({ value, onChange, placeholder, color = '#FF3D3D' }: TagInputProps) {
  const [draft, setDraft] = useState('');

  const addTag = (raw: string) => {
    const tag = raw.trim();
    if (tag && !value.includes(tag)) onChange([...value, tag]);
    setDraft('');
  };

  const removeTag = (tag: string) => onChange(value.filter(t => t !== tag));

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      addTag(draft);
    } else if (e.key === 'Backspace' && draft === '' && value.length > 0) {
      removeTag(value[value.length - 1]);
    }
  };

  return (
    <div
      className="flex flex-wrap items-center gap-1.5 rounded px-2 py-1.5"
      style={{ background: 'var(--bg-base)', border: '1px solid rgba(255,255,255,0.12)' }}
    >
      {value.map(tag => (
        <span
          key={tag}
          className="flex items-center gap-1 text-xs font-mono px-2 py-1 rounded"
          style={{
            background: `${color}1A`,
            color,
            border: `1px solid ${color}40`,
          }}
        >
          {tag}
          <button
            type="button"
            onClick={() => removeTag(tag)}
            className="flex items-center"
            style={{ color, opacity: 0.8 }}
            aria-label={`Remove ${tag}`}
          >
            <X size={11} />
          </button>
        </span>
      ))}
      <input
        value={draft}
        onChange={e => setDraft(e.target.value)}
        onKeyDown={handleKeyDown}
        onBlur={() => draft && addTag(draft)}
        placeholder={value.length === 0 ? placeholder : ''}
        className="flex-1 min-w-[100px] text-sm font-mono bg-transparent outline-none"
        style={{ color: 'var(--fg)' }}
      />
    </div>
  );
}

import { useEffect, useMemo, useRef, useState } from 'react';
import { X, Search, LibraryBig } from 'lucide-react';

export interface ExampleMeta {
  slug: string;
  name: string;
  description: string;
}

interface ExamplesPickerProps {
  examples: ExampleMeta[];
  onSelect: (slug: string) => void;
  onClose: () => void;
}

export default function ExamplesPicker({ examples, onSelect, onClose }: ExamplesPickerProps) {
  const [query, setQuery] = useState('');
  const [highlightIndex, setHighlightIndex] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);
  const cardRefs = useRef<Array<HTMLButtonElement | null>>([]);

  useEffect(() => {
    searchRef.current?.focus();
  }, []);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const list = q
      ? examples.filter(
          (e) => e.name.toLowerCase().includes(q) || e.description.toLowerCase().includes(q) || e.slug.toLowerCase().includes(q)
        )
      : examples;
    return list.slice().sort((a, b) => a.name.localeCompare(b.name));
  }, [examples, query]);

  useEffect(() => {
    setHighlightIndex(0);
  }, [query]);

  useEffect(() => {
    cardRefs.current[highlightIndex]?.scrollIntoView({ block: 'nearest' });
  }, [highlightIndex]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlightIndex((i) => Math.min(i + 1, filtered.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlightIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const target = filtered[highlightIndex];
      if (target) {
        onSelect(target.slug);
        onClose();
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-lg rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col max-h-[70vh]"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <LibraryBig className="w-4 h-4 text-slate-400" />
            Example data models
          </h3>
          <button
            onClick={onClose}
            className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400"
            title="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="px-4 pt-3">
          <div className="relative">
            <Search className="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              ref={searchRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search example data models…"
              className="w-full pl-8 pr-2 py-1.5 rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-sm text-slate-900 dark:text-white outline-none"
            />
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-2 py-2 mt-1">
          {filtered.length === 0 && <p className="px-2 py-4 text-xs text-slate-400">No examples match.</p>}
          <div className="flex flex-col gap-0.5">
            {filtered.map((ex, i) => (
              <button
                key={ex.slug}
                ref={(el) => {
                  cardRefs.current[i] = el;
                }}
                onClick={() => {
                  onSelect(ex.slug);
                  onClose();
                }}
                onMouseEnter={() => setHighlightIndex(i)}
                onFocus={() => setHighlightIndex(i)}
                className={`w-full text-left px-3 py-2 rounded-md border text-xs flex flex-col gap-0.5 outline-none ${
                  i === highlightIndex
                    ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-500/10'
                    : 'border-transparent hover:bg-slate-100 dark:hover:bg-slate-800/60'
                }`}
              >
                <span className="font-mono font-medium text-slate-900 dark:text-white">{ex.name}</span>
                {ex.description && <span className="text-slate-400">{ex.description}</span>}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

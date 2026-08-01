import { useEffect, useMemo, useState } from 'react';
import { Search, Play, TextCursorInput, ClipboardCopy, ChevronDown, ChevronRight } from 'lucide-react';
import { fetchFunctionsCatalog, type FunctionCategory, type FunctionMeta } from '../lib/functionsCatalog';
import { apiFetch } from '../lib/api';

interface FunctionsTabProps {
  onToast: (message: string, type: 'error' | 'success') => void;
  isWrite: boolean;
  onInsert?: (snippet: string) => void;
}

function argNames(signature: string): string[] {
  const m = signature.match(/\(([^)]*)\)/);
  if (!m || !m[1].trim()) return [];
  return m[1].split(',').map((s) => s.trim());
}

async function tryFunction(name: string, args: unknown[]): Promise<unknown> {
  const res = await apiFetch('/functions/try', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, args }),
  });
  const body = await res.json();
  if (!body.ok) throw new Error(body.error?.message || 'Try failed');
  return body.data.result;
}

function FunctionCard({
  fn,
  isWrite,
  onToast,
  onInsert,
}: {
  fn: FunctionMeta;
  isWrite: boolean;
  onToast: (message: string, type: 'error' | 'success') => void;
  onInsert?: (snippet: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const args = useMemo(() => argNames(fn.signature), [fn.signature]);
  const [argValues, setArgValues] = useState<string[]>(() => args.map(() => ''));
  const [result, setResult] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const isAggregate = fn.aggregate;
  const callSnippet = `${fn.name}(${args.map((_, i) => argValues[i] || '').join(', ')})`;

  const runTry = async () => {
    setBusy(true);
    setResult(null);
    try {
      const parsedArgs = argValues.map((v) => {
        if (v === '') return null;
        const n = Number(v);
        return Number.isNaN(n) || v.trim() === '' ? v : n;
      });
      const r = await tryFunction(fn.name, parsedArgs);
      setResult(JSON.stringify(r));
    } catch (err: any) {
      onToast(err.message || 'Try failed', 'error');
    } finally {
      setBusy(false);
    }
  };

  const insertSnippet = (snippet: string, label: string) => {
    if (onInsert) {
      onInsert(snippet);
      onToast(`Inserted ${label}`, 'success');
      return;
    }
    navigator.clipboard?.writeText(snippet).then(
      () => onToast(`Copied ${label} to clipboard`, 'success'),
      () => onToast('Could not copy to clipboard', 'error'),
    );
  };

  const writeTitle = isWrite ? undefined : '--write required';

  return (
    <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-3 space-y-2">
      <div className="flex items-start justify-between gap-2">
        <div>
          <div className="font-mono text-sm text-slate-900 dark:text-white">{fn.signature}</div>
          <div className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">{fn.description}</div>
          <div className="text-xs text-slate-400 dark:text-slate-500 mt-1 font-mono">
            {fn.example.call} → {fn.example.result}
          </div>
        </div>
        <button
          onClick={() => setExpanded((v) => !v)}
          className="p-1 text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
          title={expanded ? 'Collapse' : 'Expand'}
        >
          {expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
        </button>
      </div>

      {expanded && (
        <div className="space-y-2 pt-1 border-t border-slate-100 dark:border-slate-800">
          {!isAggregate && (
            <div className="space-y-1.5">
              <div className="flex flex-wrap gap-1.5">
                {args.map((a, i) => (
                  <input
                    key={i}
                    value={argValues[i] ?? ''}
                    onChange={(e) =>
                      setArgValues((prev) => prev.map((v, idx) => (idx === i ? e.target.value : v)))
                    }
                    placeholder={a}
                    className="text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none w-24"
                  />
                ))}
                <button
                  onClick={runTry}
                  disabled={busy}
                  className="flex items-center gap-1 text-xs px-2 py-1 rounded bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400 hover:bg-indigo-100"
                >
                  <Play size={12} /> Run
                </button>
              </div>
              {result !== null && (
                <div className="text-xs font-mono px-2 py-1 rounded bg-slate-50 dark:bg-slate-900 text-slate-700 dark:text-slate-300">
                  {result}
                </div>
              )}
            </div>
          )}

          <div className="flex flex-wrap gap-1.5">
            <button
              onClick={() => insertSnippet(callSnippet, 'into editor')}
              className="flex items-center gap-1 text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700"
            >
              <TextCursorInput size={12} /> Insert
            </button>
            <button
              disabled={!isWrite}
              title={writeTitle}
              onClick={() => insertSnippet(`DEFAULT (${callSnippet})`, 'DEFAULT expression')}
              className="flex items-center gap-1 text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <ClipboardCopy size={12} /> Use as DEFAULT
            </button>
            {!isAggregate && (
              <button
                disabled={!isWrite}
                title={writeTitle}
                onClick={() =>
                  insertSnippet(`GENERATED ALWAYS AS (${callSnippet}) VIRTUAL`, 'generated column expression')
                }
                className="flex items-center gap-1 text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <ClipboardCopy size={12} /> Use as generated column
              </button>
            )}
            {!isAggregate && (
              <button
                disabled={!isWrite}
                title={writeTitle}
                onClick={() => insertSnippet(`CHECK (${callSnippet})`, 'CHECK constraint')}
                className="flex items-center gap-1 text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 hover:bg-slate-200 dark:hover:bg-slate-700 disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <ClipboardCopy size={12} /> Use as CHECK
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default function FunctionsTab({ onToast, isWrite, onInsert }: FunctionsTabProps) {
  const [categories, setCategories] = useState<FunctionCategory[]>([]);
  const [activeCategory, setActiveCategory] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  useEffect(() => {
    fetchFunctionsCatalog()
      .then(setCategories)
      .catch((err) => onToast(err.message || 'Failed to load functions', 'error'));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return categories
      .map((cat) => ({
        ...cat,
        functions: cat.functions.filter((fn) => {
          if (activeCategory && cat.name !== activeCategory) return false;
          if (!q) return true;
          return fn.name.toLowerCase().includes(q) || fn.description.toLowerCase().includes(q);
        }),
      }))
      .filter((cat) => cat.functions.length > 0);
  }, [categories, activeCategory, search]);

  return (
    <div className="flex gap-4 h-full">
      <div className="w-56 shrink-0 space-y-1">
        <button
          onClick={() => setActiveCategory(null)}
          className={`w-full text-left text-xs px-2 py-1.5 rounded-md ${
            activeCategory === null
              ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400'
              : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
          }`}
        >
          All categories
        </button>
        {categories.map((cat) => (
          <button
            key={cat.name}
            onClick={() => setActiveCategory(cat.name)}
            className={`w-full text-left text-xs px-2 py-1.5 rounded-md flex justify-between ${
              activeCategory === cat.name
                ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400'
                : 'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800'
            }`}
          >
            <span>{cat.name}</span>
            <span className="text-slate-400">{cat.functions.length}</span>
          </button>
        ))}
      </div>

      <div className="flex-1 space-y-3 overflow-auto">
        <div className="relative max-w-sm">
          <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search functions..."
            className="w-full text-sm pl-8 pr-3 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
          />
        </div>

        {filtered.map((cat) => (
          <div key={cat.name} className="space-y-2">
            <div className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wide">
              {cat.name}
            </div>
            <div className="grid gap-2">
              {cat.functions.map((fn) => (
                <FunctionCard key={fn.name} fn={fn} isWrite={isWrite} onToast={onToast} onInsert={onInsert} />
              ))}
            </div>
          </div>
        ))}
        {filtered.length === 0 && (
          <div className="text-sm text-slate-400 dark:text-slate-500 py-8 text-center">No matching functions.</div>
        )}
      </div>
    </div>
  );
}

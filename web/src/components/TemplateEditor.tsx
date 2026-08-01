import { useEffect, useRef, useState } from 'react';
import { Braces, Maximize2, X } from 'lucide-react';

interface TemplateEditorProps {
  columnName: string;
  generatorName: string;
  value: string;
  onChange: (next: string) => void;
  referencedColumns: string[];
  onPreview: () => Promise<Record<string, any> | null>;
  // All sibling columns + toggle handler, used to duplicate the referenced-
  // columns chip row inside the Expand sheet (which otherwise wouldn't have
  // access to it, since the "canonical" chip row lives in GeneratorOptionsForm
  // above the inline editor).
  allColumns: string[];
  onToggleColumn: (col: string) => void;
}

const BUILTIN_TOKENS = ['@uuid', '@bool', '@year', '@month'];

// Finds top-level {{...}} template tokens via balanced-brace scanning rather
// than a regex, since nested-generator calls like
// {{@oneOf({"values":"light,dark,system"})}} contain their own { }
// pairs inside the token that a naive "stop at the first }}" regex can't
// handle correctly.
function findTemplateTokens(text: string): Array<{ start: number; end: number }> {
  const tokens: Array<{ start: number; end: number }> = [];
  let i = 0;
  while (i < text.length) {
    if (text[i] === '{' && text[i + 1] === '{') {
      const start = i;
      let depth = 2;
      let j = i + 2;
      while (j < text.length && depth > 0) {
        if (text[j] === '{') depth++;
        else if (text[j] === '}') depth--;
        j++;
      }
      tokens.push({ start, end: j });
      i = j;
    } else {
      i++;
    }
  }
  return tokens;
}

// Tokenizer matching the mockup's 4-class scheme (tok-key / tok-str /
// tok-punc / tok-tmpl) — not a real JSON parser, just enough to color
// template text the same way internal-docs/seed-tab-ux-mockup.html does.
function tokenize(text: string): Array<{ text: string; cls: string }> {
  const tokens: Array<{ text: string; cls: string }> = [];
  const stringRe = /"(?:\\.|[^"\\])*"/g;

  const pushPlain = (segment: string) => {
    let last = 0;
    let match: RegExpExecArray | null;
    stringRe.lastIndex = 0;
    while ((match = stringRe.exec(segment))) {
      if (match.index > last) {
        tokens.push({ text: segment.slice(last, match.index), cls: 'tok-punc' });
      }
      // A quoted string followed by `:` (ignoring whitespace) is a JSON key.
      const after = segment.slice(stringRe.lastIndex).match(/^\s*:/);
      tokens.push({ text: match[0], cls: after ? 'tok-key' : 'tok-str' });
      last = stringRe.lastIndex;
    }
    if (last < segment.length) {
      tokens.push({ text: segment.slice(last), cls: 'tok-punc' });
    }
  };

  let cursor = 0;
  for (const t of findTemplateTokens(text)) {
    if (t.start > cursor) pushPlain(text.slice(cursor, t.start));
    tokens.push({ text: text.slice(t.start, t.end), cls: 'tok-tmpl' });
    cursor = t.end;
  }
  if (cursor < text.length) pushPlain(text.slice(cursor));
  return tokens;
}

// Pretty-prints a jsonTemplate's JSON while leaving {{...}} tokens intact —
// JSON.parse can't handle them directly (bare tokens like {{@bool}} aren't
// valid JSON values, and tokens can contain their own { } via nested
// generator calls), so each token is swapped for a placeholder JSON string,
// the result is parsed/re-stringified with indentation, then the
// placeholders are swapped back for the original raw token text. Returns
// null if the template (with tokens removed) isn't valid JSON.
function formatJsonTemplate(text: string): string | null {
  const tokens = findTemplateTokens(text);
  if (tokens.length === 0) {
    try {
      return JSON.stringify(JSON.parse(text), null, 2);
    } catch {
      return null;
    }
  }

  const placeholders: Array<{ marker: string; raw: string; alreadyQuoted: boolean }> = [];
  let placeholderText = '';
  let cursor = 0;
  tokens.forEach((t, idx) => {
    const raw = text.slice(t.start, t.end);
    const alreadyQuoted = text[t.start - 1] === '"' && text[t.end] === '"';
    // Plain ASCII, not a control character — JSON strings forbid raw control
    // characters (U+0000-U+001F) unescaped, which JSON.parse rejects outright.
    const marker = `__SQUAD_TPL_${idx}__`;
    placeholders.push({ marker, raw, alreadyQuoted });
    placeholderText += text.slice(cursor, t.start);
    placeholderText += alreadyQuoted ? marker : `"${marker}"`;
    cursor = t.end;
  });
  placeholderText += text.slice(cursor);

  let formatted: string;
  try {
    formatted = JSON.stringify(JSON.parse(placeholderText), null, 2);
  } catch {
    return null;
  }

  for (const p of placeholders) {
    formatted = formatted.split(p.alreadyQuoted ? p.marker : `"${p.marker}"`).join(p.raw);
  }
  return formatted;
}

const tokenClassMap: Record<string, string> = {
  'tok-key': 'text-amber-700 dark:text-yellow-400',
  'tok-str': 'text-green-700 dark:text-green-400',
  'tok-punc': 'text-slate-400 dark:text-slate-500',
  'tok-tmpl': 'text-indigo-600 dark:text-indigo-400 font-semibold',
};

function HighlightedCode({ text }: { text: string }) {
  return (
    <>
      {tokenize(text).map((tok, i) => (
        <span key={i} className={tokenClassMap[tok.cls]}>
          {tok.text}
        </span>
      ))}
      {/* Trailing newline so the overlay's height always matches the textarea's. */}
      {'\n'}
    </>
  );
}

function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);
  return debounced;
}

function EditorSurface({
  columnName,
  generatorName,
  value,
  onChange,
  referencedColumns,
  onPreview,
  onExpand,
  autoFocus,
}: TemplateEditorProps & { onExpand?: () => void; autoFocus?: boolean }) {
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const preRef = useRef<HTMLPreElement>(null);
  const [previewText, setPreviewText] = useState<string>('');
  const [previewLoading, setPreviewLoading] = useState(false);
  const [formatError, setFormatError] = useState(false);
  const debouncedValue = useDebouncedValue(value, 400);

  useEffect(() => {
    let cancelled = false;
    setPreviewLoading(true);
    onPreview()
      .then((row) => {
        if (cancelled) return;
        if (row) {
          const cell = row[columnName];
          setPreviewText(typeof cell === 'string' ? cell : JSON.stringify(cell));
        }
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setPreviewLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedValue]);

  const insertToken = (token: string) => {
    const el = textareaRef.current;
    if (!el) {
      onChange(value + token);
      return;
    }
    const start = el.selectionStart ?? value.length;
    const end = el.selectionEnd ?? value.length;
    const next = value.slice(0, start) + token + value.slice(end);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      const pos = start + token.length;
      el.setSelectionRange(pos, pos);
    });
  };

  const syncScroll = () => {
    if (textareaRef.current && preRef.current) {
      preRef.current.scrollTop = textareaRef.current.scrollTop;
      preRef.current.scrollLeft = textareaRef.current.scrollLeft;
    }
  };

  const handleFormat = () => {
    const formatted = formatJsonTemplate(value);
    if (formatted === null) {
      setFormatError(true);
      setTimeout(() => setFormatError(false), 1500);
      return;
    }
    onChange(formatted);
  };

  return (
    <div className="flex flex-col flex-1 min-h-[220px] border border-slate-300 dark:border-slate-700 rounded-lg overflow-hidden bg-slate-50 dark:bg-slate-950">
      <div className="flex items-center justify-between gap-2 px-2 py-1.5 bg-slate-100 dark:bg-slate-900 border-b border-slate-200 dark:border-slate-800">
        <div className="flex flex-wrap gap-1.5">
          {referencedColumns.map((col) => (
            <button
              key={col}
              type="button"
              onClick={() => insertToken(`{{$${col}}}`)}
              className="font-mono text-[11px] px-1.5 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-500 hover:text-indigo-500 hover:border-indigo-400 cursor-pointer"
            >
              ${col}
            </button>
          ))}
          {BUILTIN_TOKENS.map((fn) => (
            <button
              key={fn}
              type="button"
              onClick={() => insertToken(`{{${fn}}}`)}
              className="font-mono text-[11px] px-1.5 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-500 hover:text-indigo-500 hover:border-indigo-400 cursor-pointer"
            >
              {fn}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {formatError && (
            <span className="text-xs text-rose-500 whitespace-nowrap">Invalid JSON</span>
          )}
          {generatorName === 'jsonTemplate' && (
            <button
              type="button"
              onClick={handleFormat}
              title="Format JSON"
              className="flex items-center gap-1 text-xs font-semibold text-slate-500 hover:text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 px-1.5 py-1 rounded cursor-pointer"
            >
              <Braces className="w-3 h-3" />
            </button>
          )}
          {onExpand && (
            <button
              type="button"
              onClick={onExpand}
              className="flex items-center gap-1 text-xs font-semibold text-indigo-500 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 px-1.5 py-1 rounded cursor-pointer"
            >
              <Maximize2 className="w-3 h-3" />
              Expand
            </button>
          )}
        </div>
      </div>

      <div className="relative flex-1 min-h-[140px]">
        <pre
          ref={preRef}
          aria-hidden
          className="absolute inset-0 m-0 font-mono text-xs leading-relaxed p-3 whitespace-pre-wrap break-words overflow-auto pointer-events-none"
        >
          <HighlightedCode text={value} />
        </pre>
        <textarea
          ref={textareaRef}
          autoFocus={autoFocus}
          value={value}
          spellCheck={false}
          onChange={(e) => onChange(e.target.value)}
          onScroll={syncScroll}
          className="absolute inset-0 w-full h-full resize-none font-mono text-xs leading-relaxed p-3 bg-transparent text-transparent caret-slate-900 dark:caret-white outline-none whitespace-pre-wrap break-words"
        />
      </div>

      <div className="flex items-baseline gap-2 px-3 py-2 border-t border-slate-200 dark:border-slate-800 bg-slate-100 dark:bg-slate-900 font-mono text-[11px] text-slate-500 dark:text-slate-400 overflow-x-auto">
        <b className="font-sans font-semibold uppercase tracking-wide text-[10px] text-slate-400 dark:text-slate-500 shrink-0">
          Live
        </b>
        <span className="text-slate-700 dark:text-slate-300 whitespace-nowrap">
          {previewLoading && !previewText ? 'generating…' : previewText || '—'}
        </span>
      </div>
    </div>
  );
}

export default function TemplateEditor(props: TemplateEditorProps) {
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    if (!expanded) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setExpanded(false);
    };
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [expanded]);

  return (
    <>
      <EditorSurface {...props} onExpand={() => setExpanded(true)} />

      {expanded && (
        <>
          <div
            className="fixed inset-0 bg-slate-900/35 z-40"
            onClick={() => setExpanded(false)}
          />
          <div className="fixed top-0 right-0 h-full w-[min(760px,92vw)] bg-white dark:bg-slate-900 shadow-2xl z-50 flex flex-col">
            <div className="flex items-center justify-between px-5 py-4 border-b border-slate-200 dark:border-slate-800">
              <div className="flex flex-col gap-0.5">
                <span className="font-mono font-semibold text-sm text-slate-900 dark:text-white">
                  {props.columnName}
                </span>
                <span className="text-[11.5px] text-slate-400">{props.generatorName}</span>
              </div>
              <button
                type="button"
                onClick={() => setExpanded(false)}
                className="w-7 h-7 flex items-center justify-center rounded-md bg-slate-100 dark:bg-slate-800 text-slate-500 hover:text-slate-900 dark:hover:text-white cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex-1 p-5 flex flex-col gap-3 min-h-0">
              <div className="flex flex-col gap-1">
                <span className="text-xs text-slate-400">Referenced columns</span>
                <div className="flex flex-wrap gap-1.5">
                  {props.allColumns.map((col) => {
                    const isSelected = props.referencedColumns.includes(col);
                    return (
                      <button
                        key={col}
                        type="button"
                        onClick={() => props.onToggleColumn(col)}
                        className={`font-mono text-[11.5px] px-2.5 py-1 rounded-full border cursor-pointer select-none ${
                          isSelected
                            ? 'bg-indigo-50 dark:bg-indigo-500/15 border-indigo-300 dark:border-indigo-500/40 text-indigo-600 dark:text-indigo-400 font-semibold'
                            : 'bg-white dark:bg-slate-950 border-slate-300 dark:border-slate-700 text-slate-500'
                        }`}
                      >
                        {col}
                      </button>
                    );
                  })}
                </div>
              </div>
              <EditorSurface {...props} autoFocus />
            </div>
          </div>
        </>
      )}
    </>
  );
}

import { useState } from 'react';
import { X, AlertTriangle } from 'lucide-react';

/** A column name is "clean" if it's a simple identifier - anything else
 * (a raw, unaliased SQL expression like `concat(first_name,' ',last_name)`
 * or a multi-line `format(...)` call) makes for a poor CSV header / JSON
 * key and should be renamed before export. */
export function isCleanIdentifier(name: string): boolean {
  return /^[A-Za-z_][A-Za-z0-9_]*$/.test(name);
}

/** Suggests a friendly field name for a raw expression column, purely as a
 * starting point the user can edit - not an attempt to parse SQL. */
export function suggestFieldName(name: string, index: number): string {
  const cleaned = name
    .replace(/[^A-Za-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .toLowerCase();
  return cleaned || `field_${index + 1}`;
}

interface ExportFieldNamesModalProps {
  columns: string[];
  onCancel: () => void;
  onConfirm: (labels: string[]) => void;
}

export default function ExportFieldNamesModal({ columns, onCancel, onConfirm }: ExportFieldNamesModalProps) {
  const [labels, setLabels] = useState<string[]>(
    columns.map((col, i) => (isCleanIdentifier(col) ? col : suggestFieldName(col, i)))
  );

  const trimmed = labels.map((l) => l.trim());
  const duplicates = new Set(
    trimmed.filter((l, i) => l !== '' && trimmed.indexOf(l) !== i)
  );
  const hasEmpty = trimmed.some((l) => l === '');
  const canConfirm = !hasEmpty && duplicates.size === 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onCancel}>
      <div
        className="w-full max-w-lg max-h-[85vh] overflow-y-auto rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between sticky top-0 bg-white dark:bg-slate-900">
          <h3 className="font-semibold text-slate-900 dark:text-white">Name your export fields</h3>
          <button onClick={onCancel} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300 cursor-pointer">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-4 space-y-3 text-sm">
          <p className="text-slate-500 dark:text-slate-400 flex items-start gap-2">
            <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0 mt-0.5" />
            <span>
              Some of this query's columns are raw SQL expressions rather than simple names. Give them clearer
              field names before exporting - these only affect the export file, not the query itself.
            </span>
          </p>

          <div className="space-y-2">
            {columns.map((col, i) => {
              const clean = isCleanIdentifier(col);
              const value = trimmed[i];
              const isDuplicate = value !== '' && duplicates.has(value);
              return (
                <div key={i} className="flex flex-col gap-1">
                  <span
                    className={`text-xs font-mono truncate ${clean ? 'text-slate-500 dark:text-slate-400' : 'text-amber-600 dark:text-amber-400'}`}
                    title={col}
                  >
                    {col}
                  </span>
                  <input
                    type="text"
                    value={labels[i]}
                    onChange={(e) =>
                      setLabels((prev) => prev.map((l, idx) => (idx === i ? e.target.value : l)))
                    }
                    className={`px-2 py-1.5 rounded border bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none ${
                      value === '' || isDuplicate
                        ? 'border-red-400 dark:border-red-500/60'
                        : 'border-slate-300 dark:border-slate-700'
                    }`}
                  />
                  {value === '' && <span className="text-[11px] text-red-500">Field name cannot be empty.</span>}
                  {isDuplicate && <span className="text-[11px] text-red-500">Duplicate field name.</span>}
                </div>
              );
            })}
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
            <button
              onClick={onCancel}
              className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
            >
              Cancel
            </button>
            <button
              disabled={!canConfirm}
              onClick={() => onConfirm(trimmed)}
              className="px-3 py-1.5 rounded-md text-xs font-semibold shadow cursor-pointer text-white bg-indigo-600 hover:bg-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              Export
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

import { useState } from 'react';
import { X } from 'lucide-react';

export interface XmlExportOptions {
  rootTag: string;
  rowTag: string;
  caseStyle: 'snake' | 'camel' | 'pascal' | 'kebab';
  pretty: boolean;
  indentSize: number;
  includeDeclaration: boolean;
  nullHandling: 'empty' | 'omit';
}

// Mirrors internal/export's singularize() heuristic closely enough to
// prefill a sensible default row tag - the server applies its own version
// of this independently if rowTag is ever left empty, so this is purely a
// UI nicety, not the source of truth.
function singularize(name: string): string {
  const lower = name.toLowerCase();
  if (lower.endsWith('ies') && name.length > 3) return name.slice(0, -3) + 'y';
  if (['ses', 'xes', 'zes', 'ches', 'shes'].some((suf) => lower.endsWith(suf))) return name.slice(0, -2);
  if (lower.endsWith('s') && !lower.endsWith('ss') && name.length > 1) return name.slice(0, -1);
  return name;
}

export function defaultXmlExportOptions(tableName: string): XmlExportOptions {
  const root = tableName || 'rows';
  return {
    rootTag: root,
    rowTag: singularize(root),
    caseStyle: 'snake',
    pretty: true,
    indentSize: 4,
    includeDeclaration: true,
    nullHandling: 'empty',
  };
}

const CASE_OPTIONS: { id: XmlExportOptions['caseStyle']; label: string; example: string }[] = [
  { id: 'camel', label: 'Lower Camel Case', example: 'orderId' },
  { id: 'pascal', label: 'Pascal Case', example: 'OrderId' },
  { id: 'snake', label: 'Snake Case', example: 'order_id' },
  { id: 'kebab', label: 'Kebab Case', example: 'order-id' },
];

interface XmlExportModalProps {
  initial: XmlExportOptions;
  onCancel: () => void;
  onConfirm: (opts: XmlExportOptions) => void;
}

export default function XmlExportModal({ initial, onCancel, onConfirm }: XmlExportModalProps) {
  const [opts, setOpts] = useState<XmlExportOptions>(initial);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onCancel}>
      <div
        className="w-full max-w-lg rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900 dark:text-white">XML export options</h3>
          <button
            onClick={onCancel}
            className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300 cursor-pointer"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-4 space-y-4 text-sm text-slate-700 dark:text-slate-300">
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Root element</span>
              <input
                type="text"
                value={opts.rootTag}
                onChange={(e) => setOpts((o) => ({ ...o, rootTag: e.target.value }))}
                className="px-2 py-1.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none"
              />
            </label>
            <label className="flex flex-col gap-1">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">Row element</span>
              <input
                type="text"
                value={opts.rowTag}
                onChange={(e) => setOpts((o) => ({ ...o, rowTag: e.target.value }))}
                className="px-2 py-1.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none"
              />
            </label>
          </div>

          <div>
            <span className="text-xs font-medium text-slate-500 dark:text-slate-400 block mb-1.5">
              Tag naming convention
            </span>
            <div className="grid grid-cols-2 gap-2">
              {CASE_OPTIONS.map((c) => (
                <label
                  key={c.id}
                  className={`flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-md border cursor-pointer text-xs transition-colors ${
                    opts.caseStyle === c.id
                      ? 'border-indigo-500 bg-indigo-50/40 dark:bg-indigo-950/20'
                      : 'border-slate-200 dark:border-slate-700 hover:border-indigo-300 dark:hover:border-indigo-500/50'
                  }`}
                >
                  <span className="flex items-center gap-2">
                    <input
                      type="radio"
                      name="xml-case-style"
                      checked={opts.caseStyle === c.id}
                      onChange={() => setOpts((o) => ({ ...o, caseStyle: c.id }))}
                      className="text-indigo-600 focus:ring-indigo-500"
                    />
                    {c.label}
                  </span>
                  <span className="font-mono text-slate-400">{c.example}</span>
                </label>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={opts.pretty}
                onChange={(e) => setOpts((o) => ({ ...o, pretty: e.target.checked }))}
                className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
              />
              <span>Pretty-print (indented)</span>
            </label>
            <label
              className={`flex items-center gap-2 ${opts.pretty ? 'cursor-pointer' : 'opacity-40 cursor-not-allowed'}`}
            >
              <span className="text-xs text-slate-500 dark:text-slate-400">Indent</span>
              <select
                disabled={!opts.pretty}
                value={opts.indentSize}
                onChange={(e) => setOpts((o) => ({ ...o, indentSize: Number(e.target.value) }))}
                className="px-1.5 py-1 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-xs"
              >
                <option value={2}>2 spaces</option>
                <option value={4}>4 spaces</option>
              </select>
            </label>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <label className="flex items-center gap-2 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={opts.includeDeclaration}
                onChange={(e) => setOpts((o) => ({ ...o, includeDeclaration: e.target.checked }))}
                className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
              />
              <span>Include XML declaration</span>
            </label>
            <label className="flex items-center gap-2">
              <span className="text-xs text-slate-500 dark:text-slate-400">NULL values</span>
              <select
                value={opts.nullHandling}
                onChange={(e) =>
                  setOpts((o) => ({ ...o, nullHandling: e.target.value as XmlExportOptions['nullHandling'] }))
                }
                className="px-1.5 py-1 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-xs"
              >
                <option value="empty">Empty element</option>
                <option value="omit">Omit element</option>
              </select>
            </label>
          </div>

          <div className="flex justify-end gap-2 pt-2 border-t border-slate-100 dark:border-slate-800">
            <button
              onClick={onCancel}
              className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
            >
              Cancel
            </button>
            <button
              onClick={() => onConfirm(opts)}
              className="px-3 py-1.5 rounded-md text-xs font-semibold shadow cursor-pointer text-white bg-indigo-600 hover:bg-indigo-500"
            >
              Export XML
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

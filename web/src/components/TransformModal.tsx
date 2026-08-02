import { useState } from 'react';
import { X, Wand2 } from 'lucide-react';
import {
  applyTransform,
  isAsyncTransform,
  TRANSFORM_LABELS,
  type TransformParams,
  type TransformType,
} from '../lib/transforms';
import { apiFetch } from '../lib/api';

const TYPES: TransformType[] = [
  'regex_replace',
  'trim',
  'upper',
  'lower',
  'title',
  'null_to_empty',
  'empty_to_null',
  'date_format',
  'base64_encode',
  'base64_decode',
  'url_encode',
  'url_decode',
  'hash_sha256',
  'hash_md5',
  'uuid',
  'template',
];

interface TransformModalProps {
  /** Column being transformed. */
  column: string;
  /** All column names — when provided (and more than one), a column picker is shown so the user can retarget. */
  columnOptions?: string[];
  onColumnChange?: (column: string) => void;
  /** The rows in scope — either the whole column or the current selection. */
  scopeLabel: string;
  /** Current values for the column, one per affected row (in the same order as onApply's newValues). */
  currentValues: any[];
  isWrite: boolean;
  onCancel: () => void;
  /** Apply the transformed values directly via the existing row-update path. */
  onApplyDirect: (newValues: any[]) => Promise<void>;
  /** Copy the equivalent UPDATE statements to the clipboard instead of applying. */
  onCopyAsUpdateSQL: (newValues: any[]) => void;
}

export default function TransformModal({
  column,
  columnOptions,
  onColumnChange,
  scopeLabel,
  currentValues,
  isWrite,
  onCancel,
  onApplyDirect,
  onCopyAsUpdateSQL,
}: TransformModalProps) {
  const [type, setType] = useState<TransformType>('trim');
  const [params, setParams] = useState<TransformParams>({});
  const [preview, setPreview] = useState<any[] | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);

  const runTransform = async (): Promise<any[]> => {
    if (type === 'template') {
      const res = await apiFetch('/transform/template', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ template: params.template || '', values: currentValues }),
      });
      const body = await res.json();
      if (!res.ok || !body.ok) {
        throw new Error(body.error?.message || `HTTP error ${res.status}`);
      }
      return body.data.results;
    }
    if (isAsyncTransform(type)) {
      return Promise.all(currentValues.map((v) => applyTransform(type, v, params)));
    }
    return Promise.all(currentValues.map((v) => applyTransform(type, v, params)));
  };

  const runPreview = async () => {
    setPreviewing(true);
    setPreviewError(null);
    try {
      const result = await runTransform();
      setPreview(result);
    } catch (err: any) {
      setPreviewError(err.message || 'Transform failed');
      setPreview(null);
    } finally {
      setPreviewing(false);
    }
  };

  const applyDirect = async () => {
    setApplying(true);
    try {
      const result = preview ?? (await runTransform());
      await onApplyDirect(result);
    } catch (err: any) {
      setPreviewError(err.message || 'Transform failed');
    } finally {
      setApplying(false);
    }
  };

  const copyAsSQL = async () => {
    try {
      const result = preview ?? (await runTransform());
      onCopyAsUpdateSQL(result);
    } catch (err: any) {
      setPreviewError(err.message || 'Transform failed');
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onCancel}>
      <div
        className="w-full max-w-lg rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <Wand2 className="w-4 h-4 text-indigo-500" />
            Transform <span className="font-mono text-indigo-500">{column}</span>
          </h3>
          <button onClick={onCancel} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-4 space-y-3 max-h-[65vh] overflow-y-auto">
          <p className="text-xs text-slate-500 dark:text-slate-400">Scope: {scopeLabel}</p>

          {columnOptions && columnOptions.length > 1 && onColumnChange && (
            <label className="block">
              <span className="text-xs text-slate-500 dark:text-slate-400">Column</span>
              <select
                value={column}
                onChange={(e) => {
                  onColumnChange(e.target.value);
                  setPreview(null);
                  setPreviewError(null);
                }}
                className="mt-1 w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none font-mono"
              >
                {columnOptions.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            </label>
          )}

          <label className="block">
            <span className="text-xs text-slate-500 dark:text-slate-400">Transform</span>
            <select
              value={type}
              onChange={(e) => {
                setType(e.target.value as TransformType);
                setPreview(null);
                setPreviewError(null);
              }}
              className="mt-1 w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
            >
              {TYPES.map((t) => (
                <option key={t} value={t}>
                  {TRANSFORM_LABELS[t]}
                </option>
              ))}
            </select>
          </label>

          {type === 'regex_replace' && (
            <div className="space-y-2">
              <input
                type="text"
                placeholder="Regex pattern (e.g. [0-9]+)"
                value={params.pattern || ''}
                onChange={(e) => setParams((p) => ({ ...p, pattern: e.target.value }))}
                className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none font-mono"
              />
              <input
                type="text"
                placeholder="Replacement"
                value={params.replacement || ''}
                onChange={(e) => setParams((p) => ({ ...p, replacement: e.target.value }))}
                className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none font-mono"
              />
            </div>
          )}

          {type === 'date_format' && (
            <input
              type="text"
              placeholder="Output format (e.g. yyyy-MM-dd, dd/MM/yyyy HH:mm:ss)"
              value={params.outputFormat || ''}
              onChange={(e) => setParams((p) => ({ ...p, outputFormat: e.target.value }))}
              className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none font-mono"
            />
          )}

          {type === 'template' && (
            <textarea
              placeholder="{{upper .Value}} or {{uuid}}"
              value={params.template || ''}
              onChange={(e) => setParams((p) => ({ ...p, template: e.target.value }))}
              rows={3}
              className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none font-mono"
            />
          )}

          <button
            onClick={runPreview}
            disabled={previewing}
            className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer disabled:opacity-50"
          >
            {previewing ? 'Running preview…' : 'Preview'}
          </button>

          {previewError && <p className="text-xs text-red-500">{previewError}</p>}

          {preview && (
            <div className="border border-slate-200 dark:border-slate-800 rounded max-h-48 overflow-y-auto">
              <table className="w-full text-xs font-mono">
                <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 dark:text-slate-400 sticky top-0">
                  <tr>
                    <th className="px-2 py-1 text-left">Current</th>
                    <th className="px-2 py-1 text-left">New</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                  {currentValues.slice(0, 50).map((v, i) => (
                    <tr key={i}>
                      <td className="px-2 py-1 text-slate-500 dark:text-slate-400">{v === null ? 'NULL' : String(v)}</td>
                      <td className="px-2 py-1 text-slate-900 dark:text-white">
                        {preview[i] === null ? 'NULL' : String(preview[i])}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {currentValues.length > 50 && (
                <p className="px-2 py-1 text-[10px] text-slate-400">
                  Showing first 50 of {currentValues.length} affected rows.
                </p>
              )}
            </div>
          )}
        </div>
        <div className="px-4 py-3 border-t border-slate-200 dark:border-slate-800 flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
          >
            Cancel
          </button>
          <button
            onClick={copyAsSQL}
            className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
          >
            Generate UPDATE statement
          </button>
          <button
            onClick={applyDirect}
            disabled={!isWrite || applying}
            title={isWrite ? undefined : 'Write mode required'}
            className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow cursor-pointer disabled:opacity-50"
          >
            {applying ? 'Applying…' : 'Apply directly'}
          </button>
        </div>
      </div>
    </div>
  );
}

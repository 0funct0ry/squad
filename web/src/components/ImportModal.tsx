import { useEffect, useRef, useState, type DragEvent } from 'react';
import { Upload, X, AlertTriangle } from 'lucide-react';
import { apiFetch, apiUrl } from '../lib/api';

interface TableRef {
  name: string;
  type: 'table' | 'view';
}

interface ColumnInfo {
  name: string;
  type: string;
  notnull: boolean;
  defaultVal: string | null;
  pk: number;
}

interface InferredColumn {
  name: string;
  type: string;
}

interface PreviewData {
  columns: string[];
  rows: Record<string, any>[];
  truncated: boolean;
  totalRows: number;
  inferredColumns: InferredColumn[];
}

interface ImportModalProps {
  tables: TableRef[];
  defaultTableName?: string;
  onClose: () => void;
  onToast: (message: string, type: 'error' | 'success') => void;
  onImported: (tableName: string) => void;
}

type Format = 'csv' | 'json' | 'yaml';
type Mode = 'existing' | 'create';

const SKIP = '__skip__';

function detectFormat(filename: string): Format | null {
  const lower = filename.toLowerCase();
  if (lower.endsWith('.csv')) return 'csv';
  if (lower.endsWith('.json')) return 'json';
  if (lower.endsWith('.yaml') || lower.endsWith('.yml')) return 'yaml';
  return null;
}

export default function ImportModal({ tables, defaultTableName, onClose, onToast, onImported }: ImportModalProps) {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [isDraggingOver, setIsDraggingOver] = useState(false);
  const [format, setFormat] = useState<Format>('csv');
  const [formatAmbiguous, setFormatAmbiguous] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [preview, setPreview] = useState<PreviewData | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const [mode, setMode] = useState<Mode>('existing');
  const [targetTable, setTargetTable] = useState<string>(defaultTableName || '');
  const [targetSchema, setTargetSchema] = useState<ColumnInfo[] | null>(null);
  const [mapping, setMapping] = useState<Record<string, string>>({});
  const [missingColumns, setMissingColumns] = useState<string[] | null>(null);

  const [newTableName, setNewTableName] = useState<string>('');
  const [newColumns, setNewColumns] = useState<InferredColumn[]>([]);
  const [newColumnsEdited, setNewColumnsEdited] = useState(false);

  const [submitting, setSubmitting] = useState(false);

  const loadTargetSchema = async (tableName: string) => {
    if (!tableName) {
      setTargetSchema(null);
      return;
    }
    try {
      const res = await apiFetch(`/tables/${encodeURIComponent(tableName)}/schema`);
      const body = await res.json();
      if (body.ok) {
        setTargetSchema(body.data.columns);
      }
    } catch {
      setTargetSchema(null);
    }
  };

  const runPreview = async (f: File, fmt: Format) => {
    setPreviewLoading(true);
    setPreviewError(null);
    setPreview(null);
    setMissingColumns(null);
    try {
      const form = new FormData();
      form.append('file', f);
      form.append('format', fmt);
      const res = await fetch(apiUrl('/import/preview'), { method: 'POST', body: form });
      const body = await res.json();
      if (!res.ok || !body.ok) {
        throw new Error(body.error?.message || `HTTP error ${res.status}`);
      }
      setPreview(body.data);
      setNewColumns(body.data.inferredColumns || []);
      setNewColumnsEdited(false);
      // Default mapping: match file columns to target columns by name (case-insensitive), else skip.
      if (targetSchema) {
        const next: Record<string, string> = {};
        body.data.columns.forEach((fc: string) => {
          const match = targetSchema.find((c) => c.name.toLowerCase() === fc.toLowerCase());
          next[fc] = match ? match.name : SKIP;
        });
        setMapping(next);
      }
    } catch (err: any) {
      setPreviewError(err.message || 'Failed to preview file');
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleFile = async (f: File) => {
    setFile(f);
    const detected = detectFormat(f.name);
    setFormatAmbiguous(detected === null);
    const fmt = detected || 'csv';
    setFormat(fmt);
    await runPreview(f, fmt);
  };

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    if (!isDraggingOver) setIsDraggingOver(true);
  };

  const handleDragLeave = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDraggingOver(false);
  };

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsDraggingOver(false);
    const f = e.dataTransfer.files?.[0];
    if (f) handleFile(f);
  };

  const handleFormatChange = async (fmt: Format) => {
    setFormat(fmt);
    if (file) await runPreview(file, fmt);
  };

  const handleTargetTableChange = async (name: string) => {
    setTargetTable(name);
    setMissingColumns(null);
    await loadTargetSchema(name);
  };

  const handleMappingChange = (fileCol: string, targetCol: string) => {
    setMapping((prev) => ({ ...prev, [fileCol]: targetCol }));
  };

  const handleNewColumnChange = (idx: number, field: 'name' | 'type', value: string) => {
    setNewColumnsEdited(true);
    setNewColumns((prev) => prev.map((c, i) => (i === idx ? { ...c, [field]: value } : c)));
  };

  const handleImportIntoExisting = async () => {
    if (!file || !targetTable) return;
    setSubmitting(true);
    setMissingColumns(null);
    try {
      const form = new FormData();
      form.append('file', file);
      form.append('format', format);
      form.append('mapping', JSON.stringify(mapping));
      const res = await fetch(apiUrl(`/tables/${encodeURIComponent(targetTable)}/import`), {
        method: 'POST',
        body: form,
      });
      const body = await res.json();
      if (!res.ok || !body.ok) {
        if (body.error?.missingColumns) {
          setMissingColumns(body.error.missingColumns);
        }
        throw new Error(body.error?.message || `HTTP error ${res.status}`);
      }
      onToast(`Imported ${body.data.inserted} row${body.data.inserted === 1 ? '' : 's'} into "${targetTable}"`, 'success');
      onImported(targetTable);
    } catch (err: any) {
      onToast(err.message || 'Import failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  const handleCreateFromFile = async () => {
    if (!file || !newTableName.trim()) return;
    setSubmitting(true);
    try {
      const form = new FormData();
      form.append('file', file);
      form.append('format', format);
      form.append('name', newTableName.trim());
      if (newColumnsEdited) {
        form.append('columns', JSON.stringify(newColumns));
      }
      const res = await fetch(apiUrl('/tables/import'), { method: 'POST', body: form });
      const body = await res.json();
      if (!res.ok || !body.ok) {
        throw new Error(body.error?.message || `HTTP error ${res.status}`);
      }
      onToast(`Created "${body.data.table}" with ${body.data.inserted} row${body.data.inserted === 1 ? '' : 's'}`, 'success');
      onImported(body.data.table);
    } catch (err: any) {
      onToast(err.message || 'Import failed', 'error');
    } finally {
      setSubmitting(false);
    }
  };

  useEffect(() => {
    if (defaultTableName) {
      loadTargetSchema(defaultTableName);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!preview || !targetSchema) return;
    const next: Record<string, string> = {};
    preview.columns.forEach((fc) => {
      const match = targetSchema.find((c) => c.name.toLowerCase() === fc.toLowerCase());
      next[fc] = match ? match.name : SKIP;
    });
    setMapping(next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [targetSchema]);

  const existingTables = tables.filter((t) => t.type === 'table');

  // A drop anywhere on the overlay (not just the exact dropzone below) must
  // never fall through to the browser's default "open this file" behavior,
  // which would navigate the whole tab away and lose all app state.
  const preventDefaultDrag = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    e.stopPropagation();
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
      onDragOver={preventDefaultDrag}
      onDrop={preventDefaultDrag}
    >
      <div
        className="w-full max-w-2xl max-h-[90vh] overflow-y-auto rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
        onDragOver={preventDefaultDrag}
        onDrop={preventDefaultDrag}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between sticky top-0 bg-white dark:bg-slate-900 z-10">
          <h3 className="font-semibold text-slate-900 dark:text-white">Import data</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="p-4 space-y-4">
          {/* File picker */}
          {!file ? (
            <div
              onDragOver={handleDragOver}
              onDragEnter={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              className={`rounded-lg border-2 border-dashed p-8 text-center transition-colors ${
                isDraggingOver
                  ? 'border-indigo-500 bg-indigo-50/40 dark:bg-indigo-950/20'
                  : 'border-slate-300 dark:border-slate-700'
              }`}
            >
              <Upload className="w-8 h-8 mx-auto mb-2 text-slate-400" />
              <p className="text-sm text-slate-500 dark:text-slate-400 mb-3">
                {isDraggingOver ? 'Drop the file to import' : 'Drag & drop a CSV, JSON, or YAML file, or'}
              </p>
              <button
                onClick={() => fileInputRef.current?.click()}
                className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850"
              >
                Browse files
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept=".csv,.json,.yaml,.yml"
                className="hidden"
                onChange={(e) => {
                  const f = e.target.files?.[0];
                  if (f) handleFile(f);
                  e.target.value = '';
                }}
              />
            </div>
          ) : (
            <div className="flex items-center justify-between text-sm p-2 rounded border border-slate-200 dark:border-slate-800">
              <span className="font-mono text-slate-700 dark:text-slate-300">{file.name}</span>
              <div className="flex items-center gap-2">
                <label className="text-xs text-slate-500 dark:text-slate-400 flex items-center gap-1.5">
                  Format:
                  <select
                    value={format}
                    onChange={(e) => handleFormatChange(e.target.value as Format)}
                    className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 text-xs"
                  >
                    <option value="csv">CSV</option>
                    <option value="json">JSON</option>
                    <option value="yaml">YAML</option>
                  </select>
                </label>
                <button
                  onClick={() => {
                    setFile(null);
                    setPreview(null);
                  }}
                  className="text-xs text-slate-400 hover:text-red-500"
                >
                  Remove
                </button>
              </div>
            </div>
          )}

          {formatAmbiguous && file && (
            <div className="text-xs text-amber-600 dark:text-amber-400 flex items-center gap-1.5">
              <AlertTriangle className="w-3.5 h-3.5 shrink-0" /> Could not detect format from filename — please confirm it above.
            </div>
          )}

          {previewLoading && <p className="text-sm text-slate-400">Loading preview…</p>}
          {previewError && <p className="text-sm text-red-500">{previewError}</p>}

          {preview && (
            <>
              {/* Sample rows */}
              <div>
                <h4 className="text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-1">
                  Preview ({preview.totalRows} row{preview.totalRows === 1 ? '' : 's'}{preview.truncated ? ', showing sample' : ''})
                </h4>
                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto max-h-40">
                  <table className="w-full text-xs font-mono">
                    <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left sticky top-0">
                      <tr>
                        {preview.columns.map((c) => (
                          <th key={c} className="px-2 py-1 font-medium">{c}</th>
                        ))}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                      {preview.rows.slice(0, 5).map((row, i) => (
                        <tr key={i}>
                          {preview.columns.map((c) => (
                            <td key={c} className="px-2 py-1 text-slate-700 dark:text-slate-300">
                              {row[c] === null || row[c] === undefined ? (
                                <span className="text-slate-400 italic">NULL</span>
                              ) : (
                                String(row[c])
                              )}
                            </td>
                          ))}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Mode toggle */}
              <div className="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                <button
                  onClick={() => setMode('existing')}
                  className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                    mode === 'existing'
                      ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-650 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-500/30'
                      : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                  }`}
                >
                  Import into existing table
                </button>
                <button
                  onClick={() => setMode('create')}
                  className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                    mode === 'create'
                      ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-650 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-500/30'
                      : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                  }`}
                >
                  Create new table from file
                </button>
              </div>

              {mode === 'existing' ? (
                <div className="space-y-3">
                  <div className="flex flex-col gap-1 max-w-sm">
                    <label className="text-xs font-semibold text-slate-500 dark:text-slate-400">Target table</label>
                    <select
                      value={targetTable}
                      onChange={(e) => handleTargetTableChange(e.target.value)}
                      className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-sm"
                    >
                      <option value="">Select a table…</option>
                      {existingTables.map((t) => (
                        <option key={t.name} value={t.name}>{t.name}</option>
                      ))}
                    </select>
                  </div>

                  {missingColumns && missingColumns.length > 0 && (
                    <div className="text-xs text-red-600 dark:text-red-400 p-2 rounded border border-red-200 dark:border-red-900/40 bg-red-50 dark:bg-red-950/20">
                      Missing required column{missingColumns.length === 1 ? '' : 's'}: {missingColumns.join(', ')}
                    </div>
                  )}

                  {targetSchema && (
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden">
                      <table className="w-full text-xs">
                        <thead className="bg-slate-50 dark:bg-slate-800/40 text-slate-500 text-left">
                          <tr>
                            <th className="px-3 py-1.5 font-medium">File column</th>
                            <th className="px-3 py-1.5 font-medium">Target column</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono">
                          {preview.columns.map((fc) => (
                            <tr key={fc}>
                              <td className="px-3 py-1.5">{fc}</td>
                              <td className="px-3 py-1.5">
                                <select
                                  value={mapping[fc] ?? SKIP}
                                  onChange={(e) => handleMappingChange(fc, e.target.value)}
                                  className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 text-xs w-full"
                                >
                                  <option value={SKIP}>Skip</option>
                                  {targetSchema.map((c) => (
                                    <option key={c.name} value={c.name}>{c.name}</option>
                                  ))}
                                </select>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}

                  <button
                    onClick={handleImportIntoExisting}
                    disabled={!targetTable || submitting}
                    className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shadow"
                  >
                    {submitting ? 'Importing…' : 'Import'}
                  </button>
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="flex flex-col gap-1 max-w-sm">
                    <label className="text-xs font-semibold text-slate-500 dark:text-slate-400">New table name</label>
                    <input
                      type="text"
                      value={newTableName}
                      onChange={(e) => setNewTableName(e.target.value)}
                      placeholder="my_table"
                      className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-2 py-1 text-sm font-mono"
                    />
                  </div>

                  <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden">
                    <table className="w-full text-xs">
                      <thead className="bg-slate-50 dark:bg-slate-800/40 text-slate-500 text-left">
                        <tr>
                          <th className="px-3 py-1.5 font-medium">Column name</th>
                          <th className="px-3 py-1.5 font-medium">Type</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono">
                        {newColumns.map((col, idx) => (
                          <tr key={idx}>
                            <td className="px-3 py-1.5">
                              <input
                                type="text"
                                value={col.name}
                                onChange={(e) => handleNewColumnChange(idx, 'name', e.target.value)}
                                className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 text-xs w-full"
                              />
                            </td>
                            <td className="px-3 py-1.5">
                              <input
                                type="text"
                                list="type-affinities"
                                value={col.type}
                                onChange={(e) => handleNewColumnChange(idx, 'type', e.target.value)}
                                className="bg-white dark:bg-slate-950 border border-slate-300 dark:border-slate-700 rounded px-1.5 py-0.5 text-xs w-full"
                              />
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <button
                    onClick={handleCreateFromFile}
                    disabled={!newTableName.trim() || submitting}
                    className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shadow"
                  >
                    {submitting ? 'Creating…' : 'Create table & import'}
                  </button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

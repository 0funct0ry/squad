import { useEffect, useRef, useState } from 'react';
import {
  LibraryBig,
  Sigma,
  Clock,
  History,
  PanelRightClose,
  PanelRightOpen,
  ChevronDown,
  ChevronsUpDown,
  Maximize2,
  Minimize2,
  Terminal,
  Trash2,
} from 'lucide-react';
import { basicSetup } from 'codemirror';
import { sql as sqlLanguage } from '@codemirror/lang-sql';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import { udfCompletionSource } from '../lib/udfCompletion';
import { type ExampleMeta } from './ExamplesPicker';
import ExportFieldNamesModal, { isCleanIdentifier } from './ExportFieldNamesModal';
import RowGrid from './RowGrid';

export interface QueryResult {
  columns: string[];
  rows: any[][];
  rowsAffected: number;
  durationMs: number;
  limit: number;
  truncated: boolean;
  schemaChanged: boolean;
  sourceTable?: string;
  primaryKeyColumns?: string[];
}

export interface QueryHistoryEntry {
  sql: string;
  ranAt: Date;
  ok: boolean;
  durationMs?: number;
}

const themeCompartment = new Compartment();

const lightTheme = EditorView.theme({
  "&": {
    color: "#1e293b",
    backgroundColor: "#ffffff",
    height: "100%"
  },
  ".cm-content": {
    caretColor: "#4f46e5",
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
  },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "#4f46e5" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": { backgroundColor: "#e0e7ff" },
  ".cm-gutters": {
    backgroundColor: "#f8fafc",
    color: "#94a3b8",
    borderRight: "1px solid #e2e8f0"
  }
}, { dark: false });

const darkTheme = EditorView.theme({
  "&": {
    color: "#f1f5f9",
    backgroundColor: "#0f172a",
    height: "100%"
  },
  ".cm-content": {
    caretColor: "#818cf8",
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace"
  },
  ".cm-cursor, .cm-dropCursor": { borderLeftColor: "#818cf8" },
  "&.cm-focused .cm-selectionBackground, .cm-selectionBackground, ::selection": { backgroundColor: "#312e81" },
  ".cm-gutters": {
    backgroundColor: "#0b0f19",
    color: "#475569",
    borderRight: "1px solid #1e293b"
  }
}, { dark: true });

interface SqlEditorProps {
  value: string;
  onChange: (val: string) => void;
  onRun: (sql: string) => void;
  theme: 'light' | 'dark';
  editorViewRef: React.MutableRefObject<EditorView | null>;
}

export function SqlEditor({ value, onChange, onRun, theme, editorViewRef }: SqlEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const onRunRef = useRef(onRun);
  const onChangeRef = useRef(onChange);

  useEffect(() => {
    onRunRef.current = onRun;
  }, [onRun]);

  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    if (!containerRef.current) return;

    const sql = sqlLanguage();
    const startState = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        sql,
        sql.language.data.of({ autocomplete: udfCompletionSource }),
        themeCompartment.of(theme === 'dark' ? darkTheme : lightTheme),
        keymap.of([
          {
            key: "Mod-Enter",
            run: () => {
              const view = editorViewRef.current;
              if (view) {
                const selection = view.state.sliceDoc(
                  view.state.selection.main.from,
                  view.state.selection.main.to
                );
                const sqlToRun = selection || view.state.doc.toString();
                onRunRef.current(sqlToRun);
              }
              return true;
            }
          }
        ]),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            onChangeRef.current(update.state.doc.toString());
          }
        })
      ]
    });

    const view = new EditorView({
      state: startState,
      parent: containerRef.current
    });

    editorViewRef.current = view;

    return () => {
      view.destroy();
      editorViewRef.current = null;
    };
  }, []);

  useEffect(() => {
    const view = editorViewRef.current;
    if (view) {
      view.dispatch({
        effects: themeCompartment.reconfigure(theme === 'dark' ? darkTheme : lightTheme)
      });
    }
  }, [theme]);

  useEffect(() => {
    const view = editorViewRef.current;
    if (view && view.state.doc.toString() !== value) {
      view.dispatch({
        changes: { from: 0, to: view.state.doc.length, insert: value }
      });
    }
  }, [value]);

  return <div ref={containerRef} className="h-full font-mono text-sm [&_.cm-editor]:h-full" />;
}

function renderCell(val: any) {
  if (val === null || val === undefined) {
    return <span className="text-slate-400 dark:text-slate-600 italic">NULL</span>;
  }
  return String(val);
}

const MIN_EDITOR_HEIGHT = 120;
const MAX_EDITOR_HEIGHT_MARGIN = 260;
const DEFAULT_EDITOR_HEIGHT = 240;
const HEIGHT_STORAGE_KEY = 'squad:sqlEditorHeight';
const HISTORY_MODE_STORAGE_KEY = 'squad:sqlHistoryMode';

function iconButtonClass(active?: boolean) {
  return [
    'p-1.5 rounded-md border flex items-center justify-center cursor-pointer transition-colors shrink-0',
    active
      ? 'bg-indigo-50 dark:bg-indigo-500/15 border-indigo-300 dark:border-indigo-500/40 text-indigo-600 dark:text-indigo-400'
      : 'border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300',
  ].join(' ');
}

export interface SqlEditorPanelProps {
  sqlValue: string;
  onSqlChange: (v: string) => void;
  onRun: (sql: string) => void;
  runQueryFromEditor: () => void;
  queryLoading: boolean;
  queryResult: QueryResult | null;
  queryError: { code: string; message: string } | null;
  lastExecutedSql: string;
  queryHistory: QueryHistoryEntry[];
  setQueryHistory: React.Dispatch<React.SetStateAction<QueryHistoryEntry[]>>;
  setEditorContents: (text: string) => void;
  editorViewRef: React.MutableRefObject<EditorView | null>;
  theme: 'light' | 'dark';
  isWrite: boolean;
  examplesList: ExampleMeta[] | null;
  onOpenExamplesPicker: () => void;
  onOpenFunctionBrowser: () => void;
  exportQueryLoading: boolean;
  onQueryExport: (format: 'csv' | 'json') => void;
  pendingQueryExportFormat: 'csv' | 'json' | null;
  setPendingQueryExportFormat: (f: 'csv' | 'json' | null) => void;
  runQueryExport: (format: 'csv' | 'json', columnLabels?: string[]) => void;
  updateRow: (name: string, key: any, values: any) => Promise<any>;
  deleteRow: (name: string, key: any) => Promise<any>;
  bulkDeleteRows: (table: string, keys: Record<string, any>[]) => Promise<number>;
  showToast: (message: string, type: 'error' | 'success') => void;
}

export default function SqlEditorPanel({
  sqlValue,
  onSqlChange,
  onRun,
  runQueryFromEditor,
  queryLoading,
  queryResult,
  queryError,
  lastExecutedSql,
  queryHistory,
  setQueryHistory,
  setEditorContents,
  editorViewRef,
  theme,
  isWrite,
  examplesList,
  onOpenExamplesPicker,
  onOpenFunctionBrowser,
  exportQueryLoading,
  onQueryExport,
  pendingQueryExportFormat,
  setPendingQueryExportFormat,
  runQueryExport,
  updateRow,
  deleteRow,
  bulkDeleteRows,
  showToast,
}: SqlEditorPanelProps) {
  const [editorHeight, setEditorHeight] = useState<number>(() => {
    const raw = localStorage.getItem(HEIGHT_STORAGE_KEY);
    const n = raw ? parseInt(raw, 10) : NaN;
    return Number.isFinite(n) && n > 0 ? n : DEFAULT_EDITOR_HEIGHT;
  });
  const [historyMode, setHistoryMode] = useState<'sidebar' | 'popover'>(() => {
    const saved = localStorage.getItem(HISTORY_MODE_STORAGE_KEY);
    return saved === 'popover' ? 'popover' : 'sidebar';
  });
  const [historyVisible, setHistoryVisible] = useState(true);
  const [historyPopoverOpen, setHistoryPopoverOpen] = useState(false);
  const [resultsCollapsed, setResultsCollapsed] = useState(false);
  const [maximized, setMaximized] = useState(false);

  const dragState = useRef<{ startY: number; startHeight: number } | null>(null);
  const [dragging, setDragging] = useState(false);

  const onHandleMouseDown = (e: React.MouseEvent) => {
    dragState.current = { startY: e.clientY, startHeight: editorHeight };
    setDragging(true);
    document.body.style.userSelect = 'none';
  };

  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!dragState.current) return;
      const delta = e.clientY - dragState.current.startY;
      const clamped = Math.max(
        MIN_EDITOR_HEIGHT,
        Math.min(dragState.current.startHeight + delta, window.innerHeight - MAX_EDITOR_HEIGHT_MARGIN)
      );
      setEditorHeight(clamped);
    };
    const onUp = () => {
      if (dragState.current) {
        dragState.current = null;
        setDragging(false);
        document.body.style.userSelect = '';
        setEditorHeight((h) => {
          localStorage.setItem(HEIGHT_STORAGE_KEY, String(h));
          return h;
        });
      }
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
    };
  }, []);

  // Close the history popover on outside click / Escape.
  useEffect(() => {
    if (!historyPopoverOpen) return;
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (!target.closest('[data-history-popover-anchor]')) {
        setHistoryPopoverOpen(false);
      }
    };
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setHistoryPopoverOpen(false);
    };
    document.addEventListener('click', onDocClick);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('click', onDocClick);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [historyPopoverOpen]);

  const toggleHistoryMode = () => {
    const next = historyMode === 'sidebar' ? 'popover' : 'sidebar';
    setHistoryMode(next);
    localStorage.setItem(HISTORY_MODE_STORAGE_KEY, next);
    if (next === 'sidebar') setHistoryPopoverOpen(false);
  };

  const historyListContent = (
    <>
      <div className="flex items-center justify-between mb-2 shrink-0">
        <h3 className="text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider">
          Query History
        </h3>
        {queryHistory.length > 0 && (
          <button
            onClick={() => setQueryHistory([])}
            title="Clear all history"
            className="text-slate-400 hover:text-red-500 dark:hover:text-red-400 p-1 rounded transition-colors cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
      <div className="flex-1 overflow-y-auto space-y-2 pr-1">
        {queryHistory.length === 0 ? (
          <div className="text-xs text-slate-400 italic">No queries run this session.</div>
        ) : (
          queryHistory.map((item, idx) => (
            <div
              key={idx}
              onClick={() => {
                setEditorContents(item.sql);
                setHistoryPopoverOpen(false);
              }}
              className="p-2 rounded border border-slate-100 dark:border-slate-800 hover:border-indigo-500 dark:hover:border-indigo-500/50 hover:bg-slate-50 dark:hover:bg-slate-800/30 cursor-pointer flex flex-col gap-1 transition-all group relative"
            >
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setQueryHistory((prev) => prev.filter((_, i) => i !== idx));
                }}
                title="Remove this entry"
                className="absolute top-1.5 right-1.5 text-slate-300 dark:text-slate-600 hover:text-red-500 dark:hover:text-red-400 p-0.5 rounded opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
              >
                <Trash2 className="w-3 h-3" />
              </button>
              <div className="text-xs font-mono line-clamp-2 pr-4 text-slate-700 dark:text-slate-300 group-hover:text-indigo-650 dark:group-hover:text-indigo-400 break-all">
                {item.sql}
              </div>
              <div className="flex items-center justify-between text-[10px] text-slate-400">
                <span className="flex items-center gap-1">
                  <span className={item.ok ? "text-emerald-500" : "text-red-500"}>
                    {item.ok ? "✓" : "✗"}
                  </span>
                  {item.durationMs !== undefined ? `${item.durationMs.toFixed(1)}ms` : ''}
                </span>
                <span>{item.ranAt.toLocaleTimeString()}</span>
              </div>
            </div>
          ))
        )}
      </div>
    </>
  );

  const showSidebar = historyMode === 'sidebar' && historyVisible && !maximized;

  return (
    <section className="space-y-4 h-full flex flex-col min-h-0">
      <div className="flex gap-4 flex-1 min-h-0">
        {/* Left: Editor and Results */}
        <div className="flex-1 flex flex-col min-h-0 gap-4">
          <div
            className={`border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 flex flex-col ${maximized ? 'flex-1' : 'shrink-0'}`}
            style={maximized ? undefined : { height: editorHeight }}
          >
            <div className="flex items-center justify-between px-3 py-2 bg-slate-100 dark:bg-slate-850 text-sm border-b border-slate-200 dark:border-slate-800 shrink-0">
              <span className="font-medium text-slate-700 dark:text-slate-300">query.sql</span>
              <div className="flex items-center gap-2">
                {examplesList && (
                  <button
                    onClick={onOpenExamplesPicker}
                    className="px-2.5 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 font-medium text-xs flex items-center gap-1.5 cursor-pointer text-slate-700 dark:text-slate-300"
                    title="Insert an example data model"
                  >
                    <LibraryBig className="w-3.5 h-3.5" />
                    Examples
                  </button>
                )}
                <button
                  onClick={onOpenFunctionBrowser}
                  className="px-2.5 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 font-medium text-xs flex items-center gap-1 cursor-pointer text-slate-700 dark:text-slate-300 italic"
                  title="Function browser"
                >
                  <Sigma className="w-3.5 h-3.5 not-italic" />
                  fx
                </button>

                <span className="w-px self-stretch bg-slate-200 dark:bg-slate-700 mx-0.5" />

                <button
                  onClick={toggleHistoryMode}
                  className={iconButtonClass(historyMode === 'popover')}
                  title={
                    historyMode === 'sidebar'
                      ? 'History mode: sidebar (click to switch to popover)'
                      : 'History mode: popover (click to switch to sidebar)'
                  }
                >
                  <Clock className="w-3.5 h-3.5" />
                </button>

                {!maximized && historyMode === 'sidebar' && (
                  <button
                    onClick={() => setHistoryVisible((v) => !v)}
                    className={iconButtonClass(!historyVisible)}
                    title={historyVisible ? 'Hide history' : 'Show history'}
                  >
                    {historyVisible ? (
                      <PanelRightClose className="w-3.5 h-3.5" />
                    ) : (
                      <PanelRightOpen className="w-3.5 h-3.5" />
                    )}
                  </button>
                )}
                {!maximized && historyMode === 'popover' && (
                  <div className="relative" data-history-popover-anchor>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setHistoryPopoverOpen((o) => !o);
                      }}
                      className={iconButtonClass(historyPopoverOpen)}
                      title="Query history"
                    >
                      <History className="w-3.5 h-3.5" />
                    </button>
                    {historyPopoverOpen && (
                      <div className="absolute top-[calc(100%+6px)] right-0 w-[300px] max-h-[360px] bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-lg shadow-xl flex flex-col z-20 p-3">
                        {historyListContent}
                      </div>
                    )}
                  </div>
                )}

                <button
                  onClick={() => setResultsCollapsed((c) => !c)}
                  className={iconButtonClass(resultsCollapsed)}
                  title={resultsCollapsed ? 'Expand results' : 'Collapse results'}
                >
                  <ChevronsUpDown className="w-3.5 h-3.5" />
                </button>

                <span className="w-px self-stretch bg-slate-200 dark:bg-slate-700 mx-0.5" />

                <button
                  onClick={() => setMaximized((m) => !m)}
                  className={iconButtonClass(maximized)}
                  title={maximized ? 'Restore layout' : 'Maximize editor'}
                >
                  {maximized ? <Minimize2 className="w-3.5 h-3.5" /> : <Maximize2 className="w-3.5 h-3.5" />}
                </button>
                <button
                  onClick={runQueryFromEditor}
                  disabled={queryLoading}
                  className="px-3 py-1.5 rounded-md bg-indigo-600 text-white hover:bg-indigo-500 disabled:bg-indigo-400 font-medium text-xs flex items-center gap-1.5 cursor-pointer disabled:cursor-not-allowed"
                >
                  {queryLoading ? (
                    <span className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white border-t-transparent"></span>
                  ) : (
                    <span>Run ▸</span>
                  )}
                  <span className="opacity-70 text-[10px]">⌘↵</span>
                </button>
              </div>
            </div>
            <div className="flex-1 min-h-0 overflow-auto">
              <SqlEditor
                value={sqlValue}
                onChange={onSqlChange}
                onRun={onRun}
                theme={theme}
                editorViewRef={editorViewRef}
              />
            </div>
          </div>

          {!maximized && (
            <div
              onMouseDown={onHandleMouseDown}
              className="h-1.5 -my-2 shrink-0 cursor-row-resize flex items-center justify-center group z-[1]"
              title="Drag to resize"
            >
              <span
                className={`w-9 h-[3px] rounded-full transition-colors ${dragging ? 'bg-indigo-400' : 'bg-slate-300 dark:bg-slate-700 group-hover:bg-indigo-400'}`}
              />
            </div>
          )}

          {!maximized && (
            <>
              {/* Error Banner */}
              {queryError && (
                <div className="rounded-lg border border-red-300 dark:border-red-500/30 bg-red-50 dark:bg-red-500/10 text-red-700 dark:text-red-400 text-sm px-4 py-3 shrink-0 flex flex-col gap-1">
                  <div className="font-semibold">{queryError.code}</div>
                  <div className="font-mono text-xs whitespace-pre-wrap">{queryError.message}</div>
                </div>
              )}

              {/* Placeholder before first run */}
              {!queryResult && !queryLoading && !queryError && (
                <div className="flex-1 flex flex-col items-center justify-center text-center text-slate-400 dark:text-slate-600 gap-2">
                  <Terminal className="w-8 h-8" />
                  <p className="text-sm">Run a query to see results</p>
                </div>
              )}

              {/* Skeleton while query runs */}
              {queryLoading && (
                <div className="flex-1 flex flex-col min-h-0 gap-2 animate-pulse">
                  <div className="h-4 w-40 bg-slate-200 dark:bg-slate-800 rounded shrink-0" />
                  <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 flex-1 min-h-0 p-3 space-y-2">
                    <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                  </div>
                </div>
              )}

              {/* Results / Grid */}
              {queryResult && !queryLoading && (
                <div className={`flex flex-col min-h-0 gap-2 ${resultsCollapsed ? 'shrink-0' : 'flex-1'}`}>
                  {/* Status Bar */}
                  <div
                    onClick={() => setResultsCollapsed((c) => !c)}
                    className="flex items-center justify-between text-xs text-slate-500 shrink-0 border border-slate-200 dark:border-slate-800 rounded-lg px-3 py-2 bg-white dark:bg-slate-900 cursor-pointer select-none"
                  >
                    <div className="flex items-center gap-3">
                      <span className="flex items-center gap-1 text-emerald-600 dark:text-emerald-455 font-medium">
                        ● success
                      </span>
                      <span>{queryResult.durationMs.toFixed(1)} ms</span>
                      {queryResult.rowsAffected > 0 ? (
                        <span>{queryResult.rowsAffected} rows affected</span>
                      ) : (
                        <span>{queryResult.rows.length} rows</span>
                      )}
                      {queryResult.truncated && (
                        <span className="text-amber-600 dark:text-amber-400 font-medium">
                          (showing first {queryResult.limit} rows)
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      {queryResult.columns.length > 0 && (
                        <>
                          <span className="text-slate-400 dark:text-slate-500">Export:</span>
                          <button
                            disabled={exportQueryLoading}
                            onClick={(e) => {
                              e.stopPropagation();
                              onQueryExport('csv');
                            }}
                            className="px-2 py-0.5 rounded border border-slate-205 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 text-[11px] font-medium transition-colors cursor-pointer flex items-center gap-1"
                          >
                            {exportQueryLoading ? (
                              <span className="h-2.5 w-2.5 animate-spin rounded-full border border-slate-500 border-t-transparent"></span>
                            ) : null}
                            <span>CSV</span>
                          </button>
                          <button
                            disabled={exportQueryLoading}
                            onClick={(e) => {
                              e.stopPropagation();
                              onQueryExport('json');
                            }}
                            className="px-2 py-0.5 rounded border border-slate-205 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 text-[11px] font-medium transition-colors cursor-pointer flex items-center gap-1"
                          >
                            {exportQueryLoading ? (
                              <span className="h-2.5 w-2.5 animate-spin rounded-full border border-slate-500 border-t-transparent"></span>
                            ) : null}
                            <span>JSON</span>
                          </button>
                        </>
                      )}
                      <ChevronDown
                        className={`w-3.5 h-3.5 text-slate-400 transition-transform ${resultsCollapsed ? '-rotate-90' : ''}`}
                      />
                    </div>
                  </div>

                  {!resultsCollapsed && (
                    queryResult.sourceTable && queryResult.primaryKeyColumns && queryResult.primaryKeyColumns.length > 0 ? (
                      <RowGrid
                        columns={queryResult.columns}
                        rows={queryResult.rows}
                        isWrite={isWrite}
                        resetKey={lastExecutedSql}
                        getRowKey={(row) => {
                          const key: Record<string, any> = {};
                          (queryResult.primaryKeyColumns || []).forEach((pkCol) => {
                            const idx = queryResult.columns.indexOf(pkCol);
                            if (idx !== -1) key[pkCol] = row[idx];
                          });
                          return key;
                        }}
                        onSaveEdit={async (key, values) => {
                          try {
                            await updateRow(queryResult.sourceTable!, key, values);
                            showToast('Row updated successfully', 'success');
                            onRun(lastExecutedSql);
                          } catch (err: any) {
                            showToast(err.message || 'Failed to update row', 'error');
                          }
                        }}
                        onDeleteRow={async (key) => {
                          try {
                            await deleteRow(queryResult.sourceTable!, key);
                            showToast('Row deleted successfully', 'success');
                            onRun(lastExecutedSql);
                          } catch (err: any) {
                            showToast(err.message || 'Failed to delete row', 'error');
                          }
                        }}
                        onBulkDelete={async (keys) => {
                          try {
                            const deleted = await bulkDeleteRows(queryResult.sourceTable!, keys);
                            showToast(`${deleted} row${deleted === 1 ? '' : 's'} deleted`, 'success');
                            onRun(lastExecutedSql);
                          } catch (err: any) {
                            showToast(err.message || 'Failed to delete selected rows', 'error');
                          }
                        }}
                      />
                    ) : (
                      <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto bg-white dark:bg-slate-900 flex-1 min-h-0">
                        <table className="w-full text-sm font-mono relative">
                          <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 dark:text-slate-400 text-left sticky top-0 z-10">
                            <tr>
                              {queryResult.columns.map((col) => (
                                <th key={col} className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800">
                                  {col}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
                            {queryResult.rows.map((row, rIdx) => (
                              <tr key={rIdx} className="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                                {row.map((val, cIdx) => (
                                  <td key={cIdx} className="px-3 py-1.5 whitespace-nowrap overflow-hidden max-w-xs text-ellipsis">
                                    {renderCell(val)}
                                  </td>
                                ))}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )
                  )}
                </div>
              )}
            </>
          )}
        </div>

        {/* Right: History Panel */}
        {showSidebar && (
          <div className="w-64 border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 flex flex-col min-h-0 shrink-0">
            {historyListContent}
          </div>
        )}
      </div>

      {pendingQueryExportFormat && queryResult && (
        <ExportFieldNamesModal
          columns={queryResult.columns}
          onCancel={() => setPendingQueryExportFormat(null)}
          onConfirm={(labels) => {
            const format = pendingQueryExportFormat;
            setPendingQueryExportFormat(null);
            runQueryExport(format, labels);
          }}
        />
      )}
    </section>
  );
}

export { isCleanIdentifier };

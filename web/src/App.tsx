import { useEffect, useState, useRef } from 'react';
import { basicSetup } from 'codemirror';
import { sql as sqlLanguage } from '@codemirror/lang-sql';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';

interface MetaData {
  name: string;
  path: string;
  mode: 'ro' | 'rw';
  sqliteVersion: string;
  sizeBytes: number;
  pageSize: number;
  pageCount: number;
  encoding: string;
  journalMode: string;
  tableCount: number;
  viewCount: number;
}

interface TableInfo {
  name: string;
  type: 'table' | 'view';
  rowCount: number;
}

interface ColumnInfo {
  name: string;
  type: string;
  notnull: boolean;
  defaultVal: string | null;
  pk: number;
  hidden: number;
  generated: 'virtual' | 'stored' | null;
}

interface IndexInfo {
  name: string;
  unique: boolean;
  origin: string;
  partial: boolean;
  columns: string[];
  sql: string | null;
}

interface ForeignKeyInfo {
  id: number;
  seq: number;
  table: string;
  from: string;
  to: string;
  onUpdate: string;
  onDelete: string;
  match: string;
}

interface TriggerInfo {
  name: string;
  sql: string;
}

interface TableSchema {
  name: string;
  type: 'table' | 'view';
  rowCount: number;
  withoutRowid: boolean;
  columns: ColumnInfo[];
  primaryKey: string[];
  indexes: IndexInfo[];
  foreignKeys: ForeignKeyInfo[];
  triggers: TriggerInfo[];
  ddl: string;
}

interface RowsData {
  columns: string[];
  rows: any[][];
  total: number;
}

interface QueryResult {
  columns: string[];
  rows: any[][];
  rowsAffected: number;
  durationMs: number;
  limit: number;
  truncated: boolean;
}

interface QueryHistoryEntry {
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
    height: "200px"
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
    height: "200px"
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

function SqlEditor({ value, onChange, onRun, theme, editorViewRef }: SqlEditorProps) {
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

    const startState = EditorState.create({
      doc: value,
      extensions: [
        basicSetup,
        sqlLanguage(),
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

  return <div ref={containerRef} className="border border-slate-200 dark:border-slate-800 rounded-md overflow-hidden font-mono text-sm" />;
}

async function runQuery(sql: string, limit?: number): Promise<QueryResult> {
  const res = await fetch('/api/query', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sql, limit }),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function exportQuery(sql: string, format: string): Promise<Blob> {
  const res = await fetch(`/api/export/query?format=${format}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ sql }),
  });
  if (!res.ok) {
    let errBody;
    try {
      errBody = await res.json();
    } catch {
      throw new Error(`HTTP_ERROR: HTTP error ${res.status}`);
    }
    const err = errBody.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return await res.blob();
}

export default function App() {
  const [meta, setMeta] = useState<MetaData | null>(null);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTable, setSelectedTable] = useState<TableInfo | null>(null);
  const [schema, setSchema] = useState<TableSchema | null>(null);
  const [schemaError, setSchemaError] = useState<string | null>(null);
  const [blobModal, setBlobModal] = useState<{ column: string; hex: string } | null>(null);
  const [rowsData, setRowsData] = useState<RowsData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const saved = localStorage.getItem('color-scheme');
    if (saved === 'dark' || saved === 'light') return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });

  const [activeTab, setActiveTab] = useState<string>('data');
  const [searchQuery, setSearchQuery] = useState<string>('');
  
  // Pagination & Sorting & Filtering states
  const [page, setPage] = useState<number>(1);
  const [pageSize, setPageSize] = useState<number>(100);
  const [orderBy, setOrderBy] = useState<string>('');
  const [dir, setDir] = useState<'asc' | 'desc' | ''>('');
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [filterInputVisible, setFilterInputVisible] = useState<Record<string, boolean>>({});

  // Info Tab State
  const [infoLoading, setInfoLoading] = useState<boolean>(false);
  const [infoError, setInfoError] = useState<string | null>(null);
  const [infoSortBy, setInfoSortBy] = useState<'name' | 'rowCount'>('name');
  const [infoSortDir, setInfoSortDir] = useState<'asc' | 'desc'>('asc');
  const [toast, setToast] = useState<{ message: string; type: 'error' | 'success' } | null>(null);

  // Export States
  const [applyFilterSort, setApplyFilterSort] = useState<boolean>(false);
  const [includeSchema, setIncludeSchema] = useState<boolean>(false);
  const [selectedExportFormat, setSelectedExportFormat] = useState<'csv' | 'json' | 'sql'>('csv');
  const [exportQueryLoading, setExportQueryLoading] = useState<boolean>(false);
  const [lastExecutedSql, setLastExecutedSql] = useState<string>('');

  const fetchMetaAndTables = () => {
    setInfoLoading(true);
    setInfoError(null);
    Promise.all([
      fetch('/api/meta').then(res => res.json()),
      fetch('/api/tables').then(res => res.json())
    ])
      .then(([metaBody, tablesBody]) => {
        if (metaBody.ok && metaBody.data) {
          setMeta(metaBody.data);
        } else {
          throw new Error(metaBody.error?.message || 'Failed to fetch database metadata');
        }

        if (tablesBody.ok && tablesBody.data) {
          setTables(tablesBody.data);
          if (!selectedTable && tablesBody.data.length > 0) {
            setSelectedTable(tablesBody.data[0]);
          }
        } else {
          throw new Error(tablesBody.error?.message || 'Failed to fetch database tables');
        }
      })
      .catch((err) => {
        console.error(err);
        setInfoError(err.message || 'Failed to fetch database info');
        setError(err.message || 'Failed to fetch database info');
        setToast({ message: err.message || 'Failed to fetch database info', type: 'error' });
        setTimeout(() => setToast(null), 5000);
      })
      .finally(() => {
        setInfoLoading(false);
        setLoading(false);
      });
  };

  // SQL Editor state
  const [queryHistory, setQueryHistory] = useState<QueryHistoryEntry[]>([]);
  const [sqlValue, setSqlValue] = useState<string>('SELECT * FROM sqlite_master LIMIT 10;');
  const [queryResult, setQueryResult] = useState<QueryResult | null>(null);
  const [queryError, setQueryError] = useState<{ code: string; message: string } | null>(null);
  const [queryLoading, setQueryLoading] = useState<boolean>(false);
  const editorViewRef = useRef<EditorView | null>(null);

  const handleExecuteQuery = async (sqlToRun: string) => {
    if (queryLoading || !sqlToRun.trim()) return;

    setQueryLoading(true);
    setQueryError(null);
    setQueryResult(null);

    try {
      const data = await runQuery(sqlToRun);
      setQueryResult(data);
      setLastExecutedSql(sqlToRun);
      setQueryHistory((prev) => [
        {
          sql: sqlToRun,
          ranAt: new Date(),
          ok: true,
          durationMs: data.durationMs,
        },
        ...prev,
      ]);
    } catch (err: any) {
      let code = 'SQL_ERROR';
      let message = err.message;
      const colonIdx = err.message.indexOf(':');
      if (colonIdx !== -1) {
        code = err.message.substring(0, colonIdx).trim();
        message = err.message.substring(colonIdx + 1).trim();
      }

      setQueryError({ code, message });
      setQueryHistory((prev) => [
        {
          sql: sqlToRun,
          ranAt: new Date(),
          ok: false,
        },
        ...prev,
      ]);
    } finally {
      setQueryLoading(false);
    }
  };

  const runQueryFromEditor = () => {
    const view = editorViewRef.current;
    if (!view) return;
    const selection = view.state.sliceDoc(
      view.state.selection.main.from,
      view.state.selection.main.to
    );
    const sqlToRun = selection || view.state.doc.toString();
    handleExecuteQuery(sqlToRun);
  };

  const handleQueryExport = async (format: 'csv' | 'json') => {
    if (!lastExecutedSql) return;
    setExportQueryLoading(true);
    try {
      const blob = await exportQuery(lastExecutedSql, format);
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `query-export.${format}`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
      setToast({ message: 'Query exported successfully', type: 'success' });
      setTimeout(() => setToast(null), 3000);
    } catch (err: any) {
      console.error(err);
      setToast({ message: err.message || 'Failed to export query result', type: 'error' });
      setTimeout(() => setToast(null), 5000);
    } finally {
      setExportQueryLoading(false);
    }
  };

  useEffect(() => {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
      localStorage.setItem('color-scheme', 'dark');
    } else {
      document.documentElement.classList.remove('dark');
      localStorage.setItem('color-scheme', 'light');
    }
  }, [theme]);

  // Fetch Meta & Tables
  useEffect(() => {
    fetchMetaAndTables();
  }, []);

  // Refetch when entering Info tab
  useEffect(() => {
    if (activeTab === 'info') {
      fetchMetaAndTables();
    }
  }, [activeTab]);

  // Fetch schema and reset rows parameters on table change
  useEffect(() => {
    if (!selectedTable) return;
    setSchema(null);
    setSchemaError(null);
    setRowsData(null);
    setPage(1);
    setOrderBy('');
    setDir('');
    setFilters({});
    setFilterInputVisible({});

    fetch(`/api/tables/${selectedTable.name}/schema`)
      .then(res => res.json())
      .then(body => {
        if (body.ok && body.data) {
          setSchema(body.data);
        } else {
          const message = body.error?.message || 'Failed to fetch table schema';
          console.error(message);
          setSchemaError(message);
        }
      })
      .catch((err) => {
        console.error(err);
        setSchemaError(err.message || 'Failed to fetch table schema');
      });
  }, [selectedTable]);

  // Fetch rows
  useEffect(() => {
    if (!selectedTable) return;

    const offset = (page - 1) * pageSize;
    let url = `/api/tables/${selectedTable.name}/rows?limit=${pageSize}&offset=${offset}`;
    if (orderBy) {
      url += `&orderBy=${orderBy}&dir=${dir}`;
    }
    Object.entries(filters).forEach(([col, val]) => {
      if (val) {
        url += `&filter[${col}]=${encodeURIComponent(val)}`;
      }
    });

    fetch(url)
      .then(res => res.json())
      .then(body => {
        if (body.ok && body.data) {
          setRowsData(body.data);
        } else {
          console.error(body.error?.message);
        }
      })
      .catch(console.error);
  }, [selectedTable, page, pageSize, orderBy, dir, filters]);

  const toggleTheme = () => {
    setTheme((prev) => (prev === 'light' ? 'dark' : 'light'));
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const handleSort = (colName: string) => {
    if (orderBy === colName) {
      if (dir === 'asc') setDir('desc');
      else if (dir === 'desc') {
        setOrderBy('');
        setDir('');
      }
    } else {
      setOrderBy(colName);
      setDir('asc');
    }
    setPage(1);
  };

  const handleFilterChange = (colName: string, value: string) => {
    setFilters(prev => ({ ...prev, [colName]: value }));
    setPage(1);
  };

  const formatHexDump = (hexStr: string): string => {
    const bytes: number[] = [];
    for (let i = 0; i < hexStr.length; i += 2) {
      bytes.push(parseInt(hexStr.substring(i, i + 2), 16));
    }
    const lines: string[] = [];
    for (let offset = 0; offset < bytes.length; offset += 16) {
      const chunk = bytes.slice(offset, offset + 16);
      const hexPart = chunk
        .map((b) => b.toString(16).padStart(2, '0'))
        .join(' ')
        .padEnd(47, ' ');
      const asciiPart = chunk
        .map((b) => (b >= 0x20 && b <= 0x7e ? String.fromCharCode(b) : '.'))
        .join('');
      lines.push(`${offset.toString(16).padStart(8, '0')}  ${hexPart}  ${asciiPart}`);
    }
    return lines.join('\n') || '(empty)';
  };

  const renderCell = (val: any) => {
    if (val === null || val === undefined) {
      return <span className="text-slate-400 dark:text-slate-600 italic">NULL</span>;
    }
    return String(val);
  };

  if (loading) {
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-4 border-indigo-500 border-t-transparent"></div>
          <span className="text-sm font-medium">Loading squad metadata...</span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200">
        <div className="max-w-md rounded-lg border border-red-200 bg-red-50 p-6 dark:border-red-900/50 dark:bg-red-950/20">
          <h2 className="text-lg font-semibold text-red-700 dark:text-red-400">Failed to connect to backend</h2>
          <p className="mt-2 text-sm text-red-600 dark:text-red-300/80">{error}</p>
          <button
            onClick={() => window.location.reload()}
            className="mt-4 rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-500"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  const dbName = meta?.name || 'database.db';
  const isWrite = meta?.mode === 'rw';
  const sqliteVer = meta?.sqliteVersion || 'unknown';
  const dbSize = meta ? formatBytes(meta.sizeBytes) : '0 B';

  const filteredTables = tables.filter((t) =>
    t.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const totalRows = rowsData?.total || 0;
  const totalPages = Math.ceil(totalRows / pageSize) || 1;

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      alert('Copied to clipboard!');
    });
  };

  return (
    <div className="flex flex-col h-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 antialiased font-sans">
      {/* ============ HEADER ============ */}
      <header className="flex items-center justify-between px-4 h-12 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shrink-0">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded bg-gradient-to-br from-indigo-500 to-sky-500 flex items-center justify-center text-white font-bold text-sm">s</div>
            <span className="font-semibold tracking-tight text-slate-900 dark:text-white">squad</span>
          </div>
          <span
            className={`text-xs px-2 py-0.5 rounded-full font-medium ${
              isWrite
                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400'
                : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400'
            }`}
          >
            {isWrite ? 'WRITE MODE' : 'READ-ONLY'}
          </span>
          <span className="font-mono text-xs text-slate-400 dark:text-slate-500 hidden sm:inline">
            {dbName} · sqlite {sqliteVer} · {dbSize}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-slate-400 hidden md:inline">127.0.0.1:7071</span>
          <button
            onClick={toggleTheme}
            className="w-8 h-8 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center transition-colors"
            title="Toggle theme"
          >
            <span className="text-base">{theme === 'light' ? '🌙' : '☀️'}</span>
          </button>
        </div>
      </header>

      <div className="flex flex-1 min-h-0">
        {/* ============ SIDEBAR ============ */}
        <aside className="w-60 border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 flex flex-col shrink-0">
          <div className="p-2">
            <input
              placeholder="Search tables…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full text-sm px-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
            />
          </div>
          <div className="px-3 pt-1 pb-1 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
            Tables & Views
          </div>
          <nav className="flex-1 overflow-y-auto px-2 text-sm space-y-0.5">
            {filteredTables.map((t) => (
              <div
                key={t.name}
                onClick={() => setSelectedTable(t)}
                className={`flex items-center justify-between px-2 py-1.5 rounded-md cursor-pointer ${
                  selectedTable?.name === t.name
                    ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-300'
                    : 'hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-700 dark:text-slate-300'
                }`}
              >
                <span className="flex items-center gap-2">
                  <span className="text-slate-400 dark:text-slate-500">
                    {t.type === 'view' ? '◫' : '▤'}
                  </span>
                  <span className="font-medium font-mono text-xs">{t.name}</span>
                </span>
                <span className="text-xs text-slate-400 font-mono">{t.rowCount.toLocaleString()}</span>
              </div>
            ))}
          </nav>
        </aside>

        {/* ============ MAIN CONTENT ============ */}
        <main className="flex-1 flex flex-col min-w-0 bg-slate-50 dark:bg-slate-950">
          {/* Tabs */}
          <div className="flex items-center gap-1 px-3 h-10 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-x-auto shrink-0">
            {[
              { id: 'data', label: 'Data' },
              { id: 'schema', label: 'Schema' },
              { id: 'sql', label: 'SQL Editor' },
              { id: 'editor', label: 'Table Editor' },
              { id: 'seed', label: 'Seed' },
              { id: 'export', label: 'Export' },
              { id: 'rest', label: 'REST' },
              { id: 'codegen', label: 'Code Gen' },
              { id: 'info', label: 'Info' },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`px-3 py-1.5 rounded-md text-xs font-medium white-space-nowrap ${
                  activeTab === tab.id
                    ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400'
                    : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                }`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex-1 overflow-auto p-4">
            {/* DATA PANEL */}
            {activeTab === 'data' && selectedTable && (
              <section className="space-y-4 h-full flex flex-col min-h-0">
                <div className="flex items-center justify-between shrink-0">
                  <h2 className="font-semibold text-slate-900 dark:text-white">
                    <span className="font-mono text-indigo-500">{selectedTable.name}</span>{' '}
                    <span className="text-xs text-slate-400 font-normal">
                      {totalRows.toLocaleString()} rows
                    </span>
                  </h2>
                  <div className="flex items-center gap-2 text-sm">
                    <button
                      className={`px-2.5 py-1 rounded-md border border-slate-200 dark:border-slate-700 ${
                        isWrite ? 'hover:bg-slate-100 dark:hover:bg-slate-850' : 'opacity-50 cursor-not-allowed'
                      }`}
                      title={isWrite ? 'Add new row' : 'Write mode required'}
                      disabled={!isWrite}
                    >
                      + Row
                    </button>
                  </div>
                </div>

                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto bg-white dark:bg-slate-900 flex-1 min-h-0">
                  <table className="w-full text-sm font-mono relative">
                    <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 dark:text-slate-400 text-left sticky top-0 z-10">
                      <tr>
                        {rowsData?.columns.map((col) => {
                          return (
                            <th key={col} className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800">
                              <div className="flex flex-col gap-1">
                                <div
                                  className="flex items-center gap-1 cursor-pointer select-none hover:text-indigo-500"
                                  onClick={() => handleSort(col)}
                                >
                                  <span>{col}</span>
                                  <span className="text-xs">
                                    {orderBy === col ? (dir === 'asc' ? '▲' : '▼') : '↕'}
                                  </span>
                                </div>
                                <div className="mt-1 font-normal">
                                  {filterInputVisible[col] ? (
                                    <input
                                      type="text"
                                      placeholder="Filter..."
                                      value={filters[col] || ''}
                                      onChange={(e) => handleFilterChange(col, e.target.value)}
                                      className="text-xs font-normal px-1 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-900 dark:text-white outline-none w-24"
                                    />
                                  ) : (
                                    <button
                                      onClick={() => setFilterInputVisible(prev => ({ ...prev, [col]: true }))}
                                      className="text-[10px] text-slate-400 hover:text-indigo-500 font-normal px-1 rounded border border-transparent hover:border-slate-200 dark:hover:border-slate-800"
                                    >
                                      🔍 Filter
                                    </button>
                                  )}
                                </div>
                              </div>
                            </th>
                          );
                        })}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
                      {rowsData?.rows.map((row, rIdx) => (
                        <tr key={rIdx} className="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                          {row.map((val, cIdx) => {
                            const colName = rowsData.columns[cIdx];
                            const colType = schema?.columns.find(c => c.name === colName)?.type || '';
                            const isBlob = colType.toLowerCase() === 'blob';

                            if (isBlob && val !== null) {
                              const bytesCount = typeof val === 'string' ? Math.ceil(val.length / 2) : 0;
                              return (
                                <td key={cIdx} className="px-3 py-1.5">
                                  <button
                                    onClick={() => setBlobModal({ column: colName, hex: String(val) })}
                                    className="text-amber-600 dark:text-amber-400 underline decoration-dotted hover:text-amber-500 dark:hover:text-amber-300"
                                  >
                                    BLOB ({bytesCount} bytes)
                                  </button>
                                </td>
                              );
                            }

                            return (
                              <td key={cIdx} className="px-3 py-1.5 whitespace-nowrap overflow-hidden max-w-xs text-ellipsis">
                                {renderCell(val)}
                              </td>
                            );
                          })}
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>

                <div className="flex items-center justify-between text-sm text-slate-500 shrink-0">
                  <span>
                    Rows {totalRows === 0 ? 0 : (page - 1) * pageSize + 1}–
                    {Math.min(page * pageSize, totalRows)} of {totalRows.toLocaleString()}
                  </span>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => setPage(p => Math.max(p - 1, 1))}
                      disabled={page === 1}
                      className="px-2 py-1 rounded border border-slate-200 dark:border-slate-700 disabled:opacity-50"
                    >
                      ←
                    </button>
                    <span className="px-2">
                      {page} / {totalPages}
                    </span>
                    <button
                      onClick={() => setPage(p => Math.min(p + 1, totalPages))}
                      disabled={page === totalPages}
                      className="px-2 py-1 rounded border border-slate-200 dark:border-slate-700 disabled:opacity-50"
                    >
                      →
                    </button>
                    <select
                      value={pageSize}
                      onChange={(e) => {
                        setPageSize(Number(e.target.value));
                        setPage(1);
                      }}
                      className="ml-2 bg-transparent border border-slate-200 dark:border-slate-700 rounded px-1 py-1 text-slate-700 dark:text-slate-300"
                    >
                      <option value={50}>50</option>
                      <option value={100}>100</option>
                      <option value={250}>250</option>
                      <option value={500}>500</option>
                    </select>
                  </div>
                </div>
              </section>
            )}

            {/* SCHEMA PANEL */}
            {activeTab === 'schema' && schemaError && selectedTable && (
              <div className="max-w-md rounded-lg border border-red-200 bg-red-50 p-6 dark:border-red-900/50 dark:bg-red-950/20">
                <h2 className="text-lg font-semibold text-red-700 dark:text-red-400">Failed to load schema</h2>
                <p className="mt-2 text-sm text-red-600 dark:text-red-300/80">{schemaError}</p>
              </div>
            )}
            {activeTab === 'schema' && schema && selectedTable && (
              <section className="space-y-6">
                <div>
                  <h3 className="font-semibold text-slate-900 dark:text-white mb-2">Columns</h3>
                  <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900">
                    <table className="w-full text-sm font-mono">
                      <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                        <tr>
                          <th className="px-3 py-2 font-medium">name</th>
                          <th className="px-3 py-2 font-medium">type</th>
                          <th className="px-3 py-2 font-medium">null</th>
                          <th className="px-3 py-2 font-medium">default</th>
                          <th className="px-3 py-2 font-medium">pk</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
                        {schema.columns.map((col) => {
                          const isUnique = schema.indexes.some(
                            (idx) => idx.unique && idx.columns.length === 1 && idx.columns[0] === col.name
                          );
                          return (
                            <tr key={col.name}>
                              <td className="px-3 py-1.5">
                                <span className="inline-flex items-center gap-1.5">
                                  {col.name}
                                  {col.pk > 0 && (
                                    <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
                                      PK
                                    </span>
                                  )}
                                  {isUnique && (
                                    <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400">
                                      UNIQUE
                                    </span>
                                  )}
                                  {col.generated && (
                                    <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400">
                                      GENERATED {col.generated}
                                    </span>
                                  )}
                                </span>
                              </td>
                              <td className="px-3 py-1.5 text-sky-500">{col.type}</td>
                              <td className="px-3 py-1.5">{col.notnull ? 'NO' : 'YES'}</td>
                              <td className="px-3 py-1.5 text-slate-400">
                                {col.defaultVal !== null ? (
                                  <span className="text-emerald-600 dark:text-emerald-400">{col.defaultVal}</span>
                                ) : (
                                  '—'
                                )}
                              </td>
                              <td className="px-3 py-1.5">{col.pk > 0 ? `🔑 (${col.pk})` : ''}</td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                </div>

                <div className="grid md:grid-cols-2 gap-6">
                  <div>
                    <h3 className="font-semibold text-slate-900 dark:text-white mb-2">Indexes</h3>
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 space-y-2">
                      {schema.indexes.length === 0 ? (
                        <span className="text-sm text-slate-400 italic">none</span>
                      ) : (
                        schema.indexes.map((idx) => (
                          <div key={idx.name} className="text-sm font-mono text-slate-700 dark:text-slate-300">
                            <span className="font-semibold text-indigo-500">{idx.name}</span>{' '}
                            <span className="text-slate-400">
                              ({idx.columns.join(', ')})
                            </span>{' '}
                            {idx.unique && (
                              <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400">
                                UNIQUE
                              </span>
                            )}{' '}
                            {idx.partial && (
                              <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400">
                                PARTIAL
                              </span>
                            )}
                          </div>
                        ))
                      )}
                    </div>
                  </div>

                  <div>
                    <h3 className="font-semibold text-slate-900 dark:text-white mb-2">Foreign keys</h3>
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 space-y-2">
                      {schema.foreignKeys.length === 0 ? (
                        <span className="text-sm text-slate-400 italic">none</span>
                      ) : (
                        schema.foreignKeys.map((fk, idx) => (
                          <div key={idx} className="text-sm font-mono text-slate-700 dark:text-slate-300">
                            <span className="text-indigo-500">{fk.from}</span> →{' '}
                            <span className="font-semibold">{fk.table}</span>({fk.to})
                            {fk.onDelete && <span className="text-red-500 text-xs ml-2">ON DELETE {fk.onDelete}</span>}
                          </div>
                        ))
                      )}
                    </div>
                  </div>
                </div>

                {schema.triggers.length > 0 && (
                  <div>
                    <h3 className="font-semibold text-slate-900 dark:text-white mb-2">Triggers</h3>
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 space-y-3">
                      {schema.triggers.map((t) => (
                        <div key={t.name} className="font-mono text-sm space-y-1">
                          <div className="font-semibold text-indigo-500">{t.name}</div>
                          <pre className="bg-slate-50 dark:bg-slate-950 p-2 rounded border border-slate-100 dark:border-slate-800 overflow-x-auto text-xs">
                            {t.sql}
                          </pre>
                        </div>
                      ))}
                    </div>
                  </div>
                )}

                <div>
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="font-semibold text-slate-900 dark:text-white">CREATE statement</h3>
                    <button
                      onClick={() => copyToClipboard(schema.ddl)}
                      className="text-xs px-2.5 py-1 rounded border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      Copy
                    </button>
                  </div>
                  <pre className="font-mono text-xs bg-slate-900 text-slate-100 rounded-lg p-4 overflow-auto border border-slate-800">
                    {schema.ddl}
                  </pre>
                </div>
              </section>
            )}

            {/* SQL PANEL */}
            {activeTab === 'sql' && (
              <section className="space-y-4 flex-1 flex flex-col min-h-0">
                <div className="flex gap-4 flex-1 min-h-0">
                  {/* Left: Editor and Results */}
                  <div className="flex-1 flex flex-col min-h-0 gap-4">
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 shrink-0">
                      <div className="flex items-center justify-between px-3 py-2 bg-slate-100 dark:bg-slate-850 text-sm border-b border-slate-200 dark:border-slate-800">
                        <span className="font-medium text-slate-700 dark:text-slate-300">query.sql</span>
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
                      <SqlEditor
                        value={sqlValue}
                        onChange={setSqlValue}
                        onRun={handleExecuteQuery}
                        theme={theme}
                        editorViewRef={editorViewRef}
                      />
                    </div>

                    {/* Error Banner */}
                    {queryError && (
                      <div className="rounded-lg border border-red-300 dark:border-red-500/30 bg-red-50 dark:bg-red-500/10 text-red-700 dark:text-red-400 text-sm px-4 py-3 shrink-0 flex flex-col gap-1">
                        <div className="font-semibold">{queryError.code}</div>
                        <div className="font-mono text-xs whitespace-pre-wrap">{queryError.message}</div>
                      </div>
                    )}

                    {/* Results / Grid */}
                    {queryResult && (
                      <div className="flex-1 flex flex-col min-h-0 gap-2">
                        {/* Status Bar */}
                        <div className="flex items-center justify-between text-xs text-slate-500 shrink-0">
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
                          {queryResult.columns.length > 0 && (
                            <div className="flex items-center gap-2">
                              <span className="text-slate-400 dark:text-slate-500">Export:</span>
                              <button
                                disabled={exportQueryLoading}
                                onClick={() => handleQueryExport('csv')}
                                className="px-2 py-0.5 rounded border border-slate-205 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 text-[11px] font-medium transition-colors cursor-pointer flex items-center gap-1"
                              >
                                {exportQueryLoading ? (
                                  <span className="h-2.5 w-2.5 animate-spin rounded-full border border-slate-500 border-t-transparent"></span>
                                ) : null}
                                <span>CSV</span>
                              </button>
                              <button
                                disabled={exportQueryLoading}
                                onClick={() => handleQueryExport('json')}
                                className="px-2 py-0.5 rounded border border-slate-205 dark:border-slate-800 hover:bg-slate-50 dark:hover:bg-slate-800 disabled:opacity-50 text-[11px] font-medium transition-colors cursor-pointer flex items-center gap-1"
                              >
                                {exportQueryLoading ? (
                                  <span className="h-2.5 w-2.5 animate-spin rounded-full border border-slate-500 border-t-transparent"></span>
                                ) : null}
                                <span>JSON</span>
                              </button>
                            </div>
                          )}
                        </div>

                        {/* Results Grid */}
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
                      </div>
                    )}
                  </div>

                  {/* Right: History Panel */}
                  <div className="w-64 border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 flex flex-col min-h-0 shrink-0">
                    <h3 className="text-xs font-semibold text-slate-400 dark:text-slate-500 uppercase tracking-wider mb-2 shrink-0">
                      Query History
                    </h3>
                    <div className="flex-1 overflow-y-auto space-y-2 pr-1">
                      {queryHistory.length === 0 ? (
                        <div className="text-xs text-slate-400 italic">No queries run this session.</div>
                      ) : (
                        queryHistory.map((item, idx) => (
                          <div
                            key={idx}
                            onClick={() => setSqlValue(item.sql)}
                            className="p-2 rounded border border-slate-100 dark:border-slate-800 hover:border-indigo-500 dark:hover:border-indigo-500/50 hover:bg-slate-50 dark:hover:bg-slate-800/30 cursor-pointer flex flex-col gap-1 transition-all group"
                          >
                            <div className="text-xs font-mono line-clamp-2 text-slate-700 dark:text-slate-300 group-hover:text-indigo-650 dark:group-hover:text-indigo-400 break-all">
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
                  </div>
                </div>
              </section>
            )}

            {/* TABLE EDITOR PANEL */}
            {activeTab === 'editor' && (
              <section className="space-y-4">
                {!isWrite && (
                  <div className="rounded-lg border border-amber-300 dark:border-amber-500/30 bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 text-sm px-4 py-2.5">
                    Read-only mode — the table editor is disabled. Relaunch with <span className="font-mono">--write</span> to enable.
                  </div>
                )}
                <h3 className="font-semibold text-slate-900 dark:text-white">Create table</h3>
                <div className="opacity-60 pointer-events-none space-y-3">
                  <input
                    value="new_table"
                    disabled
                    className="font-mono text-sm px-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800"
                  />
                  <button className="px-3 py-1.5 rounded-md bg-indigo-600 text-white text-sm">
                    + Add column
                  </button>
                </div>
              </section>
            )}

            {/* SEED PANEL */}
            {activeTab === 'seed' && selectedTable && (
              <section className="space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">
                  Seed {selectedTable.name} with fake data
                </h3>
                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 max-w-xl">
                  <table className="w-full text-sm">
                    <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                      <tr>
                        <th className="px-3 py-2 font-medium">Column</th>
                        <th className="px-3 py-2 font-medium">Generator</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono text-xs">
                      {schema?.columns.map((col) => (
                        <tr key={col.name}>
                          <td className="px-3 py-2">{col.name}</td>
                          <td className="px-3 py-2">
                            <span className="px-2 py-0.5 rounded bg-indigo-50 dark:bg-indigo-500/20 text-indigo-600 dark:text-indigo-300">
                              default
                            </span>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            )}

            {/* EXPORT PANEL */}
            {activeTab === 'export' && selectedTable && (
              <section className="space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">
                  Export <span className="font-mono text-indigo-650 dark:text-indigo-400">{selectedTable.name}</span>
                </h3>
                
                <div className="grid sm:grid-cols-3 gap-3 max-w-2xl">
                  {/* CSV Card */}
                  <button
                    onClick={() => {
                      setSelectedExportFormat('csv');
                      let url = `/api/tables/${encodeURIComponent(selectedTable.name)}/export?format=csv`;
                      if (applyFilterSort && (orderBy || Object.values(filters).some(v => v !== ''))) {
                        url += `&filtered=true`;
                        if (orderBy) url += `&orderBy=${encodeURIComponent(orderBy)}`;
                        if (dir) url += `&dir=${encodeURIComponent(dir)}`;
                        Object.entries(filters).forEach(([col, val]) => {
                          if (val !== '') url += `&filter[${encodeURIComponent(col)}]=${encodeURIComponent(val)}`;
                        });
                      }
                      window.location.href = url;
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'csv'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <div className="text-2xl mb-1">📄</div>
                    <div className="font-medium text-slate-900 dark:text-white">CSV</div>
                    <div className="text-xs text-slate-400 mb-1">Comma-separated</div>
                    <div className="text-[10px] text-slate-500 italic mt-2 border-t border-slate-100 dark:border-slate-800/80 pt-1.5">
                      NULL values export as empty fields.
                    </div>
                  </button>

                  {/* JSON Card */}
                  <button
                    onClick={() => {
                      setSelectedExportFormat('json');
                      let url = `/api/tables/${encodeURIComponent(selectedTable.name)}/export?format=json`;
                      if (applyFilterSort && (orderBy || Object.values(filters).some(v => v !== ''))) {
                        url += `&filtered=true`;
                        if (orderBy) url += `&orderBy=${encodeURIComponent(orderBy)}`;
                        if (dir) url += `&dir=${encodeURIComponent(dir)}`;
                        Object.entries(filters).forEach(([col, val]) => {
                          if (val !== '') url += `&filter[${encodeURIComponent(col)}]=${encodeURIComponent(val)}`;
                        });
                      }
                      window.location.href = url;
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'json'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <div className="text-2xl mb-1">{`{ }`}</div>
                    <div className="font-medium text-slate-900 dark:text-white">JSON</div>
                    <div className="text-xs text-slate-400">Array of objects</div>
                  </button>

                  {/* SQL Card */}
                  <button
                    onClick={() => {
                      setSelectedExportFormat('sql');
                      let url = `/api/tables/${encodeURIComponent(selectedTable.name)}/export?format=sql`;
                      if (applyFilterSort && (orderBy || Object.values(filters).some(v => v !== ''))) {
                        url += `&filtered=true`;
                        if (orderBy) url += `&orderBy=${encodeURIComponent(orderBy)}`;
                        if (dir) url += `&dir=${encodeURIComponent(dir)}`;
                        Object.entries(filters).forEach(([col, val]) => {
                          if (val !== '') url += `&filter[${encodeURIComponent(col)}]=${encodeURIComponent(val)}`;
                        });
                      }
                      if (includeSchema) {
                        url += `&includeSchema=true`;
                      }
                      window.location.href = url;
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'sql'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <div className="text-2xl mb-1">🗄️</div>
                    <div className="font-medium text-slate-900 dark:text-white">SQL</div>
                    <div className="text-xs text-slate-400">INSERT statements</div>
                  </button>
                </div>

                {/* Toggles */}
                <div className="flex flex-col sm:flex-row sm:items-center gap-4 text-sm mt-4 text-slate-500 dark:text-slate-400 border-t border-slate-100 dark:border-slate-800/80 pt-4">
                  {/* Apply Filter/Sort Toggle */}
                  {(orderBy || Object.values(filters).some(v => v !== '')) ? (
                    <label className="flex items-center gap-2 cursor-pointer select-none">
                      <input
                        type="checkbox"
                        checked={applyFilterSort}
                        onChange={(e) => setApplyFilterSort(e.target.checked)}
                        className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
                      />
                      <span>Apply current filter/sort</span>
                    </label>
                  ) : null}

                  {/* Include Schema DDL Toggle */}
                  <label className={`flex items-center gap-2 select-none ${selectedExportFormat === 'sql' ? 'cursor-pointer opacity-100' : 'opacity-40 cursor-not-allowed'}`}>
                    <input
                      type="checkbox"
                      disabled={selectedExportFormat !== 'sql'}
                      checked={selectedExportFormat === 'sql' && includeSchema}
                      onChange={(e) => setIncludeSchema(e.target.checked)}
                      className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
                    />
                    <span>Include CREATE TABLE statement</span>
                  </label>
                </div>
              </section>
            )}

            {/* REST PANEL */}
            {activeTab === 'rest' && (
              <section className="space-y-4">
                <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/40 text-sm px-4 py-2.5">
                  REST serving is <span className="text-slate-400">off</span>. Relaunch with{' '}
                  <span className="font-mono">--rest</span> to expose tables.
                </div>
              </section>
            )}

            {/* CODEGEN PANEL */}
            {activeTab === 'codegen' && (
              <section className="space-y-4">
                <pre className="font-mono text-sm bg-slate-900 text-slate-100 rounded-lg p-4 overflow-x-auto border border-slate-800">
                  {`package main\n\ntype User struct {\n\tID    int64  \`json:"id"\`\n\tEmail string \`json:"email"\`\n}`}
                </pre>
              </section>
            )}

            {/* INFO PANEL */}
            {activeTab === 'info' && (
              <section className="space-y-6 max-w-4xl pb-10">
                {infoError && (
                  <div className="rounded-lg border border-red-300 dark:border-red-950/40 bg-red-50 dark:bg-red-950/15 text-red-700 dark:text-red-400 text-sm px-4 py-3 flex items-center justify-between shrink-0">
                    <span className="font-medium">Error: {infoError}</span>
                    <button
                      onClick={fetchMetaAndTables}
                      className="px-2 py-1 text-xs font-semibold rounded bg-red-100 dark:bg-red-900/30 hover:bg-red-200"
                    >
                      Retry
                    </button>
                  </div>
                )}

                <div className="flex items-center justify-between">
                  <h2 className="text-xl font-bold tracking-tight text-slate-900 dark:text-white">Database Information</h2>
                  <button
                    onClick={fetchMetaAndTables}
                    disabled={infoLoading}
                    className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold bg-white dark:bg-slate-900 hover:bg-slate-50 dark:hover:bg-slate-800 border border-slate-200 dark:border-slate-800 text-slate-700 dark:text-slate-200 transition-all disabled:opacity-50"
                  >
                    <span>🔄</span>
                    {infoLoading ? 'Refreshing...' : 'Refresh'}
                  </button>
                </div>

                {infoLoading && !meta ? (
                  // Skeleton state
                  <div className="space-y-6 animate-pulse">
                    <div className="h-28 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl"></div>
                    <div className="grid md:grid-cols-2 gap-4">
                      <div className="h-44 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl"></div>
                      <div className="h-44 bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl"></div>
                    </div>
                  </div>
                ) : (
                  <>
                    {/* Database file section */}
                    <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
                      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 dark:text-slate-550">Database</h3>
                      <div className="grid sm:grid-cols-2 md:grid-cols-4 gap-4">
                        <div className="sm:col-span-2 space-y-1">
                          <span className="text-xs text-slate-400 dark:text-slate-500">File Path</span>
                          <div className="group relative">
                            <div className="font-mono text-xs text-slate-750 dark:text-slate-250 bg-slate-50 dark:bg-slate-950 p-2 rounded-lg border border-slate-100 dark:border-slate-850 truncate max-w-full" title={meta?.path}>
                              {meta?.path || ':memory:'}
                            </div>
                          </div>
                        </div>
                        <div className="space-y-1">
                          <span className="text-xs text-slate-400 dark:text-slate-500">On-Disk Size</span>
                          <div className="font-semibold text-slate-900 dark:text-white text-base">
                            {meta ? formatBytes(meta.sizeBytes) : '0 B'}
                          </div>
                        </div>
                        <div className="space-y-1">
                          <span className="text-xs text-slate-400 dark:text-slate-500">SQLite Version</span>
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-slate-900 dark:text-white text-base">{meta?.sqliteVersion || 'unknown'}</span>
                            <span className={`text-[10px] px-1.5 py-0.5 rounded font-bold uppercase ${
                              meta?.mode === 'rw' 
                                ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-400' 
                                : 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400'
                            }`}>
                              {meta?.mode === 'rw' ? 'RW' : 'RO'}
                            </span>
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Storage Section */}
                    <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-5 shadow-sm space-y-4">
                      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 dark:text-slate-550">Storage & Engine</h3>
                      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                        <div className="space-y-0.5">
                          <span className="text-xs text-slate-400 dark:text-slate-500">Page Size</span>
                          <div className="font-semibold text-slate-950 dark:text-white">{meta ? `${meta.pageSize.toLocaleString()} bytes` : '—'}</div>
                        </div>
                        <div className="space-y-0.5">
                          <span className="text-xs text-slate-400 dark:text-slate-500">Page Count</span>
                          <div className="font-semibold text-slate-950 dark:text-white">{meta?.pageCount.toLocaleString() ?? '—'}</div>
                        </div>
                        <div className="space-y-0.5">
                          <span className="text-xs text-slate-400 dark:text-slate-500">Encoding</span>
                          <div className="font-semibold text-slate-950 dark:text-white">{meta?.encoding || '—'}</div>
                        </div>
                        <div className="space-y-0.5">
                          <span className="text-xs text-slate-400 dark:text-slate-500">Journal Mode</span>
                          <div className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-semibold bg-indigo-50 dark:bg-indigo-500/10 text-indigo-650 dark:text-indigo-400 uppercase">
                            {meta?.journalMode || '—'}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Objects section */}
                    <div className="space-y-4">
                      <h3 className="text-xs font-bold uppercase tracking-wider text-slate-400 dark:text-slate-550">Objects</h3>
                      <div className="grid grid-cols-2 gap-4">
                        <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm flex items-center justify-between">
                          <div>
                            <span className="text-xs text-slate-400 dark:text-slate-500">Tables</span>
                            <div className="text-2xl font-bold text-slate-900 dark:text-white mt-1">{meta?.tableCount ?? 0}</div>
                          </div>
                          <span className="text-2xl opacity-60">▤</span>
                        </div>
                        <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl p-4 shadow-sm flex items-center justify-between">
                          <div>
                            <span className="text-xs text-slate-400 dark:text-slate-500">Views</span>
                            <div className="text-2xl font-bold text-slate-900 dark:text-white mt-1">{meta?.viewCount ?? 0}</div>
                          </div>
                          <span className="text-2xl opacity-60">◫</span>
                        </div>
                      </div>

                      {/* Tables and Row counts list */}
                      <div className="bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 rounded-xl shadow-sm overflow-hidden flex flex-col max-h-[300px]">
                        <div className="overflow-y-auto">
                          <table className="w-full text-sm font-mono relative border-collapse">
                            <thead className="bg-slate-50 dark:bg-slate-800/40 text-slate-400 dark:text-slate-500 text-left sticky top-0 border-b border-slate-105 dark:border-slate-850 z-10">
                              <tr>
                                <th 
                                  onClick={() => {
                                    setInfoSortDir(prev => infoSortBy === 'name' ? (prev === 'asc' ? 'desc' : 'asc') : 'asc');
                                    setInfoSortBy('name');
                                  }}
                                  className="px-4 py-2 font-medium cursor-pointer select-none hover:text-indigo-500"
                                >
                                  Table Name {infoSortBy === 'name' ? (infoSortDir === 'asc' ? '▲' : '▼') : '↕'}
                                </th>
                                <th 
                                  onClick={() => {
                                    setInfoSortDir(prev => infoSortBy === 'rowCount' ? (prev === 'asc' ? 'desc' : 'asc') : 'desc');
                                    setInfoSortBy('rowCount');
                                  }}
                                  className="px-4 py-2 font-medium text-right cursor-pointer select-none hover:text-indigo-500"
                                >
                                  Row Count {infoSortBy === 'rowCount' ? (infoSortDir === 'asc' ? '▲' : '▼') : '↕'}
                                </th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-350">
                              {tables.filter(t => t.type === 'table').length === 0 ? (
                                <tr>
                                  <td colSpan={2} className="px-4 py-8 text-center text-slate-450 dark:text-slate-500 italic">No tables</td>
                                </tr>
                              ) : (
                                [...tables]
                                  .filter(t => t.type === 'table')
                                  .sort((a, b) => {
                                    const factor = infoSortDir === 'asc' ? 1 : -1;
                                    if (infoSortBy === 'name') {
                                      return a.name.localeCompare(b.name) * factor;
                                    } else {
                                      return (a.rowCount - b.rowCount) * factor;
                                    }
                                  })
                                  .map((t) => (
                                    <tr 
                                      key={t.name} 
                                      onClick={() => {
                                        setSelectedTable(t);
                                        setActiveTab('data');
                                      }}
                                      className="hover:bg-slate-50 dark:hover:bg-slate-800/40 cursor-pointer"
                                    >
                                      <td className="px-4 py-2 font-medium text-slate-900 dark:text-slate-205">{t.name}</td>
                                      <td className="px-4 py-2 text-right text-slate-500">{t.rowCount.toLocaleString()}</td>
                                    </tr>
                                  ))
                              )}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    </div>
                  </>
                )}
              </section>
            )}
          </div>
        </main>
      </div>

      {/* BLOB HEX VIEWER MODAL */}
      {blobModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setBlobModal(null)}
        >
          <div
            className="w-full max-w-2xl rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800">
              <h3 className="font-semibold text-slate-900 dark:text-white">
                BLOB — <span className="font-mono text-indigo-500">{blobModal.column}</span>
              </h3>
              <button
                onClick={() => setBlobModal(null)}
                className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400"
                title="Close"
              >
                ✕
              </button>
            </div>
            <div className="p-4">
              <pre className="font-mono text-xs bg-slate-900 text-slate-100 rounded-lg p-4 overflow-auto border border-slate-800 max-h-96 whitespace-pre">
                {formatHexDump(blobModal.hex)}
              </pre>
              <p className="mt-2 text-xs text-slate-400">
                {Math.ceil(blobModal.hex.length / 2).toLocaleString()} bytes
              </p>
            </div>
          </div>
        </div>
      )}

      {/* TOAST SYSTEM */}
      {toast && (
        <div className="fixed bottom-4 right-4 z-50 animate-bounce">
          <div className={`px-4 py-2.5 rounded-lg shadow-lg text-white font-medium text-sm flex items-center gap-2 ${
            toast.type === 'error' ? 'bg-red-650' : 'bg-emerald-600'
          }`}>
            <span>{toast.type === 'error' ? '⚠️' : '✅'}</span>
            <span>{toast.message}</span>
          </div>
        </div>
      )}
    </div>
  );
}

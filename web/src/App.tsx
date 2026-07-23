import { useEffect, useState } from 'react';

interface MetaData {
  name: string;
  mode: 'ro' | 'rw';
  sqliteVersion: string;
  sizeBytes: number;
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
          if (tablesBody.data.length > 0) {
            setSelectedTable(tablesBody.data[0]);
          }
        } else {
          throw new Error(tablesBody.error?.message || 'Failed to fetch database tables');
        }
      })
      .catch((err) => {
        console.error(err);
        setError(err.message);
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

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
              <section className="space-y-4">
                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden">
                  <div className="flex items-center justify-between px-3 py-2 bg-slate-100 dark:bg-slate-800/60 text-sm">
                    <span className="text-slate-500">query.sql</span>
                    <button className="px-3 py-1 rounded bg-indigo-600 text-white hover:bg-indigo-500 text-xs">
                      Run ▸
                    </button>
                  </div>
                  <pre className="font-mono text-sm p-4 bg-white dark:bg-slate-900 text-slate-700 dark:text-slate-300 overflow-x-auto">
                    <span className="text-purple-500">SELECT</span> id, email, created_at{'\n'}
                    <span className="text-purple-500">FROM</span> users{'\n'}
                    <span className="text-purple-500">LIMIT</span> <span className="text-amber-500">10</span>;
                  </pre>
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
                <h3 className="font-semibold text-slate-900 dark:text-white">Export {selectedTable.name}</h3>
                <div className="grid sm:grid-cols-3 gap-3 max-w-xl">
                  <button className="p-4 rounded-lg border border-slate-200 dark:border-slate-800 hover:border-indigo-400 text-left">
                    <div className="text-2xl mb-1">📄</div>
                    <div className="font-medium">CSV</div>
                    <div className="text-xs text-slate-400">Comma-separated</div>
                  </button>
                  <button className="p-4 rounded-lg border border-slate-200 dark:border-slate-800 hover:border-indigo-400 text-left">
                    <div className="text-2xl mb-1">{`{ }`}</div>
                    <div className="font-medium">JSON</div>
                    <div className="text-xs text-slate-400">Array of objects</div>
                  </button>
                  <button className="p-4 rounded-lg border border-slate-200 dark:border-slate-800 hover:border-indigo-400 text-left">
                    <div className="text-2xl mb-1">🗄️</div>
                    <div className="font-medium">SQL</div>
                    <div className="text-xs text-slate-400">INSERT statements</div>
                  </button>
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
              <section className="max-w-xl space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">Database info</h3>
                <dl className="text-sm font-mono border border-slate-200 dark:border-slate-800 rounded-lg divide-y divide-slate-100 dark:divide-slate-800 bg-white dark:bg-slate-900">
                  <div className="flex justify-between px-4 py-2">
                    <dt className="text-slate-400">Path</dt>
                    <dd>{meta?.name}</dd>
                  </div>
                  <div className="flex justify-between px-4 py-2">
                    <dt className="text-slate-400">Size on disk</dt>
                    <dd>{dbSize}</dd>
                  </div>
                  <div className="flex justify-between px-4 py-2">
                    <dt className="text-slate-400">SQLite version</dt>
                    <dd>{sqliteVer}</dd>
                  </div>
                  <div className="flex justify-between px-4 py-2">
                    <dt className="text-slate-400">Mode</dt>
                    <dd>{isWrite ? 'read-write' : 'read-only'}</dd>
                  </div>
                </dl>
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
    </div>
  );
}

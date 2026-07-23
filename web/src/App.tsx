import { useEffect, useState } from 'react';

interface MetaData {
  name: string;
  mode: 'ro' | 'rw';
  sqliteVersion: string;
  sizeBytes: number;
}

export default function App() {
  const [meta, setMeta] = useState<MetaData | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    // Check initial theme preference
    const saved = localStorage.getItem('color-scheme');
    if (saved === 'dark' || saved === 'light') return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });

  const [activeTab, setActiveTab] = useState<string>('data');
  const [searchQuery, setSearchQuery] = useState<string>('');

  useEffect(() => {
    // Sync theme class on document element
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
      localStorage.setItem('color-scheme', 'dark');
    } else {
      document.documentElement.classList.remove('dark');
      localStorage.setItem('color-scheme', 'light');
    }
  }, [theme]);

  useEffect(() => {
    fetch('/api/meta')
      .then((res) => {
        if (!res.ok) {
          throw new Error(`Server responded with ${res.status}`);
        }
        return res.json();
      })
      .then((body) => {
        if (body.ok && body.data) {
          setMeta(body.data);
        } else {
          throw new Error(body.error?.message || 'Failed to fetch database metadata');
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

  const mockTables = [
    { name: 'users', count: 1240 },
    { name: 'orders', count: 5981 },
    { name: 'products', count: 214 },
    { name: 'order_items', count: 18204 },
    { name: 'categories', count: 12 },
    { name: 'sessions', count: 892 },
  ];

  const filteredTables = mockTables.filter((t) =>
    t.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

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
            Tables
          </div>
          <nav className="flex-1 overflow-y-auto px-2 text-sm space-y-0.5">
            {filteredTables.map((t) => (
              <div
                key={t.name}
                className="flex items-center justify-between px-2 py-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer text-slate-700 dark:text-slate-300"
              >
                <span className="flex items-center gap-2">
                  <span className="text-slate-400 dark:text-slate-500">◫</span>
                  <span className="font-medium font-mono text-xs">{t.name}</span>
                </span>
                <span className="text-xs text-slate-400 font-mono">{t.count.toLocaleString()}</span>
              </div>
            ))}
          </nav>
        </aside>

        {/* ============ MAIN CONTENT ============ */}
        <main className="flex-1 flex flex-col min-w-0">
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
                className={`tab ${activeTab === tab.id ? 'tab-active' : ''}`}
              >
                {tab.label}
              </button>
            ))}
          </div>

          <div className="flex-1 overflow-auto p-4">
            {/* DATA PANEL */}
            {activeTab === 'data' && (
              <section className="space-y-4">
                <div className="flex items-center justify-between">
                  <h2 className="font-semibold text-slate-900 dark:text-white">
                    <span className="font-mono text-indigo-500">users</span>{' '}
                    <span className="text-xs text-slate-400 font-normal">1,240 rows</span>
                  </h2>
                  <div className="flex items-center gap-2 text-sm">
                    <button className="px-2.5 py-1 rounded-md border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-850">
                      Filter
                    </button>
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
                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900">
                  <table className="w-full text-sm font-mono">
                    <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 dark:text-slate-400 text-left">
                      <tr>
                        <th className="px-3 py-2 font-medium">id ▲</th>
                        <th className="px-3 py-2 font-medium">email</th>
                        <th className="px-3 py-2 font-medium">full_name</th>
                        <th className="px-3 py-2 font-medium">is_active</th>
                        <th className="px-3 py-2 font-medium">created_at</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
                      <tr>
                        <td className="px-3 py-1.5">1</td>
                        <td className="px-3 py-1.5">ada@example.com</td>
                        <td className="px-3 py-1.5">Ada Lovelace</td>
                        <td className="px-3 py-1.5">1</td>
                        <td className="px-3 py-1.5">2020-01-04 08:22:10</td>
                      </tr>
                      <tr>
                        <td className="px-3 py-1.5">2</td>
                        <td className="px-3 py-1.5">linus@example.com</td>
                        <td className="px-3 py-1.5">Linus Torvalds</td>
                        <td className="px-3 py-1.5">1</td>
                        <td className="px-3 py-1.5">2020-02-11 14:05:33</td>
                      </tr>
                      <tr>
                        <td className="px-3 py-1.5">3</td>
                        <td className="px-3 py-1.5">grace@example.com</td>
                        <td className="px-3 py-1.5">Grace Hopper</td>
                        <td className="px-3 py-1.5">0</td>
                        <td className="px-3 py-1.5">2020-03-19 19:44:01</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <div className="flex items-center justify-between text-sm text-slate-500">
                  <span>Rows 1–3 of 1,240</span>
                  <div className="flex items-center gap-1">
                    <button className="px-2 py-1 rounded border border-slate-200 dark:border-slate-700">←</button>
                    <span className="px-2">1 / 414</span>
                    <button className="px-2 py-1 rounded border border-slate-200 dark:border-slate-700">→</button>
                  </div>
                </div>
              </section>
            )}

            {/* SCHEMA PANEL */}
            {activeTab === 'schema' && (
              <section className="space-y-5">
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
                        <tr>
                          <td className="px-3 py-1.5">id</td>
                          <td className="px-3 py-1.5 text-sky-500">INTEGER</td>
                          <td className="px-3 py-1.5">NO</td>
                          <td className="px-3 py-1.5 text-slate-400">—</td>
                          <td className="px-3 py-1.5">🔑</td>
                        </tr>
                        <tr>
                          <td className="px-3 py-1.5">email</td>
                          <td className="px-3 py-1.5 text-sky-500">TEXT</td>
                          <td className="px-3 py-1.5">NO</td>
                          <td className="px-3 py-1.5 text-slate-400">—</td>
                          <td className="px-3 py-1.5"></td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
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
            {activeTab === 'seed' && (
              <section className="space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">Seed users with fake data</h3>
                <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 max-w-xl">
                  <table className="w-full text-sm">
                    <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                      <tr>
                        <th className="px-3 py-2 font-medium">Column</th>
                        <th className="px-3 py-2 font-medium">Generator</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono text-xs">
                      <tr>
                        <td className="px-3 py-2">email</td>
                        <td className="px-3 py-2">
                          <span className="px-2 py-0.5 rounded bg-indigo-105 dark:bg-indigo-500/20 text-indigo-600 dark:text-indigo-300">
                            email
                          </span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </section>
            )}

            {/* EXPORT PANEL */}
            {activeTab === 'export' && (
              <section className="space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">Export users</h3>
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
                <pre className="font-mono text-sm bg-slate-900 text-slate-100 rounded-lg p-4 overflow-x-auto">
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
    </div>
  );
}

import { useEffect, useState } from 'react';
import { Play, Square, RefreshCw, Copy, Check } from 'lucide-react';
import { apiFetch } from '../lib/api';

interface ColumnInfo {
  name: string;
  type: string;
  notnull: boolean;
  defaultVal: string | null;
  pk: number;
  hidden: number;
  generated: 'virtual' | 'stored' | null;
}

interface RestStatus {
  enabled: boolean;
  write: boolean;
  running: boolean;
  bindAddr: string;
  port: number;
  dbLabel?: string;
  startedAt?: string;
  lastStopReason?: string;
}

interface RestTableStatus {
  name: string;
  type: 'table' | 'view';
  hasPkRoute: boolean;
  writeAllowed: boolean;
  exposed: boolean;
  create: boolean;
  update: boolean;
  delete: boolean;
  restartNeeded?: boolean;
}

interface SelectedTableRef {
  name: string;
  type: 'table' | 'view';
}

interface RestTabProps {
  selectedTable: SelectedTableRef | null;
  onToast: (message: string, type: 'error' | 'success') => void;
}

async function jsonFetch(path: string, init?: RequestInit) {
  const res = await fetch(path, init);
  const body = await res.json();
  if (!body.ok) throw new Error(body.error?.message || 'Request failed');
  return body.data;
}

export default function RestTab({ selectedTable, onToast }: RestTabProps) {
  const [status, setStatus] = useState<RestStatus | null>(null);
  const [tables, setTables] = useState<RestTableStatus[]>([]);
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [columns, setColumns] = useState<ColumnInfo[] | null>(null);

  useEffect(() => {
    if (!selectedTable) {
      setColumns(null);
      return;
    }
    let cancelled = false;
    apiFetch(`/tables/${encodeURIComponent(selectedTable.name)}/schema`)
      .then((res) => res.json())
      .then((body) => {
        if (!cancelled && body.ok) setColumns(body.data.columns);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [selectedTable?.name]);

  const refresh = () => {
    jsonFetch('/api/rest/status')
      .then(setStatus)
      .catch(() => {});
    jsonFetch('/api/rest/tables')
      .then(setTables)
      .catch(() => {});
  };

  useEffect(() => {
    refresh();
    const interval = setInterval(refresh, 4000);
    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleStart = async () => {
    setBusy(true);
    try {
      await jsonFetch('/api/rest/start', { method: 'POST' });
      onToast('REST server started', 'success');
      refresh();
    } catch (err: any) {
      onToast(err.message || 'Failed to start REST server', 'error');
    } finally {
      setBusy(false);
    }
  };

  const handleStop = async () => {
    setBusy(true);
    try {
      await jsonFetch('/api/rest/stop', { method: 'POST' });
      onToast('REST server stopped', 'success');
      refresh();
    } catch (err: any) {
      onToast(err.message || 'Failed to stop REST server', 'error');
    } finally {
      setBusy(false);
    }
  };

  const updateTableConfig = async (name: string, patch: Partial<Pick<RestTableStatus, 'exposed' | 'create' | 'update' | 'delete'>>) => {
    try {
      await jsonFetch(`/api/rest/tables/${encodeURIComponent(name)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(patch),
      });
      if (status?.running) {
        onToast('REST config changed — restart the REST server to apply these changes', 'error');
      }
      refresh();
    } catch (err: any) {
      onToast(err.message || 'Failed to update REST config', 'error');
    }
  };

  // sampleValueForColumn picks a placeholder value that at least matches
  // the column's declared type affinity, so generated curl bodies aren't
  // just visually plausible JSON but actually insertable.
  const sampleValueForColumn = (col: ColumnInfo): unknown => {
    const t = col.type.toUpperCase();
    if (t.includes('INT')) return 1;
    if (t.includes('REAL') || t.includes('FLOA') || t.includes('DOUB') || t.includes('NUM') || t.includes('DEC')) return 1.0;
    if (t.includes('BLOB')) return '';
    return 'text';
  };

  // buildSampleBody includes only columns that are actually required for an
  // insert: NOT NULL, no default, and not hidden/generated. Columns with a
  // default (including an autoincrement rowid PK, which reports notnull=0)
  // are deliberately omitted since the server fills them in.
  const buildSampleBody = (cols: ColumnInfo[] | null): Record<string, unknown> => {
    const body: Record<string, unknown> = {};
    if (!cols) return body;
    for (const col of cols) {
      if (col.notnull && !col.defaultVal && !col.hidden && !col.generated) {
        body[col.name] = sampleValueForColumn(col);
      }
    }
    return body;
  };

  const buildCurlCommands = (table: RestTableStatus, s: RestStatus): { key: string; label: string; command: string }[] => {
    const base = `http://${s.bindAddr}:${s.port}/rest/${table.name}`;
    const sampleBody = JSON.stringify(buildSampleBody(columns));
    const cmds: { key: string; label: string; command: string }[] = [
      { key: 'list', label: 'GET /rest/' + table.name, command: `curl "${base}?limit=10"` },
    ];
    if (table.hasPkRoute) {
      cmds.push({ key: 'get', label: 'GET /rest/' + table.name + '/:pk', command: `curl "${base}/:pk"` });
    }
    if (table.create) {
      cmds.push({
        key: 'create',
        label: 'POST /rest/' + table.name,
        command: `curl "${base}" -X POST -H "Content-Type: application/json" -d '${sampleBody}'`,
      });
    }
    if (table.update && table.hasPkRoute) {
      cmds.push({
        key: 'update',
        label: 'PATCH /rest/' + table.name + '/:pk',
        command: `curl "${base}/:pk" -X PATCH -H "Content-Type: application/json" -d '${sampleBody}'`,
      });
    }
    if (table.delete && table.hasPkRoute) {
      cmds.push({
        key: 'delete',
        label: 'DELETE /rest/' + table.name + '/:pk',
        command: `curl "${base}/:pk" -X DELETE`,
      });
    }
    return cmds;
  };

  const copyCurl = (key: string, command: string) => {
    navigator.clipboard?.writeText(command).then(() => {
      setCopied(key);
      setTimeout(() => setCopied(null), 1500);
    });
  };

  if (!status) {
    return <div className="text-sm text-slate-500">Loading…</div>;
  }

  if (!selectedTable) {
    return (
      <section className="space-y-4">
        {!status.enabled && (
          <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/40 text-sm px-4 py-2.5">
            REST serving is <span className="text-slate-400">off</span>. Relaunch with{' '}
            <span className="font-mono">--rest</span> to expose tables.
          </div>
        )}
        <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 text-sm text-slate-500">
          Select a table or view from the sidebar to configure its REST endpoints.
        </div>
      </section>
    );
  }

  const t = tables.find((tbl) => tbl.name === selectedTable.name);

  return (
    <section className="space-y-4">
      {!status.enabled && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/40 text-sm px-4 py-2.5">
          REST serving is <span className="text-slate-400">off</span>. Relaunch with{' '}
          <span className="font-mono">--rest</span> to expose tables.
        </div>
      )}

      <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <div className="flex items-center gap-2 text-sm">
            <span
              className={`inline-block w-2 h-2 rounded-full ${status.running ? 'bg-emerald-500' : 'bg-slate-400 dark:bg-slate-600'}`}
            />
            <span className="font-medium">{status.running ? 'Running' : 'Stopped'}</span>
            {status.running && (
              <span className="font-mono text-slate-500 dark:text-slate-400">
                {status.bindAddr}:{status.port}
                {status.dbLabel ? ` · ${status.dbLabel}` : ''}
              </span>
            )}
            {!status.running && status.lastStopReason && status.lastStopReason !== 'manual' && (
              <span className="text-amber-600 dark:text-amber-400 text-xs">
                (last stopped: {status.lastStopReason})
              </span>
            )}
          </div>

          <div className="flex items-center gap-2">
            {t?.restartNeeded && status.running && (
              <span className="text-xs px-2 py-1 rounded bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300">
                restart needed to apply changes
              </span>
            )}
            <button
              onClick={handleStart}
              disabled={busy || !status.enabled || status.running}
              title={!status.enabled ? '--rest was not passed at launch' : undefined}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-emerald-600 text-white hover:bg-emerald-700 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Play className="w-3.5 h-3.5" /> Start REST server
            </button>
            <button
              onClick={handleStop}
              disabled={busy || !status.running}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-md text-sm font-medium bg-slate-200 dark:bg-slate-700 text-slate-700 dark:text-slate-200 hover:bg-slate-300 dark:hover:bg-slate-600 disabled:opacity-40 disabled:cursor-not-allowed"
            >
              <Square className="w-3.5 h-3.5" /> Stop REST server
            </button>
            <button
              onClick={refresh}
              className="p-1.5 rounded-md text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
              title="Refresh"
            >
              <RefreshCw className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
        {!status.write && (
          <p className="text-xs text-slate-500">
            Started without <span className="font-mono">--write</span> — create/update/delete toggles are disabled below.
          </p>
        )}
        <p className="text-xs text-slate-400 dark:text-slate-500">
          Changing a table's REST configuration — including enabling REST on a different table — requires restarting the REST server to take effect.
        </p>
      </div>

      <div className="rounded-lg border border-slate-200 dark:border-slate-800">
        {!t ? (
          <div className="p-4 text-sm text-slate-500">Loading table info…</div>
        ) : (
          <div className="p-4 space-y-3">
            <div className="flex items-center justify-between flex-wrap gap-2">
              <div className="flex items-center gap-2">
                <span className="font-mono text-sm text-indigo-600 dark:text-indigo-400">{t.name}</span>
                <span className="text-xs text-slate-400">{t.type}</span>
                {t.restartNeeded && (
                  <span className="text-xs px-1.5 py-0.5 rounded bg-amber-100 text-amber-700 dark:bg-amber-500/20 dark:text-amber-300">
                    restart needed
                  </span>
                )}
              </div>
              <label className="flex items-center gap-1.5 text-xs">
                <input
                  type="checkbox"
                  checked={t.exposed}
                  onChange={(e) => updateTableConfig(t.name, { exposed: e.target.checked })}
                  className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
                />
                Expose via REST
              </label>
            </div>

            {t.exposed && (
              <>
                <div className="code text-sm space-y-1.5">
                  <div className="flex items-center gap-2">
                    <span className="px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300 w-16 text-center text-xs">GET</span>
                    /rest/{t.name}
                  </div>
                  {t.hasPkRoute && (
                    <div className="flex items-center gap-2">
                      <span className="px-2 py-0.5 rounded bg-emerald-100 text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-300 w-16 text-center text-xs">GET</span>
                      /rest/{t.name}/:pk
                    </div>
                  )}
                  {(['create', 'update', 'delete'] as const).map((op) => {
                    const method = op === 'create' ? 'POST' : op === 'update' ? 'PATCH' : 'DELETE';
                    const path = op === 'create' ? `/rest/${t.name}` : `/rest/${t.name}/:pk`;
                    const disabled = !t.writeAllowed || (op !== 'create' && !t.hasPkRoute);
                    return (
                      <div key={op} className={`flex items-center gap-2 ${!t[op] ? 'opacity-50' : ''}`}>
                        <span className="px-2 py-0.5 rounded bg-sky-100 text-sky-700 dark:bg-sky-500/20 dark:text-sky-300 w-16 text-center text-xs">{method}</span>
                        <span>{path}</span>
                        <label
                          className="flex items-center gap-1 text-xs text-slate-500 ml-2"
                          title={disabled ? (!t.writeAllowed ? 'start squad with --write to enable' : 'no usable primary key for this table') : undefined}
                        >
                          <input
                            type="checkbox"
                            checked={t[op]}
                            disabled={disabled}
                            onChange={(e) => updateTableConfig(t.name, { [op]: e.target.checked })}
                            className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500 disabled:opacity-40"
                          />
                          {op}
                        </label>
                      </div>
                    );
                  })}
                </div>
                <div className="space-y-2">
                  {buildCurlCommands(t, status).map(({ key, label, command }) => (
                    <div key={key} className="flex items-center gap-2">
                      <pre className="code text-xs bg-slate-900 text-slate-100 rounded-lg p-3 flex-1 overflow-auto">
                        $ {command}
                      </pre>
                      <button
                        onClick={() => copyCurl(key, command)}
                        className="p-2 rounded-md text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800"
                        title={`Copy curl command for ${label}`}
                      >
                        {copied === key ? <Check className="w-3.5 h-3.5 text-emerald-600" /> : <Copy className="w-3.5 h-3.5" />}
                      </button>
                    </div>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </section>
  );
}

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Webhook, Plus, Trash2, Pencil, Play, Save, RefreshCw, Power, PowerOff,
  ScrollText, CircleCheck, CircleX, Filter, Lock, Unlock, Globe, GlobeLock, Zap, Timer, X, Shuffle, Eraser,
} from 'lucide-react';
import { basicSetup } from 'codemirror';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { StreamLanguage } from '@codemirror/language';
import { lua } from '@codemirror/legacy-modes/mode/lua';
import { apiFetch } from '../lib/api';
import ConfirmModal from './ConfirmModal';

// jsonFetch unwraps squad's { ok, data | error } envelope, mirroring
// ModulesTab's helper of the same name.
async function jsonFetch<T = any>(path: string, init?: RequestInit): Promise<T> {
  const res = await apiFetch(path, {
    ...init,
    headers: { 'Content-Type': 'application/json', ...(init?.headers || {}) },
  });
  const body = await res.json();
  if (!body.ok) throw new Error(body.error?.message || 'Request failed');
  return body.data as T;
}

export interface HookSummary {
  id: number;
  table: string;
  event: string;
  timing: string;
  scope: string;
  name: string;
  description: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
}

interface HookDetail extends HookSummary {
  source: string;
}

interface HookRun {
  id: number;
  hookId: number;
  ranAt: string;
  event: string;
  success: boolean;
  error: string;
  durationMs: number;
  logs: string[];
}

interface TestResult {
  result: boolean | null;
  message: string | null;
  logs: string[];
  durationMs: number;
  error?: string;
}

interface HooksIndex {
  hooks: HookSummary[];
  hookMode: string;
  write: boolean;
  allowNet: boolean;
}

interface TableRowsPage {
  columns: string[];
  rows: any[][];
  total: number;
}

interface HooksTabProps {
  onToast: (message: string, type: 'error' | 'success') => void;
  /**
   * The table selected in the sidebar. The Hooks tab is scoped to it: only
   * that table's hooks are listed, and new hooks can only be created on it.
   * Undefined while no table is selected (e.g. an empty database).
   */
  tableFilter?: string;
  theme?: 'light' | 'dark';
  /**
   * Called after a hook is created, saved, deleted, or enabled/disabled —
   * any of which installs or drops the underlying __squad_hook_<id> SQL
   * trigger. Lets the Schema tab's Triggers section (which reads separate
   * state in the parent) refresh instead of going stale.
   */
  onHookChanged?: () => void;
}

const EVENTS = ['insert', 'update', 'delete'];
const TIMINGS = ['before', 'after'];
const SCOPES = ['row', 'statement'];

const STARTER_SOURCE = `-- 'new' and 'old' are tables of the row's columns (nil where
-- the event doesn't provide them). Available modules: json, db, http.
-- A 'before' hook can reject the write: return false, "why".
if new.email == nil or new.email == "" then
  return false, "email required"
end
return true
`;

const luaLanguage = StreamLanguage.define(lua);
const themeCompartment = new Compartment();

const lightTheme = EditorView.theme({
  '&': { color: '#1e293b', backgroundColor: '#ffffff', height: '100%' },
  '.cm-content': { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
  '.cm-gutters': { backgroundColor: '#f8fafc', color: '#94a3b8', borderRight: '1px solid #e2e8f0' },
}, { dark: false });

const darkTheme = EditorView.theme({
  '&': { color: '#f1f5f9', backgroundColor: '#0f172a', height: '100%' },
  '.cm-content': { fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace' },
  '.cm-gutters': { backgroundColor: '#0b0f19', color: '#475569', borderRight: '1px solid #1e293b' },
}, { dark: true });

/** LuaEditor is the CodeMirror 6 Lua-mode editor used for a hook's body. */
function LuaEditor({ value, onChange, theme, minHeight = '16rem' }: {
  value: string;
  onChange: (v: string) => void;
  theme: 'light' | 'dark';
  minHeight?: string;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  useEffect(() => { onChangeRef.current = onChange; }, [onChange]);

  useEffect(() => {
    if (!containerRef.current) return;
    const view = new EditorView({
      state: EditorState.create({
        doc: value,
        extensions: [
          basicSetup,
          luaLanguage,
          themeCompartment.of(theme === 'dark' ? darkTheme : lightTheme),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) onChangeRef.current(u.state.doc.toString());
          }),
        ],
      }),
      parent: containerRef.current,
    });
    viewRef.current = view;
    return () => { view.destroy(); viewRef.current = null; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    viewRef.current?.dispatch({
      effects: themeCompartment.reconfigure(theme === 'dark' ? darkTheme : lightTheme),
    });
  }, [theme]);

  // Reconcile external value changes (switching between hooks) without
  // clobbering what the user is typing.
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const current = view.state.doc.toString();
    if (current !== value) {
      view.dispatch({ changes: { from: 0, to: current.length, insert: value } });
    }
  }, [value]);

  return (
    <div
      ref={containerRef}
      style={{ minHeight }}
      className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden text-sm"
    />
  );
}

export default function HooksTab({
  onToast, tableFilter, theme = 'light', onHookChanged,
}: HooksTabProps) {
  const [index, setIndex] = useState<HooksIndex | null>(null);
  const [loading, setLoading] = useState(true);
  const [deleteTarget, setDeleteTarget] = useState<HookSummary | null>(null);

  // Edit/create modal — fields only, no test panel or execution log.
  const [creating, setCreating] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [draft, setDraft] = useState<Partial<HookDetail>>({});
  const [draftSource, setDraftSource] = useState('');
  const [saving, setSaving] = useState(false);

  // Test modal — dry-run against sample old/new JSON.
  const [testHook, setTestHook] = useState<HookSummary | null>(null);
  const [testOld, setTestOld] = useState('null');
  const [testNew, setTestNew] = useState('null');
  const [testResult, setTestResult] = useState<TestResult | null>(null);
  const [testRunning, setTestRunning] = useState(false);
  const [samplingRandom, setSamplingRandom] = useState(false);

  // Execution-log modal — view + clear.
  const [logsHook, setLogsHook] = useState<HookSummary | null>(null);
  const [runs, setRuns] = useState<HookRun[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [clearingLogs, setClearingLogs] = useState(false);

  const write = index?.write ?? false;
  const hookMode = index?.hookMode ?? 'sync';
  const allowNet = index?.allowNet ?? false;
  const asyncMode = hookMode === 'async';

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const qs = tableFilter ? `?table=${encodeURIComponent(tableFilter)}` : '';
      const data = await jsonFetch<HooksIndex>(`/hooks${qs}`);
      setIndex(data);
    } catch (err: any) {
      onToast(err.message || 'Failed to load hooks', 'error');
    } finally {
      setLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tableFilter]);

  useEffect(() => { load(); }, [load]);

  // Switching the sidebar's selected table should drop any modal open for
  // the previous table rather than leaving it open against a table that's
  // no longer in view.
  useEffect(() => {
    setCreating(false);
    setEditingId(null);
    setTestHook(null);
    setTestResult(null);
    setLogsHook(null);
    setRuns([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tableFilter]);

  const hooks = useMemo(() => index?.hooks ?? [], [index]);

  // ---- edit/create modal --------------------------------------------------

  const openEdit = async (h: HookSummary) => {
    try {
      const data = await jsonFetch<HookDetail>(`/hooks/${h.id}`);
      setCreating(false);
      setEditingId(h.id);
      setDraft(data);
      setDraftSource(data.source);
    } catch (err: any) {
      onToast(err.message || 'Failed to load hook', 'error');
    }
  };

  const startCreate = () => {
    setEditingId(null);
    setCreating(true);
    setDraft({
      table: tableFilter || '',
      event: 'insert',
      timing: asyncMode ? 'after' : 'before',
      scope: 'row',
      name: '',
      description: '',
      enabled: true,
    });
    setDraftSource(STARTER_SOURCE);
  };

  const closeEditor = () => {
    setCreating(false);
    setEditingId(null);
  };

  const save = async () => {
    setSaving(true);
    try {
      const body = {
        table: draft.table, event: draft.event, timing: draft.timing, scope: draft.scope,
        name: draft.name || '', description: draft.description || '',
        source: draftSource, enabled: draft.enabled ?? true,
      };
      if (creating) {
        const created = await jsonFetch<HookSummary>('/hooks', {
          method: 'POST', body: JSON.stringify(body),
        });
        onToast(`Hook #${created.id} created`, 'success');
      } else if (editingId != null) {
        await jsonFetch<HookSummary>(`/hooks/${editingId}`, {
          method: 'PATCH', body: JSON.stringify(body),
        });
        onToast('Hook saved', 'success');
      }
      closeEditor();
      await load();
      onHookChanged?.();
    } catch (err: any) {
      onToast(err.message || 'Failed to save hook', 'error');
    } finally {
      setSaving(false);
    }
  };

  // ---- test modal ----------------------------------------------------------

  const fetchRandomRow = async (table: string): Promise<Record<string, any> | null> => {
    const first = await jsonFetch<TableRowsPage>(`/tables/${encodeURIComponent(table)}/rows?limit=1`);
    if (!first.total) return null;
    const offset = Math.floor(Math.random() * first.total);
    const page = offset === 0
      ? first
      : await jsonFetch<TableRowsPage>(`/tables/${encodeURIComponent(table)}/rows?limit=1&offset=${offset}`);
    const row = page.rows[0];
    if (!row) return null;
    const obj: Record<string, any> = {};
    page.columns.forEach((c, i) => { obj[c] = row[i]; });
    return obj;
  };

  const fillRandom = async () => {
    if (!tableFilter) return;
    setSamplingRandom(true);
    try {
      const row = await fetchRandomRow(tableFilter);
      if (!row) {
        onToast(`${tableFilter} has no rows to sample`, 'error');
        return;
      }
      const json = JSON.stringify(row, null, 2);
      setTestOld(json);
      setTestNew(json);
    } catch (err: any) {
      onToast(err.message || 'Failed to sample a row', 'error');
    } finally {
      setSamplingRandom(false);
    }
  };

  const openTest = (h: HookSummary) => {
    setTestHook(h);
    setTestResult(null);
    setTestOld('null');
    setTestNew('null');
    fillRandom();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  };

  const closeTest = () => {
    setTestHook(null);
    setTestResult(null);
  };

  const runTest = async () => {
    if (!testHook) return;
    let oldRow: unknown = null;
    let newRow: unknown = null;
    try {
      oldRow = testOld.trim() ? JSON.parse(testOld) : null;
      newRow = testNew.trim() ? JSON.parse(testNew) : null;
    } catch (err: any) {
      onToast(`Sample row JSON is invalid: ${err.message}`, 'error');
      return;
    }
    setTestRunning(true);
    try {
      const data = await jsonFetch<TestResult>(`/hooks/${testHook.id}/test`, {
        method: 'POST',
        body: JSON.stringify({ old: oldRow, new: newRow }),
      });
      setTestResult(data);
    } catch (err: any) {
      onToast(err.message || 'Test run failed', 'error');
    } finally {
      setTestRunning(false);
    }
  };

  // ---- execution-log modal --------------------------------------------------

  const loadRunsFor = useCallback(async (id: number) => {
    setLogsLoading(true);
    try {
      const data = await jsonFetch<{ runs: HookRun[] }>(`/hooks/${id}/log?limit=200`);
      setRuns(data.runs || []);
    } catch (err: any) {
      onToast(err.message || 'Failed to load execution log', 'error');
      setRuns([]);
    } finally {
      setLogsLoading(false);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const openLogs = (h: HookSummary) => {
    setLogsHook(h);
    loadRunsFor(h.id);
  };

  const closeLogs = () => {
    setLogsHook(null);
    setRuns([]);
  };

  const clearLogs = async () => {
    if (!logsHook) return;
    setClearingLogs(true);
    try {
      await jsonFetch(`/hooks/${logsHook.id}/log`, { method: 'DELETE' });
      onToast('Execution log cleared', 'success');
      setRuns([]);
    } catch (err: any) {
      onToast(err.message || 'Failed to clear execution log', 'error');
    } finally {
      setClearingLogs(false);
    }
  };

  // ---- enable/disable + delete ----------------------------------------------

  const toggleEnabled = async (h: HookSummary) => {
    try {
      await jsonFetch(`/hooks/${h.id}`, {
        method: 'PATCH', body: JSON.stringify({ enabled: !h.enabled }),
      });
      onToast(`Hook #${h.id} ${h.enabled ? 'disabled' : 'enabled'}`, 'success');
      load();
      onHookChanged?.();
    } catch (err: any) {
      onToast(err.message || 'Failed to toggle hook', 'error');
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    try {
      await jsonFetch(`/hooks/${deleteTarget.id}`, { method: 'DELETE' });
      onToast(`Hook #${deleteTarget.id} deleted`, 'success');
      setDeleteTarget(null);
      load();
      onHookChanged?.();
    } catch (err: any) {
      onToast(err.message || 'Failed to delete hook', 'error');
      setDeleteTarget(null);
    }
  };

  // ---- render ----------------------------------------------------------------

  const statusStrip = (
    <div className="flex flex-wrap items-center gap-2 text-xs">
      <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900">
        {asyncMode ? <Zap className="w-3.5 h-3.5 text-amber-500" /> : <Timer className="w-3.5 h-3.5 text-indigo-500" />}
        <span className="font-medium">{hookMode}</span>
        <span className="text-slate-500">
          {asyncMode ? 'fire-and-forget, after hooks only' : 'blocking, before hooks can abort a write'}
        </span>
      </span>
      <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900">
        {write ? <Unlock className="w-3.5 h-3.5 text-emerald-500" /> : <Lock className="w-3.5 h-3.5 text-slate-400" />}
        <span className="font-medium">{write ? 'write' : 'read-only'}</span>
        <span className="text-slate-500">{write ? 'db.exec allowed' : 'no cross-table writes'}</span>
      </span>
      <span className="inline-flex items-center gap-1.5 px-2 py-1 rounded border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900">
        {allowNet ? <Globe className="w-3.5 h-3.5 text-emerald-500" /> : <GlobeLock className="w-3.5 h-3.5 text-slate-400" />}
        <span className="font-medium">{allowNet ? 'network enabled' : 'network disabled'}</span>
      </span>
    </div>
  );

  const list = (
    <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 divide-y divide-slate-100 dark:divide-slate-800">
      {loading && <div className="p-4 text-sm text-slate-500">Loading hooks…</div>}
      {!loading && hooks.length === 0 && (
        <div className="p-4 text-sm text-slate-500">
          No hooks defined{tableFilter ? ` for ${tableFilter}` : ''}.
        </div>
      )}
      {hooks.map((h) => (
        <div key={h.id} className="flex items-center gap-3 px-3 py-2 text-sm">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-indigo-500">#{h.id}</span>
              <span className="font-medium truncate">{h.name || '(unnamed)'}</span>
              {!h.enabled && (
                <span className="text-[10px] uppercase tracking-wide px-1.5 py-0.5 rounded bg-slate-100 dark:bg-slate-800 text-slate-500">
                  disabled
                </span>
              )}
            </div>
            <div className="text-xs text-slate-500 font-mono truncate">
              {h.timing} {h.event} on {h.table} · {h.scope}
            </div>
          </div>
          <button
            onClick={() => openEdit(h)}
            title="Edit hook"
            className="p-1.5 rounded text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
          >
            <Pencil className="w-4 h-4" />
          </button>
          <button
            onClick={() => openTest(h)}
            title="Test with sample row"
            className="p-1.5 rounded text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
          >
            <Play className="w-4 h-4" />
          </button>
          <button
            onClick={() => openLogs(h)}
            title="View execution log"
            className="p-1.5 rounded text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 cursor-pointer"
          >
            <ScrollText className="w-4 h-4" />
          </button>
          <button
            onClick={() => toggleEnabled(h)}
            disabled={!write}
            title={write ? (h.enabled ? 'Disable hook' : 'Enable hook') : 'Write mode required'}
            className="p-1.5 rounded text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer"
          >
            {h.enabled ? <PowerOff className="w-4 h-4" /> : <Power className="w-4 h-4" />}
          </button>
          <button
            onClick={() => setDeleteTarget(h)}
            disabled={!write}
            title={write ? 'Delete hook' : 'Write mode required'}
            className="p-1.5 rounded text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer"
          >
            <Trash2 className="w-4 h-4" />
          </button>
        </div>
      ))}
    </div>
  );

  const editing = creating || editingId !== null;

  const editorModal = editing && (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={closeEditor}>
      <div
        className="w-full max-w-4xl max-h-[90vh] rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <Webhook className="w-4 h-4 text-indigo-500" />
            {creating ? 'New hook' : `Edit hook #${editingId}`}
          </h3>
          <button
            onClick={closeEditor}
            className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400 cursor-pointer"
            title="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Table</span>
              <input
                value={draft.table ?? ''}
                readOnly
                disabled
                title="Hooks are scoped to the table selected in the sidebar"
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-950 text-sm text-slate-500 disabled:opacity-100"
              />
            </label>
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Event</span>
              <select
                value={draft.event ?? 'insert'}
                onChange={(e) => setDraft({ ...draft, event: e.target.value })}
                disabled={!write}
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm disabled:opacity-60"
              >
                {EVENTS.map((e) => <option key={e} value={e}>{e}</option>)}
              </select>
            </label>
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Timing</span>
              <select
                value={draft.timing ?? 'after'}
                onChange={(e) => setDraft({ ...draft, timing: e.target.value })}
                disabled={!write}
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm disabled:opacity-60"
              >
                {TIMINGS.map((t) => (
                  <option
                    key={t}
                    value={t}
                    disabled={t === 'before' && asyncMode}
                    title={t === 'before' && asyncMode
                      ? 'before hooks need --hook-mode sync: they can only be meaningful when they can block the write'
                      : undefined}
                  >
                    {t}{t === 'before' && asyncMode ? ' (needs --hook-mode sync)' : ''}
                  </option>
                ))}
              </select>
            </label>
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Scope</span>
              <select
                value={draft.scope ?? 'row'}
                onChange={(e) => setDraft({ ...draft, scope: e.target.value })}
                disabled={!write}
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm disabled:opacity-60"
              >
                {SCOPES.map((s) => <option key={s} value={s}>{s}</option>)}
              </select>
            </label>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Name</span>
              <input
                value={draft.name ?? ''}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
                disabled={!write}
                placeholder="require-email"
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm disabled:opacity-60"
              />
            </label>
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Description</span>
              <input
                value={draft.description ?? ''}
                onChange={(e) => setDraft({ ...draft, description: e.target.value })}
                disabled={!write}
                className="w-full px-2 py-1.5 rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm disabled:opacity-60"
              />
            </label>
          </div>

          <div>
            <span className="text-xs text-slate-500">Lua source</span>
            <LuaEditor value={draftSource} onChange={setDraftSource} theme={theme} />
          </div>
        </div>
        <div className="flex items-center justify-end gap-2 px-4 py-3 border-t border-slate-200 dark:border-slate-800 shrink-0">
          <button
            onClick={closeEditor}
            className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
          >
            Cancel
          </button>
          <button
            onClick={save}
            disabled={!write || saving}
            title={write ? 'Save hook' : 'Write mode required to create or edit hooks'}
            className="inline-flex items-center gap-1.5 text-xs px-3 py-1.5 rounded-md bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
          >
            <Save className="w-3.5 h-3.5" /> {creating ? 'Create hook' : 'Save'}
          </button>
        </div>
      </div>
    </div>
  );

  const testModal = testHook && (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={closeTest}>
      <div
        className="w-full max-w-3xl max-h-[90vh] rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <Play className="w-4 h-4 text-indigo-500" />
            Test hook #{testHook.id}{testHook.name ? ` — ${testHook.name}` : ''}
          </h3>
          <button
            onClick={closeTest}
            className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400 cursor-pointer"
            title="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          <div className="flex items-center justify-between gap-3">
            <span className="text-xs text-slate-500">
              Dry run — no trigger fires and no real row is touched.
            </span>
            <button
              onClick={fillRandom}
              disabled={samplingRandom || !tableFilter}
              title={tableFilter ? 'Fill old/new with a random row' : 'No table selected'}
              className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shrink-0"
            >
              <Shuffle className="w-3.5 h-3.5" /> {samplingRandom ? 'Sampling…' : 'Fill with random record'}
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Sample <code>old</code> row (JSON)</span>
              <textarea
                value={testOld}
                onChange={(e) => setTestOld(e.target.value)}
                rows={8}
                className="w-full px-2 py-1.5 font-mono text-xs rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
              />
            </label>
            <label className="text-xs space-y-1">
              <span className="text-slate-500">Sample <code>new</code> row (JSON)</span>
              <textarea
                value={testNew}
                onChange={(e) => setTestNew(e.target.value)}
                rows={8}
                className="w-full px-2 py-1.5 font-mono text-xs rounded border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900"
              />
            </label>
          </div>

          <button
            onClick={runTest}
            disabled={testRunning}
            className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
          >
            <Play className="w-3.5 h-3.5" /> {testRunning ? 'Running…' : 'Run test'}
          </button>

          {testResult && (
            <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 p-3 text-sm space-y-1">
              <div className="flex items-center gap-2">
                {testResult.error || testResult.result === false
                  ? <CircleX className="w-4 h-4 text-red-500" />
                  : <CircleCheck className="w-4 h-4 text-emerald-500" />}
                <span className="font-medium">
                  result: {testResult.result === null ? 'nil' : String(testResult.result)}
                </span>
                <span className="text-slate-500 text-xs">{testResult.durationMs}ms</span>
              </div>
              {testResult.message && <div className="text-xs">message: {testResult.message}</div>}
              {testResult.error && <div className="text-xs text-red-500">error: {testResult.error}</div>}
              {testResult.logs?.length > 0 && (
                <pre className="text-xs bg-slate-50 dark:bg-slate-950 rounded p-2 overflow-x-auto">
                  {testResult.logs.join('\n')}
                </pre>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );

  const logsModal = logsHook && (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={closeLogs}>
      <div
        className="w-full max-w-4xl max-h-[90vh] rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <ScrollText className="w-4 h-4 text-indigo-500" />
            Execution log — #{logsHook.id}{logsHook.name ? ` ${logsHook.name}` : ''}
            <span className="text-xs font-normal text-slate-500">(last 200 runs)</span>
          </h3>
          <div className="flex items-center gap-1">
            <button
              onClick={() => loadRunsFor(logsHook.id)}
              title="Refresh"
              className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400 cursor-pointer"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
            <button
              onClick={clearLogs}
              disabled={!write || clearingLogs || runs.length === 0}
              title={write ? 'Clear execution log' : 'Write mode required'}
              className="w-7 h-7 rounded-md hover:bg-red-50 dark:hover:bg-red-500/10 flex items-center justify-center text-red-500 disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer"
            >
              <Eraser className="w-4 h-4" />
            </button>
            <button
              onClick={closeLogs}
              title="Close"
              className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400 cursor-pointer"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-4">
          {logsLoading && <div className="text-sm text-slate-500">Loading…</div>}
          {!logsLoading && (
            <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 divide-y divide-slate-100 dark:divide-slate-800">
              {runs.length === 0 && <div className="p-3 text-sm text-slate-500">No recorded runs yet.</div>}
              {runs.map((r) => (
                <div key={r.id} className="px-3 py-2 text-xs font-mono flex items-start gap-2">
                  {r.success
                    ? <CircleCheck className="w-3.5 h-3.5 text-emerald-500 shrink-0 mt-0.5" />
                    : <CircleX className="w-3.5 h-3.5 text-red-500 shrink-0 mt-0.5" />}
                  <span className="text-slate-500 shrink-0">{r.ranAt}</span>
                  <span className="shrink-0">{r.event}</span>
                  <span className="text-slate-500 shrink-0">{r.durationMs}ms</span>
                  <span className="min-w-0 break-all">
                    {r.error}
                    {r.logs?.length > 0 && <span className="text-slate-500"> · {r.logs.join(' | ')}</span>}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );

  const canCreate = write && !!tableFilter;
  const createDisabledReason = !tableFilter
    ? 'Select a table in the sidebar first'
    : !write
      ? 'Write mode required to create hooks'
      : 'New hook';

  const header = (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex items-center gap-2">
        <Webhook className="w-5 h-5 text-indigo-500" />
        <h2 className="font-semibold text-slate-900 dark:text-white">Hooks</h2>
        {tableFilter && (
          <span className="inline-flex items-center gap-1 text-xs text-slate-500">
            <Filter className="w-3 h-3" /> {tableFilter}
          </span>
        )}
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={startCreate}
          disabled={!canCreate}
          title={createDisabledReason}
          className="inline-flex items-center gap-1.5 text-xs px-2.5 py-1.5 rounded bg-indigo-500 text-white hover:bg-indigo-600 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
        >
          <Plus className="w-3.5 h-3.5" /> New hook
        </button>
      </div>
    </div>
  );

  return (
    <section className="space-y-4 pb-10">
      {header}
      {statusStrip}
      {list}
      {editorModal}
      {testModal}
      {logsModal}
      {deleteTarget && (
        <ConfirmModal
          title="Delete hook"
          destructive
          confirmLabel="Delete"
          body={<>Delete hook <span className="font-semibold font-mono text-indigo-500">#{deleteTarget.id}{deleteTarget.name ? ` ${deleteTarget.name}` : ''}</span>? Its SQL trigger and execution log are dropped too.</>}
          onConfirm={confirmDelete}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </section>
  );
}

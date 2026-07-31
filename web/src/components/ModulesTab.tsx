import { useEffect, useMemo, useState } from 'react';
import {
  Boxes, FileText, FileJson, Table2, Sheet, FileCode, FileSpreadsheet,
  Hash, Calendar, Sparkles, SplitSquareHorizontal, Search, Trash2, Eye, Plug, Plus,
} from 'lucide-react';
import { apiFetch } from '../lib/api';
import ConfirmModal from './ConfirmModal';

export type OptionKind = 'int' | 'float' | 'bool' | 'string' | 'date' | 'datetime' | 'select' | 'columns' | 'textarea' | 'generator';

export interface OptionField {
  key: string;
  label: string;
  kind: OptionKind;
  default?: unknown;
  choices?: string[];
  min?: number;
  max?: number;
  required?: boolean;
  description?: string;
}

export interface ModuleMeta {
  name: string;
  group: string;
  description: string;
  args: OptionField[];
  requiresNet: boolean;
  requiresFile: boolean;
  writable: boolean;
}

interface MountView {
  alias: string;
  module: string;
  args: Record<string, string>;
  declaredColumns: string[];
  createdAt: string;
}

interface ModulesInfo {
  enabled: boolean;
  write: boolean;
  modulesRoot: string;
  catalog: ModuleMeta[];
  mounts: MountView[];
}

interface ModulesTabProps {
  onToast: (message: string, type: 'error' | 'success') => void;
  onMountsChanged?: () => void;
}

// GROUP_PRESENTATION is presentation-only — an icon/label map keyed by the
// backend's catalog group string. It never decides which modules exist or
// what group they're in; an unknown group still renders via the fallback.
const GROUP_PRESENTATION: Record<string, { label: string; icon: typeof Boxes }> = {
  files: { label: 'File readers', icon: FileText },
  generators: { label: 'Generators', icon: Sparkles },
};

function groupPresentation(group: string) {
  return GROUP_PRESENTATION[group] || { label: group, icon: Boxes };
}

const MODULE_ICONS: Record<string, typeof Boxes> = {
  csv: Table2,
  jsonl: FileJson,
  parquet: FileSpreadsheet,
  xlsx: Sheet,
  yaml: FileCode,
  xml: FileCode,
  series: Hash,
  calendar: Calendar,
  fake: Sparkles,
  tokens: SplitSquareHorizontal,
};

async function jsonFetch(path: string, init?: RequestInit) {
  const res = await apiFetch(path, init);
  const body = await res.json();
  if (!body.ok) throw new Error(body.error?.message || 'Request failed');
  return body.data;
}

function defaultValueFor(field: OptionField): string {
  if (field.default !== undefined && field.default !== null) return String(field.default);
  if (field.kind === 'bool') return 'false';
  return '';
}

// coerceOptionValue converts a fake generator option's string wire value
// into the JSON type internal/seed's opt*() helpers expect (float64 for
// int/float, bool for bool, string otherwise) before it's embedded as
// <generator>:<json> in the column's mount arg.
function coerceOptionValue(field: OptionField, value: string): unknown {
  if (field.kind === 'int' || field.kind === 'float') {
    const n = Number(value);
    return Number.isNaN(n) ? value : n;
  }
  if (field.kind === 'bool') return value === 'true';
  return value;
}

const inputClass =
  'w-full text-sm px-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white';

function ArgField({
  field,
  value,
  onChange,
}: {
  field: OptionField;
  value: string;
  onChange: (v: string) => void;
}) {
  if (field.kind === 'bool') {
    return (
      <label className="flex items-center gap-1.5 text-xs">
        <input
          type="checkbox"
          checked={value === 'true'}
          onChange={(e) => onChange(e.target.checked ? 'true' : 'false')}
          className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
        />
        {field.label}
      </label>
    );
  }
  if (field.kind === 'textarea') {
    return (
      <label className="block space-y-1">
        <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
          {field.label}
          {field.required && <span className="text-red-500"> *</span>}
        </span>
        <textarea
          rows={2}
          value={value}
          placeholder={field.description}
          onChange={(e) => onChange(e.target.value)}
          className={inputClass}
        />
      </label>
    );
  }
  return (
    <label className="block space-y-1">
      <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
        {field.label}
        {field.required && <span className="text-red-500"> *</span>}
      </span>
      <input
        type={field.kind === 'int' || field.kind === 'float' ? 'number' : field.kind === 'date' ? 'date' : 'text'}
        step={field.kind === 'float' ? 'any' : undefined}
        value={value}
        placeholder={field.description}
        onChange={(e) => onChange(e.target.value)}
        className={inputClass}
      />
    </label>
  );
}

// fake is the one module with a dynamic argument set (VTABS.md #9): rows=
// plus one <column>=<generator> pair per declared column, rather than a
// fixed schema — so it can't be rendered from ModuleDef.Args like every
// other module and needs its own repeatable-row UI below.
const FAKE_MODULE_NAME = 'fake';

interface GeneratorMeta {
  name: string;
  group: string;
  description?: string;
  optionsSchema?: OptionField[];
}

interface FakeColumn {
  name: string;
  generator: string;
  // Values for the selected generator's own OptionsSchema (e.g. oneOf's
  // `values`), keyed by option key, all as their string wire form — same
  // convention as argValues for the module's own args.
  options: Record<string, string>;
}

export default function ModulesTab({ onToast, onMountsChanged }: ModulesTabProps) {
  const [info, setInfo] = useState<ModulesInfo | null>(null);
  const [generatorCatalog, setGeneratorCatalog] = useState<GeneratorMeta[]>([]);
  const [search, setSearch] = useState('');
  const [activeGroup, setActiveGroup] = useState<string | null>(null);
  const [selectedModule, setSelectedModule] = useState<ModuleMeta | null>(null);
  const [alias, setAlias] = useState('');
  const [argValues, setArgValues] = useState<Record<string, string>>({});
  const [fakeColumns, setFakeColumns] = useState<FakeColumn[]>([]);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState<{ alias: string; columns: string[]; rows: unknown[][] } | null>(null);
  const [unmountConfirm, setUnmountConfirm] = useState<string | null>(null);

  const refresh = () => {
    jsonFetch('/modules')
      .then(setInfo)
      .catch((err) => onToast(err.message || 'Failed to load modules', 'error'));
  };

  useEffect(() => {
    refresh();
    // The fake module's generator names (and their own OptionsSchema, for
    // generators like oneOf that require options to run at all) come from
    // the same catalog the Seed tab uses — fetched standalone here since
    // this tab has no target table to build a seed plan against.
    jsonFetch('/seed/generators/catalog')
      .then((data) => setGeneratorCatalog((data.generatorCatalog ?? []) as GeneratorMeta[]))
      .catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const generatorNames = useMemo(
    () => generatorCatalog.map((g) => g.name).sort(),
    [generatorCatalog]
  );

  const catalog = info?.catalog ?? [];

  const groups = useMemo(() => {
    const counts = new Map<string, number>();
    for (const m of catalog) counts.set(m.group, (counts.get(m.group) ?? 0) + 1);
    return Array.from(counts.entries()).map(([group, count]) => ({ group, count }));
  }, [catalog]);

  const filtered = useMemo(() => {
    return catalog.filter((m) => {
      if (activeGroup && m.group !== activeGroup) return false;
      if (!search) return true;
      const q = search.toLowerCase();
      return m.name.toLowerCase().includes(q) || m.description.toLowerCase().includes(q);
    });
  }, [catalog, activeGroup, search]);

  const selectModule = (mod: ModuleMeta) => {
    setSelectedModule(mod);
    setAlias('');
    setPreview(null);
    const initial: Record<string, string> = {};
    for (const f of mod.args) initial[f.key] = defaultValueFor(f);
    setArgValues(initial);
    setFakeColumns(mod.name === FAKE_MODULE_NAME ? [{ name: '', generator: '', options: {} }] : []);
  };

  const handleMount = async () => {
    if (!selectedModule) return;
    if (!alias.trim()) {
      onToast('An alias is required', 'error');
      return;
    }
    const isFake = selectedModule.name === FAKE_MODULE_NAME;
    if (isFake) {
      const complete = fakeColumns.filter((c) => c.name.trim() && c.generator.trim());
      if (complete.length === 0) {
        onToast('fake needs at least one column with a name and a generator', 'error');
        return;
      }
      const seen = new Set<string>();
      for (const c of complete) {
        if (seen.has(c.name.trim())) {
          onToast(`Duplicate column name "${c.name.trim()}"`, 'error');
          return;
        }
        seen.add(c.name.trim());
      }
    }
    setBusy(true);
    try {
      const args: Record<string, string> = {};
      for (const f of selectedModule.args) {
        const v = argValues[f.key];
        if (v !== undefined && v !== '') args[f.key] = v;
      }
      if (isFake) {
        for (const c of fakeColumns) {
          if (!c.name.trim() || !c.generator.trim()) continue;
          const genName = c.generator.trim();
          const schema = generatorCatalog.find((g) => g.name === genName)?.optionsSchema ?? [];
          const opts: Record<string, unknown> = {};
          for (const f of schema) {
            const v = c.options[f.key];
            if (v === undefined || v === '') continue;
            opts[f.key] = coerceOptionValue(f, v);
          }
          const value = Object.keys(opts).length > 0 ? `${genName}:${JSON.stringify(opts)}` : genName;
          args[c.name.trim()] = value;
        }
      }
      const data = await jsonFetch('/modules/mounts', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ module: selectedModule.name, alias: alias.trim(), args }),
      });
      onToast(`Mounted ${selectedModule.name} as ${alias.trim()}`, 'success');
      setPreview(null);
      const prev = await jsonFetch(`/modules/mounts/${encodeURIComponent(data.alias)}/preview`, { method: 'POST' });
      setPreview({ alias: data.alias, columns: prev.columns, rows: prev.rows });
      refresh();
      onMountsChanged?.();
    } catch (err: any) {
      onToast(err.message || 'Failed to mount module', 'error');
    } finally {
      setBusy(false);
    }
  };

  const handlePreview = async (a: string) => {
    try {
      const data = await jsonFetch(`/modules/mounts/${encodeURIComponent(a)}/preview`, { method: 'POST' });
      setPreview({ alias: a, columns: data.columns, rows: data.rows });
    } catch (err: any) {
      onToast(err.message || 'Failed to preview mount', 'error');
    }
  };

  const handleUnmount = async (a: string) => {
    try {
      await jsonFetch(`/modules/mounts/${encodeURIComponent(a)}`, { method: 'DELETE' });
      onToast(`Unmounted ${a}`, 'success');
      if (preview?.alias === a) setPreview(null);
      refresh();
      onMountsChanged?.();
    } catch (err: any) {
      onToast(err.message || 'Failed to unmount', 'error');
    }
  };

  if (!info) {
    return <div className="text-sm text-slate-500">Loading…</div>;
  }

  return (
    <section className="space-y-4 max-w-5xl">
      {!info.enabled && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-800/40 text-sm px-4 py-2.5">
          Virtual table modules are <span className="text-slate-400">off</span>. Relaunch with{' '}
          <span className="font-mono">--modules</span> to enable them.
        </div>
      )}

      {/* Active mounts */}
      <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
        <div className="flex items-center gap-2 text-sm font-medium">
          <Plug className="w-4 h-4 text-slate-400" />
          Active mounts
        </div>
        {info.mounts.length === 0 ? (
          <p className="text-xs text-slate-500">No active mounts.</p>
        ) : (
          <div className="space-y-1.5">
            {info.mounts.map((m) => (
              <div
                key={m.alias}
                className="flex items-center justify-between gap-2 px-2.5 py-1.5 rounded-md bg-slate-50 dark:bg-slate-800/40 text-sm"
              >
                <div className="flex items-center gap-2 min-w-0">
                  <span className="font-mono text-xs text-indigo-600 dark:text-indigo-400 truncate">{m.alias}</span>
                  <span className="text-xs text-slate-400">{m.module}</span>
                  <span className="text-xs text-slate-400">{m.declaredColumns.length} columns</span>
                </div>
                <div className="flex items-center gap-1 shrink-0">
                  <button
                    onClick={() => handlePreview(m.alias)}
                    disabled={!info.enabled}
                    className="p-1.5 rounded-md text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40"
                    title="Preview rows"
                  >
                    <Eye className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={() => setUnmountConfirm(m.alias)}
                    disabled={!info.enabled}
                    className="p-1.5 rounded-md text-red-500 hover:bg-red-50 dark:hover:bg-red-950/30 disabled:opacity-40"
                    title="Unmount"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {preview && (
          <div className="rounded-md border border-slate-200 dark:border-slate-800 overflow-auto">
            <table className="w-full text-xs code">
              <thead className="bg-slate-50 dark:bg-slate-800/60">
                <tr>
                  {preview.columns.map((c) => (
                    <th key={c} className="text-left px-2 py-1 font-medium text-slate-500">{c}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {preview.rows.map((row, i) => (
                  <tr key={i} className="border-t border-slate-100 dark:border-slate-800">
                    {row.map((v, j) => (
                      <td key={j} className="px-2 py-1 whitespace-nowrap">{v === null ? <span className="text-slate-400">NULL</span> : String(v)}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Catalog browser + mount form */}
      <div className="rounded-lg border border-slate-200 dark:border-slate-800 flex" style={{ minHeight: 360 }}>
        <div className="w-40 border-r border-slate-200 dark:border-slate-800 p-2 space-y-0.5 shrink-0">
          <button
            onClick={() => setActiveGroup(null)}
            className={`w-full text-left px-2 py-1.5 rounded-md text-xs ${!activeGroup ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-800'}`}
          >
            All ({catalog.length})
          </button>
          {groups.map(({ group, count }) => {
            const { label, icon: Icon } = groupPresentation(group);
            return (
              <button
                key={group}
                onClick={() => setActiveGroup(group)}
                className={`w-full text-left px-2 py-1.5 rounded-md text-xs flex items-center gap-1.5 ${activeGroup === group ? 'bg-indigo-50 text-indigo-600 dark:bg-indigo-400/10 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-800'}`}
              >
                <Icon className="w-3.5 h-3.5" /> {label} ({count})
              </button>
            );
          })}
        </div>

        <div className="flex-1 flex flex-col min-w-0">
          <div className="p-2 border-b border-slate-200 dark:border-slate-800">
            <div className="relative">
              <Search className="w-3.5 h-3.5 text-slate-400 absolute left-2 top-1/2 -translate-y-1/2" />
              <input
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search modules…"
                className="w-full text-sm pl-7 pr-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
              />
            </div>
          </div>

          <div className="flex-1 flex min-h-0">
            <div className="w-1/2 overflow-y-auto p-2 space-y-1 border-r border-slate-200 dark:border-slate-800">
              {filtered.map((mod) => {
                const Icon = MODULE_ICONS[mod.name] || Boxes;
                return (
                  <button
                    key={mod.name}
                    onClick={() => selectModule(mod)}
                    className={`w-full text-left p-2 rounded-md border ${
                      selectedModule?.name === mod.name
                        ? 'border-indigo-400 bg-indigo-50 dark:bg-indigo-400/10'
                        : 'border-transparent hover:bg-slate-100 dark:hover:bg-slate-800'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Icon className="w-4 h-4 text-slate-400" />
                      <span className="font-mono text-xs font-medium">{mod.name}</span>
                    </div>
                    <p className="text-xs text-slate-500 mt-0.5">{mod.description}</p>
                  </button>
                );
              })}
            </div>

            <div className="w-1/2 overflow-y-auto p-3">
              {!selectedModule ? (
                <p className="text-xs text-slate-500">Select a module to mount it.</p>
              ) : (
                <div className="space-y-3">
                  <div>
                    <p className="font-mono text-sm font-medium">{selectedModule.name}</p>
                    <p className="text-xs text-slate-500">{selectedModule.description}</p>
                  </div>
                  <label className="block space-y-1">
                    <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
                      Alias<span className="text-red-500"> *</span>
                    </span>
                    <input
                      value={alias}
                      onChange={(e) => setAlias(e.target.value)}
                      placeholder="e.g. vendor_prices"
                      className={inputClass}
                    />
                  </label>
                  {selectedModule.args.map((f) => (
                    <ArgField
                      key={f.key}
                      field={f}
                      value={argValues[f.key] ?? ''}
                      onChange={(v) => setArgValues((prev) => ({ ...prev, [f.key]: v }))}
                    />
                  ))}
                  {selectedModule.name === FAKE_MODULE_NAME && (
                    <div className="space-y-1.5">
                      <span className="text-xs font-medium text-slate-600 dark:text-slate-300">
                        Columns<span className="text-red-500"> *</span>
                      </span>
                      <datalist id="fake-generator-names">
                        {generatorNames.map((n) => (
                          <option key={n} value={n} />
                        ))}
                      </datalist>
                      {fakeColumns.map((col, i) => {
                        const genSchema = generatorCatalog.find((g) => g.name === col.generator.trim())?.optionsSchema ?? [];
                        return (
                          <div key={i} className="space-y-1.5 rounded-md border border-slate-200 dark:border-slate-800 p-2">
                            <div className="flex items-center gap-1.5">
                              <input
                                value={col.name}
                                placeholder="column name"
                                onChange={(e) =>
                                  setFakeColumns((prev) => prev.map((c, j) => (j === i ? { ...c, name: e.target.value } : c)))
                                }
                                className={inputClass}
                              />
                              <input
                                value={col.generator}
                                list="fake-generator-names"
                                placeholder="generator, e.g. email"
                                onChange={(e) =>
                                  setFakeColumns((prev) =>
                                    prev.map((c, j) => (j === i ? { ...c, generator: e.target.value, options: {} } : c))
                                  )
                                }
                                className={inputClass}
                              />
                              <button
                                onClick={() => setFakeColumns((prev) => prev.filter((_, j) => j !== i))}
                                disabled={fakeColumns.length === 1}
                                title="Remove column"
                                className="p-1.5 rounded-md text-red-500 hover:bg-red-50 dark:hover:bg-red-950/30 disabled:opacity-30 disabled:cursor-not-allowed shrink-0"
                              >
                                <Trash2 className="w-3.5 h-3.5" />
                              </button>
                            </div>
                            {genSchema.length > 0 && (
                              <div className="pl-1 space-y-1.5 border-l-2 border-indigo-200 dark:border-indigo-900 ml-1">
                                <p className="text-xs text-slate-400">
                                  {col.generator.trim()} needs:
                                </p>
                                {genSchema.map((f) => (
                                  <ArgField
                                    key={f.key}
                                    field={f}
                                    value={col.options[f.key] ?? defaultValueFor(f)}
                                    onChange={(v) =>
                                      setFakeColumns((prev) =>
                                        prev.map((c, j) => (j === i ? { ...c, options: { ...c.options, [f.key]: v } } : c))
                                      )
                                    }
                                  />
                                ))}
                              </div>
                            )}
                          </div>
                        );
                      })}
                      <button
                        onClick={() => setFakeColumns((prev) => [...prev, { name: '', generator: '', options: {} }])}
                        className="flex items-center gap-1 text-xs text-indigo-600 dark:text-indigo-400 hover:underline"
                      >
                        <Plus className="w-3 h-3" /> Add column
                      </button>
                      <p className="text-xs text-slate-400">
                        Generator names come from the Seed tab's registry (email, firstName, country, …) — start typing for suggestions.
                      </p>
                    </div>
                  )}
                  <button
                    onClick={handleMount}
                    disabled={busy || !info.enabled}
                    title={!info.enabled ? '--modules was not passed at launch' : undefined}
                    className="w-full px-3 py-1.5 rounded-md text-sm font-medium bg-indigo-600 text-white hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    Mount
                  </button>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {unmountConfirm && (
        <ConfirmModal
          title="Unmount table"
          destructive
          confirmLabel="Unmount"
          body={<>Unmount <span className="font-semibold font-mono text-indigo-650 dark:text-indigo-400">"{unmountConfirm}"</span>? Any query or join relying on it will stop working until it's remounted.</>}
          onCancel={() => setUnmountConfirm(null)}
          onConfirm={() => {
            handleUnmount(unmountConfirm);
            setUnmountConfirm(null);
          }}
        />
      )}
    </section>
  );
}

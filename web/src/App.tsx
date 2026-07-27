import { useEffect, useState, useRef } from 'react';
import {
  Moon,
  Sun,
  Monitor,
  Terminal,
  Search,
  Save,
  X,
  Edit2,
  Trash2,
  AlertTriangle,
  FileSpreadsheet,
  Braces,
  Database,
  RefreshCw,
  Check,
  AlertCircle,
  Download,
  AudioLines,
  LibraryBig
} from 'lucide-react';
import {
  sniffHex,
  dataUriFromHex,
  downloadHex,
  type BlobMediaType,
} from './lib/blobMedia';
import { basicSetup } from 'codemirror';
import { sql as sqlLanguage } from '@codemirror/lang-sql';
import { EditorState, Compartment } from '@codemirror/state';
import { EditorView, keymap } from '@codemirror/view';
import GeneratorPicker from './components/GeneratorPicker';
import GeneratorOptionsForm from './components/GeneratorOptionsForm';
import SandboxEmptyState from './components/SandboxEmptyState';
import DbSwitcher from './components/DbSwitcher';
import SandboxManagePage from './components/SandboxManagePage';
import ExamplesPicker, { type ExampleMeta } from './components/ExamplesPicker';
import ConfirmModal from './components/ConfirmModal';
import RestTab from './components/RestTab';
import { apiFetch, apiUrl, setApiBase } from './lib/api';

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

interface SandboxDbEntry {
  id: string;
  displayName: string;
  sizeBytes: number;
  createdAt: string;
  lastModifiedAt: string;
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

interface ForeignKeyDraft {
  columns: string[];
  refTable: string;
  refColumns: string[];
  onDelete: string;
  onUpdate: string;
  match: string;
}

const FK_ACTIONS = ['NO ACTION', 'RESTRICT', 'CASCADE', 'SET NULL', 'SET DEFAULT'];
const FK_MATCH_MODES = ['NONE', 'SIMPLE', 'PARTIAL', 'FULL'];

function emptyFkDraft(): ForeignKeyDraft {
  return { columns: [], refTable: '', refColumns: [], onDelete: 'NO ACTION', onUpdate: 'NO ACTION', match: 'NONE' };
}

// isRefColumnsCovered mirrors the backend's rule that refColumns must be
// covered by a primary key or unique index on the referenced table — used
// only for a client-side hint, the backend remains the source of truth.
function isRefColumnsCovered(refSchema: TableSchema | undefined, refColumns: string[]): boolean {
  if (!refSchema || refColumns.length === 0) return true;
  const wanted = refColumns.map(c => c.toLowerCase());
  const pkSet = new Set(refSchema.primaryKey.map(c => c.toLowerCase()));
  if (wanted.length === refSchema.primaryKey.length && wanted.every(c => pkSet.has(c))) return true;
  return refSchema.indexes.some(idx => {
    if (!idx.unique || idx.columns.length !== wanted.length) return false;
    const idxSet = new Set(idx.columns.map(c => c.toLowerCase()));
    return wanted.every(c => idxSet.has(c));
  });
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

interface SeedColumnPlan {
  name: string;
  type: string;
  skip: boolean;
  reason: string | null;
  generator: string | null;
  options: Record<string, any> | null;
  uniqueGroup?: string[];
}

type OptionKind = 'int' | 'float' | 'bool' | 'string' | 'datetime' | 'select' | 'columns' | 'textarea' | 'generator';

interface OptionField {
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

interface GeneratorMeta {
  name: string;
  group: string;
  aliases?: string[];
  description?: string;
  affinities: string[];
  optionsSchema?: OptionField[];
  stateful?: boolean;
}

interface SeedPlan {
  columns: SeedColumnPlan[];
  availableGenerators: string[];
  generatorCatalog: GeneratorMeta[];
}

interface SeedColumnSelection {
  generator: string;
  options: Record<string, any>;
}

interface QueryResult {
  columns: string[];
  rows: any[][];
  rowsAffected: number;
  durationMs: number;
  limit: number;
  truncated: boolean;
  schemaChanged: boolean;
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
  const res = await apiFetch('/query', {
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
  const res = await apiFetch(`/export/query?format=${format}`, {
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

async function createTable(data: any): Promise<any> {
  const res = await apiFetch('/tables', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function alterTable(name: string, data: any): Promise<any> {
  const res = await apiFetch(`/tables/${name}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function dropTable(name: string): Promise<any> {
  const res = await apiFetch(`/tables/${name}`, {
    method: 'DELETE',
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function insertRow(name: string, values: any): Promise<any> {
  const res = await apiFetch(`/tables/${name}/rows`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ values }),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function updateRow(name: string, key: any, values: any): Promise<any> {
  const res = await apiFetch(`/tables/${name}/rows`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key, values }),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function deleteRow(name: string, key: any): Promise<any> {
  const res = await apiFetch(`/tables/${name}/rows`, {
    method: 'DELETE',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key }),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function getSeedPlan(name: string): Promise<SeedPlan> {
  const res = await apiFetch(`/tables/${name}/seed/plan`);
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

async function seedTable(
  name: string,
  opts: { count: number; dryRun: boolean; columns: Record<string, { generator: string; options?: Record<string, any> }> }
): Promise<{ inserted: number } | { rows: Record<string, any>[] }> {
  const res = await apiFetch(`/tables/${name}/seed`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(opts),
  });
  const body = await res.json();
  if (!res.ok || !body.ok) {
    const err = body.error || { code: 'HTTP_ERROR', message: `HTTP error ${res.status}` };
    throw new Error(`${err.code}: ${err.message}`);
  }
  return body.data;
}

export default function App() {
  const [meta, setMeta] = useState<MetaData | null>(null);
  const [tables, setTables] = useState<TableInfo[]>([]);
  const [selectedTable, setSelectedTable] = useState<TableInfo | null>(null);
  const [schema, setSchema] = useState<TableSchema | null>(null);
  const [schemaError, setSchemaError] = useState<string | null>(null);
  const [blobModal, setBlobModal] = useState<{ column: string; hex: string; type: BlobMediaType } | null>(null);
  const [blobModalView, setBlobModalView] = useState<'preview' | 'hex'>('preview');
  const [rowsData, setRowsData] = useState<RowsData | null>(null);
  const [rowsLoading, setRowsLoading] = useState<boolean>(false);
  const [tablesLoading, setTablesLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [theme, setTheme] = useState<'light' | 'dark' | 'system'>(() => {
    const saved = localStorage.getItem('color-scheme');
    if (saved === 'dark' || saved === 'light' || saved === 'system') return saved;
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  });
  const [systemPrefersDark, setSystemPrefersDark] = useState<boolean>(() =>
    window.matchMedia('(prefers-color-scheme: dark)').matches
  );
  const resolvedDark = theme === 'dark' || (theme === 'system' && systemPrefersDark);

  const [activeTab, setActiveTab] = useState<string>('data');
  const [editorMode, setEditorMode] = useState<'create' | 'alter'>('create');
  const [searchQuery, setSearchQuery] = useState<string>('');
  
  // Table Editor create states
  const [newTableName, setNewTableName] = useState<string>('');
  const [newTableColumns, setNewTableColumns] = useState<Array<{ name: string; type: string; pk: boolean; notnull: boolean; unique: boolean; defaultVal: string }>>([
    { name: 'id', type: 'INTEGER', pk: true, notnull: false, unique: false, defaultVal: '' }
  ]);
  const [isCompositePk, setIsCompositePk] = useState<boolean>(false);
  const [compositePkColumns, setCompositePkColumns] = useState<string[]>([]);
  const [createTableError, setCreateTableError] = useState<string | null>(null);
  const [newTableForeignKeys, setNewTableForeignKeys] = useState<ForeignKeyDraft[]>([]);
  const [createFkErrors, setCreateFkErrors] = useState<Record<number, string>>({});

  // Table Editor alter states
  const [newTableNameInput, setNewTableNameInput] = useState<string>('');
  const [addColName, setAddColName] = useState<string>('');
  const [addColType, setAddColType] = useState<string>('TEXT');
  const [addColNotNull, setAddColNotNull] = useState<boolean>(false);
  const [addColDefault, setAddColDefault] = useState<string>('');
  const [renamingColumn, setRenamingColumn] = useState<Record<string, string>>({}); // originalName -> newName
  const [allSchemas, setAllSchemas] = useState<Record<string, TableSchema>>({});
  const [addFk, setAddFk] = useState<ForeignKeyDraft>(emptyFkDraft());
  const [addFkError, setAddFkError] = useState<string | null>(null);
  const [dropFkConfirmation, setDropFkConfirmation] = useState<ForeignKeyInfo | null>(null);

  // Data grid inline CRUD states
  const [inlineAddRow, setInlineAddRow] = useState<Record<string, any> | null>(null);
  const [editingRowIndex, setEditingRowIndex] = useState<number | null>(null);
  const [editingRowValues, setEditingRowValues] = useState<Record<string, any>>({});
  const [refetchTrigger, setRefetchTrigger] = useState<number>(0);
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ row: any[]; rIdx: number } | null>(null);
  const [dropColumnConfirmation, setDropColumnConfirmation] = useState<{ colName: string } | null>(null);
  const [dropTableConfirmation, setDropTableConfirmation] = useState<boolean>(false);

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
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const showToast = (message: string, type: 'error' | 'success', duration?: number) => {
    if (toastTimerRef.current) clearTimeout(toastTimerRef.current);
    setToast({ message, type });
    toastTimerRef.current = setTimeout(() => setToast(null), duration ?? (type === 'error' ? 5000 : 3000));
  };

  // Seed states
  const [seedPlan, setSeedPlan] = useState<SeedPlan | null>(null);
  const [seedPlanLoading, setSeedPlanLoading] = useState<boolean>(false);
  const [seedPlanError, setSeedPlanError] = useState<string | null>(null);
  const [seedSelections, setSeedSelections] = useState<Record<string, SeedColumnSelection>>({});
  const [seedOverrides, setSeedOverrides] = useState<Record<string, boolean>>({});
  const [generatorPickerColumn, setGeneratorPickerColumn] = useState<string | null>(null);
  const [recentlyUsedGenerators, setRecentlyUsedGenerators] = useState<string[]>([]);
  const [seedGeneratorSamples, setSeedGeneratorSamples] = useState<Record<string, string>>({});
  const [seedCount, setSeedCount] = useState<number>(1000);
  const [seedPreviewRows, setSeedPreviewRows] = useState<Record<string, any>[] | null>(null);
  const [seedPreviewLoading, setSeedPreviewLoading] = useState<boolean>(false);
  const [seedInsertLoading, setSeedInsertLoading] = useState<boolean>(false);
  const [seedError, setSeedError] = useState<string | null>(null);

  // Export States
  const [applyFilterSort, setApplyFilterSort] = useState<boolean>(false);
  const [includeSchema, setIncludeSchema] = useState<boolean>(false);
  const [selectedExportFormat, setSelectedExportFormat] = useState<'csv' | 'json' | 'sql'>('csv');
  const [exportQueryLoading, setExportQueryLoading] = useState<boolean>(false);
  const [lastExecutedSql, setLastExecutedSql] = useState<string>('');

  // Sandbox mode state — normal (single-DB) mode is entirely unaffected by
  // this: sandboxMode stays false and none of it renders.
  const [sandboxMode, setSandboxMode] = useState<boolean>(false);
  const [activeDbId, setActiveDbId] = useState<string | null>(null);
  const [sandboxDbs, setSandboxDbs] = useState<SandboxDbEntry[]>([]);
  const [sandboxManageOpen, setSandboxManageOpen] = useState<boolean>(false);

  const refreshSandboxDbs = () => {
    fetch('/api/sandbox/dbs')
      .then((res) => res.json())
      .then((body) => {
        if (body.ok) setSandboxDbs(body.data ?? []);
      })
      .catch(console.error);
  };

  const switchActiveDb = (id: string) => {
    fetch('/api/sandbox/dbs/active', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id }),
    })
      .then((res) => res.json())
      .then((body) => {
        if (body.ok && body.data?.restStopped) {
          showToast('REST server stopped: active database changed', 'error');
        }
      })
      .catch(console.error);

    setApiBase(`/api/sandbox/dbs/${id}`);
    setActiveDbId(id);
    setSandboxManageOpen(false);
    setSelectedTable(null);
    setSchema(null);
    setSchemaError(null);
    setRowsData(null);
    setQueryResult(null);
    setQueryError(null);
    setTables([]);
    setMeta(null);
    setAllSchemas({});
    setActiveTab('data');
    setLoading(true);
    setError(null);
  };

  const handleSandboxUpload = async (file: File, name?: string): Promise<boolean> => {
    const form = new FormData();
    form.append('file', file);
    if (name) form.append('name', name);
    try {
      const res = await fetch('/api/sandbox/dbs', { method: 'POST', body: form });
      const body = await res.json();
      if (!body.ok) {
        throw new Error(body.error?.message || 'Upload failed');
      }
      refreshSandboxDbs();
      switchActiveDb(body.data.id);
      return true;
    } catch (err: any) {
      throw new Error(err.message || 'Upload failed');
    }
  };

  const handleSandboxCreate = async (name: string): Promise<boolean> => {
    try {
      const res = await fetch('/api/sandbox/dbs/new', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
      });
      const body = await res.json();
      if (!body.ok) {
        throw new Error(body.error?.message || 'Failed to create database');
      }
      refreshSandboxDbs();
      switchActiveDb(body.data.id);
      return true;
    } catch (err: any) {
      throw new Error(err.message || 'Failed to create database');
    }
  };

  const handleSandboxRename = async (id: string, displayName: string) => {
    try {
      const res = await fetch(`/api/sandbox/dbs/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ displayName }),
      });
      const body = await res.json();
      if (!body.ok) throw new Error(body.error?.message || 'Rename failed');
      refreshSandboxDbs();
    } catch (err: any) {
      showToast(err.message || 'Rename failed', 'error');
    }
  };

  const handleSandboxDelete = async (id: string) => {
    try {
      const res = await fetch(`/api/sandbox/dbs/${id}`, { method: 'DELETE' });
      const body = await res.json();
      if (!body.ok) throw new Error(body.error?.message || 'Delete failed');
      refreshSandboxDbs();
      if (id === activeDbId) {
        setApiBase('/api');
        setActiveDbId(null);
      }
    } catch (err: any) {
      showToast(err.message || 'Delete failed', 'error');
    }
  };

  const handleSandboxDownload = (id: string) => {
    window.location.href = `/api/sandbox/dbs/${id}/download`;
  };

  const fetchMetaAndTables = () => {
    setInfoLoading(true);
    setTablesLoading(true);
    setInfoError(null);
    Promise.all([
      apiFetch('/meta').then(res => res.json()),
      apiFetch('/tables').then(res => res.json())
    ])
      .then(([metaBody, tablesBody]) => {
        if (metaBody.ok) {
          setMeta(metaBody.data ?? null);
        } else {
          throw new Error(metaBody.error?.message || 'Failed to fetch database metadata');
        }

        if (tablesBody.ok) {
          const tableList = tablesBody.data ?? [];
          setTables(tableList);
          if (!selectedTable && tableList.length > 0) {
            setSelectedTable(tableList[0]);
          }
        } else {
          throw new Error(tablesBody.error?.message || 'Failed to fetch database tables');
        }
      })
      .catch((err) => {
        console.error(err);
        setInfoError(err.message || 'Failed to fetch database info');
        setError(err.message || 'Failed to fetch database info');
        showToast(err.message || 'Failed to fetch database info', 'error');
      })
      .finally(() => {
        setInfoLoading(false);
        setTablesLoading(false);
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
  const tableSearchRef = useRef<HTMLInputElement | null>(null);

  // Examples (--examples) state: null until we know whether the feature is
  // enabled server-side (GET /api/examples 404s when the flag is off).
  const [examplesList, setExamplesList] = useState<ExampleMeta[] | null>(null);
  const [examplesPickerOpen, setExamplesPickerOpen] = useState(false);
  const [pendingExampleSlug, setPendingExampleSlug] = useState<string | null>(null);

  const setEditorContents = (text: string) => {
    setSqlValue(text);
    const view = editorViewRef.current;
    if (view) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: text } });
    }
  };

  const applyExample = async (slug: string) => {
    try {
      // The examples registry is not scoped per-database, so this always
      // hits the top-level /api/examples/:slug route — never apiFetch,
      // which would resolve against the sandbox-scoped /api/sandbox/dbs/:id
      // base once a sandbox database is selected.
      const res = await fetch(`/api/examples/${encodeURIComponent(slug)}`);
      const body = await res.json();
      if (!res.ok || !body.ok) throw new Error(body.error?.message || 'Failed to load example');
      setEditorContents(body.data.schema);
    } catch (err: any) {
      showToast(err.message || 'Failed to load example', 'error');
    }
  };

  const handleSelectExample = (slug: string) => {
    const trivial = sqlValue.trim() === '' || sqlValue.trim() === 'SELECT * FROM sqlite_master LIMIT 10;';
    if (trivial) {
      applyExample(slug);
    } else {
      setPendingExampleSlug(slug);
    }
  };

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
      if (data.rowsAffected > 0 || data.schemaChanged) {
        fetchMetaAndTables();
        setRefetchTrigger((prev) => prev + 1);
        const message = data.rowsAffected > 0
          ? `${data.rowsAffected} row${data.rowsAffected === 1 ? '' : 's'} affected`
          : 'Schema updated';
        showToast(message, 'success');
      }
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
      showToast('Query exported successfully', 'success');
    } catch (err: any) {
      console.error(err);
      showToast(err.message || 'Failed to export query result', 'error');
    } finally {
      setExportQueryLoading(false);
    }
  };

  const getFkReferencingTables = (tableName: string) => {
    const referencing: string[] = [];
    Object.entries(allSchemas).forEach(([otherTableName, schema]) => {
      if (otherTableName === tableName) return;
      schema.foreignKeys.forEach(fk => {
        if (fk.table.toLowerCase() === tableName.toLowerCase()) {
          referencing.push(otherTableName);
        }
      });
    });
    return referencing;
  };

  const handleCreateTableSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!isWrite) return;
    setCreateTableError(null);
    setCreateFkErrors({});
    try {
      const columns = newTableColumns.map(col => ({
        name: col.name,
        type: col.type,
        pk: isCompositePk ? false : col.pk,
        notnull: col.notnull,
        unique: col.unique,
        default: col.defaultVal.trim() !== '' ? col.defaultVal.trim() : null
      }));

      const body: any = {
        name: newTableName,
        columns
      };

      if (isCompositePk) {
        body.primaryKey = compositePkColumns;
      }

      if (newTableForeignKeys.length > 0) {
        body.foreignKeys = newTableForeignKeys;
      }

      await createTable(body);
      showToast(`Table "${newTableName}" created successfully!`, 'success');

      // Reset form
      setNewTableName('');
      setNewTableColumns([{ name: 'id', type: 'INTEGER', pk: true, notnull: false, unique: false, defaultVal: '' }]);
      setIsCompositePk(false);
      setCompositePkColumns([]);
      setNewTableForeignKeys([]);

      fetchMetaAndTables();
      setSelectedTable({ name: newTableName, type: 'table', rowCount: 0 });
      setActiveTab('data');
    } catch (err: any) {
      // Backend errors are prefixed "foreign key <index>: <message>" — route
      // them to the specific FK row rather than the generic form banner.
      const msg: string = err.message || 'Failed to create table';
      const fkMatch = msg.match(/foreign key (\d+): (.+)$/i);
      if (fkMatch) {
        setCreateFkErrors({ [Number(fkMatch[1])]: fkMatch[2] });
      } else {
        setCreateTableError(msg);
      }
    }
  };

  const handleRenameTableSubmit = async () => {
    if (!selectedTable || !isWrite) return;
    try {
      await alterTable(selectedTable.name, {
        op: 'rename_table',
        newName: newTableNameInput
      });
      showToast(`Table renamed to "${newTableNameInput}" successfully!`, 'success');
      
      fetchMetaAndTables();
      setSelectedTable({ name: newTableNameInput, type: selectedTable.type, rowCount: selectedTable.rowCount });
    } catch (err: any) {
      showToast(err.message || 'Failed to rename table', 'error');
    }
  };

  const handleAddColumnSubmit = async () => {
    if (!selectedTable || !isWrite) return;
    try {
      await alterTable(selectedTable.name, {
        op: 'add_column',
        column: {
          name: addColName,
          type: addColType,
          notnull: addColNotNull,
          default: addColDefault.trim() !== '' ? addColDefault.trim() : null
        }
      });
      showToast(`Column "${addColName}" added successfully!`, 'success');
      setAddColName('');
      setAddColType('TEXT');
      setAddColNotNull(false);
      setAddColDefault('');
      
      // Reload schema
      const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
      const body = await res.json();
      if (body.ok) setSchema(body.data);
    } catch (err: any) {
      showToast(err.message || 'Failed to add column', 'error');
    }
  };

  const handleRenameColumnSubmit = async (fromCol: string) => {
    if (!selectedTable || !isWrite) return;
    const toCol = renamingColumn[fromCol];
    if (!toCol || toCol.trim() === '') return;
    try {
      await alterTable(selectedTable.name, {
        op: 'rename_column',
        from: fromCol,
        to: toCol
      });
      showToast(`Column renamed from "${fromCol}" to "${toCol}" successfully!`, 'success');
      setRenamingColumn(prev => {
        const copy = { ...prev };
        delete copy[fromCol];
        return copy;
      });
      
      // Reload schema
      const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
      const body = await res.json();
      if (body.ok) setSchema(body.data);
    } catch (err: any) {
      showToast(err.message || 'Failed to rename column', 'error');
    }
  };

  const handleDropColumnClick = (colName: string) => {
    if (!selectedTable || !isWrite) return;
    setDropColumnConfirmation({ colName });
  };

  const executeDropColumn = async (colName: string) => {
    if (!selectedTable || !isWrite) return;
    try {
      const data = await alterTable(selectedTable.name, {
        op: 'drop_column',
        column: colName
      });
      if (data.warnings && data.warnings.length > 0) {
        showToast(`Dropped column. Warning: ${data.warnings.join(', ')}`, 'success', 5000);
      } else {
        showToast(`Column "${colName}" dropped successfully!`, 'success', 5000);
      }

      // Reload schema
      const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
      const body = await res.json();
      if (body.ok) setSchema(body.data);
    } catch (err: any) {
      showToast(err.message || 'Failed to drop column', 'error');
    }
  };

  const handleAddForeignKeySubmit = async () => {
    if (!selectedTable || !isWrite) return;
    setAddFkError(null);
    try {
      await alterTable(selectedTable.name, {
        op: 'add_foreign_key',
        foreignKey: addFk
      });
      showToast('Foreign key added successfully!', 'success');
      setAddFk(emptyFkDraft());

      // Reload schema
      const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
      const body = await res.json();
      if (body.ok) {
        setSchema(body.data);
        setAllSchemas(prev => ({ ...prev, [selectedTable.name]: body.data }));
      }
    } catch (err: any) {
      setAddFkError(err.message || 'Failed to add foreign key');
    }
  };

  const handleDropForeignKeyClick = async (fk: ForeignKeyInfo) => {
    if (!selectedTable || !isWrite) return;
    // Refetch schema first since foreign key ids are not stable across mutations.
    const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
    const body = await res.json();
    if (body.ok) {
      setSchema(body.data);
      const fresh = (body.data.foreignKeys as ForeignKeyInfo[]).find(
        f => f.from === fk.from && f.table === fk.table && f.to === fk.to
      );
      setDropFkConfirmation(fresh || fk);
    } else {
      setDropFkConfirmation(fk);
    }
  };

  const executeDropForeignKey = async (fk: ForeignKeyInfo) => {
    if (!selectedTable || !isWrite) return;
    try {
      await alterTable(selectedTable.name, {
        op: 'drop_foreign_key',
        foreignKey: { id: fk.id }
      });
      showToast('Foreign key dropped successfully!', 'success');

      const res = await apiFetch(`/tables/${selectedTable.name}/schema`);
      const body = await res.json();
      if (body.ok) {
        setSchema(body.data);
        setAllSchemas(prev => ({ ...prev, [selectedTable.name]: body.data }));
      }
    } catch (err: any) {
      showToast(err.message || 'Failed to drop foreign key', 'error');
    }
  };

  const handleDropTableClick = () => {
    if (!selectedTable || !isWrite) return;
    setDropTableConfirmation(true);
  };

  const executeDropTable = async () => {
    if (!selectedTable || !isWrite) return;
    try {
      await dropTable(selectedTable.name);
      showToast(`Table "${selectedTable.name}" dropped successfully!`, 'success');
      
      fetchMetaAndTables();
      setSelectedTable(null);
      setActiveTab('info');
    } catch (err: any) {
      showToast(err.message || 'Failed to drop table', 'error');
    }
  };

  // Standard SQLite type-affinity rules (see SPEC.md), used to drive the
  // GeneratorPicker's default type-compatibility filtering.
  const sqliteAffinity = (type: string): string => {
    const t = (type || '').toUpperCase();
    if (t.includes('INT')) return 'INTEGER';
    if (t.includes('CHAR') || t.includes('CLOB') || t.includes('TEXT')) return 'TEXT';
    if (t.includes('BLOB') || t === '') return 'BLOB';
    if (t.includes('REAL') || t.includes('FLOA') || t.includes('DOUB')) return 'REAL';
    return 'NUMERIC';
  };

  const defaultGeneratorForType = (type: string): string => {
    const t = type.toUpperCase();
    if (t.includes('INT')) return 'int';
    if (t.includes('CHAR') || t.includes('CLOB') || t.includes('TEXT')) return 'uuid';
    if (t.includes('BLOB')) return 'bytes';
    return 'float';
  };

  const toggleSeedOverride = (col: SeedColumnPlan) => {
    setSeedOverrides((prev) => ({ ...prev, [col.name]: !prev[col.name] }));
    setSeedSelections((prev) => {
      if (prev[col.name]) return prev;
      return {
        ...prev,
        [col.name]: { generator: col.generator || defaultGeneratorForType(col.type), options: col.options || {} },
      };
    });
  };

  const updateSeedGenerator = (colName: string, generator: string) => {
    setSeedSelections((prev) => ({ ...prev, [colName]: { generator, options: {} } }));
    setRecentlyUsedGenerators((prev) => [generator, ...prev.filter((g) => g !== generator)].slice(0, 8));
  };

  const updateSeedOption = (colName: string, key: string, value: any) => {
    setSeedSelections((prev) => ({
      ...prev,
      [colName]: { generator: prev[colName]?.generator || '', options: { ...prev[colName]?.options, [key]: value } },
    }));
  };

  const buildSeedColumnsPayload = (): Record<string, { generator: string; options?: Record<string, any> }> => {
    const payload: Record<string, { generator: string; options?: Record<string, any> }> = {};
    if (!seedPlan) return payload;
    seedPlan.columns.forEach((col) => {
      const included = !col.skip || seedOverrides[col.name];
      if (!included) return;
      const sel = seedSelections[col.name];
      if (!sel || !sel.generator) return;
      payload[col.name] = { generator: sel.generator, options: sel.options };
    });
    return payload;
  };

  const generatorMetaByName = (name: string): GeneratorMeta | undefined =>
    seedPlan?.generatorCatalog.find((g) => g.name === name);

  // Fetch (once per column+generator pair, cached in state) the live sample
  // value shown next to the generator-picker trigger button. Skipped for
  // foreignKey/formula, which the sample endpoint rejects (need row context).
  useEffect(() => {
    if (!seedPlan) return;
    Object.entries(seedSelections).forEach(([colName, sel]) => {
      if (!sel?.generator || sel.generator === 'foreignKey' || sel.generator === 'formula') return;
      const cacheKey = `${colName}:${sel.generator}`;
      if (seedGeneratorSamples[cacheKey] !== undefined) return;
      const meta = generatorMetaByName(sel.generator);
      if (!meta) return;
      const col = seedPlan.columns.find((c) => c.name === colName);
      const affinity = col && meta.affinities.includes(sqliteAffinity(col.type)) ? sqliteAffinity(col.type) : meta.affinities[0];
      if (!affinity) return;
      apiFetch(`/seed/generators/${encodeURIComponent(sel.generator)}/sample?affinity=${encodeURIComponent(affinity)}`)
        .then((res) => res.json())
        .then((body) => {
          if (!body.ok) return;
          const text = body.data?.sample === null || body.data?.sample === undefined ? '' : String(body.data.sample);
          setSeedGeneratorSamples((prev) => ({ ...prev, [cacheKey]: text }));
        })
        .catch(() => {});
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedSelections, seedPlan]);

  const handleSeedCountChange = (raw: string) => {
    const n = parseInt(raw, 10);
    if (isNaN(n)) {
      setSeedCount(1);
      return;
    }
    setSeedCount(Math.max(1, Math.min(100000, n)));
  };

  const handleSeedPreview = async () => {
    if (!selectedTable) return;
    setSeedPreviewLoading(true);
    setSeedError(null);
    try {
      const data = await seedTable(selectedTable.name, { count: 5, dryRun: true, columns: buildSeedColumnsPayload() });
      setSeedPreviewRows((data as { rows: Record<string, any>[] }).rows || []);
    } catch (err: any) {
      setSeedError(err.message || 'Failed to preview seed data');
    } finally {
      setSeedPreviewLoading(false);
    }
  };

  const handleSeedInsert = async () => {
    if (!selectedTable || !isWrite) return;
    setSeedInsertLoading(true);
    setSeedError(null);
    try {
      const data = await seedTable(selectedTable.name, { count: seedCount, dryRun: false, columns: buildSeedColumnsPayload() });
      const inserted = (data as { inserted: number }).inserted;
      showToast(`Inserted ${inserted} rows`, 'success');
      setSeedPreviewRows(null);
      setPage(1);
      setRefetchTrigger((prev) => prev + 1);
      fetchMetaAndTables();
    } catch (err: any) {
      setSeedError(err.message || 'Failed to seed table');
    } finally {
      setSeedInsertLoading(false);
    }
  };

  const getRowKey = (row: any[]) => {
    if (!schema || !rowsData) return {};
    const key: Record<string, any> = {};
    if (schema.primaryKey.length > 0) {
      schema.primaryKey.forEach(pkCol => {
        const idx = rowsData.columns.indexOf(pkCol);
        if (idx !== -1) {
          key[pkCol] = row[idx];
        }
      });
    } else if (!schema.withoutRowid) {
      const idx = rowsData.columns.indexOf('rowid');
      if (idx !== -1) {
        key['rowid'] = row[idx];
      }
    }
    return key;
  };

  const handleAddRowClick = () => {
    if (!isWrite || !schema) return;
    const emptyRow: Record<string, any> = {};
    schema.columns.forEach(col => {
      if (col.defaultVal) {
        let cleanDef = col.defaultVal;
        if (cleanDef.startsWith("'") && cleanDef.endsWith("'")) {
          cleanDef = cleanDef.slice(1, -1);
        }
        emptyRow[col.name] = cleanDef;
      } else {
        emptyRow[col.name] = '';
      }
    });
    setInlineAddRow(emptyRow);
  };

  const handleSaveAddRow = async () => {
    if (!selectedTable || !inlineAddRow || !isWrite) return;
    try {
      // Coerce numeric columns on client side
      const values: Record<string, any> = {};
      Object.entries(inlineAddRow).forEach(([col, val]) => {
        if (col === 'rowid') return; // omit rowid
        const colType = schema?.columns.find(c => c.name === col)?.type || '';
        if (val === '' || val === null || val === undefined) {
          values[col] = null;
        } else if (['integer', 'real', 'numeric'].includes(colType.toLowerCase())) {
          const num = Number(val);
          values[col] = isNaN(num) ? val : num;
        } else {
          values[col] = val;
        }
      });

      await insertRow(selectedTable.name, values);
      showToast('Row inserted successfully', 'success');
      setInlineAddRow(null);
      
      // Reload rows
      setPage(1);
      setRefetchTrigger(prev => prev + 1);
      fetchMetaAndTables(); // updates count in sidebar
    } catch (err: any) {
      showToast(err.message || 'Failed to insert row', 'error');
    }
  };

  const handleSaveEditRow = async (row: any[]) => {
    if (!selectedTable || !isWrite) return;
    try {
      const key = getRowKey(row);
      const values: Record<string, any> = {};
      Object.entries(editingRowValues).forEach(([col, val]) => {
        if (col === 'rowid') return; // omit rowid
        const colType = schema?.columns.find(c => c.name === col)?.type || '';
        if (val === '' || val === null || val === undefined) {
          values[col] = null;
        } else if (['integer', 'real', 'numeric'].includes(colType.toLowerCase())) {
          const num = Number(val);
          values[col] = isNaN(num) ? val : num;
        } else {
          values[col] = val;
        }
      });

      await updateRow(selectedTable.name, key, values);
      showToast('Row updated successfully', 'success');
      setEditingRowIndex(null);
      setRefetchTrigger(prev => prev + 1);
    } catch (err: any) {
      showToast(err.message || 'Failed to update row', 'error');
    }
  };

  const handleDeleteRowClick = (row: any[], rIdx: number) => {
    if (!selectedTable || !isWrite) return;
    setDeleteConfirmation({ row, rIdx });
  };

  const executeDeleteRow = async (row: any[], rIdx: number) => {
    if (!selectedTable || !isWrite) return;
    const key = getRowKey(row);
    
    // Optimistic delete
    const originalRows = rowsData ? [...rowsData.rows] : [];
    const originalTotal = rowsData ? rowsData.total : 0;
    if (rowsData) {
      setRowsData({
        ...rowsData,
        rows: rowsData.rows.filter((_, i) => i !== rIdx),
        total: rowsData.total - 1
      });
    }

    try {
      await deleteRow(selectedTable.name, key);
      showToast('Row deleted successfully', 'success');
      setRefetchTrigger(prev => prev + 1);
      fetchMetaAndTables(); // refresh sidebar count
    } catch (err: any) {
      // Revert on error
      if (rowsData) {
        setRowsData({
          ...rowsData,
          rows: originalRows,
          total: originalTotal
        });
      }
      showToast(err.message || 'Failed to delete row', 'error');
    }
  };

  useEffect(() => {
    localStorage.setItem('color-scheme', theme);
    if (theme !== 'system') return;
    const mql = window.matchMedia('(prefers-color-scheme: dark)');
    setSystemPrefersDark(mql.matches);
    const onChange = (e: MediaQueryListEvent) => setSystemPrefersDark(e.matches);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, [theme]);

  useEffect(() => {
    document.documentElement.classList.toggle('dark', resolvedDark);
  }, [resolvedDark]);

  // Detect sandbox mode (no fixed db — /api/meta doesn't exist) vs normal
  // single-DB mode, then fetch meta & tables only in the latter case.
  useEffect(() => {
    fetch('/api/meta')
      .then((res) => {
        if (res.status === 404) {
          setSandboxMode(true);
          setLoading(false);
          refreshSandboxDbs();
          return null;
        }
        return res;
      })
      .then((res) => {
        if (res) fetchMetaAndTables();
      })
      .catch(() => fetchMetaAndTables());

    // Always top-level /api/examples — not scoped per sandbox database.
    fetch('/api/examples')
      .then((res) => (res.ok ? res.json() : null))
      .then((body) => {
        if (body && body.ok) setExamplesList(body.data ?? []);
      })
      .catch(() => {});
  }, []);

  // Refetch when entering Info tab
  useEffect(() => {
    if (activeTab === 'info' && (!sandboxMode || activeDbId)) {
      fetchMetaAndTables();
    }
  }, [activeTab]);

  // Switching the active sandbox db is a full navigation: re-fetch this
  // db's meta/tables once its id becomes active.
  useEffect(() => {
    if (sandboxMode && activeDbId) {
      fetchMetaAndTables();
    }
  }, [activeDbId]);

  // Background fetch all schemas when in editor tab to inspect foreign keys
  useEffect(() => {
    if (activeTab === 'editor' && tables.length > 0) {
      tables.forEach(t => {
        if (!allSchemas[t.name]) {
          apiFetch(`/tables/${t.name}/schema`)
            .then(res => res.json())
            .then(body => {
              if (body.ok && body.data) {
                setAllSchemas(prev => ({ ...prev, [t.name]: body.data }));
              }
            })
            .catch(console.error);
        }
      });
    }
  }, [activeTab, tables, allSchemas]);

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
    setEditorMode('alter');
    setNewTableNameInput(selectedTable.name);
    setInlineAddRow(null);
    setEditingRowIndex(null);

    apiFetch(`/tables/${selectedTable.name}/schema`)
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

  // Load the seed plan when entering the Seed tab (or switching tables while on it)
  useEffect(() => {
    if (activeTab !== 'seed' || !selectedTable) return;

    setSeedPlan(null);
    setSeedPlanError(null);
    setSeedSelections({});
    setSeedOverrides({});
    setSeedPreviewRows(null);
    setSeedError(null);
    setSeedPlanLoading(true);
    setGeneratorPickerColumn(null);
    setRecentlyUsedGenerators([]);
    setSeedGeneratorSamples({});

    getSeedPlan(selectedTable.name)
      .then((plan) => {
        setSeedPlan(plan);
        const selections: Record<string, SeedColumnSelection> = {};
        plan.columns.forEach((col) => {
          if (!col.skip && col.generator) {
            selections[col.name] = { generator: col.generator, options: col.options || {} };
          }
        });
        setSeedSelections(selections);
      })
      .catch((err: any) => {
        setSeedPlanError(err.message || 'Failed to load seed plan');
      })
      .finally(() => setSeedPlanLoading(false));
  }, [activeTab, selectedTable?.name]);

  // Fetch rows
  useEffect(() => {
    if (!selectedTable) return;

    const offset = (page - 1) * pageSize;
    let url = `/tables/${selectedTable.name}/rows?limit=${pageSize}&offset=${offset}`;
    if (orderBy) {
      url += `&orderBy=${orderBy}&dir=${dir}`;
    }
    Object.entries(filters).forEach(([col, val]) => {
      if (val) {
        url += `&filter[${col}]=${encodeURIComponent(val)}`;
      }
    });

    setRowsLoading(true);
    apiFetch(url)
      .then(res => res.json())
      .then(body => {
        if (body.ok && body.data) {
          setRowsData(body.data);
        } else {
          console.error(body.error?.message);
        }
      })
      .catch(console.error)
      .finally(() => setRowsLoading(false));
  }, [selectedTable, page, pageSize, orderBy, dir, filters, refetchTrigger]);

  const toggleTheme = () => {
    setTheme((prev) => (prev === 'dark' ? 'light' : prev === 'light' ? 'system' : 'dark'));
  };

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const isModified = e.metaKey || e.ctrlKey;

      if (isModified && e.key === 'Enter') {
        if (activeTab === 'sql') {
          e.preventDefault();
          runQueryFromEditor();
        }
        return;
      }

      if (isModified && (e.key === 'j' || e.key === 'J')) {
        e.preventDefault();
        toggleTheme();
        return;
      }

      if (!isModified && e.key === '/') {
        const active = document.activeElement as HTMLElement | null;
        const isTyping =
          !!active &&
          (active.tagName === 'INPUT' ||
            active.tagName === 'TEXTAREA' ||
            active.isContentEditable ||
            !!active.closest('.cm-editor'));
        if (!isTyping) {
          e.preventDefault();
          tableSearchRef.current?.focus();
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [activeTab]);

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

  const showSandboxError = (message: string) => {
    showToast(message, 'error');
  };

  if (sandboxMode && sandboxManageOpen) {
    return (
      <SandboxManagePage
        dbs={sandboxDbs}
        activeDbId={activeDbId}
        onBack={() => setSandboxManageOpen(false)}
        onSwitch={switchActiveDb}
        onRename={handleSandboxRename}
        onDelete={handleSandboxDelete}
        onDownload={handleSandboxDownload}
        onUpload={handleSandboxUpload}
        onCreate={handleSandboxCreate}
        onError={showSandboxError}
        theme={theme}
        resolvedDark={resolvedDark}
        toggleTheme={toggleTheme}
      />
    );
  }

  if (sandboxMode && !activeDbId) {
    return (
      <SandboxEmptyState
        dbs={sandboxDbs}
        onUpload={handleSandboxUpload}
        onCreate={handleSandboxCreate}
        onSelect={switchActiveDb}
        onError={showSandboxError}
        theme={theme}
        resolvedDark={resolvedDark}
        toggleTheme={toggleTheme}
      />
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
      showToast('Copied to clipboard!', 'success');
    });
  };

  const refTableOptions = tables.filter(t => t.type === 'table');

  // Renders one repeatable foreign-key-entry row, shared by the Create
  // Table form and the Alter panel's "Add Foreign Key" card.
  const renderFkEntryRow = (
    draft: ForeignKeyDraft,
    localColumnOptions: string[],
    onChange: (next: ForeignKeyDraft) => void,
    onRemove: (() => void) | null,
    error?: string
  ) => {
    const refSchema = draft.refTable ? allSchemas[draft.refTable] : undefined;
    const covered = isRefColumnsCovered(refSchema, draft.refColumns);

    return (
      <div className="p-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/40 space-y-2">
        <div className="grid sm:grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <label className="text-[10px] uppercase font-semibold text-slate-400">Local column(s)</label>
            <select
              multiple
              value={draft.columns}
              onChange={(e) => {
                const selected = Array.from(e.target.selectedOptions).map(o => o.value);
                onChange({ ...draft, columns: selected });
              }}
              className="font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none min-h-16"
            >
              {localColumnOptions.map(c => (
                <option key={c} value={c}>{c}</option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1">
            <label className="text-[10px] uppercase font-semibold text-slate-400">References table</label>
            <select
              value={draft.refTable}
              onChange={(e) => onChange({ ...draft, refTable: e.target.value, refColumns: [] })}
              className="font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
            >
              <option value="">— select table —</option>
              {refTableOptions.map(t => (
                <option key={t.name} value={t.name}>{t.name}</option>
              ))}
            </select>
          </div>
        </div>

        <div className="grid sm:grid-cols-2 gap-3">
          <div className="flex flex-col gap-1">
            <label className="text-[10px] uppercase font-semibold text-slate-400">References column(s)</label>
            <select
              multiple
              disabled={!draft.refTable}
              value={draft.refColumns}
              onChange={(e) => {
                const selected = Array.from(e.target.selectedOptions).map(o => o.value);
                onChange({ ...draft, refColumns: selected });
              }}
              className="font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none min-h-16 disabled:opacity-50"
            >
              {(refSchema?.columns || []).map(c => {
                const inPk = refSchema!.primaryKey.some(p => p.toLowerCase() === c.name.toLowerCase());
                const inUnique = refSchema!.indexes.some(idx => idx.unique && idx.columns.some(ic => ic.toLowerCase() === c.name.toLowerCase()));
                return (
                  <option key={c.name} value={c.name} className={!inPk && !inUnique ? 'text-amber-500' : ''}>
                    {c.name}{!inPk && !inUnique ? ' (not PK/unique)' : ''}
                  </option>
                );
              })}
            </select>
            {draft.refTable && draft.refColumns.length > 0 && !covered && (
              <p className="text-[11px] text-amber-600 dark:text-amber-400">
                Hint: refColumns should be part of a primary key or unique constraint on the target table.
              </p>
            )}
            {draft.columns.length !== draft.refColumns.length && (draft.columns.length > 0 || draft.refColumns.length > 0) && (
              <p className="text-[11px] text-amber-600 dark:text-amber-400">
                Local column count ({draft.columns.length}) must match reference column count ({draft.refColumns.length}).
              </p>
            )}
          </div>
          <div className="grid grid-cols-2 gap-2">
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase font-semibold text-slate-400">On Delete</label>
              <select
                value={draft.onDelete}
                onChange={(e) => onChange({ ...draft, onDelete: e.target.value })}
                className="font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
              >
                {FK_ACTIONS.map(a => <option key={a} value={a}>{a}</option>)}
              </select>
            </div>
            <div className="flex flex-col gap-1">
              <label className="text-[10px] uppercase font-semibold text-slate-400">On Update</label>
              <select
                value={draft.onUpdate}
                onChange={(e) => onChange({ ...draft, onUpdate: e.target.value })}
                className="font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
              >
                {FK_ACTIONS.map(a => <option key={a} value={a}>{a}</option>)}
              </select>
            </div>
          </div>
        </div>

        <details className="text-xs">
          <summary className="cursor-pointer text-slate-500 select-none">Advanced (MATCH)</summary>
          <select
            value={draft.match}
            onChange={(e) => onChange({ ...draft, match: e.target.value })}
            className="mt-2 font-mono text-xs px-2 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
          >
            {FK_MATCH_MODES.map(m => <option key={m} value={m}>{m}</option>)}
          </select>
        </details>

        {error && (
          <div className="p-2 text-xs text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20 rounded-md">
            {error}
          </div>
        )}

        {onRemove && (
          <div className="flex justify-end">
            <button type="button" onClick={onRemove} className="text-red-500 hover:text-red-650 text-xs font-sans cursor-pointer">
              × Remove
            </button>
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="flex flex-col h-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 antialiased font-sans">
      {/* ============ HEADER ============ */}
      <header className="flex items-center justify-between px-4 h-12 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shrink-0">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <img src="/favicon.svg" alt="squad" className="w-6 h-6 rounded" />
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
          {sandboxMode && (
            <DbSwitcher
              activeDbId={activeDbId}
              dbs={sandboxDbs}
              onSwitch={switchActiveDb}
              onRename={handleSandboxRename}
              onDelete={handleSandboxDelete}
              onDownload={handleSandboxDownload}
              onUpload={handleSandboxUpload}
              onCreate={handleSandboxCreate}
              onError={showSandboxError}
              onOpenManage={() => setSandboxManageOpen(true)}
            />
          )}
        </div>
        <div className="flex items-center gap-2">
          <span className="font-mono text-xs text-slate-400 hidden md:inline">127.0.0.1:7071</span>
          <button
            onClick={toggleTheme}
            className="w-8 h-8 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center transition-colors"
            title={`Theme: ${theme}`}
          >
            {theme === 'system' ? (
              <Monitor className="w-4 h-4 text-slate-500" />
            ) : resolvedDark ? (
              <Moon className="w-4 h-4 text-slate-500" />
            ) : (
              <Sun className="w-4 h-4 text-amber-400" />
            )}
          </button>
        </div>
      </header>

      <div className="flex flex-1 min-h-0">
        {/* ============ SIDEBAR ============ */}
        <aside className="w-60 border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 flex flex-col shrink-0">
          <div className="p-2">
            <input
              ref={tableSearchRef}
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
            {tablesLoading && tables.length === 0 ? (
              <div className="px-2 space-y-1.5 animate-pulse">
                <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
                <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
                <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
                <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
              </div>
            ) : tables.length === 0 ? (
              <div className="flex flex-col items-center text-center gap-2 px-3 py-8 text-slate-400 dark:text-slate-600">
                <Database className="w-6 h-6" />
                <span className="text-xs">No tables yet</span>
              </div>
            ) : filteredTables.length === 0 ? (
              <div className="px-3 py-6 text-center text-xs text-slate-400">No tables match your search</div>
            ) : (
              filteredTables.map((t) => (
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
              ))
            )}
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
            {/* EMPTY DATABASE STATE */}
            {['data', 'schema', 'seed', 'export'].includes(activeTab) && tables.length === 0 && (
              <div className="flex-1 flex items-center justify-center p-8 h-full">
                <div className="max-w-sm text-center flex flex-col items-center gap-3">
                  <Database className="w-10 h-10 text-slate-300 dark:text-slate-700" />
                  <h2 className="font-semibold text-slate-900 dark:text-white">No tables in this database</h2>
                  <p className="text-sm text-slate-500 dark:text-slate-400">
                    This database is empty. Create a table to get started, or run raw SQL DDL in the SQL Editor.
                  </p>
                  {isWrite ? (
                    <div className="flex gap-2 mt-1">
                      <button
                        onClick={() => setActiveTab('editor')}
                        className="px-3 py-1.5 rounded-md text-xs font-medium bg-indigo-600 text-white hover:bg-indigo-500"
                      >
                        + Create New Table
                      </button>
                      <button
                        onClick={() => setActiveTab('sql')}
                        className="px-3 py-1.5 rounded-md text-xs font-medium border border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                      >
                        Open SQL Editor
                      </button>
                    </div>
                  ) : (
                    <p className="text-xs text-amber-600 dark:text-amber-400 mt-1">
                      Read-only mode — relaunch with <span className="font-mono">--write</span> to create tables.
                    </p>
                  )}
                </div>
              </div>
            )}

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
                      onClick={() => {
                        fetchMetaAndTables();
                        setRefetchTrigger((prev) => prev + 1);
                      }}
                      className="p-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-850"
                      title="Refresh — picks up rows/changes made outside this UI (e.g. via /rest/*)"
                    >
                      <RefreshCw className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={handleAddRowClick}
                      className={`px-2.5 py-1 rounded-md border border-slate-200 dark:border-slate-700 ${
                        isWrite ? 'hover:bg-slate-100 dark:hover:bg-slate-850 cursor-pointer' : 'opacity-50 cursor-not-allowed'
                      }`}
                      title={isWrite ? 'Add new row' : 'Write mode required'}
                      disabled={!isWrite}
                    >
                      + Row
                    </button>
                  </div>
                </div>

                {rowsLoading ? (
                  <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 flex-1 min-h-0 p-3 space-y-2 animate-pulse">
                    <div className="h-6 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                    <div className="h-5 bg-slate-100 dark:bg-slate-800 rounded" />
                  </div>
                ) : rowsData && rowsData.total === 0 && !inlineAddRow ? (
                  <div className="border border-slate-200 dark:border-slate-800 rounded-lg bg-white dark:bg-slate-900 flex-1 min-h-0 flex flex-col items-center justify-center text-center text-slate-400 dark:text-slate-600 gap-2">
                    <Database className="w-8 h-8" />
                    <p className="text-sm">This table has no rows</p>
                  </div>
                ) : (
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
                                      className="text-[10px] text-slate-400 hover:text-indigo-500 font-normal px-1 rounded border border-transparent hover:border-slate-200 dark:hover:border-slate-800 flex items-center gap-1"
                                    >
                                      <Search className="w-2.5 h-2.5" /> Filter
                                    </button>
                                  )}
                                </div>
                              </div>
                            </th>
                          );
                        })}
                        {isWrite && (
                          <th className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800 w-24 text-right">
                            Actions
                          </th>
                        )}
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
                      {/* Inline Add Row */}
                      {inlineAddRow && (
                        <tr className="bg-indigo-50/20 dark:bg-indigo-950/10">
                          {rowsData?.columns.map((col) => {
                            const colType = schema?.columns.find(c => c.name === col)?.type || '';
                            const isRowid = col === 'rowid';
                            const isReadOnly = isRowid || schema?.columns.find(c => c.name === col)?.generated !== null;
                            const isNumeric = ['integer', 'real', 'numeric'].includes(colType.toLowerCase());

                            return (
                              <td key={col} className="px-3 py-1.5">
                                <input
                                  type={isNumeric ? "number" : "text"}
                                  step="any"
                                  disabled={isReadOnly}
                                  value={inlineAddRow[col] ?? ''}
                                  placeholder={isReadOnly ? '(auto)' : ''}
                                  onChange={(e) => {
                                    setInlineAddRow(prev => ({ ...prev!, [col]: e.target.value }));
                                  }}
                                  className="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs w-full outline-none"
                                />
                              </td>
                            );
                          })}
                          <td className="px-3 py-1.5 space-x-2 text-right whitespace-nowrap">
                            <button
                              onClick={handleSaveAddRow}
                              title="Save"
                              className="text-emerald-600 dark:text-emerald-450 hover:bg-emerald-50 dark:hover:bg-emerald-500/10 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                            >
                              <Save className="w-4 h-4" />
                            </button>
                            <button
                              onClick={() => setInlineAddRow(null)}
                              title="Cancel"
                              className="text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-850 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                            >
                              <X className="w-4 h-4" />
                            </button>
                          </td>
                        </tr>
                      )}

                      {rowsData?.rows.map((row, rIdx) => {
                        const isEditing = editingRowIndex === rIdx;
                        return (
                          <tr key={rIdx} className="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                            {row.map((val, cIdx) => {
                              const colName = rowsData.columns[cIdx];
                              const colType = schema?.columns.find(c => c.name === colName)?.type || '';
                              const isBlob = colType.toLowerCase() === 'blob';
                              const isRowid = colName === 'rowid';
                              const isReadOnly = isRowid || schema?.columns.find(c => c.name === colName)?.generated !== null;
                              const isNumeric = ['integer', 'real', 'numeric'].includes(colType.toLowerCase());

                              if (isEditing) {
                                return (
                                  <td key={cIdx} className="px-3 py-1.5">
                                    <input
                                      type={isNumeric ? "number" : "text"}
                                      step="any"
                                      disabled={isReadOnly}
                                      value={editingRowValues[colName] ?? ''}
                                      onChange={(e) => {
                                        setEditingRowValues(prev => ({ ...prev, [colName]: e.target.value }));
                                      }}
                                      className="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs w-full outline-none"
                                    />
                                  </td>
                                );
                              }

                              if (isBlob && val !== null) {
                                const hex = String(val);
                                const bytesCount = Math.ceil(hex.length / 2);
                                const mediaType = sniffHex(hex);
                                const openModal = () => {
                                  setBlobModalView(mediaType === 'unknown' ? 'hex' : 'preview');
                                  setBlobModal({ column: colName, hex, type: mediaType });
                                };
                                if (mediaType !== 'unknown') {
                                  return (
                                    <td key={cIdx} className="px-3 py-1.5">
                                      <button
                                        onClick={openModal}
                                        title={`BLOB (${bytesCount} bytes)`}
                                        className="inline-flex items-center justify-center w-9 h-9 rounded border border-slate-200 dark:border-slate-750 bg-slate-50 dark:bg-slate-800/60 overflow-hidden hover:border-indigo-400 dark:hover:border-indigo-500 transition-colors"
                                      >
                                        {mediaType === 'wav' ? (
                                          <AudioLines className="w-4 h-4 text-indigo-500 dark:text-indigo-400" />
                                        ) : (
                                          <img
                                            src={dataUriFromHex(hex, mediaType)}
                                            alt={colName}
                                            className="max-w-full max-h-full object-contain"
                                          />
                                        )}
                                      </button>
                                    </td>
                                  );
                                }
                                return (
                                  <td key={cIdx} className="px-3 py-1.5">
                                    <button
                                      onClick={openModal}
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
                            {isWrite && (
                              <td className="px-3 py-1.5 space-x-2 text-right whitespace-nowrap">
                                {isEditing ? (
                                  <>
                                    <button
                                      onClick={() => handleSaveEditRow(row)}
                                      title="Save"
                                      className="text-emerald-600 dark:text-emerald-450 hover:bg-emerald-50 dark:hover:bg-emerald-500/10 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                                    >
                                      <Save className="w-4 h-4" />
                                    </button>
                                    <button
                                      onClick={() => setEditingRowIndex(null)}
                                      title="Cancel"
                                      className="text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-850 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                                    >
                                      <X className="w-4 h-4" />
                                    </button>
                                  </>
                                ) : (
                                  <>
                                    <button
                                      onClick={() => {
                                        setEditingRowIndex(rIdx);
                                        const vals: Record<string, any> = {};
                                        rowsData.columns.forEach((c, i) => {
                                          vals[c] = row[i];
                                        });
                                        setEditingRowValues(vals);
                                      }}
                                      title="Edit"
                                      className="text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                                    >
                                      <Edit2 className="w-4 h-4" />
                                    </button>
                                    <button
                                      onClick={() => handleDeleteRowClick(row, rIdx)}
                                      title="Delete"
                                      className="text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                                    >
                                      <Trash2 className="w-4 h-4" />
                                    </button>
                                  </>
                                )}
                              </td>
                            )}
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
                )}

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
                        <div className="flex items-center gap-2">
                          {examplesList && (
                            <button
                              onClick={() => setExamplesPickerOpen(true)}
                              className="px-2.5 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 hover:bg-slate-100 dark:hover:bg-slate-800 font-medium text-xs flex items-center gap-1.5 cursor-pointer text-slate-700 dark:text-slate-300"
                              title="Insert an example data model"
                            >
                              <LibraryBig className="w-3.5 h-3.5" />
                              Examples
                            </button>
                          )}
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
                      <SqlEditor
                        value={sqlValue}
                        onChange={setSqlValue}
                        onRun={handleExecuteQuery}
                        theme={resolvedDark ? 'dark' : 'light'}
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
              <section className="space-y-6 max-w-4xl">
                {!isWrite && (
                  <div className="rounded-lg border border-amber-300 dark:border-amber-500/30 bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-400 text-sm px-4 py-2.5">
                    Read-only mode — the table editor is disabled. Relaunch with <span className="font-mono">--write</span> to enable.
                  </div>
                )}

                {/* Mode Switcher */}
                <div className="flex items-center gap-2 border-b border-slate-200 dark:border-slate-800 pb-3">
                  <button
                    onClick={() => setEditorMode('create')}
                    className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                      editorMode === 'create'
                        ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-650 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-500/30'
                        : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                    }`}
                  >
                    + Create New Table
                  </button>
                  {selectedTable && (
                    <button
                      onClick={() => setEditorMode('alter')}
                      className={`px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                        editorMode === 'alter'
                          ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-650 dark:text-indigo-400 border border-indigo-200 dark:border-indigo-500/30'
                          : 'text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-800'
                      }`}
                    >
                      <span className="flex items-center gap-1">
                        <Edit2 className="w-3.5 h-3.5" /> Alter Table "{selectedTable.name}"
                      </span>
                    </button>
                  )}
                </div>

                {editorMode === 'create' ? (
                  /* CREATE TABLE FORM */
                  <form onSubmit={handleCreateTableSubmit} className={`space-y-4 ${!isWrite ? 'opacity-60 pointer-events-none' : ''}`}>
                    <div className="flex flex-col gap-1 max-w-sm">
                      <label className="text-xs font-semibold text-slate-500 dark:text-slate-400">Table Name</label>
                      <input
                        type="text"
                        placeholder="users"
                        required
                        value={newTableName}
                        onChange={(e) => setNewTableName(e.target.value.replace(/[^a-zA-Z0-9_]/g, ''))}
                        className="font-mono text-sm px-3 py-2 rounded-md border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 text-slate-900 dark:text-white outline-none focus:border-indigo-500"
                      />
                    </div>

                    <div className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id="composite-pk-toggle"
                        checked={isCompositePk}
                        onChange={(e) => {
                          setIsCompositePk(e.target.checked);
                          if (!e.target.checked) setCompositePkColumns([]);
                        }}
                        className="rounded border-slate-300 dark:border-slate-700"
                      />
                      <label htmlFor="composite-pk-toggle" className="text-sm text-slate-650 dark:text-slate-300 cursor-pointer">
                        Composite Primary Key Mode
                      </label>
                    </div>

                    {isCompositePk && (
                      <div className="p-3 rounded-lg border border-slate-200 dark:border-slate-800 bg-slate-50/50 dark:bg-slate-900/40 text-sm space-y-2">
                        <div className="font-medium text-slate-700 dark:text-slate-300">Composite Primary Key Columns (in order):</div>
                        <div className="flex flex-wrap gap-2">
                          {newTableColumns.map(col => {
                            if (!col.name.trim()) return null;
                            const isSelected = compositePkColumns.includes(col.name);
                            return (
                              <button
                                key={col.name}
                                type="button"
                                onClick={() => {
                                  if (isSelected) {
                                    setCompositePkColumns(prev => prev.filter(c => c !== col.name));
                                  } else {
                                    setCompositePkColumns(prev => [...prev, col.name]);
                                  }
                                }}
                                className={`px-2.5 py-1 rounded text-xs font-mono border transition-all ${
                                  isSelected
                                    ? 'bg-indigo-600 text-white border-indigo-650'
                                    : 'bg-white dark:bg-slate-900 border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-400'
                                }`}
                              >
                                {col.name} {isSelected && `(PK #${compositePkColumns.indexOf(col.name) + 1})`}
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}

                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 shadow-sm">
                      <table className="w-full text-sm">
                        <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                          <tr>
                            <th className="px-3 py-2 font-medium">Name</th>
                            <th className="px-3 py-2 font-medium">Type</th>
                            {!isCompositePk && <th className="px-3 py-2 font-medium text-center">PK</th>}
                            <th className="px-3 py-2 font-medium text-center">Not Null</th>
                            <th className="px-3 py-2 font-medium text-center">Unique</th>
                            <th className="px-3 py-2 font-medium">Default Expr</th>
                            <th className="px-3 py-2 text-center w-12"></th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono">
                          {newTableColumns.map((col, idx) => (
                            <tr key={idx}>
                              <td className="px-3 py-1.5">
                                <input
                                  type="text"
                                  placeholder="column_name"
                                  required
                                  value={col.name}
                                  onChange={(e) => {
                                    const updated = [...newTableColumns];
                                    updated[idx].name = e.target.value.replace(/[^a-zA-Z0-9_]/g, '');
                                    setNewTableColumns(updated);
                                  }}
                                  className="bg-transparent w-full outline-none border-b border-transparent focus:border-indigo-400 py-0.5 text-slate-900 dark:text-white"
                                />
                              </td>
                              <td className="px-3 py-1.5">
                                <input
                                  type="text"
                                  list="type-affinities"
                                  required
                                  value={col.type}
                                  onChange={(e) => {
                                    const updated = [...newTableColumns];
                                    updated[idx].type = e.target.value.toUpperCase();
                                    setNewTableColumns(updated);
                                  }}
                                  className="bg-transparent w-full outline-none border-b border-transparent focus:border-indigo-400 py-0.5 text-slate-900 dark:text-white"
                                />
                              </td>
                              {!isCompositePk && (
                                <td className="px-3 py-1.5 text-center">
                                  <input
                                    type="checkbox"
                                    checked={col.pk}
                                    onChange={(e) => {
                                      const updated = newTableColumns.map((c, i) => ({
                                        ...c,
                                        pk: i === idx ? e.target.checked : false
                                      }));
                                      setNewTableColumns(updated);
                                    }}
                                  />
                                </td>
                              )}
                              <td className="px-3 py-1.5 text-center">
                                <input
                                  type="checkbox"
                                  checked={col.notnull}
                                  onChange={(e) => {
                                    const updated = [...newTableColumns];
                                    updated[idx].notnull = e.target.checked;
                                    setNewTableColumns(updated);
                                  }}
                                />
                              </td>
                              <td className="px-3 py-1.5 text-center">
                                <input
                                  type="checkbox"
                                  checked={col.unique}
                                  onChange={(e) => {
                                    const updated = [...newTableColumns];
                                    updated[idx].unique = e.target.checked;
                                    setNewTableColumns(updated);
                                  }}
                                />
                              </td>
                              <td className="px-3 py-1.5">
                                <input
                                  type="text"
                                  placeholder="NULL, 0, 'active'"
                                  value={col.defaultVal}
                                  onChange={(e) => {
                                    const updated = [...newTableColumns];
                                    updated[idx].defaultVal = e.target.value;
                                    setNewTableColumns(updated);
                                  }}
                                  className="bg-transparent w-full outline-none border-b border-transparent focus:border-indigo-400 py-0.5 text-slate-900 dark:text-white text-xs"
                                />
                              </td>
                              <td className="px-3 py-1.5 text-center">
                                <button
                                  type="button"
                                  disabled={newTableColumns.length <= 1}
                                  onClick={() => {
                                    setNewTableColumns(prev => prev.filter((_, i) => i !== idx));
                                  }}
                                  className="text-red-500 hover:text-red-650 disabled:opacity-30 disabled:cursor-not-allowed font-sans"
                                >
                                  ✕
                                </button>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>

                    {/* Foreign Keys Section */}
                    <div className="space-y-2">
                      <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Foreign Keys</h4>
                      {newTableForeignKeys.map((fk, idx) => (
                        <div key={idx}>
                          {renderFkEntryRow(
                            fk,
                            newTableColumns.map(c => c.name).filter(Boolean),
                            (next) => {
                              setNewTableForeignKeys(prev => prev.map((f, i) => (i === idx ? next : f)));
                            },
                            () => setNewTableForeignKeys(prev => prev.filter((_, i) => i !== idx)),
                            createFkErrors[idx]
                          )}
                        </div>
                      ))}
                      <button
                        type="button"
                        onClick={() => setNewTableForeignKeys(prev => [...prev, emptyFkDraft()])}
                        className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 text-xs font-semibold hover:bg-slate-50 dark:hover:bg-slate-800 cursor-pointer"
                      >
                        + Add Foreign Key
                      </button>
                    </div>

                    <div className="flex items-center gap-3">
                      <button
                        type="button"
                        onClick={() => {
                          setNewTableColumns(prev => [...prev, { name: '', type: 'TEXT', pk: false, notnull: false, unique: false, defaultVal: '' }]);
                        }}
                        className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 text-xs font-semibold hover:bg-slate-50 dark:hover:bg-slate-800 cursor-pointer"
                      >
                        + Add Column
                      </button>

                      <button
                        type="submit"
                        className="px-4 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold cursor-pointer shadow"
                      >
                        Create Table
                      </button>
                    </div>

                    {createTableError && (
                      <div className="p-3 text-sm text-red-600 dark:text-red-400 bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20 rounded-md">
                        {createTableError}
                      </div>
                    )}
                  </form>
                ) : (
                  /* ALTER TABLE PANEL */
                  selectedTable && (
                    <div className={`space-y-6 ${!isWrite ? 'opacity-60 pointer-events-none' : ''}`}>
                      
                      {/* Rename Table Card */}
                      <div className="p-4 rounded-xl border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm space-y-3">
                        <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Rename Table</h4>
                        <div className="flex items-center gap-3 max-w-md">
                          <input
                            type="text"
                            value={newTableNameInput}
                            onChange={(e) => setNewTableNameInput(e.target.value.replace(/[^a-zA-Z0-9_]/g, ''))}
                            className="font-mono text-sm px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-white outline-none flex-1 focus:border-indigo-500"
                          />
                          <button
                            onClick={handleRenameTableSubmit}
                            disabled={newTableNameInput === selectedTable.name || !newTableNameInput.trim()}
                            className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shadow"
                          >
                            Rename
                          </button>
                        </div>
                      </div>

                      {/* Add Column Card */}
                      <div className="p-4 rounded-xl border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm space-y-3">
                        <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Add Column</h4>
                        <div className="grid sm:grid-cols-4 gap-3">
                          <div className="flex flex-col gap-1">
                            <label className="text-[10px] uppercase font-semibold text-slate-400">Name</label>
                            <input
                              type="text"
                              value={addColName}
                              onChange={(e) => setAddColName(e.target.value.replace(/[^a-zA-Z0-9_]/g, ''))}
                              placeholder="new_col"
                              className="font-mono text-sm px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
                            />
                          </div>
                          <div className="flex flex-col gap-1">
                            <label className="text-[10px] uppercase font-semibold text-slate-400">Type</label>
                            <input
                              type="text"
                              list="type-affinities"
                              value={addColType}
                              onChange={(e) => setAddColType(e.target.value.toUpperCase())}
                              placeholder="TEXT"
                              className="font-mono text-sm px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
                            />
                          </div>
                          <div className="flex flex-col gap-1">
                            <label className="text-[10px] uppercase font-semibold text-slate-400">Default Value</label>
                            <input
                              type="text"
                              value={addColDefault}
                              onChange={(e) => setAddColDefault(e.target.value)}
                              placeholder="e.g. 'active', 0"
                              className="font-mono text-xs px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
                            />
                          </div>
                          <div className="flex items-center gap-2 pt-5">
                            <input
                              type="checkbox"
                              id="add-col-notnull"
                              checked={addColNotNull}
                              onChange={(e) => setAddColNotNull(e.target.checked)}
                              className="rounded border-slate-300 dark:border-slate-700"
                            />
                            <label htmlFor="add-col-notnull" className="text-xs text-slate-650 dark:text-slate-300 cursor-pointer">
                              Not Null
                            </label>
                          </div>
                        </div>
                        <button
                          onClick={handleAddColumnSubmit}
                          disabled={!addColName.trim()}
                          className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shadow mt-2"
                        >
                          Add Column
                        </button>
                      </div>

                      {/* Columns Alter List */}
                      <div className="p-4 rounded-xl border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm space-y-3">
                        <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Existing Columns</h4>
                        <div className="border border-slate-100 dark:border-slate-800 rounded-lg overflow-hidden">
                          <table className="w-full text-sm">
                            <thead className="bg-slate-50 dark:bg-slate-800/40 text-slate-500 text-left">
                              <tr>
                                <th className="px-3 py-2 font-medium">Name</th>
                                <th className="px-3 py-2 font-medium">Type</th>
                                <th className="px-3 py-2 font-medium">PK</th>
                                <th className="px-3 py-2 font-medium">Not Null</th>
                                <th className="px-3 py-2 font-medium">Default</th>
                                <th className="px-3 py-2 text-right">Actions</th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono text-xs text-slate-700 dark:text-slate-300">
                              {schema?.columns.map(col => {
                                const isRenaming = renamingColumn[col.name] !== undefined;
                                return (
                                  <tr key={col.name} className="hover:bg-slate-50 dark:hover:bg-slate-900/30">
                                    <td className="px-3 py-2 font-semibold">
                                      {isRenaming ? (
                                        <input
                                          type="text"
                                          value={renamingColumn[col.name]}
                                          onChange={(e) => {
                                            const val = e.target.value.replace(/[^a-zA-Z0-9_]/g, '');
                                            setRenamingColumn(prev => ({ ...prev, [col.name]: val }));
                                          }}
                                          className="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs w-36 outline-none"
                                        />
                                      ) : (
                                        col.name
                                      )}
                                    </td>
                                    <td className="px-3 py-2 text-slate-500">{col.type}</td>
                                    <td className="px-3 py-2">{col.pk > 0 ? `☑ (idx: ${col.pk})` : '☐'}</td>
                                    <td className="px-3 py-2">{col.notnull ? '☑' : '☐'}</td>
                                    <td className="px-3 py-2 text-slate-500">{col.defaultVal || '—'}</td>
                                    <td className="px-3 py-2 text-right space-x-2 font-sans whitespace-nowrap">
                                      {isRenaming ? (
                                        <>
                                          <button
                                            onClick={() => handleRenameColumnSubmit(col.name)}
                                            title="Save"
                                            className="text-emerald-600 dark:text-emerald-450 hover:bg-emerald-50 dark:hover:bg-emerald-500/10 p-1.5 rounded transition-colors inline-flex items-center justify-center cursor-pointer"
                                          >
                                            <Save className="w-4 h-4" />
                                          </button>
                                          <button
                                            onClick={() => {
                                              setRenamingColumn(prev => {
                                                const copy = { ...prev };
                                                delete copy[col.name];
                                                return copy;
                                              });
                                            }}
                                            title="Cancel"
                                            className="text-slate-500 hover:bg-slate-100 dark:hover:bg-slate-850 p-1.5 rounded transition-colors inline-flex items-center justify-center cursor-pointer"
                                          >
                                            <X className="w-4 h-4" />
                                          </button>
                                        </>
                                      ) : (
                                        <>
                                          <button
                                            onClick={() => setRenamingColumn(prev => ({ ...prev, [col.name]: col.name }))}
                                            title="Rename Column"
                                            className="text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 p-1.5 rounded transition-colors inline-flex items-center justify-center cursor-pointer"
                                          >
                                            <Edit2 className="w-4 h-4" />
                                          </button>
                                          <button
                                            onClick={() => handleDropColumnClick(col.name)}
                                            disabled={schema.columns.length <= 1}
                                            title="Drop Column"
                                            className="text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 p-1.5 rounded transition-colors inline-flex items-center justify-center disabled:opacity-30 disabled:cursor-not-allowed cursor-pointer"
                                          >
                                            <Trash2 className="w-4 h-4" />
                                          </button>
                                        </>
                                      )}
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        </div>
                      </div>

                      {/* Foreign Keys Card */}
                      <div className="p-4 rounded-xl border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm space-y-3">
                        <h4 className="text-sm font-semibold text-slate-900 dark:text-white">Foreign Keys</h4>
                        <div className="border border-slate-100 dark:border-slate-800 rounded-lg overflow-hidden">
                          <table className="w-full text-sm">
                            <thead className="bg-slate-50 dark:bg-slate-800/40 text-slate-500 text-left">
                              <tr>
                                <th className="px-3 py-2 font-medium">Column</th>
                                <th className="px-3 py-2 font-medium">References</th>
                                <th className="px-3 py-2 font-medium">On Delete / On Update</th>
                                <th className="px-3 py-2 text-right">Actions</th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-100 dark:divide-slate-800 font-mono text-xs text-slate-700 dark:text-slate-300">
                              {(!schema?.foreignKeys || schema.foreignKeys.length === 0) ? (
                                <tr>
                                  <td colSpan={4} className="px-3 py-3 text-slate-400 italic font-sans">
                                    No foreign keys defined.
                                  </td>
                                </tr>
                              ) : (
                                schema.foreignKeys.map((fk, idx) => (
                                  <tr key={idx} className="hover:bg-slate-50 dark:hover:bg-slate-900/30">
                                    <td className="px-3 py-2 font-semibold">{fk.from}</td>
                                    <td className="px-3 py-2 text-indigo-650 dark:text-indigo-400">{fk.table}.{fk.to}</td>
                                    <td className="px-3 py-2 text-slate-500">{fk.onDelete} / {fk.onUpdate}</td>
                                    <td className="px-3 py-2 text-right font-sans">
                                      <button
                                        onClick={() => handleDropForeignKeyClick(fk)}
                                        title="Drop Foreign Key"
                                        className="text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 p-1.5 rounded transition-colors inline-flex items-center justify-center cursor-pointer"
                                      >
                                        <Trash2 className="w-4 h-4" />
                                      </button>
                                    </td>
                                  </tr>
                                ))
                              )}
                            </tbody>
                          </table>
                        </div>

                        <h5 className="text-xs font-semibold text-slate-500 pt-2">Add Foreign Key</h5>
                        {renderFkEntryRow(
                          addFk,
                          (schema?.columns || []).map(c => c.name),
                          setAddFk,
                          null,
                          addFkError || undefined
                        )}
                        <button
                          onClick={handleAddForeignKeySubmit}
                          disabled={addFk.columns.length === 0 || addFk.refColumns.length === 0 || !addFk.refTable}
                          className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer shadow"
                        >
                          Add Foreign Key
                        </button>
                      </div>

                      {/* Danger Zone */}
                      <div className="p-4 rounded-xl border border-red-200 dark:border-red-900/30 bg-red-50/20 dark:bg-red-950/10 space-y-3">
                        <h4 className="text-sm font-semibold text-red-600 dark:text-red-400">Danger Zone</h4>
                        <p className="text-xs text-slate-400">
                          Dropping a table is permanent and will delete all data inside it.
                        </p>
                        {getFkReferencingTables(selectedTable.name).length > 0 && (
                          <div className="text-xs text-amber-600 dark:text-amber-400 font-medium flex items-center gap-1.5">
                            <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0" />
                            <span>Warning: Other tables reference this table via foreign keys: {getFkReferencingTables(selectedTable.name).join(', ')}.</span>
                          </div>
                        )}
                        <button
                          onClick={handleDropTableClick}
                          className="px-4 py-2 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
                        >
                          Drop Table
                        </button>
                      </div>

                    </div>
                  )
                )}
              </section>
            )}

            {/* Datalist for SQL Type Affinities */}
            <datalist id="type-affinities">
              <option value="INTEGER" />
              <option value="TEXT" />
              <option value="REAL" />
              <option value="BLOB" />
              <option value="NUMERIC" />
            </datalist>

            {/* SEED PANEL */}
            {activeTab === 'seed' && selectedTable && (
              <section className="space-y-4">
                <h3 className="font-semibold text-slate-900 dark:text-white">
                  Seed <span className="font-mono text-indigo-500">{selectedTable.name}</span> with fake data
                </h3>
                <p className="text-sm text-slate-500 -mt-2">Generators auto-suggested from column name &amp; type.</p>

                {!isWrite && (
                  <div className="rounded-lg border border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-500/10 text-amber-700 dark:text-amber-300 text-sm px-4 py-2.5 max-w-4xl">
                    Write mode is required to seed data. Relaunch with <code className="font-mono">--write</code>.
                  </div>
                )}

                {seedPlanLoading && <p className="text-sm text-slate-400">Loading seed plan…</p>}
                {seedPlanError && <p className="text-sm text-rose-500">{seedPlanError}</p>}

                {seedPlan && (
                  <>
                    <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 max-w-4xl">
                      <table className="w-full text-sm">
                        <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                          <tr>
                            <th className="px-3 py-2 font-medium">Column</th>
                            <th className="px-3 py-2 font-medium">Generator</th>
                            <th className="px-3 py-2 font-medium">Options</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-xs">
                          {seedPlan.columns.map((col) => {
                            const overridden = !!seedOverrides[col.name];
                            const active = !col.skip || overridden;
                            const sel = seedSelections[col.name];

                            if (col.skip && !overridden) {
                              return (
                                <tr key={col.name} className="opacity-50">
                                  <td className="px-3 py-2 font-mono">{col.name}</td>
                                  <td className="px-3 py-2 text-slate-400 italic" colSpan={2}>
                                    skipped — {col.reason}{' '}
                                    <button
                                      onClick={() => toggleSeedOverride(col)}
                                      className="ml-2 not-italic underline text-indigo-500 hover:text-indigo-600 cursor-pointer"
                                    >
                                      Override
                                    </button>
                                  </td>
                                </tr>
                              );
                            }

                            return (
                              <tr key={col.name}>
                                <td className="px-3 py-2 font-mono align-top">
                                  {col.name}
                                  {col.skip && (
                                    <button
                                      onClick={() => toggleSeedOverride(col)}
                                      className="block mt-1 not-italic underline text-slate-400 hover:text-indigo-500 cursor-pointer"
                                    >
                                      Skip again
                                    </button>
                                  )}
                                </td>
                                <td className="px-3 py-2 align-top">
                                  <button
                                    type="button"
                                    onClick={() => setGeneratorPickerColumn(col.name)}
                                    disabled={!isWrite || !active}
                                    className="px-2 py-1 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none text-left disabled:opacity-50 disabled:cursor-not-allowed hover:border-indigo-400 flex flex-col"
                                  >
                                    <span>{sel?.generator || '—'}</span>
                                    {sel?.generator && seedGeneratorSamples[`${col.name}:${sel.generator}`] && (
                                      <span className="font-normal text-slate-400 truncate max-w-[10rem]">
                                        → {seedGeneratorSamples[`${col.name}:${sel.generator}`]}
                                      </span>
                                    )}
                                  </button>
                                </td>
                                <td className="px-3 py-2 align-top">
                                  {sel?.generator === 'foreignKey' || sel?.generator === 'enumFromColumn' ? (
                                    <span className="text-slate-400">
                                      {sel.options?.table}.{sel.options?.column}
                                    </span>
                                  ) : (
                                    <GeneratorOptionsForm
                                      schema={generatorMetaByName(sel?.generator || '')?.optionsSchema || []}
                                      values={sel?.options || {}}
                                      onChange={(key, value) => updateSeedOption(col.name, key, value)}
                                      siblingColumns={seedPlan.columns.map((c) => c.name)}
                                      catalog={seedPlan.generatorCatalog}
                                      affinity={sqliteAffinity(col.type)}
                                    />
                                  )}
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>

                    <div className="flex items-center gap-3 flex-wrap">
                      <label className="text-sm flex items-center gap-2">
                        Rows
                        <input
                          type="number"
                          min={1}
                          max={100000}
                          value={seedCount}
                          onChange={(e) => handleSeedCountChange(e.target.value)}
                          disabled={!isWrite}
                          className="font-mono w-28 px-2 py-1 rounded border border-slate-300 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-white outline-none"
                        />
                      </label>
                      {seedCount >= 100000 && (
                        <span className="text-xs text-amber-500">clamped to 100,000</span>
                      )}
                      <button
                        onClick={handleSeedPreview}
                        disabled={!isWrite || seedPreviewLoading}
                        className={`px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-sm ${
                          isWrite ? 'hover:bg-slate-100 dark:hover:bg-slate-850 cursor-pointer' : 'opacity-50 cursor-not-allowed'
                        }`}
                      >
                        {seedPreviewLoading ? 'Previewing…' : 'Preview 5 rows'}
                      </button>
                      <button
                        onClick={handleSeedInsert}
                        disabled={!isWrite || seedInsertLoading}
                        title={isWrite ? 'Insert seeded rows' : 'Write mode required'}
                        className={`px-3 py-1.5 rounded-md bg-indigo-600 text-white text-sm ${
                          isWrite && !seedInsertLoading ? 'hover:bg-indigo-700 cursor-pointer' : 'opacity-50 cursor-not-allowed'
                        }`}
                      >
                        {seedInsertLoading ? 'Inserting…' : 'Insert'}
                      </button>
                    </div>

                    {seedError && (
                      <p className="text-sm text-rose-500 max-w-4xl">{seedError}</p>
                    )}

                    {seedPreviewRows && (
                      <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto bg-white dark:bg-slate-900 max-w-4xl">
                        <table className="w-full text-xs font-mono">
                          <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
                            <tr>
                              {Object.keys(seedPreviewRows[0] || {}).map((col) => (
                                <th key={col} className="px-3 py-2 font-medium">{col}</th>
                              ))}
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                            {seedPreviewRows.map((row, i) => (
                              <tr key={i}>
                                {Object.keys(seedPreviewRows[0] || {}).map((col) => (
                                  <td key={col} className="px-3 py-2 text-slate-700 dark:text-slate-300">
                                    {row[col] === null || row[col] === undefined ? (
                                      <span className="text-slate-400 italic">NULL</span>
                                    ) : (
                                      String(row[col])
                                    )}
                                  </td>
                                ))}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </>
                )}
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
                      let url = `/tables/${encodeURIComponent(selectedTable.name)}/export?format=csv`;
                      if (applyFilterSort && (orderBy || Object.values(filters).some(v => v !== ''))) {
                        url += `&filtered=true`;
                        if (orderBy) url += `&orderBy=${encodeURIComponent(orderBy)}`;
                        if (dir) url += `&dir=${encodeURIComponent(dir)}`;
                        Object.entries(filters).forEach(([col, val]) => {
                          if (val !== '') url += `&filter[${encodeURIComponent(col)}]=${encodeURIComponent(val)}`;
                        });
                      }
                      window.location.href = apiUrl(url);
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'csv'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <FileSpreadsheet className="w-6 h-6 text-indigo-500 mb-1" />
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
                      let url = `/tables/${encodeURIComponent(selectedTable.name)}/export?format=json`;
                      if (applyFilterSort && (orderBy || Object.values(filters).some(v => v !== ''))) {
                        url += `&filtered=true`;
                        if (orderBy) url += `&orderBy=${encodeURIComponent(orderBy)}`;
                        if (dir) url += `&dir=${encodeURIComponent(dir)}`;
                        Object.entries(filters).forEach(([col, val]) => {
                          if (val !== '') url += `&filter[${encodeURIComponent(col)}]=${encodeURIComponent(val)}`;
                        });
                      }
                      window.location.href = apiUrl(url);
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'json'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <Braces className="w-6 h-6 text-indigo-500 mb-1" />
                    <div className="font-medium text-slate-900 dark:text-white">JSON</div>
                    <div className="text-xs text-slate-400">Array of objects</div>
                  </button>

                  {/* SQL Card */}
                  <button
                    onClick={() => {
                      setSelectedExportFormat('sql');
                      let url = `/tables/${encodeURIComponent(selectedTable.name)}/export?format=sql`;
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
                      window.location.href = apiUrl(url);
                    }}
                    className={`p-4 rounded-lg border text-left transition-all cursor-pointer ${
                      selectedExportFormat === 'sql'
                        ? 'border-indigo-600 dark:border-indigo-500 bg-indigo-50/30 dark:bg-indigo-950/20 shadow-sm font-medium'
                        : 'border-slate-205 dark:border-slate-800 hover:border-indigo-400 dark:hover:border-indigo-500/50'
                    }`}
                  >
                    <Database className="w-6 h-6 text-indigo-500 mb-1" />
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
              <RestTab
                selectedTable={selectedTable}
                onToast={(message, type) => showToast(message, type)}
              />
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
                    <RefreshCw className={`w-3.5 h-3.5 ${infoLoading ? 'animate-spin' : ''}`} />
                    <span>{infoLoading ? 'Refreshing...' : 'Refresh'}</span>
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

      {/* BLOB HEX/PREVIEW VIEWER MODAL */}
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

            {blobModal.type !== 'unknown' && (
              <div className="flex items-center gap-1 px-4 pt-3">
                <button
                  onClick={() => setBlobModalView('preview')}
                  className={`px-2.5 py-1 rounded text-xs font-medium ${
                    blobModalView === 'preview'
                      ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400'
                      : 'text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60'
                  }`}
                >
                  Preview
                </button>
                <button
                  onClick={() => setBlobModalView('hex')}
                  className={`px-2.5 py-1 rounded text-xs font-medium ${
                    blobModalView === 'hex'
                      ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400'
                      : 'text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800/60'
                  }`}
                >
                  Raw hex
                </button>
              </div>
            )}

            <div className="p-4">
              {blobModal.type !== 'unknown' && blobModalView === 'preview' ? (
                <div
                  className="flex items-center justify-center rounded-lg border border-slate-200 dark:border-slate-800 p-4 min-h-48"
                  style={
                    blobModal.type !== 'wav'
                      ? {
                          backgroundImage:
                            'repeating-conic-gradient(#cbd5e1 0% 25%, #f8fafc 0% 50%)',
                          backgroundSize: '16px 16px',
                        }
                      : undefined
                  }
                >
                  {blobModal.type === 'wav' ? (
                    <audio controls src={dataUriFromHex(blobModal.hex, blobModal.type)} className="w-full" />
                  ) : (
                    <img
                      src={dataUriFromHex(blobModal.hex, blobModal.type)}
                      alt={blobModal.column}
                      className="max-w-full max-h-96 object-contain"
                    />
                  )}
                </div>
              ) : (
                <pre className="font-mono text-xs bg-slate-900 text-slate-100 rounded-lg p-4 overflow-auto border border-slate-800 max-h-96 whitespace-pre">
                  {formatHexDump(blobModal.hex)}
                </pre>
              )}
              <div className="mt-2 flex items-center justify-between">
                <p className="text-xs text-slate-400">
                  {Math.ceil(blobModal.hex.length / 2).toLocaleString()} bytes
                </p>
                <button
                  onClick={() => downloadHex(blobModal.hex, blobModal.column, blobModal.type)}
                  className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800/60 border border-slate-200 dark:border-slate-750"
                >
                  <Download className="w-3.5 h-3.5" />
                  Download
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* GENERATOR PICKER MODAL */}
      {generatorPickerColumn && seedPlan && (() => {
        const col = seedPlan.columns.find((c) => c.name === generatorPickerColumn);
        const sel = seedSelections[generatorPickerColumn];
        return (
          <GeneratorPicker
            catalog={seedPlan.generatorCatalog}
            currentGenerator={sel?.generator || ''}
            targetAffinity={col ? sqliteAffinity(col.type) : 'TEXT'}
            recentlyUsed={recentlyUsedGenerators}
            onSelect={(name) => updateSeedGenerator(generatorPickerColumn, name)}
            onClose={() => setGeneratorPickerColumn(null)}
          />
        );
      })()}

      {examplesPickerOpen && examplesList && (
        <ExamplesPicker
          examples={examplesList}
          onSelect={handleSelectExample}
          onClose={() => setExamplesPickerOpen(false)}
        />
      )}

      {pendingExampleSlug && (
        <ConfirmModal
          title="Replace editor contents?"
          destructive
          confirmLabel="Replace"
          body="This will replace the current contents of the SQL Editor with the selected example's DDL."
          onCancel={() => setPendingExampleSlug(null)}
          onConfirm={() => {
            const slug = pendingExampleSlug;
            setPendingExampleSlug(null);
            applyExample(slug);
          }}
        />
      )}

      {/* TOAST SYSTEM */}
      {toast && (
        <div className="fixed bottom-4 right-4 z-50 animate-bounce">
          <div className={`px-4 py-2.5 rounded-lg shadow-lg text-white font-medium text-sm flex items-center gap-2 ${
            toast.type === 'error' ? 'bg-red-600' : 'bg-emerald-600'
          }`}>
            {toast.type === 'error' ? (
              <AlertCircle className="w-4 h-4 text-white shrink-0" />
            ) : (
              <Check className="w-4 h-4 text-white shrink-0" />
            )}
            <span>{toast.message}</span>
          </div>
        </div>
      )}

      {/* DELETE CONFIRMATION MODAL */}
      {deleteConfirmation && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setDeleteConfirmation(null)}
        >
          <div
            className="w-full max-w-sm rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0" /> Confirm Delete
              </h3>
              <button
                onClick={() => setDeleteConfirmation(null)}
                className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300"
              >
                ✕
              </button>
            </div>
            <div className="p-4 space-y-4">
              <p className="text-sm text-slate-650 dark:text-slate-350 font-sans">
                Are you sure you want to delete this row? This action is permanent and cannot be undone.
              </p>
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setDeleteConfirmation(null)}
                  className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    executeDeleteRow(deleteConfirmation.row, deleteConfirmation.rIdx);
                    setDeleteConfirmation(null);
                  }}
                  className="px-3 py-1.5 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
                >
                  Delete
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* DROP COLUMN CONFIRMATION MODAL */}
      {dropColumnConfirmation && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setDropColumnConfirmation(null)}
        >
          <div
            className="w-full max-w-md rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0" /> Confirm Drop Column
              </h3>
              <button
                onClick={() => setDropColumnConfirmation(null)}
                className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300"
              >
                ✕
              </button>
            </div>
            <div className="p-4 space-y-4">
              <p className="text-sm text-slate-650 dark:text-slate-350 font-sans">
                Are you sure you want to drop column <span className="font-semibold font-mono text-indigo-650 dark:text-indigo-400">"{dropColumnConfirmation.colName}"</span>?
                This may require rebuilding the table and dropping indexes referencing it. This action cannot be undone.
              </p>
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setDropColumnConfirmation(null)}
                  className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    executeDropColumn(dropColumnConfirmation.colName);
                    setDropColumnConfirmation(null);
                  }}
                  className="px-3 py-1.5 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
                >
                  Drop Column
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* DROP FOREIGN KEY CONFIRMATION MODAL */}
      {dropFkConfirmation && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setDropFkConfirmation(null)}
        >
          <div
            className="w-full max-w-md rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0" /> Confirm Drop Foreign Key
              </h3>
              <button
                onClick={() => setDropFkConfirmation(null)}
                className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300"
              >
                ✕
              </button>
            </div>
            <div className="p-4 space-y-4">
              <p className="text-sm text-slate-650 dark:text-slate-350 font-sans">
                Are you sure you want to drop the foreign key{' '}
                <span className="font-semibold font-mono text-indigo-650 dark:text-indigo-400">
                  {dropFkConfirmation.from} → {dropFkConfirmation.table}.{dropFkConfirmation.to}
                </span>?
                This requires rebuilding the table. This action cannot be undone.
              </p>
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setDropFkConfirmation(null)}
                  className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    executeDropForeignKey(dropFkConfirmation);
                    setDropFkConfirmation(null);
                  }}
                  className="px-3 py-1.5 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
                >
                  Drop Foreign Key
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* DROP TABLE CONFIRMATION MODAL */}
      {dropTableConfirmation && selectedTable && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
          onClick={() => setDropTableConfirmation(false)}
        >
          <div
            className="w-full max-w-md rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
              <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4 text-red-500 shrink-0" /> Confirm Drop Table
              </h3>
              <button
                onClick={() => setDropTableConfirmation(false)}
                className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300"
              >
                ✕
              </button>
            </div>
            <div className="p-4 space-y-4">
              <p className="text-sm text-slate-650 dark:text-slate-350 font-sans">
                Are you sure you want to drop table <span className="font-semibold font-mono text-red-650 dark:text-red-400">"{selectedTable.name}"</span>?
                This action is permanent and all data will be lost.
              </p>
              {getFkReferencingTables(selectedTable.name).length > 0 && (
                <div className="p-3 rounded border border-amber-250 dark:border-amber-500/30 bg-amber-50/50 dark:bg-amber-500/10 text-xs text-amber-800 dark:text-amber-400 font-sans flex items-start gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-500 shrink-0 mt-0.5" />
                  <div>
                    <span className="font-semibold">⚠️ WARNING:</span> The following tables reference this table via foreign keys:
                    <div className="font-semibold mt-1 font-mono text-[11px]">{getFkReferencingTables(selectedTable.name).join(', ')}</div>
                    Dropping it will cause FK violations or failures.
                  </div>
                </div>
              )}
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setDropTableConfirmation(false)}
                  className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  onClick={() => {
                    executeDropTable();
                    setDropTableConfirmation(false);
                  }}
                  className="px-3 py-1.5 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
                >
                  Drop Table
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

import { useState } from 'react';
import { ArrowLeft, Database, Pencil, Trash2, Download, Upload, Plus, Check, X, Moon, Sun } from 'lucide-react';
import ConfirmModal from './ConfirmModal';
import UploadDbModal from './UploadDbModal';

interface SandboxDbEntry {
  id: string;
  displayName: string;
  sizeBytes: number;
  createdAt: string;
  lastModifiedAt: string;
}

interface SandboxManagePageProps {
  dbs: SandboxDbEntry[];
  activeDbId: string | null;
  onBack: () => void;
  onSwitch: (id: string) => void;
  onRename: (id: string, displayName: string) => void;
  onDelete: (id: string) => void;
  onDownload: (id: string) => void;
  onUpload: (file: File, name?: string) => Promise<boolean>;
  onCreate: (name: string) => Promise<boolean>;
  onError: (message: string) => void;
  theme: 'light' | 'dark';
  toggleTheme: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  return d.toLocaleString();
}

export default function SandboxManagePage({
  dbs,
  activeDbId,
  onBack,
  onSwitch,
  onRename,
  onDelete,
  onDownload,
  onUpload,
  onCreate,
  onError,
  theme,
  toggleTheme,
}: SandboxManagePageProps) {
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<SandboxDbEntry | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newDbName, setNewDbName] = useState('');
  const [creating, setCreating] = useState(false);
  const [showUploadModal, setShowUploadModal] = useState(false);

  const sorted = [...dbs].sort(
    (a, b) => new Date(b.lastModifiedAt).getTime() - new Date(a.lastModifiedAt).getTime()
  );

  const startRename = (db: SandboxDbEntry) => {
    setRenamingId(db.id);
    setRenameValue(db.displayName);
  };

  const commitRename = () => {
    if (renamingId && renameValue.trim()) {
      onRename(renamingId, renameValue.trim());
    }
    setRenamingId(null);
  };

  const handleCreate = async () => {
    if (!newDbName.trim()) return;
    setCreating(true);
    try {
      await onCreate(newDbName.trim());
      setShowCreateForm(false);
      setNewDbName('');
    } catch (err: any) {
      onError(err.message || 'Failed to create database');
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="flex flex-col h-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 antialiased font-sans">
      <header className="flex items-center justify-between px-4 h-12 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shrink-0">
        <div className="flex items-center gap-3">
          <button
            onClick={onBack}
            className="flex items-center gap-1.5 text-xs font-medium text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-200"
          >
            <ArrowLeft className="w-4 h-4" /> Back
          </button>
          <div className="w-px h-4 bg-slate-200 dark:bg-slate-800" />
          <div className="flex items-center gap-2">
            <div className="w-6 h-6 rounded bg-gradient-to-br from-indigo-500 to-sky-500 flex items-center justify-center text-white font-bold text-sm">
              s
            </div>
            <span className="font-semibold tracking-tight text-slate-900 dark:text-white">
              Manage sandbox databases
            </span>
          </div>
        </div>
        <button
          onClick={toggleTheme}
          className="w-8 h-8 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center transition-colors"
          title="Toggle theme"
        >
          {theme === 'light' ? (
            <Moon className="w-4 h-4 text-slate-500" />
          ) : (
            <Sun className="w-4 h-4 text-amber-400" />
          )}
        </button>
      </header>

      <div className="flex-1 overflow-auto p-6">
        <div className="max-w-3xl mx-auto space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            {showCreateForm ? (
              <div className="flex items-center gap-1">
                <input
                  autoFocus
                  value={newDbName}
                  onChange={(e) => setNewDbName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleCreate();
                    if (e.key === 'Escape') setShowCreateForm(false);
                  }}
                  placeholder="New db name"
                  className="text-sm px-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
                />
                <button
                  onClick={handleCreate}
                  disabled={creating || !newDbName.trim()}
                  className="px-2.5 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold disabled:opacity-50"
                >
                  {creating ? 'Creating…' : 'Create'}
                </button>
                <button
                  onClick={() => setShowCreateForm(false)}
                  className="px-2.5 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => setShowCreateForm(true)}
                className="flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium bg-indigo-600 text-white hover:bg-indigo-500"
              >
                <Plus className="w-3.5 h-3.5" /> Create New Database
              </button>
            )}
            <button
              onClick={() => setShowUploadModal(true)}
              className="flex items-center gap-2 px-3 py-1.5 rounded-md text-xs font-medium border border-slate-200 dark:border-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              <Upload className="w-3.5 h-3.5" /> Upload another database
            </button>
          </div>

          <div className="rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 overflow-hidden">
            <div className="px-4 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500 border-b border-slate-100 dark:border-slate-800">
              All databases ({sorted.length})
            </div>
            {sorted.length === 0 ? (
              <div className="px-4 py-10 text-center text-sm text-slate-400">
                No databases yet. Create or upload one to get started.
              </div>
            ) : (
              <div className="divide-y divide-slate-100 dark:divide-slate-800">
                {sorted.map((db) => (
                  <div
                    key={db.id}
                    className={`flex items-center justify-between gap-3 px-4 py-3 ${
                      db.id === activeDbId ? 'bg-indigo-50 dark:bg-indigo-500/10' : ''
                    }`}
                  >
                    <div className="flex items-center gap-3 min-w-0 flex-1">
                      <Database className="w-4 h-4 text-slate-400 shrink-0" />
                      {renamingId === db.id ? (
                        <div className="flex items-center gap-1 flex-1 min-w-0">
                          <input
                            autoFocus
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') commitRename();
                              if (e.key === 'Escape') setRenamingId(null);
                            }}
                            className="flex-1 min-w-0 text-sm px-2 py-1 rounded border border-indigo-300 dark:border-indigo-600 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
                          />
                          <button onClick={commitRename} className="text-emerald-600 hover:text-emerald-500">
                            <Check className="w-4 h-4" />
                          </button>
                          <button onClick={() => setRenamingId(null)} className="text-slate-400 hover:text-slate-500">
                            <X className="w-4 h-4" />
                          </button>
                        </div>
                      ) : (
                        <button onClick={() => onSwitch(db.id)} className="min-w-0 text-left">
                          <div className="text-sm font-medium text-slate-800 dark:text-slate-200 truncate flex items-center gap-2">
                            {db.displayName}
                            {db.id === activeDbId && (
                              <span className="text-[10px] px-1.5 py-0.5 rounded-full font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
                                active
                              </span>
                            )}
                          </div>
                          <div className="text-xs text-slate-400 font-mono">
                            {formatBytes(db.sizeBytes)} · modified {formatDate(db.lastModifiedAt)}
                          </div>
                        </button>
                      )}
                    </div>
                    {renamingId !== db.id && (
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          onClick={() => startRename(db)}
                          title="Rename"
                          className="w-7 h-7 rounded flex items-center justify-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                        >
                          <Pencil className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => onDownload(db.id)}
                          title="Download"
                          className="w-7 h-7 rounded flex items-center justify-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                        >
                          <Download className="w-4 h-4" />
                        </button>
                        <button
                          onClick={() => setDeleteTarget(db)}
                          title="Delete"
                          className="w-7 h-7 rounded flex items-center justify-center text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>

      {showUploadModal && (
        <UploadDbModal
          onUpload={onUpload}
          onClose={() => setShowUploadModal(false)}
          onError={onError}
        />
      )}

      {deleteTarget && (
        <ConfirmModal
          title="Delete database"
          destructive
          confirmLabel="Delete"
          body={
            <>
              Are you sure you want to delete{' '}
              <span className="font-semibold font-mono text-red-650 dark:text-red-400">
                "{deleteTarget.displayName}"
              </span>
              ? This permanently removes the file and cannot be undone.
            </>
          }
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => {
            onDelete(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </div>
  );
}

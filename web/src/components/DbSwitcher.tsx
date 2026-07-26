import { useEffect, useRef, useState } from 'react';
import { ChevronDown, ChevronRight, Database, Pencil, Trash2, Download, Upload, Plus, Check, X } from 'lucide-react';
import ConfirmModal from './ConfirmModal';
import UploadDbModal from './UploadDbModal';

const RECENT_LIMIT = 5;

interface SandboxDbEntry {
  id: string;
  displayName: string;
  sizeBytes: number;
  createdAt: string;
  lastModifiedAt: string;
}

interface DbSwitcherProps {
  activeDbId: string | null;
  dbs: SandboxDbEntry[];
  onSwitch: (id: string) => void;
  onRename: (id: string, displayName: string) => void;
  onDelete: (id: string) => void;
  onDownload: (id: string) => void;
  onUpload: (file: File, name?: string) => Promise<boolean>;
  onCreate: (name: string) => Promise<boolean>;
  onError: (message: string) => void;
  onOpenManage: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export default function DbSwitcher({
  activeDbId,
  dbs,
  onSwitch,
  onRename,
  onDelete,
  onDownload,
  onUpload,
  onCreate,
  onError,
  onOpenManage,
}: DbSwitcherProps) {
  const [open, setOpen] = useState(false);
  const [renamingId, setRenamingId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<SandboxDbEntry | null>(null);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newDbName, setNewDbName] = useState('');
  const [showUploadModal, setShowUploadModal] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const activeDb = dbs.find((d) => d.id === activeDbId) || null;

  const recentDbs = [...dbs]
    .sort((a, b) => new Date(b.lastModifiedAt).getTime() - new Date(a.lastModifiedAt).getTime())
    .slice(0, RECENT_LIMIT);
  const hasMore = dbs.length > RECENT_LIMIT;

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
    try {
      await onCreate(newDbName.trim());
      setShowCreateForm(false);
      setNewDbName('');
      setOpen(false);
    } catch (err: any) {
      onError(err.message || 'Failed to create database');
    }
  };

  return (
    <div className="relative" ref={containerRef}>
      <button
        onClick={() => setOpen((o) => !o)}
        className="flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-mono text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
      >
        <Database className="w-3.5 h-3.5" />
        {activeDb ? activeDb.displayName : 'Select database'}
        <ChevronDown className="w-3 h-3" />
      </button>

      {open && (
        <div className="absolute left-0 top-full mt-1 w-80 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl z-40 overflow-hidden">
          <div className="px-3 py-2 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500 border-b border-slate-100 dark:border-slate-800">
            Recent databases
          </div>
          <div className="max-h-64 overflow-y-auto divide-y divide-slate-100 dark:divide-slate-800">
            {recentDbs.length === 0 && (
              <div className="px-3 py-4 text-xs text-slate-400 text-center">No databases yet</div>
            )}
            {recentDbs.map((db) => (
              <div
                key={db.id}
                className={`flex items-center justify-between gap-2 px-3 py-2 text-sm ${
                  db.id === activeDbId ? 'bg-indigo-50 dark:bg-indigo-500/10' : 'hover:bg-slate-50 dark:hover:bg-slate-850'
                }`}
              >
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
                      className="flex-1 min-w-0 text-xs px-1.5 py-1 rounded border border-indigo-300 dark:border-indigo-600 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
                    />
                    <button onClick={commitRename} className="text-emerald-600 hover:text-emerald-500">
                      <Check className="w-3.5 h-3.5" />
                    </button>
                    <button onClick={() => setRenamingId(null)} className="text-slate-400 hover:text-slate-500">
                      <X className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => {
                      onSwitch(db.id);
                      setOpen(false);
                    }}
                    className="flex-1 min-w-0 text-left"
                  >
                    <div className="font-medium text-slate-800 dark:text-slate-200 truncate">{db.displayName}</div>
                    <div className="text-[10px] text-slate-400 font-mono">{formatBytes(db.sizeBytes)}</div>
                  </button>
                )}
                {renamingId !== db.id && (
                  <div className="flex items-center gap-1 shrink-0">
                    <button
                      onClick={() => startRename(db)}
                      title="Rename"
                      className="w-6 h-6 rounded flex items-center justify-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => onDownload(db.id)}
                      title="Download"
                      className="w-6 h-6 rounded flex items-center justify-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
                    >
                      <Download className="w-3.5 h-3.5" />
                    </button>
                    <button
                      onClick={() => setDeleteTarget(db)}
                      title="Delete"
                      className="w-6 h-6 rounded flex items-center justify-center text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                )}
              </div>
            ))}
            {hasMore && (
              <button
                onClick={() => {
                  setOpen(false);
                  onOpenManage();
                }}
                className="w-full flex items-center justify-between px-3 py-2 text-xs font-medium text-indigo-600 dark:text-indigo-400 hover:bg-slate-50 dark:hover:bg-slate-850"
              >
                More ({dbs.length} total)
                <ChevronRight className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <div className="border-t border-slate-100 dark:border-slate-800 p-2 space-y-1">
            {showCreateForm ? (
              <div className="flex items-center gap-1">
                <input
                  autoFocus
                  value={newDbName}
                  onChange={(e) => setNewDbName(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                  placeholder="New db name"
                  className="flex-1 min-w-0 text-xs px-2 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
                />
                <button onClick={handleCreate} className="text-emerald-600 hover:text-emerald-500">
                  <Check className="w-4 h-4" />
                </button>
                <button onClick={() => setShowCreateForm(false)} className="text-slate-400 hover:text-slate-500">
                  <X className="w-4 h-4" />
                </button>
              </div>
            ) : (
              <button
                onClick={() => setShowCreateForm(true)}
                className="w-full flex items-center gap-2 px-2 py-1.5 rounded-md text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
              >
                <Plus className="w-3.5 h-3.5" /> Create new database
              </button>
            )}
            <button
              onClick={() => setShowUploadModal(true)}
              className="w-full flex items-center gap-2 px-2 py-1.5 rounded-md text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              <Upload className="w-3.5 h-3.5" /> Upload another database
            </button>
          </div>
        </div>
      )}

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
            setOpen(false);
          }}
        />
      )}
    </div>
  );
}

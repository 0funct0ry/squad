import { useRef, useState } from 'react';
import { Database, Moon, Sun, Monitor, Upload, Plus } from 'lucide-react';

interface SandboxDbEntry {
  id: string;
  displayName: string;
  sizeBytes: number;
  createdAt: string;
  lastModifiedAt: string;
}

interface SandboxEmptyStateProps {
  dbs: SandboxDbEntry[];
  onUpload: (file: File, name?: string) => Promise<boolean>;
  onCreate: (name: string) => Promise<boolean>;
  onSelect: (id: string) => void;
  onError: (message: string) => void;
  theme: 'light' | 'dark' | 'system';
  resolvedDark: boolean;
  toggleTheme: () => void;
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}

export default function SandboxEmptyState({
  dbs,
  onUpload,
  onCreate,
  onSelect,
  onError,
  theme,
  resolvedDark,
  toggleTheme,
}: SandboxEmptyStateProps) {
  const [dragOver, setDragOver] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newDbName, setNewDbName] = useState('');
  const [showCreateForm, setShowCreateForm] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFile = async (file: File) => {
    setUploading(true);
    try {
      await onUpload(file);
    } catch (err: any) {
      onError(err.message || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const handleCreate = async () => {
    if (!newDbName.trim()) return;
    setCreating(true);
    try {
      await onCreate(newDbName.trim());
    } catch (err: any) {
      onError(err.message || 'Failed to create database');
    } finally {
      setCreating(false);
      setShowCreateForm(false);
      setNewDbName('');
    }
  };

  return (
    <div className="flex flex-col h-screen bg-slate-50 dark:bg-slate-950 text-slate-800 dark:text-slate-200 antialiased font-sans">
      <header className="flex items-center justify-between px-4 h-12 border-b border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shrink-0">
        <div className="flex items-center gap-2">
          <div className="w-6 h-6 rounded bg-gradient-to-br from-indigo-500 to-sky-500 flex items-center justify-center text-white font-bold text-sm">
            s
          </div>
          <span className="font-semibold tracking-tight text-slate-900 dark:text-white">squad</span>
          <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-400">
            SANDBOX
          </span>
        </div>
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
      </header>

      <div className="flex-1 flex items-center justify-center p-8">
        <div className="w-full max-w-lg space-y-6">
          <div
            onDragOver={(e) => {
              e.preventDefault();
              setDragOver(true);
            }}
            onDragLeave={() => setDragOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragOver(false);
              const file = e.dataTransfer.files?.[0];
              if (file) handleFile(file);
            }}
            onClick={() => fileInputRef.current?.click()}
            className={`rounded-lg border-2 border-dashed p-8 text-center cursor-pointer transition-colors ${
              dragOver
                ? 'border-indigo-500 bg-indigo-50/50 dark:bg-indigo-500/10'
                : 'border-slate-300 dark:border-slate-700 hover:border-indigo-400 dark:hover:border-indigo-500/50'
            }`}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".db,.sqlite,.sqlite3"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) handleFile(file);
                e.target.value = '';
              }}
            />
            <Upload className="w-8 h-8 mx-auto mb-2 text-slate-400" />
            <h2 className="font-semibold text-slate-900 dark:text-white">Upload a database</h2>
            <p className="text-sm text-slate-500 dark:text-slate-400 mt-1">
              {uploading ? 'Uploading…' : 'Drag & drop a .db / .sqlite file here, or click to browse'}
            </p>
          </div>

          <div className="flex items-center gap-3">
            <div className="flex-1 h-px bg-slate-200 dark:bg-slate-800" />
            <span className="text-xs text-slate-400">or</span>
            <div className="flex-1 h-px bg-slate-200 dark:bg-slate-800" />
          </div>

          {showCreateForm ? (
            <div className="rounded-lg border border-slate-200 dark:border-slate-800 p-4 space-y-3">
              <label className="text-xs font-semibold text-slate-500 dark:text-slate-400">
                New database name
              </label>
              <input
                autoFocus
                value={newDbName}
                onChange={(e) => setNewDbName(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
                placeholder="scratch"
                className="w-full text-sm px-2.5 py-1.5 rounded-md bg-slate-100 dark:bg-slate-800 border border-transparent focus:border-indigo-400 outline-none text-slate-950 dark:text-white"
              />
              <div className="flex justify-end gap-2">
                <button
                  onClick={() => setShowCreateForm(false)}
                  className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreate}
                  disabled={creating || !newDbName.trim()}
                  className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow disabled:opacity-50"
                >
                  {creating ? 'Creating…' : 'Create'}
                </button>
              </div>
            </div>
          ) : (
            <button
              onClick={() => setShowCreateForm(true)}
              className="w-full flex items-center justify-center gap-2 px-3 py-2.5 rounded-lg border border-slate-200 dark:border-slate-800 text-sm font-medium text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              <Plus className="w-4 h-4" /> Create a new database
            </button>
          )}

          {dbs.length > 0 && (
            <div className="space-y-2">
              <div className="text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500">
                Or pick an existing database
              </div>
              <div className="rounded-lg border border-slate-200 dark:border-slate-800 divide-y divide-slate-100 dark:divide-slate-800 overflow-hidden">
                {dbs.map((db) => (
                  <button
                    key={db.id}
                    onClick={() => onSelect(db.id)}
                    className="w-full flex items-center justify-between px-3 py-2 text-left hover:bg-slate-50 dark:hover:bg-slate-900 transition-colors"
                  >
                    <span className="flex items-center gap-2 min-w-0">
                      <Database className="w-4 h-4 text-slate-400 shrink-0" />
                      <span className="text-sm font-medium text-slate-800 dark:text-slate-200 truncate">
                        {db.displayName}
                      </span>
                    </span>
                    <span className="text-xs text-slate-400 font-mono shrink-0">
                      {formatBytes(db.sizeBytes)}
                    </span>
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

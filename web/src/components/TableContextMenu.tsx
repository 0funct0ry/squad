import { useEffect, useRef, useState } from 'react';
import {
  Search,
  Copy,
  Clipboard,
  FileCode2,
  Download,
  Pencil,
  Eraser,
  Trash2,
  Check,
  X,
  ChevronRight,
} from 'lucide-react';
import ConfirmModal from './ConfirmModal';

export interface ContextMenuTableInfo {
  name: string;
  type: 'table' | 'view';
  isVirtual?: boolean;
}

const EXPORT_FORMATS: { id: string; label: string }[] = [
  { id: 'csv', label: 'CSV' },
  { id: 'json', label: 'JSON' },
  { id: 'sql', label: 'SQL' },
  { id: 'yaml', label: 'YAML' },
  { id: 'xml', label: 'XML' },
  { id: 'toml', label: 'TOML' },
  { id: 'bson', label: 'BSON' },
  { id: 'protobuf', label: 'Protobuf' },
  { id: 'xlsx', label: 'XLSX' },
  { id: 'parquet', label: 'Parquet' },
];

interface TableContextMenuProps {
  table: ContextMenuTableInfo;
  x: number;
  y: number;
  isWrite: boolean;
  onClose: () => void;
  onQuery: (table: ContextMenuTableInfo) => void;
  onCopyName: (table: ContextMenuTableInfo) => void;
  onCopyDDL: (table: ContextMenuTableInfo) => void;
  onExport: (table: ContextMenuTableInfo, format: string) => void;
  onDuplicate: (table: ContextMenuTableInfo, newName: string, includeData: boolean) => void;
  onRename: (table: ContextMenuTableInfo, newName: string) => void;
  onTruncate: (table: ContextMenuTableInfo) => void;
  onDrop: (table: ContextMenuTableInfo) => void;
}

const writeGateTitle = 'Requires --write mode';

export default function TableContextMenu({
  table,
  x,
  y,
  isWrite,
  onClose,
  onQuery,
  onCopyName,
  onCopyDDL,
  onExport,
  onDuplicate,
  onRename,
  onTruncate,
  onDrop,
}: TableContextMenuProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const [showExportSubmenu, setShowExportSubmenu] = useState(false);
  const [showDuplicateForm, setShowDuplicateForm] = useState(false);
  const [duplicateName, setDuplicateName] = useState(`${table.name}_copy`);
  const [duplicateIncludeData, setDuplicateIncludeData] = useState(true);
  const [showRenameForm, setShowRenameForm] = useState(false);
  const [renameValue, setRenameValue] = useState(table.name);
  const [truncateTarget, setTruncateTarget] = useState<ContextMenuTableInfo | null>(null);
  const [dropTarget, setDropTarget] = useState<ContextMenuTableInfo | null>(null);

  useEffect(() => {
    // While the Truncate/Drop ConfirmModal is open, it renders as a sibling
    // outside containerRef (its own fixed-position overlay), so a mousedown
    // on its Confirm/Cancel button would otherwise be seen as "outside" and
    // close (unmount) this whole menu before the button's click handler —
    // which fires after mousedown — ever runs.
    if (truncateTarget || dropTarget) return;

    const handleClick = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKey);
    };
  }, [onClose, truncateTarget, dropTarget]);

  const isView = table.type === 'view';

  const menuWidth = 240;
  const clampedX = Math.min(x, window.innerWidth - menuWidth - 8);
  const clampedY = Math.min(y, window.innerHeight - 8);

  const itemClass =
    'w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent';

  const commitDuplicate = () => {
    if (!duplicateName.trim()) return;
    onDuplicate(table, duplicateName.trim(), duplicateIncludeData);
    onClose();
  };

  const commitRename = () => {
    if (!renameValue.trim() || renameValue.trim() === table.name) {
      setShowRenameForm(false);
      return;
    }
    onRename(table, renameValue.trim());
    onClose();
  };

  return (
    <>
      <div
        ref={containerRef}
        style={{ position: 'fixed', top: clampedY, left: clampedX, width: menuWidth }}
        className="z-50 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl py-1 text-sm"
      >
        {showDuplicateForm ? (
          <div className="px-3 py-2 space-y-2">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Duplicate table</div>
            <input
              autoFocus
              value={duplicateName}
              onChange={(e) => setDuplicateName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') commitDuplicate();
                if (e.key === 'Escape') setShowDuplicateForm(false);
              }}
              className="w-full text-xs px-2 py-1 rounded border border-indigo-300 dark:border-indigo-600 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
            />
            <label className="flex items-center gap-1.5 text-xs text-slate-600 dark:text-slate-300">
              <input
                type="checkbox"
                checked={duplicateIncludeData}
                onChange={(e) => setDuplicateIncludeData(e.target.checked)}
              />
              Include data
            </label>
            <div className="flex justify-end gap-1">
              <button onClick={commitDuplicate} className="text-emerald-600 hover:text-emerald-500">
                <Check className="w-3.5 h-3.5" />
              </button>
              <button onClick={() => setShowDuplicateForm(false)} className="text-slate-400 hover:text-slate-500">
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        ) : showRenameForm ? (
          <div className="px-3 py-2 space-y-2">
            <div className="text-[10px] font-semibold uppercase tracking-wider text-slate-400">Rename</div>
            <div className="flex items-center gap-1">
              <input
                autoFocus
                value={renameValue}
                onChange={(e) => setRenameValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') commitRename();
                  if (e.key === 'Escape') setShowRenameForm(false);
                }}
                className="flex-1 min-w-0 text-xs px-2 py-1 rounded border border-indigo-300 dark:border-indigo-600 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none"
              />
              <button onClick={commitRename} className="text-emerald-600 hover:text-emerald-500">
                <Check className="w-3.5 h-3.5" />
              </button>
              <button onClick={() => setShowRenameForm(false)} className="text-slate-400 hover:text-slate-500">
                <X className="w-3.5 h-3.5" />
              </button>
            </div>
          </div>
        ) : showExportSubmenu ? (
          <div>
            <button
              onClick={() => setShowExportSubmenu(false)}
              className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-slate-500 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-slate-800"
            >
              ← Back
            </button>
            <div className="max-h-64 overflow-y-auto">
              {EXPORT_FORMATS.map((f) => (
                <button
                  key={f.id}
                  onClick={() => {
                    onExport(table, f.id);
                    onClose();
                  }}
                  className={itemClass}
                >
                  {f.label}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <>
            <button onClick={() => { onQuery(table); onClose(); }} className={itemClass}>
              <Search className="w-3.5 h-3.5" /> Query this table
            </button>
            <button onClick={() => { onCopyName(table); onClose(); }} className={itemClass}>
              <Clipboard className="w-3.5 h-3.5" /> Copy name
            </button>
            <button onClick={() => { onCopyDDL(table); onClose(); }} className={itemClass}>
              <FileCode2 className="w-3.5 h-3.5" /> Copy CREATE {isView ? 'VIEW' : 'TABLE'} DDL
            </button>
            <button onClick={() => setShowExportSubmenu(true)} className={itemClass}>
              <Download className="w-3.5 h-3.5" /> Export…
              <ChevronRight className="w-3 h-3 ml-auto" />
            </button>
            {!isView && (
              <button
                onClick={() => setShowDuplicateForm(true)}
                disabled={!isWrite}
                title={!isWrite ? writeGateTitle : undefined}
                className={itemClass}
              >
                <Copy className="w-3.5 h-3.5" /> Duplicate table
              </button>
            )}
            <button
              onClick={() => setShowRenameForm(true)}
              disabled={!isWrite}
              title={!isWrite ? writeGateTitle : undefined}
              className={itemClass}
            >
              <Pencil className="w-3.5 h-3.5" /> Rename
            </button>
            {!isView && (
              <button
                onClick={() => setTruncateTarget(table)}
                disabled={!isWrite}
                title={!isWrite ? writeGateTitle : undefined}
                className={itemClass}
              >
                <Eraser className="w-3.5 h-3.5" /> Truncate
              </button>
            )}
            <div className="my-1 border-t border-slate-100 dark:border-slate-800" />
            <button
              onClick={() => setDropTarget(table)}
              disabled={!isWrite}
              title={!isWrite ? writeGateTitle : undefined}
              className={`${itemClass} text-red-600 dark:text-red-400 hover:bg-red-50 dark:hover:bg-red-500/10`}
            >
              <Trash2 className="w-3.5 h-3.5" /> Drop
            </button>
          </>
        )}
      </div>

      {truncateTarget && (
        <ConfirmModal
          title="Truncate table"
          destructive
          confirmLabel="Truncate"
          body={
            <>
              This deletes all rows in{' '}
              <span className="font-semibold font-mono text-red-650 dark:text-red-400">"{truncateTarget.name}"</span>{' '}
              but keeps the table structure. This cannot be undone.
            </>
          }
          onCancel={() => {
            setTruncateTarget(null);
            onClose();
          }}
          onConfirm={() => {
            onTruncate(truncateTarget);
            setTruncateTarget(null);
            onClose();
          }}
        />
      )}

      {dropTarget && (
        <ConfirmModal
          title="Drop table"
          destructive
          confirmLabel="Drop"
          body={
            <>
              Are you sure you want to drop{' '}
              <span className="font-semibold font-mono text-red-650 dark:text-red-400">"{dropTarget.name}"</span>?
              This action is permanent and all data will be lost.
            </>
          }
          onCancel={() => {
            setDropTarget(null);
            onClose();
          }}
          onConfirm={() => {
            onDrop(dropTarget);
            setDropTarget(null);
            onClose();
          }}
        />
      )}
    </>
  );
}

import { useEffect, useRef, useState } from 'react';
import {
  MoreVertical,
  Table2,
  Eye,
  Upload,
  RefreshCw,
  ArrowDownAZ,
  Hash,
  FileDown,
  Network,
  Check,
} from 'lucide-react';

const writeGateTitle = 'Requires --write mode';

export type SidebarSortBy = 'name' | 'rowcount';

interface SidebarSectionMenuProps {
  isWrite: boolean;
  sortBy: SidebarSortBy;
  onSortByChange: (sortBy: SidebarSortBy) => void;
  showSystemTables: boolean;
  onShowSystemTablesChange: (show: boolean) => void;
  onCreateTable: () => void;
  onCreateView: () => void;
  onImportTable: () => void;
  onRefresh: () => void;
  onExportAll: () => void;
  onShowErDiagram: () => void;
}

export default function SidebarSectionMenu({
  isWrite,
  sortBy,
  onSortByChange,
  showSystemTables,
  onShowSystemTablesChange,
  onCreateTable,
  onCreateView,
  onImportTable,
  onRefresh,
  onExportAll,
  onShowErDiagram,
}: SidebarSectionMenuProps) {
  const [open, setOpen] = useState(false);
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

  const itemClass =
    'w-full flex items-center gap-2 px-3 py-1.5 text-xs text-left text-slate-700 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40 disabled:cursor-not-allowed disabled:hover:bg-transparent';

  const sectionLabelClass = 'px-3 pt-2 pb-1 text-[10px] font-semibold uppercase tracking-wider text-slate-400 dark:text-slate-500';

  return (
    <div className="relative" ref={containerRef}>
      <button
        onClick={() => setOpen((o) => !o)}
        title="Tables & Views actions"
        className="w-5 h-5 rounded flex items-center justify-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800"
      >
        <MoreVertical className="w-3.5 h-3.5" />
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-1 w-56 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl z-40 py-1 overflow-hidden">
          <div className={sectionLabelClass}>Create</div>
          <button
            onClick={() => { onCreateTable(); setOpen(false); }}
            disabled={!isWrite}
            title={!isWrite ? writeGateTitle : undefined}
            className={itemClass}
          >
            <Table2 className="w-3.5 h-3.5" /> Create table
          </button>
          <button
            onClick={() => { onCreateView(); setOpen(false); }}
            disabled={!isWrite}
            title={!isWrite ? writeGateTitle : undefined}
            className={itemClass}
          >
            <Eye className="w-3.5 h-3.5" /> Create view
          </button>
          <button
            onClick={() => { onImportTable(); setOpen(false); }}
            disabled={!isWrite}
            title={!isWrite ? writeGateTitle : undefined}
            className={itemClass}
          >
            <Upload className="w-3.5 h-3.5" /> Import table
          </button>

          <div className="my-1 border-t border-slate-100 dark:border-slate-800" />
          <div className={sectionLabelClass}>Section</div>
          <button onClick={() => { onRefresh(); setOpen(false); }} className={itemClass}>
            <RefreshCw className="w-3.5 h-3.5" /> Refresh
          </button>
          <button onClick={() => onSortByChange('name')} className={itemClass}>
            <ArrowDownAZ className="w-3.5 h-3.5" /> Sort by name
            {sortBy === 'name' && <Check className="w-3 h-3 ml-auto text-indigo-500" />}
          </button>
          <button onClick={() => onSortByChange('rowcount')} className={itemClass}>
            <Hash className="w-3.5 h-3.5" /> Sort by row count
            {sortBy === 'rowcount' && <Check className="w-3 h-3 ml-auto text-indigo-500" />}
          </button>

          <div className="my-1 border-t border-slate-100 dark:border-slate-800" />
          <div className={sectionLabelClass}>Database</div>
          <button onClick={() => { onExportAll(); setOpen(false); }} className={itemClass}>
            <FileDown className="w-3.5 h-3.5" /> Export all
          </button>
          <button onClick={() => { onShowErDiagram(); setOpen(false); }} className={itemClass}>
            <Network className="w-3.5 h-3.5" /> Show schema diagram
          </button>

          <div className="my-1 border-t border-slate-100 dark:border-slate-800" />
          <button
            onClick={() => onShowSystemTablesChange(!showSystemTables)}
            className={itemClass}
          >
            <span className="w-3.5 h-3.5 inline-flex items-center justify-center">
              {showSystemTables && <Check className="w-3 h-3 text-indigo-500" />}
            </span>
            Show system tables
          </button>
        </div>
      )}
    </div>
  );
}

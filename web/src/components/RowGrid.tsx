import { useEffect, useState } from 'react';
import { Save, X, Edit2, Trash2 } from 'lucide-react';
import ConfirmModal from './ConfirmModal';

export interface RowGridProps {
  columns: string[];
  rows: any[][];
  /** Resolve the key (PK columns, or rowid) for a given row — used for edit/delete/bulk-delete requests. */
  getRowKey: (row: any[]) => Record<string, any>;
  /** Whether write actions (edit/delete/bulk-delete) are available at all. */
  isWrite: boolean;
  /** Column is not editable (e.g. rowid, generated columns). Defaults to never read-only. */
  isColumnReadOnly?: (colName: string) => boolean;
  /** Column should use a numeric <input>. Defaults to false. */
  isColumnNumeric?: (colName: string) => boolean;
  /** Custom cell renderer (e.g. BLOB preview). Falls back to a plain string / NULL renderer. */
  renderCell?: (val: any, colName: string, cIdx: number) => React.ReactNode;
  /** Custom header content per column (e.g. sort/filter controls). Falls back to the plain column name. */
  renderHeaderCell?: (colName: string) => React.ReactNode;
  /** Extra <tr> rendered as the first row of the body (e.g. an inline "add row" form). */
  addRowSlot?: React.ReactNode;
  /**
   * Column names to omit from rendering entirely (header + cells), while
   * keeping them present in the underlying row data — so edit/delete key
   * resolution and in-progress edits of a hidden column's value still work
   * correctly, only the display is affected. Useful for a "column
   * visibility" selector over wide tables.
   */
  hiddenColumns?: Set<string>;
  onSaveEdit: (key: Record<string, any>, values: Record<string, any>) => Promise<void>;
  onDeleteRow: (key: Record<string, any>) => Promise<void>;
  onBulkDelete: (keys: Record<string, any>[]) => Promise<void>;
  /** Changing this value (e.g. the active table name) clears the current selection/edit state. */
  resetKey?: string | number;
}

function defaultRenderCell(val: any): React.ReactNode {
  if (val === null || val === undefined) {
    return <span className="text-slate-400 dark:text-slate-600 italic">NULL</span>;
  }
  return String(val);
}

export default function RowGrid({
  columns,
  rows,
  getRowKey,
  isWrite,
  isColumnReadOnly,
  isColumnNumeric,
  renderCell,
  renderHeaderCell,
  addRowSlot,
  hiddenColumns,
  onSaveEdit,
  onDeleteRow,
  onBulkDelete,
  resetKey,
}: RowGridProps) {
  const [editingRowIndex, setEditingRowIndex] = useState<number | null>(null);
  const [editingRowValues, setEditingRowValues] = useState<Record<string, any>>({});
  const [deleteConfirmation, setDeleteConfirmation] = useState<{ row: any[] } | null>(null);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [bulkDeleteConfirmOpen, setBulkDeleteConfirmOpen] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);

  useEffect(() => {
    setSelected(new Set());
    setEditingRowIndex(null);
    setEditingRowValues({});
  }, [resetKey]);

  const cell = renderCell || ((val: any) => defaultRenderCell(val));

  const allSelected = rows.length > 0 && selected.size === rows.length;
  const someSelected = selected.size > 0 && !allSelected;

  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set());
    } else {
      setSelected(new Set(rows.map((_, i) => i)));
    }
  };

  const toggleRow = (rIdx: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(rIdx)) next.delete(rIdx);
      else next.add(rIdx);
      return next;
    });
  };

  const startEdit = (rIdx: number, row: any[]) => {
    setEditingRowIndex(rIdx);
    const vals: Record<string, any> = {};
    columns.forEach((c, i) => {
      vals[c] = row[i];
    });
    setEditingRowValues(vals);
  };

  const saveEdit = async (row: any[]) => {
    const key = getRowKey(row);
    await onSaveEdit(key, editingRowValues);
    setEditingRowIndex(null);
  };

  const handleBulkDelete = async () => {
    setBulkDeleting(true);
    try {
      const keys = Array.from(selected).map((idx) => getRowKey(rows[idx]));
      await onBulkDelete(keys);
      setSelected(new Set());
    } finally {
      setBulkDeleting(false);
      setBulkDeleteConfirmOpen(false);
    }
  };

  return (
    <div className="flex flex-col gap-2 h-full min-h-0">
      {isWrite && selected.size > 0 && (
        <div className="flex items-center justify-between shrink-0 px-1">
          <span className="text-xs text-slate-500 dark:text-slate-400">
            {selected.size} row{selected.size === 1 ? '' : 's'} selected
          </span>
          <button
            onClick={() => setBulkDeleteConfirmOpen(true)}
            className="flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-red-600 hover:bg-red-500 text-white text-xs font-semibold shadow cursor-pointer"
          >
            <Trash2 className="w-3.5 h-3.5" /> Delete selected
          </button>
        </div>
      )}

      <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto bg-white dark:bg-slate-900 flex-1 min-h-0">
        <table className="w-full text-sm font-mono relative">
          <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 dark:text-slate-400 text-left sticky top-0 z-10">
            <tr>
              {isWrite && (
                <th className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800 w-8">
                  <input
                    type="checkbox"
                    checked={allSelected}
                    ref={(el) => {
                      if (el) el.indeterminate = someSelected;
                    }}
                    onChange={toggleAll}
                    className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
                  />
                </th>
              )}
              {columns.map((col) => (
                hiddenColumns?.has(col) ? null : (
                  <th key={col} className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800">
                    {renderHeaderCell ? renderHeaderCell(col) : col}
                  </th>
                )
              ))}
              {isWrite && (
                <th className="px-3 py-2 font-medium border-b border-slate-200 dark:border-slate-800 w-24 text-right">
                  Actions
                </th>
              )}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100 dark:divide-slate-800 text-slate-700 dark:text-slate-300">
            {addRowSlot}
            {rows.map((row, rIdx) => {
              const isEditing = editingRowIndex === rIdx;
              return (
                <tr key={rIdx} className="hover:bg-slate-50 dark:hover:bg-slate-800/40">
                  {isWrite && (
                    <td className="px-3 py-1.5">
                      <input
                        type="checkbox"
                        checked={selected.has(rIdx)}
                        onChange={() => toggleRow(rIdx)}
                        className="rounded border-slate-300 dark:border-slate-700 text-indigo-600 focus:ring-indigo-500"
                      />
                    </td>
                  )}
                  {row.map((val, cIdx) => {
                    const colName = columns[cIdx];
                    if (hiddenColumns?.has(colName)) return null;
                    const readOnly = isColumnReadOnly ? isColumnReadOnly(colName) : false;
                    const numeric = isColumnNumeric ? isColumnNumeric(colName) : false;

                    if (isEditing) {
                      return (
                        <td key={cIdx} className="px-3 py-1.5">
                          <input
                            type={numeric ? 'number' : 'text'}
                            step="any"
                            disabled={readOnly}
                            value={editingRowValues[colName] ?? ''}
                            onChange={(e) => {
                              setEditingRowValues((prev) => ({ ...prev, [colName]: e.target.value }));
                            }}
                            className="px-2 py-0.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs w-full outline-none"
                          />
                        </td>
                      );
                    }

                    return (
                      <td key={cIdx} className="px-3 py-1.5 whitespace-nowrap overflow-hidden max-w-xs text-ellipsis">
                        {cell(val, colName, cIdx)}
                      </td>
                    );
                  })}
                  {isWrite && (
                    <td className="px-3 py-1.5 space-x-2 text-right whitespace-nowrap">
                      {isEditing ? (
                        <>
                          <button
                            onClick={() => saveEdit(row)}
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
                            onClick={() => startEdit(rIdx, row)}
                            title="Edit"
                            className="text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 p-1.5 rounded transition-colors text-base cursor-pointer inline-flex items-center justify-center"
                          >
                            <Edit2 className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => setDeleteConfirmation({ row })}
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

      {deleteConfirmation && (
        <ConfirmModal
          title="Confirm Delete"
          destructive
          confirmLabel="Delete"
          body="Are you sure you want to delete this row? This action is permanent and cannot be undone."
          onCancel={() => setDeleteConfirmation(null)}
          onConfirm={() => {
            const key = getRowKey(deleteConfirmation.row);
            setDeleteConfirmation(null);
            onDeleteRow(key);
          }}
        />
      )}

      {bulkDeleteConfirmOpen && (
        <ConfirmModal
          title="Confirm Bulk Delete"
          destructive
          confirmLabel={bulkDeleting ? 'Deleting…' : `Delete ${selected.size} row${selected.size === 1 ? '' : 's'}`}
          body={`Are you sure you want to delete ${selected.size} selected row${selected.size === 1 ? '' : 's'}? This action is permanent and cannot be undone.`}
          onCancel={() => setBulkDeleteConfirmOpen(false)}
          onConfirm={handleBulkDelete}
        />
      )}
    </div>
  );
}

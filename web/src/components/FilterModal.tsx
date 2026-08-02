import { useState } from 'react';
import { X } from 'lucide-react';
import {
  type ColumnFilter,
  type FilterOperator,
  OPERATOR_LABELS,
  operatorNeedsList,
  operatorNeedsNoValue,
  operatorNeedsTwoValues,
} from '../lib/columnFilter';

const OPERATORS: FilterOperator[] = [
  'eq',
  'neq',
  'contains',
  'starts_with',
  'ends_with',
  'gt',
  'lt',
  'between',
  'is_null',
  'is_not_null',
  'in',
  'not_in',
];

interface FilterModalProps {
  column: string;
  initial?: ColumnFilter;
  onCancel: () => void;
  onApply: (filter: ColumnFilter) => void;
}

export default function FilterModal({ column, initial, onCancel, onApply }: FilterModalProps) {
  const [operator, setOperator] = useState<FilterOperator>(initial?.operator || 'contains');
  const [value, setValue] = useState<string>(initial?.value !== undefined ? String(initial.value) : '');
  const [value2, setValue2] = useState<string>(initial?.value2 !== undefined ? String(initial.value2) : '');
  const [listValue, setListValue] = useState<string>((initial?.values || []).join(', '));

  const apply = () => {
    if (operatorNeedsNoValue(operator)) {
      onApply({ column, operator });
      return;
    }
    if (operatorNeedsTwoValues(operator)) {
      onApply({ column, operator, value, value2 });
      return;
    }
    if (operatorNeedsList(operator)) {
      const values = listValue
        .split(',')
        .map((v) => v.trim())
        .filter((v) => v !== '');
      onApply({ column, operator, values });
      return;
    }
    onApply({ column, operator, value });
  };

  const canApply = operatorNeedsNoValue(operator)
    || (operatorNeedsTwoValues(operator) && value !== '' && value2 !== '')
    || (operatorNeedsList(operator) && listValue.trim() !== '')
    || (!operatorNeedsNoValue(operator) && !operatorNeedsTwoValues(operator) && !operatorNeedsList(operator) && value !== '');

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onCancel}
    >
      <div
        className="w-full max-w-sm rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900 dark:text-white">
            Filter <span className="font-mono text-indigo-500">{column}</span>
          </h3>
          <button onClick={onCancel} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-4 space-y-3">
          <label className="block">
            <span className="text-xs text-slate-500 dark:text-slate-400">Operator</span>
            <select
              value={operator}
              onChange={(e) => setOperator(e.target.value as FilterOperator)}
              className="mt-1 w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
            >
              {OPERATORS.map((op) => (
                <option key={op} value={op}>
                  {OPERATOR_LABELS[op]}
                </option>
              ))}
            </select>
          </label>

          {operatorNeedsTwoValues(operator) && (
            <div className="flex items-center gap-2">
              <input
                type="text"
                placeholder="From"
                value={value}
                onChange={(e) => setValue(e.target.value)}
                className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
              />
              <span className="text-slate-400 text-xs">and</span>
              <input
                type="text"
                placeholder="To"
                value={value2}
                onChange={(e) => setValue2(e.target.value)}
                className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
              />
            </div>
          )}

          {operatorNeedsList(operator) && (
            <input
              type="text"
              placeholder="value1, value2, value3"
              value={listValue}
              onChange={(e) => setListValue(e.target.value)}
              className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
            />
          )}

          {!operatorNeedsNoValue(operator) && !operatorNeedsTwoValues(operator) && !operatorNeedsList(operator) && (
            <input
              type="text"
              placeholder="Value"
              value={value}
              onChange={(e) => setValue(e.target.value)}
              className="w-full px-2 py-1.5 rounded border border-slate-300 dark:border-slate-750 bg-white dark:bg-slate-950 text-slate-900 dark:text-white text-sm outline-none"
              autoFocus
            />
          )}
        </div>
        <div className="px-4 py-3 border-t border-slate-200 dark:border-slate-800 flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 cursor-pointer"
          >
            Cancel
          </button>
          <button
            onClick={apply}
            disabled={!canApply}
            className="px-3 py-1.5 rounded-md bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shadow cursor-pointer disabled:opacity-50"
          >
            Apply filter
          </button>
        </div>
      </div>
    </div>
  );
}

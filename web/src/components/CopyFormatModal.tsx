import { Copy, X } from 'lucide-react';

export interface CopyFormatOption {
  label: string;
  run: () => void;
}

interface CopyFormatModalProps {
  title: string;
  options: CopyFormatOption[];
  onCancel: () => void;
}

/** Picker modal for "Copy as…" — one clear list of formats instead of a long scrolling dropdown. */
export default function CopyFormatModal({ title, options, onCancel }: CopyFormatModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onCancel}>
      <div
        className="w-full max-w-sm rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <Copy className="w-4 h-4 text-indigo-500" />
            {title}
          </h3>
          <button onClick={onCancel} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-2 max-h-[60vh] overflow-y-auto grid grid-cols-2 gap-1.5">
          {options.map((option) => (
            <button
              key={option.label}
              onClick={() => {
                option.run();
                onCancel();
              }}
              className="px-3 py-2 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-medium text-slate-700 dark:text-slate-300 hover:bg-indigo-50 dark:hover:bg-indigo-500/10 hover:border-indigo-300 dark:hover:border-indigo-700 hover:text-indigo-600 dark:hover:text-indigo-400 cursor-pointer text-left"
            >
              {option.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

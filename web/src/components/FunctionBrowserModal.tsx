import { X, Sigma } from 'lucide-react';
import FunctionsTab from './FunctionsTab';

interface FunctionBrowserModalProps {
  onToast: (message: string, type: 'error' | 'success') => void;
  isWrite: boolean;
  onInsert: (snippet: string) => void;
  onClose: () => void;
}

// FunctionBrowserModal hosts the exact same FunctionsTab content (category
// sidebar, search, Try it/Insert/DEFAULT/generated-column/CHECK actions) in
// a modal opened from the SQL editor's "fx" button, rather than a
// standalone tab — onInsert both performs the insert and closes the modal,
// so picking an action returns the user straight to the editor.
export default function FunctionBrowserModal({ onToast, isWrite, onInsert, onClose }: FunctionBrowserModalProps) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-4xl h-[80vh] rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800 shrink-0">
          <h3 className="font-semibold text-slate-900 dark:text-white flex items-center gap-1.5">
            <Sigma className="w-4 h-4 text-slate-400" />
            Function browser
          </h3>
          <button
            onClick={onClose}
            className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400"
            title="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="flex-1 overflow-hidden p-4">
          <FunctionsTab
            onToast={onToast}
            isWrite={isWrite}
            onInsert={(snippet) => {
              onInsert(snippet);
              onClose();
            }}
          />
        </div>
      </div>
    </div>
  );
}

import { useRef, useState } from 'react';
import { Upload, X } from 'lucide-react';

interface UploadDbModalProps {
  onUpload: (file: File, name?: string) => Promise<boolean>;
  onClose: () => void;
  onError: (message: string) => void;
}

export default function UploadDbModal({ onUpload, onClose, onError }: UploadDbModalProps) {
  const [dragOver, setDragOver] = useState(false);
  const [uploading, setUploading] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFile = async (file: File) => {
    setUploading(true);
    try {
      await onUpload(file);
      onClose();
    } catch (err: any) {
      onError(err.message || 'Upload failed');
      setUploading(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-md rounded-lg border border-slate-205 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between">
          <h3 className="font-semibold text-slate-900 dark:text-white">Upload a database</h3>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-4">
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
            className={`rounded-lg border-2 border-dashed p-8 text-center transition-colors ${
              dragOver
                ? 'border-indigo-500 bg-indigo-50/50 dark:bg-indigo-500/10'
                : 'border-slate-300 dark:border-slate-700'
            }`}
          >
            <Upload className="w-8 h-8 mx-auto mb-2 text-slate-400" />
            <p className="text-sm text-slate-500 dark:text-slate-400 mb-3">
              {uploading ? 'Uploading…' : 'Drag & drop a .db / .sqlite file here'}
            </p>
            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={uploading}
              className="px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-xs font-semibold text-slate-700 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-850 disabled:opacity-50"
            >
              Browse files
            </button>
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
          </div>
        </div>
      </div>
    </div>
  );
}

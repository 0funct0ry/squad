import { useEffect, useRef, useState } from 'react';
import { X, Clipboard } from 'lucide-react';
import { apiFetch } from '../lib/api';

interface ErDiagramTableInfo {
  name: string;
  type: 'table' | 'view';
}

interface SchemaColumn {
  name: string;
  type: string;
  pk: number;
}

interface SchemaForeignKey {
  table: string;
  from: string;
  to: string;
}

interface TableSchemaResponse {
  columns: SchemaColumn[];
  foreignKeys: SchemaForeignKey[];
}

interface ErDiagramModalProps {
  tables: ErDiagramTableInfo[];
  theme: 'light' | 'dark';
  onClose: () => void;
  onToast: (message: string, type: 'success' | 'error') => void;
}

// sanitizeMermaidId strips characters mermaid can't parse in an unquoted
// entity/attribute name (identifiers may contain spaces/punctuation in
// SQLite but not in bare Mermaid erDiagram syntax).
function sanitizeMermaidId(name: string): string {
  const cleaned = name.replace(/[^A-Za-z0-9_]/g, '_');
  return /^[A-Za-z_]/.test(cleaned) ? cleaned : `_${cleaned}`;
}

async function buildErDiagramSource(tables: ErDiagramTableInfo[]): Promise<{ source: string; hasRelationships: boolean }> {
  const lines: string[] = ['erDiagram'];
  let hasRelationships = false;

  const schemas = new Map<string, TableSchemaResponse>();
  for (const t of tables) {
    if (t.type === 'view') continue;
    try {
      const res = await apiFetch(`/tables/${encodeURIComponent(t.name)}/schema`);
      const body = await res.json();
      if (res.ok && body.ok) {
        schemas.set(t.name, body.data);
      }
    } catch {
      // skip tables we can't introspect rather than failing the whole diagram
    }
  }

  for (const [name, schema] of schemas) {
    const entityId = sanitizeMermaidId(name);
    lines.push(`    ${entityId} {`);
    for (const col of schema.columns) {
      const attrType = sanitizeMermaidId(col.type || 'ANY') || 'ANY';
      const attrName = sanitizeMermaidId(col.name);
      lines.push(`        ${attrType} ${attrName}${col.pk ? ' PK' : ''}`);
    }
    lines.push('    }');
  }

  for (const [name, schema] of schemas) {
    for (const fk of schema.foreignKeys) {
      if (!schemas.has(fk.table)) continue;
      hasRelationships = true;
      const from = sanitizeMermaidId(name);
      const to = sanitizeMermaidId(fk.table);
      lines.push(`    ${from} }o--|| ${to} : "${fk.from} -> ${fk.to}"`);
    }
  }

  return { source: lines.join('\n'), hasRelationships };
}

export default function ErDiagramModal({ tables, theme, onClose, onToast }: ErDiagramModalProps) {
  const [source, setSource] = useState<string | null>(null);
  const [hasRelationships, setHasRelationships] = useState(false);
  const [svg, setSvg] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const renderIdRef = useRef(`er-diagram-${Math.random().toString(36).slice(2)}`);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      const { source: src, hasRelationships: hasRel } = await buildErDiagramSource(tables);
      if (cancelled) return;
      setSource(src);
      setHasRelationships(hasRel);

      if (!hasRel) return;

      try {
        const mermaid = (await import('mermaid')).default;
        mermaid.initialize({ startOnLoad: false, theme: theme === 'dark' ? 'dark' : 'default' });
        const { svg: rendered } = await mermaid.render(renderIdRef.current, src);
        if (!cancelled) setSvg(rendered);
      } catch (err: any) {
        if (!cancelled) setError(err.message || 'Failed to render diagram');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [tables, theme]);

  const copySource = () => {
    if (!source) return;
    navigator.clipboard.writeText(source);
    onToast('Copied diagram source', 'success');
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={onClose}>
      <div
        className="w-full max-w-4xl h-[80vh] rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b border-slate-200 dark:border-slate-800 flex items-center justify-between shrink-0">
          <h3 className="font-semibold text-slate-900 dark:text-white">Schema diagram (ER view)</h3>
          <div className="flex items-center gap-2">
            <button
              onClick={copySource}
              disabled={!source}
              className="flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-40"
              title="Copy diagram source"
            >
              <Clipboard className="w-3.5 h-3.5" /> Copy diagram source
            </button>
            <button onClick={onClose} className="text-slate-400 hover:text-slate-500 dark:hover:text-slate-300">
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>
        <div className="flex-1 overflow-auto p-4">
          {error ? (
            <div className="text-sm text-red-600 dark:text-red-400">{error}</div>
          ) : !hasRelationships && source !== null ? (
            <div className="flex flex-col items-center justify-center h-full text-center gap-2 text-slate-400 dark:text-slate-600">
              <span className="text-sm">No foreign key relationships found</span>
              <span className="text-xs">Add a foreign key to a table to see it visualized here.</span>
            </div>
          ) : svg ? (
            <div dangerouslySetInnerHTML={{ __html: svg }} />
          ) : (
            <div className="text-sm text-slate-400">Loading…</div>
          )}
        </div>
      </div>
    </div>
  );
}

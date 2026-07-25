type OptionKind = 'int' | 'float' | 'bool' | 'string' | 'datetime' | 'select' | 'columns';

interface OptionField {
  key: string;
  label: string;
  kind: OptionKind;
  default?: unknown;
  choices?: string[];
  min?: number;
  max?: number;
  required?: boolean;
  description?: string;
}

interface GeneratorOptionsFormProps {
  schema: OptionField[];
  values: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
  siblingColumns?: string[];
}

const inputClass =
  'px-1.5 py-0.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white outline-none text-xs';

// Convert an RFC3339 wire value (matches the app-wide `datetime` generator
// from/to convention) to the value expected by <input type="datetime-local">,
// and back on change. datetime-local has no timezone, so we truncate to
// minutes precision the same way the rest of the app does for that input type.
function rfc3339ToLocalInput(value: unknown): string {
  if (typeof value !== 'string' || !value) return '';
  const d = new Date(value);
  if (isNaN(d.getTime())) return '';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function localInputToRfc3339(value: string): string | undefined {
  if (!value) return undefined;
  const d = new Date(value);
  if (isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

export default function GeneratorOptionsForm({ schema, values, onChange, siblingColumns }: GeneratorOptionsFormProps) {
  if (!schema || schema.length === 0) {
    return <span className="text-slate-300 dark:text-slate-700">—</span>;
  }

  return (
    <div className="flex items-center gap-2 flex-wrap">
      {schema.map((field) => {
        const value = values?.[field.key];

        if (field.kind === 'int' || field.kind === 'float') {
          return (
            <label key={field.key} className="flex items-center gap-1 text-slate-400">
              {field.label}
              <input
                type="number"
                step={field.kind === 'float' ? 'any' : 1}
                min={field.min}
                max={field.max}
                placeholder={field.label}
                value={value === undefined || value === null ? '' : (value as number)}
                onChange={(e) =>
                  onChange(field.key, e.target.value === '' ? undefined : Number(e.target.value))
                }
                className={`w-20 ${inputClass}`}
              />
            </label>
          );
        }

        if (field.kind === 'bool') {
          return (
            <label key={field.key} className="flex items-center gap-1 text-slate-400">
              <input
                type="checkbox"
                checked={!!value}
                onChange={(e) => onChange(field.key, e.target.checked)}
              />
              {field.label}
            </label>
          );
        }

        if (field.kind === 'datetime') {
          return (
            <label key={field.key} className="flex items-center gap-1 text-slate-400">
              {field.label}
              <input
                type="datetime-local"
                value={rfc3339ToLocalInput(value)}
                onChange={(e) => onChange(field.key, localInputToRfc3339(e.target.value))}
                className={inputClass}
              />
            </label>
          );
        }

        if (field.kind === 'select') {
          return (
            <label key={field.key} className="flex items-center gap-1 text-slate-400">
              {field.label}
              <select
                value={(value as string) ?? ''}
                onChange={(e) => onChange(field.key, e.target.value)}
                className={inputClass}
              >
                <option value="">—</option>
                {(field.choices || []).map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            </label>
          );
        }

        if (field.kind === 'columns') {
          const selected: string[] = Array.isArray(value) ? (value as string[]) : [];
          const toggle = (col: string) => {
            const next = selected.includes(col)
              ? selected.filter((c) => c !== col)
              : [...selected, col];
            onChange(field.key, next);
          };
          return (
            <div key={field.key} className="flex flex-col gap-0.5">
              <span className="text-slate-400">{field.label}</span>
              <div className="flex flex-wrap gap-1 max-w-xs">
                {(siblingColumns || []).map((col) => (
                  <label
                    key={col}
                    className="flex items-center gap-1 px-1 py-0.5 rounded border border-slate-200 dark:border-slate-800 text-slate-500"
                  >
                    <input
                      type="checkbox"
                      checked={selected.includes(col)}
                      onChange={() => toggle(col)}
                    />
                    {col}
                  </label>
                ))}
              </div>
            </div>
          );
        }

        // string (default)
        return (
          <label key={field.key} className="flex items-center gap-1 text-slate-400">
            {field.label}
            <input
              type="text"
              placeholder={field.label}
              value={(value as string) ?? ''}
              onChange={(e) => onChange(field.key, e.target.value || undefined)}
              className={`w-28 ${inputClass}`}
            />
          </label>
        );
      })}
    </div>
  );
}

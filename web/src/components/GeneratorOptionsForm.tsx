import { useState } from 'react';
import GeneratorPicker, { type OptionField, type GeneratorMeta } from './GeneratorPicker';

export type { OptionField, GeneratorMeta };

interface GeneratorOptionsFormProps {
  schema: OptionField[];
  values: Record<string, unknown>;
  onChange: (key: string, value: unknown) => void;
  siblingColumns?: string[];
  catalog?: GeneratorMeta[];
  affinity?: string;
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

// Value shape stored for an OptKindGenerator option (nullWithProbability's
// "generator" option): the wrapped generator's name plus its own options,
// mirroring ColumnSpec's own {generator, options} shape one level down.
interface WrappedGeneratorValue {
  generator?: string;
  options?: Record<string, unknown>;
}

function WrappedGeneratorField({
  field,
  value,
  onChange,
  catalog,
  affinity,
  siblingColumns,
}: {
  field: OptionField;
  value: WrappedGeneratorValue;
  onChange: (value: WrappedGeneratorValue) => void;
  catalog: GeneratorMeta[];
  affinity: string;
  siblingColumns?: string[];
}) {
  const [pickerOpen, setPickerOpen] = useState(false);
  const wrappedName = value?.generator || '';
  const wrappedMeta = catalog.find((g) => g.name === wrappedName);

  return (
    <div className="flex flex-col gap-1">
      <span className="text-slate-400">{field.label}</span>
      <button
        type="button"
        onClick={() => setPickerOpen(true)}
        className="px-2 py-1 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none text-left hover:border-indigo-400"
      >
        {wrappedName || '— choose a generator —'}
      </button>
      {wrappedMeta && (wrappedMeta.optionsSchema || []).length > 0 && (
        <div className="pl-2 border-l border-slate-200 dark:border-slate-800">
          <GeneratorOptionsForm
            schema={wrappedMeta.optionsSchema || []}
            values={value?.options || {}}
            onChange={(key, v) =>
              onChange({ generator: wrappedName, options: { ...(value?.options || {}), [key]: v } })
            }
            siblingColumns={siblingColumns}
            catalog={catalog}
            affinity={affinity}
          />
        </div>
      )}
      {pickerOpen && (
        <GeneratorPicker
          catalog={catalog.filter((g) => g.name !== 'nullWithProbability')}
          currentGenerator={wrappedName}
          targetAffinity={affinity}
          recentlyUsed={[]}
          onSelect={(name) => onChange({ generator: name, options: {} })}
          onClose={() => setPickerOpen(false)}
        />
      )}
    </div>
  );
}

export default function GeneratorOptionsForm({
  schema,
  values,
  onChange,
  siblingColumns,
  catalog,
  affinity,
}: GeneratorOptionsFormProps) {
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

        if (field.kind === 'textarea') {
          return (
            <label key={field.key} className="flex flex-col gap-0.5 text-slate-400 w-full max-w-sm">
              {field.label}
              <textarea
                placeholder={field.description || field.label}
                value={(value as string) ?? ''}
                onChange={(e) => onChange(field.key, e.target.value || undefined)}
                rows={3}
                className={`${inputClass} w-full font-mono resize-y`}
              />
            </label>
          );
        }

        if (field.kind === 'generator') {
          return (
            <WrappedGeneratorField
              key={field.key}
              field={field}
              value={(value as WrappedGeneratorValue) || {}}
              onChange={(v) => onChange(field.key, v)}
              catalog={catalog || []}
              affinity={affinity || 'TEXT'}
              siblingColumns={siblingColumns}
            />
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

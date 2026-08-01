import { useEffect, useState } from 'react';
import GeneratorPicker, { type GeneratorMeta } from './GeneratorPicker';
import GeneratorOptionsForm from './GeneratorOptionsForm';

interface SeedColumnPlan {
  name: string;
  type: string;
  skip: boolean;
  reason: string | null;
  generator: string | null;
  options: Record<string, any> | null;
  uniqueGroup?: string[];
}

interface SeedPlan {
  columns: SeedColumnPlan[];
  availableGenerators: string[];
  generatorCatalog: GeneratorMeta[];
}

interface SeedColumnSelection {
  generator: string;
  options: Record<string, any>;
}

interface SeedPanelProps {
  tableName: string;
  isWrite: boolean;
  seedPlan: SeedPlan | null;
  seedPlanLoading: boolean;
  seedPlanError: string | null;
  seedSelections: Record<string, SeedColumnSelection>;
  seedOverrides: Record<string, boolean>;
  seedGeneratorSamples: Record<string, string>;
  seedCount: number;
  seedPreviewRows: Record<string, any>[] | null;
  seedPreviewLoading: boolean;
  seedInsertLoading: boolean;
  seedError: string | null;
  recentlyUsedGenerators: string[];
  toggleSeedOverride: (col: SeedColumnPlan) => void;
  updateSeedGenerator: (colName: string, generator: string) => void;
  updateSeedOption: (colName: string, key: string, value: any) => void;
  generatorMetaByName: (name: string) => GeneratorMeta | undefined;
  sqliteAffinity: (type: string) => string;
  handleSeedCountChange: (raw: string) => void;
  handleSeedPreview: () => Promise<void>;
  handleSeedInsert: () => Promise<void>;
  previewSingleRow: (colName: string) => Promise<Record<string, any> | null>;
}

export default function SeedPanel({
  isWrite,
  seedPlan,
  seedPlanLoading,
  seedPlanError,
  seedSelections,
  seedOverrides,
  seedGeneratorSamples,
  seedCount,
  seedPreviewRows,
  seedPreviewLoading,
  seedInsertLoading,
  seedError,
  recentlyUsedGenerators,
  toggleSeedOverride,
  updateSeedGenerator,
  updateSeedOption,
  generatorMetaByName,
  sqliteAffinity,
  handleSeedCountChange,
  handleSeedPreview,
  handleSeedInsert,
  previewSingleRow,
}: SeedPanelProps) {
  const [selectedColumn, setSelectedColumn] = useState<string | null>(null);
  const [generatorPickerOpen, setGeneratorPickerOpen] = useState(false);

  // Default to the first non-skipped column once a plan loads (or reset when
  // it's cleared, e.g. switching tables), without ever picking on the user's
  // behalf if they've already selected something.
  useEffect(() => {
    if (!seedPlan) {
      setSelectedColumn(null);
      return;
    }
    setSelectedColumn((prev) => {
      if (prev && seedPlan.columns.some((c) => c.name === prev)) return prev;
      const firstActive = seedPlan.columns.find((c) => !c.skip || seedOverrides[c.name]);
      return firstActive ? firstActive.name : seedPlan.columns[0]?.name ?? null;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedPlan]);

  if (seedPlanLoading) return <p className="text-sm text-slate-400">Loading seed plan…</p>;
  if (seedPlanError) return <p className="text-sm text-rose-500">{seedPlanError}</p>;
  if (!seedPlan) return null;

  const selectedCol = seedPlan.columns.find((c) => c.name === selectedColumn) || null;
  const selectedOverridden = selectedCol ? !!seedOverrides[selectedCol.name] : false;
  const selectedActive = selectedCol ? (!selectedCol.skip || selectedOverridden) : false;
  const selectedSel = selectedColumn ? seedSelections[selectedColumn] : undefined;
  const selectedMeta = selectedSel ? generatorMetaByName(selectedSel.generator) : undefined;

  return (
    <>
      <div className="flex-1 min-h-0 grid grid-cols-[260px_1fr] border border-slate-200 dark:border-slate-800 rounded-lg overflow-hidden bg-white dark:bg-slate-900 max-w-4xl">
        {/* Column list */}
        <div className="border-r border-slate-200 dark:border-slate-800 bg-slate-50 dark:bg-slate-950/40 flex flex-col overflow-y-auto">
          <div className="px-3.5 py-2.5 text-[11px] font-semibold uppercase tracking-wide text-slate-400 border-b border-slate-200 dark:border-slate-800 shrink-0">
            {seedPlan.columns.length} columns
          </div>
          {seedPlan.columns.map((col) => {
            const overridden = !!seedOverrides[col.name];
            const active = !col.skip || overridden;
            const sel = seedSelections[col.name];
            const isSelected = col.name === selectedColumn;
            return (
              <button
                key={col.name}
                type="button"
                onClick={() => setSelectedColumn(col.name)}
                className={`flex items-center justify-between gap-2 px-3.5 py-2.5 text-left border-b border-slate-200 dark:border-slate-800 cursor-pointer ${
                  isSelected
                    ? 'bg-indigo-50 dark:bg-indigo-500/10 shadow-[inset_3px_0_0_0] shadow-indigo-500'
                    : 'hover:bg-slate-100 dark:hover:bg-slate-900'
                }`}
              >
                <div className="min-w-0">
                  <div
                    className={`font-mono text-xs font-semibold truncate ${
                      isSelected ? 'text-indigo-600 dark:text-indigo-400' : 'text-slate-700 dark:text-slate-300'
                    } ${!active ? 'italic font-medium text-slate-400' : ''}`}
                  >
                    {col.name}
                  </div>
                  {!active && (
                    <div className="text-[10.5px] italic text-slate-400">skipped · {col.reason}</div>
                  )}
                </div>
                {active && sel?.generator && (
                  <span
                    className={`shrink-0 font-mono text-[10px] px-1.5 py-0.5 rounded-full border ${
                      isSelected
                        ? 'bg-indigo-500 border-indigo-500 text-white'
                        : 'bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-500'
                    }`}
                  >
                    {sel.generator}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {/* Detail pane */}
        <div className="flex flex-col min-w-0 min-h-0">
          {!selectedCol ? (
            <div className="flex-1 flex items-center justify-center text-sm text-slate-400">
              Select a column to configure it.
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between gap-3 px-5 py-3.5 border-b border-slate-200 dark:border-slate-800 shrink-0">
                <h3 className="font-mono font-semibold text-sm text-slate-900 dark:text-white">
                  {selectedCol.name}
                </h3>
                <div className="flex items-center gap-2">
                  {selectedCol.skip && (
                    <button
                      onClick={() => toggleSeedOverride(selectedCol)}
                      className="text-xs underline text-slate-400 hover:text-indigo-500 cursor-pointer"
                    >
                      {selectedOverridden ? 'Skip again' : 'Override'}
                    </button>
                  )}
                  {selectedActive && (
                    <button
                      type="button"
                      onClick={() => setGeneratorPickerOpen(true)}
                      disabled={!isWrite}
                      className="px-2.5 py-1.5 rounded border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-slate-900 dark:text-white font-mono text-xs outline-none disabled:opacity-50 disabled:cursor-not-allowed hover:border-indigo-400 cursor-pointer flex flex-col items-start"
                    >
                      <span>{selectedSel?.generator || '— choose a generator —'}</span>
                      {selectedSel?.generator &&
                        seedGeneratorSamples[`${selectedCol.name}:${selectedSel.generator}`] && (
                          <span className="font-normal text-slate-400 truncate max-w-[14rem]">
                            → {seedGeneratorSamples[`${selectedCol.name}:${selectedSel.generator}`]}
                          </span>
                        )}
                    </button>
                  )}
                </div>
              </div>

              <div className="flex-1 min-h-0 overflow-y-auto p-5 flex flex-col gap-4">
                {!selectedActive && (
                  <p className="text-sm text-slate-400 italic">
                    This column is skipped — {selectedCol.reason}. Click Override to configure it anyway.
                  </p>
                )}
                {selectedActive &&
                  (selectedSel?.generator === 'foreignKey' || selectedSel?.generator === 'enumFromColumn' ? (
                    <span className="text-sm text-slate-400">
                      {selectedSel.options?.table}.{selectedSel.options?.column}
                    </span>
                  ) : (
                    <GeneratorOptionsForm
                      schema={selectedMeta?.optionsSchema || []}
                      values={selectedSel?.options || {}}
                      onChange={(key, value) => updateSeedOption(selectedCol.name, key, value)}
                      siblingColumns={seedPlan.columns.map((c) => c.name)}
                      catalog={seedPlan.generatorCatalog}
                      affinity={sqliteAffinity(selectedCol.type)}
                      columnName={selectedCol.name}
                      generatorName={selectedSel?.generator || ''}
                      onPreviewSingleRow={() => previewSingleRow(selectedCol.name)}
                    />
                  ))}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Sticky footer */}
      <div className="flex items-center gap-3 flex-wrap border-t border-slate-200 dark:border-slate-800 pt-3 mt-3 max-w-4xl shrink-0">
        <label className="text-sm flex items-center gap-2">
          Rows
          <input
            type="number"
            min={1}
            max={100000}
            value={seedCount}
            onChange={(e) => handleSeedCountChange(e.target.value)}
            disabled={!isWrite}
            className="font-mono w-28 px-2 py-1 rounded border border-slate-300 dark:border-slate-700 bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-white outline-none"
          />
        </label>
        {seedCount >= 100000 && <span className="text-xs text-amber-500">clamped to 100,000</span>}
        <div className="flex-1" />
        <button
          onClick={handleSeedPreview}
          disabled={!isWrite || seedPreviewLoading}
          className={`px-3 py-1.5 rounded-md border border-slate-200 dark:border-slate-700 text-sm ${
            isWrite ? 'hover:bg-slate-100 dark:hover:bg-slate-850 cursor-pointer' : 'opacity-50 cursor-not-allowed'
          }`}
        >
          {seedPreviewLoading ? 'Previewing…' : 'Preview 5 rows'}
        </button>
        <button
          onClick={handleSeedInsert}
          disabled={!isWrite || seedInsertLoading}
          title={isWrite ? 'Insert seeded rows' : 'Write mode required'}
          className={`px-3 py-1.5 rounded-md bg-indigo-600 text-white text-sm ${
            isWrite && !seedInsertLoading ? 'hover:bg-indigo-700 cursor-pointer' : 'opacity-50 cursor-not-allowed'
          }`}
        >
          {seedInsertLoading ? 'Inserting…' : 'Insert'}
        </button>
      </div>

      {seedError && <p className="text-sm text-rose-500 max-w-4xl">{seedError}</p>}

      {seedPreviewRows && (
        <div className="border border-slate-200 dark:border-slate-800 rounded-lg overflow-auto bg-white dark:bg-slate-900 max-w-4xl">
          <table className="w-full text-xs font-mono">
            <thead className="bg-slate-100 dark:bg-slate-800/60 text-slate-500 text-left">
              <tr>
                {Object.keys(seedPreviewRows[0] || {}).map((col) => (
                  <th key={col} className="px-3 py-2 font-medium">
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
              {seedPreviewRows.map((row, i) => (
                <tr key={i}>
                  {Object.keys(seedPreviewRows[0] || {}).map((col) => (
                    <td key={col} className="px-3 py-2 text-slate-700 dark:text-slate-300">
                      {row[col] === null || row[col] === undefined ? (
                        <span className="text-slate-400 italic">NULL</span>
                      ) : (
                        String(row[col])
                      )}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {generatorPickerOpen && selectedCol && (
        <GeneratorPicker
          catalog={seedPlan.generatorCatalog}
          currentGenerator={selectedSel?.generator || ''}
          targetAffinity={sqliteAffinity(selectedCol.type)}
          recentlyUsed={recentlyUsedGenerators}
          onSelect={(name) => {
            updateSeedGenerator(selectedCol.name, name);
          }}
          onClose={() => setGeneratorPickerOpen(false)}
        />
      )}
    </>
  );
}

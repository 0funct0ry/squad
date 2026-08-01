import type { CompletionSource } from '@codemirror/autocomplete';
import { fetchFunctionsCatalog, allFunctions, type FunctionMeta } from './functionsCatalog';

let fnsPromise: Promise<FunctionMeta[]> | null = null;

function loadFunctions(): Promise<FunctionMeta[]> {
  if (!fnsPromise) {
    fnsPromise = fetchFunctionsCatalog().then(allFunctions).catch(() => []);
  }
  return fnsPromise;
}

// udfCompletionSource is a CodeMirror 6 CompletionSource that suggests
// squad's curated UDFs (M10b) alongside SQL's own keyword completions,
// sourced from the same GET /api/functions catalog the Functions tab uses.
export const udfCompletionSource: CompletionSource = (context) => {
  const word = context.matchBefore(/[A-Za-z_][A-Za-z0-9_]*/);
  if (!word || (word.from === word.to && !context.explicit)) return null;

  return loadFunctions().then((fns) => ({
    from: word.from,
    options: fns.map((fn) => ({
      label: fn.name,
      detail: fn.signature,
      info: fn.description,
      type: fn.aggregate ? 'function' : 'function',
      apply: fn.signature.includes('()') ? `${fn.name}()` : `${fn.name}(`,
    })),
    validFor: /^[A-Za-z_][A-Za-z0-9_]*$/,
  }));
};

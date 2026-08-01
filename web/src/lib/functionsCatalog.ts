import { apiFetch } from './api';

export interface FunctionExample {
  call: string;
  result: string;
}

export interface FunctionMeta {
  name: string;
  signature: string;
  description: string;
  example: FunctionExample;
  aggregate: boolean;
  deterministic: boolean;
}

export interface FunctionCategory {
  name: string;
  functions: FunctionMeta[];
}

let cached: Promise<FunctionCategory[]> | null = null;

// fetchFunctionsCatalog fetches GET /api/functions once per session (cached
// across callers — the Functions tab and the SQL editor's autocomplete both
// call this instead of hand-duplicating the function list).
export function fetchFunctionsCatalog(): Promise<FunctionCategory[]> {
  if (!cached) {
    cached = apiFetch('/functions')
      .then((res) => res.json())
      .then((body) => {
        if (!body.ok) throw new Error(body.error?.message || 'Failed to load functions');
        return body.data.categories as FunctionCategory[];
      })
      .catch((err) => {
        cached = null;
        throw err;
      });
  }
  return cached;
}

export function allFunctions(categories: FunctionCategory[]): FunctionMeta[] {
  return categories.flatMap((c) => c.functions);
}

// Thin fetch wrapper that resolves every `/api`-relative call against
// whichever database is currently active. In normal (single-DB) mode the
// prefix is just `/api`; in sandbox mode it becomes
// `/api/sandbox/dbs/:id`, so every existing call site keeps working
// unmodified aside from dropping the literal `/api` prefix.
let apiBase = '/api';

export function setApiBase(base: string) {
  apiBase = base;
}

export function apiUrl(path: string): string {
  return `${apiBase}${path}`;
}

export function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  return fetch(apiUrl(path), init);
}

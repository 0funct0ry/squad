// Theme toggle logic shared by the marketing page and Starlight docs.
// Mirrors the pattern in web/src/App.tsx: 'color-scheme' in localStorage
// holds 'light' | 'dark' | 'system'; we resolve 'system' against
// prefers-color-scheme and stamp the resolved value onto
// <html data-theme="...">.
export type ThemeChoice = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'color-scheme';

export function getStoredChoice(): ThemeChoice {
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === 'light' || saved === 'dark' || saved === 'system') return saved;
  return 'system';
}

export function resolveTheme(choice: ThemeChoice): 'light' | 'dark' {
  if (choice === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return choice;
}

export function applyTheme(choice: ThemeChoice) {
  const resolved = resolveTheme(choice);
  document.documentElement.setAttribute('data-theme', resolved);
  localStorage.setItem(STORAGE_KEY, choice);
}

export function initTheme() {
  const choice = getStoredChoice();
  applyTheme(choice);
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (getStoredChoice() === 'system') applyTheme('system');
  });
}

export function cycleTheme() {
  const order: ThemeChoice[] = ['system', 'light', 'dark'];
  const current = getStoredChoice();
  const next = order[(order.indexOf(current) + 1) % order.length];
  applyTheme(next);
  return next;
}

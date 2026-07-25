import { useEffect, useMemo, useRef, useState } from 'react';
import {
  X,
  Search,
  User,
  MapPin,
  Calendar,
  Hash,
  Globe,
  DollarSign,
  Building2,
  Palette,
  Type,
  UtensilsCrossed,
  Package,
  IdCard,
  Lock,
  BarChart3,
  Sparkles,
  Layers,
  SquareFunction,
  Boxes,
  Image as ImageIcon,
  AudioLines,
  type LucideIcon,
} from 'lucide-react';
import { sniffBase64, dataUriFromBase64, type BlobMediaType } from '../lib/blobMedia';

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

interface GeneratorMeta {
  name: string;
  group: string;
  aliases?: string[];
  description?: string;
  affinities: string[];
  optionsSchema?: OptionField[];
  stateful?: boolean;
}

interface GeneratorPickerProps {
  catalog: GeneratorMeta[];
  currentGenerator: string;
  targetAffinity: string;
  recentlyUsed: string[];
  onSelect: (name: string) => void;
  onClose: () => void;
}

// Presentation-only mapping of backend group keys -> icon/label. The set of
// groups and which generators belong to them always comes from the backend
// catalog; this map never decides membership, only how a known group renders.
// Any group not listed here falls back to a generic icon (Boxes) with its
// raw key as the label.
const GROUP_PRESENTATION: Record<string, { label: string; icon: LucideIcon }> = {
  person: { label: 'Person', icon: User },
  geo: { label: 'Geo', icon: MapPin },
  datetime: { label: 'Date & Time', icon: Calendar },
  numeric: { label: 'Numeric', icon: Hash },
  internet: { label: 'Internet', icon: Globe },
  finance: { label: 'Finance', icon: DollarSign },
  company: { label: 'Company', icon: Building2 },
  color: { label: 'Color', icon: Palette },
  text: { label: 'Text', icon: Type },
  food: { label: 'Food', icon: UtensilsCrossed },
  product: { label: 'Product', icon: Package },
  identifier: { label: 'Identifier', icon: IdCard },
  security: { label: 'Security', icon: Lock },
  distribution: { label: 'Distribution', icon: BarChart3 },
  'cross-column': { label: 'Cross-Column', icon: SquareFunction },
  novelty: { label: 'Novelty', icon: Sparkles },
  'domain-lookup': { label: 'Domain Lookup', icon: Layers },
  special: { label: 'Special', icon: Boxes },
  media: { label: 'Media', icon: ImageIcon },
};

const FALLBACK_PRESENTATION = { label: '', icon: Boxes };

function groupPresentation(group: string): { label: string; icon: LucideIcon } {
  const p = GROUP_PRESENTATION[group];
  if (p) return p;
  return { label: group, icon: FALLBACK_PRESENTATION.icon };
}

// Hand-rolled scored substring match — no fuzzy-match library in package.json.
// Prefix match on the name scores highest, then substring-in-name, then
// matches against aliases/group/description at progressively lower weight.
function scoreMatch(g: GeneratorMeta, query: string): number {
  if (!query) return 0;
  const q = query.toLowerCase();
  const name = g.name.toLowerCase();
  let score = 0;
  if (name === q) score = Math.max(score, 100);
  else if (name.startsWith(q)) score = Math.max(score, 80);
  else if (name.includes(q)) score = Math.max(score, 60);

  for (const alias of g.aliases || []) {
    const a = alias.toLowerCase();
    if (a === q) score = Math.max(score, 90);
    else if (a.startsWith(q)) score = Math.max(score, 70);
    else if (a.includes(q)) score = Math.max(score, 50);
  }

  if (g.group.toLowerCase().includes(q)) score = Math.max(score, 30);
  if ((g.description || '').toLowerCase().includes(q)) score = Math.max(score, 20);

  return score;
}

const sampleCache = new Map<string, string>();

async function fetchSample(name: string, affinity: string): Promise<string | null> {
  const cacheKey = `${name}::${affinity}`;
  if (sampleCache.has(cacheKey)) return sampleCache.get(cacheKey)!;
  try {
    const res = await fetch(`/api/seed/generators/${encodeURIComponent(name)}/sample?affinity=${encodeURIComponent(affinity)}`);
    const body = await res.json();
    if (!res.ok || !body.ok) return null;
    const sample = body.data?.sample;
    const text = sample === null || sample === undefined ? '' : String(sample);
    sampleCache.set(cacheKey, text);
    return text;
  } catch {
    return null;
  }
}

function GeneratorCard({
  gen,
  highlighted,
  targetAffinity,
  onSelect,
  onHighlight,
  cardRef,
}: {
  gen: GeneratorMeta;
  highlighted: boolean;
  targetAffinity: string;
  onSelect: () => void;
  onHighlight: () => void;
  cardRef?: (el: HTMLButtonElement | null) => void;
}) {
  const [sample, setSample] = useState<string | null>(null);
  const [fetched, setFetched] = useState(false);
  const [sampleAffinity, setSampleAffinity] = useState<string | null>(null);
  const presentation = groupPresentation(gen.group);
  const Icon = presentation.icon;

  // Lazy fetch: only load a live sample once the card is actually hovered,
  // focused, or becomes the keyboard-highlighted row — keeps the sample
  // endpoint from being hit for every card in the (potentially 199-entry)
  // catalog just because the modal is open.
  const load = () => {
    if (fetched || gen.name === 'foreignKey' || gen.name === 'formula') return;
    setFetched(true);
    const affinity = gen.affinities.includes(targetAffinity) ? targetAffinity : gen.affinities[0];
    if (!affinity) return;
    setSampleAffinity(affinity);
    fetchSample(gen.name, affinity).then((s) => setSample(s));
  };

  // For BLOB samples, Go's json.Marshal encodes []byte as base64 — sniff and
  // render a thumbnail instead of dumping the raw base64 text.
  const blobMediaType: BlobMediaType | null =
    sampleAffinity === 'BLOB' && sample ? sniffBase64(sample) : null;

  useEffect(() => {
    if (highlighted) load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [highlighted]);

  return (
    <button
      ref={cardRef}
      onClick={onSelect}
      onMouseEnter={() => {
        onHighlight();
        load();
      }}
      onFocus={() => {
        onHighlight();
        load();
      }}
      className={`w-full text-left px-3 py-2 rounded-md border text-xs flex flex-col gap-0.5 outline-none ${
        highlighted
          ? 'border-indigo-500 bg-indigo-50 dark:bg-indigo-500/10'
          : 'border-transparent hover:bg-slate-100 dark:hover:bg-slate-800/60'
      }`}
    >
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 font-mono font-medium text-slate-900 dark:text-white">
          <Icon className="w-3.5 h-3.5 text-slate-400 shrink-0" />
          {gen.name}
        </span>
        {(gen.group === 'novelty' || gen.group === 'domain-lookup') && (
          <span className="px-1.5 py-0.5 rounded bg-slate-200 dark:bg-slate-800 text-slate-500 text-[10px] font-medium">
            {gen.group}
          </span>
        )}
      </div>
      {gen.description && <span className="text-slate-400">{gen.description}</span>}
      {sample !== null && sample !== '' && blobMediaType && blobMediaType !== 'unknown' && (
        <span className="flex items-center gap-1.5 text-slate-500 dark:text-slate-400">
          →
          <span className="inline-flex items-center justify-center w-7 h-7 rounded border border-slate-200 dark:border-slate-750 bg-slate-50 dark:bg-slate-800/60 overflow-hidden shrink-0">
            {blobMediaType === 'wav' ? (
              <AudioLines className="w-3.5 h-3.5 text-indigo-500 dark:text-indigo-400" />
            ) : (
              <img
                src={dataUriFromBase64(sample, blobMediaType)}
                alt={gen.name}
                className="max-w-full max-h-full object-contain"
              />
            )}
          </span>
        </span>
      )}
      {sample !== null && sample !== '' && !blobMediaType && (
        <span className="text-slate-500 dark:text-slate-400 font-mono truncate">→ {sample}</span>
      )}
    </button>
  );
}

export default function GeneratorPicker({
  catalog,
  currentGenerator,
  targetAffinity,
  recentlyUsed,
  onSelect,
  onClose,
}: GeneratorPickerProps) {
  const [query, setQuery] = useState('');
  const [category, setCategory] = useState<string | null>(null);
  const [showAllTypes, setShowAllTypes] = useState(false);
  const [highlightIndex, setHighlightIndex] = useState(0);
  const searchRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    searchRef.current?.focus();
  }, []);

  const groups = useMemo(() => {
    const counts = new Map<string, number>();
    catalog.forEach((g) => counts.set(g.group, (counts.get(g.group) || 0) + 1));
    return Array.from(counts.entries()).sort((a, b) => a[0].localeCompare(b[0]));
  }, [catalog]);

  const noFilterActive = query.trim() === '' && category === null;

  const recentEntries = useMemo(() => {
    if (!noFilterActive) return [];
    const byName = new Map(catalog.map((g) => [g.name, g]));
    return recentlyUsed
      .map((n) => byName.get(n))
      .filter((g): g is GeneratorMeta => !!g)
      .slice(0, 5);
  }, [noFilterActive, recentlyUsed, catalog]);

  const filtered = useMemo(() => {
    let list = catalog;
    if (category) list = list.filter((g) => g.group === category);
    if (!showAllTypes) list = list.filter((g) => (g.affinities || []).includes(targetAffinity));

    const q = query.trim();
    if (!q) {
      return list.slice().sort((a, b) => a.name.localeCompare(b.name));
    }
    return list
      .map((g) => ({ g, score: scoreMatch(g, q) }))
      .filter((x) => x.score > 0)
      .sort((a, b) => b.score - a.score || a.g.name.localeCompare(b.g.name))
      .map((x) => x.g);
  }, [catalog, category, showAllTypes, query, targetAffinity]);

  // The list actually rendered/navigated: recently-used pinned block (only
  // when no filter active) followed by the regular filtered results, with
  // recent entries de-duplicated out of the main list.
  const displayList = useMemo(() => {
    if (recentEntries.length === 0) return filtered;
    const recentNames = new Set(recentEntries.map((g) => g.name));
    return [...recentEntries, ...filtered.filter((g) => !recentNames.has(g.name))];
  }, [recentEntries, filtered]);

  useEffect(() => {
    setHighlightIndex(0);
  }, [query, category, showAllTypes]);

  const cardRefs = useRef<Array<HTMLButtonElement | null>>([]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setHighlightIndex((i) => Math.min(i + 1, displayList.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setHighlightIndex((i) => Math.max(i - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const target = displayList[highlightIndex];
      if (target) {
        onSelect(target.name);
        onClose();
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      onClose();
    }
  };

  useEffect(() => {
    cardRefs.current[highlightIndex]?.scrollIntoView({ block: 'nearest' });
  }, [highlightIndex]);

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
    >
      <div
        className="w-full max-w-4xl rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-xl flex flex-col max-h-[80vh]"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={handleKeyDown}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-slate-200 dark:border-slate-800">
          <h3 className="font-semibold text-slate-900 dark:text-white">
            Choose a generator
            {currentGenerator && (
              <span className="ml-2 font-mono text-xs text-slate-400">current: {currentGenerator}</span>
            )}
          </h3>
          <button
            onClick={onClose}
            className="w-7 h-7 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center justify-center text-slate-500 dark:text-slate-400"
            title="Close"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="px-4 pt-3">
          <div className="relative">
            <Search className="w-4 h-4 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-400" />
            <input
              ref={searchRef}
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Search generators…"
              className="w-full pl-8 pr-2 py-1.5 rounded-md border border-slate-300 dark:border-slate-700 bg-white dark:bg-slate-950 text-sm text-slate-900 dark:text-white outline-none"
            />
          </div>
          <label className="mt-2 flex items-center gap-1.5 text-xs text-slate-400">
            <input type="checkbox" checked={showAllTypes} onChange={(e) => setShowAllTypes(e.target.checked)} />
            Show all types (ignore column type compatibility)
          </label>
        </div>

        <div className="flex flex-1 min-h-0 mt-3">
          <div className="w-56 shrink-0 border-r border-slate-200 dark:border-slate-800 overflow-y-auto px-2 py-2">
            <button
              onClick={() => setCategory(null)}
              className={`w-full text-left px-2 py-1.5 rounded text-xs mb-0.5 ${
                category === null ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-800/60 text-slate-600 dark:text-slate-300'
              }`}
            >
              All ({catalog.length})
            </button>
            {groups.map(([group, count]) => {
              const presentation = groupPresentation(group);
              const Icon = presentation.icon;
              return (
                <button
                  key={group}
                  onClick={() => setCategory(group)}
                  className={`w-full text-left px-2 py-1.5 rounded text-xs mb-0.5 flex items-center gap-1.5 ${
                    category === group ? 'bg-indigo-50 dark:bg-indigo-500/10 text-indigo-600 dark:text-indigo-400' : 'hover:bg-slate-100 dark:hover:bg-slate-800/60 text-slate-600 dark:text-slate-300'
                  }`}
                >
                  <Icon className="w-3.5 h-3.5 shrink-0" />
                  <span className="truncate flex-1">{presentation.label}</span>
                  <span className="text-slate-400">{count}</span>
                </button>
              );
            })}
          </div>

          <div className="flex-1 overflow-y-auto px-2 py-2">
            {recentEntries.length > 0 && (
              <div className="mb-2">
                <div className="px-1 pb-1 text-[10px] uppercase tracking-wide text-slate-400 font-medium">
                  Recently used
                </div>
              </div>
            )}
            {displayList.length === 0 && (
              <p className="px-2 py-4 text-xs text-slate-400">No generators match.</p>
            )}
            <div className="flex flex-col gap-0.5">
              {displayList.map((gen, i) => (
                <GeneratorCard
                  key={gen.name}
                  gen={gen}
                  highlighted={i === highlightIndex}
                  targetAffinity={targetAffinity}
                  onSelect={() => {
                    onSelect(gen.name);
                    onClose();
                  }}
                  onHighlight={() => setHighlightIndex(i)}
                  cardRef={(el) => (cardRefs.current[i] = el)}
                />
              ))}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

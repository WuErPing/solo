// Host-side cache for mermaid diagrams rendered to SVG by the shared service.
// Keyed by theme + full source (exact match — hash collisions would render the
// wrong diagram). LRU eviction: least-recently-used entries are dropped first.

export interface MermaidSvgEntry {
  /** PNG data URL when the service rasterized the diagram (preferred). */
  pngDataUrl?: string;
  /** Full `<svg …>…</svg>` markup fallback when rasterization is unavailable. */
  xml?: string;
  /** Natural size of the diagram in CSS px. */
  widthPx: number;
  heightPx: number;
}

export type MermaidSvgCacheValue = MermaidSvgEntry | null;

const CACHE_LIMIT = 200;

const cache = new Map<string, MermaidSvgCacheValue>();

export function mermaidCacheKey(theme: string, source: string): string {
  // NUL never appears in theme names ("default"/"dark") and is not a valid
  // mermaid source character, so the pair stays unambiguous.
  return `${theme}\0${source}`;
}

export function getCachedMermaidSvg(theme: string, source: string): MermaidSvgCacheValue | undefined {
  const key = mermaidCacheKey(theme, source);
  if (!cache.has(key)) {
    return undefined;
  }
  // Touch: re-insert at the tail so hot entries survive eviction.
  const value = cache.get(key) ?? null;
  cache.delete(key);
  cache.set(key, value);
  return value;
}

export function setCachedMermaidSvg(theme: string, source: string, value: MermaidSvgCacheValue): void {
  const key = mermaidCacheKey(theme, source);
  cache.delete(key);
  cache.set(key, value);
  while (cache.size > CACHE_LIMIT) {
    const oldest = cache.keys().next();
    if (oldest.done) break;
    cache.delete(oldest.value);
  }
}

/** Test hook. */
export function clearMermaidSvgCache(): void {
  cache.clear();
}

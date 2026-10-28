// App-side cache for PlantUML SVGs served by the daemon. Keyed by host +
// theme + full source (exact match — collisions would paint the wrong
// diagram). LRU eviction: least-recently-used entries are dropped first.

import type { PlantUmlSvg } from "./plantuml-svg";

export type PlantUmlSvgEntry = PlantUmlSvg;

const CACHE_LIMIT = 100;

const cache = new Map<string, PlantUmlSvgEntry>();

export function plantUmlCacheKey(serverId: string, dark: boolean, source: string): string {
  // NUL never appears in server IDs and is not valid PlantUML source, so the
  // three parts stay unambiguous.
  return `${serverId}\0${dark ? "dark" : "light"}\0${source}`;
}

export function getCachedPlantUmlSvg(
  serverId: string,
  dark: boolean,
  source: string,
): PlantUmlSvgEntry | undefined {
  const key = plantUmlCacheKey(serverId, dark, source);
  if (!cache.has(key)) {
    return undefined;
  }
  // Touch: re-insert at the tail so hot entries survive eviction.
  const value = cache.get(key)!;
  cache.delete(key);
  cache.set(key, value);
  return value;
}

export function setCachedPlantUmlSvg(
  serverId: string,
  dark: boolean,
  source: string,
  entry: PlantUmlSvgEntry,
): void {
  const key = plantUmlCacheKey(serverId, dark, source);
  cache.delete(key);
  cache.set(key, entry);
  while (cache.size > CACHE_LIMIT) {
    const oldest = cache.keys().next();
    if (oldest.done) break;
    cache.delete(oldest.value);
  }
}

/** Test hook. */
export function clearPlantUmlSvgCache(): void {
  cache.clear();
}

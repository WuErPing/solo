import { beforeEach, describe, expect, it } from "vitest";
import {
  clearMermaidSvgCache,
  getCachedMermaidSvg,
  mermaidCacheKey,
  setCachedMermaidSvg,
  type MermaidSvgEntry,
} from "./mermaid-svg-cache";

function entry(width: number): MermaidSvgEntry {
  return { xml: `<svg width="${width}"/>`, widthPx: width, heightPx: width };
}

describe("mermaid-svg-cache", () => {
  beforeEach(() => {
    clearMermaidSvgCache();
  });

  it("returns undefined for unknown keys", () => {
    expect(getCachedMermaidSvg("default", "graph TD; A-->B")).toBeUndefined();
  });

  it("stores and returns entries by theme+source", () => {
    setCachedMermaidSvg("default", "graph TD; A-->B", entry(100));
    expect(getCachedMermaidSvg("default", "graph TD; A-->B")).toEqual(entry(100));
    expect(getCachedMermaidSvg("dark", "graph TD; A-->B")).toBeUndefined();
    expect(getCachedMermaidSvg("default", "graph TD; B-->A")).toBeUndefined();
  });

  it("caches failures (null) distinctly from misses (undefined)", () => {
    setCachedMermaidSvg("default", "not a diagram", null);
    expect(getCachedMermaidSvg("default", "not a diagram")).toBeNull();
  });

  it("evicts the least-recently-used entry beyond the limit", () => {
    for (let i = 0; i < 200; i++) {
      setCachedMermaidSvg("default", `diagram-${i}`, entry(i));
    }
    // Touch the oldest entry so it survives the next eviction.
    expect(getCachedMermaidSvg("default", "diagram-0")).toEqual(entry(0));
    setCachedMermaidSvg("default", "diagram-200", entry(200));
    expect(getCachedMermaidSvg("default", "diagram-0")).toEqual(entry(0));
    expect(getCachedMermaidSvg("default", "diagram-1")).toBeUndefined();
    expect(getCachedMermaidSvg("default", "diagram-200")).toEqual(entry(200));
  });

  it("overwrites an existing key without growing the map", () => {
    setCachedMermaidSvg("default", "d", entry(1));
    setCachedMermaidSvg("default", "d", entry(2));
    expect(getCachedMermaidSvg("default", "d")).toEqual(entry(2));
  });

  it("builds keys that separate theme and source unambiguously", () => {
    expect(mermaidCacheKey("a|b", "c")).not.toBe(mermaidCacheKey("a", "b|c"));
  });
});

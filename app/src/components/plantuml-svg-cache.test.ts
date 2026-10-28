import { beforeEach, describe, expect, it } from "vitest";
import {
  clearPlantUmlSvgCache,
  getCachedPlantUmlSvg,
  plantUmlCacheKey,
  setCachedPlantUmlSvg,
  type PlantUmlSvgEntry,
} from "./plantuml-svg-cache";

function entry(width: number): PlantUmlSvgEntry {
  return { xml: `<svg width="${width}"/>`, widthPx: width, heightPx: width };
}

describe("plantuml-svg-cache", () => {
  beforeEach(() => {
    clearPlantUmlSvgCache();
  });

  it("returns undefined for unknown keys", () => {
    expect(getCachedPlantUmlSvg("s1", false, "@startuml\nA -> B\n@enduml")).toBeUndefined();
  });

  it("stores entries per server, theme and source", () => {
    const source = "@startuml\nA -> B\n@enduml";
    setCachedPlantUmlSvg("s1", true, source, entry(100));

    expect(getCachedPlantUmlSvg("s1", true, source)).toEqual(entry(100));
    expect(getCachedPlantUmlSvg("s1", false, source)).toBeUndefined();
    expect(getCachedPlantUmlSvg("s2", true, source)).toBeUndefined();
  });

  it("keeps keys unambiguous across parts", () => {
    setCachedPlantUmlSvg("s1", false, "b", entry(1));
    expect(plantUmlCacheKey("s1", false, "b")).not.toBe(plantUmlCacheKey("s1", false, "a\0b"));
  });

  it("evicts the least recently used entry", () => {
    for (let i = 0; i < 100; i += 1) {
      setCachedPlantUmlSvg("s1", false, `s${i}`, entry(i));
    }
    // Touch the oldest so it survives the next insert.
    expect(getCachedPlantUmlSvg("s1", false, "s0")).toEqual(entry(0));
    setCachedPlantUmlSvg("s1", false, "overflow", entry(999));

    expect(getCachedPlantUmlSvg("s1", false, "s0")).toEqual(entry(0));
    expect(getCachedPlantUmlSvg("s1", false, "s1")).toBeUndefined();
    expect(getCachedPlantUmlSvg("s1", false, "overflow")).toEqual(entry(999));
  });
});

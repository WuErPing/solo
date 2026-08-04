import { describe, expect, it } from "vitest";
import { getWorkingIndicatorDotStrength, WORKING_INDICATOR_OFFSETS } from "./working-indicator";

describe("getWorkingIndicatorDotStrength", () => {
  it("returns a stable triangular pulse over one cycle", () => {
    expect(getWorkingIndicatorDotStrength(0, 0)).toBe(0);
    expect(getWorkingIndicatorDotStrength(0.125, 0)).toBe(0.25);
    expect(getWorkingIndicatorDotStrength(0.25, 0)).toBe(0.5);
    expect(getWorkingIndicatorDotStrength(0.5, 0)).toBe(1);
    expect(getWorkingIndicatorDotStrength(0.75, 0)).toBe(0.5);
    expect(getWorkingIndicatorDotStrength(1, 0)).toBe(0);
  });

  it("keeps the dots phase-shifted instead of identical", () => {
    expect(new Set(WORKING_INDICATOR_OFFSETS).size).toBe(WORKING_INDICATOR_OFFSETS.length);

    const strengths = WORKING_INDICATOR_OFFSETS.map((offset) =>
      getWorkingIndicatorDotStrength(0, offset),
    );

    expect(new Set(strengths).size).toBeGreaterThan(1);
  });

  it("wraps progress cleanly across loop boundaries", () => {
    expect(getWorkingIndicatorDotStrength(1.1, 0)).toBeCloseTo(
      getWorkingIndicatorDotStrength(0.1, 0),
    );
    expect(getWorkingIndicatorDotStrength(-0.1, 0)).toBeCloseTo(
      getWorkingIndicatorDotStrength(0.9, 0),
    );
  });
});

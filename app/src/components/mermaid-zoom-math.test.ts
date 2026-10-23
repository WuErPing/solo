import { describe, expect, it } from "vitest";
import {
  clampOffset,
  clampScale,
  fitContainScale,
} from "./mermaid-zoom-math";

describe("fitContainScale", () => {
  it("shrinks a wide gantt-style diagram to the viewport width", () => {
    // 1200x400 content in a 390x800 viewport fits by width.
    expect(fitContainScale(1200, 400, 390, 800)).toBeCloseTo(390 / 1200);
  });

  it("shrinks a tall diagram to the viewport height", () => {
    expect(fitContainScale(400, 2000, 390, 800)).toBeCloseTo(800 / 2000);
  });

  it("upscales small content to fill the viewport", () => {
    expect(fitContainScale(100, 50, 390, 800)).toBeCloseTo(3.9);
  });

  it("returns 1 for degenerate input", () => {
    expect(fitContainScale(0, 100, 390, 800)).toBe(1);
    expect(fitContainScale(100, 100, 0, 800)).toBe(1);
    expect(fitContainScale(-5, 100, 390, 800)).toBe(1);
  });
});

describe("clampScale", () => {
  it("keeps values inside [min, max]", () => {
    expect(clampScale(0.5, 1, 8)).toBe(1);
    expect(clampScale(10, 1, 8)).toBe(8);
    expect(clampScale(4, 1, 8)).toBe(4);
  });
});

describe("clampOffset", () => {
  it("allows panning up to half the overflow on each side", () => {
    // 1000-wide content at scale 1 in a 390 viewport: 305px overflow per side.
    expect(clampOffset(400, 1000, 390, 1)).toBeCloseTo(305);
    expect(clampOffset(-400, 1000, 390, 1)).toBeCloseTo(-305);
    expect(clampOffset(100, 1000, 390, 1)).toBe(100);
  });

  it("grows the pan range linearly with scale", () => {
    // 1000-wide content at scale 2: (2000-390)/2 = 805.
    expect(clampOffset(1000, 1000, 390, 2)).toBeCloseTo(805);
  });

  it("keeps content that fits the viewport centered", () => {
    expect(clampOffset(50, 300, 390, 1)).toBe(0);
    expect(clampOffset(-50, 300, 390, 1)).toBe(0);
  });
});

describe("worklet safety", () => {
  // Gesture callbacks run on the UI runtime; every helper they call must be a
  // worklet, otherwise the app crashes with "Tried to synchronously call a
  // Remote Function" (observed on a release build).
  it("marks every zoom-math function as a worklet", () => {
    for (const fn of [fitContainScale, clampScale, clampOffset]) {
      expect(fn.toString()).toContain('"worklet"');
    }
  });
});

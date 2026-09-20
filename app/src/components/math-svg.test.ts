/**
 * @vitest-environment node
 *
 * The native math renderer: MathJax tex-svg → react-native-svg. Size is
 * derived synchronously from the TeX source (no async measuring), so these
 * tests pin the geometry contract the components rely on.
 */
import { describe, expect, it } from "vitest";
import { fitScale, texToMathSvg } from "./math-svg";

const COLOR = "#1f2328";
const EX = 8.5;

describe("texToMathSvg", () => {
  it("renders inline math with positive bounds and a baseline inside the box", () => {
    const data = texToMathSvg("y_{true}", false, COLOR, EX);
    expect(data).not.toBeNull();
    expect(data!.widthPx).toBeGreaterThan(0);
    expect(data!.heightPx).toBeGreaterThan(0);
    expect(data!.baselinePx).toBeGreaterThan(0);
    expect(data!.baselinePx).toBeLessThanOrEqual(data!.heightPx);
    expect(data!.xml.startsWith("<svg")).toBe(true);
    expect(data!.xml).toContain("<path");
  });

  it("renders display math and substitutes the theme color", () => {
    const data = texToMathSvg("\\frac{a}{b} + \\int_{0}^{\\infty} x^2 \\, dx", true, COLOR, EX);
    expect(data).not.toBeNull();
    expect(data!.xml).not.toContain("currentColor");
    expect(data!.xml).toContain(COLOR);
  });

  it("returns null for un-typesettable TeX", () => {
    expect(texToMathSvg("\\frac{", false, COLOR, EX)).toBeNull();
    expect(texToMathSvg("\\notarealcommand{", false, COLOR, EX)).toBeNull();
  });

  it("scales dimensions with pxPerEx", () => {
    const small = texToMathSvg("x^2", false, COLOR, 8.5);
    const large = texToMathSvg("x^2", false, COLOR, 17);
    expect(large!.widthPx).toBeGreaterThan(small!.widthPx);
    expect(large!.heightPx).toBeGreaterThan(small!.heightPx);
  });

  it("reports a downward baseline shift for inline math (MathJax vertical-align)", () => {
    const data = texToMathSvg("y_{true}", false, COLOR, EX);
    expect(data).not.toBeNull();
    // vertical-align is negative for inline math, i.e. shift down.
    expect(data!.baselineShiftPx).toBeGreaterThan(0);
  });

  it("caches results for identical inputs", () => {
    const a = texToMathSvg("\\sum_{k=1}^{N} x_k", false, COLOR, EX);
    const b = texToMathSvg("\\sum_{k=1}^{N} x_k", false, COLOR, EX);
    expect(a).toBe(b);
  });

  it("fitScale shrinks only overflowing formulas", () => {
    expect(fitScale(100, 0)).toBe(1);
    expect(fitScale(100, 300)).toBe(1);
    expect(fitScale(600, 300)).toBeCloseTo(0.5);
  });
});

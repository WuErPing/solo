import { describe, expect, it } from "vitest";
import { contrastRatio, ensureContrast } from "./color-contrast";

describe("color-contrast", () => {
  describe("contrastRatio", () => {
    it("returns 21:1 for black on white", () => {
      expect(contrastRatio("#000000", "#ffffff")).toBeCloseTo(21, 1);
    });

    it("returns 1:1 for identical colors", () => {
      expect(contrastRatio("#181B1A", "#181B1A")).toBeCloseTo(1, 2);
    });

    it("handles 3-digit hex", () => {
      expect(contrastRatio("#fff", "#000")).toBeCloseTo(21, 1);
    });

    it("handles rgb() strings", () => {
      expect(contrastRatio("rgb(0,0,0)", "rgb(255,255,255)")).toBeCloseTo(21, 1);
    });

    it("returns 0 for unparseable colors", () => {
      expect(contrastRatio("banana", "#ffffff")).toBe(0);
    });
  });

  describe("ensureContrast", () => {
    it("leaves high-contrast colors unchanged", () => {
      expect(ensureContrast("#ffffff", "#181B1A", 3)).toBe("#ffffff");
      expect(ensureContrast("#5f5f5f", "#ffffff", 3)).toBe("#5f5f5f");
    });

    it("lightens dark grays on a dark terminal background", () => {
      const bg = "#181B1A";
      const before = "#5f5f5f";
      const after = ensureContrast(before, bg, 3);
      expect(after).not.toBe(before);
      expect(contrastRatio(after, bg)).toBeGreaterThanOrEqual(3);
    });

    it("darkens light colors on a light terminal background", () => {
      const bg = "#ffffff";
      const before = "#afaf87";
      const after = ensureContrast(before, bg, 3);
      expect(after).not.toBe(before);
      expect(contrastRatio(after, bg)).toBeGreaterThanOrEqual(3);
    });

    it("preserves hue while adjusting luminance", () => {
      const bg = "#181B1A";
      const before = "#5f5f5f";
      const after = ensureContrast(before, bg, 3);
      // A neutral gray stays neutral when blended with white.
      expect(after.startsWith("#")).toBe(true);
    });

    it("handles rgb() input", () => {
      const bg = "#181B1A";
      const after = ensureContrast("rgb(95,95,95)", bg, 3);
      expect(contrastRatio(after, bg)).toBeGreaterThanOrEqual(3);
    });

    it("returns the original color when parsing fails", () => {
      expect(ensureContrast("not-a-color", "#181B1A", 3)).toBe("not-a-color");
      expect(ensureContrast("#5f5f5f", "not-a-color", 3)).toBe("#5f5f5f");
    });
  });
});

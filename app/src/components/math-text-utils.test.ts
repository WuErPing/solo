import { describe, expect, it } from "vitest";
import { renderKatexHtml } from "./math-text-utils";

describe("renderKatexHtml", () => {
  it("renders valid tex into KaTeX markup", () => {
    const html = renderKatexHtml("x^2", false);
    expect(html).toContain("katex");
  });

  it("renders display mode", () => {
    const html = renderKatexHtml("\\frac{a}{b}", true);
    expect(html).toContain("katex-display");
  });

  it("does not throw on invalid tex (throwOnError is disabled)", () => {
    expect(() => renderKatexHtml("\\notarealmacro{", false)).not.toThrow();
  });

  it("degrades to escaped text when KaTeX fails hard", () => {
    const html = renderKatexHtml("a\\textbf{#}b", true);
    expect(typeof html).toBe("string");
    expect(html.length).toBeGreaterThan(0);
  });
});

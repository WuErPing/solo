import { describe, expect, it, vi } from "vitest";

vi.mock("react-native-webview", () => ({
  WebView: () => null,
}));

describe("isSvgContent", () => {
  it("returns true for valid SVG content", async () => {
    const { isSvgContent } = await import("./svg-preview");
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    expect(isSvgContent(svg)).toBe(true);
  });

  it("returns true for SVG with whitespace", async () => {
    const { isSvgContent } = await import("./svg-preview");
    const svg = '  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">  </svg>  ';
    expect(isSvgContent(svg)).toBe(true);
  });

  it("returns false for non-SVG content", async () => {
    const { isSvgContent } = await import("./svg-preview");
    expect(isSvgContent("hello world")).toBe(false);
    expect(isSvgContent("<html><body>test</body></html>")).toBe(false);
    expect(isSvgContent("")).toBe(false);
  });

  it("returns false for SVG without namespace", async () => {
    const { isSvgContent } = await import("./svg-preview");
    expect(isSvgContent('<svg viewBox="0 0 100 100"></svg>')).toBe(false);
  });
});

describe("sanitizeSvgContent", () => {
  it("removes script tags", async () => {
    const { sanitizeSvgContent } = await import("./svg-preview");
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script><circle/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("<script>");
    expect(sanitized).toContain("<circle");
  });

  it("removes event handlers", async () => {
    const { sanitizeSvgContent } = await import("./svg-preview");
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><circle/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("onload");
    expect(sanitized).toContain("<circle");
  });

  it("removes javascript: URLs", async () => {
    const { sanitizeSvgContent } = await import("./svg-preview");
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><a xlink:href="javascript:alert(1)"><circle/></a></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("javascript:");
  });

  it("preserves safe SVG content", async () => {
    const { sanitizeSvgContent } = await import("./svg-preview");
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).toContain("circle");
    expect(sanitized).toContain('fill="blue"');
  });
});

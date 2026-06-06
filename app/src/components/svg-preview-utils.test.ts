import { describe, expect, it } from "vitest";
import { isSvgContent, sanitizeSvgContent, generateSvgHtml } from "./svg-preview-utils";

describe("isSvgContent", () => {
  it("returns true for valid SVG content", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    expect(isSvgContent(svg)).toBe(true);
  });

  it("returns true for SVG with whitespace", () => {
    const svg = '  <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">  </svg>  ';
    expect(isSvgContent(svg)).toBe(true);
  });

  it("returns false for non-SVG content", () => {
    expect(isSvgContent("hello world")).toBe(false);
    expect(isSvgContent("<html><body>test</body></html>")).toBe(false);
    expect(isSvgContent("")).toBe(false);
  });

  it("returns false for SVG without namespace", () => {
    expect(isSvgContent('<svg viewBox="0 0 100 100"></svg>')).toBe(false);
  });
});

describe("sanitizeSvgContent", () => {
  it("removes script tags", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script><circle/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("<script>");
    expect(sanitized).toContain("<circle");
  });

  it("removes event handlers", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><circle/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("onload");
    expect(sanitized).toContain("<circle");
  });

  it("removes javascript: URLs", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><a xlink:href="javascript:alert(1)"><circle/></a></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).not.toContain("javascript:");
  });

  it("preserves safe SVG content", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>';
    const sanitized = sanitizeSvgContent(svg);
    expect(sanitized).toContain("circle");
    expect(sanitized).toContain('fill="blue"');
  });
});

describe("generateSvgHtml", () => {
  it("generates HTML with proper viewport for mobile rendering", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain('width=device-width');
    expect(html).toContain('initial-scale=1.0');
  });

  it("generates HTML with SVG filling viewport", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain("width: 100%");
    expect(html).toContain("height: 100%");
  });

  it("generates HTML with proper body styling for mobile", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain("margin: 0");
    expect(html).toContain("padding: 0");
  });

  it("adds explicit width and height to SVG if missing and no viewBox", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain('width="100%"');
    expect(html).toContain('height="100%"');
  });

  it("does not add width/height if viewBox exists", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).not.toContain('width="100%"');
    expect(html).not.toContain('height="100%"');
  });

  it("generates HTML with SVG using 100% width", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain("width: 100%");
  });

  it("generates HTML with pinch-to-zoom support", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain('user-scalable=yes');
    expect(html).toContain('maximum-scale=5.0');
  });

  it("generates HTML with touch-action for zoom", () => {
    const svg = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40"/></svg>';
    const html = generateSvgHtml(svg, false);
    expect(html).toContain('touch-action: pinch-zoom');
  });
});

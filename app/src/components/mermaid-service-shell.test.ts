import { describe, expect, it } from "vitest";
import { generateMermaidServiceShellHtml } from "./mermaid-service-shell";

const SENTINEL_JS = "/*mermaid-test-bundle*/";

describe("generateMermaidServiceShellHtml", () => {
  it("inlines the given mermaid bundle instead of loading from a CDN", () => {
    const html = generateMermaidServiceShellHtml(SENTINEL_JS);
    expect(html).toContain(SENTINEL_JS);
    expect(html).not.toContain("cdn.jsdelivr.net");
    expect(html).not.toContain("<script src=");
  });

  it("exposes the SVG render entrypoint and announces readiness", () => {
    const html = generateMermaidServiceShellHtml(SENTINEL_JS);
    expect(html).toContain("__renderToSvg");
    expect(html).toContain("mermaid:svg");
    expect(html).toContain("mermaid:service-ready");
    expect(html).toContain("ReactNativeWebView.postMessage");
  });

  it("inlines computed styles and strips style blocks for react-native-svg", () => {
    const html = generateMermaidServiceShellHtml(SENTINEL_JS);
    expect(html).toContain("getComputedStyle");
    expect(html).toContain("querySelectorAll('style')");
    expect(html).toContain("htmlLabels: false");
  });

  it("does not interpolate any diagram source", () => {
    const html = generateMermaidServiceShellHtml(SENTINEL_JS);
    expect(html).not.toContain("graph TD");
  });

  it("supports a vectorOnly mode that skips canvas rasterization", () => {
    const html = generateMermaidServiceShellHtml(SENTINEL_JS);
    expect(html).toContain("function (id, source, theme, vectorOnly)");
    // The early return must sit before the canvas/Image raster path.
    expect(html.indexOf("vectorOnly === true")).toBeGreaterThan(-1);
    expect(html.indexOf("vectorOnly === true")).toBeLessThan(
      html.indexOf("canvas.toDataURL"),
    );
  });
});

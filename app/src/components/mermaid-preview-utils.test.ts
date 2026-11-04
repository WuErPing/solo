import { describe, expect, it } from "vitest";
import { escapeHtml, generateMermaidShellHtml } from "./mermaid-preview-utils";

describe("escapeHtml", () => {
  it("escapes less-than and greater-than characters", () => {
    expect(escapeHtml("<script>")).toBe("&lt;script&gt;");
  });

  it("escapes ampersands", () => {
    expect(escapeHtml("a & b")).toBe("a &amp; b");
  });

  it("escapes double and single quotes", () => {
    expect(escapeHtml('"hello"')).toBe("&quot;hello&quot;");
    expect(escapeHtml("'hello'")).toBe("&#039;hello&#039;");
  });

  it("leaves plain text without special characters unchanged", () => {
    expect(escapeHtml("graph TD; A -- B;")).toBe("graph TD; A -- B;");
  });
});

describe("generateMermaidShellHtml", () => {
  it("returns a static HTML document that loads mermaid once", () => {
    const html = generateMermaidShellHtml();
    expect(html.startsWith("<!DOCTYPE html>")).toBe(true);
    expect(html).toContain("mermaid.min.js");
    expect(html).toContain('id="mermaid-graph"');
  });

  it("exposes an incremental render entrypoint and message listener", () => {
    const html = generateMermaidShellHtml();
    expect(html).toContain("__renderMermaid");
    expect(html).toContain("addEventListener('message'");
    expect(html).toContain("mermaid:render");
    expect(html).toContain("mermaid:ready");
  });

  it("caches rendered output and only re-initializes on theme change", () => {
    const html = generateMermaidShellHtml();
    expect(html).toContain("new Map()");
    expect(html).toContain("cache.has(key)");
    expect(html).toContain("mermaid.initialize");
  });

  it("is identical across calls so the document is never rebuilt per diagram", () => {
    expect(generateMermaidShellHtml()).toBe(generateMermaidShellHtml());
  });

  it("pins gantt charts to the pane width so they scale with the device", () => {
    const html = generateMermaidShellHtml();
    // mermaid v10 otherwise sizes gantt charts from the task list alone
    // (~226px), ignoring the container.
    expect(html).toContain("gantt: { useWidth: useWidth }");
    expect(html).toContain("el.clientWidth");
    // The rendered width is part of the cache key, so a resized pane re-renders.
    expect(html).toContain("theme + '\\u0000' + useWidth + '\\u0000' + source");
  });

  it("dedupes duplicate render requests and discards stale results", () => {
    const html = generateMermaidShellHtml();
    // The host posts each request twice (iframe load + shell ready); without
    // this guard two concurrent mermaid.render calls flash a half-drawn diagram.
    expect(html).toContain("inFlightKey");
    expect(html).toContain("renderedKey");
    expect(html).toContain("latestKey");
    expect(html).toContain("key === renderedKey || key === inFlightKey");
    expect(html).toContain("key !== latestKey");
  });

  it("keeps mermaid's temporary measurement div off-screen", () => {
    const html = generateMermaidShellHtml();
    // mermaid v10 appends an unstyled div to body while rendering; visible in
    // normal flow it flashes and stretches the document height.
    expect(html).toContain("body > div:not(#mermaid-graph)");
    expect(html).toContain("left: -100000px");
  });

  it("reports the document height so the host can size the container", () => {
    const html = generateMermaidShellHtml();
    expect(html).toContain("document.body.scrollHeight");
    expect(html).toContain("mermaid:rendered");
  });

  it("does not interpolate any mermaid source into the document", () => {
    const html = generateMermaidShellHtml();
    // Source is pushed in at runtime (postMessage / injectJavaScript), so the
    // static shell must never embed diagram text — guards against injection.
    expect(html).not.toContain("graph TD");
    expect(html).not.toContain("<img src=x");
  });
});

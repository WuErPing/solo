import { describe, expect, it } from "vitest";
import { escapeHtml, generateMermaidHtml } from "./mermaid-preview-utils";

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

describe("generateMermaidHtml", () => {
  it("returns a valid HTML document containing the mermaid source", () => {
    const html = generateMermaidHtml("graph TD; A -- B;");
    expect(html.startsWith("<!DOCTYPE html>")).toBe(true);
    expect(html).toContain("graph TD; A -- B;");
    expect(html).toContain("mermaid.min.js");
    expect(html).toContain('id="mermaid-graph"');
    expect(html).toContain("mermaid.initialize");
  });

  it("escapes HTML in the mermaid source to prevent injection", () => {
    const malicious = '<img src=x onerror="alert(1)">';
    const html = generateMermaidHtml(malicious);
    expect(html).not.toContain('<img src=x onerror="alert(1)">');
    expect(html).toContain("&lt;img src=x onerror=&quot;alert(1)&quot;&gt;");
  });

  it("uses dark theme and dark background when isDark is true", () => {
    const html = generateMermaidHtml("graph TD; A -- B;", true);
    expect(html).toContain("theme: 'dark'");
    expect(html).toContain("#0d1117");
  });

  it("uses default theme and light background when isDark is false", () => {
    const html = generateMermaidHtml("graph TD; A -- B;", false);
    expect(html).toContain("theme: 'default'");
    expect(html).toContain("#ffffff");
  });

  it("defaults to light theme when isDark is omitted", () => {
    const html = generateMermaidHtml("graph TD; A -- B;");
    expect(html).toContain("theme: 'default'");
    expect(html).toContain("#ffffff");
  });

  it("includes a render script that trims whitespace and catches errors", () => {
    const html = generateMermaidHtml("  graph TD; A -- B;  ");
    expect(html).toContain("el.textContent.trim()");
    expect(html).toContain("catch");
  });
});

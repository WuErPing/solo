/**
 * @vitest-environment node
 *
 * Guards the generated KaTeX assets: the native math WebView must render fully
 * offline (a remote stylesheet/fonts is what made formulas flicker on slow or
 * unstable networks), and the assets must be regenerated whenever the katex
 * version changes, otherwise the CSS/font metrics drift from the KaTeX JS that
 * renders the markup.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { KATEX_ASSETS_VERSION, KATEX_CSS } from "./katex-assets";

const require = createRequire(join(__dirname, "..", "..", "package.json"));

describe("katex-assets", () => {
  it("matches the installed katex version", () => {
    const pkg = JSON.parse(readFileSync(require.resolve("katex/package.json"), "utf-8"));
    expect(KATEX_ASSETS_VERSION).toBe(pkg.version);
  });

  it("inlines the stylesheet with woff2 fonts and no file references", () => {
    expect(KATEX_CSS).toContain(".katex");
    expect(KATEX_CSS).toContain("@font-face");
    expect(KATEX_CSS).toContain("data:font/woff2;base64,");
    expect(KATEX_CSS).not.toContain("url(fonts/");
  });

  it("keeps every @font-face from the upstream stylesheet", () => {
    const distDir = join(dirname(require.resolve("katex/package.json")), "dist");
    const upstream = readFileSync(join(distDir, "katex.min.css"), "utf-8");
    const upstreamCount = (upstream.match(/@font-face/g) ?? []).length;
    const inlinedCount = (KATEX_CSS.match(/@font-face/g) ?? []).length;
    expect(inlinedCount).toBe(upstreamCount);
  });
});

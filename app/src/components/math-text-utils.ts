import katex from "katex";
import { escapeHtml } from "./mermaid-preview-utils";
import { KATEX_CSS } from "./katex-assets";

export { KATEX_CSS };

/**
 * Web renderer only: KaTeX markup embedded via dangerouslySetInnerHTML with
 * the inlined stylesheet from katex-assets.ts. Native renders MathJax SVG
 * instead (see math-svg.ts) — no HTML/CSS pipeline there.
 */
export function renderKatexHtml(tex: string, display: boolean): string {
  try {
    return katex.renderToString(tex, {
      displayMode: display,
      throwOnError: false,
      output: "html",
    });
  } catch {
    return escapeHtml(tex);
  }
}


// TeX → SVG rendering on the native side. MathJax v3 (tex-svg) runs purely in
// JS via its lite DOM adaptor, and react-native-svg paints the resulting paths
// as ordinary native views. Unlike the previous per-formula WebView, SVG needs
// no async measuring and no separate surface, so formulas paint synchronously
// with deterministic size — the source of the Android flicker is gone.
//
// mathjax-full is required lazily on first use so app startup never parses it.
/* eslint-disable @typescript-eslint/no-require-imports -- lazy requires keep mathjax-full out of startup */

export interface MathSvgData {
  /** Complete `<svg …>…</svg>` document ready for `SvgXml`. */
  xml: string;
  widthPx: number;
  heightPx: number;
  /** Distance from the box top to the TeX baseline, in px. */
  baselinePx: number;
  /** Downward shift (px) that aligns the box with the host text baseline. */
  baselineShiftPx: number;
}

interface MathJaxLite {
  convert(tex: string, options: { display: boolean }): unknown;
  outerHTML(node: unknown): string;
}

let mathjaxDoc: MathJaxLite | null | undefined;

function getMathJaxDoc(): MathJaxLite | null {
  if (mathjaxDoc !== undefined) {
    return mathjaxDoc;
  }
  try {
    const { mathjax } = require("mathjax-full/js/mathjax.js");
    const { TeX } = require("mathjax-full/js/input/tex.js");
    const { SVG } = require("mathjax-full/js/output/svg.js");
    const { liteAdaptor } = require("mathjax-full/js/adaptors/liteAdaptor.js");
    const { RegisterHTMLHandler } = require("mathjax-full/js/handlers/html.js");
    const adaptor = liteAdaptor();
    RegisterHTMLHandler(adaptor);
    const doc = mathjax.document("", {
      InputJax: new TeX({ packages: ["base", "ams"] }),
      // fontCache 'none' inlines every glyph path — slower per render, but the
      // results are cached below and SvgXml needs no external <defs>.
      OutputJax: new SVG({ fontCache: "none" }),
    });
    mathjaxDoc = {
      convert: (tex, options) => doc.convert(tex, options),
      outerHTML: (node) => adaptor.outerHTML(node),
    };
  } catch {
    mathjaxDoc = null;
  }
  return mathjaxDoc;
}

const SVG_NS = 'xmlns="http://www.w3.org/2000/svg"';
const CACHE_LIMIT = 300;
const cache = new Map<string, MathSvgData | null>();

/** Shrink factor so a display formula never overflows its container. */
export function fitScale(naturalWidthPx: number, containerWidthPx: number): number {
  if (containerWidthPx <= 0 || naturalWidthPx <= containerWidthPx) {
    return 1;
  }
  return containerWidthPx / naturalWidthPx;
}

function attr(tag: string, name: string): string | null {
  const match = tag.match(new RegExp(`${name}="([^"]*)"`));
  return match ? match[1] : null;
}

function parseSvgDocument(outer: string, color: string, pxPerEx: number): MathSvgData | null {
  const start = outer.indexOf("<svg");
  if (start < 0) return null;
  // MathJax does not throw on malformed TeX — it renders an <merror> box
  // instead. Treat that as failure so the caller can degrade to raw TeX text.
  if (outer.includes("data-mjx-error")) return null;
  const tagEnd = outer.indexOf(">", start);
  const close = outer.lastIndexOf("</svg>");
  if (tagEnd < 0 || close < 0 || close <= tagEnd) return null;

  const svgTag = outer.slice(start, tagEnd + 1);
  const widthEx = Number(attr(svgTag, "width")?.replace(/ex$/, ""));
  const heightEx = Number(attr(svgTag, "height")?.replace(/ex$/, ""));
  const viewBox = (attr(svgTag, "viewBox") ?? "").trim().split(/\s+/).map(Number);
  if (!Number.isFinite(widthEx) || !Number.isFinite(heightEx) || viewBox.length !== 4) {
    return null;
  }
  const [vbX, vbY, vbW, vbH] = viewBox;
  if (viewBox.some((n) => !Number.isFinite(n)) || vbW <= 0 || vbH <= 0 || heightEx <= 0) {
    return null;
  }

  const widthPx = Math.max(1, Math.round(widthEx * pxPerEx));
  const heightPx = Math.max(1, Math.round(heightEx * pxPerEx));
  const baselinePx = ((-vbY) / vbH) * heightPx;
  // MathJax declares how the box aligns with the surrounding text baseline
  // (e.g. vertical-align: -0.464ex). A negative value means "shift down"; the
  // attachment layout in RN leaves the box too high by exactly this amount.
  const verticalAlign = svgTag.match(/vertical-align:\s*(-?[\d.]+)ex/);
  const baselineShiftPx = verticalAlign ? -Number(verticalAlign[1]) * pxPerEx : 0;
  // MathJax paints with currentColor; swap in the theme color so the glyphs
  // match the surrounding markdown text in both light and dark themes.
  const inner = outer.slice(tagEnd + 1, close).split("currentColor").join(color);

  return {
    xml: `<svg ${SVG_NS} viewBox="${vbX} ${vbY} ${vbW} ${vbH}">${inner}</svg>`,
    widthPx,
    heightPx,
    baselinePx,
    baselineShiftPx,
  };
}

/**
 * Render TeX to SVG data sized for React Native, or `null` when the input
 * cannot be typeset. Results are cached by (tex, display, color, scale).
 */
export function texToMathSvg(
  tex: string,
  display: boolean,
  color: string,
  pxPerEx: number,
): MathSvgData | null {
  const key = `${display ? "b" : "i"}|${pxPerEx}|${color}|${tex}`;
  if (cache.has(key)) {
    return cache.get(key) ?? null;
  }

  let result: MathSvgData | null = null;
  const doc = getMathJaxDoc();
  if (doc) {
    try {
      result = parseSvgDocument(doc.outerHTML(doc.convert(tex, { display })), color, pxPerEx);
    } catch {
      result = null;
    }
  }
  if (cache.size >= CACHE_LIMIT) {
    cache.clear();
  }
  cache.set(key, result);
  return result;
}

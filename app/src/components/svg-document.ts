// Parses an SVG document into something react-native-svg can paint inline.
// Unlike the PlantUML normalizer (plantuml-svg.ts), the root element's own
// attributes are preserved — hand-authored diagrams carry font-family and
// fill defaults there — while width/height/style are dropped so the caller
// decides the rendered size.

import { sanitizeSvgContent } from "./svg-preview-utils";

export interface SvgDocument {
  /** Complete `<svg …>…</svg>` document ready for `SvgXml`. */
  xml: string;
  /** Natural size of the drawing in CSS px. */
  widthPx: number;
  heightPx: number;
}

const SVG_NS = "http://www.w3.org/2000/svg";
const SIZING_ATTRS = new Set(["width", "height", "style", "xmlns"]);

function rootAttributes(tag: string): Map<string, string> {
  const attrs = new Map<string, string>();
  const pattern = /([\w:.-]+)\s*=\s*(?:"([^"]*)"|'([^']*)')/g;
  for (const match of tag.matchAll(pattern)) {
    const name = match[1];
    if (name) {
      attrs.set(name, match[2] ?? match[3] ?? "");
    }
  }
  return attrs;
}

function pxValue(raw: string | undefined): number | null {
  if (!raw) {
    return null;
  }
  // Percentages and em/ex units have no intrinsic pixel size.
  if (!/^[\d.]/.test(raw.trim())) {
    return null;
  }
  const value = Number(raw.trim().replace(/px$/i, ""));
  return Number.isFinite(value) && value > 0 ? value : null;
}

function buildKeptAttributes(attrs: Map<string, string>): string {
  const kept: string[] = [];
  for (const [name, value] of attrs) {
    if (!SIZING_ATTRS.has(name.toLowerCase())) {
      kept.push(`${name}="${value.replace(/"/g, "&quot;")}"`);
    }
  }
  return kept.length > 0 ? ` ${kept.join(" ")}` : "";
}

function viewBoxOf(attrs: Map<string, string>): [number, number, number, number] | null {
  const parts = (attrs.get("viewBox") ?? attrs.get("viewbox") ?? "")
    .trim()
    .split(/[\s,]+/)
    .map(Number);
  if (
    parts.length !== 4 ||
    !parts.every((n) => Number.isFinite(n)) ||
    parts[2] <= 0 ||
    parts[3] <= 0
  ) {
    return null;
  }
  return [parts[0], parts[1], parts[2], parts[3]];
}

/**
 * Parse SVG markup, or return `null` when it is not a usable SVG document or
 * its natural size cannot be determined.
 */
export function parseSvgDocument(raw: string): SvgDocument | null {
  if (typeof raw !== "string" || raw.trim().length === 0) {
    return null;
  }
  const cleaned = sanitizeSvgContent(raw).replace(/<\?[\s\S]*?\?>/g, "");

  const start = cleaned.indexOf("<svg");
  const tagEnd = cleaned.indexOf(">", start);
  const close = cleaned.lastIndexOf("</svg>");
  if (start < 0 || tagEnd < 0 || close <= tagEnd) {
    return null;
  }

  const attrs = rootAttributes(cleaned.slice(start, tagEnd + 1));
  const viewBox = viewBoxOf(attrs);
  const widthPx = pxValue(attrs.get("width")) ?? (viewBox ? viewBox[2] : null);
  const heightPx = pxValue(attrs.get("height")) ?? (viewBox ? viewBox[3] : null);
  if (widthPx == null || heightPx == null) {
    return null;
  }

  const kept = buildKeptAttributes(attrs);
  const box = viewBox
    ? `${viewBox[0]} ${viewBox[1]} ${viewBox[2]} ${viewBox[3]}`
    : `0 0 ${widthPx} ${heightPx}`;

  return {
    xml: `<svg xmlns="${SVG_NS}" viewBox="${box}"${kept}>${cleaned.slice(tagEnd + 1, close)}</svg>`,
    widthPx: Math.round(widthPx),
    heightPx: Math.round(heightPx),
  };
}

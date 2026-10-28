// Normalizes the SVG markup produced by the daemon-side PlantUML renderer into
// something react-native-svg can paint: drop the leading processing
// instructions, keep the inner figure markup (PlantUML inlines colors in
// element attributes), and rebuild the root so the server's white background
// style never fights the app theme.

export interface PlantUmlSvg {
  /** Complete `<svg …>…</svg>` document ready for `SvgXml`. */
  xml: string;
  /** Natural size of the diagram in CSS px. */
  widthPx: number;
  heightPx: number;
}

const SVG_NS = 'xmlns="http://www.w3.org/2000/svg"';

function attr(tag: string, name: string): string | null {
  const match = tag.match(new RegExp(`${name}="([^"]*)"`));
  return match ? match[1] : null;
}

function pxValue(raw: string | null): number | null {
  if (!raw) {
    return null;
  }
  const value = Number(raw.trim().replace(/px$/, ""));
  return Number.isFinite(value) && value > 0 ? value : null;
}

/**
 * Parse PlantUML SVG output (e.g. `<?plantuml 1.2026.2?><svg width="108px" …>`)
 * or `null` when the markup is not a usable SVG document.
 */
export function parsePlantUmlSvg(raw: string): PlantUmlSvg | null {
  if (typeof raw !== "string" || raw.length === 0) {
    return null;
  }
  const cleaned = raw
    // PlantUML emits `<?plantuml 1.x?>` and sometimes `<?plantuml-src …?>`.
    .replace(/<\?[\s\S]*?\?>/g, "")
    .replace(/<script\b[\s\S]*?<\/script>/gi, "");

  const start = cleaned.indexOf("<svg");
  if (start < 0) {
    return null;
  }
  const tagEnd = cleaned.indexOf(">", start);
  const close = cleaned.lastIndexOf("</svg>");
  if (tagEnd < 0 || close < 0 || close <= tagEnd) {
    return null;
  }

  const svgTag = cleaned.slice(start, tagEnd + 1);
  const viewBox = (attr(svgTag, "viewBox") ?? "").trim().split(/\s+/).map(Number);
  const hasViewBox =
    viewBox.length === 4 &&
    viewBox.every((n) => Number.isFinite(n)) &&
    viewBox[2] > 0 &&
    viewBox[3] > 0;

  const widthPx = pxValue(attr(svgTag, "width")) ?? (hasViewBox ? viewBox[2] : null);
  const heightPx = pxValue(attr(svgTag, "height")) ?? (hasViewBox ? viewBox[3] : null);
  if (widthPx == null || heightPx == null) {
    return null;
  }

  const box = hasViewBox
    ? `${viewBox[0]} ${viewBox[1]} ${viewBox[2]} ${viewBox[3]}`
    : `0 0 ${widthPx} ${heightPx}`;
  const inner = cleaned.slice(tagEnd + 1, close);

  return {
    xml: `<svg ${SVG_NS} viewBox="${box}">${inner}</svg>`,
    widthPx: Math.round(widthPx),
    heightPx: Math.round(heightPx),
  };
}

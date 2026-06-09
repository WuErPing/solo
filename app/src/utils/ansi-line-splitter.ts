import type { AnsiSegment } from "./ansi-parser";

/**
 * Splits an array of AnsiSegments into per-line arrays by splitting
 * segment text on `\n` boundaries. Style references are preserved (not copied).
 */
export function splitSegmentsByLine(segments: AnsiSegment[]): AnsiSegment[][] {
  if (segments.length === 0) return [];
  const lines: AnsiSegment[][] = [[]];
  for (const seg of segments) {
    const parts = seg.text.split("\n");
    for (let i = 0; i < parts.length; i++) {
      if (i > 0) {
        lines.push([]);
      }
      if (parts[i].length > 0) {
        lines[lines.length - 1].push({ text: parts[i], style: seg.style });
      }
    }
  }
  return lines;
}

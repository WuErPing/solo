import { describe, it, expect } from "vitest";
import { splitSegmentsByLine } from "./ansi-line-splitter";
import type { AnsiSegment, AnsiStyle } from "./ansi-parser";

const defaultStyle: AnsiStyle = {
  bold: false,
  dim: false,
  italic: false,
  underline: false,
  strikethrough: false,
  inverse: false,
  fg: null,
  bg: null,
};

const redStyle: AnsiStyle = {
  ...defaultStyle,
  fg: { type: "basic", index: 1 },
};

const blueStyle: AnsiStyle = {
  ...defaultStyle,
  fg: { type: "basic", index: 4 },
};

function seg(text: string, style: AnsiStyle = defaultStyle): AnsiSegment {
  return { text, style };
}

describe("splitSegmentsByLine", () => {
  it("returns empty array for empty input", () => {
    expect(splitSegmentsByLine([])).toEqual([]);
  });

  it("returns single line for segment with no newlines", () => {
    expect(splitSegmentsByLine([seg("hello")])).toEqual([[seg("hello")]]);
  });

  it("splits single segment on newline", () => {
    const result = splitSegmentsByLine([seg("line1\nline2", redStyle)]);
    expect(result).toEqual([
      [seg("line1", redStyle)],
      [seg("line2", redStyle)],
    ]);
  });

  it("splits segment spanning newline, preserving style", () => {
    const result = splitSegmentsByLine([seg("a\nb\nc", redStyle)]);
    expect(result).toEqual([
      [seg("a", redStyle)],
      [seg("b", redStyle)],
      [seg("c", redStyle)],
    ]);
  });

  it("handles segment boundary coinciding with newline", () => {
    const result = splitSegmentsByLine([seg("a", redStyle), seg("\nb", blueStyle)]);
    expect(result).toEqual([
      [seg("a", redStyle)],
      [seg("b", blueStyle)],
    ]);
  });

  it("produces empty final line for trailing newline", () => {
    const result = splitSegmentsByLine([seg("hello\n")]);
    expect(result).toEqual([[seg("hello")], []]);
  });

  it("handles consecutive newlines as empty lines", () => {
    const result = splitSegmentsByLine([seg("a\n\nb")]);
    expect(result).toEqual([[seg("a")], [], [seg("b")]]);
  });

  it("handles multiple segments on same line", () => {
    const result = splitSegmentsByLine([
      seg("hello ", redStyle),
      seg("world", blueStyle),
    ]);
    expect(result).toEqual([[seg("hello ", redStyle), seg("world", blueStyle)]]);
  });

  it("handles mixed multi-line segments", () => {
    const result = splitSegmentsByLine([
      seg("a", redStyle),
      seg("b\nc", blueStyle),
      seg("d", redStyle),
    ]);
    expect(result).toEqual([
      [seg("a", redStyle), seg("b", blueStyle)],
      [seg("c", blueStyle), seg("d", redStyle)],
    ]);
  });

  it("preserves style references without copying", () => {
    const style: AnsiStyle = { ...defaultStyle, bold: true };
    const input = [seg("x\ny", style)];
    const result = splitSegmentsByLine(input);
    expect(result[0][0].style).toBe(style);
    expect(result[1][0].style).toBe(style);
  });
});

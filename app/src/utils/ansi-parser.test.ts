import { describe, expect, it } from "vitest";
import { parseAnsi, type AnsiSegment, type AnsiStyle } from "./ansi-parser";

function defaultStyle(): AnsiStyle {
  return { bold: false, dim: false, italic: false, underline: false, strikethrough: false, inverse: false, fg: null, bg: null };
}

function textOf(segments: AnsiSegment[]): string {
  return segments.map((s) => s.text).join("");
}

describe("parseAnsi", () => {
  describe("plain text", () => {
    it("returns empty array for empty string", () => {
      expect(parseAnsi("")).toEqual([]);
    });

    it("returns single segment for plain text", () => {
      const segs = parseAnsi("hello world");
      expect(segs).toHaveLength(1);
      expect(segs[0]!.text).toBe("hello world");
      expect(segs[0]!.style).toEqual(defaultStyle());
    });

    it("preserves newlines", () => {
      const segs = parseAnsi("line1\nline2");
      expect(textOf(segs)).toBe("line1\nline2");
    });
  });

  describe("basic foreground colors", () => {
    it("parses red foreground", () => {
      const segs = parseAnsi("\x1b[31mred\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
      expect(segs[0]!.text).toBe("red");
    });

    it("parses green foreground", () => {
      const segs = parseAnsi("\x1b[32mgreen\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 2 });
    });
  });

  describe("bright colors", () => {
    it("parses bright red (90-97 range)", () => {
      const segs = parseAnsi("\x1b[91mbright red\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 9 });
    });

    it("parses bright black background (100-107 range)", () => {
      const segs = parseAnsi("\x1b[100mbg\x1b[0m");
      expect(segs[0]!.style.bg).toEqual({ type: "basic", index: 8 });
    });
  });

  describe("256-color mode", () => {
    it("parses SGR 38;5;N foreground", () => {
      const segs = parseAnsi("\x1b[38;5;196mcolor\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "palette", index: 196 });
    });

    it("parses SGR 48;5;N background", () => {
      const segs = parseAnsi("\x1b[48;5;232mbg\x1b[0m");
      expect(segs[0]!.style.bg).toEqual({ type: "palette", index: 232 });
    });
  });

  describe("true color RGB", () => {
    it("parses SGR 38;2;R;G;B foreground", () => {
      const segs = parseAnsi("\x1b[38;2;255;128;0morange\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "rgb", r: 255, g: 128, b: 0 });
    });

    it("parses SGR 48;2;R;G;B background", () => {
      const segs = parseAnsi("\x1b[48;2;0;0;128mbg\x1b[0m");
      expect(segs[0]!.style.bg).toEqual({ type: "rgb", r: 0, g: 0, b: 128 });
    });
  });

  describe("text attributes", () => {
    it("parses bold", () => {
      const segs = parseAnsi("\x1b[1mbold\x1b[0m");
      expect(segs[0]!.style.bold).toBe(true);
    });

    it("parses dim", () => {
      const segs = parseAnsi("\x1b[2mdim\x1b[0m");
      expect(segs[0]!.style.dim).toBe(true);
    });

    it("parses italic", () => {
      const segs = parseAnsi("\x1b[3mitalic\x1b[0m");
      expect(segs[0]!.style.italic).toBe(true);
    });

    it("parses underline", () => {
      const segs = parseAnsi("\x1b[4munderline\x1b[0m");
      expect(segs[0]!.style.underline).toBe(true);
    });

    it("parses inverse", () => {
      const segs = parseAnsi("\x1b[7minverse\x1b[0m");
      expect(segs[0]!.style.inverse).toBe(true);
    });

    it("parses strikethrough", () => {
      const segs = parseAnsi("\x1b[9mstrike\x1b[0m");
      expect(segs[0]!.style.strikethrough).toBe(true);
    });
  });

  describe("reset and partial reset", () => {
    it("resets all attributes on SGR 0", () => {
      const segs = parseAnsi("\x1b[1;31mbold red\x1b[0m plain");
      expect(segs).toHaveLength(2);
      expect(segs[0]!.style.bold).toBe(true);
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
      expect(segs[1]!.style).toEqual(defaultStyle());
      expect(segs[1]!.text).toBe(" plain");
    });

    it("resets bold only with SGR 22", () => {
      const segs = parseAnsi("\x1b[1;31mbold red\x1b[22mjust red\x1b[0m");
      expect(segs).toHaveLength(2);
      expect(segs[0]!.style.bold).toBe(true);
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
      expect(segs[1]!.style.bold).toBe(false);
      expect(segs[1]!.style.fg).toEqual({ type: "basic", index: 1 });
    });
  });

  describe("multiple attributes in one SGR", () => {
    it("parses combined bold+italic+color", () => {
      const segs = parseAnsi("\x1b[1;3;31mtext\x1b[0m");
      expect(segs[0]!.style.bold).toBe(true);
      expect(segs[0]!.style.italic).toBe(true);
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
    });
  });

  describe("coalescing", () => {
    it("merges adjacent segments with same style", () => {
      const segs = parseAnsi("hello ");
      expect(segs).toHaveLength(1);
      expect(segs[0]!.text).toBe("hello ");
    });

    it("merges redundant SGR producing same style", () => {
      const segs = parseAnsi("\x1b[31m\x1b[31mred\x1b[0m");
      expect(segs).toHaveLength(1);
      expect(segs[0]!.text).toBe("red");
    });
  });

  describe("non-SGR escape stripping", () => {
    it("strips cursor movement (CSI with non-m final byte)", () => {
      const segs = parseAnsi("before\x1b[2Jafter");
      expect(textOf(segs)).toBe("beforeafter");
    });

    it("strips cursor position", () => {
      const segs = parseAnsi("a\x1b[5;10Hb");
      expect(textOf(segs)).toBe("ab");
    });

    it("strips OSC sequences", () => {
      const segs = parseAnsi("a\x1b]0;title\x07b");
      expect(textOf(segs)).toBe("ab");
    });

    it("strips charset selection", () => {
      const segs = parseAnsi("a\x1b(Bb");
      expect(textOf(segs)).toBe("ab");
    });

    it("strips DEC private mode", () => {
      const segs = parseAnsi("a\x1b[?25lb");
      expect(textOf(segs)).toBe("ab");
    });
  });

  describe("control character and garbage stripping", () => {
    it("strips null bytes", () => {
      expect(textOf(parseAnsi("a\x00b"))).toBe("ab");
    });

    it("strips control characters", () => {
      expect(textOf(parseAnsi("a\x01\x02\x03b"))).toBe("ab");
    });

    it("strips box-drawing characters", () => {
      expect(textOf(parseAnsi("a─│┌b"))).toBe("ab");
    });

    it("strips block elements", () => {
      expect(textOf(parseAnsi("a█▓▒b"))).toBe("ab");
    });

    it("strips braille patterns", () => {
      expect(textOf(parseAnsi("a⠁⠃b"))).toBe("ab");
    });
  });

  describe("realistic input", () => {
    it("handles ls --color directory listing", () => {
      const input = "\x1b[1;34mdirname\x1b[0m  file.txt";
      const segs = parseAnsi(input);
      expect(segs).toHaveLength(2);
      expect(segs[0]!.text).toBe("dirname");
      expect(segs[0]!.style.bold).toBe(true);
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 4 });
      expect(segs[1]!.text).toBe("  file.txt");
    });

    it("handles git diff output", () => {
      const input = "\x1b[32m+added line\x1b[0m";
      const segs = parseAnsi(input);
      expect(segs[0]!.text).toBe("+added line");
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 2 });
    });
  });

  describe("edge cases", () => {
    it("handles truncated SGR at end of input", () => {
      const segs = parseAnsi("text\x1b[31");
      expect(textOf(segs)).toBe("text");
    });

    it("treats empty SGR as reset", () => {
      const segs = parseAnsi("\x1b[1mbold\x1b[m plain");
      expect(segs[1]!.style).toEqual(defaultStyle());
    });

    it("handles foreground and background together", () => {
      const segs = parseAnsi("\x1b[31;42mred on green\x1b[0m");
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
      expect(segs[0]!.style.bg).toEqual({ type: "basic", index: 2 });
    });

    it("resets foreground only with SGR 39", () => {
      const segs = parseAnsi("\x1b[31;42mred\x1b[39m still-green-bg");
      expect(segs).toHaveLength(2);
      expect(segs[0]!.style.fg).toEqual({ type: "basic", index: 1 });
      expect(segs[0]!.style.bg).toEqual({ type: "basic", index: 2 });
      expect(segs[1]!.style.fg).toBeNull();
      expect(segs[1]!.style.bg).toEqual({ type: "basic", index: 2 });
    });
  });
});

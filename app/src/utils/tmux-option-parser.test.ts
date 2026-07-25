import { describe, expect, it } from "vitest";
import { parseContextOptions, stripAnsi } from "./tmux-option-parser";

describe("stripAnsi", () => {
  it("removes CSI color sequences", () => {
    expect(stripAnsi("\x1b[32mgreen\x1b[0m text")).toBe("green text");
  });

  it("removes OSC sequences", () => {
    expect(stripAnsi("\x1b]0;window title\x07content")).toBe("content");
    expect(stripAnsi("\x1b]8;;https://example.com\x1b\\link")).toBe("link");
  });

  it("removes charset sequences", () => {
    expect(stripAnsi("\x1b(B\x1b)0box")).toBe("box");
  });

  it("preserves newlines and tabs", () => {
    expect(stripAnsi("a\nb\tc")).toBe("a\nb\tc");
  });
});

describe("parseContextOptions", () => {
  it("parses numbered options with dot separator", () => {
    const content = "Choose an action:\n1. Yes\n2. No\n3. Always allow";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "Yes" },
      { digit: "2", label: "No" },
      { digit: "3", label: "Always allow" },
    ]);
  });

  it("parses options with parentheses", () => {
    const content = "Select:\n(1) Continue\n(2) Abort";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "Continue" },
      { digit: "2", label: "Abort" },
    ]);
  });

  it("parses options with CJK parentheses", () => {
    const content = "（1）继续\n（2）取消";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "继续" },
      { digit: "2", label: "取消" },
    ]);
  });

  it("parses options with colon separator", () => {
    const content = "1: Yes\n2: No";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "Yes" },
      { digit: "2", label: "No" },
    ]);
  });

  it("strips ANSI codes before parsing", () => {
    const content = "\x1b[1mChoose:\x1b[0m\n\x1b[36m1. \x1b[0mDeploy\n\x1b[36m2. \x1b[0mCancel";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "Deploy" },
      { digit: "2", label: "Cancel" },
    ]);
  });

  it("truncates labels longer than 12 characters", () => {
    const content = "1. Always allow this permission\n2. Deny";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "Always allow…" },
      { digit: "2", label: "Deny" },
    ]);
  });

  it("returns empty for a single option", () => {
    const content = "Some log line\n1. Only one option";
    expect(parseContextOptions(content)).toEqual([]);
  });

  it("returns empty when options do not start from 1", () => {
    const content = "2. Second\n3. Third";
    expect(parseContextOptions(content)).toEqual([]);
  });

  it("returns empty for plain terminal output", () => {
    const content = "$ ls -la\ntotal 42\ndrwxr-xr-x  5 user  staff  160 Jul 25 10:00 .";
    expect(parseContextOptions(content)).toEqual([]);
  });

  it("only scans the last N lines", () => {
    const content = "1. Old option\n2. Old option\n" + "log line\n".repeat(15) + "prompt>";
    expect(parseContextOptions(content)).toEqual([]);
  });

  it("deduplicates repeated digits keeping first occurrence", () => {
    const content = "1. First\n2. Second\n1. Duplicate";
    expect(parseContextOptions(content)).toEqual([
      { digit: "1", label: "First" },
      { digit: "2", label: "Second" },
    ]);
  });

  it("handles empty content", () => {
    expect(parseContextOptions("")).toEqual([]);
  });

  it("respects custom tailLines", () => {
    const content = "1. Yes\n2. No\n" + "filler\n".repeat(5);
    expect(parseContextOptions(content, { tailLines: 20 })).toEqual([
      { digit: "1", label: "Yes" },
      { digit: "2", label: "No" },
    ]);
    expect(parseContextOptions(content, { tailLines: 3 })).toEqual([]);
  });
});

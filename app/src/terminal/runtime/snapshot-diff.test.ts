import { describe, expect, it } from "vitest";
import { diffSnapshots } from "./snapshot-diff";

describe("diffSnapshots", () => {
  it("returns no-op for identical strings", () => {
    const result = diffSnapshots("line1\nline2", "line1\nline2");
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBeNull();
  });

  it("returns full rewrite when line counts differ", () => {
    const result = diffSnapshots("line1\nline2", "line1\nline2\nline3");
    expect(result.fullRewrite).toBe(true);
    expect(result.patch).toBeNull();
  });

  it("patches a single changed line with cursor addressing", () => {
    const result = diffSnapshots("aaa\nbbb\nccc", "aaa\nXXX\nccc");
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBe("\x1b[2;1H\x1b[2KXXX");
  });

  it("patches multiple changed lines", () => {
    const result = diffSnapshots("aaa\nbbb\nccc\nddd", "XXX\nbbb\nYYY\nddd");
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBe("\x1b[1;1H\x1b[2KXXX\x1b[3;1H\x1b[2KYYY");
  });

  it("returns full rewrite when more than 80% of lines change", () => {
    const prev = "a\nb\nc\nd\ne\nf";
    const next = "X\nY\nZ\nW\nV\nf";
    const result = diffSnapshots(prev, next);
    expect(result.fullRewrite).toBe(true);
    expect(result.patch).toBeNull();
  });

  it("handles empty strings", () => {
    const result = diffSnapshots("", "");
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBeNull();
  });

  it("returns full rewrite for single line change (100% exceeds threshold)", () => {
    const result = diffSnapshots("old", "new");
    expect(result.fullRewrite).toBe(true);
    expect(result.patch).toBeNull();
  });

  it("handles ANSI-containing lines", () => {
    const prev = "\x1b[32mgreen\x1b[0m\nplain";
    const next = "\x1b[31mred\x1b[0m\nplain";
    const result = diffSnapshots(prev, next);
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBe("\x1b[1;1H\x1b[2K\x1b[31mred\x1b[0m");
  });

  it("returns no-op when both are empty with same line count", () => {
    const result = diffSnapshots("\n\n", "\n\n");
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).toBeNull();
  });

  it("patches when exactly 80% of lines change (boundary)", () => {
    // 4 out of 5 lines change = 80%, which is NOT > 80%, so patch
    const prev = "a\nb\nc\nd\ne";
    const next = "X\nY\nZ\nW\ne";
    // Actually 4/5 = 0.8 which is not > 0.8, so it should patch
    const result = diffSnapshots(prev, next);
    expect(result.fullRewrite).toBe(false);
    expect(result.patch).not.toBeNull();
  });
});

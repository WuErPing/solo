import { describe, expect, it } from "vitest";
import { chunkMarkdownBlocks } from "./chunk-markdown-blocks";

describe("chunkMarkdownBlocks", () => {
  it("merges small text blocks into one chunk", () => {
    expect(chunkMarkdownBlocks(["a", "b", "c"], 1200)).toEqual(["a\n\nb\n\nc"]);
  });

  it("keeps fence blocks isolated from text", () => {
    const fence = "```mermaid\ngraph TD; A-->B\n```";
    expect(chunkMarkdownBlocks(["before", fence, "after"], 1200)).toEqual([
      "before",
      fence,
      "after",
    ]);
  });

  it("splits when the next block would exceed the size cap", () => {
    expect(chunkMarkdownBlocks(["aaaa", "bbbb", "cc"], 9)).toEqual(["aaaa\n\nbbbb", "cc"]);
  });

  it("never merges a fence into a pending text run", () => {
    const fence = "~~~\ncode\n~~~";
    expect(chunkMarkdownBlocks(["x", fence], 1200)).toEqual(["x", fence]);
  });

  it("keeps a fence larger than the cap as its own chunk", () => {
    const big = "```go\n" + "x".repeat(5000) + "\n```";
    expect(chunkMarkdownBlocks([big], 100)).toEqual([big]);
  });

  it("returns an empty array for empty input", () => {
    expect(chunkMarkdownBlocks([])).toEqual([]);
  });
});

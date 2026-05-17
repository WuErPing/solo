import { describe, expect, it } from "vitest";
import { getMarkdownListMarker } from "./markdown-list";

describe("getMarkdownListMarker", () => {
  it("returns a bullet marker for unordered list items", () => {
    expect(getMarkdownListMarker({ index: 0 }, [{ type: "bullet_list" }])).toEqual({
      isOrdered: false,
      marker: "•",
    });
  });

  it("returns numbered markers for ordered list items", () => {
    expect(getMarkdownListMarker({ index: 1, markup: "." }, [{ type: "ordered_list" }])).toEqual({
      isOrdered: true,
      marker: "2.",
    });
  });

  it("respects ordered list start attribute", () => {
    expect(
      getMarkdownListMarker({ index: 2, markup: ")" }, [
        { type: "ordered_list", attributes: { start: "5" } },
      ]),
    ).toEqual({
      isOrdered: true,
      marker: "7)",
    });
  });

  it("prefers the nearest list ancestor in nested lists", () => {
    expect(
      getMarkdownListMarker({ index: 0, markup: "." }, [
        { type: "ordered_list" },
        { type: "bullet_list" },
      ]),
    ).toEqual({
      isOrdered: true,
      marker: "1.",
    });
  });
});

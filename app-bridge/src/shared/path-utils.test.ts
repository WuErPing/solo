import { describe, it, expect } from "vitest";
import { stripCwdPrefix } from "./path-utils.js";

describe("stripCwdPrefix", () => {
  it("returns path unchanged when cwd is undefined", () => {
    expect(stripCwdPrefix("/foo/bar", undefined)).toBe("/foo/bar");
  });

  it("returns path unchanged when path is empty", () => {
    expect(stripCwdPrefix("", "/foo")).toBe("");
  });

  it("strips matching prefix", () => {
    expect(stripCwdPrefix("/home/user/project/src/main.ts", "/home/user/project")).toBe(
      "src/main.ts"
    );
  });

  it("returns '.' when path equals cwd", () => {
    expect(stripCwdPrefix("/home/user/project", "/home/user/project")).toBe(".");
  });

  it("normalizes backslashes to slashes", () => {
    expect(stripCwdPrefix("\\home\\user\\project\\src\\main.ts", "\\home\\user\\project")).toBe(
      "src/main.ts"
    );
  });

  it("returns path unchanged when no match", () => {
    expect(stripCwdPrefix("/other/path", "/home/user/project")).toBe("/other/path");
  });

  it("handles cwd with trailing slash", () => {
    expect(stripCwdPrefix("/home/user/project/src/main.ts", "/home/user/project/")).toBe(
      "src/main.ts"
    );
  });

  it("returns path unchanged when path is a sibling", () => {
    expect(stripCwdPrefix("/home/user/other", "/home/user/project")).toBe("/home/user/other");
  });
});

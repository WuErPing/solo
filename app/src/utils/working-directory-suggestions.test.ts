import { describe, expect, it } from "vitest";
import { buildWorkingDirectorySuggestions } from "./working-directory-suggestions";

describe("buildWorkingDirectorySuggestions", () => {
  it("returns de-duplicated recommendations when query is empty", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo", "/Users/me/projects/solo"],
      serverPaths: ["/Users/me/projects/playground"],
      query: "",
    });

    expect(results).toEqual(["/Users/me/projects/solo"]);
  });

  it("prioritizes matching recommended directories before server matches", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo", "/Users/me/documents"],
      serverPaths: [
        "/Users/me/projects/playground",
        "/Users/me/projects/solo",
        "/Users/me/projects/planbook",
      ],
      query: "pla",
    });

    expect(results).toEqual(["/Users/me/projects/playground", "/Users/me/projects/planbook"]);
  });

  it("puts matching recommended items first when they also match query", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/playground", "/Users/me/projects/solo"],
      serverPaths: ["/Users/me/projects/planbook", "/Users/me/projects/playground"],
      query: "pla",
    });

    expect(results).toEqual(["/Users/me/projects/playground", "/Users/me/projects/planbook"]);
  });

  it("treats '~' as an active query and includes server suggestions", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo"],
      serverPaths: ["/Users/me/documents", "/Users/me/projects"],
      query: "~",
    });

    expect(results).toEqual([
      "/Users/me/projects/solo",
      "/Users/me/documents",
      "/Users/me/projects",
    ]);
  });

  it("offers a typed path that matches nothing so it stays confirmable", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo"],
      serverPaths: [],
      query: "/Users/me/code/brand-new",
    });

    expect(results).toEqual(["/Users/me/code/brand-new"]);
  });

  it("puts a typed path ahead of index matches", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo"],
      serverPaths: ["/Users/me/projects/solo-mobile"],
      query: "~/projects/solo",
    });

    expect(results).toEqual([
      "~/projects/solo",
      "/Users/me/projects/solo",
      "/Users/me/projects/solo-mobile",
    ]);
  });

  it("does not duplicate a typed path already present in the results", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/solo"],
      serverPaths: [],
      query: " /Users/me/projects/solo ",
    });

    expect(results).toEqual(["/Users/me/projects/solo"]);
  });

  it("does not offer bare substring searches as a path", () => {
    const results = buildWorkingDirectorySuggestions({
      recommendedPaths: ["/Users/me/projects/playground"],
      serverPaths: [],
      query: "play",
    });

    expect(results).toEqual(["/Users/me/projects/playground"]);
  });
});

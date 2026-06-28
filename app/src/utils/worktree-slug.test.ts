import { describe, expect, it, vi } from "vitest";
import { generateWorktreeSlug } from "./worktree-slug";

vi.mock("mnemonic-id", () => ({
  createUniqueNameId: () => "married-pelican-abc123",
}));

describe("generateWorktreeSlug", () => {
  it("uses the slugified user prompt when provided", () => {
    expect(
      generateWorktreeSlug({
        prompt: "  Fix the login bug! ",
        projectDisplayName: "Solo",
        sourceDirectory: "/Users/me/solo",
      }),
    ).toBe("fix-the-login-bug");
  });

  it("falls back to the PR title when there is no prompt", () => {
    expect(
      generateWorktreeSlug({
        prTitle: "Refactor picker component",
        projectDisplayName: "Solo",
        sourceDirectory: "/Users/me/solo",
      }),
    ).toBe("refactor-picker-component");
  });

  it("falls back to the project display name when there is no prompt or PR", () => {
    expect(
      generateWorktreeSlug({
        projectDisplayName: "Solo",
        sourceDirectory: "/Users/me/solo",
      }),
    ).toBe("solo");
  });

  it("falls back to the source directory basename when nothing else is available", () => {
    expect(
      generateWorktreeSlug({
        sourceDirectory: "/Users/me/solo",
      }),
    ).toBe("solo");
  });

  it("uses the mnemonic fallback when no useful input is provided", () => {
    expect(generateWorktreeSlug({})).toBe("married-pelican-abc123");
  });

  it("truncates long slugs at a word boundary when possible", () => {
    const prompt =
      "this is a really long prompt that should be truncated at a reasonable word boundary";
    const slug = generateWorktreeSlug({ prompt });
    expect(slug.length).toBeLessThanOrEqual(40);
    expect(slug.endsWith("-")).toBe(false);
  });

  it("ignores empty or whitespace-only inputs", () => {
    expect(
      generateWorktreeSlug({
        prompt: "   ",
        projectDisplayName: "Solo",
      }),
    ).toBe("solo");
  });
});

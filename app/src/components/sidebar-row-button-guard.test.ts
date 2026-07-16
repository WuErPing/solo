/**
 * @vitest-environment node
 *
 * Regression guard: on web, RNW renders any element with role/accessibilityRole
 * "button" as a real <button>. Sidebar workspace rows contain action buttons
 * (kebab menu, archive/copy) inside the row, so the row Pressable itself must
 * NOT request role="button" — otherwise the DOM nests <button> in <button> and
 * React logs "<button> cannot contain a nested <button>".
 *
 * The project row in the same file already follows this rule (no role prop).
 * Keep the row a plain pressable div; inner action buttons keep their own
 * accessibilityRole="button" and stay keyboard-accessible.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const SOURCE = readFileSync(join(__dirname, "sidebar-workspace-list.tsx"), "utf-8");

describe("sidebar workspace row button nesting", () => {
  it("the workspace row Pressable does not request accessibilityRole=button", () => {
    const rowTestIdIndex = SOURCE.indexOf("testID={`sidebar-workspace-row-");
    expect(rowTestIdIndex).toBeGreaterThan(-1);

    // Inspect the Pressable opening tag that wraps the row (the segment from
    // the "<Pressable" that precedes the row testID up to the testID itself).
    const pressableStart = SOURCE.lastIndexOf("<Pressable", rowTestIdIndex);
    expect(pressableStart).toBeGreaterThan(-1);
    const openingTag = SOURCE.slice(pressableStart, rowTestIdIndex);

    expect(openingTag).not.toContain('accessibilityRole="button"');
  });
});

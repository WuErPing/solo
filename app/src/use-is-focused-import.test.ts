/**
 * @vitest-environment node
 *
 * Regression guard: expo-router SDK 57 no longer re-exports `useIsFocused`.
 * Importing it from "expo-router" yields `undefined` at runtime, causing
 * "useIsFocused is not a function". Source files must import from
 * "@react-navigation/native" instead.
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const SRC_DIR = join(__dirname, "..", "src");

function collectTsFiles(dir: string): string[] {
  const results: string[] = [];
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    const stat = statSync(full);
    if (stat.isDirectory()) {
      results.push(...collectTsFiles(full));
    } else if (entry.endsWith(".ts") || entry.endsWith(".tsx")) {
      results.push(full);
    }
  }
  return results;
}

const IMPORT_PATTERN = /import\s+\{[^}]*\buseIsFocused\b[^}]*\}\s+from\s+["']expo-router["']/;

describe("useIsFocused import source", () => {
  it("no source file imports useIsFocused from expo-router", () => {
    const files = collectTsFiles(SRC_DIR);
    const violators = files
      .filter((f) => IMPORT_PATTERN.test(readFileSync(f, "utf-8")))
      .map((f) => relative(SRC_DIR, f));

    expect(violators).toEqual([]);
  });
});

/**
 * @vitest-environment node
 *
 * Regression guard: expo-router ships a forked react-navigation whose Stack
 * navigators provide the fork's NavigationContext. Importing useIsFocused
 * from "@react-navigation/native" (or anything other than the fork) reads a
 * React context nothing provides — on web the hook throws
 * "Couldn't find a navigation object. Is your component inside NavigationContainer?"
 * and blanks the route (Sessions/Schedules/Loops).
 *
 * All call sites must import useIsFocused from "@/hooks/use-is-focused",
 * which re-exports the fork's hook (the only one that observes screen focus).
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";

const SRC_DIR = join(__dirname, "..", "src");
const ALLOWED_SOURCE = "@/hooks/use-is-focused";

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

const IMPORT_PATTERN = /import\s+\{[^}]*\buseIsFocused\b[^}]*\}\s+from\s+["']([^"']+)["']/g;

describe("useIsFocused import source", () => {
  it("all useIsFocused imports come from @/hooks/use-is-focused", () => {
    const files = collectTsFiles(SRC_DIR).filter(
      (f) => !f.endsWith(".test.ts") && !f.endsWith(".test.tsx") && !f.endsWith("hooks/use-is-focused.ts"),
    );
    const violators: string[] = [];
    for (const file of files) {
      const source = readFileSync(file, "utf-8");
      for (const match of source.matchAll(IMPORT_PATTERN)) {
        if (match[1] !== ALLOWED_SOURCE) {
          violators.push(`${relative(SRC_DIR, file)} (from "${match[1]}")`);
        }
      }
    }

    expect(violators).toEqual([]);
  });
});

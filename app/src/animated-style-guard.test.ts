/**
 * @vitest-environment node
 *
 * Regression guard: on web, unistyles StyleSheet.create objects carry an
 * enumerable `unistyles_<id>: {}` metadata key (their real style props are
 * non-enumerable). Reanimated's web CSS engine flattens Animated.View style
 * arrays and validates every prop, throwing
 * `[Reanimated] Invalid value for "unistyles_<id>": an empty object is not a
 * valid style value.`
 *
 * Static styles combined with animated styles must therefore be plain RN
 * StyleSheet styles (see explorerStaticStyles in explorer-sidebar.tsx), not
 * unistyles styles. This guard flags `[styles.X, <...>Animated<...>]` arrays,
 * the shape that crashed message-input/composer/file-drop-zone on web.
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

const UNISTYLES_STYLE_IN_ANIMATED_ARRAY = /\[\s*styles\.\w+\s*,\s*\w*[Aa]nimated\w*\s*\]/;

describe("reanimated Animated.View style composition", () => {
  it("no unistyles style object is combined with an animated style", () => {
    const files = collectTsFiles(SRC_DIR).filter((f) => !f.endsWith(".test.ts") && !f.endsWith(".test.tsx"));
    const violators = files
      .filter((f) => {
        const source = readFileSync(f, "utf-8");
        return (
          source.includes("react-native-reanimated") && UNISTYLES_STYLE_IN_ANIMATED_ARRAY.test(source)
        );
      })
      .map((f) => relative(SRC_DIR, f));

    expect(violators).toEqual([]);
  });
});

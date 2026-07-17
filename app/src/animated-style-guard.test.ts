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
const HAS_LAYOUT_ANIMATION = /entering=|exiting=/;
const DIRECT_UNISTYLES_REF = /^styles(?:heet)?\./;

// Extract <Animated.X ...> opening tags, honoring brace depth so `>` inside
// attribute expressions (e.g. `(finished) => {...}`) does not end the tag.
function extractAnimatedTags(source: string): string[] {
  const tags: string[] = [];
  const startRe = /<Animated\.[A-Za-z]+/g;
  for (const start of source.matchAll(startRe)) {
    let depth = 0;
    let end = -1;
    for (let i = start.index! + start[0].length; i < source.length; i++) {
      const ch = source[i];
      if (ch === "{") depth++;
      else if (ch === "}") depth--;
      else if (ch === ">" && depth === 0) {
        end = i;
        break;
      }
    }
    if (end !== -1) tags.push(source.slice(start.index, end + 1));
  }
  return tags;
}

// Extract the statement starting at the first regex hit, using bracket-depth
// tracking (handles nested => functions, objects and arrays inside).
function extractStatement(source: string, startRe: RegExp): string | null {
  const match = startRe.exec(source);
  if (!match) return null;
  let depth = 0;
  for (let i = match.index; i < source.length; i++) {
    const ch = source[i];
    if (ch === "{" || ch === "(" || ch === "[") depth++;
    else if (ch === "}" || ch === ")" || ch === "]") depth--;
    else if (ch === ";" && depth === 0) return source.slice(match.index, i + 1);
  }
  return null;
}

function extractFunctionBody(source: string, name: string): string | null {
  const match = new RegExp(`function ${name}\\(`).exec(source);
  if (!match) return null;
  const bodyStart = source.indexOf("{", match.index);
  if (bodyStart === -1) return null;
  let depth = 0;
  for (let i = bodyStart; i < source.length; i++) {
    const ch = source[i];
    if (ch === "{") depth++;
    else if (ch === "}") {
      depth--;
      if (depth === 0) return source.slice(bodyStart, i + 1);
    }
  }
  return null;
}

function composedStyleViolates(source: string, ident: string): boolean {
  // The style variable's own definition (const X = [...] / useMemo(...)).
  const region = extractStatement(source, new RegExp(`const ${ident} =`));
  if (region && /\[\s*styles(?:heet)?\./.test(region)) return true;
  // Delegated: const X = useMemo(() => buildSomething({...})) — inspect the builder body.
  const delegated = region?.match(/=>\s*(\w+)\(/);
  if (delegated) {
    const builderRegion = extractFunctionBody(source, delegated[1]);
    if (builderRegion && /\[\s*styles(?:heet)?\./.test(builderRegion)) return true;
  }
  return false;
}

function hasLayoutAnimationWithUnistylesStyle(source: string): boolean {
  if (!source.includes("react-native-reanimated")) return false;
  for (const tag of extractAnimatedTags(source)) {
    if (!HAS_LAYOUT_ANIMATION.test(tag)) continue;
    const styleProp = tag.match(/style=\{([^}]*)\}/)?.[1]?.trim();
    if (!styleProp) continue;
    if (DIRECT_UNISTYLES_REF.test(styleProp)) return true;
    // style={contentStyle} — trace the local identifier's definition.
    if (/^\w+$/.test(styleProp) && composedStyleViolates(source, styleProp)) return true;
    // style={props.desktopContainerStyle as never} — trace through a
    // `prop={local}` handoff rendered elsewhere in the same file.
    const viaProps = styleProp.match(/^props\.(\w+)(?:\s+as\s+.*)?$/);
    if (viaProps) {
      const handoff = source.match(new RegExp(`${viaProps[1]}=\\{(\\w+)\\}`));
      if (handoff && composedStyleViolates(source, handoff[1])) return true;
    }
  }
  return false;
}

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

  it("no unistyles style object reaches entering/exiting animated components", () => {
    // Layout animations (entering/exiting) also route the style through
    // Reanimated's CSS validator, so unistyles styles crash there too —
    // whether passed directly (style={styles.x}) or via a composed style
    // variable. Plain elements in the same file are fine and must not be
    // flagged: the check traces only the style prop of the animated tag.
    const files = collectTsFiles(SRC_DIR).filter((f) => !f.endsWith(".test.ts") && !f.endsWith(".test.tsx"));
    const violators = files
      .filter((f) => hasLayoutAnimationWithUnistylesStyle(readFileSync(f, "utf-8")))
      .map((f) => relative(SRC_DIR, f));

    expect(violators).toEqual([]);
  });
});

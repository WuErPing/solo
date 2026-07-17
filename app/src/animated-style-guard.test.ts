/**
 * @vitest-environment node
 *
 * Regression guard: on web, unistyles StyleSheet.create objects carry an
 * enumerable `unistyles_<id>: {}` metadata key (their real style props are
 * non-enumerable). Every Animated.* component mounts a Reanimated CSSManager
 * that flattens props.style and validates every entry, throwing
 * `[Reanimated] Invalid value for "unistyles_<id>": an empty object is not a
 * valid style value.` (reanimated src/css/utils/props.ts +
 * createAnimatedComponent/AnimatedComponent.tsx `_updateStyles`).
 *
 * So a unistyles style must NEVER reach an Animated.* style prop on web —
 * with or without animated styles, with or without entering/exiting. Static
 * styles combined with animated styles must be plain theme-derived objects
 * (useUnistyles + useMemo) or plain RN StyleSheet styles (see
 * explorerStaticStyles in explorer-sidebar.tsx).
 *
 * This guard extracts every <Animated.X> tag, reads its style prop
 * (brace-balanced), and flags any reference — direct, inline-array, traced
 * through local const/useMemo identifiers, delegated builder functions, or
 * prop handoffs — to a style object created by unistyles StyleSheet.create.
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

// Extract the style={...} expression from a tag, brace-balanced so nested
// objects/arrays (`[styles.x, { width: 1 }]`) are captured in full.
// `\bstyle` does not match contentContainerStyle/inputStyle etc.
function extractStyleExpression(tag: string): string | null {
  const match = /\bstyle=\s*\{/.exec(tag);
  if (!match) return null;
  const open = match.index + match[0].length - 1;
  let depth = 0;
  for (let i = open; i < tag.length; i++) {
    const ch = tag[i];
    if (ch === "{") depth++;
    else if (ch === "}") {
      depth--;
      if (depth === 0) return tag.slice(open + 1, i).trim();
    }
  }
  return null;
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

// Local names bound to unistyles' StyleSheet (`import { StyleSheet } from
// "react-native-unistyles"` or an aliased variant).
function unistylesStyleSheetNames(source: string): string[] {
  const names: string[] = [];
  const importRe =
    /import\s*\{([^}]*)\}\s*from\s*["']react-native-unistyles["']/g;
  for (const m of source.matchAll(importRe)) {
    for (const spec of m[1].split(",")) {
      const part = spec.trim();
      const aliased = part.match(/^StyleSheet\s+as\s+(\w+)$/);
      if (aliased) names.push(aliased[1]);
      else if (part === "StyleSheet") names.push("StyleSheet");
    }
  }
  return names;
}

// Identifiers defined via unistyles StyleSheet.create, e.g. `const styles =
// StyleSheet.create((theme) => ({...}))`. Only these objects carry the
// web metadata key that crashes Reanimated; RN StyleSheet objects are safe.
function unistylesStyleIdents(source: string): Set<string> {
  const idents = new Set<string>();
  for (const name of unistylesStyleSheetNames(source)) {
    const createRe = new RegExp(`const (\\w+) = ${name}\\.create`, "g");
    for (const m of source.matchAll(createRe)) idents.add(m[1]);
  }
  return idents;
}

function referencesUnistylesStyle(
  expr: string,
  idents: Set<string>,
): string | null {
  for (const ident of idents) {
    if (new RegExp(`\\b${ident}\\.`).test(expr)) return ident;
  }
  return null;
}

// Trace a local identifier's definition (`const X = ...`, possibly a useMemo
// or a delegated builder call) and report whether a unistyles style object
// flows into it. Recurses into other local consts referenced in the body.
function identViolates(
  source: string,
  ident: string,
  idents: Set<string>,
  visited: Set<string>,
): boolean {
  if (visited.has(ident)) return false;
  visited.add(ident);
  const region = extractStatement(source, new RegExp(`const ${ident} =`));
  if (!region) return false;
  if (referencesUnistylesStyle(region, idents)) return true;
  // Delegated: const X = useMemo(() => buildSomething({...})) — inspect the builder body.
  const delegated = region.match(/=>\s*(\w+)\(/);
  if (delegated) {
    const builderRegion = extractFunctionBody(source, delegated[1]);
    if (builderRegion) {
      if (referencesUnistylesStyle(builderRegion, idents)) return true;
      for (const ref of builderRegion.matchAll(/\b(\w+)\b/g)) {
        if (identViolates(source, ref[1], idents, visited)) return true;
      }
    }
  }
  for (const ref of region.matchAll(/\b(\w+)\b/g)) {
    if (identViolates(source, ref[1], idents, visited)) return true;
  }
  return false;
}

// Split a style expression into top-level elements: `[a, b, { ... }]` yields
// its comma-separated items; anything else is a single expression.
function styleElements(expr: string): string[] {
  const trimmed = expr.trim();
  if (!trimmed.startsWith("[") || !trimmed.endsWith("]")) return [trimmed];
  const inner = trimmed.slice(1, -1);
  const elements: string[] = [];
  let depth = 0;
  let start = 0;
  for (let i = 0; i < inner.length; i++) {
    const ch = inner[i];
    if (ch === "{" || ch === "(" || ch === "[") depth++;
    else if (ch === "}" || ch === ")" || ch === "]") depth--;
    else if (ch === "," && depth === 0) {
      elements.push(inner.slice(start, i).trim());
      start = i + 1;
    }
  }
  const last = inner.slice(start).trim();
  if (last) elements.push(last);
  return elements.filter(Boolean);
}

function styleExpressionViolates(
  source: string,
  expr: string,
  idents: Set<string>,
): boolean {
  for (const element of styleElements(expr)) {
    // Direct: styles.x, [styles.x, ...], cond && styles.x, ...styles.x
    if (referencesUnistylesStyle(element, idents)) return true;
    // Traced local identifier: style={trackStyle} / [trackStyle, ...]
    if (/^\w+$/.test(element)) {
      if (identViolates(source, element, idents, new Set())) return true;
      continue;
    }
    // Prop handoff: style={props.desktopContainerStyle as never} — trace
    // through a `prop={local}` handoff rendered elsewhere in the same file.
    const viaProps = element.match(/^props\.(\w+)(?:\s+as\s+.*)?$/);
    if (viaProps) {
      const handoff = source.match(new RegExp(`${viaProps[1]}=\\{(\\w+)\\}`));
      if (handoff && identViolates(source, handoff[1], idents, new Set())) {
        return true;
      }
    }
  }
  return false;
}

describe("reanimated Animated.* style composition", () => {
  it("no unistyles style object reaches any Animated.* style prop", () => {
    const files = collectTsFiles(SRC_DIR).filter(
      (f) => !f.endsWith(".test.ts") && !f.endsWith(".test.tsx"),
    );
    const violators = files
      .filter((f) => {
        const source = readFileSync(f, "utf-8");
        if (!source.includes("react-native-reanimated")) return false;
        const idents = unistylesStyleIdents(source);
        if (idents.size === 0) return false;
        return extractAnimatedTags(source).some((tag) => {
          const expr = extractStyleExpression(tag);
          return expr !== null && styleExpressionViolates(source, expr, idents);
        });
      })
      .map((f) => relative(SRC_DIR, f));

    expect(violators).toEqual([]);
  });
});

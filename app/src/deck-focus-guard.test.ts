/**
 * @vitest-environment node
 *
 * Regression guard: WorkspaceDeck (src/app/h/[serverId]/workspace/[workspaceId]/_layout.tsx)
 * renders WorkspaceScreen as plain children of a View, outside any navigation
 * screen. Components rendered there must not call useIsFocused() — it throws
 * "Couldn't find a navigation object. Is your component inside NavigationContainer?"
 *
 * Only real route screens (rendered by a navigator) may call useIsFocused.
 * Adding a new call site requires consciously classifying it here.
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

// Route-level screens rendered by a navigator (navigation context present).
const ALLOWED_FILES = new Set([
  "hooks/use-is-focused.ts",
  "screens/loop-detail-screen.tsx",
  "screens/loop-instance-detail-screen.tsx",
  "screens/loops-screen.tsx",
  "screens/schedule-detail-screen.tsx",
  "screens/schedules-screen.tsx",
  "screens/schedules/schedules-dashboard-screen.tsx",
  "screens/sessions-screen.tsx",
  "screens/settings/keyboard-shortcuts-section.tsx",
]);

describe("useIsFocused call sites", () => {
  it("only route-level screens call useIsFocused", () => {
    const files = collectTsFiles(SRC_DIR).filter((f) => !f.endsWith(".test.ts") && !f.endsWith(".test.tsx"));
    const violators = files
      .filter((f) => /\buseIsFocused\b/.test(readFileSync(f, "utf-8")))
      .map((f) => relative(SRC_DIR, f))
      .filter((f) => !ALLOWED_FILES.has(f));

    expect(violators).toEqual([]);
  });
});

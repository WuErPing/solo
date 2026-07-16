/**
 * @vitest-environment node
 *
 * Regression guard: expo-router's forked react-navigation web Screen
 * (build/react-navigation/elements/Screen.js) wraps only the header in
 * NavigationProvider — screen CONTENT gets no NavigationContext on web.
 * Any navigation hook in a screen body (useIsFocused, useNavigation, ...)
 * throws "Couldn't find a navigation object. Is your component inside
 * NavigationContainer?" and blanks the page (Sessions/Schedules/Loops routes).
 * The native variant wraps every screen (NativeStackView.native.js).
 *
 * Fixed via patch-package (patches/expo-router+*.patch), which wraps the
 * content branch in NavigationProvider with the same route/navigation props.
 * This test fails if the patch is missing or stops applying.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const require = createRequire(join(__dirname, "..", "package.json"));

describe("expo-router web Screen navigation context patch", () => {
  it("wraps screen content in NavigationProvider", () => {
    const packageJsonPath = require.resolve("expo-router/package.json");
    const screenPath = join(
      dirname(packageJsonPath),
      "build/react-navigation/elements/Screen.js",
    );
    const source = readFileSync(screenPath, "utf-8");

    expect(source).toMatch(
      /NavigationProvider, \{ route: route, navigation: navigation, children:.*?styles\.content/s,
    );
  });
});

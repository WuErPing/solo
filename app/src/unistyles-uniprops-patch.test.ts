/**
 * @vitest-environment node
 *
 * Regression guard: react-native-unistyles' web `withUnistyles` merges the
 * original props (including `uniProps`) into the props forwarded to the wrapped
 * component. For components that render DOM elements (e.g. lucide icons via
 * react-native-svg), `uniProps` leaks onto the DOM and React logs
 * "React does not recognize the `uniProps` prop on a DOM element".
 *
 * Fixed via patch-package (patches/react-native-unistyles+*.patch), which
 * strips `uniProps` from the forwarded props. This test fails if the patch is
 * missing or stops applying (e.g. after an unistyles upgrade).
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";

const require = createRequire(join(__dirname, "..", "package.json"));

describe("unistyles web withUnistyles patch", () => {
  it("strips uniProps from props forwarded to the wrapped component", () => {
    const packageJsonPath = require.resolve("react-native-unistyles/package.json");
    const withUnistylesPath = join(
      dirname(packageJsonPath),
      "lib/module/core/withUnistyles/withUnistyles.js",
    );
    const source = readFileSync(withUnistylesPath, "utf-8");

    expect(source).toContain("propsWithoutUniProps");
  });
});

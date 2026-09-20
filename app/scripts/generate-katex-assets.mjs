#!/usr/bin/env node
// Generates src/components/katex-assets.ts from the installed `katex` package:
// the minified stylesheet with every woff2 font inlined as a base64 data URI.
// The native math WebView loads zero network resources, so formulas render in
// one stable pass instead of popping in when a remote stylesheet/fonts arrive.
//
// Usage: npm run generate:katex-assets   (re-run after changing katex version)

import { createRequire } from "node:module";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const appRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const require = createRequire(path.join(appRoot, "package.json"));

const katexPkgDir = path.dirname(require.resolve("katex/package.json"));
const katexDistDir = path.join(katexPkgDir, "dist");
const katexVersion = JSON.parse(
  fs.readFileSync(path.join(katexPkgDir, "package.json"), "utf8"),
).version;

const sourceCss = fs.readFileSync(path.join(katexDistDir, "katex.min.css"), "utf8");

// Inline every woff2 reference, then drop the trailing woff/ttf fallbacks
// (KaTeX lists woff2 first: `url(...) format("woff2"),url(...) format("woff")`).
let css = sourceCss.replace(/url\(fonts\/([A-Za-z0-9_-]+\.woff2)\)/g, (_match, name) => {
  const font = fs.readFileSync(path.join(katexDistDir, "fonts", name));
  return `url(data:font/woff2;base64,${font.toString("base64")})`;
});
css = css.replace(/,url\(fonts\/[^)]+\.(?:woff|ttf)\) format\("[^"]*"\)/g, "");

if (/url\(fonts\//.test(css)) {
  throw new Error("KaTeX CSS still references unpacked font files; check the rewrite patterns");
}
const fontFaceCount = (css.match(/@font-face/g) ?? []).length;
if (fontFaceCount === 0 || !css.includes("data:font/woff2;base64")) {
  throw new Error("KaTeX CSS inlining produced no font faces");
}

const output = `// GENERATED FILE - do not edit manually.
// Produced by scripts/generate-katex-assets.mjs from katex@${katexVersion}
// (katex.min.css with woff2 fonts inlined as data URIs, so the math WebView
// renders fully offline). Re-run after changing the katex version:
//   npm run generate:katex-assets
export const KATEX_ASSETS_VERSION = ${JSON.stringify(katexVersion)};

export const KATEX_CSS = ${JSON.stringify(css)};
`;

const outPath = path.join(appRoot, "src", "components", "katex-assets.ts");
fs.writeFileSync(outPath, output);
console.log(
  `wrote ${path.relative(appRoot, outPath)} (${(output.length / 1024).toFixed(0)} KiB, ` +
    `${fontFaceCount} font faces, katex@${katexVersion})`,
);

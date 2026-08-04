// Component-level tests for SvgPreview, kept in the E2E suite because they
// need a real browser (iframe srcdoc rendering). The component is bundled
// from repo-local sources with esbuild in beforeAll — no external CDN.
import path from "node:path";
import { buildSync } from "esbuild";
import { expect, test } from "@playwright/test";

let fixtureScript: string;

test.beforeAll(() => {
  const result = buildSync({
    entryPoints: [path.join(__dirname, "helpers", "svg-preview-fixture-entry.tsx")],
    bundle: true,
    write: false,
    format: "iife",
    platform: "browser",
    minify: true,
    logLevel: "silent",
  });
  fixtureScript = result.outputFiles[0].text;
});

async function renderSvgPreview(page: import("@playwright/test").Page, source: string) {
  await page.setContent(`<div id="root"></div>`);
  await page.addScriptTag({ content: fixtureScript });
  await page.evaluate(
    (src) => (window as Window & { __renderSvgPreview?: (s: string) => void }).__renderSvgPreview?.(src),
    source,
  );
}

test.describe("SVG Preview", () => {
  test("renders valid SVG content in iframe", async ({ page }) => {
    await renderSvgPreview(
      page,
      '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>',
    );

    await page.waitForSelector('[data-testid="svg-preview"]');
    const iframe = page.locator('[data-testid="svg-preview"] iframe');
    await expect(iframe).toBeVisible();

    const srcdoc = await iframe.getAttribute("srcdoc");
    expect(srcdoc).toContain("<svg");
    expect(srcdoc).toContain("circle");
    expect(srcdoc).toContain('fill="blue"');
  });

  test("shows error for invalid SVG", async ({ page }) => {
    await renderSvgPreview(page, "not valid svg");

    await page.waitForSelector('[data-testid="svg-preview-error"]');
    const error = page.locator('[data-testid="svg-preview-error"]');
    await expect(error).toContainText("Invalid SVG content");
  });

  test("sanitizes malicious SVG content", async ({ page }) => {
    await renderSvgPreview(
      page,
      '<svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script><circle/></svg>',
    );

    await page.waitForSelector('[data-testid="svg-preview"]');
    const iframe = page.locator('[data-testid="svg-preview"] iframe');
    const srcdoc = await iframe.getAttribute("srcdoc");
    expect(srcdoc).not.toContain("<script>");
    expect(srcdoc).toContain("circle");
  });
});

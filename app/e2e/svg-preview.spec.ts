import { expect, test } from "@playwright/test";

test.describe("SVG Preview", () => {
  test("renders valid SVG content in iframe", async ({ page }) => {
    await page.setContent(`
      <div id="root"></div>
      <script type="module">
        import React from 'https://esm.sh/react@18';
        import { createRoot } from 'https://esm.sh/react-dom@18/client';
        import { SvgPreview } from '/src/components/svg-preview.web.tsx';

        const root = createRoot(document.getElementById('root'));
        root.render(React.createElement(SvgPreview, {
          source: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="blue"/></svg>'
        }));
      </script>
    `);

    await page.waitForSelector('[data-testid="svg-preview"]');
    const iframe = page.locator('[data-testid="svg-preview"] iframe');
    await expect(iframe).toBeVisible();

    const srcdoc = await iframe.getAttribute("srcdoc");
    expect(srcdoc).toContain("<svg");
    expect(srcdoc).toContain("circle");
    expect(srcdoc).toContain('fill="blue"');
  });

  test("shows error for invalid SVG", async ({ page }) => {
    await page.setContent(`
      <div id="root"></div>
      <script type="module">
        import React from 'https://esm.sh/react@18';
        import { createRoot } from 'https://esm.sh/react-dom@18/client';
        import { SvgPreview } from '/src/components/svg-preview.web.tsx';

        const root = createRoot(document.getElementById('root'));
        root.render(React.createElement(SvgPreview, {
          source: 'not valid svg'
        }));
      </script>
    `);

    await page.waitForSelector('[data-testid="svg-preview-error"]');
    const error = page.locator('[data-testid="svg-preview-error"]');
    await expect(error).toContainText("Invalid SVG content");
  });

  test("sanitizes malicious SVG content", async ({ page }) => {
    await page.setContent(`
      <div id="root"></div>
      <script type="module">
        import React from 'https://esm.sh/react@18';
        import { createRoot } from 'https://esm.sh/react-dom@18/client';
        import { SvgPreview } from '/src/components/svg-preview.web.tsx';

        const root = createRoot(document.getElementById('root'));
        root.render(React.createElement(SvgPreview, {
          source: '<svg xmlns="http://www.w3.org/2000/svg"><script>alert("xss")</script><circle/></svg>'
        }));
      </script>
    `);

    await page.waitForSelector('[data-testid="svg-preview"]');
    const iframe = page.locator('[data-testid="svg-preview"] iframe');
    const srcdoc = await iframe.getAttribute("srcdoc");
    expect(srcdoc).not.toContain("<script>");
    expect(srcdoc).toContain("circle");
  });
});

/**
 * E2E test: Verify the ⤓ (End) key on the tmux pane keybar scrolls the pane
 * content view to the bottom (latest output) instead of sending a tmux key.
 *
 * Creates a tmux session with tall content, opens the native pane view,
 * scrolls to the top, clicks ⤓, and asserts the view is back at the bottom.
 */

import { execFileSync } from "node:child_process";
import { mkdtemp, writeFile, chmod, rm } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import { test, expect } from "./fixtures";

test.describe("tmux pane keybar scroll-to-bottom", () => {
  test.describe.configure({ timeout: 90_000 });

  let fakeBinDir: string | null = null;
  let sessionName: string | null = null;

  test.beforeAll(async () => {
    fakeBinDir = await mkdtemp(path.join(tmpdir(), "solo-fake-tall-"));
    const fakeCli = path.join(fakeBinDir, "qodercli");
    await writeFile(
      fakeCli,
      `#!/usr/bin/env node
// Emit many lines so the pane view is scrollable, ending with a marker.
for (let i = 1; i <= 120; i++) {
  process.stdout.write("line-" + String(i).padStart(3, "0") + "\\r\\n");
}
process.stdout.write("BOTTOM-MARKER\\r\\n");
setInterval(() => {}, 60_000);
`,
    );
    await chmod(fakeCli, 0o755);

    sessionName = `solo-tall-${Date.now()}-${process.pid}`;
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      PATH: `${fakeBinDir}${path.delimiter}${process.env.PATH ?? ""}`,
      TMUX: undefined,
    };

    execFileSync("tmux", ["new-session", "-d", "-s", sessionName, "-n", "main", "qodercli"], {
      env,
      timeout: 10_000,
    });

    execFileSync("tmux", ["select-pane", "-T", "qodercli", "-t", `${sessionName}:0.0`], {
      timeout: 5_000,
    });

    await new Promise((resolve) => setTimeout(resolve, 500));
  });

  test.afterAll(async () => {
    if (sessionName) {
      try {
        execFileSync("tmux", ["kill-session", "-t", sessionName], { timeout: 5_000 });
      } catch {
        // Session may already be gone; ignore.
      }
    }
    if (fakeBinDir) {
      await rm(fakeBinDir, { recursive: true, force: true });
    }
  });

  test("clicking ⤓ scrolls the pane view to the latest output", async ({ page }) => {
    test.setTimeout(60_000);

    await page.goto("/tmux-dashboard");

    const agentCard = page.getByText(sessionName, { exact: true }).first();
    await expect(agentCard).toBeVisible({ timeout: 15_000 });
    await agentCard.click();

    // Wait for the captured pane content to render.
    const anchor = page.getByText("BOTTOM-MARKER");
    await expect(anchor).toBeVisible({ timeout: 15_000 });

    const scroll = page.getByTestId("tmux-pane-scroll");
    await expect(scroll).toBeVisible();

    // Locate the actual scrollable element (the one with overflowing content).
    const scrollInfo = () =>
      scroll.evaluate((el) => {
        const findScrollable = (node: Element | null): Element | null => {
          if (!node) return null;
          if (node.scrollHeight > node.clientHeight + 1) return node;
          return findScrollable(node.parentElement);
        };
        const target = findScrollable(el) ?? el;
        return {
          scrollTop: target.scrollTop,
          scrollHeight: target.scrollHeight,
          clientHeight: target.clientHeight,
        };
      });

    // Scroll the view to the top first so the ⤓ action has work to do.
    await scroll.evaluate((el) => {
      const findScrollable = (node: Element | null): Element | null => {
        if (!node) return null;
        if (node.scrollHeight > node.clientHeight + 1) return node;
        return findScrollable(node.parentElement);
      };
      const target = findScrollable(el) ?? el;
      target.scrollTop = 0;
    });
    await page.waitForTimeout(300);

    const atTop = await scrollInfo();
    expect(atTop.scrollTop, "view should start at the top").toBeLessThan(50);

    // Click the floating scroll-to-bottom button overlaid on the content.
    const scrollButton = page.getByTestId("tmux-scroll-to-bottom");
    await expect(scrollButton).toBeVisible({ timeout: 10_000 });
    await scrollButton.click();
    await page.waitForTimeout(400);

    const after = await scrollInfo();
    const distanceFromBottom = after.scrollHeight - after.clientHeight - after.scrollTop;
    expect(
      distanceFromBottom,
      `after clicking ⤓ the view should be at the bottom (scrollTop=${after.scrollTop}, scrollHeight=${after.scrollHeight}, clientHeight=${after.clientHeight})`,
    ).toBeLessThan(50);
  });
});

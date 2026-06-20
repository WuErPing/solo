/**
 * E2E test: Verify horizontal scroll toggle on the tmux pane xterm screen.
 *
 * Creates a tmux session with wide content, opens the xterm view, and tests
 * that the Width toggle button switches between fit and original modes.
 */

import { spawn, execFileSync } from "node:child_process";
import { mkdtemp, writeFile, chmod, rm } from "node:fs/promises";
import path from "node:path";
import { tmpdir } from "node:os";
import { test, expect } from "./fixtures";

test.describe("tmux pane horizontal scroll toggle", () => {
  test.describe.configure({ timeout: 90_000 });

  let fakeBinDir: string | null = null;
  let sessionName: string | null = null;

  test.beforeAll(async () => {
    fakeBinDir = await mkdtemp(path.join(tmpdir(), "solo-fake-wide-"));

    // Create a script that outputs wide content (200 columns) and stays alive.
    const fakeWide = path.join(fakeBinDir, "wide-cli");
    await writeFile(
      fakeWide,
      `#!/usr/bin/env node
// Output a line that is 200 characters wide.
const line = "W".repeat(200);
process.stdout.write(line + "\\r\\n");
process.stdout.write("second line of wide content".padEnd(200, "X") + "\\r\\n");
setInterval(() => {}, 60_000);
`,
    );
    await chmod(fakeWide, 0o755);

    sessionName = `solo-wide-${Date.now()}-${process.pid}`;
    const env: NodeJS.ProcessEnv = {
      ...process.env,
      PATH: `${fakeBinDir}${path.delimiter}${process.env.PATH ?? ""}`,
      TMUX: undefined,
    };

    // Start a detached tmux session running our wide-content script.
    spawn("tmux", ["new-session", "-d", "-s", sessionName, "-n", "main", "wide-cli"], {
      env,
      detached: false,
    });

    // Set the pane title so the daemon's agent detection finds it.
    execFileSync("tmux", ["select-pane", "-T", "wide-cli", "-t", `${sessionName}:0.0`], {
      timeout: 5_000,
    });

    // Give tmux a moment to create the pane.
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

  test("Width toggle switches between fit and original modes", async ({ page }) => {
    test.setTimeout(60_000);

    await page.goto("/tmux-dashboard");

    // Find and click the session card to expand it.
    const card = page.locator(`text=S:${sessionName}`).first();
    await expect(card).toBeVisible({ timeout: 15_000 });
    await card.click();

    // Click the xterm button to open the xterm pane view.
    const xtermButton = page.getByTestId("open-xterm-button").first();
    await expect(xtermButton).toBeVisible({ timeout: 10_000 });
    await xtermButton.click();

    // Wait for the xterm surface to appear.
    const surface = page.getByTestId("tmux-xterm-surface");
    await expect(surface).toBeVisible({ timeout: 15_000 });

    // The Width toggle button should be visible.
    const widthToggle = page.getByTestId("tmux-xterm-width-toggle-button");
    await expect(widthToggle).toBeVisible({ timeout: 10_000 });

    // In fit mode, the button should say "Width".
    await expect(widthToggle).toHaveText("Width");

    // Click to switch to original mode.
    await widthToggle.click();

    // The button should now say "Fit".
    await expect(widthToggle).toHaveText("Fit");

    // Click again to switch back to fit mode.
    await widthToggle.click();

    // The button should say "Width" again.
    await expect(widthToggle).toHaveText("Width");
  });
});

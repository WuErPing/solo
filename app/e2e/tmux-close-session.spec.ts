/**
 * E2E test: Close (kill) a tmux session from the dashboard.
 *
 * Validates that clicking the X button on an agent card shows a confirmation
 * dialog and, after confirming, the session is removed from the dashboard.
 */

import { execFileSync } from "node:child_process";
import { test, expect } from "./fixtures";

test.describe("tmux close session", () => {
  test.describe.configure({ timeout: 90_000 });

  let sessionName: string | null = null;

  test.beforeAll(async () => {
    sessionName = `solo-close-${Date.now()}-${process.pid}`;

    // Create a real tmux session running sleep so it stays alive.
    execFileSync("tmux", [
      "new-session",
      "-d",
      "-s",
      sessionName,
      "-n",
      "main",
      "sleep",
      "300",
    ], { timeout: 5_000 });

    // Give tmux a moment before the first dashboard poll.
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
  });

  test("clicking X and confirming kills the tmux session", async ({ page }) => {
    test.setTimeout(60_000);

    // Accept the confirm dialog when it appears.
    page.on("dialog", async (dialog) => {
      expect(dialog.type()).toBe("confirm");
      await dialog.accept();
    });

    await page.goto("/tmux-dashboard");

    // Find the session card by its session name badge.
    const sessionBadge = page.getByText(sessionName!, { exact: true });
    await expect(sessionBadge).toBeVisible({ timeout: 15_000 });

    // Click the close (X) button on the card.
    const closeButton = page.getByTestId("close-session").first();
    await expect(closeButton).toBeVisible();
    await closeButton.click();

    // After killing, the session badge should disappear from the dashboard.
    await expect(sessionBadge).toBeHidden({ timeout: 10_000 });

    // Mark sessionName as null so afterAll doesn't try to kill it again.
    sessionName = null;
  });

  test("clicking X and cancelling keeps the session alive", async ({ page }) => {
    test.setTimeout(60_000);

    // Create a fresh session for this test.
    const keepSession = `solo-keep-${Date.now()}-${process.pid}`;
    execFileSync("tmux", [
      "new-session",
      "-d",
      "-s",
      keepSession,
      "-n",
      "main",
      "sleep",
      "300",
    ], { timeout: 5_000 });
    await new Promise((resolve) => setTimeout(resolve, 500));

    // Dismiss the confirm dialog (cancel).
    page.on("dialog", async (dialog) => {
      expect(dialog.type()).toBe("confirm");
      await dialog.dismiss();
    });

    await page.goto("/tmux-dashboard");

    const sessionBadge = page.getByText(keepSession, { exact: true });
    await expect(sessionBadge).toBeVisible({ timeout: 15_000 });

    // Click the close button.
    const closeButton = page.getByTestId("close-session").first();
    await expect(closeButton).toBeVisible();
    await closeButton.click();

    // The session should still be visible.
    await expect(sessionBadge).toBeVisible({ timeout: 5_000 });

    // Clean up the session.
    try {
      execFileSync("tmux", ["kill-session", "-t", keepSession], { timeout: 5_000 });
    } catch {
      // Ignore.
    }
  });
});

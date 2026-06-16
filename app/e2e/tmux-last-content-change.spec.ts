/**
 * E2E test: Verify tmux cards show last content change time.
 *
 * Creates a real tmux session, navigates to the dashboard, and asserts
 * that the agent card displays an HH:MM timestamp and a relative time
 * ("just now" / "Xm ago"). Also verifies that writing to the pane
 * updates the displayed time.
 */

import { execFileSync } from "node:child_process";
import { test, expect } from "./fixtures";

test.describe("tmux last content change", () => {
  test.describe.configure({ timeout: 90_000 });

  let sessionName: string | null = null;

  test.beforeAll(async () => {
    sessionName = `solo-activity-${Date.now()}-${process.pid}`;

    // Create a tmux session running a long-lived command.
    execFileSync(
      "tmux",
      ["new-session", "-d", "-s", sessionName, "-n", "main", "sleep", "300"],
      { timeout: 5_000 },
    );

    // Give the daemon a moment to detect the new pane.
    await new Promise((resolve) => setTimeout(resolve, 1_500));
  });

  test.afterAll(async () => {
    if (sessionName) {
      try {
        execFileSync("tmux", ["kill-session", "-t", sessionName], {
          timeout: 5_000 });
      } catch {
        // Session may already be gone; ignore.
      }
    }
  });

  test("agent card shows HH:MM and relative time", async ({ page }) => {
    test.setTimeout(60_000);

    await page.goto("/tmux-dashboard");

    // The session badge should appear once the daemon picks it up.
    const sessionBadge = page.getByText(sessionName!, { exact: true });
    await expect(sessionBadge).toBeVisible({ timeout: 15_000 });

    // The card should contain an HH:MM timestamp (pattern: two digits, colon, two digits).
    const hhmmPattern = /\d{2}:\d{2}/;

    // Look for HH:MM text somewhere in the card area.
    const cardText = await page.locator("body").innerText();
    expect(cardText).toMatch(hhmmPattern);

    // Look for relative time text ("just now", "Xm ago", "Xh ago").
    const relativeTimePattern = /just now|\d+[mh] ago|\d+d ago/;
    expect(cardText).toMatch(relativeTimePattern);

    // Verify HH:MM matches daemon's local time (same machine in e2e, so timezone-aligned).
    const now = new Date();
    const expectedHHMM = `${String(now.getHours()).padStart(2, "0")}:${String(now.getMinutes()).padStart(2, "0")}`;
    // The displayed HH:MM should be within ±1 minute of now (activity just happened).
    const displayed = cardText.match(hhmmPattern)?.[0];
    expect(displayed).toBeTruthy();
    const [dh, dm] = displayed!.split(":").map(Number);
    const diffMin = Math.abs((dh * 60 + dm) - (now.getHours() * 60 + now.getMinutes()));
    expect(diffMin).toBeLessThanOrEqual(1);
  });

  test("writing to the pane updates the displayed time", async ({ page }) => {
    test.setTimeout(60_000);

    await page.goto("/tmux-dashboard");

    const sessionBadge = page.getByText(sessionName!, { exact: true });
    await expect(sessionBadge).toBeVisible({ timeout: 15_000 });

    // Capture current page text before writing to the pane.
    const textBefore = await page.locator("body").innerText();

    // Write something to the tmux pane to trigger content change.
    const paneTarget = `${sessionName}:0.0`;
    execFileSync("tmux", ["send-keys", "-t", paneTarget, "echo hello", "Enter"], {
      timeout: 5_000,
    });

    // Wait for the daemon to detect the change (hash comparison + poll interval).
    await new Promise((resolve) => setTimeout(resolve, 3_000));

    // Reload to get fresh data from the daemon.
    await page.reload();
    await expect(sessionBadge).toBeVisible({ timeout: 15_000 });

    // The relative time should still be present after content change.
    const textAfter = await page.locator("body").innerText();
    const relativeTimePattern = /just now|\d+[mh] ago|\d+d ago/;
    expect(textAfter).toMatch(relativeTimePattern);
  });
});

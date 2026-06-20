import type { Page } from "@playwright/test";

/**
 * Open the xterm view for a tmux session from the dashboard.
 * Navigates to /tmux-dashboard, finds the session card by name, and clicks the
 * "Open in xterm pane" button.
 */
export async function openTmuxPaneXterm(page: Page, sessionName: string): Promise<void> {
  await page.goto("/tmux-dashboard");

  // Find the card that shows this session name.
  const card = page.locator(`text=S:${sessionName}`).first();
  await card.waitFor({ state: "visible", timeout: 30_000 });
  await card.click();

  // Click the xterm button on the expanded card.
  const xtermButton = page.getByTestId("open-xterm-button").first();
  await xtermButton.waitFor({ state: "visible", timeout: 10_000 });
  await xtermButton.click();
}

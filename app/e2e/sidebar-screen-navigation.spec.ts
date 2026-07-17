import { test, expect } from "./fixtures";
import { gotoAppShell } from "./helpers/app";

// The left sidebar only stays mounted on host-scoped routes (/h/:serverId/*).
// Every sidebar destination must therefore navigate to a host-scoped route;
// global routes (/dashboard, /tmux-dashboard, /schedules) unmount the sidebar.
const destinations = [
  {
    testId: "sidebar-sessions",
    urlPattern: /\/h\/[^/]+\/sessions/,
    expectedText: "Sessions",
  },
  {
    testId: "sidebar-loops",
    urlPattern: /\/h\/[^/]+\/loops/,
    expectedText: "Loops",
  },
  {
    testId: "sidebar-schedules",
    urlPattern: /\/h\/[^/]+\/schedules/,
    expectedText: "Schedules",
  },
  {
    testId: "sidebar-dashboard",
    urlPattern: /\/h\/[^/]+\/dashboard/,
    expectedText: "Agents",
  },
  {
    testId: "sidebar-tmux-dashboard",
    urlPattern: /\/h\/[^/]+\/tmux-dashboard/,
    expectedText: "Tmux",
  },
] as const;

test.describe("sidebar screen navigation", () => {
  for (const { testId, urlPattern, expectedText } of destinations) {
    test(`${testId} keeps the sidebar and renders without errors`, async ({ page }) => {
      const pageErrors: string[] = [];
      page.on("pageerror", (error) => pageErrors.push(error.message));

      await gotoAppShell(page);

      const button = page.getByTestId(testId).first();
      await expect(button).toBeVisible({ timeout: 10000 });
      await button.click();

      await expect(page).toHaveURL(urlPattern);
      // The sidebar must stay mounted (host-scoped route keeps app chrome).
      await expect(page.getByTestId("sidebar-sessions").first()).toBeVisible();
      // A crashed (blank) page has no rendered content.
      await expect(page.getByText(expectedText, { exact: true }).first()).toBeVisible({
        timeout: 15000,
      });
      expect(pageErrors).toEqual([]);
    });
  }
});

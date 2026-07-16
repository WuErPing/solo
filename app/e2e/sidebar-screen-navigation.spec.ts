import { test, expect } from "./fixtures";
import { gotoAppShell } from "./helpers/app";

// Regression: on web, expo-router's Stack provided no NavigationContext around
// screen content, so screens calling useIsFocused crashed with
// "Couldn't find a navigation object. Is your component inside NavigationContainer?"
// and the whole page went blank. These sidebar destinations must render cleanly.
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
    urlPattern: /\/schedules/,
    expectedText: "Schedules",
  },
] as const;

test.describe("sidebar screen navigation", () => {
  for (const { testId, urlPattern, expectedText } of destinations) {
    test(`${testId} renders without navigation errors`, async ({ page }) => {
      const pageErrors: string[] = [];
      page.on("pageerror", (error) => pageErrors.push(error.message));

      await gotoAppShell(page);

      const button = page.getByTestId(testId).first();
      await expect(button).toBeVisible({ timeout: 10000 });
      await button.click();

      await expect(page).toHaveURL(urlPattern);
      // A crashed (blank) page has no rendered content.
      await expect(page.getByText(expectedText, { exact: true }).first()).toBeVisible({
        timeout: 15000,
      });
      expect(pageErrors).toEqual([]);
    });
  }
});

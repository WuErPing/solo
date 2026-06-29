import { test, expect } from "./fixtures";
import { gotoAppShell } from "./helpers/app";

test.describe("loop crud", () => {
  test("creates, updates, and deletes a loop end-to-end", async ({ page }) => {
    await gotoAppShell(page);

    // Navigate to the Loops page via the sidebar.
    const loopsSidebarButton = page.getByTestId("sidebar-loops").first();
    await expect(loopsSidebarButton).toBeVisible({ timeout: 10000 });
    await loopsSidebarButton.click();
    await expect(page).toHaveURL(/\/h\/[^/]+\/loops/);

    // Start creating a loop.
    await page.getByText("New Loop", { exact: true }).first().click();
    await expect(page).toHaveURL(/\/h\/[^/]+\/loops\/create/);

    const loopName = `e2e-loop-${Date.now()}`;
    await page.getByPlaceholder("e.g. Fix CI").first().fill(loopName);
    await page
      .getByPlaceholder("e.g. Fix all test failures until CI passes")
      .first()
      .fill("Run a trivial successful command");
    await page.getByPlaceholder("e.g. /Users/me/project").first().fill("/tmp");
    await page.getByPlaceholder("e.g. go test ./...").first().fill("true");

    // Limit execution so the loop finishes quickly.
    const maxIterationsInput = page
      .locator('div:has-text("Max Iterations") + input, div:has-text("Max Iterations"):has(~ input) input')
      .first();
    await maxIterationsInput.fill("1");

    await page.getByText("Create Loop", { exact: true }).first().click();

    // We should land on the detail page (not the create route).
    await expect(page).toHaveURL(/\/h\/[^/]+\/loops\/(?!create)[^/]+$/);
    await expect(page.getByTestId("loop-detail-name").first()).toBeVisible({ timeout: 10000 });

    // Wait for the loop to finish executing before attempting delete.
    await expect
      .poll(
        async () => {
          const status = page.getByTestId("loop-detail-status").first();
          if (!(await status.isVisible().catch(() => false))) {
            return "";
          }
          return (await status.innerText()).trim();
        },
        { timeout: 60000 },
      )
      .toMatch(/succeeded|failed|stopped/);

    // Update the loop name from the detail page.
    await page.getByText("Edit", { exact: true }).first().click();
    const editNameInput = page.getByPlaceholder("Loop name").first();
    await expect(editNameInput).toBeVisible();
    const renamedName = `${loopName}-renamed`;
    await editNameInput.fill(renamedName);
    await page.getByText("Save", { exact: true }).first().click();

    await expect(page.getByText(renamedName).first()).toBeVisible({ timeout: 10000 });

    // Delete the loop.
    await page.getByText("Delete", { exact: true }).first().click();
    await page.getByText("Delete", { exact: true }).nth(1).click();

    // We should be back on the list page with no loops.
    await expect(page).toHaveURL(/\/h\/[^/]+\/loops/);
    await expect(page.getByText("No loops found.").first()).toBeVisible({ timeout: 10000 });
  });
});

import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { expect, test } from "./fixtures";
import { gotoAppShell } from "./helpers/app";

// Regression: the bottom-right "Add project" entry opened a path prompt with no
// confirmable action, so a path that was not already in the suggestion list left
// the user stranded on a blank panel.
test("add project picker confirms a freshly typed path and opens it", async ({ page }) => {
  const projectPath = mkdtempSync(path.join(tmpdir(), "solo-e2e-picker-"));

  try {
    await gotoAppShell(page);

    const browserDialogs: string[] = [];
    page.on("dialog", (dialog) => {
      browserDialogs.push(dialog.message());
      void dialog.dismiss();
    });
    await page.locator('[data-testid="sidebar-add-project"]').first().click();

    const input = page.locator('[data-testid="project-picker-input"]');
    await expect(input).toBeVisible();
    expect(browserDialogs).toEqual([]);

    const submit = page.locator('[data-testid="project-picker-submit"]');
    await expect(submit).toBeVisible();
    await expect(submit).toBeDisabled();

    await input.fill(projectPath);

    await expect(page.getByText("Use this path")).toBeVisible();
    await expect(submit).toBeEnabled();

    await submit.click();

    await expect(input).toBeHidden();
    await expect(page.getByText(path.basename(projectPath)).first()).toBeVisible();

    await page.locator('[data-testid="sidebar-add-project"]').first().click();
    await expect(input).toBeVisible();
    const recentDirectory = page.getByText(projectPath, { exact: true }).last();
    await expect(recentDirectory).toBeVisible();
    await recentDirectory.click();
    await expect(input).toBeHidden();
  } finally {
    rmSync(projectPath, { recursive: true, force: true });
  }
});

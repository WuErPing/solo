import { test, expect } from "./fixtures";
import { gotoAppShell } from "./helpers/app";
import { createTempGitRepo } from "./helpers/workspace";
import { connectWorkspaceSetupClient } from "./helpers/workspace-setup";

test("sidebar New agent opens a draft and selects an installed agent", async ({ page }) => {
  const client = await connectWorkspaceSetupClient();
  const repo = await createTempGitRepo("sidebar-new-agent-");

  try {
    const result = await client.openProject(repo.path);
    if (!result.workspace || result.error) {
      throw new Error(result.error ?? "Failed to open test project");
    }

    await gotoAppShell(page);
    const workspaceRow = page.getByTestId(
      `sidebar-workspace-row-${process.env.E2E_SERVER_ID}:${result.workspace.id}`,
    );
    await expect(workspaceRow).toBeVisible({ timeout: 30_000 });
    await workspaceRow.click();
    await expect(page).toHaveURL(/\/workspace\//);

    const tabs = page.locator('[data-testid^="workspace-tab-"]');
    const initialTabCount = await tabs.count();
    await page.getByTestId("sidebar-new-agent").first().click();
    await expect(tabs).toHaveCount(initialTabCount + 1);

    const modelSelector = page
      .locator('[data-testid="combined-model-selector"]:visible')
      .first();
    await modelSelector.click();
    const dialog = page.getByRole("dialog").last();
    await dialog.getByText("Codex", { exact: true }).click();
    await dialog.getByText("Kimi", { exact: true }).click();
    await dialog.getByText("Auto", { exact: true }).first().click();
    await expect(modelSelector).toContainText("Auto");

    const composer = page.getByRole("textbox", { name: "Message agent..." });
    await composer.fill("Task started from the sidebar");
    await expect(composer).toHaveValue("Task started from the sidebar");
  } finally {
    await client.close();
    await repo.cleanup();
  }
});

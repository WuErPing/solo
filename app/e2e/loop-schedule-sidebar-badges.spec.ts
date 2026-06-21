/**
 * E2E test: Verify sidebar shows correct loop and schedule count badges.
 *
 * Creates a temp git repo, opens it as a project, then creates a loop and
 * a schedule with cwd in the repo. Asserts the sidebar project row shows
 * loop count 1 and schedule count 1. Verifies badge click navigates to
 * the correct page.
 */

import path from "node:path";
import { test, expect } from "./fixtures";
import { gotoAppShell } from "./helpers/app";
import { createTempGitRepo } from "./helpers/workspace";
import { connectWorkspaceSetupClient } from "./helpers/workspace-setup";

async function waitForSidebarProject(page: import("@playwright/test").Page, projectName: string) {
  const row = page
    .locator('[data-testid^="sidebar-project-row-"]')
    .filter({ hasText: projectName })
    .first();
  await expect(row).toBeVisible({ timeout: 30_000 });
  return row;
}

test.describe("loop and schedule sidebar badges", () => {
  test.describe.configure({ timeout: 120_000 });

  let repoPath: string;
  let repoCleanup: () => Promise<void>;
  let loopId: string | null = null;
  let scheduleId: string | null = null;

  test.beforeAll(async () => {
    const repo = await createTempGitRepo("solo-badge-");
    repoPath = repo.path;
    repoCleanup = repo.cleanup;

    const client = await connectWorkspaceSetupClient();
    try {
      await client.openProject(repoPath);

      // Create a loop with cwd in the repo.
      // Use a long sleep so the loop stays running for the test duration.
      const loopResult = await client.loopRun({
        prompt: "sleep 300",
        cwd: repoPath,
        name: "e2e-badge-test-loop",
        maxIterations: 1,
        sleepMs: 60_000,
      });
      loopId = loopResult.loop?.id ?? null;

      // Create a schedule with new-agent target pointing to the repo.
      const scheduleResult = await client.scheduleCreate({
        prompt: "echo hello",
        name: "e2e-badge-test-schedule",
        cadence: { type: "every", everyMs: 60_000 },
        target: {
          type: "new-agent",
          config: {
            provider: "claude",
            cwd: repoPath,
          },
        },
      });
      scheduleId = scheduleResult.schedule?.id ?? null;
    } finally {
      await client.close();
    }

    // Give the daemon time to register the loop and schedule.
    await new Promise((resolve) => setTimeout(resolve, 2_000));
  });

  test.afterAll(async () => {
    const client = await connectWorkspaceSetupClient();
    try {
      if (loopId) {
        await client.loopStop(loopId).catch(() => {});
      }
      if (scheduleId) {
        await client.scheduleDelete({ id: scheduleId }).catch(() => {});
      }
    } finally {
      await client.close();
    }
    await repoCleanup();
  });

  test("sidebar project row shows loop and schedule count badges", async ({ page }) => {
    test.setTimeout(60_000);

    await gotoAppShell(page);
    const projectRow = await waitForSidebarProject(page, path.basename(repoPath));

    // The loop badge should show "1".
    const loopBadge = projectRow.locator('[data-testid="loop-badge"]').first();
    await expect(loopBadge).toBeVisible({ timeout: 15_000 });
    await expect(loopBadge).toContainText("1");

    // The schedule badge should show "1".
    const scheduleBadge = projectRow.locator('[data-testid="schedule-badge"]').first();
    await expect(scheduleBadge).toBeVisible({ timeout: 15_000 });
    await expect(scheduleBadge).toContainText("1");
  });

  test("clicking loop badge navigates to loops page", async ({ page }) => {
    test.setTimeout(60_000);

    await gotoAppShell(page);
    const projectRow = await waitForSidebarProject(page, path.basename(repoPath));

    const loopBadge = projectRow.locator('[data-testid="loop-badge"]').first();
    await expect(loopBadge).toBeVisible({ timeout: 15_000 });
    await loopBadge.click();

    // Should navigate to the loops page.
    await expect(page).toHaveURL(/\/loops/, { timeout: 15_000 });
  });

  test("clicking schedule badge navigates to schedules page", async ({ page }) => {
    test.setTimeout(60_000);

    await gotoAppShell(page);
    const projectRow = await waitForSidebarProject(page, path.basename(repoPath));

    const scheduleBadge = projectRow.locator('[data-testid="schedule-badge"]').first();
    await expect(scheduleBadge).toBeVisible({ timeout: 15_000 });
    await scheduleBadge.click();

    // Should navigate to the schedules page.
    await expect(page).toHaveURL(/\/schedules/, { timeout: 15_000 });
  });
});

/**
 * E2E test: Verify sidebar shows correct tmux pane and agent count badges.
 *
 * Creates a temp git repo, opens it as a project, then creates 4 tmux
 * sessions (2 agent, 2 regular pane) with cwd in the repo. Asserts the
 * sidebar project row shows pane count 4 and agent count 2.
 */

import { execFileSync } from "node:child_process";
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

test.describe("tmux pane counts in sidebar", () => {
  test.describe.configure({ timeout: 120_000 });

  let repoPath: string;
  let repoCleanup: () => Promise<void>;
  let sessionNames: string[] = [];

  test.beforeAll(async () => {
    // Create a temp git repo and open it as a project.
    const repo = await createTempGitRepo("solo-pane-counts-");
    repoPath = repo.path;
    repoCleanup = repo.cleanup;

    const client = await connectWorkspaceSetupClient();
    try {
      await client.openProject(repoPath);
    } finally {
      await client.close();
    }

    // Create 4 tmux sessions with cwd in the repo.
    // 2 agents (pane title set to known agent names) + 2 regular panes.
    const agentNames = ["claude", "pi"];
    const ts = Date.now();

    for (let i = 0; i < 4; i++) {
      const name = `solo-counts-${ts}-${i}-${process.pid}`;
      sessionNames.push(name);

      execFileSync(
        "tmux",
        ["new-session", "-d", "-s", name, "-n", "main", "-c", repoPath, "sleep", "300"],
        { timeout: 5_000 },
      );

      // For agent sessions, set the pane title to a known agent name.
      if (i < agentNames.length) {
        execFileSync(
          "tmux",
          ["select-pane", "-T", agentNames[i], "-t", `${name}:0.0`],
          { timeout: 5_000 },
        );
      }
    }

    // Give the daemon time to detect all new panes.
    await new Promise((resolve) => setTimeout(resolve, 2_000));
  });

  test.afterAll(async () => {
    for (const name of sessionNames) {
      try {
        execFileSync("tmux", ["kill-session", "-t", name], { timeout: 5_000 });
      } catch {
        // Session may already be gone; ignore.
      }
    }
    await repoCleanup();
  });

  test("sidebar project row shows pane count 4 and agent count 2", async ({ page }) => {
    test.setTimeout(60_000);

    await gotoAppShell(page);
    const projectRow = await waitForSidebarProject(page, path.basename(repoPath));

    // The pane badge should show "4" (2 agents + 2 regular panes = 4 total).
    await expect(projectRow.getByText("4")).toBeVisible({ timeout: 15_000 });

    // The agent badge should show "2".
    await expect(projectRow.getByText("2")).toBeVisible({ timeout: 15_000 });
  });

  test("clicking pane count badge navigates to tmux dashboard with dir filter", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    await gotoAppShell(page);
    const projectRow = await waitForSidebarProject(page, path.basename(repoPath));

    // The pane badge is a Pressable containing the pane count "4".
    // Click it to navigate to tmux dashboard with dir filter.
    const paneBadge = projectRow.getByText("4");
    await expect(paneBadge).toBeVisible({ timeout: 15_000 });
    await paneBadge.click();

    // Should navigate to tmux-dashboard with a dir query param.
    await expect(page).toHaveURL(/\/tmux-dashboard/, { timeout: 15_000 });
    await expect(page).toHaveURL(new RegExp(encodeURIComponent(repoPath)), { timeout: 15_000 });

    // The dashboard should show a filter banner with the project path.
    const banner = page.getByText(path.basename(repoPath));
    await expect(banner).toBeVisible({ timeout: 15_000 });
  });
});

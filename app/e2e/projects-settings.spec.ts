import { readFile } from "node:fs/promises";
import path from "node:path";
import { expect, test as base, type Page } from "./fixtures";
import { gotoAppShell } from "./helpers/app";
import { connectNewWorkspaceDaemonClient, openProjectViaDaemon } from "./helpers/new-workspace";
import { createTempGitRepo } from "./helpers/workspace";

const updatedSetup = ["npm install", "npm run build"];

interface ProjectsSettingsProject {
  name: string;
  path: string;
}

interface ProjectsSettingsFixtures {
  editableProject: ProjectsSettingsProject;
}

const initialSoloConfig = {
  worktree: {
    setup: ["echo initial setup"],
    teardown: "echo cleanup",
    customWorktreeField: "preserved",
  },
  scripts: {
    dev: {
      command: "npm run dev",
      type: "server",
      port: 3000,
      customScriptField: "preserved",
    },
  },
  customTopLevelField: "preserved",
};

const test = base.extend<ProjectsSettingsFixtures>({
  editableProject: async ({ page: _page }, provide) => {
    const client = await connectNewWorkspaceDaemonClient();
    const repo = await createTempGitRepo("projects-settings-", {
      soloConfig: initialSoloConfig,
    });
    const openedProject = await openProjectViaDaemon(client, repo.path);

    await provide({
      name: openedProject.projectDisplayName,
      path: repo.path,
    });

    await client.close();
    await repo.cleanup();
  },
});

async function openProjects(page: Page): Promise<void> {
  await gotoAppShell(page);
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await expect(page).toHaveURL(/\/projects$/);
}

async function openProjectSettings(page: Page, projectName: string): Promise<void> {
  await page.getByRole("button", { name: `Edit ${projectName}` }).click();
  await expect(page.getByRole("textbox", { name: "Worktree setup commands" })).toBeVisible({
    timeout: 30_000,
  });
}

async function editWorktreeSetup(page: Page, setupCommands: string[]): Promise<void> {
  await page.getByRole("textbox", { name: "Worktree setup commands" }).fill(setupCommands.join("\n"));
}

async function saveProjectConfig(page: Page): Promise<void> {
  await page.getByRole("button", { name: "Save project config" }).click();
}

async function expectProjectConfigSaved(project: ProjectsSettingsProject): Promise<void> {
  await expect
    .poll(
      async () => {
        const contents = await readProjectConfigFile(project);
        return JSON.parse(contents) as unknown;
      },
      {
        timeout: 30_000,
      },
    )
    .toMatchObject({
      worktree: {
        setup: updatedSetup,
        teardown: initialSoloConfig.worktree.teardown,
        customWorktreeField: initialSoloConfig.worktree.customWorktreeField,
      },
      scripts: {
        dev: {
          command: initialSoloConfig.scripts.dev.command,
          type: initialSoloConfig.scripts.dev.type,
          port: initialSoloConfig.scripts.dev.port,
          customScriptField: initialSoloConfig.scripts.dev.customScriptField,
        },
      },
      customTopLevelField: initialSoloConfig.customTopLevelField,
    });

  const savedConfig = await readProjectConfigFile(project);
  expect(savedConfig).toBe(`${JSON.stringify(JSON.parse(savedConfig), null, 2)}\n`);
}

async function readProjectConfigFile(project: ProjectsSettingsProject): Promise<string> {
  return readFile(path.join(project.path, "solo.json"), "utf8");
}

test.describe("Projects settings", () => {
  test("user edits worktree setup from the projects page", async ({ page, editableProject }) => {
    await openProjects(page);
    await openProjectSettings(page, editableProject.name);
    await editWorktreeSetup(page, updatedSetup);
    await saveProjectConfig(page);
    await expectProjectConfigSaved(editableProject);
  });
});

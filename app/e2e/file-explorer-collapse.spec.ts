import { expect, test } from "./fixtures";
import {
  collapseFolder,
  expandFolder,
  expectExplorerEntryHidden,
  expectExplorerEntryVisible,
  expectFileTabOpen,
  openFileExplorer,
  openFileFromExplorer,
} from "./helpers/file-explorer";
import { gotoWorkspace } from "./helpers/launcher";
import { createTempGitRepo } from "./helpers/workspace";
import {
  connectWorkspaceSetupClient,
  type WorkspaceSetupDaemonClient,
} from "./helpers/workspace-setup";

let tempRepo: { path: string; cleanup: () => Promise<void> };
let workspaceId: string;
let seedClient: WorkspaceSetupDaemonClient;

test.beforeAll(async () => {
  tempRepo = await createTempGitRepo("file-explorer-collapse-", {
    files: [
      { path: "assets/logo.png", content: "image bytes for explorer e2e\n" },
      { path: "docs/guide.md", content: "# Guide\n" },
    ],
  });
  seedClient = await connectWorkspaceSetupClient();
  const result = await seedClient.openProject(tempRepo.path);
  if (!result.workspace) {
    throw new Error(result.error ?? "Failed to seed workspace");
  }
  workspaceId = String(result.workspace.id);
});

test.afterAll(async () => {
  await seedClient?.close();
  await tempRepo?.cleanup();
});

test.describe("File explorer collapse", () => {
  test("collapses an opened image file parent folder and still expands other folders", async ({
    page,
  }) => {
    await gotoWorkspace(page, workspaceId);
    await openFileExplorer(page);

    await expandFolder(page, "assets");
    await expectExplorerEntryVisible(page, "logo.png");

    await openFileFromExplorer(page, "logo.png");
    await expectFileTabOpen(page, "assets/logo.png");

    await collapseFolder(page, "assets");
    await expectExplorerEntryHidden(page, "logo.png");

    await expandFolder(page, "docs");
    await expectExplorerEntryVisible(page, "guide.md");
  });

  test("prunes a deleted restored directory and recovers to the project root", async ({ page }) => {
    const missingPath = ".VSCodeCounter/2026-05-19_04-56-10";
    await page.addInitScript(
      ({ stateKey, stalePath }) => {
        const stored = JSON.parse(localStorage.getItem("panel-state") ?? "{}") as {
          state?: Record<string, unknown>;
          version?: number;
        };
        const state = stored.state ?? {};
        const expandedPathsByWorkspace =
          (state.expandedPathsByWorkspace as Record<string, string[]> | undefined) ?? {};
        expandedPathsByWorkspace[stateKey] = [".", stalePath, "assets"];
        state.expandedPathsByWorkspace = expandedPathsByWorkspace;
        localStorage.setItem("panel-state", JSON.stringify({ state, version: 10 }));
      },
      { stateKey: `workspace:${workspaceId}`, stalePath: missingPath },
    );

    await gotoWorkspace(page, workspaceId);
    await openFileExplorer(page);
    await expectExplorerEntryVisible(page, "logo.png");

    const expandedPaths = await page.evaluate((stateKey) => {
      const stored = JSON.parse(localStorage.getItem("panel-state") ?? "{}") as {
        state?: { expandedPathsByWorkspace?: Record<string, string[]> };
      };
      return stored.state?.expandedPathsByWorkspace?.[stateKey] ?? [];
    }, `workspace:${workspaceId}`);
    expect(expandedPaths).toEqual([".", "assets"]);
  });
});

// PlantUML fences render through the daemon using a locally installed
// plantuml binary. The whole suite shares one daemon, so skip when the host
// has no PlantUML (e.g. CI runners) — the daemon then correctly shows the
// actionable error state instead.
import { execFileSync } from "node:child_process";
import { expect, test } from "./fixtures";
import {
  expectFileTabOpen,
  expandFolder,
  openFileExplorer,
  openFileFromExplorer,
} from "./helpers/file-explorer";
import { gotoWorkspace } from "./helpers/launcher";
import { createTempGitRepo } from "./helpers/workspace";
import {
  connectWorkspaceSetupClient,
  type WorkspaceSetupDaemonClient,
} from "./helpers/workspace-setup";

function hasPlantUmlBinary(): boolean {
  try {
    execFileSync("which", ["plantuml"], { stdio: "ignore" });
    return true;
  } catch {
    return false;
  }
}

const plantUmlAvailable = hasPlantUmlBinary();

let tempRepo: { path: string; cleanup: () => Promise<void> };
let workspaceId: string;
let seedClient: WorkspaceSetupDaemonClient;

test.beforeAll(async () => {
  tempRepo = await createTempGitRepo("plantuml-preview-", {
    files: [
      {
        path: "docs/diagram.md",
        content: [
          "# Diagram",
          "",
          "```plantuml",
          "@startuml",
          "Alice -> Bob: hello",
          "Bob --> Alice: hi",
          "@enduml",
          "```",
          "",
        ].join("\n"),
      },
      {
        path: "docs/empty.md",
        content: ["# Empty", "", "```plantuml", "```", ""].join("\n"),
      },
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

test.describe("PlantUML preview", () => {
  test.skip(!plantUmlAvailable, "plantuml binary not installed on the daemon host");

  test("renders a fence and opens the fullscreen viewer", async ({ page }) => {
    await gotoWorkspace(page, workspaceId);
    await openFileExplorer(page);
    await expandFolder(page, "docs");
    await openFileFromExplorer(page, "diagram.md");
    await expectFileTabOpen(page, "docs/diagram.md");

    const preview = page.getByTestId("plantuml-preview").first();
    // First render includes a JVM cold start on the daemon host.
    await expect(preview).toBeVisible({ timeout: 60_000 });
    await expect(preview.locator("svg").first()).toBeVisible();
    await expect(preview).toContainText("Alice");
    await expect(page.getByTestId("plantuml-preview-error")).toHaveCount(0);

    await preview.getByRole("button", { name: "Expand diagram" }).click();
    const close = page.getByTestId("diagram-lightbox-close");
    await expect(close).toBeVisible();
    await close.click();
    await expect(close).toHaveCount(0);
  });

  test("falls back to the source plus error when nothing can be rendered", async ({ page }) => {
    await gotoWorkspace(page, workspaceId);
    await openFileExplorer(page);
    await expandFolder(page, "docs");
    await openFileFromExplorer(page, "empty.md");
    await expectFileTabOpen(page, "docs/empty.md");

    const error = page.getByTestId("plantuml-preview-error").first();
    await expect(error).toBeVisible({ timeout: 30_000 });
    await expect(error).toContainText("PlantUML");
  });
});

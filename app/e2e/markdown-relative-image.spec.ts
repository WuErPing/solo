// Markdown previews resolve relative image references (e.g. `imgs/foo.svg`)
// against the document's own directory and fetch them from the daemon. Without
// that, the markdown renderer prefixes its `https://` default handler and
// requests a bogus URL, so imported diagrams never appear.
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

const SVG_LABEL = "relative-svg-ok";
const SVG_SOURCE = [
  '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 100" font-family="sans-serif">',
  '<rect width="240" height="100" fill="#ffffff"/>',
  `<text x="12" y="40" font-size="16" fill="#111827">${SVG_LABEL}</text>`,
  "</svg>",
].join("");

let tempRepo: { path: string; cleanup: () => Promise<void> };
let workspaceId: string;
let seedClient: WorkspaceSetupDaemonClient;

test.beforeAll(async () => {
  tempRepo = await createTempGitRepo("markdown-image-", {
    files: [
      {
        path: "docs/guide.md",
        content: [
          "# Guide",
          "",
          `![Architecture](imgs/architecture.svg)`,
          "",
          "![Missing Diagram](imgs/does-not-exist.svg)",
          "",
        ].join("\n"),
      },
      { path: "docs/imgs/architecture.svg", content: SVG_SOURCE },
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

async function openGuide(page: Parameters<typeof gotoWorkspace>[0]): Promise<void> {
  await gotoWorkspace(page, workspaceId);
  await openFileExplorer(page);
  await expandFolder(page, "docs");
  await openFileFromExplorer(page, "guide.md");
  await expectFileTabOpen(page, "docs/guide.md");
}

test.describe("Markdown relative images", () => {
  test("renders an imported SVG and opens the fullscreen viewer", async ({ page }) => {
    await openGuide(page);

    const figure = page.getByTestId("markdown-file-image-svg").first();
    await expect(figure).toBeVisible({ timeout: 30_000 });
    await expect(figure.locator("svg").first()).toBeVisible();
    await expect(figure).toContainText(SVG_LABEL);

    await figure.getByRole("button", { name: "Expand Architecture" }).click();
    const close = page.getByTestId("diagram-lightbox-close");
    await expect(close).toBeVisible();
    await close.click();
    await expect(close).toHaveCount(0);
  });

  test("falls back to the alt text when the referenced file is missing", async ({ page }) => {
    await openGuide(page);

    const missing = page.getByTestId("markdown-file-image-missing").first();
    await expect(missing).toBeVisible({ timeout: 30_000 });
    await expect(missing).toContainText("Missing Diagram");
  });
});

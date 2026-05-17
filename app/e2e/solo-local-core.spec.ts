import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test, type Page } from "./fixtures";
import { buildHostAgentDetailRoute, buildHostWorkspaceRoute } from "@/utils/host-routes";
import { gotoAppShell, openSettings } from "./helpers/app";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import { createTempGitRepo } from "./helpers/workspace";
import {
  expectFileTabOpen,
  expectExplorerEntryVisible,
  openFileExplorer,
  openFileFromExplorer,
} from "./helpers/file-explorer";

interface CoreDaemonClient {
  connect(): Promise<void>;
  close(): Promise<void>;
  openProject(cwd: string): Promise<{
    workspace: {
      id: string;
      name: string;
      workspaceDirectory: string;
      projectRootPath: string;
    } | null;
    error: string | null;
  }>;
  createAgent(input: {
    provider?: "mock";
    cwd?: string;
    config?: {
      provider: "mock";
      cwd: string;
      title?: string;
    };
    initialPrompt?: string;
    labels?: Record<string, string>;
  }): Promise<{ id: string; cwd: string; title?: string | null }>;
  sendMessage(agentId: string, text: string): Promise<void>;
  fetchAgentTimeline(agentId: string, options?: { direction?: "tail"; limit?: number }): Promise<{
    entries: Array<{ item: unknown }>;
  }>;
  deleteAgent(agentId: string): Promise<void>;
}

function getDaemonWsUrl(): string {
  const daemonPort = process.env.E2E_DAEMON_PORT;
  if (!daemonPort) {
    throw new Error("E2E_DAEMON_PORT is not set.");
  }
  return `ws://127.0.0.1:${daemonPort}/ws`;
}

async function connectCoreDaemonClient(): Promise<CoreDaemonClient> {
  const repoRoot = path.resolve(__dirname, "../..");
  const moduleUrl = pathToFileURL(
    path.join(repoRoot, "app-bridge/dist/client/daemon-client.js"),
  ).href;
  const mod = (await import(moduleUrl)) as {
    DaemonClient: new (config: {
      url: string;
      clientId: string;
      clientType: "cli";
      webSocketFactory?: NodeWebSocketFactory;
    }) => CoreDaemonClient;
  };
  const client = new mod.DaemonClient({
    url: getDaemonWsUrl(),
    clientId: `solo-local-core-${randomUUID()}`,
    clientType: "cli",
    webSocketFactory: createNodeWebSocketFactory(),
  });
  await client.connect();
  return client;
}

function getServerId(): string {
  const serverId = process.env.E2E_SERVER_ID;
  if (!serverId) {
    throw new Error("E2E_SERVER_ID is not set.");
  }
  return serverId;
}

function workspaceRow(page: Page, serverId: string, workspaceId: string) {
  return page.getByTestId(`sidebar-workspace-row-${serverId}:${workspaceId}`).first();
}

async function waitForTimelineText(
  client: CoreDaemonClient,
  agentId: string,
  text: string,
): Promise<void> {
  await expect
    .poll(
      async () => {
        const timeline = await client.fetchAgentTimeline(agentId, { direction: "tail", limit: 50 });
        return timeline.entries.some((entry) => JSON.stringify(entry.item).includes(text));
      },
      { timeout: 30_000 },
    )
    .toBe(true);
}

test.describe("Solo local web core interactions", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let workspaceId: string;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient();
    repo = await createTempGitRepo("solo-local-core-", {
      files: [
        { path: "src/app.ts", content: "export const answer = 42;\n" },
        { path: "docs/core.md", content: "# Core flow\n" },
      ],
    });

    const workspace = await client.openProject(repo.path);
    if (!workspace.workspace) {
      throw new Error(workspace.error ?? `Failed to open project ${repo.path}`);
    }
    workspaceId = String(workspace.workspace.id);

    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Solo local web core",
      },
      initialPrompt: "Core web e2e initial prompt",
      labels: { "solo.e2e": "local-core" },
    });
    agentId = agent.id;
    await waitForTimelineText(client, agent.id, "Mock response to: Core web e2e initial prompt");
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("validates local daemon, workspace, agent, file explorer, and settings from web", async ({
    page,
  }) => {
    const serverId = getServerId();
    if (!agentId) {
      throw new Error("agentId was not initialized.");
    }

    await gotoAppShell(page);
    await page.goto(buildHostWorkspaceRoute(serverId, workspaceId));

    await expect(workspaceRow(page, serverId, workspaceId)).toBeVisible({ timeout: 30_000 });
    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByTestId("workspace-header-title").filter({ visible: true }).first(),
    ).toHaveText("main", { timeout: 30_000 });

    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByText("Mock response to: Core web e2e initial prompt", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    await openFileExplorer(page);
    await expectExplorerEntryVisible(page, "src");
    await page.getByText("src", { exact: true }).click();
    await openFileFromExplorer(page, "app.ts");
    await expectFileTabOpen(page, "src/app.ts");

    await openSettings(page);
    await page.getByTestId(`settings-host-entry-${serverId}`).click();
    await expect(page.getByTestId(`settings-host-page-${serverId}`)).toBeVisible();
    await expect(page.getByTestId("host-page-connections-card")).toContainText(
      process.env.E2E_DAEMON_PORT ?? "",
    );
  });

  test("syncs CLI-created agent workspace into an already-open web sidebar", async ({ page }) => {
    const serverId = getServerId();
    await gotoAppShell(page);
    await page.goto(`/h/${encodeURIComponent(serverId)}/open-project`);

    const cliCreatedRepo = await createTempGitRepo("solo-local-core-cli-created-");
    let cliCreatedAgentId: string | null = null;

    try {
      const agent = await client.createAgent({
        config: {
          provider: "mock",
          cwd: cliCreatedRepo.path,
          title: "CLI-created core agent",
        },
        labels: { "solo.e2e": "cli-created" },
      });
      cliCreatedAgentId = agent.id;

      await expect(workspaceRow(page, serverId, cliCreatedRepo.path)).toBeVisible({
        timeout: 30_000,
      });
      await workspaceRow(page, serverId, cliCreatedRepo.path).click();
      await expect(page).toHaveURL(buildHostWorkspaceRoute(serverId, cliCreatedRepo.path));
    } finally {
      if (cliCreatedAgentId) {
        await client.deleteAgent(cliCreatedAgentId).catch(() => undefined);
      }
      await cliCreatedRepo.cleanup();
    }
  });
});

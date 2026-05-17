import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test, type Page } from "./fixtures";
import { buildHostAgentDetailRoute, buildHostWorkspaceRoute } from "@/utils/host-routes";
import { gotoAppShell } from "./helpers/app";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import { createTempGitRepo } from "./helpers/workspace";

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
    clientId: `relay-timeout-regression-${randomUUID()}`,
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

async function agentPanelIsLoaded(page: Page): Promise<boolean> {
  const loading = page.getByTestId("agent-loading").first();
  const notFound = page.getByTestId("agent-not-found").first();
  const error = page.getByTestId("agent-load-error").first();
  return (
    !(await loading.isVisible().catch(() => false)) &&
    !(await notFound.isVisible().catch(() => false)) &&
    !(await error.isVisible().catch(() => false))
  );
}

test.describe("Relay data socket timeout + reconnect regression", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let workspaceId: string;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient();
    repo = await createTempGitRepo("relay-timeout-regression-", {
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
        title: "Relay timeout regression agent",
      },
      initialPrompt: "Hello from relay timeout regression test",
      labels: { "solo.e2e": "relay-timeout-regression" },
    });
    agentId = agent.id;
    await waitForTimelineText(client, agent.id, "Mock response to: Hello from relay timeout regression test");
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("agent detail page recovers after page reload without stuck loading", async ({ page }) => {
    const serverId = getServerId();
    if (!agentId) {
      throw new Error("agentId was not initialized.");
    }

    // Navigate to the agent detail page.
    await gotoAppShell(page);
    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));

    // Verify the agent panel loads and shows the expected content.
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Reload the page to simulate app restart / reconnect.
    await page.reload();

    // After reload, the agent panel must NOT be stuck in the loading state.
    // Before the fix, rapid reconnect could trigger the "replacing existing non-grace"
    // race or the relay data-socket open timeout, causing an infinite loading loop.
    await expect
      .poll(async () => agentPanelIsLoaded(page), {
        timeout: 30_000,
        message:
          "Agent panel is stuck in loading/not-found/error state after reload. " +
          "This may indicate the relay data-socket timeout or attach-socket race is not fixed.",
      })
      .toBe(true);

    // Content must be visible again after recovery.
    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test("workspace page recovers after page reload without stuck loading", async ({ page }) => {
    const serverId = getServerId();
    if (!agentId) {
      throw new Error("agentId was not initialized.");
    }

    await gotoAppShell(page);
    await page.goto(buildHostWorkspaceRoute(serverId, workspaceId));

    // Wait for workspace to be fully rendered.
    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByTestId("workspace-header-title").filter({ visible: true }).first(),
    ).toHaveText("main", { timeout: 30_000 });

    // Open the agent tab in the workspace.
    const agentTab = page.getByTestId(`workspace-tab-agent_${agentId}`).first();
    await expect(agentTab).toBeVisible({ timeout: 30_000 });
    await agentTab.click();

    // Verify agent content is visible inside the workspace.
    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Reload the page.
    await page.reload();

    // Workspace must recover without getting stuck.
    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });

    // Agent tab should still be present and load correctly.
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });

    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test("rapid reloads do not freeze the agent panel", async ({ page }) => {
    const serverId = getServerId();
    if (!agentId) {
      throw new Error("agentId was not initialized.");
    }

    await gotoAppShell(page);
    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));

    // Initial load.
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Simulate rapid reconnects (the race condition window was ~60ms).
    for (let i = 0; i < 3; i++) {
      await page.reload();
      // After each reload, ensure we don't get stuck.
      await expect
        .poll(async () => agentPanelIsLoaded(page), {
          timeout: 30_000,
          message: `Agent panel stuck after reload #${i + 1}`,
        })
        .toBe(true);
    }

    // Final verification.
    await expect(
      page.getByText("Mock response to: Hello from relay timeout regression test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });
  });
});

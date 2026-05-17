import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test } from "./fixtures";
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
    clientId: `web-direct-tcp-${randomUUID()}`,
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

interface DaemonRegistryEntry {
  serverId?: string;
  connections?: Array<{ type?: string; endpoint?: string; relayEndpoint?: string }>;
  preferredConnectionId?: string;
}

test.describe("Local web direct-TCP reconnect (no relay)", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let workspaceId: string;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient();
    repo = await createTempGitRepo("web-direct-tcp-", {
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
        title: "Direct TCP reconnect agent",
      },
      initialPrompt: "Hello from direct-tcp reconnect test",
      labels: { "solo.e2e": "web-direct-tcp" },
    });
    agentId = agent.id;
    await waitForTimelineText(client, agent.id, "Mock response to: Hello from direct-tcp reconnect test");
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("web client connects via directTcp without relay", async ({ page }) => {
    const serverId = getServerId();
    const daemonPort = process.env.E2E_DAEMON_PORT;
    if (!daemonPort) {
      throw new Error("E2E_DAEMON_PORT is not set.");
    }

    await gotoAppShell(page);

    // Verify the seeded daemon registry in the browser uses directTcp only.
    const registryRaw = await page.evaluate(() => {
      return localStorage.getItem("@solo:daemon-registry");
    });
    expect(registryRaw).not.toBeNull();

    const registry = JSON.parse(registryRaw!) as DaemonRegistryEntry[];
    expect(registry).toHaveLength(1);

    const entry = registry[0];
    expect(entry?.serverId).toBe(serverId);
    expect(entry?.connections).toBeDefined();
    expect(entry!.connections!).toHaveLength(1);

    const conn = entry!.connections![0];
    expect(conn.type).toBe("directTcp");
    expect(conn.endpoint).toBe(`127.0.0.1:${daemonPort}`);
    expect(conn.relayEndpoint).toBeUndefined();

    // No relay connection should be present.
    const hasRelay = entry!.connections!.some((c) => c.type === "relay");
    expect(hasRelay).toBe(false);
  });

  test("agent detail page recovers after reload on directTcp", async ({ page }) => {
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
      page.getByText("Mock response to: Hello from direct-tcp reconnect test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Reload simulates app restart / browser refresh on localhost:19000.
    await page.reload();

    // Must not get stuck in loading state.
    const loading = page.getByTestId("agent-loading").first();
    const notFound = page.getByTestId("agent-not-found").first();
    const error = page.getByTestId("agent-load-error").first();

    await expect(loading).not.toBeVisible({ timeout: 30_000 });
    await expect(notFound).not.toBeVisible({ timeout: 10_000 });
    await expect(error).not.toBeVisible({ timeout: 10_000 });

    // Content must reappear.
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByText("Mock response to: Hello from direct-tcp reconnect test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test("workspace page recovers after reload on directTcp", async ({ page }) => {
    const serverId = getServerId();
    if (!agentId) {
      throw new Error("agentId was not initialized.");
    }

    await gotoAppShell(page);
    await page.goto(buildHostWorkspaceRoute(serverId, workspaceId));

    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });

    const agentTab = page.getByTestId(`workspace-tab-agent_${agentId}`).first();
    await expect(agentTab).toBeVisible({ timeout: 30_000 });
    await agentTab.click();

    await expect(
      page.getByText("Mock response to: Hello from direct-tcp reconnect test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    await page.reload();

    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect(
      page.getByText("Mock response to: Hello from direct-tcp reconnect test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test("git-commit-helper prompt survives page reload on directTcp", async ({ page }) => {
    const serverId = getServerId();
    const prompt = "git add . and invoke skill git commit helper";

    // Ensure the web client is connected before creating the agent so it
    // receives the real-time agent creation and timeline events.
    await gotoAppShell(page);
    await page.goto(buildHostWorkspaceRoute(serverId, workspaceId));
    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });

    // Create the agent via CLI (reliable) with the target prompt.
    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Git commit helper regression",
      },
      initialPrompt: prompt,
      labels: { "solo.e2e": "git-commit-helper-regression" },
    });
    const webAgentId = agent.id;
    await waitForTimelineText(client, webAgentId, `Mock response to: ${prompt}`);

    // Navigate to the agent page via web.
    await page.goto(buildHostAgentDetailRoute(serverId, webAgentId, workspaceId));

    // Verify content is visible.
    const mockResponse = `Mock response to: ${prompt}`;
    await expect(page.getByTestId(`workspace-tab-agent_${webAgentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    // Use partial match because the web UI may concatenate multiple timeline
    // items into a single text node.
    await expect
      .poll(
        async () =>
          page.getByText(mockResponse, { exact: false }).first().isVisible().catch(() => false),
        { timeout: 30_000 },
      )
      .toBe(true);

    // Reload the page to simulate localhost:19000 refresh during a long skill invocation.
    await page.reload();

    // Must not get stuck in loading state.
    const loading = page.getByTestId("agent-loading").first();
    const notFound = page.getByTestId("agent-not-found").first();
    const error = page.getByTestId("agent-load-error").first();

    await expect(loading).not.toBeVisible({ timeout: 30_000 });
    await expect(notFound).not.toBeVisible({ timeout: 10_000 });
    await expect(error).not.toBeVisible({ timeout: 10_000 });

    // Content must reappear after recovery.
    await expect(page.getByTestId(`workspace-tab-agent_${webAgentId}`).first()).toBeVisible({
      timeout: 30_000,
    });
    await expect
      .poll(
        async () =>
          page.getByText(mockResponse, { exact: false }).first().isVisible().catch(() => false),
        { timeout: 30_000 },
      )
      .toBe(true);

    // Cleanup this web-created agent.
    await client.deleteAgent(webAgentId).catch(() => undefined);
  });
});

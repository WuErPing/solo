/**
 * E2E test: User message sent via UI composer does not duplicate when
 * the server echoes back the user_message timeline event.
 *
 * Regression guard for: optimistic user_message and server echo creating
 * duplicate visible messages.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test } from "./fixtures";
import { buildHostAgentDetailRoute, buildHostWorkspaceRoute } from "@/utils/host-routes";

import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import { getDaemonWsUrl, getServerId, waitForTimelineText, type CoreDaemonClient } from "./helpers/daemon-client";
import { createTempGitRepo } from "./helpers/workspace";
import { gotoAppShell } from "./helpers/app";

async function connectCoreDaemonClient(
  clientIdPrefix = "e2e-client",
): Promise<CoreDaemonClient> {
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
    clientId: `${clientIdPrefix}-${randomUUID()}`,
    clientType: "cli",
    webSocketFactory: createNodeWebSocketFactory(),
  });
  await client.connect();
  return client;
}

test.describe("Optimistic message deduplication", () => {
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let client: CoreDaemonClient;
  let agentId: string | null = null;
  let workspaceId: string | null = null;

  test.beforeAll(async () => {
    repo = await createTempGitRepo("optimistic-dedup-", {
      files: [{ path: "README.md", content: "# Optimistic dedup test\n" }],
    });
    client = await connectCoreDaemonClient("optimistic-dedup");

    const workspace = await client.openProject(repo.path);
    workspaceId = workspace.workspace?.id ?? repo.path;

    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Optimistic dedup",
      },
      initialPrompt: "Initial prompt for optimistic dedup test",
      clientMessageId: "e2e-optimistic-dedup-init",
      labels: { "solo.e2e": "optimistic-dedup" },
    });
    agentId = agent.id;

    await waitForTimelineText(client, agent.id, "Mock response to: Initial prompt for optimistic dedup test");
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("user message appears exactly once after server echo", async ({ page }) => {
    if (!agentId || !workspaceId) {
      throw new Error("agentId or workspaceId was not initialized in beforeAll.");
    }

    const serverId = getServerId();
    await gotoAppShell(page);
    await page.goto(buildHostWorkspaceRoute(serverId, workspaceId));

    await expect(page.getByTestId("workspace-tabs-row").filter({ visible: true }).first()).toBeVisible({
      timeout: 30_000,
    });

    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });

    // Wait for the initial assistant response to appear
    await expect(
      page.getByText("Mock response to: Initial prompt for optimistic dedup test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Now send a follow-up message via the UI composer
    const composer = page.getByRole("textbox", { name: "Message agent..." });
    await expect(composer).toBeVisible({ timeout: 30_000 });
    await expect(composer).toBeEditable();

    const followUpText = "Follow-up message to test dedup";
    await composer.fill(followUpText);
    await composer.press("Enter");

    // Wait for the mock response to the follow-up
    await expect(
      page.getByText(`Mock response to: ${followUpText}`, { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Count how many times the user message text appears in the stream.
    // The frontend dedup logic (messageId or text matching within 30s) should
    // ensure the optimistic message and server echo are merged into a single
    // visible DOM element.
    const userMessageInstances = page.getByText(followUpText, { exact: true });

    // There should be exactly one visible user message node.
    // Note: in some layout configurations there may be multiple matching nodes
    // (e.g. hidden copies for measurement), so we assert on the visible count.
    const visibleCount = await userMessageInstances.evaluateAll((elements) =>
      elements.filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      }).length,
    );

    expect(visibleCount).toBe(1);
  });

  test("initial prompt from createAgent appears exactly once", async ({ page }) => {
    if (!agentId || !workspaceId) {
      throw new Error("agentId or workspaceId was not initialized in beforeAll.");
    }

    const serverId = getServerId();
    await gotoAppShell(page);
    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });

    // Wait for the mock response to appear (turn has completed)
    await expect(
      page.getByText("Mock response to: Initial prompt for optimistic dedup test", { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    // Count visible instances of the initial prompt text
    const initialPromptText = "Initial prompt for optimistic dedup test";
    const instances = page.getByText(initialPromptText, { exact: true });
    const visibleCount = await instances.evaluateAll((elements) =>
      elements.filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      }).length,
    );

    expect(visibleCount).toBe(1);
  });

  test("initial response from createAgent appears exactly once (no dual-delivery duplication)", async ({ page }) => {
    if (!agentId || !workspaceId) {
      throw new Error("agentId or workspaceId was not initialized in beforeAll.");
    }

    const serverId = getServerId();
    await gotoAppShell(page);
    await page.goto(buildHostAgentDetailRoute(serverId, agentId, workspaceId));
    await expect(page.getByTestId(`workspace-tab-agent_${agentId}`).first()).toBeVisible({
      timeout: 30_000,
    });

    const responseText = "Mock response to: Initial prompt for optimistic dedup test";
    await expect(
      page.getByText(responseText, { exact: true }).first(),
    ).toBeVisible({ timeout: 30_000 });

    const instances = page.getByText(responseText, { exact: true });
    const visibleCount = await instances.evaluateAll((elements) =>
      elements.filter((el) => {
        const rect = el.getBoundingClientRect();
        return rect.width > 0 && rect.height > 0;
      }).length,
    );

    expect(visibleCount).toBe(1);
  });
});

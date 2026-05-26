/**
 * E2E test: Timeline entries preserve the order in which messages were sent.
 *
 * Regression guard for: out-of-order delivery or reducer mis-sequencing.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test } from "./fixtures";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import {
  fetchAllTimelineEntries,
  waitForTimelineText,
  getDaemonWsUrl,
  sendMessageWithRetry,
  type CoreDaemonClient,
} from "./helpers/daemon-client";
import { createTempGitRepo } from "./helpers/workspace";

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

test.describe("Message ordering", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient("ordering");
    repo = await createTempGitRepo("message-ordering-", {
      files: [{ path: "README.md", content: "# Ordering test\n" }],
    });
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("user and assistant messages appear in strict send order", async () => {
    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Message ordering",
      },
      initialPrompt: "Turn 0",
      labels: { "solo.e2e": "message-ordering" },
    });
    agentId = agent.id;

    await waitForTimelineText(client, agent.id, "Mock response to: Turn 0");

    // Send 4 additional sequential messages
    const turns = ["Turn 1", "Turn 2", "Turn 3", "Turn 4"];
    for (const text of turns) {
      await client.sendMessage(agent.id, text);
      // Yield to allow mock provider to complete the turn
      await new Promise((r) => setTimeout(r, 150));
    }

    // Poll until all assistant responses are present
    await expect
      .poll(
        async () => {
          const timeline = await client.fetchAgentTimeline(agent.id, {
            direction: "tail",
            limit: 50,
          });
          const assistantMessages = timeline.entries.filter(
            (e) => (e.item as { type?: string }).type === "assistant_message",
          );
          return assistantMessages.length;
        },
        { timeout: 60_000, intervals: [1000, 2000, 3000] },
      )
      .toBe(turns.length + 1);

    const timeline = await fetchAllTimelineEntries(client, agent.id);

    // Extract user_messages in timeline order
    const userMessages = timeline
      .filter((e) => (e.item as { type?: string }).type === "user_message")
      .map((e) => (e.item as { text?: string }).text ?? "");

    // Extract assistant_messages in timeline order
    const assistantMessages = timeline
      .filter((e) => (e.item as { type?: string }).type === "assistant_message")
      .map((e) => (e.item as { text?: string }).text ?? "");

    expect(userMessages.length).toBe(5);
    expect(assistantMessages.length).toBe(5);

    // Verify strict ordering: each user message precedes its assistant response
    const expectedUserOrder = ["Turn 0", "Turn 1", "Turn 2", "Turn 3", "Turn 4"];
    const expectedAssistantOrder = expectedUserOrder.map(
      (t) => `Mock response to: ${t}`,
    );

    expect(userMessages).toEqual(expectedUserOrder);
    expect(assistantMessages).toEqual(expectedAssistantOrder);
  });
});

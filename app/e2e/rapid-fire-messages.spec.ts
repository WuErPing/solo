/**
 * E2E test: Rapidly sending many messages does not lose any timeline entries.
 *
 * Regression guard for: inbound queue overflow causing silent message drops.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test } from "./fixtures";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import {
  fetchAllTimelineEntries,
  getDaemonWsUrl,
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

test.describe("Rapid fire messages", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient("rapid-fire");
    repo = await createTempGitRepo("rapid-fire-", {
      files: [{ path: "README.md", content: "# Rapid fire test\n" }],
    });
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("20 rapid messages are all recorded without loss", async () => {
    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Rapid fire",
      },
      initialPrompt: "Message 0",
      labels: { "solo.e2e": "rapid-fire" },
    });
    agentId = agent.id;

    // Wait for initial prompt response
    await expect
      .poll(
        async () => {
          const timeline = await client.fetchAgentTimeline(agent.id, { direction: "tail", limit: 50 });
          return timeline.entries.some((entry) =>
            JSON.stringify(entry.item).includes("Mock response to: Message 0"),
          );
        },
        { timeout: 30_000 },
      )
      .toBe(true);

    // Rapidly send 19 additional messages in sequence (daemon rejects concurrent turns)
    const messageCount = 19;
    for (let i = 1; i <= messageCount; i++) {
      await client.sendMessage(agent.id, `Message ${i}`);
      // Small yield to allow the mock provider to complete and lifecycle to update
      await new Promise((r) => setTimeout(r, 100));
    }

    // Poll until all responses are present
    await expect
      .poll(
        async () => {
          const timeline = await client.fetchAgentTimeline(agent.id, { direction: "tail", limit: 500 });
          const assistantMessages = timeline.entries.filter(
            (e) => (e.item as { type?: string }).type === "assistant_message",
          );
          return assistantMessages.length;
        },
        { timeout: 60_000, intervals: [1000, 2000, 3000] },
      )
      .toBe(messageCount + 1);

    // Final verification
    const timeline = await fetchAllTimelineEntries(client, agent.id);
    const userMessages = timeline.filter(
      (e) => (e.item as { type?: string }).type === "user_message",
    );
    const assistantMessages = timeline.filter(
      (e) => (e.item as { type?: string }).type === "assistant_message",
    );

    expect(userMessages.length).toBe(messageCount + 1);
    expect(assistantMessages.length).toBe(messageCount + 1);

    // Verify each expected message text is present
    for (let i = 0; i <= messageCount; i++) {
      const hasUserMessage = userMessages.some((e) =>
        JSON.stringify(e.item).includes(`Message ${i}`),
      );
      const hasAssistantMessage = assistantMessages.some((e) =>
        JSON.stringify(e.item).includes(`Mock response to: Message ${i}`),
      );
      expect(hasUserMessage).toBe(true);
      expect(hasAssistantMessage).toBe(true);
    }
  });
});

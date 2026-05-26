/**
 * E2E test: Timeline remains complete after disconnect and reconnect.
 *
 * Regression guard for: messages getting stuck or lost after network interruption.
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

test.describe("Reconnect resilience", () => {
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;

  test.beforeAll(async () => {
    repo = await createTempGitRepo("reconnect-resilience-", {
      files: [{ path: "README.md", content: "# Reconnect test\n" }],
    });
  });

  test.afterAll(async () => {
    await repo?.cleanup();
  });

  test("timeline is intact after disconnect and reconnect", async () => {
    // First connection: create agent and send a message
    const client1 = await connectCoreDaemonClient("reconnect-1");
    let agentId: string | null = null;

    try {
      const agent = await client1.createAgent({
        config: {
          provider: "mock",
          cwd: repo.path,
          title: "Reconnect resilience",
        },
        initialPrompt: "First turn before disconnect",
        labels: { "solo.e2e": "reconnect-resilience" },
      });
      agentId = agent.id;

      await waitForTimelineText(client1, agent.id, "Mock response to: First turn before disconnect");

      // Fetch timeline before disconnect
      const timelineBefore = await fetchAllTimelineEntries(client1, agent.id);
      expect(timelineBefore.length).toBeGreaterThan(0);

      // Disconnect
      await client1.close();

      // Reconnect with a new client instance
      const client2 = await connectCoreDaemonClient("reconnect-2");
      try {
        // Timeline should still be fetchable after reconnect
        const timelineAfter = await fetchAllTimelineEntries(client2, agent.id);
        expect(timelineAfter.length).toBe(timelineBefore.length);

        // Send a second message after reconnect
        await client2.sendMessage(agent.id, "Second turn after reconnect");
        await waitForTimelineText(
          client2,
          agent.id,
          "Mock response to: Second turn after reconnect",
        );

        const timelineFinal = await fetchAllTimelineEntries(client2, agent.id);
        const userMessages = timelineFinal.filter(
          (e) => (e.item as { type?: string }).type === "user_message",
        );
        const assistantMessages = timelineFinal.filter(
          (e) => (e.item as { type?: string }).type === "assistant_message",
        );

        expect(userMessages.length).toBe(2);
        expect(assistantMessages.length).toBe(2);
      } finally {
        await client2.deleteAgent(agent.id).catch(() => undefined);
        await client2.close().catch(() => undefined);
      }
    } finally {
      if (agentId) {
        await client1.deleteAgent(agentId).catch(() => undefined);
      }
    }
  });
});

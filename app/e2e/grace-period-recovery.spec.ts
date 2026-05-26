/**
 * E2E test: Messages sent while a client is disconnected are replayed
 * after reconnect (grace-period buffering).
 *
 * Regression guard for: grace period playback failure causing stuck messages.
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

test.describe("Grace period recovery", () => {
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;

  test.beforeAll(async () => {
    repo = await createTempGitRepo("grace-period-", {
      files: [{ path: "README.md", content: "# Grace period test\n" }],
    });
  });

  test.afterAll(async () => {
    await repo?.cleanup();
  });

  test("message sent while disconnected is visible after reconnect", async () => {
    // Client A: create agent and send first message
    const clientA = await connectCoreDaemonClient("grace-a");
    let agentId: string | null = null;

    try {
      const agent = await clientA.createAgent({
        config: {
          provider: "mock",
          cwd: repo.path,
          title: "Grace period recovery",
        },
        initialPrompt: "First turn",
        labels: { "solo.e2e": "grace-period" },
      });
      agentId = agent.id;

      await waitForTimelineText(clientA, agent.id, "Mock response to: First turn");

      // Client A disconnects
      await clientA.close();

      // Client B connects and sends a message while A is offline
      const clientB = await connectCoreDaemonClient("grace-b");
      try {
        await clientB.sendMessage(agent.id, "Message while A is offline");
        await waitForTimelineText(
          clientB,
          agent.id,
          "Mock response to: Message while A is offline",
        );
      } finally {
        await clientB.close();
      }

      // Client A reconnects
      const clientA2 = await connectCoreDaemonClient("grace-a2");
      try {
        const timeline = await fetchAllTimelineEntries(clientA2, agent.id);

        const userMessages = timeline.filter(
          (e) => (e.item as { type?: string }).type === "user_message",
        );
        const assistantMessages = timeline.filter(
          (e) => (e.item as { type?: string }).type === "assistant_message",
        );

        // Should see both turns
        expect(userMessages.length).toBe(2);
        expect(assistantMessages.length).toBe(2);

        // Verify the second message is present
        const hasOfflineMessage = userMessages.some((e) =>
          JSON.stringify(e.item).includes("Message while A is offline"),
        );
        expect(hasOfflineMessage).toBe(true);
      } finally {
        await clientA2.deleteAgent(agent.id).catch(() => undefined);
        await clientA2.close().catch(() => undefined);
      }
    } finally {
      if (agentId) {
        await clientA.deleteAgent(agentId).catch(() => undefined);
      }
    }
  });
});

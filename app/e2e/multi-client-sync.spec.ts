/**
 * E2E test: Multiple clients connected to the same daemon should see
 * identical, non-duplicate timeline content.
 *
 * Regression guard for: message duplication when web + app are both open.
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

test.describe("Multi-client timeline synchronization", () => {
  let clientA: CoreDaemonClient;
  let clientB: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    clientA = await connectCoreDaemonClient("multi-client-a");
    clientB = await connectCoreDaemonClient("multi-client-b");
    repo = await createTempGitRepo("multi-client-sync-", {
      files: [{ path: "README.md", content: "# Multi-client test\n" }],
    });
  });

  test.afterAll(async () => {
    if (agentId) {
      await clientA?.deleteAgent(agentId).catch(() => undefined);
    }
    await clientA?.close().catch(() => undefined);
    await clientB?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("two clients see the same timeline after sending a message", async () => {
    const agent = await clientA.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Multi-client sync",
      },
      initialPrompt: "Hello from client A",
      labels: { "solo.e2e": "multi-client-sync" },
    });
    agentId = agent.id;

    await waitForTimelineText(clientA, agent.id, "Mock response to: Hello from client A");

    // Give a small buffer for any async propagation
    await new Promise((r) => setTimeout(r, 500));

    const timelineA = await fetchAllTimelineEntries(clientA, agent.id);
    const timelineB = await fetchAllTimelineEntries(clientB, agent.id);

    // Both clients should see the same number of timeline entries
    expect(timelineA.length).toBe(timelineB.length);

    // Verify both contain user_message and assistant_message
    const userMessagesA = timelineA.filter((e) => (e.item as { type?: string }).type === "user_message");
    const userMessagesB = timelineB.filter((e) => (e.item as { type?: string }).type === "user_message");
    const assistantMessagesA = timelineA.filter(
      (e) => (e.item as { type?: string }).type === "assistant_message",
    );
    const assistantMessagesB = timelineB.filter(
      (e) => (e.item as { type?: string }).type === "assistant_message",
    );

    expect(userMessagesA.length).toBe(1);
    expect(userMessagesB.length).toBe(1);
    expect(assistantMessagesA.length).toBe(1);
    expect(assistantMessagesB.length).toBe(1);

    expect((userMessagesA[0]?.item as { text?: string }).text).toBe("Hello from client A");
    expect((userMessagesB[0]?.item as { text?: string }).text).toBe("Hello from client A");

    // Verify no duplicate entries based on type+text dedup key
    const dedupKey = (entry: { item: unknown }) => {
      const item = entry.item as { type?: string; text?: string; callId?: string; status?: string };
      return `${item.type ?? "unknown"}|${item.text ?? ""}|${item.callId ?? ""}|${item.status ?? ""}`;
    };

    const keysA = new Set(timelineA.map(dedupKey));
    const keysB = new Set(timelineB.map(dedupKey));

    expect(keysA.size).toBe(timelineA.length);
    expect(keysB.size).toBe(timelineB.length);
  });

  test("second message from client A is visible to client B", async () => {
    if (!agentId) {
      test.skip();
      return;
    }

    // Wait a moment to ensure the first turn fully completed and agent is idle
    await new Promise((r) => setTimeout(r, 1500));

    await clientA.sendMessage(agentId, "Second message from client A");

    // Poll until the second assistant_message appears
    await expect
      .poll(
        async () => {
          const timeline = await clientA.fetchAgentTimeline(agentId, { direction: "tail", limit: 50 });
          const assistantMessages = timeline.entries.filter(
            (e) => (e.item as { type?: string }).type === "assistant_message",
          );
          return assistantMessages.length >= 2;
        },
        { timeout: 30_000, intervals: [500, 1000, 2000] },
      )
      .toBe(true);

    // Give buffer for propagation
    await new Promise((r) => setTimeout(r, 500));

    const timelineA = await fetchAllTimelineEntries(clientA, agentId);
    const timelineB = await fetchAllTimelineEntries(clientB, agentId);

    // Both should now have 2 user_messages and 2 assistant_messages
    const userMessagesA = timelineA.filter((e) => (e.item as { type?: string }).type === "user_message");
    const userMessagesB = timelineB.filter((e) => (e.item as { type?: string }).type === "user_message");

    expect(userMessagesA.length).toBe(2);
    expect(userMessagesB.length).toBe(2);
    expect(timelineA.length).toBe(timelineB.length);
  });
});

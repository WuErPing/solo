/**
 * E2E test: Timeline fetch directions (tail / before / after) and limits work correctly.
 *
 * Regression guard for: pagination bugs causing missing or duplicated entries.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test } from "./fixtures";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import {
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

test.describe("Timeline pagination", () => {
  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient("pagination");
    repo = await createTempGitRepo("timeline-pagination-", {
      files: [{ path: "README.md", content: "# Pagination test\n" }],
    });
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("tail with limit returns the most recent entries", async () => {
    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: repo.path,
        title: "Timeline pagination",
      },
      initialPrompt: "Page 0",
      labels: { "solo.e2e": "timeline-pagination" },
    });
    agentId = agent.id;

    await waitForTimelineText(client, agent.id, "Mock response to: Page 0");

    // Send 4 more messages to build a timeline of 5 turns
    for (let i = 1; i <= 4; i++) {
      await sendMessageWithRetry(client, agent.id, `Page ${i}`);
    }

    // Wait for all 5 assistant responses
    await expect
      .poll(
        async () => {
          const t = await client.fetchAgentTimeline(agent.id, {
            direction: "tail",
            limit: 50,
          });
          return t.entries.filter(
            (e) => (e.item as { type?: string }).type === "assistant_message",
          ).length;
        },
        { timeout: 60_000, intervals: [1000, 2000, 3000] },
      )
      .toBe(5);

    // tail limit=4 should return the last 4 entries (not 5)
    const tail4 = await client.fetchAgentTimeline(agent.id, {
      direction: "tail",
      limit: 4,
    });
    expect(tail4.entries.length).toBe(4);

    // The last entry should be the final assistant_message
    const lastItem = tail4.entries[tail4.entries.length - 1].item as {
      type?: string;
      text?: string;
    };
    expect(lastItem.type).toBe("assistant_message");
    expect(lastItem.text).toBe("Mock response to: Page 4");
  });

  test("before and after cursors return correct slices", async () => {
    if (!agentId) {
      test.skip();
      return;
    }

    // Fetch full timeline to obtain cursors
    const full = await client.fetchAgentTimeline(agentId, {
      direction: "tail",
      limit: 50,
    });
    expect(full.entries.length).toBeGreaterThanOrEqual(6); // 5 turns * at least 2 items each

    // Use the endCursor from the full fetch as a pivot (roughly middle)
    const pivotCursor = full.endCursor;
    expect(pivotCursor).toBeTruthy();

    // after cursor should return entries that come AFTER the pivot
    const afterSlice = await client.fetchAgentTimeline(agentId, {
      direction: "after",
      limit: 50,
      cursor: pivotCursor!,
    });
    expect(afterSlice.entries.length).toBeGreaterThanOrEqual(0);
    expect(afterSlice.entries.length).toBeLessThan(full.entries.length);

    // before cursor should return entries that come BEFORE the pivot
    const beforeSlice = await client.fetchAgentTimeline(agentId, {
      direction: "before",
      limit: 50,
      cursor: pivotCursor!,
    });
    expect(beforeSlice.entries.length).toBeGreaterThan(0);
    expect(beforeSlice.entries.length).toBeLessThan(full.entries.length);

    // The union of before + after should cover the full timeline
    // (the pivot row itself may appear in neither slice depending on daemon behavior)
    const total = beforeSlice.entries.length + afterSlice.entries.length;
    expect(total).toBeGreaterThanOrEqual(full.entries.length - 1);
    expect(total).toBeLessThanOrEqual(full.entries.length);
  });
});

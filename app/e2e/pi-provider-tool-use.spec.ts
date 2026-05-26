/**
 * E2E regression test: Pi provider must return the final text response
 * when the query triggers a tool call (e.g. "date").
 *
 * Bug: When Pi runs a tool, it emits multiple turns:
 *   Turn 1: turn_end { stopReason: "toolUse" }  ← intermediate, NOT final
 *   Turn 2: turn_end { stopReason: "stop" }      ← the real assistant response
 *
 * The translator was previously treating any turn_end as terminal, so it
 * stopped after Turn 1 and never emitted the assistant text.
 *
 * This test verifies that the pipeline waits for the final turn and the
 * assistant_message timeline item is present in the agent timeline.
 */

import { randomUUID } from "node:crypto";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { expect, test, type Page } from "./fixtures";
import { buildHostAgentDetailRoute } from "@/utils/host-routes";
import { gotoAppShell } from "./helpers/app";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import {
  waitForTimelineItemType,
  getServerId,
  type CoreDaemonClient,
} from "./helpers/daemon-client";
import { createTempGitRepo } from "./helpers/workspace";

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
    clientId: `pi-tool-use-${randomUUID()}`,
    clientType: "cli",
    webSocketFactory: createNodeWebSocketFactory(),
  });
  await client.connect();
  return client;
}

/**
 * Skip the test when the Pi binary is not available in the environment.
 * This allows the test suite to run in CI without Pi installed.
 */
function isPiAvailable(): boolean {
  try {
    const { execSync } = require("node:child_process") as typeof import("node:child_process");
    execSync("which pi || ls ~/.bun/bin/pi ~/.local/bin/pi /usr/local/bin/pi 2>/dev/null | head -1", {
      stdio: "pipe",
    });
    return true;
  } catch {
    return false;
  }
}

test.describe("Pi provider: tool-use query returns final text response", () => {
  test.skip(!isPiAvailable(), "Pi binary not available — skipping Pi E2E tests");

  let client: CoreDaemonClient;
  let repo: Awaited<ReturnType<typeof createTempGitRepo>>;
  let agentId: string | null = null;

  test.beforeAll(async () => {
    client = await connectCoreDaemonClient();
    repo = await createTempGitRepo("pi-tool-use-e2e-", {
      files: [{ path: "README.md", content: "# Test repo\n" }],
    });
  });

  test.afterAll(async () => {
    if (agentId) {
      await client?.deleteAgent(agentId).catch(() => undefined);
    }
    await client?.close().catch(() => undefined);
    await repo?.cleanup();
  });

  test("sending 'date' returns an assistant_message, not just thinking", async ({ page }) => {
    const serverId = getServerId();

    // Create a Pi agent and send "date" as the initial prompt.
    // This triggers a tool call (bash: date), which means Pi emits 2 turns.
    // The fix ensures we wait for the second turn's actual text response.
    const agent = await client.createAgent({
      config: {
        provider: "pi",
        cwd: repo.path,
        title: "Pi tool-use regression",
      },
      initialPrompt: "date",
      labels: { "solo.e2e": "pi-tool-use" },
    });
    agentId = agent.id;

    // The timeline must contain an assistant_message item (not just reasoning).
    // If the bug is present, only "reasoning" items appear and this times out.
    const assistantText = await waitForTimelineItemType(client, agent.id, "assistant_message", 60_000);

    expect(assistantText.length).toBeGreaterThan(0);

    // Verify the text does NOT only contain internal thinking — it must be a
    // real response visible to the user.
    expect(assistantText).not.toMatch(/^The user/i);

    // Navigate to the agent in the web UI and verify the response is visible.
    await gotoAppShell(page);

    const agentUrl = buildHostAgentDetailRoute(serverId, agent.id, repo.path);
    await page.goto(agentUrl);

    await expect(page.getByTestId(`workspace-tab-agent_${agent.id}`).first()).toBeVisible({
      timeout: 30_000,
    });

    // The timeline in the web UI must show the assistant message, not just thinking.
    // We look for any text node containing "assistant" role content being rendered.
    // The actual date text (e.g. "Sun May 24") should appear in the timeline.
    await expect(
      page.locator('[data-timeline-item-type="assistant_message"]').first(),
    ).toBeVisible({ timeout: 30_000 });
  });

  test("multi-turn: second message after tool-use also returns assistant text", async () => {
    if (!agentId) {
      test.skip();
      return;
    }

    // Send a second message. Pi may use tools again. We verify each response
    // includes an assistant_message in the timeline.
    await client.sendMessage(agentId, "what day of the week is it?");

    // Wait for a new assistant_message after the second sendMessage.
    // Poll until we have at least 2 assistant_message items (one from "date", one from this).
    await expect
      .poll(
        async () => {
          const timeline = await client.fetchAgentTimeline(agentId!, {
            direction: "tail",
            limit: 200,
          });
          const assistantMessages = timeline.entries.filter((e) => {
            const item = e.item as { type?: string };
            return item?.type === "assistant_message";
          });
          return assistantMessages.length >= 2;
        },
        { timeout: 60_000, intervals: [2000, 3000, 5000] },
      )
      .toBe(true);
  });
});

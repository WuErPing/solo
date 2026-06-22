/**
 * E2E test: create a schedule through the UI and verify it appears in the list.
 *
 * Seeds an agent directly via the daemon so the schedule-create modal has a
 * selectable target. This avoids relying on tmux-detected agents, which are not
 * guaranteed to be present in the daemon's agent directory.
 */
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { randomUUID } from "node:crypto";
import { test, expect } from "./fixtures";
import { createNodeWebSocketFactory, type NodeWebSocketFactory } from "./helpers/node-ws-factory";
import { buildSeededHost, buildCreateAgentPreferences } from "./helpers/daemon-registry";

interface SeededAgent {
  id: string;
  cwd: string;
}

interface DaemonClientLike {
  connect(): Promise<void>;
  close(): Promise<void>;
  createAgent(input: {
    config: { provider: string; cwd: string; title?: string };
    initialPrompt?: string;
  }): Promise<{ id: string; cwd: string; title?: string | null }>;
  deleteAgent(agentId: string): Promise<void>;
}

async function loadDaemonClientConstructor(): Promise<
  new (config: {
    url: string;
    clientId: string;
    clientType: "cli";
    webSocketFactory?: NodeWebSocketFactory;
  }) => DaemonClientLike
> {
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
    }) => DaemonClientLike;
  };
  return mod.DaemonClient;
}

async function getDaemonWsUrl(): Promise<string> {
  const daemonPort = process.env.E2E_DAEMON_PORT;
  if (!daemonPort) {
    throw new Error("E2E_DAEMON_PORT is not set.");
  }
  return `ws://127.0.0.1:${daemonPort}/ws`;
}

test.describe("schedule creation", () => {
  test.describe.configure({ timeout: 120_000 });

  let seededAgent: SeededAgent | null = null;
  let seededCwd: string | null = null;
  let daemonClient: DaemonClientLike | null = null;

  test.beforeAll(async () => {
    const serverId = process.env.E2E_SERVER_ID;
    if (!serverId) {
      throw new Error("E2E_SERVER_ID is not set");
    }

    seededCwd = await mkdtemp(path.join(tmpdir(), "solo-schedule-e2e-"));
    const DaemonClient = await loadDaemonClientConstructor();
    const client = new DaemonClient({
      url: await getDaemonWsUrl(),
      clientId: `schedule-e2e-${randomUUID()}`,
      clientType: "cli",
      webSocketFactory: createNodeWebSocketFactory(),
    });
    await client.connect();

    const agent = await client.createAgent({
      config: {
        provider: "mock",
        cwd: seededCwd,
        title: "Schedule E2E Agent",
      },
    });

    seededAgent = { id: agent.id, cwd: agent.cwd };
    daemonClient = client;
  });

  test.afterAll(async () => {
    if (daemonClient) {
      if (seededAgent) {
        await daemonClient.deleteAgent(seededAgent.id).catch(() => undefined);
      }
      await daemonClient.close().catch(() => undefined);
    }
    if (seededCwd) {
      await rm(seededCwd, { recursive: true, force: true });
    }
  });

  test("creates a schedule from the schedules page and shows it in the list", async ({ page }) => {
    test.setTimeout(120_000);

    const serverId = process.env.E2E_SERVER_ID;
    const daemonPort = process.env.E2E_DAEMON_PORT;
    if (!serverId) {
      throw new Error("E2E_SERVER_ID is not set");
    }
    if (!daemonPort) {
      throw new Error("E2E_DAEMON_PORT is not set");
    }
    if (!seededAgent) {
      throw new Error("Seeded agent was not created");
    }

    // Swallow Metro HMR updates so they cannot starve the daemon keepalive timers.
    const metroPort = process.env.E2E_METRO_PORT;
    if (metroPort) {
      await page.routeWebSocket(`ws://localhost:${metroPort}/hot`, (route) => {
        route.onMessage(() => {
          /* drop HMR frames */
        });
      });
    }

    // Seed local storage before the app loads.
    const endpoint = `127.0.0.1:${daemonPort}`;
    const daemon = buildSeededHost({ serverId, endpoint, nowIso: new Date().toISOString() });
    const preferences = buildCreateAgentPreferences(serverId);
    await page.context().addInitScript(
      (([seededDaemon, seededPreferences]: [unknown, unknown]) => {
        localStorage.setItem("@solo:e2e", "1");
        localStorage.setItem("@solo:daemon-registry", JSON.stringify([seededDaemon]));
        localStorage.setItem("@solo:create-agent-preferences", JSON.stringify(seededPreferences));
        localStorage.removeItem("@solo:settings");
      }) as () => void,
      [daemon, preferences],
    );

    // Open the host schedules page with the create modal visible.
    await page.goto(`/h/${serverId}/schedules?create=true`);
    await expect(page.getByText("New Schedule").first()).toBeVisible({ timeout: 15_000 });

    const scheduleName = `e2e-schedule-${Date.now()}`;

    await page.getByTestId("schedule-create-name-input").first().fill(scheduleName);
    await page.getByTestId("schedule-create-prompt-input").first().fill("echo hello from schedule");

    // Switch to interval cadence for a simple, fast-recurring schedule.
    await page.getByText("Interval", { exact: true }).first().click();
    await page.getByTestId("schedule-create-interval-input").first().fill("60000");

    // Select the seeded agent.
    const agentCard = page.getByTestId("schedule-create-agent-card").first();
    await expect(agentCard).toBeVisible({ timeout: 30_000 });
    await agentCard.click();

    // Submit the form.
    await page.getByTestId("schedule-create-submit-button").first().click();

    // Modal should close and the new schedule should appear in the list.
    await expect(page.getByText("New Schedule").first()).not.toBeVisible({ timeout: 20_000 });
    await expect(page.getByText(scheduleName).first()).toBeVisible({ timeout: 15_000 });
  });
});

/**
 * E2E tests for the chat-based schedule assistant
 * (docs/product/chat-schedule-assistant-design.md §10.4).
 *
 * The daemon under test is pointed at a stub OpenAI-compatible endpoint
 * (helpers/stub-llm-server) via an out-of-band `llmProviders` config patch,
 * so proposal/clarify payloads are fully deterministic while the whole
 * app → daemon → LLM → app chain runs for real.
 *
 * Ordering: the no-provider test MUST run first — the daemon's empty
 * llmProviders state only exists before any provider config is patched
 * (clearing the list afterwards does not reliably restore the empty state),
 * so this file uses describe.serial.
 */
import { randomUUID } from "node:crypto";
import type { Locator, Page } from "@playwright/test";
import { test, expect } from "./fixtures";
import {
  connectWorkspaceSetupClient,
  type WorkspaceSetupDaemonClient,
} from "./helpers/workspace-setup";
import { StubLlmServer } from "./helpers/stub-llm-server";
import { cronToUTC, describeCron } from "../src/utils/cron-timezone";

function getRequiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is not set`);
  }
  return value;
}

async function swallowMetroHmr(page: Page): Promise<void> {
  const metroPort = process.env.E2E_METRO_PORT;
  if (!metroPort) {
    return;
  }
  await page.routeWebSocket(`ws://localhost:${metroPort}/hot`, (route) => {
    route.onMessage(() => {
      /* drop HMR frames */
    });
  });
}

/** Wrap the intent JSON in a ```json fence, the LLM output shape the daemon prefers. */
function fencedJson(intent: Record<string, unknown>): string {
  return `\`\`\`json\n${JSON.stringify(intent, null, 2)}\n\`\`\``;
}

async function configureStubLlmProvider(
  client: WorkspaceSetupDaemonClient,
  stub: StubLlmServer,
): Promise<void> {
  await client.patchDaemonConfig({
    llmProviders: [
      {
        id: "stub-llm",
        label: "Stub LLM",
        enabled: true,
        baseURL: stub.baseURL,
        apiKey: "test-key",
        models: [{ id: "stub-model", isDefault: true }],
      },
    ],
  });
}

async function createScheduleViaApi(
  client: WorkspaceSetupDaemonClient,
  input: { name: string; prompt: string; everyMs: number },
): Promise<string> {
  const result = await client.scheduleCreate({
    requestId: `req-${randomUUID()}`,
    name: input.name,
    prompt: input.prompt,
    cadence: { type: "every", everyMs: input.everyMs },
    target: { type: "provider", providerId: "mock" },
  });
  if (!result.schedule) {
    throw new Error(result.error ?? `scheduleCreate returned no schedule for ${input.name}`);
  }
  return result.schedule.id;
}

async function deleteScheduleQuietly(
  client: WorkspaceSetupDaemonClient,
  id: string | null,
): Promise<void> {
  if (!id) {
    return;
  }
  await client.scheduleDelete({ id }).catch(() => undefined);
}

async function openSchedulesScreen(page: Page, serverId: string): Promise<void> {
  await page.goto(`/h/${serverId}/schedules`);
  await expect(page.getByText("Schedules").first()).toBeVisible({ timeout: 15_000 });
}

async function openAssistantPanel(page: Page): Promise<Locator> {
  await page.getByTestId("schedule-assistant-button").click();
  const panel = page.getByTestId("schedule-assistant-panel");
  await expect(panel).toBeVisible({ timeout: 15_000 });
  return panel;
}

async function closeAssistantPanel(page: Page): Promise<void> {
  await page.getByTestId("schedule-assistant-panel").getByLabel("Close").click();
  await expect(page.getByTestId("schedule-assistant-panel")).toHaveCount(0, { timeout: 10_000 });
}

async function sendAssistantMessage(panel: Locator, text: string): Promise<void> {
  await panel.getByTestId("assistant-composer-input").fill(text);
  await panel.getByTestId("assistant-composer-send").click();
}

test.describe.serial("schedule assistant", () => {
  test.describe.configure({ timeout: 180_000 });

  test("shows the setup card when no LLM provider is configured", async ({ page }) => {
    test.setTimeout(180_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    await swallowMetroHmr(page);

    await openSchedulesScreen(page, serverId);
    const panel = await openAssistantPanel(page);

    // The empty state is replaced by the setup card deep-linking to settings.
    await expect(panel.getByText("No LLM provider configured")).toBeVisible({ timeout: 20_000 });
    const openSettings = panel.getByTestId("assistant-setup-settings");
    await expect(openSettings).toBeVisible();
    await expect(openSettings).toHaveText(/Open Settings/);
    await expect(panel.getByTestId("assistant-suggestion-chip")).toHaveCount(0);

    await closeAssistantPanel(page);
  });

  test("creates a schedule from a confirmed create proposal", async ({ page }) => {
    test.setTimeout(180_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const scheduleName = `e2e-assist-create-${Date.now()}`;
    const sentinelName = `e2e-assist-sentinel-${Date.now()}`;
    const stub = new StubLlmServer();
    await stub.start();
    const client = await connectWorkspaceSetupClient();
    let createdId: string | null = null;
    let sentinelId: string | null = null;

    try {
      stub.setContent(
        fencedJson({
          kind: "proposal",
          op: "create",
          name: scheduleName,
          prompt: "Summarize the nightly test runs",
          cadence: { type: "cron", expression: "0 9 * * 1-5" },
          target: { type: "provider", providerId: "mock" },
          summary: `Create '${scheduleName}' on weekdays at 09:00`,
        }),
      );
      await configureStubLlmProvider(client, stub);
      // Sentinel schedule: its appearance in the list proves the app's daemon
      // connection is up before we send (works whether or not other schedules exist).
      sentinelId = await createScheduleViaApi(client, {
        name: sentinelName,
        prompt: "echo sentinel",
        everyMs: 60_000,
      });

      await swallowMetroHmr(page);
      await openSchedulesScreen(page, serverId);
      await expect(page.getByText(sentinelName).first()).toBeVisible({ timeout: 30_000 });
      const panel = await openAssistantPanel(page);

      await sendAssistantMessage(panel, "Every weekday at 9am summarize the nightly test runs");

      const card = panel.getByTestId("proposal-card");
      await expect(card).toBeVisible({ timeout: 30_000 });
      await expect(card.getByText(scheduleName, { exact: true })).toBeVisible();
      await expect(card.getByText("create", { exact: true })).toBeVisible();
      // The card renders the cadence via describeCron(), so assert exactly that.
      await expect(
        card.getByText(describeCron("0 9 * * 1-5"), { exact: true }),
      ).toBeVisible();
      await expect(card.getByText("Summarize the nightly test runs")).toBeVisible();
      await expect(card.getByText("provider · mock", { exact: true })).toBeVisible();

      await card.getByTestId("proposal-confirm-button").click();

      await expect(panel.getByText(`Created ✓ "${scheduleName}"`)).toBeVisible({
        timeout: 20_000,
      });
      await expect(panel.getByTestId("assistant-receipt-link")).toBeVisible();

      await closeAssistantPanel(page);
      await expect(page.getByText(scheduleName).first()).toBeVisible({ timeout: 15_000 });

      // Out-of-band verification: the app converted the local cron to UTC on confirm.
      const list = await client.scheduleList();
      const stored = list.schedules.find((schedule) => schedule.name === scheduleName);
      expect(stored, `schedule '${scheduleName}' should be stored`).toBeTruthy();
      createdId = stored?.id ?? null;
      expect(stored?.cadence.type).toBe("cron");
      if (stored?.cadence.type === "cron") {
        expect(stored.cadence.timezone).toBeTruthy();
        expect(stored.cadence.expression).toBe(
          cronToUTC("0 9 * * 1-5", stored.cadence.timezone ?? "UTC"),
        );
      }
    } finally {
      await deleteScheduleQuietly(client, createdId);
      await deleteScheduleQuietly(client, sentinelId);
      await client.close();
      await stub.close();
    }
  });

  test("updates a schedule cadence from a confirmed update proposal", async ({ page }) => {
    test.setTimeout(180_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const stub = new StubLlmServer();
    await stub.start();
    const client = await connectWorkspaceSetupClient();
    let scheduleId: string | null = null;

    try {
      await configureStubLlmProvider(client, stub);
      scheduleId = await createScheduleViaApi(client, {
        name: "Disk cleanup",
        prompt: "Remove old temp files",
        everyMs: 3_600_000,
      });

      stub.setContent(
        fencedJson({
          kind: "proposal",
          op: "update",
          scheduleId,
          name: "Disk cleanup",
          cadence: { type: "every", everyMs: 7_200_000 },
          summary: "Run 'Disk cleanup' every 2 hours instead of every 1 hour",
        }),
      );

      await swallowMetroHmr(page);
      await openSchedulesScreen(page, serverId);
      // The pre-created schedule in the list proves the daemon connection is up.
      await expect(page.getByText("Disk cleanup").first()).toBeVisible({ timeout: 30_000 });
      const panel = await openAssistantPanel(page);

      await sendAssistantMessage(panel, "Run the disk cleanup every 2 hours");

      const card = panel.getByTestId("proposal-card");
      await expect(card).toBeVisible({ timeout: 30_000 });
      await expect(card.getByText("Disk cleanup", { exact: true })).toBeVisible();
      await expect(card.getByText("update", { exact: true })).toBeVisible();
      // Per-field diff (old → new), rendered once the inspect query resolves.
      await expect(card.getByText("every 60 min → every 120 min")).toBeVisible({
        timeout: 15_000,
      });

      await card.getByTestId("proposal-confirm-button").click();

      await expect(panel.getByText('Updated ✓ "Disk cleanup"')).toBeVisible({ timeout: 20_000 });

      const inspected = await client.scheduleInspect({ id: scheduleId });
      expect(inspected.schedule?.cadence).toEqual({ type: "every", everyMs: 7_200_000 });
    } finally {
      await deleteScheduleQuietly(client, scheduleId);
      await client.close();
      await stub.close();
    }
  });

  test("answers with a clarify bubble when the schedule reference is ambiguous", async ({
    page,
  }) => {
    test.setTimeout(180_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const stub = new StubLlmServer();
    await stub.start();
    const client = await connectWorkspaceSetupClient();
    let dbBackupId: string | null = null;
    let filesBackupId: string | null = null;

    try {
      await configureStubLlmProvider(client, stub);
      dbBackupId = await createScheduleViaApi(client, {
        name: "DB backup",
        prompt: "Back up the database",
        everyMs: 3_600_000,
      });
      filesBackupId = await createScheduleViaApi(client, {
        name: "Files backup",
        prompt: "Back up the files",
        everyMs: 3_600_000,
      });

      stub.setContent(
        fencedJson({
          kind: "clarify",
          message: "Which one — 'DB backup' or 'Files backup'?",
        }),
      );

      await swallowMetroHmr(page);
      await openSchedulesScreen(page, serverId);
      await expect(page.getByText("DB backup").first()).toBeVisible({ timeout: 30_000 });
      const panel = await openAssistantPanel(page);

      await sendAssistantMessage(panel, "Pause the backup");

      // The clarify message renders as a plain assistant bubble — no proposal card.
      await expect(panel.getByText(/Which one — 'DB backup' or 'Files backup'\?/)).toBeVisible({
        timeout: 30_000,
      });
      await expect(panel.getByTestId("proposal-card")).toHaveCount(0);
    } finally {
      await deleteScheduleQuietly(client, dbBackupId);
      await deleteScheduleQuietly(client, filesBackupId);
      await client.close();
      await stub.close();
    }
  });

  test("updates a schedule via Ask AI from the edit modal", async ({ page }) => {
    test.setTimeout(180_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const stub = new StubLlmServer();
    await stub.start();
    const client = await connectWorkspaceSetupClient();
    let scheduleId: string | null = null;

    try {
      await configureStubLlmProvider(client, stub);
      scheduleId = await createScheduleViaApi(client, {
        name: "Log rotation",
        prompt: "Rotate application logs",
        everyMs: 3_600_000,
      });

      stub.setContent(
        fencedJson({
          kind: "proposal",
          op: "update",
          scheduleId,
          prompt: "Rotate and compress application logs",
          summary: "Update 'Log rotation' prompt to include compression",
        }),
      );

      await swallowMetroHmr(page);
      await openSchedulesScreen(page, serverId);
      await expect(page.getByText("Log rotation").first()).toBeVisible({ timeout: 30_000 });

      // Open the edit modal for this schedule
      await page.locator(`[data-action="edit-${scheduleId}"]`).click();
      await expect(page.getByTestId("schedule-edit-modal")).toBeVisible({ timeout: 15_000 });

      // Click Ask AI — edit modal closes, assistant panel opens
      await page.getByTestId("schedule-edit-ask-ai").click();
      await expect(page.getByTestId("schedule-edit-modal")).toHaveCount(0, { timeout: 10_000 });
      const panel = page.getByTestId("schedule-assistant-panel");
      await expect(panel).toBeVisible({ timeout: 15_000 });

      await sendAssistantMessage(panel, "Also compress the logs after rotating");

      const card = panel.getByTestId("proposal-card");
      await expect(card).toBeVisible({ timeout: 30_000 });
      await expect(card.getByText("update", { exact: true })).toBeVisible();

      await card.getByTestId("proposal-confirm-button").click();

      await expect(panel.getByText('Updated ✓ "Log rotation"')).toBeVisible({ timeout: 20_000 });

      const inspected = await client.scheduleInspect({ id: scheduleId });
      expect(inspected.schedule?.prompt).toBe("Rotate and compress application logs");
    } finally {
      await deleteScheduleQuietly(client, scheduleId);
      await client.close();
      await stub.close();
    }
  });
});

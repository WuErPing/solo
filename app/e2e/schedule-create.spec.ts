/**
 * E2E tests for schedule save, load and edit.
 *
 * The test daemon enables the mock provider, so schedules can target it and
 * spawn a new agent instance per run.
 */
import { randomUUID } from "node:crypto";
import { test, expect, type Page } from "./fixtures";
import { connectWorkspaceSetupClient } from "./helpers/workspace-setup";

interface CreatedSchedule {
  id: string;
  name: string;
  prompt: string;
}

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

async function createScheduleViaApi(name: string, prompt: string): Promise<CreatedSchedule> {
  const client = await connectWorkspaceSetupClient();
  try {
    const result = await client.scheduleCreate({
      requestId: `req-${randomUUID()}`,
      prompt,
      name,
      cadence: { type: "every", everyMs: 60_000 },
      target: { type: "provider", providerId: "mock" },
    });
    if (!result.schedule) {
      throw new Error(result.error ?? "scheduleCreate returned no schedule");
    }
    return { id: result.schedule.id, name, prompt };
  } finally {
    await client.close();
  }
}

async function deleteScheduleViaApi(id: string): Promise<void> {
  const client = await connectWorkspaceSetupClient();
  try {
    await client.scheduleDelete({ id });
  } finally {
    await client.close();
  }
}

async function openCreateModal(page: Page, serverId: string): Promise<void> {
  await page.goto(`/h/${serverId}/schedules?create=true`);
  await expect(page.getByText("New Schedule").first()).toBeVisible({ timeout: 15_000 });
}

async function openSchedulesList(page: Page, serverId: string): Promise<void> {
  await page.goto(`/h/${serverId}/schedules`);
  await expect(page.getByText("Schedules").first()).toBeVisible({ timeout: 15_000 });
}

test.describe("schedule management", () => {
  test.describe.configure({ timeout: 120_000 });

  test("saves a new schedule via the UI and shows it in the list", async ({ page }) => {
    test.setTimeout(120_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    await swallowMetroHmr(page);

    await openCreateModal(page, serverId);

    const scheduleName = `e2e-save-${Date.now()}`;
    await page.getByTestId("schedule-create-name-input").first().fill(scheduleName);
    await page.getByTestId("schedule-create-prompt-input").first().fill("echo hello from schedule");

    // Switch to interval cadence for a simple, fast-recurring schedule.
    await page.getByText("Interval", { exact: true }).first().click();
    await page.getByTestId("schedule-create-interval-input").first().fill("60000");

    // Select the mock provider.
    const providerCard = page.getByTestId("schedule-create-provider-card").first();
    await expect(providerCard).toBeVisible({ timeout: 30_000 });
    await providerCard.click();

    await page.getByTestId("schedule-create-submit-button").first().click();

    // Modal should close and the new schedule should appear in the list.
    await expect(page.getByText("New Schedule").first()).not.toBeVisible({ timeout: 20_000 });
    await expect(page.getByText(scheduleName).first()).toBeVisible({ timeout: 15_000 });
  });

  test("loads existing schedules on the schedules page", async ({ page }) => {
    test.setTimeout(120_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const schedule = await createScheduleViaApi(`e2e-load-${Date.now()}`, "echo load");

    try {
      await swallowMetroHmr(page);
      await openSchedulesList(page, serverId);

      await expect(page.getByText(schedule.name).first()).toBeVisible({ timeout: 15_000 });
    } finally {
      await deleteScheduleViaApi(schedule.id);
    }
  });

  test("edits a schedule and reflects the change in the list", async ({ page }) => {
    test.setTimeout(120_000);

    const serverId = getRequiredEnv("E2E_SERVER_ID");
    const schedule = await createScheduleViaApi(`e2e-edit-${Date.now()}`, "echo before");

    try {
      await swallowMetroHmr(page);
      await openSchedulesList(page, serverId);

      await page.locator(`[data-action="edit-${schedule.id}"]`).first().click();
      await expect(page.getByText("Edit Schedule").first()).toBeVisible({ timeout: 15_000 });

      const newName = `${schedule.name}-edited`;
      await page.getByPlaceholder("Optional name").first().fill(newName);
      await page.getByPlaceholder("What should the agent do?").first().fill("echo after");

      // Ensure a provider is selected before saving.
      const providerCard = page.getByTestId("schedule-edit-provider-card").first();
      await expect(providerCard).toBeVisible({ timeout: 30_000 });
      await providerCard.click();

      await page.getByTestId("schedule-edit-submit-button").first().click();

      await expect(page.getByText("Edit Schedule").first()).not.toBeVisible({ timeout: 20_000 });
      await expect(page.getByText(newName).first()).toBeVisible({ timeout: 15_000 });
    } finally {
      await deleteScheduleViaApi(schedule.id);
    }
  });
});

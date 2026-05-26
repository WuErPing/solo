import { expect } from "@playwright/test";

export interface CoreDaemonClient {
  connect(): Promise<void>;
  close(): Promise<void>;
  openProject(cwd: string): Promise<{
    workspace: {
      id: string;
      name: string;
      workspaceDirectory: string;
      projectRootPath: string;
    } | null;
    error: string | null;
  }>;
  createAgent(input: {
    provider?: string;
    cwd?: string;
    config?: {
      provider: string;
      cwd: string;
      title?: string;
    };
    initialPrompt?: string;
    labels?: Record<string, string>;
  }): Promise<{ id: string; cwd: string; title?: string | null }>;
  sendMessage(agentId: string, text: string): Promise<void>;
  fetchAgentTimeline(
    agentId: string,
    options?: {
      direction?: "tail" | "after" | "before";
      limit?: number;
      cursor?: { epoch: string; seq: number };
    },
  ): Promise<{
    entries: Array<{ item: unknown; seqStart?: number; seqEnd?: number }>;
    startCursor?: { epoch: string; seq: number } | null;
    endCursor?: { epoch: string; seq: number } | null;
    hasOlder?: boolean;
    hasNewer?: boolean;
    window?: { minSeq: number; maxSeq: number; nextSeq: number };
  }>;
  deleteAgent(agentId: string): Promise<void>;
}

export function getDaemonWsUrl(): string {
  const daemonPort = process.env.E2E_DAEMON_PORT;
  if (!daemonPort) {
    throw new Error("E2E_DAEMON_PORT is not set.");
  }
  return `ws://127.0.0.1:${daemonPort}/ws`;
}

export function getServerId(): string {
  const serverId = process.env.E2E_SERVER_ID;
  if (!serverId) {
    throw new Error("E2E_SERVER_ID is not set.");
  }
  return serverId;
}

/**
 * Poll the agent timeline until an entry of the given type appears.
 * Returns the matching entry text or throws on timeout.
 */
export async function waitForTimelineItemType(
  client: CoreDaemonClient,
  agentId: string,
  itemType: string,
  timeoutMs = 60_000,
): Promise<string> {
  let lastEntries: Array<{ item: unknown }> = [];
  await expect
    .poll(
      async () => {
        const timeline = await client.fetchAgentTimeline(agentId, { direction: "tail", limit: 100 });
        lastEntries = timeline.entries;
        return timeline.entries.some((entry) => {
          const item = entry.item as { type?: string };
          return item?.type === itemType;
        });
      },
      { timeout: timeoutMs, intervals: [1000, 2000, 3000] },
    )
    .toBe(true);

  const match = lastEntries.find((e) => {
    const item = e.item as { type?: string };
    return item?.type === itemType;
  });
  const item = match?.item as { type?: string; text?: string };
  return item?.text ?? "";
}

/**
 * Poll the agent timeline until an entry whose JSON includes the given text appears.
 */
export async function waitForTimelineText(
  client: CoreDaemonClient,
  agentId: string,
  text: string,
  timeoutMs = 30_000,
): Promise<void> {
  await expect
    .poll(
      async () => {
        const timeline = await client.fetchAgentTimeline(agentId, { direction: "tail", limit: 50 });
        return timeline.entries.some((entry) => JSON.stringify(entry.item).includes(text));
      },
      { timeout: timeoutMs },
    )
    .toBe(true);
}

/**
 * Wait for the timeline to contain at least N entries of the given type.
 */
export async function waitForTimelineItemCount(
  client: CoreDaemonClient,
  agentId: string,
  itemType: string,
  count: number,
  timeoutMs = 60_000,
): Promise<void> {
  await expect
    .poll(
      async () => {
        const timeline = await client.fetchAgentTimeline(agentId, { direction: "tail", limit: 200 });
        const items = timeline.entries.filter((entry) => {
          const item = entry.item as { type?: string };
          return item?.type === itemType;
        });
        return items.length >= count;
      },
      { timeout: timeoutMs, intervals: [1000, 2000, 3000] },
    )
    .toBe(true);
}

/**
 * Fetch all timeline entries for an agent (tail direction with a large limit).
 */
export async function fetchAllTimelineEntries(
  client: CoreDaemonClient,
  agentId: string,
): Promise<Array<{ item: unknown }>> {
  const timeline = await client.fetchAgentTimeline(agentId, { direction: "tail", limit: 500 });
  return timeline.entries;
}

/**
 * Return the number of duplicate rows in a timeline based on a dedup key function.
 */
export function countDuplicateRows<T>(
  entries: Array<{ item: unknown }>,
  keyFn: (item: T) => string,
): number {
  const seen = new Set<string>();
  let duplicates = 0;
  for (const entry of entries) {
    const item = entry.item as T;
    const key = keyFn(item);
    if (seen.has(key)) {
      duplicates++;
    } else {
      seen.add(key);
    }
  }
  return duplicates;
}

/**
 * Send a message to an agent with retry backoff.
 * The mock provider serializes turns, so rapid sequential sends may fail
 * with "agent is already running". This helper retries up to 10 times.
 */
export async function sendMessageWithRetry(
  client: CoreDaemonClient,
  agentId: string,
  text: string,
  maxRetries = 10,
  baseDelayMs = 200,
): Promise<void> {
  for (let attempt = 0; attempt <= maxRetries; attempt++) {
    try {
      await client.sendMessage(agentId, text);
      return;
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      if (message.includes("already running") && attempt < maxRetries) {
        await new Promise((r) => setTimeout(r, baseDelayMs * (attempt + 1)));
        continue;
      }
      throw error;
    }
  }
}

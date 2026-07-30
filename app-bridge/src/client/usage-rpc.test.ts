import { describe, expect, it, vi, afterEach } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

afterEach(() => {
  vi.useRealTimers();
});

function findSentMessage(
  transport: { sentMessages: Array<{ parsed: { type: string; message?: unknown } }> },
  messageType: string,
) {
  return transport.sentMessages.find(
    (m) =>
      m.parsed.type === "session" &&
      (m.parsed as { message?: { type?: string } }).message?.type === messageType,
  );
}

function getRequestId(sent: { parsed: { type: string; message?: unknown } } | undefined): string {
  return (sent?.parsed as { message?: { requestId?: string } }).message?.requestId ?? "";
}

const mockSnapshot = {
  provider: "kimi",
  plan: { name: "Pro", tier: "pro" },
  quotas: [
    {
      name: "weekly",
      label: "Weekly usage",
      used: 42,
      limit: 100,
      usedPct: 42,
      unit: "%",
      windowStart: null,
      resetAt: "2026-07-30T00:00:00Z",
      resetIn: "3d",
    },
  ],
  fetchedAt: "2026-07-27T00:00:00Z",
};

describe("UsageRpc — usage quota", () => {
  it("usageQuotaList sends usage/quota/list and resolves with the payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.usageQuotaList({ requestId: "req-quota-list" });

    const sent = findSentMessage(transport, "usage/quota/list");
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: "req-quota-list",
        snapshots: [mockSnapshot],
        errors: {},
        cachedAt: "2026-07-27T00:00:05Z",
        error: null,
      },
    });

    const result = await promise;
    expect(result.snapshots).toHaveLength(1);
    expect(result.snapshots[0]?.provider).toBe("kimi");
    expect(result.snapshots[0]?.quotas[0]?.usedPct).toBe(42);
    expect(result.cachedAt).toBe("2026-07-27T00:00:05Z");
    expect(result.error).toBeNull();
    await cleanup();
  });

  it("serializes forceRefresh only when provided", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.usageQuotaList({ forceRefresh: true });

    const sent = findSentMessage(transport, "usage/quota/list");
    expect(sent).toBeDefined();
    const wireMessage = (sent!.parsed as { message?: Record<string, unknown> }).message;
    expect(wireMessage).toMatchObject({ type: "usage/quota/list", forceRefresh: true });

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: getRequestId(sent),
        snapshots: [],
        cachedAt: "2026-07-27T00:00:05Z",
        error: null,
      },
    });
    await promise;

    const defaultPromise = client.usageQuotaList();
    const defaultSent = transport.sentMessages
      .filter(
        (m) =>
          m.parsed.type === "session" &&
          (m.parsed as { message?: { type?: string } }).message?.type === "usage/quota/list",
      )
      .at(-1);
    const defaultWire = (defaultSent!.parsed as { message?: Record<string, unknown> }).message;
    expect(defaultWire).not.toHaveProperty("forceRefresh");

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: getRequestId(defaultSent),
        snapshots: [],
        cachedAt: "2026-07-27T00:00:06Z",
        error: null,
      },
    });
    await defaultPromise;
    await cleanup();
  });

  it("preserves per-provider errors", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.usageQuotaList({ requestId: "req-quota-errors" });

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: "req-quota-errors",
        snapshots: [mockSnapshot],
        errors: { anthropic: "rate limited" },
        cachedAt: "2026-07-27T00:00:05Z",
        error: null,
      },
    });

    const result = await promise;
    expect(result.snapshots).toHaveLength(1);
    expect(result.errors?.anthropic).toBe("rate limited");
    await cleanup();
  });

  it("resolves (not rejects) a top-level daemon error payload", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.usageQuotaList({ requestId: "req-quota-top-err" });

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: "req-quota-top-err",
        snapshots: [],
        cachedAt: "2026-07-27T00:00:05Z",
        error: "usage service unavailable",
      },
    });

    const result = await promise;
    expect(result.error).toBe("usage service unavailable");
    expect(result.snapshots).toHaveLength(0);
    await cleanup();
  });

  it("drops a malformed response and still resolves on a valid one", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.usageQuotaList({ requestId: "req-quota-malformed" });

    // Missing `cachedAt` — fails SessionOutboundMessageSchema validation.
    transport.simulateMessage(
      JSON.stringify({
        type: "session",
        message: {
          type: "usage/quota/list/response",
          payload: {
            requestId: "req-quota-malformed",
            snapshots: [],
            error: null,
          },
        },
      }),
    );

    simulateServerResponse(transport, {
      type: "usage/quota/list/response",
      payload: {
        requestId: "req-quota-malformed",
        snapshots: [mockSnapshot],
        cachedAt: "2026-07-27T00:00:07Z",
        error: null,
      },
    });

    const result = await promise;
    expect(result.cachedAt).toBe("2026-07-27T00:00:07Z");
    expect(result.snapshots).toHaveLength(1);
    await cleanup();
  });
});

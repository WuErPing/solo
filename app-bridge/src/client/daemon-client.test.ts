import { describe, expect, it, vi, afterEach } from "vitest";
import { DaemonClient } from "./daemon-client.js";
import { MockTransport, createMockTransportFactory } from "./mock-transport.js";
import {
  createConnectedClient,
  simulateServerResponse,
  buildSessionMessage,
  mockAgentSnapshot,
  mockWorkspaceDescriptor,
} from "./daemon-client-test-harness.js";

afterEach(() => {
  vi.useRealTimers();
});

describe("DaemonClient — connection lifecycle", () => {
  it("sends hello message on transport open", () => {
    const transport = new MockTransport();
    const client = new DaemonClient({
      url: "ws://localhost:17612",
      clientId: "test-client",
      clientType: "cli",
      reconnect: { enabled: false },
      transportFactory: createMockTransportFactory(transport),
      connectTimeoutMs: 60000,
      suppressSendErrors: true,
    });

    void client.connect();
    transport.simulateOpen();

    const hello = transport.findSentMessage("hello");
    expect(hello).toBeDefined();
    expect(hello!.parsed.clientId).toBe("test-client");
    expect(hello!.parsed.clientType).toBe("cli");
    expect(hello!.parsed.protocolVersion).toBe(2);
  });

  it("transitions to connected after receiving server_info", async () => {
    const { client, cleanup } = createConnectedClient();
    expect(client.isConnected).toBe(true);
    expect(client.getConnectionState().status).toBe("connected");
    await cleanup();
  });

  it("stores last server info message", async () => {
    const { client, cleanup } = createConnectedClient();
    const info = client.getLastServerInfoMessage();
    expect(info).not.toBeNull();
    expect(info!.serverId).toBe("test-server-id");
    expect(info!.hostname).toBe("test-host");
    await cleanup();
  });

  it("transitions to disposed on close", async () => {
    const { client, cleanup } = createConnectedClient();
    await cleanup();
    expect(client.getConnectionState().status).toBe("disposed");
  });

  it("emits connection state changes to subscribers", async () => {
    const transport = new MockTransport();
    const client = new DaemonClient({
      url: "ws://localhost:17612",
      clientId: "test-client",
      clientType: "cli",
      reconnect: { enabled: false },
      transportFactory: createMockTransportFactory(transport),
      connectTimeoutMs: 60000,
      suppressSendErrors: true,
    });

    const states: string[] = [];
    client.subscribeConnectionStatus((s) => states.push(s.status));

    void client.connect();
    transport.simulateOpen();
    transport.simulateMessage(
      buildSessionMessage({
        type: "status",
        payload: { status: "server_info", serverId: "s1" },
      }),
    );

    expect(states).toContain("connecting");
    expect(states).toContain("connected");

    await client.close();
    expect(states).toContain("disposed");
  });
});

describe("DaemonClient — RPC primitives", () => {
  it("sendRequest resolves when matching response arrives", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const responsePromise = client.fetchAgents({ requestId: "req-1" });

    const sent = transport.sentMessages.find(
      (m) => m.parsed.type === "session" &&
        (m.parsed as { message?: { type?: string } }).message?.type === "fetch_agents_request",
    );
    expect(sent).toBeDefined();

    simulateServerResponse(transport, {
      type: "fetch_agents_response",
      payload: {
        requestId: "req-1",
        entries: [],
        pageInfo: { nextCursor: null, prevCursor: null, hasMore: false },
      },
    });

    const result = await responsePromise;
    expect(result.entries).toEqual([]);
    expect(result.pageInfo.hasMore).toBe(false);

    await cleanup();
  });

  it("sendRequest rejects on rpc_error", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const responsePromise = client.fetchAgents({ requestId: "req-err" });

    simulateServerResponse(transport, {
      type: "rpc_error",
      payload: {
        requestId: "req-err",
        error: "Internal error",
        requestType: "fetch_agents_request",
        code: "INTERNAL",
      },
    });

    await expect(responsePromise).rejects.toThrow("Internal error");
    await cleanup();
  });

  it("sendRequest times out when no response", async () => {
    vi.useFakeTimers();
    const { client, cleanup } = createConnectedClient({ connectTimeoutMs: 60000 });

    const responsePromise = client.fetchAgents({ requestId: "req-timeout" });
    vi.advanceTimersByTime(11000);

    await expect(responsePromise).rejects.toThrow("Timeout waiting for message");
    await cleanup();
  });

  it("sendCorrelatedSessionRequest correlates by requestId", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const responsePromise = client.openProject("/test/cwd", "req-open-1");

    const sent = transport.sentMessages.find(
      (m) => m.parsed.type === "session" &&
        (m.parsed as { message?: { type?: string } }).message?.type === "open_project_request",
    );
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/cwd");

    simulateServerResponse(transport, {
      type: "open_project_response",
      payload: {
        requestId: "req-open-1",
        workspace: mockWorkspaceDescriptor(),
        error: null,
      },
    });

    const result = await responsePromise;
    expect(result.requestId).toBe("req-open-1");
    expect(result.workspace).not.toBeNull();
    await cleanup();
  });
});

describe("DaemonClient — subscriptions and events", () => {
  it("subscribe receives events", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const events: string[] = [];
    const unsub = client.subscribe((event) => events.push(event.type));

    simulateServerResponse(transport, {
      type: "agent_update",
      payload: {
        kind: "upsert",
        agent: mockAgentSnapshot({ id: "agent-1" }),
      },
    });

    expect(events).toContain("agent_update");
    unsub();

    simulateServerResponse(transport, {
      type: "agent_update",
      payload: {
        kind: "upsert",
        agent: mockAgentSnapshot({ id: "agent-2" }),
      },
    });

    expect(events).toHaveLength(1);
    await cleanup();
  });

  it("subscribeRawMessages receives all session messages", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const messages: string[] = [];
    const unsub = client.subscribeRawMessages((msg) => messages.push(msg.type));

    simulateServerResponse(transport, {
      type: "agent_update",
      payload: { kind: "upsert", agent: mockAgentSnapshot() },
    });
    simulateServerResponse(transport, {
      type: "workspace_update",
      payload: { kind: "upsert", workspace: mockWorkspaceDescriptor() },
    });

    expect(messages).toEqual(["agent_update", "workspace_update"]);
    unsub();
    await cleanup();
  });

  it("on(type, handler) receives typed messages", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const agentUpdates: string[] = [];
    client.on("agent_update", (msg) => {
      const payload = (msg as { payload?: { agent?: { id?: string } } }).payload;
      agentUpdates.push(payload?.agent?.id ?? "");
    });

    simulateServerResponse(transport, {
      type: "agent_update",
      payload: { kind: "upsert", agent: mockAgentSnapshot({ id: "a1" }) },
    });
    simulateServerResponse(transport, {
      type: "workspace_update",
      payload: { kind: "upsert", workspace: mockWorkspaceDescriptor() },
    });

    expect(agentUpdates).toEqual(["a1"]);
    await cleanup();
  });
});

describe("DaemonClient — fire-and-forget messages", () => {
  it("sendHeartbeat sends client_heartbeat without waiting", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    client.sendHeartbeat({
      deviceType: "web",
      focusedAgentId: null,
      lastActivityAt: "2026-01-01T00:00:00Z",
      appVisible: true,
    });

    const heartbeat = transport.sentMessages.find(
      (m) => m.parsed.type === "session" &&
        (m.parsed as { message?: { type?: string } }).message?.type === "client_heartbeat",
    );
    expect(heartbeat).toBeDefined();
    await cleanup();
  });

  it("registerPushToken sends register_push_token", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    client.registerPushToken("token-123");

    const msg = transport.sentMessages.find(
      (m) => m.parsed.type === "session" &&
        (m.parsed as { message?: { type?: string } }).message?.type === "register_push_token",
    );
    expect(msg).toBeDefined();
    await cleanup();
  });
});

describe("DaemonClient — resource caps", () => {
  it("rejects new waiters once the waiter cap is reached", async () => {
    vi.useFakeTimers();
    const { client, cleanup } = createConnectedClient();

    const handles = [];
    for (let i = 0; i < 500; i += 1) {
      const handle = client.waitForWithCancel(() => null, 30000);
      handle.promise.catch(() => undefined);
      handles.push(handle);
    }

    const overflow = client.waitForWithCancel(() => null, 30000);
    await expect(overflow.promise).rejects.toThrow("TooManyPendingRequests");

    for (const handle of handles) {
      handle.cancel(new Error("teardown"));
    }
    await cleanup();
  });

  it("drops the oldest queued send when the pending-send cap is exceeded", async () => {
    vi.useFakeTimers();
    const transport = new MockTransport();
    const client = new DaemonClient({
      url: "ws://localhost:17612",
      clientId: "test-client-id",
      clientType: "cli",
      reconnect: { enabled: false },
      transportFactory: createMockTransportFactory(transport),
      connectTimeoutMs: 60000,
      suppressSendErrors: true,
    });

    // Transport never opens, so the client stays in "connecting" and sends queue up.
    void client.connect();

    const internals = (
      client as unknown as {
        connection: {
          sendSessionMessageOrThrow: (message: unknown) => Promise<void>;
          pendingSendQueue: unknown[];
          rejectPendingSendQueue: (error: Error) => void;
        };
      }
    ).connection;

    const message = {
      type: "client_heartbeat",
      payload: {
        deviceType: "web",
        focusedAgentId: null,
        lastActivityAt: "2026-01-01T00:00:00Z",
        appVisible: true,
      },
    };

    const queued: Promise<void>[] = [];
    for (let i = 0; i < 1000; i += 1) {
      const pending = internals.sendSessionMessageOrThrow(message);
      pending.catch(() => undefined);
      queued.push(pending);
    }
    expect(internals.pendingSendQueue.length).toBe(1000);

    const overflowSend = internals.sendSessionMessageOrThrow(message);
    overflowSend.catch(() => undefined);

    await expect(queued[0]).rejects.toThrow("Send queue overflow");
    expect(internals.pendingSendQueue.length).toBe(1000);

    internals.rejectPendingSendQueue(new Error("teardown"));
  });
});

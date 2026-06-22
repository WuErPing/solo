import { DaemonClient } from "./daemon-client.js";
import type { DaemonClientConfig } from "./daemon-client.js";
import type { SessionOutboundMessage } from "../shared/messages.js";
import { MockTransport, createMockTransportFactory } from "./mock-transport.js";

const TEST_SERVER_ID = "test-server-id";
const TEST_HOSTNAME = "test-host";
const TEST_CLIENT_ID = "test-client-id";

export interface ConnectedClient {
  client: DaemonClient;
  transport: MockTransport;
  cleanup: () => Promise<void>;
}

export function createConnectedClient(configOverrides?: Partial<DaemonClientConfig>): ConnectedClient {
  const transport = new MockTransport();

  const config: DaemonClientConfig = {
    url: "ws://localhost:17612",
    clientId: TEST_CLIENT_ID,
    clientType: "cli",
    reconnect: { enabled: false },
    transportFactory: createMockTransportFactory(transport),
    connectTimeoutMs: 60000,
    suppressSendErrors: true,
    ...configOverrides,
  };

  const client = new DaemonClient(config);

  void client.connect();
  transport.simulateOpen();

  const helloMessage = transport.sentMessages.find((m) => m.parsed.type === "hello");
  if (!helloMessage) {
    throw new Error("Client did not send hello message on open");
  }

  transport.simulateMessage(
    JSON.stringify({
      type: "session",
      message: {
        type: "status",
        payload: {
          status: "server_info",
          serverId: TEST_SERVER_ID,
          hostname: TEST_HOSTNAME,
        },
      },
    }),
  );

  const cleanup = async (): Promise<void> => {
    await client.close();
  };

  return { client, transport, cleanup };
}

export function buildSessionMessage(message: SessionOutboundMessage): string {
  return JSON.stringify({ type: "session", message });
}

export function simulateServerResponse(
  transport: MockTransport,
  message: SessionOutboundMessage,
): void {
  transport.simulateMessage(buildSessionMessage(message));
}

export function extractRequestId(
  transport: MockTransport,
  messageType: string,
): string | undefined {
  const captured = transport.sentMessages.find(
    (m) => m.parsed.type === "session" && (m.parsed as { message?: { type?: string } }).message?.type === messageType,
  );
  const message = (captured?.parsed as { message?: { requestId?: string } }).message;
  return message?.requestId;
}

export function mockAgentSnapshot(overrides?: Record<string, unknown>) {
  return {
    id: "agent-1",
    provider: "claude-code",
    cwd: "/test/project",
    model: "claude-sonnet-4",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
    lastUserMessageAt: null,
    status: "idle" as const,
    capabilities: {
      supportsStreaming: true,
      supportsSessionPersistence: true,
      supportsDynamicModes: false,
      supportsMcpServers: false,
      supportsReasoningStream: false,
      supportsToolInvocations: true,
    },
    currentModeId: null,
    pendingPermissions: [],
    persistence: null,
    title: null,
    labels: {},
    ...overrides,
  };
}

export function mockWorkspaceDescriptor(overrides?: Record<string, unknown>) {
  return {
    id: "ws-1",
    projectId: "proj-1",
    projectDisplayName: "Test Project",
    projectRootPath: "/test/project",
    projectKind: "non_git" as const,
    workspaceKind: "directory" as const,
    name: "test-workspace",
    workspaceDirectory: "/test/project",
    status: "done" as const,
    activityAt: null,
    scripts: [],
    gitRuntime: null,
    githubRuntime: null,
    ...overrides,
  };
}

import type { ConnectionManager } from "./connection-manager.js";
import {
  SessionInboundMessageSchema,
  type SessionOutboundMessage,
} from "../shared/messages.js";
import {
  DaemonVersionSwitchRequestedStatusPayloadSchema,
  type DaemonVersionSwitchRequestedStatusPayload,
} from "../server/version/rpc-schemas.js";

type ListDaemonVersionsPayload = Extract<
  SessionOutboundMessage,
  { type: "list_daemon_versions_response" }
>["payload"];

export class VersionRpc {
  constructor(private readonly client: ConnectionManager) {}

  async listDaemonVersions(requestId?: string): Promise<ListDaemonVersionsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "list_daemon_versions_request",
      },
      responseType: "list_daemon_versions_response",
      timeout: 10000,
    });
  }

  async switchDaemonVersion(
    version?: string,
    requestId?: string,
  ): Promise<DaemonVersionSwitchRequestedStatusPayload> {
    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "switch_daemon_version_request",
      ...(version && version.trim().length > 0 ? { version } : {}),
      requestId: resolvedRequestId,
    });
    return this.client.sendRequest({
      requestId: resolvedRequestId,
      message,
      timeout: 10000,
      options: { skipQueue: true },
      select: (msg) => {
        if (msg.type !== "status") {
          return null;
        }
        const switched = DaemonVersionSwitchRequestedStatusPayloadSchema.safeParse(msg.payload);
        if (!switched.success) {
          return null;
        }
        if (switched.data.requestId !== resolvedRequestId) {
          return null;
        }
        return switched.data;
      },
    });
  }
}

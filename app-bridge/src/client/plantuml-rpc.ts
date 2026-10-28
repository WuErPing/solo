import type { SessionOutboundMessage } from "../shared/messages.js";
import type { ConnectionManager } from "./connection-manager.js";

export type PlantUMLRenderPayload = Extract<
  SessionOutboundMessage,
  { type: "plantuml_render_response" }
>["payload"];

export class PlantUmlRpc {
  constructor(private readonly client: ConnectionManager) {}

  async render(
    source: string,
    dark: boolean,
    requestId?: string,
  ): Promise<PlantUMLRenderPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "plantuml_render_request",
        source,
        dark,
      },
      responseType: "plantuml_render_response",
      // Rendering runs on the daemon host; a cold JVM start can take seconds.
      timeout: 30000,
    });
  }
}

import { describe, expect, it } from "vitest";
import { createConnectedClient, simulateServerResponse } from "./daemon-client-test-harness.js";

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

describe("PlantUmlRpc", () => {
  it("render sends plantuml_render_request and resolves with SVG", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.plantuml.render("@startuml\nA -> B\n@enduml", true, "req-puml-1");

    const sent = findSentMessage(transport, "plantuml_render_request");
    expect(sent).toBeDefined();
    const message = (sent!.parsed as { message: { source: string; dark: boolean; requestId: string } })
      .message;
    expect(message.source).toBe("@startuml\nA -> B\n@enduml");
    expect(message.dark).toBe(true);
    expect(message.requestId).toBe("req-puml-1");

    simulateServerResponse(transport, {
      type: "plantuml_render_response",
      payload: {
        svg: '<svg xmlns="http://www.w3.org/2000/svg"></svg>',
        error: null,
        requestId: "req-puml-1",
      },
    });

    const result = await promise;
    expect(result.svg).toContain("<svg");
    expect(result.error).toBeNull();
    await cleanup();
  });

  it("render resolves with an error payload from the daemon", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.plantuml.render("@startuml\nbogus ->\n@enduml", false, "req-puml-2");

    simulateServerResponse(transport, {
      type: "plantuml_render_response",
      payload: {
        svg: "",
        error: "PlantUML renderer unavailable: install PlantUML",
        requestId: "req-puml-2",
      },
    });

    const result = await promise;
    expect(result.svg).toBe("");
    expect(result.error).toContain("install PlantUML");
    await cleanup();
  });
});

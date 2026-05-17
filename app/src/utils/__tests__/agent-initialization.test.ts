import { describe, expect, it } from "vitest";
import * as init from "@/utils/agent-initialization";

describe("agent initialization deferreds", () => {
  it("clears only pending deferreds for the requested server", async () => {
    const serverAAgent1 = init.createInitDeferred("server-a:agent-1", "tail");
    const serverAAgent2 = init.createInitDeferred("server-a:agent-2", "after");
    const serverBAgent1 = init.createInitDeferred("server-b:agent-1", "tail");

    const serverAAgent1Result = serverAAgent1.promise.catch((error) => error);
    const serverAAgent2Result = serverAAgent2.promise.catch((error) => error);
    const serverBAgent1Result = serverBAgent1.promise.then(
      () => "resolved",
      (error) => error,
    );

    try {
      const clearInitDeferredsForServer = (
        init as typeof init & {
          clearInitDeferredsForServer?: (serverId: string, error: Error) => void;
        }
      ).clearInitDeferredsForServer;

      expect(clearInitDeferredsForServer).toBeTypeOf("function");
      clearInitDeferredsForServer?.("server-a", new Error("Disconnected"));

      await expect(serverAAgent1Result).resolves.toMatchObject({ message: "Disconnected" });
      await expect(serverAAgent2Result).resolves.toMatchObject({ message: "Disconnected" });
      expect(init.getInitDeferred("server-a:agent-1")).toBeUndefined();
      expect(init.getInitDeferred("server-a:agent-2")).toBeUndefined();
      expect(init.getInitDeferred("server-b:agent-1")).toBe(serverBAgent1);
    } finally {
      init.rejectInitDeferred("server-a:agent-1", new Error("cleanup"));
      init.rejectInitDeferred("server-a:agent-2", new Error("cleanup"));
      init.resolveInitDeferred("server-b:agent-1");
      await serverBAgent1Result;
    }
  });
});


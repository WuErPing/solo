import { describe, expect, it } from "vitest";
import { WorkspaceDescriptorSchema } from "../generated/protocol-schemas.js";
import {
  AgentTimelineEntryPayloadSchema,
  WorkspaceDescriptorSchemaWithDefault,
  FetchWorkspacesResponseMessageSchema,
} from "./messages.js";

describe("Generated WorkspaceDescriptorSchema", () => {
  it("parses a minimal wire payload matching Go WorkspaceDescriptor", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      projectKind: "git",
      workspaceKind: "local_checkout",
      name: "main",
      status: "running",
      activityAt: null,
    };
    const result = WorkspaceDescriptorSchema.parse(wire);
    expect(result.id).toBe("ws-1");
    expect(result.workspaceDirectory).toBeUndefined();
    expect(result.scripts).toBeUndefined();
  });

  it("accepts optional workspaceDirectory", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      workspaceDirectory: "/repo/sub",
      projectKind: "git",
      workspaceKind: "worktree",
      name: "feat-branch",
      status: "running",
      activityAt: null,
    };
    const result = WorkspaceDescriptorSchema.parse(wire);
    expect(result.workspaceDirectory).toBe("/repo/sub");
  });

  it("accepts any string for projectKind, workspaceKind, status", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      projectKind: "some_future_kind",
      workspaceKind: "some_future_workspace",
      name: "main",
      status: "some_future_status",
      activityAt: null,
    };
    // z.string() accepts any string — no enum rejection
    expect(() => WorkspaceDescriptorSchema.parse(wire)).not.toThrow();
  });
});

describe("WorkspaceDescriptorSchemaWithDefault", () => {
  it("defaults workspaceDirectory to projectRootPath when absent", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      projectKind: "git",
      workspaceKind: "local_checkout",
      name: "main",
      status: "running",
      activityAt: null,
    };
    const result = WorkspaceDescriptorSchemaWithDefault.parse(wire);
    expect(result.workspaceDirectory).toBe("/repo");
  });

  it("preserves explicit workspaceDirectory when present", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      workspaceDirectory: "/repo/sub",
      projectKind: "git",
      workspaceKind: "worktree",
      name: "feat-branch",
      status: "running",
      activityAt: null,
    };
    const result = WorkspaceDescriptorSchemaWithDefault.parse(wire);
    expect(result.workspaceDirectory).toBe("/repo/sub");
  });

  it("accepts arbitrary strings for enum-like fields", () => {
    const wire = {
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/repo",
      projectKind: "unknown_kind",
      workspaceKind: "unknown_workspace",
      name: "main",
      status: "unknown_status",
      activityAt: null,
    };
    expect(() => WorkspaceDescriptorSchemaWithDefault.parse(wire)).not.toThrow();
  });
});

describe("Message schemas with generated descriptor", () => {
  it("FetchWorkspacesResponseMessageSchema parses entries", () => {
    const message = {
      type: "fetch_workspaces_response",
      payload: {
        requestId: "req-1",
        entries: [
          {
            id: "ws-1",
            projectId: "proj-1",
            projectDisplayName: "My Project",
            projectRootPath: "/repo",
            projectKind: "git",
            workspaceKind: "local_checkout",
            name: "main",
            status: "done",
            activityAt: null,
            scripts: [
              {
                scriptName: "web",
                hostname: "localhost",
                port: 3000,
                lifecycle: "running",
                health: "healthy",
              },
            ],
          },
        ],
        pageInfo: { nextCursor: null, prevCursor: null, hasMore: false },
      },
    };
    expect(() =>
      FetchWorkspacesResponseMessageSchema.parse(message),
    ).not.toThrow();
  });

  it("FetchWorkspacesResponseMessageSchema applies workspaceDirectory default", () => {
    const message = {
      type: "fetch_workspaces_response",
      payload: {
        requestId: "req-1",
        entries: [
          {
            id: "ws-1",
            projectId: "proj-1",
            projectDisplayName: "My Project",
            projectRootPath: "/repo",
            projectKind: "git",
            workspaceKind: "local_checkout",
            name: "main",
            status: "done",
            activityAt: null,
          },
        ],
        pageInfo: { nextCursor: null, prevCursor: null, hasMore: false },
      },
    };
    const result = FetchWorkspacesResponseMessageSchema.parse(message);
    expect(result.payload.entries[0].workspaceDirectory).toBe("/repo");
  });
});

describe("Agent timeline history schemas", () => {
  it("accepts a tool call without detail when Go omits the empty field", () => {
    const entry = {
      provider: "kimi",
      item: {
        type: "tool_call",
        callId: "call-without-detail",
        name: "run_command",
        status: "running",
      },
      timestamp: "2026-11-03T17:00:00Z",
      seqStart: 21,
      seqEnd: 21,
      sourceSeqRanges: [{ startSeq: 21, endSeq: 21 }],
      collapsed: [],
    };

    expect(() => AgentTimelineEntryPayloadSchema.parse(entry)).not.toThrow();
  });

  it("accepts a tool call without a name when Go omits the empty field", () => {
    const entry = {
      provider: "kimi",
      item: {
        type: "tool_call",
        callId: "call-without-name",
        status: "running",
      },
      timestamp: "2026-11-03T17:00:00Z",
      seqStart: 22,
      seqEnd: 22,
      sourceSeqRanges: [{ startSeq: 22, endSeq: 22 }],
      collapsed: [],
    };

    expect(() => AgentTimelineEntryPayloadSchema.parse(entry)).not.toThrow();
  });
});

import { describe, it, expect } from "vitest";
import {
  WorkspaceDescriptorSchema,
  WorkspaceScriptSchema,
  WorkspaceGitRuntimeSchema,
  WorkspaceGitHubRuntimeSchema,
} from "./protocol-schemas.js";

describe("generated protocol schemas", () => {
  it("parses a minimal workspace descriptor", () => {
    const result = WorkspaceDescriptorSchema.safeParse({
      id: "ws-1",
      projectId: "proj-1",
      projectDisplayName: "My Project",
      projectRootPath: "/home/user/my-project",
      projectKind: "git",
      workspaceKind: "worktree",
      name: "my-project",
      status: "running",
      activityAt: null,
    });
    expect(result.success).toBe(true);
  });

  it("parses a workspace descriptor with nested runtime and scripts", () => {
    const result = WorkspaceDescriptorSchema.safeParse({
      id: "ws-2",
      projectId: "proj-2",
      projectDisplayName: "Other Project",
      projectRootPath: "/home/user/other-project",
      workspaceDirectory: "/home/user/other-project/.solo-worktrees/feature",
      projectKind: "git",
      workspaceKind: "worktree",
      name: "other-project",
      status: "attention",
      activityAt: "2026-06-23T10:00:00Z",
      scripts: [
        {
          scriptName: "dev",
          hostname: "localhost",
          lifecycle: "running",
        },
      ],
      gitRuntime: {
        currentBranch: "feature",
        remoteUrl: "git@github.com:example/other-project.git",
        isSoloOwnedWorktree: true,
        isDirty: false,
      },
      githubRuntime: {
        featuresEnabled: true,
      },
    });
    expect(result.success).toBe(true);
  });

  it("rejects a workspace descriptor missing required fields", () => {
    const result = WorkspaceDescriptorSchema.safeParse({
      id: "ws-3",
    });
    expect(result.success).toBe(false);
  });

  it("parses workspace script with optional fields omitted", () => {
    const result = WorkspaceScriptSchema.safeParse({
      scriptName: "build",
      hostname: "localhost",
      lifecycle: "completed",
    });
    expect(result.success).toBe(true);
  });

  it("parses git runtime with all fields omitted", () => {
    const result = WorkspaceGitRuntimeSchema.safeParse({});
    expect(result.success).toBe(true);
  });

  it("parses GitHub runtime", () => {
    const result = WorkspaceGitHubRuntimeSchema.safeParse({
      featuresEnabled: false,
    });
    expect(result.success).toBe(true);
  });
});

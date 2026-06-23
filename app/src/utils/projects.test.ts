import { describe, expect, it } from "vitest";
import type { WorkspaceDescriptor } from "@/stores/session-store";
import { buildProjects } from "./projects";

function workspace(input: {
  id: string;
  repoRoot: string;
  projectId?: string;
  projectName?: string;
  remoteUrl?: string | null;
}): WorkspaceDescriptor {
  return {
    id: input.id,
    projectId: input.projectId ?? input.repoRoot,
    projectDisplayName: input.projectName ?? "Project",
    projectRootPath: input.repoRoot,
    workspaceDirectory: input.repoRoot,
    projectKind: "git",
    workspaceKind: "local_checkout",
    name: input.id,
    status: "done",
    scripts: [],
    gitRuntime: {
      currentBranch: "main",
      remoteUrl: input.remoteUrl ?? undefined,
      isSoloOwnedWorktree: false,
      isDirty: false,
    },
    githubRuntime: undefined,
  };
}

describe("buildProjects", () => {
  it("groups two daemons with the same GitHub project key into one project with one host entry per daemon", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "main",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
            workspace({
              id: "feature-a",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
            workspace({
              id: "feature-b",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
          ],
        },
        {
          serverId: "laptop",
          serverName: "Laptop",
          isOnline: true,
          workspaces: [
            workspace({
              id: "main",
              repoRoot: "/work/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "git@github.com:acme/app.git",
            }),
            workspace({
              id: "feature",
              repoRoot: "/work/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "git@github.com:acme/app.git",
            }),
          ],
        },
      ],
    });

    expect(result.projects).toHaveLength(1);
    const summary = result.projects[0];
    expect(summary?.projectKey).toBe("remote:github.com/acme/app");
    expect(summary?.projectName).toBe("acme/app");
    expect(summary?.hostCount).toBe(2);
    expect(summary?.onlineHostCount).toBe(2);
    expect(summary?.totalWorkspaceCount).toBe(5);
    expect(summary?.githubUrl).toBe("https://github.com/acme/app");
    expect(summary?.hosts).toHaveLength(2);
    const local = summary?.hosts.find((host) => host.serverId === "local");
    const laptop = summary?.hosts.find((host) => host.serverId === "laptop");
    expect(local?.workspaceCount).toBe(3);
    expect(laptop?.workspaceCount).toBe(2);
    expect(local?.workspaces.map((entry) => entry.id)).toEqual(["main", "feature-a", "feature-b"]);
    expect(laptop?.workspaces.map((entry) => entry.id)).toEqual(["main", "feature"]);
  });

  it("collapses five workspaces on one host into a single host entry whose workspaceCount is five", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: Array.from({ length: 5 }, (_, index) =>
            workspace({
              id: `ws-${index}`,
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
          ),
        },
      ],
    });

    expect(result.projects).toHaveLength(1);
    expect(result.projects[0]?.hosts).toHaveLength(1);
    expect(result.projects[0]?.hosts[0]?.workspaceCount).toBe(5);
    expect(result.projects[0]?.totalWorkspaceCount).toBe(5);
    expect(result.projects[0]?.hostCount).toBe(1);
  });

  it("uses projectRootPath as the host repoRoot", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "main",
              repoRoot: "/worktrees/app/main",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
          ],
        },
        {
          serverId: "legacy",
          serverName: "Legacy",
          isOnline: true,
          workspaces: [
            workspace({
              id: "legacy",
              repoRoot: "/repo/legacy",
              projectId: "legacy-project",
              projectName: "Legacy",
            }),
          ],
        },
      ],
    });

    const acme = result.projects.find(
      (project) => project.projectKey === "remote:github.com/acme/app",
    );
    const legacy = result.projects.find((project) => project.projectKey === "legacy-project");

    expect(acme?.hosts[0]?.repoRoot).toBe("/worktrees/app/main");
    expect(legacy?.hosts[0]?.repoRoot).toBe("/repo/legacy");
  });

  it("derives githubUrl only when projectKey matches remote:github.com/{owner}/{repo}", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "github",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
            workspace({
              id: "local",
              repoRoot: "/repo/local",
              projectId: "/repo/local",
              projectName: "local",
              remoteUrl: null,
            }),
          ],
        },
      ],
    });

    const github = result.projects.find(
      (project) => project.projectKey === "remote:github.com/acme/app",
    );
    const local = result.projects.find((project) => project.projectKey === "/repo/local");

    expect(github?.githubUrl).toBe("https://github.com/acme/app");
    expect(local?.githubUrl).toBeUndefined();
  });

  it("totals hostCount across all hosts and counts only online ones in onlineHostCount", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "online",
          serverName: "Online",
          isOnline: true,
          workspaces: [
            workspace({
              id: "ws",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
          ],
        },
        {
          serverId: "offline",
          serverName: "Offline",
          isOnline: false,
          workspaces: [
            workspace({
              id: "ws",
              repoRoot: "/repo/app",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
          ],
        },
      ],
    });

    expect(result.projects).toHaveLength(1);
    expect(result.projects[0]?.hostCount).toBe(2);
    expect(result.projects[0]?.onlineHostCount).toBe(1);
    expect(result.projects[0]?.hosts.find((host) => host.serverId === "online")?.isOnline).toBe(
      true,
    );
    expect(result.projects[0]?.hosts.find((host) => host.serverId === "offline")?.isOnline).toBe(
      false,
    );
  });

  it("does not merge fallback repo-root-keyed projects with different roots", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "one",
              repoRoot: "/repo/one",
              projectId: "/repo/one",
              projectName: "one",
              remoteUrl: null,
            }),
            workspace({
              id: "two",
              repoRoot: "/repo/two",
              projectId: "/repo/two",
              projectName: "two",
              remoteUrl: null,
            }),
          ],
        },
      ],
    });

    expect(result.projects.map((project) => project.projectKey)).toEqual([
      "/repo/one",
      "/repo/two",
    ]);
  });

  it("filters non-GitHub remote projects while keeping GitHub and local projects", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "github",
              repoRoot: "/repo/github",
              projectId: "remote:github.com/acme/app",
              projectName: "acme/app",
              remoteUrl: "https://github.com/acme/app.git",
            }),
            workspace({
              id: "gitlab",
              repoRoot: "/repo/gitlab",
              projectId: "remote:gitlab.com/acme/app",
              projectName: "app",
              remoteUrl: "https://gitlab.com/acme/app.git",
            }),
            workspace({
              id: "local",
              repoRoot: "/repo/local",
              projectId: "/repo/local",
              projectName: "local",
              remoteUrl: null,
            }),
          ],
        },
      ],
    });

    expect(result.hiddenUnsupportedRemoteCount).toBe(1);
    expect(result.projects.map((project) => project.projectKey)).toEqual([
      "remote:github.com/acme/app",
      "/repo/local",
    ]);
  });

  it("produces the unsupported empty-state signal when only non-GitHub remote projects are present", () => {
    const result = buildProjects({
      hosts: [
        {
          serverId: "local",
          serverName: "Local",
          isOnline: true,
          workspaces: [
            workspace({
              id: "gitlab",
              repoRoot: "/repo/gitlab",
              projectId: "remote:gitlab.com/acme/app",
              projectName: "app",
              remoteUrl: "https://gitlab.com/acme/app.git",
            }),
          ],
        },
      ],
    });

    expect(result.projects).toEqual([]);
    expect(result.hiddenUnsupportedRemoteCount).toBe(1);
  });
});

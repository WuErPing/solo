import { describe, expect, it } from "vitest";
import {
  createConnectedClient,
  simulateServerResponse,
} from "./daemon-client-test-harness.js";

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

describe("GitRpc", () => {
  it("getCheckoutStatus sends checkout_status_request and resolves with status", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.getCheckoutStatus("/test/project", {
      requestId: "req-checkout-status",
    });

    const sent = findSentMessage(transport, "checkout_status_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");

    simulateServerResponse(transport, {
      type: "checkout_status_response" as const,
      payload: {
        cwd: "/test/project",
        isGit: false as const,
        isSoloOwnedWorktree: false as const,
        repoRoot: null,
        currentBranch: null,
        isDirty: null,
        baseRef: null,
        aheadBehind: null,
        aheadOfOrigin: null,
        behindOfOrigin: null,
        hasRemote: false,
        remoteUrl: null,
        error: null,
        requestId: "req-checkout-status",
      },
    });

    const result = await promise;
    expect(result.isGit).toBe(false);
    await cleanup();
  });

  it("checkoutCommit sends checkout_commit_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.checkoutCommit(
      "/test/project",
      { message: "test commit", addAll: true },
      "req-checkout-commit",
    );

    const sent = findSentMessage(transport, "checkout_commit_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { message?: string } }).message?.message).toBe(
      "test commit",
    );
    expect((sent!.parsed as { message?: { addAll?: boolean } }).message?.addAll).toBe(true);

    simulateServerResponse(transport, {
      type: "checkout_commit_response" as const,
      payload: {
        cwd: "/test/project",
        success: true,
        error: null,
        requestId: "req-checkout-commit",
      },
    });

    const result = await promise;
    expect(result.success).toBe(true);
    await cleanup();
  });

  it("stashList sends stash_list_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.stashList("/test/project", { soloOnly: true }, "req-stash-list");

    const sent = findSentMessage(transport, "stash_list_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { soloOnly?: boolean } }).message?.soloOnly).toBe(true);

    simulateServerResponse(transport, {
      type: "stash_list_response" as const,
      payload: {
        cwd: "/test/project",
        entries: [],
        error: null,
        requestId: "req-stash-list",
      },
    });

    const result = await promise;
    expect(result.entries).toHaveLength(0);
    await cleanup();
  });

  it("validateBranch sends validate_branch_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.validateBranch(
      { cwd: "/test/project", branchName: "feature-branch" },
      "req-validate-branch",
    );

    const sent = findSentMessage(transport, "validate_branch_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { branchName?: string } }).message?.branchName).toBe(
      "feature-branch",
    );

    simulateServerResponse(transport, {
      type: "validate_branch_response" as const,
      payload: {
        exists: false,
        resolvedRef: null,
        isRemote: false,
        error: null,
        requestId: "req-validate-branch",
      },
    });

    const result = await promise;
    expect(result.exists).toBe(false);
    await cleanup();
  });

  it("searchGitHub sends github_search_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.searchGitHub(
      { cwd: "/test/project", query: "test query" },
      "req-search-github",
    );

    const sent = findSentMessage(transport, "github_search_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { query?: string } }).message?.query).toBe("test query");

    simulateServerResponse(transport, {
      type: "github_search_response" as const,
      payload: {
        items: [],
        githubFeaturesEnabled: true,
        error: null,
        requestId: "req-search-github",
      },
    });

    const result = await promise;
    expect(result.items).toHaveLength(0);
    expect(result.githubFeaturesEnabled).toBe(true);
    await cleanup();
  });

  it("createSoloWorktree sends create_solo_worktree_request", async () => {
    const { client, transport, cleanup } = createConnectedClient();

    const promise = client.createSoloWorktree(
      { cwd: "/test/project", worktreeSlug: "feature-wt" },
      "req-create-worktree",
    );

    const sent = findSentMessage(transport, "create_solo_worktree_request");
    expect(sent).toBeDefined();
    expect((sent!.parsed as { message?: { cwd?: string } }).message?.cwd).toBe("/test/project");
    expect((sent!.parsed as { message?: { worktreeSlug?: string } }).message?.worktreeSlug).toBe(
      "feature-wt",
    );

    simulateServerResponse(transport, {
      type: "create_solo_worktree_response" as const,
      payload: {
        workspace: null,
        error: null,
        setupTerminalId: null,
        requestId: "req-create-worktree",
      },
    });

    const result = await promise;
    expect(result.workspace).toBeNull();
    expect(result.error).toBeNull();
    await cleanup();
  });
});

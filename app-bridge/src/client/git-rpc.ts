import { SessionInboundMessageSchema } from "../shared/messages.js";
import type {
  CheckoutStatusResponse,
  CheckoutCommitResponse,
  CheckoutMergeResponse,
  CheckoutMergeFromBaseResponse,
  CheckoutPullResponse,
  CheckoutPushResponse,
  CheckoutPrCreateResponse,
  CheckoutPrStatusResponse,
  PullRequestTimelineResponse,
  CheckoutSwitchBranchResponse,
  StashSaveResponse,
  StashPopResponse,
  StashListResponse,
  ValidateBranchResponse,
  BranchSuggestionsResponse,
  GitHubSearchResponse,
  GitHubSearchRequest,
  DirectorySuggestionsResponse,
  SoloWorktreeListResponse,
  SoloWorktreeArchiveResponse,
  SessionOutboundMessage,
} from "../shared/messages.js";
import type { DaemonClient, CreateSoloWorktreeInput } from "./daemon-client.js";

type CheckoutStatusPayload = CheckoutStatusResponse["payload"];
type SubscribeCheckoutDiffPayload = Extract<
  SessionOutboundMessage,
  { type: "subscribe_checkout_diff_response" }
>["payload"];
type CheckoutDiffPayload = Omit<SubscribeCheckoutDiffPayload, "subscriptionId">;
type CheckoutCommitPayload = CheckoutCommitResponse["payload"];
type CheckoutMergePayload = CheckoutMergeResponse["payload"];
type CheckoutMergeFromBasePayload = CheckoutMergeFromBaseResponse["payload"];
type CheckoutPullPayload = CheckoutPullResponse["payload"];
type CheckoutPushPayload = CheckoutPushResponse["payload"];
type CheckoutPrCreatePayload = CheckoutPrCreateResponse["payload"];
type CheckoutPrStatusPayload = CheckoutPrStatusResponse["payload"];
type PullRequestTimelinePayload = PullRequestTimelineResponse["payload"];
type CheckoutSwitchBranchPayload = CheckoutSwitchBranchResponse["payload"];
type StashSavePayload = StashSaveResponse["payload"];
type StashPopPayload = StashPopResponse["payload"];
type StashListPayload = StashListResponse["payload"];
type ValidateBranchPayload = ValidateBranchResponse["payload"];
type BranchSuggestionsPayload = BranchSuggestionsResponse["payload"];
type GitHubSearchPayload = GitHubSearchResponse["payload"];
type DirectorySuggestionsPayload = DirectorySuggestionsResponse["payload"];
type SoloWorktreeListPayload = SoloWorktreeListResponse["payload"];
type SoloWorktreeArchivePayload = SoloWorktreeArchiveResponse["payload"];
type CreateSoloWorktreePayload = Extract<
  SessionOutboundMessage,
  { type: "create_solo_worktree_response" }
>["payload"];

export class GitRpc {
  private checkoutStatusInFlight: Map<string, Promise<CheckoutStatusPayload>> = new Map();
  private checkoutDiffSubscriptions = new Map<
    string,
    {
      cwd: string;
      compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean };
    }
  >();

  constructor(private readonly client: DaemonClient) {}

  // ============================================================================
  // Checkout Status
  // ============================================================================

  async getCheckoutStatus(
    cwd: string,
    options?: { requestId?: string },
  ): Promise<CheckoutStatusPayload> {
    const requestId = options?.requestId;

    if (!requestId) {
      const existing = this.checkoutStatusInFlight.get(cwd);
      if (existing) {
        return existing;
      }
    }

    const resolvedRequestId = this.client.createRequestId(requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "checkout_status_request",
      cwd,
      requestId: resolvedRequestId,
    });

    const responsePromise = this.client.sendRequest({
      requestId: resolvedRequestId,
      message,
      timeout: 60000,
      options: { skipQueue: true },
      select: (msg) => {
        if (msg.type !== "checkout_status_response") {
          return null;
        }
        if (msg.payload.requestId !== resolvedRequestId) {
          return null;
        }
        return msg.payload;
      },
    });

    if (!requestId) {
      this.checkoutStatusInFlight.set(cwd, responsePromise);
      void responsePromise
        .finally(() => {
          if (this.checkoutStatusInFlight.get(cwd) === responsePromise) {
            this.checkoutStatusInFlight.delete(cwd);
          }
        })
        .catch(() => undefined);
    }

    return responsePromise;
  }

  // ============================================================================
  // Checkout Diff
  // ============================================================================

  private normalizeCheckoutDiffCompare(compare: {
    mode: "uncommitted" | "base";
    baseRef?: string;
    ignoreWhitespace?: boolean;
  }): { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean } {
    if (compare.mode === "uncommitted") {
      return compare.ignoreWhitespace === true
        ? { mode: "uncommitted", ignoreWhitespace: true }
        : { mode: "uncommitted" };
    }
    const trimmedBaseRef = compare.baseRef?.trim();
    if (!trimmedBaseRef) {
      return compare.ignoreWhitespace === true
        ? { mode: "base", ignoreWhitespace: true }
        : { mode: "base" };
    }
    return compare.ignoreWhitespace === true
      ? { mode: "base", baseRef: trimmedBaseRef, ignoreWhitespace: true }
      : { mode: "base", baseRef: trimmedBaseRef };
  }

  async getCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    requestId?: string,
  ): Promise<CheckoutDiffPayload> {
    const oneShotSubscriptionId = `oneshot-checkout-diff:${crypto.randomUUID()}`;
    try {
      const payload = await this.subscribeCheckoutDiff(cwd, compare, {
        subscriptionId: oneShotSubscriptionId,
        requestId,
      });
      return {
        cwd: payload.cwd,
        files: payload.files,
        error: payload.error,
        requestId: payload.requestId,
      };
    } finally {
      try {
        this.unsubscribeCheckoutDiff(oneShotSubscriptionId);
      } catch {
        // Ignore disconnect races during one-shot cleanup.
      }
    }
  }

  async subscribeCheckoutDiff(
    cwd: string,
    compare: { mode: "uncommitted" | "base"; baseRef?: string; ignoreWhitespace?: boolean },
    options?: { subscriptionId?: string; requestId?: string },
  ): Promise<SubscribeCheckoutDiffPayload> {
    const subscriptionId = options?.subscriptionId ?? crypto.randomUUID();
    const normalizedCompare = this.normalizeCheckoutDiffCompare(compare);
    const previousSubscription = this.checkoutDiffSubscriptions.get(subscriptionId) ?? null;
    this.checkoutDiffSubscriptions.set(subscriptionId, {
      cwd,
      compare: normalizedCompare,
    });

    const resolvedRequestId = this.client.createRequestId(options?.requestId);
    const message = SessionInboundMessageSchema.parse({
      type: "subscribe_checkout_diff_request",
      subscriptionId,
      cwd,
      compare: normalizedCompare,
      requestId: resolvedRequestId,
    });

    try {
      return await this.client.sendCorrelatedRequest({
        requestId: resolvedRequestId,
        message,
        responseType: "subscribe_checkout_diff_response",
        timeout: 60000,
        options: { skipQueue: true },
        selectPayload: (payload) => {
          if (payload.subscriptionId !== subscriptionId) {
            return null;
          }
          return payload;
        },
      });
    } catch (error) {
      if (previousSubscription) {
        this.checkoutDiffSubscriptions.set(subscriptionId, previousSubscription);
      } else {
        this.checkoutDiffSubscriptions.delete(subscriptionId);
      }
      throw error;
    }
  }

  unsubscribeCheckoutDiff(subscriptionId: string): void {
    this.checkoutDiffSubscriptions.delete(subscriptionId);
    this.client.sendSessionMessage({
      type: "unsubscribe_checkout_diff_request",
      subscriptionId,
    });
  }

  // ============================================================================
  // Checkout Operations
  // ============================================================================

  async checkoutCommit(
    cwd: string,
    input: { message?: string; addAll?: boolean },
    requestId?: string,
  ): Promise<CheckoutCommitPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_commit_request",
        cwd,
        message: input.message,
        addAll: input.addAll,
      },
      responseType: "checkout_commit_response",
      timeout: 60000,
    });
  }

  async checkoutMerge(
    cwd: string,
    input: { baseRef?: string; strategy?: "merge" | "squash"; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_merge_request",
        cwd,
        baseRef: input.baseRef,
        strategy: input.strategy,
        requireCleanTarget: input.requireCleanTarget,
      },
      responseType: "checkout_merge_response",
      timeout: 60000,
    });
  }

  async checkoutMergeFromBase(
    cwd: string,
    input: { baseRef?: string; requireCleanTarget?: boolean },
    requestId?: string,
  ): Promise<CheckoutMergeFromBasePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_merge_from_base_request",
        cwd,
        baseRef: input.baseRef,
        requireCleanTarget: input.requireCleanTarget,
      },
      responseType: "checkout_merge_from_base_response",
      timeout: 60000,
    });
  }

  async checkoutPull(cwd: string, requestId?: string): Promise<CheckoutPullPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_pull_request",
        cwd,
      },
      responseType: "checkout_pull_response",
      timeout: 60000,
    });
  }

  async checkoutPush(cwd: string, requestId?: string): Promise<CheckoutPushPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_push_request",
        cwd,
      },
      responseType: "checkout_push_response",
      timeout: 60000,
    });
  }

  async checkoutPrCreate(
    cwd: string,
    input: { title?: string; body?: string; baseRef?: string },
    requestId?: string,
  ): Promise<CheckoutPrCreatePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_pr_create_request",
        cwd,
        title: input.title,
        body: input.body,
        baseRef: input.baseRef,
      },
      responseType: "checkout_pr_create_response",
      timeout: 60000,
    });
  }

  async checkoutPrStatus(cwd: string, requestId?: string): Promise<CheckoutPrStatusPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_pr_status_request",
        cwd,
      },
      responseType: "checkout_pr_status_response",
      timeout: 60000,
    });
  }

  async pullRequestTimeline(
    input: { cwd: string; prNumber: number; repoOwner: string; repoName: string },
    requestId?: string,
  ): Promise<PullRequestTimelinePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "pull_request_timeline_request",
        cwd: input.cwd,
        prNumber: input.prNumber,
        repoOwner: input.repoOwner,
        repoName: input.repoName,
      },
      responseType: "pull_request_timeline_response",
      timeout: 60000,
    });
  }

  async checkoutSwitchBranch(
    cwd: string,
    branch: string,
    requestId?: string,
  ): Promise<CheckoutSwitchBranchPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "checkout_switch_branch_request",
        cwd,
        branch,
      },
      responseType: "checkout_switch_branch_response",
      timeout: 30000,
    });
  }

  // ============================================================================
  // Stash
  // ============================================================================

  async stashSave(
    cwd: string,
    options?: { branch?: string },
    requestId?: string,
  ): Promise<StashSavePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "stash_save_request",
        cwd,
        branch: options?.branch,
      },
      responseType: "stash_save_response",
      timeout: 30000,
    });
  }

  async stashPop(cwd: string, stashIndex: number, requestId?: string): Promise<StashPopPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "stash_pop_request",
        cwd,
        stashIndex,
      },
      responseType: "stash_pop_response",
      timeout: 30000,
    });
  }

  async stashList(
    cwd: string,
    options?: { soloOnly?: boolean },
    requestId?: string,
  ): Promise<StashListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "stash_list_request",
        cwd,
        soloOnly: options?.soloOnly,
      },
      responseType: "stash_list_response",
      timeout: 10000,
    });
  }

  // ============================================================================
  // Solo Worktree
  // ============================================================================

  async getSoloWorktreeList(
    input: { cwd?: string; repoRoot?: string },
    requestId?: string,
  ): Promise<SoloWorktreeListPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "solo_worktree_list_request",
        cwd: input.cwd,
        repoRoot: input.repoRoot,
      },
      responseType: "solo_worktree_list_response",
      timeout: 60000,
    });
  }

  async archiveSoloWorktree(
    input: { worktreePath?: string; repoRoot?: string; branchName?: string },
    requestId?: string,
  ): Promise<SoloWorktreeArchivePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "solo_worktree_archive_request",
        worktreePath: input.worktreePath,
        repoRoot: input.repoRoot,
        branchName: input.branchName,
      },
      responseType: "solo_worktree_archive_response",
      timeout: 20000,
    });
  }

  async createSoloWorktree(
    input: CreateSoloWorktreeInput,
    requestId?: string,
  ): Promise<CreateSoloWorktreePayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "create_solo_worktree_request",
        cwd: input.cwd,
        worktreeSlug: input.worktreeSlug,
        ...(input.attachments && input.attachments.length > 0
          ? { attachments: input.attachments }
          : {}),
        ...(input.refName !== undefined ? { refName: input.refName } : {}),
        ...(input.action !== undefined ? { action: input.action } : {}),
        ...(input.githubPrNumber !== undefined ? { githubPrNumber: input.githubPrNumber } : {}),
      },
      responseType: "create_solo_worktree_response",
      timeout: 60000,
    });
  }

  // ============================================================================
  // Branch / GitHub / Directory Suggestions
  // ============================================================================

  async validateBranch(
    options: { cwd: string; branchName: string },
    requestId?: string,
  ): Promise<ValidateBranchPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "validate_branch_request",
        cwd: options.cwd,
        branchName: options.branchName,
      },
      responseType: "validate_branch_response",
      timeout: 10000,
    });
  }

  async getBranchSuggestions(
    options: { cwd: string; query?: string; limit?: number },
    requestId?: string,
  ): Promise<BranchSuggestionsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "branch_suggestions_request",
        cwd: options.cwd,
        query: options.query,
        limit: options.limit,
      },
      responseType: "branch_suggestions_response",
      timeout: 10000,
    });
  }

  async searchGitHub(
    options: { cwd: string; query: string; limit?: number; kinds?: GitHubSearchRequest["kinds"] },
    requestId?: string,
  ): Promise<GitHubSearchPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "github_search_request",
        cwd: options.cwd,
        query: options.query,
        limit: options.limit,
        kinds: options.kinds,
      },
      responseType: "github_search_response",
      timeout: 15000,
    });
  }

  async getDirectorySuggestions(
    options: {
      query: string;
      limit?: number;
      cwd?: string;
      includeFiles?: boolean;
      includeDirectories?: boolean;
    },
    requestId?: string,
  ): Promise<DirectorySuggestionsPayload> {
    return this.client.sendCorrelatedSessionRequest({
      requestId,
      message: {
        type: "directory_suggestions_request",
        query: options.query,
        cwd: options.cwd,
        includeFiles: options.includeFiles,
        includeDirectories: options.includeDirectories,
        limit: options.limit,
      },
      responseType: "directory_suggestions_response",
      timeout: 10000,
    });
  }

  // ============================================================================
  // Resubscribe
  // ============================================================================

  resubscribe(): void {
    if (this.checkoutDiffSubscriptions.size === 0) {
      return;
    }
    for (const [subscriptionId, subscription] of this.checkoutDiffSubscriptions) {
      const message = SessionInboundMessageSchema.parse({
        type: "subscribe_checkout_diff_request",
        subscriptionId,
        cwd: subscription.cwd,
        compare: subscription.compare,
        requestId: this.client.createRequestId(),
      });
      this.client.sendSessionMessage(message);
    }
  }
}

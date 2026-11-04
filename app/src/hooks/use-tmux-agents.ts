import { keepPreviousData, useQueries, useQueryClient } from "@tanstack/react-query";
import { useMemo, useCallback, useRef } from "react";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";
import { useAppVisible } from "@/hooks/use-app-visible";

export interface TmuxAgent {
  sessionName: string;
  windowName: string;
  paneId: string;
  paneIndex: number;
  panePid: number;
  agentName: string;
  currentCmd: string;
  workingDir: string;
  title?: string;
  status?: string;
  activity?: string;
  lastContentChange?: number;
  lastContentChangeHHMM?: string;
  lastContentChangeAgo?: string;
  serverId: string;
  serverLabel: string;
}

export interface TmuxPane {
  sessionName: string;
  windowName: string;
  paneId: string;
  paneIndex: number;
  panePid: number;
  currentCmd: string;
  workingDir: string;
  title?: string;
  lastContentChange?: number;
  lastContentChangeHHMM?: string;
  lastContentChangeAgo?: string;
  serverId: string;
  serverLabel: string;
}

export type TmuxAgentOrPane = TmuxAgent | TmuxPane;

export interface AgentCommandEntry {
  agentName: string;
  launchCmd: string;
  lastSeen: string;
  serverId: string;
  serverLabel: string;
}

export interface TmuxInputEntry {
  text: string;
  count: number;
  lastUsed: string;
  serverId: string;
  serverLabel: string;
}

export interface AggregatedTmuxAgentsResult {
  agents: TmuxAgent[];
  otherPanes: TmuxPane[];
  commandHistory: AgentCommandEntry[];
  inputHistory: TmuxInputEntry[];
  isLoading: boolean;
  isInitialLoad: boolean;
  isRevalidating: boolean;
  error: string | null;
  refreshAll: () => void;
}

export function tmuxAgentsQueryKey(serverId: string): readonly string[] {
  return ["tmux-agents", serverId];
}

// list_agents runs a full tmux + ps scan per request, so it polls far slower
// than capture_pane: 1s while pane state changed recently, 5s once stable.
const ACTIVE_LIST_INTERVAL = 1000;
const IDLE_LIST_INTERVAL = 5000;
const ACTIVE_LIST_PHASE_MS = 10_000;

interface TmuxAgentsQueryData {
  agents?: Partial<TmuxAgent>[];
  otherPanes?: Partial<TmuxPane>[];
  commandHistory?: Partial<AgentCommandEntry>[];
  inputHistory?: Partial<TmuxInputEntry>[];
  error?: string | null;
}

// The daemon rewrites lastSeen and the "N ago" strings on every scan, so data
// identity can't tell real changes apart; fingerprint only the fields that
// describe pane/agent state.
function tmuxAgentsChangeFingerprint(data: TmuxAgentsQueryData | undefined): string {
  if (!data) return "";
  const panes = [...(data.agents ?? []), ...(data.otherPanes ?? [])].map((p) => [
    p.sessionName,
    p.windowName,
    p.paneId,
    p.panePid,
    p.currentCmd,
    p.workingDir,
    p.title,
    p.lastContentChange,
    "agentName" in p ? p.agentName : undefined,
    "status" in p ? p.status : undefined,
    "activity" in p ? p.activity : undefined,
  ]);
  const commands = (data.commandHistory ?? []).map((c) => [c.agentName, c.launchCmd]);
  const inputs = (data.inputHistory ?? []).map((i) => [i.text, i.count]);
  return JSON.stringify([panes, commands, inputs, data.error ?? null]);
}

const lastChangeByQuery = new WeakMap<object, { fingerprint: string; changedAt: number }>();

export function computeTmuxAgentsRefetchInterval(
  query: { state: { data: unknown } },
  now: number,
): number {
  const fingerprint = tmuxAgentsChangeFingerprint(query.state.data as TmuxAgentsQueryData | undefined);
  let entry = lastChangeByQuery.get(query);
  if (!entry || entry.fingerprint !== fingerprint) {
    entry = { fingerprint, changedAt: now };
    lastChangeByQuery.set(query, entry);
  }
  return now - entry.changedAt <= ACTIVE_LIST_PHASE_MS ? ACTIVE_LIST_INTERVAL : IDLE_LIST_INTERVAL;
}

export function useAggregatedTmuxAgents(options?: {
  enabled?: boolean;
}): AggregatedTmuxAgentsResult {
  const enabled = options?.enabled ?? true;
  const hosts = useHosts();
  const queryClient = useQueryClient();
  const hasFetched = useRef(false);
  const isAppVisible = useAppVisible();

  const queries = useQueries({
    queries: hosts.map((host) => {
      const store = getHostRuntimeStore();
      const client = store.getClient(host.serverId);
      const snapshot = store.getSnapshot(host.serverId);
      const isConnected = isHostRuntimeConnected(snapshot);

      return {
        queryKey: tmuxAgentsQueryKey(host.serverId),
        enabled: enabled && Boolean(client && isConnected),
        placeholderData: keepPreviousData,
        refetchInterval: isAppVisible
          ? (query: { state: { data: unknown } }) =>
              computeTmuxAgentsRefetchInterval(query, Date.now())
          : false,
        staleTime: ACTIVE_LIST_INTERVAL,
        retry: 1,
        queryFn: async () => {
          const payload = await withLiveTmuxClient(host.serverId, (c) => c.tmuxListAgents());
          return {
            agents: payload.agents ?? [],
            otherPanes: payload.otherPanes ?? [],
            commandHistory: payload.commandHistory ?? [],
            inputHistory: payload.inputHistory ?? [],
            error: payload.error ?? null,
            serverId: host.serverId,
            serverLabel: host.label,
          };
        },
      };
    }),
  });

  const result = useMemo(() => {
    const allAgents: TmuxAgent[] = [];
    const allOtherPanes: TmuxPane[] = [];
    const allCommandHistory: AgentCommandEntry[] = [];
    const inputByText = new Map<string, TmuxInputEntry>();
    let anyError: string | null = null;
    let isLoading = false;
    let isFetching = false;

    for (let i = 0; i < queries.length; i++) {
      const query = queries[i];
      if (!query) continue;

      const host = hosts[i];
      if (!host) continue;

      if (query.isLoading) {
        isLoading = true;
      }
      if (query.isFetching) {
        isFetching = true;
      }
      if (query.data?.error && !anyError) {
        anyError = query.data.error;
      }
      if (
        !anyError &&
        query.error instanceof Error &&
        query.error.message !== "Daemon client not available"
      ) {
        anyError = query.error.message;
      }
      if (query.data?.agents) {
        for (const agent of query.data.agents) {
          allAgents.push({
            ...agent,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
      if (query.data?.otherPanes) {
        for (const pane of query.data.otherPanes) {
          allOtherPanes.push({
            ...pane,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
      if (query.data?.commandHistory) {
        for (const entry of query.data.commandHistory) {
          allCommandHistory.push({
            ...entry,
            serverId: host.serverId,
            serverLabel: host.label,
          });
        }
      }
      if (query.data?.inputHistory) {
        for (const entry of query.data.inputHistory) {
          // Merge identical inputs across hosts: sum counts, keep the most
          // recent lastUsed.
          const existing = inputByText.get(entry.text);
          if (existing) {
            existing.count += entry.count;
            if (entry.lastUsed > existing.lastUsed) {
              existing.lastUsed = entry.lastUsed;
              existing.serverId = host.serverId;
              existing.serverLabel = host.label;
            }
          } else {
            inputByText.set(entry.text, {
              ...entry,
              serverId: host.serverId,
              serverLabel: host.label,
            });
          }
        }
      }
    }

    // Sort by most recent session activity descending, fall back to agentName
    allAgents.sort((left, right) => {
      const leftActivity = left.lastContentChange ?? 0;
      const rightActivity = right.lastContentChange ?? 0;
      if (leftActivity !== rightActivity) return rightActivity - leftActivity;
      return left.agentName.localeCompare(right.agentName);
    });

    // Sort other panes by most recent session activity descending, fall back to sessionName
    allOtherPanes.sort((left, right) => {
      const leftActivity = left.lastContentChange ?? 0;
      const rightActivity = right.lastContentChange ?? 0;
      if (leftActivity !== rightActivity) return rightActivity - leftActivity;
      return left.sessionName.localeCompare(right.sessionName);
    });

    // Sort command history by lastSeen descending.
    allCommandHistory.sort((a, b) => b.lastSeen.localeCompare(a.lastSeen));

    // Sort input history by usage count descending, then most recent use.
    const allInputHistory = Array.from(inputByText.values());
    allInputHistory.sort((a, b) => {
      if (a.count !== b.count) return b.count - a.count;
      return b.lastUsed.localeCompare(a.lastUsed);
    });

    const hasAnyData = allAgents.length > 0 || allOtherPanes.length > 0;

    // Track whether any query has completed (success or error).
    // Once true, stays true — we only need to know the first fetch happened.
    /* eslint-disable react-hooks/refs -- one-time latch inside useMemo; converting to state would cause unnecessary re-renders */
    if (!hasFetched.current) {
      for (const query of queries) {
        if (query && (query.isSuccess || query.isError)) {
          hasFetched.current = true;
          break;
        }
      }
    }

    const isInitialLoad = !hasAnyData && !hasFetched.current;
    const isRevalidating = isFetching && !isLoading && hasAnyData;

    return {
      agents: allAgents,
      otherPanes: allOtherPanes,
      commandHistory: allCommandHistory,
      inputHistory: allInputHistory,
      isLoading,
      isInitialLoad,
      isRevalidating,
      error: anyError,
    };
    /* eslint-enable react-hooks/refs */
  }, [queries, hosts]);

  const refreshAll = useCallback(() => {
    for (const host of hosts) {
      void queryClient.invalidateQueries({ queryKey: tmuxAgentsQueryKey(host.serverId) });
    }
  }, [hosts, queryClient]);

  return {
    ...result,
    refreshAll,
  };
}

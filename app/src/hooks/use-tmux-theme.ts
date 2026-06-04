import { useQuery } from "@tanstack/react-query";
import { useMemo } from "react";
import { getHostRuntimeStore, isHostRuntimeConnected } from "@/runtime/host-runtime";

export interface TmuxThemeColors {
  background: string;
  foreground: string;
  paneActiveBorder?: string;
  paneInactiveBorder?: string;
  statusBackground?: string;
  statusForeground?: string;
  messageBackground?: string;
  messageForeground?: string;
  windowStatusCurrentBg?: string;
  windowStatusCurrentFg?: string;
}

export interface TmuxThemeResult {
  theme: TmuxThemeColors | null;
  isLoading: boolean;
  error: string | null;
}

export function tmuxThemeQueryKey(
  serverId: string,
  sessionId: string,
): readonly string[] {
  return ["tmux-theme", serverId, sessionId];
}

export function useTmuxTheme(
  serverId: string,
  sessionId: string,
  enabled: boolean,
): TmuxThemeResult {
  const store = getHostRuntimeStore();
  const client = store.getClient(serverId);
  const snapshot = store.getSnapshot(serverId);
  const isConnected = isHostRuntimeConnected(snapshot);

  const { data, isLoading, error } = useQuery({
    queryKey: tmuxThemeQueryKey(serverId, sessionId),
    enabled: enabled && Boolean(client) && isConnected && sessionId.length > 0,
    staleTime: 30_000,
    retry: 1,
    queryFn: async () => {
      const liveClient = store.getClient(serverId);
      if (!liveClient || liveClient.getConnectionState().status === "disposed") {
        throw new Error("Daemon client not available");
      }
      const payload = await liveClient.tmuxGetTheme(sessionId);
      return {
        theme: payload.theme,
        error: payload.error ?? null,
      };
    },
  });

  return useMemo(
    () => ({
      theme: data?.theme ?? null,
      isLoading,
      error:
        data?.error ??
        (error instanceof Error && error.message !== "Daemon client not available"
          ? error.message
          : null),
    }),
    [data, isLoading, error],
  );
}

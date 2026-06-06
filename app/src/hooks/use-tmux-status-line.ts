import { useQuery } from "@tanstack/react-query";
import { getHostRuntimeStore, useHosts, isHostRuntimeConnected } from "@/runtime/host-runtime";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export interface TmuxStatusLine {
  statusLeft: string;
  statusCenter: string;
  statusRight: string;
  paneBackground: string;
  paneForeground: string;
  serverId: string;
}

export function tmuxStatusLineQueryKey(serverId: string, sessionId: string): readonly string[] {
  return ["tmux-status-line", serverId, sessionId];
}

export function useTmuxStatusLine(sessionId: string) {
  const hosts = useHosts();

  return useQuery({
    queryKey: tmuxStatusLineQueryKey(hosts[0]?.serverId ?? "", sessionId),
    enabled: Boolean(sessionId) && hosts.length > 0,
    staleTime: 5000,
    queryFn: async (): Promise<TmuxStatusLine> => {
      const host = hosts[0]!;
      const payload = await withLiveTmuxClient(host.serverId, (c) =>
        c.tmuxStatusLine(sessionId),
      );
      return {
        statusLeft: payload.statusLeft ?? "",
        statusCenter: payload.statusCenter ?? "",
        statusRight: payload.statusRight ?? "",
        paneBackground: payload.paneBackground ?? "",
        paneForeground: payload.paneForeground ?? "",
        serverId: host.serverId,
      };
    },
  });
}

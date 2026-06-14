import { useState, useCallback } from "react";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export function useTmuxKillSession() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const killSession = useCallback(async (serverId: string, sessionName: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await withLiveTmuxClient(serverId, (client) =>
        client.tmuxKillSession(sessionName),
      );
      if (result.error) {
        setError(result.error);
        return false;
      }
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to kill tmux session";
      setError(message);
      return false;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { killSession, isLoading, error };
}

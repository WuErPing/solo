import { useState, useCallback } from "react";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export function useTmuxDeleteCommandHistory() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const deleteCommand = useCallback(async (serverId: string, launchCmd: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await withLiveTmuxClient(serverId, (client) =>
        client.tmuxDeleteCommandHistory(launchCmd),
      );
      if (result.error) {
        setError(result.error);
        return false;
      }
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete command history";
      setError(message);
      return false;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { deleteCommand, isLoading, error };
}

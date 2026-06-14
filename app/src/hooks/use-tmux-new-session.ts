import { useState, useCallback } from "react";
import { withLiveTmuxClient } from "@/utils/tmux-rpc";

export interface TmuxNewSessionOptions {
  name: string;
  workingDir?: string;
  command?: string;
}

export function useTmuxNewSession() {
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const createSession = useCallback(async (serverId: string, options: TmuxNewSessionOptions) => {
    setIsLoading(true);
    setError(null);
    try {
      const result = await withLiveTmuxClient(serverId, (client) =>
        client.tmuxNewSession(options.name, {
          workingDir: options.workingDir,
          command: options.command,
        }),
      );
      if (result.error) {
        setError(result.error);
        return null;
      }
      return result.sessionName;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to create tmux session";
      setError(message);
      return null;
    } finally {
      setIsLoading(false);
    }
  }, []);

  return { createSession, isLoading, error };
}

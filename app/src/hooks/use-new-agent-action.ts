import { useCallback } from "react";
import { useNavigationActiveWorkspaceSelection } from "@/stores/navigation-active-workspace-store";
import { prepareWorkspaceTab } from "@/utils/workspace-navigation";
import { useOpenProjectPicker } from "./use-open-project-picker";

export function useNewAgentAction(serverId: string | null): () => Promise<void> {
  const normalizedServerId = serverId?.trim() ?? "";
  const activeWorkspace = useNavigationActiveWorkspaceSelection();
  const openProjectPicker = useOpenProjectPicker(normalizedServerId);

  return useCallback(async () => {
    if (activeWorkspace?.serverId === normalizedServerId) {
      prepareWorkspaceTab({
        serverId: activeWorkspace.serverId,
        workspaceId: activeWorkspace.workspaceId,
        target: { kind: "draft", draftId: "new" },
      });
      return;
    }

    await openProjectPicker();
  }, [activeWorkspace, normalizedServerId, openProjectPicker]);
}

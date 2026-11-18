import { useCallback, useState } from "react";
import { Text, View } from "react-native";
import { FileText } from "lucide-react-native";
import invariant from "tiny-invariant";
import { FilePane } from "@/components/file-pane";
import { NewTmuxAgentPicker } from "@/components/new-tmux-agent-picker";
import { usePaneContext } from "@/panels/pane-context";
import type { PanelRegistration } from "@/panels/panel-registry";
import { useWorkspaceExecutionAuthority } from "@/stores/session-store-hooks";

const CENTERED_PADDED_STYLE = {
  flex: 1,
  alignItems: "center",
  justifyContent: "center",
  padding: 16,
} as const;

function useFilePanelDescriptor(target: { kind: "file"; path: string }) {
  const fileName = target.path.split("/").findLast(Boolean) ?? target.path;
  return {
    label: fileName,
    subtitle: target.path,
    titleState: "ready" as const,
    icon: FileText,
    statusBucket: null,
  };
}

function FilePanel() {
  const { serverId, workspaceId, target, openDraftForFile } = usePaneContext();
  const workspaceAuthority = useWorkspaceExecutionAuthority(serverId, workspaceId);
  const workspaceDirectory = workspaceAuthority?.ok
    ? workspaceAuthority.authority.workspaceDirectory
    : null;
  // Picker state lives here, outside the memoized markdown tree, so opening it
  // never re-parses preview blocks.
  const [tmuxPickerFilePath, setTmuxPickerFilePath] = useState<string | null>(null);
  const handleNewTmux = useCallback((filePath: string) => {
    setTmuxPickerFilePath(filePath);
  }, []);
  const handleCloseTmuxPicker = useCallback(() => {
    setTmuxPickerFilePath(null);
  }, []);
  invariant(target.kind === "file", "FilePanel requires file target");
  if (!workspaceDirectory) {
    return (
      <View style={CENTERED_PADDED_STYLE}>
        <Text>Workspace execution directory not found.</Text>
      </View>
    );
  }
  return (
    <>
      <FilePane
        serverId={serverId}
        workspaceRoot={workspaceDirectory}
        filePath={target.path}
        onNewAgent={openDraftForFile}
        onNewTmux={handleNewTmux}
      />
      {tmuxPickerFilePath ? (
        <NewTmuxAgentPicker
          serverId={serverId}
          filePath={tmuxPickerFilePath}
          workspaceRoot={workspaceDirectory}
          visible
          onClose={handleCloseTmuxPicker}
        />
      ) : null}
    </>
  );
}

export const filePanelRegistration: PanelRegistration<"file"> = {
  kind: "file",
  component: FilePanel,
  useDescriptor: useFilePanelDescriptor,
};

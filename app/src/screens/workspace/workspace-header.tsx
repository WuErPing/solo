import { useCallback, type ReactElement } from "react";
import { Text, View } from "react-native";
import { BranchSwitcher } from "@/components/branch-switcher";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { isAbsolutePath } from "@/utils/path";
import {
  ThemedEllipsis,
  ThemedEllipsisVertical,
  foregroundColorMapping,
  mutedColorMapping,
} from "@/screens/workspace/workspace-themed-icons";
import { styles } from "@/screens/workspace/workspace-screen-styles";

interface WorkspaceHeaderMenuProps {
  normalizedWorkspaceId: string;
  currentBranchName: string | null;
  showWorkspaceSetup: boolean;
  isMobile: boolean;
  createTerminalDisabled: boolean;
  menuNewAgentIcon: ReactElement;
  menuNewTerminalIcon: ReactElement;
  menuCopyIcon: ReactElement;
  menuSettingsIcon: ReactElement;
  onCreateDraftTab: () => void;
  onCreateTerminal: () => void;
  onCopyWorkspacePath: () => void;
  onCopyBranchName: () => void;
  onOpenSetupTab: () => void;
}

function WorkspaceHeaderMenuTriggerIcon({
  hovered,
  open,
  isMobile,
}: {
  hovered: boolean;
  open: boolean;
  isMobile: boolean;
}) {
  const Icon = isMobile ? ThemedEllipsisVertical : ThemedEllipsis;
  const colorMapping = hovered || open ? foregroundColorMapping : mutedColorMapping;
  return <Icon size={16} uniProps={colorMapping} />;
}

export function WorkspaceHeaderMenu({
  normalizedWorkspaceId,
  currentBranchName,
  showWorkspaceSetup,
  isMobile,
  createTerminalDisabled,
  menuNewAgentIcon,
  menuNewTerminalIcon,
  menuCopyIcon,
  menuSettingsIcon,
  onCreateDraftTab,
  onCreateTerminal,
  onCopyWorkspacePath,
  onCopyBranchName,
  onOpenSetupTab,
}: WorkspaceHeaderMenuProps) {
  const renderTriggerIcon = useCallback(
    ({ hovered, open }: { hovered: boolean; open: boolean }) => (
      <WorkspaceHeaderMenuTriggerIcon hovered={hovered} open={open} isMobile={isMobile} />
    ),
    [isMobile],
  );

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        testID="workspace-header-menu-trigger"
        style={styles.headerActionButton}
        accessibilityRole="button"
        accessibilityLabel="Workspace actions"
      >
        {renderTriggerIcon}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" width={220} testID="workspace-header-menu">
        <DropdownMenuItem
          testID="workspace-header-new-agent"
          leading={menuNewAgentIcon}
          onSelect={onCreateDraftTab}
        >
          New agent
        </DropdownMenuItem>
        <DropdownMenuItem
          testID="workspace-header-new-terminal"
          leading={menuNewTerminalIcon}
          disabled={createTerminalDisabled}
          onSelect={onCreateTerminal}
        >
          New terminal
        </DropdownMenuItem>
        <DropdownMenuItem
          testID="workspace-header-copy-path"
          leading={menuCopyIcon}
          disabled={!isAbsolutePath(normalizedWorkspaceId)}
          onSelect={onCopyWorkspacePath}
        >
          Copy workspace path
        </DropdownMenuItem>
        {currentBranchName ? (
          <DropdownMenuItem
            testID="workspace-header-copy-branch-name"
            leading={menuCopyIcon}
            onSelect={onCopyBranchName}
          >
            Copy branch name
          </DropdownMenuItem>
        ) : null}
        {showWorkspaceSetup ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              testID="workspace-header-show-setup"
              leading={menuSettingsIcon}
              onSelect={onOpenSetupTab}
            >
              Show setup
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

interface WorkspaceHeaderTitleBarProps {
  isLoading: boolean;
  title: string;
  subtitle: string;
  showSubtitle: boolean;
  currentBranchName: string | null;
  isGitCheckout: boolean;
  normalizedServerId: string;
  normalizedWorkspaceId: string;
  showWorkspaceSetup: boolean;
  isMobile: boolean;
  createTerminalDisabled: boolean;
  menuNewAgentIcon: ReactElement;
  menuNewTerminalIcon: ReactElement;
  menuCopyIcon: ReactElement;
  menuSettingsIcon: ReactElement;
  onCreateDraftTab: () => void;
  onCreateTerminal: () => void;
  onCopyWorkspacePath: () => void;
  onCopyBranchName: () => void;
  onOpenSetupTab: () => void;
}

export function WorkspaceHeaderTitleBar({
  isLoading,
  title,
  subtitle,
  showSubtitle,
  currentBranchName,
  isGitCheckout,
  normalizedServerId,
  normalizedWorkspaceId,
  showWorkspaceSetup,
  isMobile,
  createTerminalDisabled,
  menuNewAgentIcon,
  menuNewTerminalIcon,
  menuCopyIcon,
  menuSettingsIcon,
  onCreateDraftTab,
  onCreateTerminal,
  onCopyWorkspacePath,
  onCopyBranchName,
  onOpenSetupTab,
}: WorkspaceHeaderTitleBarProps) {
  return (
    <View style={styles.headerTitleContainer}>
      {isLoading ? (
        <View style={styles.headerTitleTextGroup}>
          <View style={styles.headerTitleSkeleton} />
        </View>
      ) : (
        <View style={styles.headerTitleTextGroup}>
          <BranchSwitcher
            currentBranchName={currentBranchName}
            title={title}
            serverId={normalizedServerId}
            workspaceId={normalizedWorkspaceId}
            isGitCheckout={isGitCheckout}
          />
          {showSubtitle ? (
            <Text
              testID="workspace-header-subtitle"
              style={styles.headerProjectTitle}
              numberOfLines={1}
            >
              {subtitle}
            </Text>
          ) : null}
        </View>
      )}
      <WorkspaceHeaderMenu
        normalizedWorkspaceId={normalizedWorkspaceId}
        currentBranchName={currentBranchName}
        showWorkspaceSetup={showWorkspaceSetup}
        isMobile={isMobile}
        createTerminalDisabled={createTerminalDisabled}
        menuNewAgentIcon={menuNewAgentIcon}
        menuNewTerminalIcon={menuNewTerminalIcon}
        menuCopyIcon={menuCopyIcon}
        menuSettingsIcon={menuSettingsIcon}
        onCreateDraftTab={onCreateDraftTab}
        onCreateTerminal={onCreateTerminal}
        onCopyWorkspacePath={onCopyWorkspacePath}
        onCopyBranchName={onCopyBranchName}
        onOpenSetupTab={onOpenSetupTab}
      />
    </View>
  );
}

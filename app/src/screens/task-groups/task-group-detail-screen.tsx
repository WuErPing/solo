/**
 * Task Group detail screen (multi-agent collaboration prototype).
 *
 * Wide layouts get a three-pane view (members pipeline | mock conversation |
 * shared context + changes + activity). Compact layouts collapse the same
 * content into Members / Chat / Output tabs with Chat selected by default.
 * All data comes from local mock fixtures — no backend wiring.
 */
import { useCallback, useMemo, useState } from "react";
import { Pressable, ScrollView, Text, TextInput, View } from "react-native";
import { useLocalSearchParams } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import {
  ArrowDown,
  ArrowRight,
  Check,
  FileText,
  GitMerge,
  Send,
  Trash2,
} from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { getProviderIcon } from "@/components/provider-icons";
import { useIsCompactFormFactor } from "@/constants/layout";
import {
  getTaskGroupById,
  getTaskGroupDetails,
  type TaskGroup,
  type TaskGroupActivityEvent,
  type TaskGroupChatMessage,
  type TaskGroupMember,
  type TaskGroupSharedFile,
} from "./mock-data";
import {
  BetaBadge,
  formatCost,
  formatTokensK,
  statusColor,
} from "./task-group-ui";

type ChangeState = "pending" | "merged" | "dropped";

// ---------------------------------------------------------------------------
// Members pane
// ---------------------------------------------------------------------------

function MemberRow({ member }: { member: TaskGroupMember }) {
  const { theme } = useUnistyles();
  const ProviderIcon = getProviderIcon(member.provider);
  const color = statusColor(theme, member.status);

  return (
    <View style={styles.memberRow} testID={`member-row-${member.agentID}`}>
      <View style={styles.memberRowHeader}>
        <ProviderIcon size={16} color={theme.colors.foregroundMuted} />
        <Text style={styles.memberRowName} numberOfLines={1}>
          {member.agentID}
        </Text>
        <View style={styles.roleBadge}>
          <Text style={styles.roleBadgeText}>{member.role}</Text>
        </View>
      </View>
      <View style={styles.memberRowMeta}>
        <View style={[styles.statusDot, { backgroundColor: color }]} />
        <Text style={[styles.statusText, { color }]}>{member.status}</Text>
        {member.scope ? (
          <Text style={styles.memberRowScope} numberOfLines={1}>
            {member.scope}
          </Text>
        ) : null}
      </View>
      <Text style={styles.memberRowBranch} numberOfLines={1}>
        {member.worktreeBranch}
      </Text>
    </View>
  );
}

function MembersPane({ group }: { group: TaskGroup }) {
  const { theme } = useUnistyles();
  const isPipeline = group.mode === "pipeline";

  return (
    <ScrollView style={styles.paneScroll} contentContainerStyle={styles.paneContent}>
      <Text style={styles.paneTitle}>
        Members · {isPipeline ? "pipeline order" : "parallel"}
      </Text>
      {group.members.map((member, index) => (
        <View key={member.agentID}>
          {isPipeline && index > 0 ? (
            <View style={styles.pipelineArrow}>
              <ArrowDown size={14} color={theme.colors.foregroundMuted} />
            </View>
          ) : null}
          <MemberRow member={member} />
        </View>
      ))}
    </ScrollView>
  );
}

// ---------------------------------------------------------------------------
// Chat pane (static mock conversation + disabled input bar)
// ---------------------------------------------------------------------------

function ChatMessageRow({
  message,
  group,
}: {
  message: TaskGroupChatMessage;
  group: TaskGroup;
}) {
  const { theme } = useUnistyles();
  const isUser = message.authorID === "user";
  const member = group.members.find((m) => m.agentID === message.authorID);

  return (
    <View style={[styles.chatRow, isUser && styles.chatRowUser]}>
      <View style={styles.chatBubbleHeader}>
        <Text style={styles.chatAuthor}>{isUser ? "you" : (member?.agentID ?? message.authorID)}</Text>
        <Text style={styles.chatTime}>{message.time}</Text>
      </View>
      <View
        style={[
          styles.chatBubble,
          { backgroundColor: isUser ? theme.colors.surface2 : theme.colors.surface0 },
        ]}
      >
        <Text style={styles.chatText}>{message.text}</Text>
      </View>
    </View>
  );
}

function ChatPane({ group, chat }: { group: TaskGroup; chat: TaskGroupChatMessage[] }) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.chatPane}>
      <ScrollView style={styles.paneScroll} contentContainerStyle={styles.chatContent}>
        <Text style={styles.prototypeNote}>Prototype — conversation is static mock data</Text>
        {chat.map((message) => (
          <ChatMessageRow key={message.id} message={message} group={group} />
        ))}
      </ScrollView>
      <View style={styles.chatInputBar}>
        <TextInput
          style={styles.chatInput}
          placeholder="Message the group… (disabled in prototype)"
          placeholderTextColor={theme.colors.foregroundMuted}
          editable={false}
          testID="chat-input"
        />
        <Pressable
          disabled
          accessibilityLabel="Send message"
          accessibilityRole="button"
          style={[styles.chatSendButton, { opacity: 0.4 }]}
        >
          <Send size={16} color={theme.colors.background} />
        </Pressable>
      </View>
    </View>
  );
}

// ---------------------------------------------------------------------------
// Output pane: shared context, changes, activity
// ---------------------------------------------------------------------------

function SharedContextSection({ files }: { files: TaskGroupSharedFile[] }) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.section}>
      <Text style={styles.paneTitle}>Shared Context</Text>
      {files.map((file) => (
        <View key={file.path} style={styles.fileRow}>
          <FileText size={13} color={theme.colors.foregroundMuted} />
          <Text style={styles.filePath} numberOfLines={1}>
            {file.path}
          </Text>
          <Text style={styles.fileMeta}>
            {file.addedBy} · {file.updatedAt}
          </Text>
        </View>
      ))}
    </View>
  );
}

function ChangeRow({
  member,
  state,
  onMerge,
  onDrop,
}: {
  member: TaskGroupMember;
  state: ChangeState;
  onMerge: () => void;
  onDrop: () => void;
}) {
  const { theme } = useUnistyles();
  const [showDiff, setShowDiff] = useState(false);

  return (
    <View style={[styles.changeRow, state !== "pending" && { opacity: 0.55 }]} testID={`change-row-${member.agentID}`}>
      <View style={styles.changeRowHeader}>
        <Text style={styles.changeBranch} numberOfLines={1}>
          {member.worktreeBranch}
        </Text>
        <Text style={styles.changeStat}>
          <Text style={{ color: theme.colors.diffAddition }}>+{member.additions}</Text>
          {" "}
          <Text style={{ color: theme.colors.diffDeletion }}>−{member.deletions}</Text>
        </Text>
      </View>
      <View style={styles.changeActions}>
        <Pressable
          onPress={() => setShowDiff((prev) => !prev)}
          style={styles.changeButton}
          accessibilityRole="button"
          accessibilityLabel={`Diff ${member.worktreeBranch}`}
          testID={`diff-button-${member.agentID}`}
        >
          <Text style={styles.changeButtonText}>Diff</Text>
        </Pressable>
        <Pressable
          onPress={onMerge}
          disabled={state !== "pending"}
          style={[styles.changeButton, state === "merged" && { borderColor: theme.colors.statusSuccess }]}
          accessibilityRole="button"
          accessibilityLabel={`Merge ${member.worktreeBranch}`}
          testID={`merge-button-${member.agentID}`}
        >
          {state === "merged" ? (
            <Check size={12} color={theme.colors.statusSuccess} />
          ) : (
            <GitMerge size={12} color={theme.colors.foregroundMuted} />
          )}
          <Text style={styles.changeButtonText}>{state === "merged" ? "Merged" : "Merge"}</Text>
        </Pressable>
        <Pressable
          onPress={onDrop}
          disabled={state !== "pending"}
          style={[styles.changeButton, state === "dropped" && { borderColor: theme.colors.destructive }]}
          accessibilityRole="button"
          accessibilityLabel={`Drop ${member.worktreeBranch}`}
          testID={`drop-button-${member.agentID}`}
        >
          <Trash2 size={12} color={state === "dropped" ? theme.colors.destructive : theme.colors.foregroundMuted} />
          <Text style={styles.changeButtonText}>{state === "dropped" ? "Dropped" : "Drop"}</Text>
        </Pressable>
      </View>
      {showDiff ? (
        <View style={styles.diffPreview}>
          <Text style={[styles.diffLine, { color: theme.colors.diffAddition }]}>
            + ({member.additions} additions on {member.worktreeBranch})
          </Text>
          <Text style={[styles.diffLine, { color: theme.colors.diffDeletion }]}>
            − ({member.deletions} deletions) — diff rendering is a prototype placeholder
          </Text>
        </View>
      ) : null}
    </View>
  );
}

const ACTIVITY_KIND_ICON: Record<TaskGroupActivityEvent["kind"], typeof ArrowRight> = {
  handoff: ArrowRight,
  started: ArrowRight,
  finished: Check,
  merged: GitMerge,
};

function ActivitySection({ activity }: { activity: TaskGroupActivityEvent[] }) {
  const { theme } = useUnistyles();

  return (
    <View style={styles.section}>
      <Text style={styles.paneTitle}>Activity</Text>
      {activity.map((event, index) => {
        const Icon = ACTIVITY_KIND_ICON[event.kind];
        return (
          <View key={`${event.time}-${index}`} style={styles.activityRow}>
            <Text style={styles.activityTime}>{event.time}</Text>
            <Icon size={12} color={theme.colors.foregroundMuted} />
            <Text style={styles.activityText}>{event.text}</Text>
          </View>
        );
      })}
    </View>
  );
}

function OutputPane({ group }: { group: TaskGroup }) {
  const details = getTaskGroupDetails(group.id);
  const [changeStates, setChangeStates] = useState<Record<string, ChangeState>>({});

  const setChangeState = useCallback((agentID: string, state: ChangeState) => {
    setChangeStates((prev) => ({ ...prev, [agentID]: state }));
  }, []);

  return (
    <ScrollView style={styles.paneScroll} contentContainerStyle={styles.paneContent}>
      <SharedContextSection files={details.sharedFiles} />
      <View style={styles.section}>
        <Text style={styles.paneTitle}>Changes</Text>
        {group.members.map((member) => (
          <ChangeRow
            key={member.agentID}
            member={member}
            state={changeStates[member.agentID] ?? "pending"}
            onMerge={() => setChangeState(member.agentID, "merged")}
            onDrop={() => setChangeState(member.agentID, "dropped")}
          />
        ))}
      </View>
      <ActivitySection activity={details.activity} />
    </ScrollView>
  );
}

// ---------------------------------------------------------------------------
// Screen
// ---------------------------------------------------------------------------

type DetailTab = "members" | "chat" | "output";

export function TaskGroupDetailScreen() {
  return (
    <ErrorBoundary fallbackLabel="Task group encountered an error">
      <TaskGroupDetailScreenInner />
    </ErrorBoundary>
  );
}

function TaskGroupDetailScreenInner() {
  const { theme } = useUnistyles();
  const isCompact = useIsCompactFormFactor();
  const params = useLocalSearchParams<{ groupId?: string }>();
  const groupId = typeof params.groupId === "string" ? params.groupId : "";
  const group = useMemo(() => getTaskGroupById(groupId), [groupId]);
  const [activeTab, setActiveTab] = useState<DetailTab>("chat");

  if (!group) {
    return (
      <View style={styles.container}>
        <BackHeader title="Task Groups" />
        <View style={styles.centerContent}>
          <Text style={styles.notFoundText}>Task group not found: {groupId}</Text>
        </View>
      </View>
    );
  }

  const details = getTaskGroupDetails(group.id);

  const headerRight = (
    <View style={styles.headerStats}>
      <Text style={styles.headerStatText}>
        {group.tasksDone}/{group.tasksTotal} tasks
      </Text>
      <Text style={styles.headerStatText}>{formatCost(group.costUSD)}</Text>
      <Text style={styles.headerStatText}>{formatTokensK(group.tokensK)}</Text>
    </View>
  );

  return (
    <View style={styles.container}>
      <BackHeader
        title={group.goal}
        titleAccessory={<BetaBadge />}
        rightContent={headerRight}
      />
      {isCompact ? (
        <View style={styles.compactBody}>
          <View style={styles.segmentedRow}>
            {(["members", "chat", "output"] as const).map((tab) => (
              <Pressable
                key={tab}
                onPress={() => setActiveTab(tab)}
                style={[
                  styles.segmentButton,
                  activeTab === tab && { backgroundColor: theme.colors.primary },
                ]}
                testID={`tab-${tab}`}
              >
                <Text
                  style={[
                    styles.segmentText,
                    {
                      color:
                        activeTab === tab
                          ? theme.colors.background
                          : theme.colors.foregroundMuted,
                    },
                  ]}
                >
                  {tab === "members" ? "Members" : tab === "chat" ? "Chat" : "Output"}
                </Text>
              </Pressable>
            ))}
          </View>
          {activeTab === "members" ? (
            <MembersPane group={group} />
          ) : activeTab === "chat" ? (
            <ChatPane group={group} chat={details.chat} />
          ) : (
            <OutputPane group={group} />
          )}
        </View>
      ) : (
        <View style={styles.panesRow}>
          <View style={styles.membersPane}>
            <MembersPane group={group} />
          </View>
          <View style={styles.chatPaneWrapper}>
            <ChatPane group={group} chat={details.chat} />
          </View>
          <View style={styles.outputPane}>
            <OutputPane group={group} />
          </View>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.background,
  },
  centerContent: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
  },
  notFoundText: {
    color: theme.colors.foregroundMuted,
    fontSize: 14,
  },
  headerStats: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
  },
  headerStatText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    fontVariant: ["tabular-nums"],
  },
  panesRow: {
    flex: 1,
    flexDirection: "row",
  },
  membersPane: {
    width: 260,
    borderRightWidth: 1,
    borderRightColor: theme.colors.border,
  },
  chatPaneWrapper: {
    flex: 1,
    minWidth: 0,
  },
  outputPane: {
    width: 340,
    borderLeftWidth: 1,
    borderLeftColor: theme.colors.border,
  },
  compactBody: {
    flex: 1,
  },
  segmentedRow: {
    flexDirection: "row",
    gap: 8,
    padding: 12,
  },
  segmentButton: {
    paddingHorizontal: 14,
    paddingVertical: 6,
    borderRadius: 8,
    backgroundColor: theme.colors.surface0,
  },
  segmentText: {
    fontSize: 13,
    fontWeight: "500",
  },
  paneScroll: {
    flex: 1,
  },
  paneContent: {
    padding: 12,
    gap: 8,
  },
  paneTitle: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    fontWeight: "600",
    textTransform: "uppercase",
    letterSpacing: 0.5,
    marginBottom: 4,
  },
  memberRow: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 8,
    padding: 10,
    gap: 6,
  },
  memberRowHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  memberRowName: {
    color: theme.colors.foreground,
    fontSize: 13,
    fontWeight: "600",
    flexShrink: 1,
  },
  roleBadge: {
    backgroundColor: theme.colors.surface2,
    paddingHorizontal: 6,
    paddingVertical: 1,
    borderRadius: 4,
  },
  roleBadgeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 10,
    fontWeight: "600",
  },
  memberRowMeta: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  statusDot: {
    width: 7,
    height: 7,
    borderRadius: 4,
  },
  statusText: {
    fontSize: 11,
    fontWeight: "500",
  },
  memberRowScope: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    flexShrink: 1,
  },
  memberRowBranch: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontFamily: "monospace",
  },
  pipelineArrow: {
    alignItems: "center",
    paddingVertical: 2,
  },
  chatPane: {
    flex: 1,
  },
  chatContent: {
    padding: 16,
    gap: 12,
  },
  prototypeNote: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontStyle: "italic",
    textAlign: "center",
  },
  chatRow: {
    gap: 4,
    maxWidth: "85%",
  },
  chatRowUser: {
    alignSelf: "flex-end",
  },
  chatBubbleHeader: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  chatAuthor: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontWeight: "600",
  },
  chatTime: {
    color: theme.colors.foregroundMuted,
    fontSize: 10,
  },
  chatBubble: {
    borderRadius: 10,
    paddingHorizontal: 12,
    paddingVertical: 8,
  },
  chatText: {
    color: theme.colors.foreground,
    fontSize: 13,
    lineHeight: 19,
  },
  chatInputBar: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    padding: 12,
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  chatInput: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 8,
    color: theme.colors.foreground,
    fontSize: 13,
  },
  chatSendButton: {
    backgroundColor: theme.colors.primary,
    borderRadius: 8,
    padding: 8,
  },
  section: {
    gap: 6,
    marginBottom: 12,
  },
  fileRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
  },
  filePath: {
    color: theme.colors.foreground,
    fontSize: 12,
    flexShrink: 1,
  },
  fileMeta: {
    color: theme.colors.foregroundMuted,
    fontSize: 10,
  },
  changeRow: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 8,
    padding: 10,
    gap: 8,
  },
  changeRowHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
  },
  changeBranch: {
    color: theme.colors.foreground,
    fontSize: 12,
    fontWeight: "500",
    flexShrink: 1,
  },
  changeStat: {
    fontSize: 11,
    fontVariant: ["tabular-nums"],
  },
  changeActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  changeButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
    paddingHorizontal: 8,
    paddingVertical: 4,
    borderRadius: 6,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  changeButtonText: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontWeight: "500",
  },
  diffPreview: {
    backgroundColor: theme.colors.surface2,
    borderRadius: 6,
    padding: 8,
    gap: 2,
  },
  diffLine: {
    fontSize: 11,
    fontFamily: "monospace",
  },
  activityRow: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 6,
  },
  activityTime: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontVariant: ["tabular-nums"],
    width: 36,
  },
  activityText: {
    color: theme.colors.foreground,
    fontSize: 12,
    flexShrink: 1,
  },
}));

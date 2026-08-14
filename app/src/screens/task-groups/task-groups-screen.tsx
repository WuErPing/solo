/**
 * Task Groups list screen (multi-agent collaboration prototype).
 *
 * Renders one card per mock task group: goal, project, member chips with
 * per-agent status, progress, cost/tokens, and a review-needed badge.
 * All data comes from local mock fixtures — no backend wiring.
 */
import { useCallback } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { GitBranch, Layers, Users } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { ErrorBoundary } from "@/components/error-boundary";
import { useIsCompactFormFactor } from "@/constants/layout";
import { MOCK_TASK_GROUPS, type TaskGroup } from "./mock-data";
import { BetaBadge, MemberChip, formatCost, formatTokensK } from "./task-group-ui";

function ProgressBar({ done, total }: { done: number; total: number }) {
  const { theme } = useUnistyles();
  const ratio = total > 0 ? Math.min(1, done / total) : 0;
  const isComplete = done >= total && total > 0;

  return (
    <View style={styles.progressRow}>
      <View style={styles.progressTrack}>
        <View
          style={[
            styles.progressFill,
            {
              width: `${ratio * 100}%`,
              backgroundColor: isComplete ? theme.colors.statusSuccess : theme.colors.primary,
            },
          ]}
        />
      </View>
      <Text style={styles.progressText}>
        {done}/{total} tasks
      </Text>
    </View>
  );
}

function TaskGroupCard({
  group,
  isCompact,
  onPress,
}: {
  group: TaskGroup;
  isCompact: boolean;
  onPress: () => void;
}) {
  const { theme } = useUnistyles();
  const needsReview = group.pendingReviewCount > 0;

  return (
    <Pressable
      onPress={onPress}
      testID={`task-group-card-${group.id}`}
      accessibilityRole="button"
      accessibilityLabel={`Open task group ${group.goal}`}
      style={({ pressed }) => [
        styles.card,
        isCompact && styles.cardCompact,
        pressed && { opacity: 0.85 },
      ]}
    >
      <View style={styles.cardHeader}>
        <Text style={styles.cardGoal} numberOfLines={2}>
          {group.goal}
        </Text>
        {needsReview ? (
          <View
            style={[styles.reviewBadge, { backgroundColor: theme.colors.statusWarning }]}
            testID={`review-badge-${group.id}`}
          >
            <Text style={[styles.reviewBadgeText, { color: theme.colors.background }]}>
              待验收 Review ×{group.pendingReviewCount}
            </Text>
          </View>
        ) : null}
      </View>

      <View style={styles.cardMetaRow}>
        <View style={styles.metaItem}>
          <GitBranch size={12} color={theme.colors.foregroundMuted} />
          <Text style={styles.metaText}>{group.projectName}</Text>
        </View>
        <View style={styles.metaItem}>
          {group.mode === "pipeline" ? (
            <Layers size={12} color={theme.colors.foregroundMuted} />
          ) : (
            <Users size={12} color={theme.colors.foregroundMuted} />
          )}
          <Text style={styles.metaText}>{group.mode}</Text>
        </View>
        <Text style={styles.metaText}>{group.updatedAt.slice(11, 16)}</Text>
      </View>

      <View style={styles.memberChipList}>
        {group.members.map((member) => (
          <MemberChip key={member.agentID} member={member} />
        ))}
      </View>

      <ProgressBar done={group.tasksDone} total={group.tasksTotal} />

      <View style={styles.cardFooter}>
        <Text style={styles.footerText}>{formatCost(group.costUSD)}</Text>
        <Text style={styles.footerText}>{formatTokensK(group.tokensK)}</Text>
      </View>
    </Pressable>
  );
}

export function TaskGroupsScreen() {
  return (
    <ErrorBoundary fallbackLabel="Task groups encountered an error">
      <TaskGroupsScreenInner />
    </ErrorBoundary>
  );
}

function TaskGroupsScreenInner() {
  const isCompact = useIsCompactFormFactor();

  const handleOpenGroup = useCallback((groupId: string) => {
    router.push(`/task-groups/${groupId}` as never);
  }, []);

  return (
    <View style={styles.container}>
      <BackHeader
        title="Task Groups"
        titleAccessory={<BetaBadge />}
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.countBadge}>
            <Text style={styles.countBadgeText}>{MOCK_TASK_GROUPS.length} group(s)</Text>
          </View>
        }
      />
      <ScrollView style={styles.scrollView}>
        <View style={isCompact ? styles.gridCompact : styles.grid}>
          {MOCK_TASK_GROUPS.map((group) => (
            <TaskGroupCard
              key={group.id}
              group={group}
              isCompact={isCompact}
              onPress={() => handleOpenGroup(group.id)}
            />
          ))}
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.background,
  },
  scrollView: {
    flex: 1,
  },
  countBadge: {
    backgroundColor: theme.colors.surface0,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderRadius: 12,
  },
  countBadgeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    fontWeight: "500",
  },
  grid: {
    flexDirection: "row",
    flexWrap: "wrap",
    padding: 16,
    gap: 12,
  },
  gridCompact: {
    padding: 16,
    gap: 12,
  },
  card: {
    backgroundColor: theme.colors.surface0,
    borderRadius: 10,
    padding: 14,
    width: 320,
    gap: 10,
  },
  cardCompact: {
    width: "100%",
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "flex-start",
    justifyContent: "space-between",
    gap: 8,
  },
  cardGoal: {
    color: theme.colors.foreground,
    fontSize: 15,
    fontWeight: "600",
    flexShrink: 1,
  },
  reviewBadge: {
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderRadius: 6,
  },
  reviewBadgeText: {
    fontSize: 11,
    fontWeight: "700",
  },
  cardMetaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
  },
  metaItem: {
    flexDirection: "row",
    alignItems: "center",
    gap: 4,
  },
  metaText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  memberChipList: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 6,
  },
  progressRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  progressTrack: {
    flex: 1,
    height: 6,
    borderRadius: 3,
    backgroundColor: theme.colors.surface2,
    overflow: "hidden",
  },
  progressFill: {
    height: 6,
    borderRadius: 3,
  },
  progressText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
  },
  cardFooter: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
  },
  footerText: {
    color: theme.colors.foregroundMuted,
    fontSize: 12,
    fontVariant: ["tabular-nums"],
  },
}));

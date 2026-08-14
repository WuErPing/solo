/**
 * Shared presentational pieces for the Task Groups prototype screens.
 */
import { Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { getProviderIcon } from "@/components/provider-icons";
import type { AgentStatus, TaskGroupMember } from "./mock-data";

type UnistylesTheme = ReturnType<typeof useUnistyles>["theme"];

export function statusColor(theme: UnistylesTheme, status: AgentStatus): string {
  switch (status) {
    case "busy":
      return theme.colors.primary;
    case "idle":
      return theme.colors.foregroundMuted;
    case "waiting":
      return theme.colors.statusWarning;
    case "done":
      return theme.colors.statusSuccess;
    case "error":
      return theme.colors.statusDanger;
  }
}

export function formatCost(costUSD: number): string {
  return `$${costUSD.toFixed(2)}`;
}

export function formatTokensK(tokensK: number): string {
  return tokensK >= 1000 ? `${(tokensK / 1000).toFixed(1)}M tok` : `${tokensK}K tok`;
}

export function BetaBadge() {
  return (
    <View style={styles.betaBadge}>
      <Text style={styles.betaBadgeText}>Beta</Text>
    </View>
  );
}

export function MemberChip({ member }: { member: TaskGroupMember }) {
  const { theme } = useUnistyles();
  const ProviderIcon = getProviderIcon(member.provider);
  const dotColor = statusColor(theme, member.status);

  return (
    <View style={styles.memberChip} testID={`member-chip-${member.agentID}`}>
      <ProviderIcon size={14} color={theme.colors.foregroundMuted} />
      <Text style={styles.memberChipRole}>{member.role}</Text>
      <View style={[styles.statusDot, { backgroundColor: dotColor }]} />
      <Text style={[styles.memberChipStatus, { color: dotColor }]}>{member.status}</Text>
      {member.scope ? (
        <Text style={styles.memberChipScope} numberOfLines={1}>
          {member.scope}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  betaBadge: {
    backgroundColor: theme.colors.surface2,
    paddingHorizontal: 8,
    paddingVertical: 2,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  betaBadgeText: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    fontWeight: "600",
  },
  memberChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    backgroundColor: theme.colors.surface1,
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
  },
  memberChipRole: {
    color: theme.colors.foreground,
    fontSize: 12,
    fontWeight: "500",
  },
  statusDot: {
    width: 7,
    height: 7,
    borderRadius: 4,
  },
  memberChipStatus: {
    fontSize: 11,
    fontWeight: "500",
  },
  memberChipScope: {
    color: theme.colors.foregroundMuted,
    fontSize: 11,
    flexShrink: 1,
  },
}));

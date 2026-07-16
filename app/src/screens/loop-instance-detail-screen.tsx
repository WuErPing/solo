import { useCallback, useMemo } from "react";
import { View, Text, ScrollView } from "react-native";
import { useIsFocused , router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ArrowLeft, Loader } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useLoopInspect } from "@/hooks/use-loop-inspect";
import { buildHostLoopDetailRoute } from "@/utils/host-routes";

export function LoopInstanceDetailScreen({
  serverId,
  instanceId,
  templateId,
}: {
  serverId: string;
  instanceId: string;
  templateId: string;
}) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return (
    <LoopInstanceDetailScreenContent
      serverId={serverId}
      instanceId={instanceId}
      templateId={templateId}
    />
  );
}

function formatLogTime(timestamp: string): string {
  const match = timestamp.match(/T(\d{2}:\d{2}:\d{2})/);
  return match ? match[1]! : timestamp;
}

function LoopStatusBadge({ status }: { status: string }) {
  const { theme } = useUnistyles();
  const color = useMemo(() => {
    switch (status) {
      case "running":
        return theme.colors.palette.amber[500];
      case "succeeded":
        return theme.colors.palette.green[400];
      case "failed":
        return theme.colors.palette.red[500];
      case "stopped":
        return theme.colors.foregroundMuted;
      default:
        return theme.colors.foregroundMuted;
    }
  }, [status, theme]);

  return (
    <View style={[styles.statusBadge, { borderColor: color }]}>
      {status === "running" ? (
        <Loader size={12} color={color} />
      ) : (
        <View style={[styles.statusDot, { backgroundColor: color }]} />
      )}
      <Text style={[styles.statusText, { color }]}>{status}</Text>
    </View>
  );
}

function LoopInstanceDetailScreenContent({
  serverId,
  instanceId,
  templateId,
}: {
  serverId: string;
  instanceId: string;
  templateId: string;
}) {
  const { theme } = useUnistyles();
  const { loop, isLoading, error } = useLoopInspect({ serverId, loopId: instanceId });

  const handleBack = useCallback(() => {
    router.navigate(buildHostLoopDetailRoute(serverId, templateId));
  }, [serverId, templateId]);

  if (isLoading) {
    return (
      <View style={styles.container}>
        <BackHeader title="Instance" onBack={handleBack} />
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      </View>
    );
  }

  if (error) {
    return (
      <View style={styles.container}>
        <BackHeader title="Instance" onBack={handleBack} />
        <View style={styles.emptyContainer}>
          <Text style={styles.errorText}>{error}</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  if (!loop) {
    return (
      <View style={styles.container}>
        <BackHeader title="Instance" onBack={handleBack} />
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>Instance not found</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  const name = loop.name || "Run";

  return (
    <View style={styles.container}>
      <BackHeader title={name} onBack={handleBack} />

      <ScrollView style={styles.scroll} contentContainerStyle={styles.scrollContent}>
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle}>{name}</Text>
            <LoopStatusBadge status={loop.status} />
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Status:</Text>
            <Text style={styles.detailValue}>{loop.status}</Text>
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Created:</Text>
            <Text style={styles.detailValue}>{loop.createdAt}</Text>
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Updated:</Text>
            <Text style={styles.detailValue}>{loop.updatedAt}</Text>
          </View>

          {loop.prompt ? (
            <View style={styles.detailRow}>
              <Text style={styles.detailLabel}>Prompt:</Text>
              <Text style={styles.detailValue}>{loop.prompt}</Text>
            </View>
          ) : null}
        </View>

        {loop.iterations.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Iterations ({loop.iterations.length})</Text>
            {loop.iterations.map((iteration) => (
              <View key={iteration.index} style={styles.iterationCard}>
                <View style={styles.iterationHeader}>
                  <Text style={styles.iterationTitle}>Iteration {iteration.index}</Text>
                  <LoopStatusBadge status={iteration.status} />
                </View>
                {iteration.failureReason ? (
                  <Text style={styles.errorText}>{iteration.failureReason}</Text>
                ) : null}
                {iteration.verifyChecks.map((check, idx) => {
                  const output = [check.stdout, check.stderr]
                    .map((s) => s.trim())
                    .filter((s) => s.length > 0)
                    .join("\n");
                  return (
                    <View key={idx} style={styles.checkRow}>
                      <Text
                        style={[
                          styles.checkCommand,
                          {
                            color: check.passed
                              ? theme.colors.palette.green[400]
                              : theme.colors.palette.red[500],
                          },
                        ]}
                      >
                        {check.passed ? "✓" : "✗"} {check.command} (exit {check.exitCode})
                      </Text>
                      {!check.passed && output ? (
                        <Text style={styles.codeOutput}>{output}</Text>
                      ) : null}
                    </View>
                  );
                })}
              </View>
            ))}
          </View>
        ) : null}

        {loop.logs.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Logs ({loop.logs.length})</Text>
            <View style={styles.logList}>
              {loop.logs.map((log) => (
                <View key={log.seq} style={styles.logRow}>
                  <Text style={styles.logMeta}>
                    {formatLogTime(log.timestamp)} · {log.source}
                    {log.iteration != null ? ` · #${log.iteration}` : ""}
                  </Text>
                  <Text style={[styles.logText, log.level === "error" ? styles.logTextError : null]}>
                    {log.text}
                  </Text>
                </View>
              ))}
            </View>
          </View>
        ) : null}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  loadingContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
  },
  emptyContainer: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    gap: theme.spacing[6],
    padding: theme.spacing[6],
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.base,
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[4],
  },
  section: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[4],
  },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: theme.spacing[2],
  },
  sectionTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.lg,
    fontWeight: "600",
  },
  detailRow: {
    gap: theme.spacing[1],
  },
  detailLabel: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  detailValue: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
  },
  statusBadge: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    paddingHorizontal: theme.spacing[3],
    borderRadius: theme.borderRadius.full,
    borderWidth: theme.borderWidth[1],
    flexShrink: 0,
  },
  statusDot: {
    width: 6,
    height: 6,
    borderRadius: theme.borderRadius.full,
  },
  statusText: {
    fontSize: theme.fontSize.sm,
    textTransform: "capitalize",
  },
  iterationCard: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[3],
    gap: theme.spacing[2],
  },
  iterationHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
  },
  iterationTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: "500",
  },
  checkRow: {
    gap: theme.spacing[1],
  },
  checkCommand: {
    fontSize: theme.fontSize.sm,
    fontWeight: "500",
  },
  codeOutput: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    fontFamily: "monospace",
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.sm,
    padding: theme.spacing[2],
  },
  logList: {
    gap: theme.spacing[2],
  },
  logRow: {
    gap: theme.spacing[1],
  },
  logMeta: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    fontFamily: "monospace",
  },
  logText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
  },
  logTextError: {
    color: theme.colors.palette.red[500],
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
}));

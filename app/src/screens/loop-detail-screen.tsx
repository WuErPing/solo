import { useCallback, useMemo, useState } from "react";
import { View, Text, ScrollView, Pressable } from "react-native";
import { useIsFocused } from "@/hooks/use-is-focused";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import {
  ArrowLeft,
  ChevronDown,
  ChevronRight,
  Loader,
  Pencil,
  RotateCcw,
  Trash2,
} from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { AdaptiveModalSheet, AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { useCreateLoop } from "@/hooks/use-create-loop";
import { useLoopTemplateDetail } from "@/hooks/use-loop-templates";
import { useLoopMutations } from "@/hooks/use-loop-mutations";
import { useProvidersSnapshot } from "@/hooks/use-providers-snapshot";
import { useToast } from "@/contexts/toast-context";
import { buildHostLoopDetailRoute, buildHostLoopInstanceDetailRoute, buildHostLoopsRoute } from "@/utils/host-routes";
import type { AgentProvider } from "@server/server/agent/agent-sdk-types";

export function LoopDetailScreen({ serverId, loopId }: { serverId: string; loopId: string }) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <LoopDetailScreenContent serverId={serverId} loopId={loopId} />;
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

function LoopDetailScreenContent({ serverId, loopId }: { serverId: string; loopId: string }) {
  const { theme } = useUnistyles();
  const toast = useToast();
  const { template, instances, latestRecord, isLoading, error } = useLoopTemplateDetail({ serverId, templateId: loopId });
  const { updateLoop, deleteLoop, isUpdating, isDeleting } = useLoopMutations({ serverId });
  const { createLoop, isCreating } = useCreateLoop({ serverId });

  // Use latestRecord for detailed config (prompt, verifyChecks, etc.)
  const loop = latestRecord;

  const [isEditModalVisible, setIsEditModalVisible] = useState(false);
  const { entries: providers, isLoading: isProvidersLoading } = useProvidersSnapshot(serverId, {
    enabled: isEditModalVisible,
  });
  const [editName, setEditName] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editCwd, setEditCwd] = useState("");
  const [editVerifyChecks, setEditVerifyChecks] = useState("");
  const [editMaxIterations, setEditMaxIterations] = useState("");
  const [isAgentOpen, setIsAgentOpen] = useState(false);
  const [editSelectedProvider, setEditSelectedProvider] = useState<string | null>(null);
  const [editModel, setEditModel] = useState("");
  const [editSystemPrompt, setEditSystemPrompt] = useState("");
  const [isDeleteModalVisible, setIsDeleteModalVisible] = useState(false);
  const [isInstanceDeleteModalVisible, setIsInstanceDeleteModalVisible] = useState(false);
  const [selectedInstanceId, setSelectedInstanceId] = useState<string | null>(null);
  const [editError, setEditError] = useState<string | null>(null);
  const [retryError, setRetryError] = useState<string | null>(null);

  const providerOptions = useMemo(() => {
    return (providers ?? []).map((provider) => ({
      id: provider.provider,
      label: provider.label || provider.provider,
      description: provider.description || "",
      status: provider.status,
      enabled: provider.enabled,
    }));
  }, [providers]);

  const handleBack = useCallback(() => {
    router.navigate(buildHostLoopsRoute(serverId));
  }, [serverId]);

  const handleOpenEdit = useCallback(() => {
    setEditName(loop?.name ?? "");
    setEditPrompt(loop?.prompt ?? "");
    setEditCwd(loop?.cwd ?? "");
    setEditVerifyChecks(loop?.verifyChecks?.join("\n") ?? "");
    setEditMaxIterations(loop?.maxIterations?.toString() ?? "");
    const hasTemplate = Boolean(loop?.agentTemplate);
    setIsAgentOpen(hasTemplate);
    setEditSelectedProvider(loop?.agentTemplate?.provider ?? loop?.provider ?? null);
    setEditModel(loop?.agentTemplate?.model ?? loop?.model ?? "");
    setEditSystemPrompt(loop?.agentTemplate?.systemPrompt ?? "");
    setEditError(null);
    setIsEditModalVisible(true);
  }, [
    loop?.name,
    loop?.prompt,
    loop?.cwd,
    loop?.verifyChecks,
    loop?.maxIterations,
    loop?.agentTemplate,
    loop?.provider,
    loop?.model,
  ]);

  const handleCloseEdit = useCallback(() => {
    setIsEditModalVisible(false);
  }, []);

  const handleSaveEdit = useCallback(async () => {
    setEditError(null);

    const trimmedPrompt = editPrompt.trim();
    const trimmedCwd = editCwd.trim();
    const checks = editVerifyChecks
      .split("\n")
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
    const maxIter = editMaxIterations.trim() ? parseInt(editMaxIterations.trim(), 10) : null;

    if (!trimmedPrompt) {
      setEditError("Prompt is required");
      return;
    }
    if (!trimmedCwd) {
      setEditError("Working directory is required");
      return;
    }
    if (editMaxIterations.trim() && (Number.isNaN(maxIter!) || maxIter! <= 0)) {
      setEditError("Max iterations must be a positive number");
      return;
    }
    if (isAgentOpen && !editSelectedProvider) {
      setEditError("Please select an agent provider");
      return;
    }

    const agentTemplate = editSelectedProvider
      ? {
          provider: editSelectedProvider as AgentProvider,
          cwd: trimmedCwd || "/",
          model: editModel.trim() || undefined,
          systemPrompt: editSystemPrompt.trim() || undefined,
        }
      : null;

    try {
      await updateLoop({
        loopId,
        name: editName.trim() || null,
        prompt: trimmedPrompt,
        cwd: trimmedCwd,
        verifyChecks: checks.length > 0 ? checks : null,
        maxIterations: maxIter,
        agentTemplate,
      });
      setIsEditModalVisible(false);
    } catch (e) {
      setEditError(e instanceof Error ? e.message : "Failed to update loop");
    }
  }, [
    updateLoop,
    loopId,
    editName,
    editPrompt,
    editCwd,
    editVerifyChecks,
    editMaxIterations,
    isAgentOpen,
    editSelectedProvider,
    editModel,
    editSystemPrompt,
  ]);

  const handleOpenDelete = useCallback(() => {
    setIsDeleteModalVisible(true);
  }, []);

  const handleCloseDelete = useCallback(() => {
    setIsDeleteModalVisible(false);
  }, []);

  const handleConfirmDelete = useCallback(async () => {
    try {
      await deleteLoop(loopId);
      setIsDeleteModalVisible(false);
      router.replace(buildHostLoopsRoute(serverId));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete loop");
    }
  }, [deleteLoop, loopId, serverId, toast]);

  const handleOpenInstanceDelete = useCallback((instanceId: string) => {
    setSelectedInstanceId(instanceId);
    setIsInstanceDeleteModalVisible(true);
  }, []);

  const handleCloseInstanceDelete = useCallback(() => {
    setIsInstanceDeleteModalVisible(false);
    setSelectedInstanceId(null);
  }, []);

  const handleConfirmInstanceDelete = useCallback(async () => {
    if (!selectedInstanceId) return;
    try {
      await deleteLoop(selectedInstanceId);
      setIsInstanceDeleteModalVisible(false);
      setSelectedInstanceId(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to delete loop instance");
    }
  }, [deleteLoop, selectedInstanceId, toast]);

  const handleRetry = useCallback(async () => {
    if (!loop) {
      return;
    }
    setRetryError(null);
    // Loops created through the app always carry an agentTemplate; fall back to
    // the legacy provider/model fields so older loops re-run with the same agent.
    const agentTemplate =
      loop.agentTemplate ??
      (loop.provider
        ? {
            provider: loop.provider as AgentProvider,
            cwd: loop.cwd,
            model: loop.model ?? undefined,
          }
        : null);
    try {
      const newLoop = await createLoop({
        prompt: loop.prompt,
        cwd: loop.cwd,
        name: loop.name,
        verifyPrompt: loop.verifyPrompt,
        verifyChecks: loop.verifyChecks,
        sleepMs: loop.sleepMs,
        maxIterations: loop.maxIterations ?? undefined,
        maxTimeMs: loop.maxTimeMs ?? undefined,
        agentTemplate,
        workerAgentTemplate: loop.workerAgentTemplate ?? null,
        verifierAgentTemplate: loop.verifierAgentTemplate ?? null,
      });
      router.replace(buildHostLoopDetailRoute(serverId, newLoop.id));
    } catch (e) {
      setRetryError(e instanceof Error ? e.message : "Failed to re-run loop");
    }
  }, [loop, createLoop, serverId]);

  if (isLoading) {
    return (
      <View style={styles.container}>
        <BackHeader title="Loop" onBack={handleBack} />
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      </View>
    );
  }

  if (error) {
    return (
      <View style={styles.container}>
        <BackHeader title="Loop" onBack={handleBack} />
        <View style={styles.emptyContainer}>
          <Text style={styles.errorText}>{error}</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  if (!template) {
    return (
      <View style={styles.container}>
        <BackHeader title="Loop" onBack={handleBack} />
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>Loop not found</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      </View>
    );
  }

  const name = template.name || "Untitled";

  return (
    <View style={styles.container}>
      <BackHeader
        title={name}
        onBack={handleBack}
        rightContent={
          <View style={styles.headerActions}>
            {template.latestStatus !== "running" ? (
              <Button
                variant="ghost"
                size="sm"
                leftIcon={RotateCcw}
                onPress={handleRetry}
                disabled={isCreating}
              >
                {isCreating ? "Starting..." : "Re-run"}
              </Button>
            ) : null}
            <Button variant="ghost" size="sm" leftIcon={Pencil} onPress={handleOpenEdit}>
              Edit
            </Button>
            <Button variant="ghost" size="sm" leftIcon={Trash2} onPress={handleOpenDelete}>
              Delete
            </Button>
          </View>
        }
      />

      <ScrollView style={styles.scroll} contentContainerStyle={styles.scrollContent}>
        {retryError ? (
          <View style={styles.errorBanner}>
            <Text style={styles.errorText}>{retryError}</Text>
          </View>
        ) : null}
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle} testID="loop-detail-name">
              {name}
            </Text>
            <LoopStatusBadge status={template.latestStatus || "unknown"} />
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Status:</Text>
            <Text style={styles.detailValue} testID="loop-detail-status">
              {template.latestStatus || "unknown"}
            </Text>
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Instances:</Text>
            <Text style={styles.detailValue}>{template.instanceCount}</Text>
          </View>

          {loop ? (
            <>
              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Prompt</Text>
                <Text style={styles.detailValue}>{loop.prompt}</Text>
              </View>

              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Working Directory</Text>
                <Text style={styles.detailValue}>{loop.cwd}</Text>
              </View>

              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Agent</Text>
                <Text style={styles.detailValue}>
                  {loop.agentTemplate?.provider ?? loop.provider ?? "default"}
                  {loop.agentTemplate?.model ?? loop.model
                    ? ` / ${loop.agentTemplate?.model ?? loop.model}`
                    : ""}
                </Text>
              </View>

              {loop.agentTemplate?.systemPrompt ? (
                <View style={styles.detailRow}>
                  <Text style={styles.detailLabel}>System Prompt</Text>
                  <Text style={styles.detailValue}>{loop.agentTemplate.systemPrompt}</Text>
                </View>
              ) : null}

              {loop.verifyChecks && loop.verifyChecks.length > 0 ? (
                <View style={styles.detailRow}>
                  <Text style={styles.detailLabel}>Verify Checks</Text>
                  {loop.verifyChecks.map((check, idx) => (
                    <Text key={idx} style={styles.detailValue}>
                      {check}
                    </Text>
                  ))}
                </View>
              ) : null}

              <View style={styles.detailRow}>
                <Text style={styles.detailLabel}>Max Iterations</Text>
                <Text style={styles.detailValue}>{loop.maxIterations ?? "unlimited"}</Text>
              </View>
            </>
          ) : null}
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Instances ({instances.length})</Text>
          {instances.length === 0 ? (
            <Text style={styles.emptyText}>No instances yet</Text>
          ) : (
            instances.map((instance) => (
              <View key={instance.id} style={styles.iterationCard}>
                <Pressable
                  style={styles.iterationContent}
                  onPress={() => {
                    router.navigate(
                      buildHostLoopInstanceDetailRoute(serverId, loopId, instance.id)
                    );
                  }}
                >
                  <View style={styles.iterationHeader}>
                    <Text style={styles.iterationTitle}>{instance.name || "Run"}</Text>
                    <LoopStatusBadge status={instance.status} />
                  </View>
                  <Text style={styles.detailValue}>
                    Created: {instance.createdAt}
                  </Text>
                </Pressable>
                <Pressable
                  style={styles.instanceDeleteButton}
                  onPress={() => handleOpenInstanceDelete(instance.id)}
                  disabled={isDeleting}
                >
                  <Trash2 size={16} color={theme.colors.palette.red[500]} />
                </Pressable>
              </View>
            ))
          )}
        </View>

        {loop && loop.iterations.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Latest Iterations</Text>
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
                          { color: check.passed ? theme.colors.palette.green[400] : theme.colors.palette.red[500] },
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

        {loop && loop.logs.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Logs</Text>
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

      <AdaptiveModalSheet title="Edit Loop" visible={isEditModalVisible} onClose={handleCloseEdit}>
        <View style={styles.modalContent}>
          {editError ? (
            <View style={styles.errorBanner}>
              <Text style={styles.errorText}>{editError}</Text>
            </View>
          ) : null}
          <View style={styles.field}>
            <Text style={styles.label}>Loop name</Text>
            <AdaptiveTextInput
              style={styles.input}
              placeholder="Loop name"
              value={editName}
              onChangeText={setEditName}
              placeholderTextColor={theme.colors.foregroundMuted}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>Prompt *</Text>
            <AdaptiveTextInput
              style={[styles.input, styles.textArea]}
              placeholder="e.g. Fix all test failures until CI passes"
              value={editPrompt}
              onChangeText={setEditPrompt}
              multiline
              numberOfLines={3}
              placeholderTextColor={theme.colors.foregroundMuted}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>Working Directory *</Text>
            <AdaptiveTextInput
              style={styles.input}
              placeholder="e.g. /Users/me/project"
              value={editCwd}
              onChangeText={setEditCwd}
              placeholderTextColor={theme.colors.foregroundMuted}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>Verify Checks</Text>
            <AdaptiveTextInput
              style={[styles.input, styles.textArea]}
              placeholder="One command per line, e.g. go test ./..."
              value={editVerifyChecks}
              onChangeText={setEditVerifyChecks}
              multiline
              numberOfLines={3}
              placeholderTextColor={theme.colors.foregroundMuted}
            />
          </View>
          <View style={styles.field}>
            <Text style={styles.label}>Max Iterations</Text>
            <AdaptiveTextInput
              style={styles.input}
              placeholder="e.g. 10"
              value={editMaxIterations}
              onChangeText={setEditMaxIterations}
              keyboardType="numeric"
              placeholderTextColor={theme.colors.foregroundMuted}
            />
          </View>

          <View style={styles.section}>
            <Pressable
              style={styles.sectionHeader}
              onPress={() => setIsAgentOpen((prev) => !prev)}
            >
              <Text style={styles.sectionTitle}>Agent Template</Text>
              {isAgentOpen ? (
                <ChevronDown size={20} color={theme.colors.foregroundMuted} />
              ) : (
                <ChevronRight size={20} color={theme.colors.foregroundMuted} />
              )}
            </Pressable>
            {isAgentOpen ? (
              <View style={styles.sectionContent}>
                <Text style={styles.helperText}>
                  Configure the agent that runs this loop. Leave collapsed to use
                  the default agent.
                </Text>

                <View style={styles.field}>
                  <Text style={styles.label}>Provider *</Text>
                  {isProvidersLoading ? (
                    <Text style={styles.helperText}>Loading providers...</Text>
                  ) : providerOptions.length === 0 ? (
                    <Text style={styles.helperText}>No providers available</Text>
                  ) : (
                    <View style={styles.providerList}>
                      {providerOptions.map((provider) => {
                        const isSelected = editSelectedProvider === provider.id;
                        return (
                          <Pressable
                            key={provider.id}
                            style={[
                              styles.providerCard,
                              isSelected && styles.providerCardActive,
                              !provider.enabled && styles.providerCardDisabled,
                            ]}
                            onPress={() =>
                              provider.enabled && setEditSelectedProvider(provider.id)
                            }
                            disabled={!provider.enabled}
                          >
                            <Text
                              style={[
                                styles.providerName,
                                isSelected && styles.providerNameActive,
                              ]}
                              numberOfLines={1}
                            >
                              {provider.label}
                            </Text>
                            <Text style={styles.providerDesc} numberOfLines={1}>
                              {provider.status} · {provider.enabled ? "enabled" : "disabled"}
                              {provider.description ? ` · ${provider.description}` : ""}
                            </Text>
                          </Pressable>
                        );
                      })}
                    </View>
                  )}
                </View>

                <View style={styles.field}>
                  <Text style={styles.label}>Model</Text>
                  <AdaptiveTextInput
                    style={styles.input}
                    placeholder="e.g. claude-3-opus (optional)"
                    value={editModel}
                    onChangeText={setEditModel}
                    placeholderTextColor={theme.colors.foregroundMuted}
                  />
                </View>

                <View style={styles.field}>
                  <Text style={styles.label}>System Prompt</Text>
                  <AdaptiveTextInput
                    style={[styles.input, styles.textArea]}
                    placeholder="Optional instructions for the agent"
                    value={editSystemPrompt}
                    onChangeText={setEditSystemPrompt}
                    multiline
                    numberOfLines={3}
                    placeholderTextColor={theme.colors.foregroundMuted}
                  />
                </View>
              </View>
            ) : null}
          </View>

          <Button onPress={handleSaveEdit} disabled={isUpdating}>
            Save
          </Button>
        </View>
      </AdaptiveModalSheet>

      <AdaptiveModalSheet
        title="Delete Loop"
        visible={isDeleteModalVisible}
        onClose={handleCloseDelete}
      >
        <View style={styles.modalContent}>
          <Text style={styles.confirmText}>
            Are you sure you want to delete this loop? This cannot be undone.
          </Text>
          <View style={styles.confirmButtons}>
            <Button variant="ghost" onPress={handleCloseDelete}>
              Cancel
            </Button>
            <Button variant="destructive" onPress={handleConfirmDelete} disabled={isDeleting}>
              Delete
            </Button>
          </View>
        </View>
      </AdaptiveModalSheet>

      <AdaptiveModalSheet
        title="Delete Instance"
        visible={isInstanceDeleteModalVisible}
        onClose={handleCloseInstanceDelete}
      >
        <View style={styles.modalContent}>
          <Text style={styles.confirmText}>
            Are you sure you want to delete this instance? This cannot be undone.
          </Text>
          <View style={styles.confirmButtons}>
            <Button variant="ghost" onPress={handleCloseInstanceDelete}>
              Cancel
            </Button>
            <Button variant="destructive" onPress={handleConfirmInstanceDelete} disabled={isDeleting}>
              Delete
            </Button>
          </View>
        </View>
      </AdaptiveModalSheet>
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
  headerActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
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
    flexDirection: "row",
    alignItems: "center",
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[3],
    gap: theme.spacing[2],
  },
  iterationContent: {
    flex: 1,
    gap: theme.spacing[2],
  },
  instanceDeleteButton: {
    padding: theme.spacing[2],
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
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[4],
  },
  modalContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[4],
  },
  field: {
    gap: theme.spacing[2],
  },
  label: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontWeight: "500",
  },
  input: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.md,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[3],
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
  },
  textArea: {
    minHeight: 80,
    textAlignVertical: "top",
  },
  confirmText: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
  },
  confirmButtons: {
    flexDirection: "row",
    justifyContent: "flex-end",
    gap: theme.spacing[3],
  },
  sectionContent: {
    gap: theme.spacing[4],
  },
  helperText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  providerList: {
    gap: theme.spacing[2],
  },
  providerCard: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[3],
    borderWidth: 1,
    borderColor: theme.colors.border,
  },
  providerCardActive: {
    borderColor: theme.colors.accent,
    backgroundColor: theme.colors.surface3,
  },
  providerCardDisabled: {
    opacity: 0.5,
  },
  providerName: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: "500",
  },
  providerNameActive: {
    color: theme.colors.accent,
  },
  providerDesc: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    marginTop: theme.spacing[1],
  },
}));

import { useCallback, useMemo, useState } from "react";
import { View, Text, ScrollView } from "react-native";
import { useIsFocused } from "@react-navigation/native";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ArrowLeft, Loader, Pencil, Trash2 } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { AdaptiveModalSheet, AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { useLoopInspect } from "@/hooks/use-loop-inspect";
import { useLoopMutations } from "@/hooks/use-loop-mutations";
import { buildHostLoopsRoute } from "@/utils/host-routes";

export function LoopDetailScreen({ serverId, loopId }: { serverId: string; loopId: string }) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <LoopDetailScreenContent serverId={serverId} loopId={loopId} />;
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
  const { loop, isLoading, error } = useLoopInspect({ serverId, loopId });
  const { updateLoop, deleteLoop, isUpdating, isDeleting } = useLoopMutations({ serverId });

  const [isEditModalVisible, setIsEditModalVisible] = useState(false);
  const [editName, setEditName] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editCwd, setEditCwd] = useState("");
  const [editVerifyChecks, setEditVerifyChecks] = useState("");
  const [editMaxIterations, setEditMaxIterations] = useState("");
  const [isDeleteModalVisible, setIsDeleteModalVisible] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const handleBack = useCallback(() => {
    router.navigate(buildHostLoopsRoute(serverId));
  }, [serverId]);

  const handleOpenEdit = useCallback(() => {
    setEditName(loop?.name ?? "");
    setEditPrompt(loop?.prompt ?? "");
    setEditCwd(loop?.cwd ?? "");
    setEditVerifyChecks(loop?.verifyChecks?.join("\n") ?? "");
    setEditMaxIterations(loop?.maxIterations?.toString() ?? "");
    setEditError(null);
    setIsEditModalVisible(true);
  }, [loop?.name, loop?.prompt, loop?.cwd, loop?.verifyChecks, loop?.maxIterations]);

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

    try {
      await updateLoop({
        loopId,
        name: editName.trim() || null,
        prompt: trimmedPrompt,
        cwd: trimmedCwd,
        verifyChecks: checks.length > 0 ? checks : null,
        maxIterations: maxIter,
      });
      setIsEditModalVisible(false);
    } catch (e) {
      setEditError(e instanceof Error ? e.message : "Failed to update loop");
    }
  }, [updateLoop, loopId, editName, editPrompt, editCwd, editVerifyChecks, editMaxIterations]);

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
    } catch {
      // Error surfaced by mutation
    }
  }, [deleteLoop, loopId, serverId]);

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

  if (!loop) {
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

  const name = loop.name ?? "Untitled";

  return (
    <View style={styles.container}>
      <BackHeader
        title={name}
        onBack={handleBack}
        rightContent={
          <View style={styles.headerActions}>
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
        <View style={styles.section}>
          <View style={styles.sectionHeader}>
            <Text style={styles.sectionTitle} testID="loop-detail-name">
              {name}
            </Text>
            <LoopStatusBadge status={loop.status} />
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Status:</Text>
            <Text style={styles.detailValue} testID="loop-detail-status">
              {loop.status}
            </Text>
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Prompt</Text>
            <Text style={styles.detailValue}>{loop.prompt}</Text>
          </View>

          <View style={styles.detailRow}>
            <Text style={styles.detailLabel}>Working Directory</Text>
            <Text style={styles.detailValue}>{loop.cwd}</Text>
          </View>

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
        </View>

        <View style={styles.section}>
          <Text style={styles.sectionTitle}>Iterations</Text>
          {loop.iterations.length === 0 ? (
            <Text style={styles.emptyText}>No iterations yet</Text>
          ) : (
            loop.iterations.map((iteration) => (
              <View key={iteration.index} style={styles.iterationCard}>
                <View style={styles.iterationHeader}>
                  <Text style={styles.iterationTitle}>Iteration {iteration.index}</Text>
                  <LoopStatusBadge status={iteration.status} />
                </View>
                {iteration.failureReason ? (
                  <Text style={styles.errorText}>{iteration.failureReason}</Text>
                ) : null}
              </View>
            ))
          )}
        </View>
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
}));

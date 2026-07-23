import { useCallback, useMemo, useState } from "react";
import { View, Text, ScrollView, RefreshControl, Pressable } from "react-native";
import { useIsFocused } from "@/hooks/use-is-focused";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ArrowLeft, Loader, Plus, Trash2 } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useLoopTemplates } from "@/hooks/use-loop-templates";
import { useLoopMutations } from "@/hooks/use-loop-mutations";
import { buildHostLoopCreateRoute, buildHostLoopDetailRoute, buildHostOpenProjectRoute } from "@/utils/host-routes";
import type { LoopTemplateSummary } from "@server/server/loop/rpc-schemas";

export function LoopsScreen({ serverId }: { serverId: string }) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <LoopsScreenContent serverId={serverId} />;
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

function LoopCard({
  serverId,
  template,
  onDelete,
  isDeleting,
}: {
  serverId: string;
  template: LoopTemplateSummary;
  onDelete: (id: string) => void;
  isDeleting: boolean;
}) {
  const { theme } = useUnistyles();
  const name = template.name || "Untitled";

  const handleOpenDetail = useCallback(() => {
    router.navigate(buildHostLoopDetailRoute(serverId, template.id));
  }, [serverId, template.id]);

  return (
    <Pressable style={styles.card} onPress={handleOpenDetail}>
      <View style={styles.cardHeader}>
        <View style={styles.cardTitleRow}>
          <Text style={styles.cardTitle} numberOfLines={1}>
            {name}
          </Text>
          <View style={styles.instanceCountBadge}>
            <Text style={styles.instanceCountText}>{template.instanceCount}</Text>
          </View>
        </View>
        {template.latestStatus ? (
          <LoopStatusBadge status={template.latestStatus} />
        ) : null}
      </View>

      <View style={styles.cardMetaRow}>
        <Text style={styles.cardMetaText}>{template.cwd}</Text>
        {template.provider ? (
          <Text style={styles.cardMetaText}>
            · {template.provider}
            {template.model ? ` / ${template.model}` : ""}
          </Text>
        ) : null}
      </View>

      <View style={styles.cardActions}>
        <Pressable
          style={styles.actionButton}
          onPress={() => onDelete(template.id)}
          disabled={isDeleting}
        >
          <Trash2 size={16} color={theme.colors.palette.red[500]} />
          <Text style={[styles.actionText, { color: theme.colors.palette.red[500] }]}>
            {isDeleting ? "Deleting..." : "Delete"}
          </Text>
        </Pressable>
      </View>
    </Pressable>
  );
}

function LoopsScreenContent({ serverId }: { serverId: string }) {
  const { theme } = useUnistyles();
  const { templates, isInitialLoad, isRevalidating, error, refreshAll } = useLoopTemplates({ serverId });
  const { deleteTemplate, isDeletingTemplate } = useLoopMutations({ serverId });

  const [isManualRefresh, setIsManualRefresh] = useState(false);

  const handleRefresh = useCallback(() => {
    setIsManualRefresh(true);
    refreshAll();
  }, [refreshAll]);

  const [prevRevalidating, setPrevRevalidating] = useState(isRevalidating);
  if (prevRevalidating !== isRevalidating) {
    setPrevRevalidating(isRevalidating);
    if (!isRevalidating && isManualRefresh) {
      setIsManualRefresh(false);
    }
  }

  const handleBack = useCallback(() => {
    router.navigate(buildHostOpenProjectRoute(serverId));
  }, [serverId]);

  const handleCreate = useCallback(() => {
    router.push(buildHostLoopCreateRoute(serverId));
  }, [serverId]);

  const handleDelete = useCallback(
    async (id: string) => {
      try {
        await deleteTemplate(id);
      } catch {
        // Error is surfaced by mutation
      }
    },
    [deleteTemplate],
  );

  return (
    <View style={styles.container}>
      <BackHeader
        title="Loops"
        onBack={() => router.navigate("/")}
        rightContent={
          <Button variant="ghost" size="sm" leftIcon={Plus} onPress={handleCreate}>
            New Loop
          </Button>
        }
      />

      {isInitialLoad ? (
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      ) : null}

      {!isInitialLoad && templates.length === 0 ? (
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>No loops found.</Text>
          <Button variant="ghost" leftIcon={ArrowLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      ) : null}

      {!isInitialLoad && templates.length > 0 ? (
        <ScrollView
          style={styles.list}
          contentContainerStyle={styles.listContent}
          refreshControl={
            <RefreshControl
              refreshing={isManualRefresh && isRevalidating}
              onRefresh={handleRefresh}
              tintColor={theme.colors.foregroundMuted}
            />
          }
        >
          {error ? (
            <View style={styles.errorBanner}>
              <Text style={styles.errorText}>{error}</Text>
            </View>
          ) : null}
          {templates.map((template) => (
            <LoopCard
              key={template.id}
              serverId={serverId}
              template={template}
              onDelete={handleDelete}
              isDeleting={isDeletingTemplate}
            />
          ))}
        </ScrollView>
      ) : null}
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
    fontSize: theme.fontSize.lg,
  },
  list: {
    flex: 1,
  },
  listContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  card: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    padding: theme.spacing[4],
    gap: theme.spacing[3],
  },
  cardHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: theme.spacing[2],
  },
  cardTitleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    flexShrink: 1,
  },
  cardTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.lg,
    fontWeight: "600",
    flexShrink: 1,
  },
  instanceCountBadge: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    paddingHorizontal: theme.spacing[2],
    paddingVertical: theme.spacing[1],
    minWidth: 20,
    alignItems: "center",
  },
  instanceCountText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    fontWeight: "500",
  },
  cardMetaRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
  },
  cardMetaText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  cardActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[4],
    paddingTop: theme.spacing[2],
    borderTopWidth: theme.borderWidth[1],
    borderTopColor: theme.colors.border,
  },
  actionButton: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
    paddingVertical: theme.spacing[2],
  },
  actionText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
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
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[4],
    marginBottom: theme.spacing[3],
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
}));

import { useCallback, useEffect } from "react";
import { View, Text, ScrollView, RefreshControl } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { router } from "expo-router";
import { useIsFocused } from "@/hooks/use-is-focused";
import { Gauge, LayoutDashboard, RefreshCw } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { useAggregatedUsage } from "@/hooks/use-aggregated-usage";
import { useIsCompactFormFactor } from "@/constants/layout";
import { UsageProviderCard, UsageProviderErrorCard } from "./usage-provider-card";

export function UsageDashboardScreen() {
  const { theme } = useUnistyles();
  const insets = useSafeAreaInsets();
  const isCompact = useIsCompactFormFactor();
  const isFocused = useIsFocused();
  const {
    groups,
    configuredHosts,
    isInitialLoad,
    isRevalidating,
    error,
    refreshAll,
  } = useAggregatedUsage();

  useEffect(() => {
    if (isFocused) {
      refreshAll();
    }
  }, [isFocused, refreshAll]);

  const handleRefresh = useCallback(() => {
    refreshAll({ forceRefresh: true });
  }, [refreshAll]);

  const hasSnapshots = groups.some((group) => group.snapshots.length > 0);
  const hasProviderErrors = groups.some((group) => Object.keys(group.errors).length > 0);

  return (
    <View style={styles.container}>
      <BackHeader
        title="Usage"
        onBack={() => router.navigate("/")}
        rightContent={
          <View style={styles.headerRight}>
            <Button
              variant="ghost"
              size="sm"
              leftIcon={RefreshCw}
              onPress={handleRefresh}
              testID="usage-refresh-button"
            >
              Refresh
            </Button>
          </View>
        }
      />

      <ScrollView
        style={styles.scrollView}
        contentContainerStyle={[styles.content, { paddingBottom: insets.bottom + 16 }]}
        refreshControl={
          <RefreshControl
            refreshing={isRevalidating}
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

        {isInitialLoad ? (
          <View style={styles.loadingContainer}>
            <Gauge size={32} color={theme.colors.foregroundMuted} />
            <Text style={styles.loadingText}>Loading usage...</Text>
          </View>
        ) : !hasSnapshots && !hasProviderErrors ? (
          <View style={styles.emptyContainer}>
            <LayoutDashboard size={48} color={theme.colors.foregroundMuted} />
            {configuredHosts.length === 0 ? (
              <>
                <Text style={styles.emptyTitle}>No hosts configured</Text>
                <Text style={styles.emptySubtitle}>
                  Add a host to see usage dashboards
                </Text>
              </>
            ) : configuredHosts.every((host) => !host.isConnected) ? (
              <>
                <Text style={styles.emptyTitle}>Hosts are offline</Text>
                <Text style={styles.emptySubtitle}>
                  Connect to a host to see usage dashboards
                </Text>
              </>
            ) : (
              <>
                <Text style={styles.emptyTitle}>No usage providers configured</Text>
                <Text style={styles.emptySubtitle}>
                  Run solo-usage init on your host, then edit
                  ~/.solo/usage.json
                </Text>
              </>
            )}
          </View>
        ) : (
          <View style={isCompact ? styles.usageGridCompact : styles.usageGrid}>
            {groups.flatMap((group) => {
              const snapshotProviderIds = new Set(
                group.snapshots.map((snapshot) => snapshot.provider),
              );
              const cards = group.snapshots.map((snapshot) => (
                <UsageProviderCard
                  key={`${group.serverId}:${snapshot.provider}`}
                  snapshot={snapshot}
                  hostLabel={group.serverLabel}
                />
              ));
              for (const [provider, message] of Object.entries(group.errors)) {
                if (snapshotProviderIds.has(provider)) {
                  continue;
                }
                cards.push(
                  <UsageProviderErrorCard
                    key={`${group.serverId}:${provider}:error`}
                    provider={provider}
                    message={message}
                    hostLabel={group.serverLabel}
                  />,
                );
              }
              return cards;
            })}
          </View>
        )}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  scrollView: {
    flex: 1,
  },
  content: {
    padding: 16,
    gap: 16,
  },
  headerRight: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
  },
  usageGrid: {
    flexDirection: "row",
    flexWrap: "wrap",
    gap: 12,
  },
  usageGridCompact: {
    gap: 12,
  },
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: 12,
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
  loadingContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 12,
    paddingVertical: 60,
  },
  loadingText: {
    fontSize: theme.fontSize.base,
    color: theme.colors.foregroundMuted,
  },
  emptyContainer: {
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    gap: 12,
    paddingVertical: 60,
  },
  emptyTitle: {
    fontSize: theme.fontSize.lg,
    fontWeight: theme.fontWeight.semibold,
    color: theme.colors.foreground,
  },
  emptySubtitle: {
    fontSize: theme.fontSize.sm,
    color: theme.colors.foregroundMuted,
    textAlign: "center",
  },
}));

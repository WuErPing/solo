import { useCallback, useState } from "react";
import { View, Text, ScrollView, RefreshControl } from "react-native";
import { useIsFocused } from "@/hooks/use-is-focused";
import { router } from "expo-router";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { ChevronLeft, RefreshCw } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { LoadingSpinner } from "@/components/ui/loading-spinner";
import { useUsage } from "@/hooks/use-usage";
import { buildHostOpenProjectRoute } from "@/utils/host-routes";
import { UsageProviderCard, UsageProviderErrorCard } from "./usage-provider-card";

export function UsageScreen({ serverId }: { serverId: string }) {
  const isFocused = useIsFocused();

  if (!isFocused) {
    return <View style={styles.container} />;
  }

  return <UsageScreenContent serverId={serverId} />;
}

function UsageScreenContent({ serverId }: { serverId: string }) {
  const { theme } = useUnistyles();
  const { snapshots, errors, isInitialLoad, isRevalidating, error, refreshAll } = useUsage({
    serverId,
  });

  const [isManualRefresh, setIsManualRefresh] = useState(false);

  const handleRefresh = useCallback(() => {
    setIsManualRefresh(true);
    refreshAll({ forceRefresh: true });
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

  const hasData = snapshots.length > 0 || Object.keys(errors).length > 0;

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

      {isInitialLoad ? (
        <View style={styles.loadingContainer}>
          <LoadingSpinner size="large" color={theme.colors.foregroundMuted} />
        </View>
      ) : null}

      {!isInitialLoad && !hasData ? (
        <View style={styles.emptyContainer}>
          <Text style={styles.emptyText}>No usage providers configured</Text>
          <Text style={styles.emptySubtext}>
            Configure usage providers in ~/.solo/usage.json on this host
          </Text>
          <Button variant="ghost" leftIcon={ChevronLeft} onPress={handleBack}>
            Back
          </Button>
        </View>
      ) : null}

      {!isInitialLoad && hasData ? (
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
          {snapshots.map((snapshot) => (
            <UsageProviderCard key={snapshot.provider} snapshot={snapshot} />
          ))}
          {Object.entries(errors)
            .filter(([provider]) => !snapshots.some((s) => s.provider === provider))
            .map(([provider, message]) => (
              <UsageProviderErrorCard
                key={`${provider}:error`}
                provider={provider}
                message={message}
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
  headerRight: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[2],
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
    gap: theme.spacing[4],
    padding: theme.spacing[6],
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.lg,
  },
  emptySubtext: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
  list: {
    flex: 1,
  },
  listContent: {
    padding: theme.spacing[4],
    gap: theme.spacing[3],
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

import { useCallback, useState } from "react";
import { View, Text, ScrollView } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { router } from "expo-router";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { useCreateLoop } from "@/hooks/use-create-loop";
import { buildHostLoopDetailRoute } from "@/utils/host-routes";

export function LoopCreateScreen({ serverId }: { serverId: string }) {
  const { theme } = useUnistyles();
  const { createLoop, isCreating } = useCreateLoop({ serverId });

  const [name, setName] = useState("");
  const [prompt, setPrompt] = useState("");
  const [cwd, setCwd] = useState("");
  const [verifyCheck, setVerifyCheck] = useState("");
  const [maxIterations, setMaxIterations] = useState("10");
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = useCallback(async () => {
    setError(null);
    if (!prompt.trim()) {
      setError("Prompt is required");
      return;
    }
    if (!cwd.trim()) {
      setError("Working directory is required");
      return;
    }

    const iterations = parseInt(maxIterations.trim(), 10);
    if (Number.isNaN(iterations) || iterations <= 0) {
      setError("Max iterations must be a positive number");
      return;
    }

    try {
      const loop = await createLoop({
        prompt: prompt.trim(),
        cwd: cwd.trim(),
        name: name.trim() || null,
        verifyChecks: verifyCheck.trim() ? [verifyCheck.trim()] : undefined,
        maxIterations: iterations,
      });
      router.replace(buildHostLoopDetailRoute(serverId, loop.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create loop");
    }
  }, [prompt, cwd, name, verifyCheck, maxIterations, createLoop, serverId]);

  return (
    <View style={styles.container}>
      <BackHeader title="New Loop" onBack={() => router.back()} />
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        {error ? (
          <View style={styles.errorBanner}>
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}

        <View style={styles.field}>
          <Text style={styles.label}>Name</Text>
          <AdaptiveTextInput
            style={styles.input}
            placeholder="e.g. Fix CI"
            value={name}
            onChangeText={setName}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Prompt *</Text>
          <AdaptiveTextInput
            style={[styles.input, styles.textArea]}
            placeholder="e.g. Fix all test failures until CI passes"
            value={prompt}
            onChangeText={setPrompt}
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
            value={cwd}
            onChangeText={setCwd}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Verify Check</Text>
          <AdaptiveTextInput
            style={styles.input}
            placeholder="e.g. go test ./..."
            value={verifyCheck}
            onChangeText={setVerifyCheck}
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <View style={styles.field}>
          <Text style={styles.label}>Max Iterations</Text>
          <AdaptiveTextInput
            style={styles.input}
            placeholder="10"
            value={maxIterations}
            onChangeText={setMaxIterations}
            keyboardType="numeric"
            placeholderTextColor={theme.colors.foregroundMuted}
          />
        </View>

        <Button onPress={handleSubmit} disabled={isCreating}>
          Create Loop
        </Button>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  scroll: {
    flex: 1,
  },
  content: {
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
  errorBanner: {
    backgroundColor: theme.colors.surface2,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[4],
  },
  errorText: {
    color: theme.colors.palette.red[500],
    fontSize: theme.fontSize.sm,
  },
}));

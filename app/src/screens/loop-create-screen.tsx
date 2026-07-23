import { useCallback, useEffect, useMemo, useState } from "react";
import { View, Text, ScrollView, Pressable } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { router } from "expo-router";
import { ChevronDown, ChevronRight } from "lucide-react-native";
import { BackHeader } from "@/components/headers/back-header";
import { Button } from "@/components/ui/button";
import { AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { useCreateLoop } from "@/hooks/use-create-loop";
import { useProvidersSnapshot } from "@/hooks/use-providers-snapshot";
import { buildHostLoopDetailRoute } from "@/utils/host-routes";
import type { AgentProvider } from "@server/server/agent/agent-sdk-types";

// Permission modes from most to least autonomous, mirroring the daemon's loop
// worker default (daemon/internal/agent/template.go). A loop runs unattended
// with no one to answer permission prompts, so the worker should default to the
// most autonomous mode the selected provider offers.
const LOOP_AUTONOMY_MODE_PREFERENCE = [
  "bypassPermissions",
  "full-access",
  "build",
  "auto",
  "default",
];

export function LoopCreateScreen({ serverId }: { serverId: string }) {
  const { theme } = useUnistyles();
  const { createLoop, isCreating } = useCreateLoop({ serverId });
  const { entries: providers, isLoading: isProvidersLoading } = useProvidersSnapshot(serverId, {
    enabled: true,
  });

  const [name, setName] = useState("");
  const [prompt, setPrompt] = useState("");
  const [cwd, setCwd] = useState("");
  const [verifyCheck, setVerifyCheck] = useState("");
  const [maxIterations, setMaxIterations] = useState("10");
  const [isAgentOpen, setIsAgentOpen] = useState(false);
  const [selectedProvider, setSelectedProvider] = useState<string | null>(null);
  const [selectedMode, setSelectedMode] = useState<string | null>(null);
  const [model, setModel] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [error, setError] = useState<string | null>(null);

  const providerOptions = useMemo(() => {
    return (providers ?? []).map((provider) => ({
      id: provider.provider,
      label: provider.label || provider.provider,
      description: provider.description || "",
      status: provider.status,
      enabled: provider.enabled,
    }));
  }, [providers]);

  const selectedProviderEntry = useMemo(
    () => (providers ?? []).find((p) => p.provider === selectedProvider) ?? null,
    [providers, selectedProvider],
  );
  const modeOptions = useMemo(
    () => selectedProviderEntry?.modes ?? [],
    [selectedProviderEntry],
  );

  // Loops run unattended with no one to answer permission prompts, so default
  // the worker to the most autonomous mode the provider offers (mirrors the
  // daemon's loop default). A valid current selection is preserved.
  useEffect(() => {
    if (!selectedProvider || modeOptions.length === 0) {
      return;
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- auto-select mode when provider/options change
    setSelectedMode((current) => {
      if (current && modeOptions.some((m) => m.id === current)) {
        return current;
      }
      const autonomous = LOOP_AUTONOMY_MODE_PREFERENCE.find((pref) =>
        modeOptions.some((m) => m.id === pref),
      );
      return autonomous ?? selectedProviderEntry?.defaultModeId ?? modeOptions[0]?.id ?? null;
    });
  }, [selectedProvider, modeOptions, selectedProviderEntry]);

  const agentTemplate = useMemo(() => {
    if (!selectedProvider) {
      return null;
    }
    return {
      provider: selectedProvider as AgentProvider,
      cwd: cwd.trim() || "/",
      model: model.trim() || undefined,
      systemPrompt: systemPrompt.trim() || undefined,
      modeId: selectedMode ?? undefined,
    };
  }, [selectedProvider, cwd, model, systemPrompt, selectedMode]);

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

    if (isAgentOpen && !selectedProvider) {
      setError("Please select an agent provider");
      return;
    }

    try {
      const loop = await createLoop({
        prompt: prompt.trim(),
        cwd: cwd.trim(),
        name: name.trim() || null,
        verifyChecks: verifyCheck.trim() ? [verifyCheck.trim()] : undefined,
        maxIterations: iterations,
        agentTemplate,
      });
      router.replace(buildHostLoopDetailRoute(serverId, loop.id));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to create loop");
    }
  }, [
    prompt,
    cwd,
    name,
    verifyCheck,
    maxIterations,
    isAgentOpen,
    selectedProvider,
    agentTemplate,
    createLoop,
    serverId,
  ]);

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
                      const isSelected = selectedProvider === provider.id;
                      return (
                        <Pressable
                          key={provider.id}
                          style={[
                            styles.providerCard,
                            isSelected && styles.providerCardActive,
                            !provider.enabled && styles.providerCardDisabled,
                          ]}
                          onPress={() =>
                            provider.enabled && setSelectedProvider(provider.id)
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

              {modeOptions.length > 0 ? (
                <View style={styles.field}>
                  <Text style={styles.label}>Permission Mode</Text>
                  <Text style={styles.helperText}>
                    Loops run unattended. This defaults to the most autonomous
                    mode the provider offers so the agent can apply edits and run
                    actions without prompts.
                  </Text>
                  <View style={styles.providerList}>
                    {modeOptions.map((mode) => {
                      const isSelected = selectedMode === mode.id;
                      return (
                        <Pressable
                          key={mode.id}
                          style={[
                            styles.providerCard,
                            isSelected && styles.providerCardActive,
                          ]}
                          onPress={() => setSelectedMode(mode.id)}
                        >
                          <Text
                            style={[
                              styles.providerName,
                              isSelected && styles.providerNameActive,
                            ]}
                            numberOfLines={1}
                          >
                            {mode.label}
                          </Text>
                          {mode.description ? (
                            <Text style={styles.providerDesc} numberOfLines={1}>
                              {mode.description}
                            </Text>
                          ) : null}
                        </Pressable>
                      );
                    })}
                  </View>
                </View>
              ) : null}

              <View style={styles.field}>
                <Text style={styles.label}>Model</Text>
                <AdaptiveTextInput
                  style={styles.input}
                  placeholder="e.g. claude-3-opus (optional)"
                  value={model}
                  onChangeText={setModel}
                  placeholderTextColor={theme.colors.foregroundMuted}
                />
              </View>

              <View style={styles.field}>
                <Text style={styles.label}>System Prompt</Text>
                <AdaptiveTextInput
                  style={[styles.input, styles.textArea]}
                  placeholder="Optional instructions for the agent"
                  value={systemPrompt}
                  onChangeText={setSystemPrompt}
                  multiline
                  numberOfLines={3}
                  placeholderTextColor={theme.colors.foregroundMuted}
                />
              </View>
            </View>
          ) : null}
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
  section: {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.md,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
  },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    padding: theme.spacing[4],
  },
  sectionTitle: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.base,
    fontWeight: "600",
  },
  sectionContent: {
    padding: theme.spacing[4],
    paddingTop: 0,
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

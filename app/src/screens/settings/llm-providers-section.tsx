import { useCallback, useEffect, useMemo, useState } from "react";
import { Alert, Pressable, Switch, Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { Pencil, Plus, Trash2 } from "lucide-react-native";
import { AdaptiveModalSheet, AdaptiveTextInput } from "@/components/adaptive-modal-sheet";
import { Button } from "@/components/ui/button";
import { useDaemonConfig } from "@/hooks/use-daemon-config";
import { useHostRuntimeIsConnected } from "@/runtime/host-runtime";
import { SettingsSection } from "@/screens/settings/settings-section";
import { settingsStyles } from "@/styles/settings";
import type { LLMProviderConfig } from "@server/shared/messages";

interface LLMProviderModalProps {
  provider?: LLMProviderConfig;
  visible: boolean;
  onSave: (provider: LLMProviderConfig) => void;
  onClose: () => void;
}

function LLMProviderModal({ provider, visible, onSave, onClose }: LLMProviderModalProps) {
  const { theme } = useUnistyles();
  const isEditing = provider !== undefined;
  const [id, setId] = useState(provider?.id ?? "");
  const [label, setLabel] = useState(provider?.label ?? "");
  const [description, setDescription] = useState(provider?.description ?? "");
  const [baseURL, setBaseURL] = useState(provider?.baseURL ?? "");
  const [apiKey, setApiKey] = useState(provider?.apiKey ?? "");
  const [modelsText, setModelsText] = useState((provider?.models ?? []).map((m) => m.id).join(", "));

  useEffect(() => {
    if (!visible) {
      return;
    }
    setId(provider?.id ?? "");
    setLabel(provider?.label ?? "");
    setDescription(provider?.description ?? "");
    setBaseURL(provider?.baseURL ?? "");
    setApiKey(provider?.apiKey ?? "");
    setModelsText((provider?.models ?? []).map((m) => m.id).join(", "));
  }, [visible, provider]);

  const trimmedId = id.trim();
  const canSubmit = trimmedId.length > 0;

  const handleSubmit = useCallback(() => {
    if (!canSubmit) {
      return;
    }
    const existingModelsById = new Map((provider?.models ?? []).map((m) => [m.id, m]));
    const models = modelsText
      .split(",")
      .map((entry) => entry.trim())
      .filter((entry) => entry.length > 0)
      .map((modelId) => ({ ...existingModelsById.get(modelId), id: modelId }));
    if (models.length > 0 && !models.some((m) => m.isDefault)) {
      models[0] = { ...models[0], isDefault: true };
    }
    onSave({
      id: trimmedId,
      label: label.trim() || trimmedId,
      description: description.trim(),
      enabled: provider?.enabled ?? true,
      baseURL: baseURL.trim(),
      apiKey: apiKey.trim(),
      models,
    });
  }, [canSubmit, trimmedId, label, description, baseURL, apiKey, modelsText, provider, onSave]);

  const inputStyle = useMemo(
    () => [styles.formInput, { color: theme.colors.foreground }],
    [theme.colors.foreground],
  );

  const renderField = (
    fieldLabel: string,
    value: string,
    onChangeText: (text: string) => void,
    placeholder: string,
    testID: string,
    options?: { editable?: boolean; secureTextEntry?: boolean; hint?: string },
  ) => (
    <View style={styles.fieldColumn}>
      <Text style={styles.fieldLabel}>{fieldLabel}</Text>
      <AdaptiveTextInput
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={theme.colors.foregroundMuted}
        autoCapitalize="none"
        autoCorrect={false}
        editable={options?.editable}
        secureTextEntry={options?.secureTextEntry}
        style={[inputStyle, options?.editable === false && styles.disabledInput]}
        testID={testID}
      />
      {options?.hint ? <Text style={styles.fieldHint}>{options.hint}</Text> : null}
    </View>
  );

  return (
    <AdaptiveModalSheet
      title={isEditing ? "Edit LLM provider" : "Add LLM provider"}
      visible={visible}
      onClose={onClose}
      testID="llm-provider-modal"
      desktopMaxWidth={480}
    >
      <View style={styles.form} testID="llm-provider-form">
        {renderField("Provider ID", id, setId, "Provider ID", "llm-provider-id", {
          editable: !isEditing,
        })}
        {renderField("Label", label, setLabel, "Label", "llm-provider-label")}
        {renderField("Description", description, setDescription, "Description", "llm-provider-description")}
        {renderField("Base URL", baseURL, setBaseURL, "Base URL", "llm-provider-baseurl")}
        {renderField("API Key", apiKey, setApiKey, "API Key", "llm-provider-apikey", {
          secureTextEntry: true,
        })}
        {renderField("Models", modelsText, setModelsText, "deepseek-chat, deepseek-reasoner", "llm-provider-models", {
          hint: "Comma-separated model IDs. The default model is used by the Schedule Assistant.",
        })}
      </View>
      <View style={styles.formActions}>
        <Button variant="outline" size="sm" onPress={onClose} testID="llm-provider-cancel">
          Cancel
        </Button>
        <Button
          variant="default"
          size="sm"
          onPress={handleSubmit}
          disabled={!canSubmit}
          testID="llm-provider-submit"
        >
          {isEditing ? "Save changes" : "Add provider"}
        </Button>
      </View>
    </AdaptiveModalSheet>
  );
}

interface LLMProviderRowProps {
  provider: LLMProviderConfig;
  onEdit: (provider: LLMProviderConfig) => void;
  onDelete: (providerId: string) => void;
  onToggleEnabled: (providerId: string, enabled: boolean) => void;
  isFirst: boolean;
}

function LLMProviderRow({
  provider,
  onEdit,
  onDelete,
  onToggleEnabled,
  isFirst,
}: LLMProviderRowProps) {
  const { theme } = useUnistyles();
  const modelCount = provider.models?.length ?? 0;
  const enabled = provider.enabled ?? true;

  const handleEdit = useCallback(() => {
    onEdit(provider);
  }, [onEdit, provider]);

  const handleDelete = useCallback(() => {
    onDelete(provider.id);
  }, [onDelete, provider.id]);

  const handleToggle = useCallback(
    (value: boolean) => {
      onToggleEnabled(provider.id, value);
    },
    [onToggleEnabled, provider.id],
  );

  return (
    <View style={[settingsStyles.row, !isFirst && settingsStyles.rowBorder]}>
      <View style={styles.rowContent}>
        <Text style={settingsStyles.rowTitle} numberOfLines={1}>
          {provider.label || provider.id}
        </Text>
        {provider.baseURL ? (
          <Text style={styles.rowHint} numberOfLines={1}>
            {provider.baseURL}
          </Text>
        ) : null}
        {modelCount > 0 ? (
          <Text style={styles.rowHint}>
            {modelCount === 1 ? "1 model" : `${modelCount} models`}
          </Text>
        ) : null}
      </View>
      <View style={styles.rowActions}>
        <Switch
          value={enabled}
          onValueChange={handleToggle}
          testID={`llm-provider-switch-${provider.id}`}
        />
        <Pressable
          onPress={handleEdit}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Edit ${provider.label || provider.id}`}
          testID={`edit-llm-provider-${provider.id}`}
        >
          <Pencil size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
        </Pressable>
        <Pressable
          onPress={handleDelete}
          hitSlop={8}
          accessibilityRole="button"
          accessibilityLabel={`Delete ${provider.label || provider.id}`}
          testID={`delete-llm-provider-${provider.id}`}
        >
          <Trash2 size={theme.iconSize.sm} color={theme.colors.destructive} />
        </Pressable>
      </View>
    </View>
  );
}

export interface LlmProvidersSectionProps {
  serverId: string;
}

export function LlmProvidersSection({ serverId }: LlmProvidersSectionProps) {
  const { theme } = useUnistyles();
  const isConnected = useHostRuntimeIsConnected(serverId);
  const { config, patchConfig } = useDaemonConfig(serverId);
  const [editingProvider, setEditingProvider] = useState<LLMProviderConfig | null | undefined>(
    undefined,
  );

  const providers = useMemo(() => config?.llmProviders ?? [], [config?.llmProviders]);
  const hasServer = serverId.length > 0;
  const isModalOpen = editingProvider !== undefined;

  const saveProviders = useCallback(
    async (nextProviders: LLMProviderConfig[]) => {
      try {
        await patchConfig({ llmProviders: nextProviders });
      } catch (error) {
        Alert.alert(
          "Unable to save LLM providers",
          error instanceof Error ? error.message : String(error),
        );
      }
    },
    [patchConfig],
  );

  const handleAdd = useCallback(() => {
    setEditingProvider(null);
  }, []);

  const handleClose = useCallback(() => {
    setEditingProvider(undefined);
  }, []);

  const handleSave = useCallback(
    (provider: LLMProviderConfig) => {
      const existingIndex = providers.findIndex((p) => p.id === provider.id);
      const nextProviders =
        existingIndex >= 0
          ? providers.map((p, index) => (index === existingIndex ? provider : p))
          : [...providers, provider];
      void saveProviders(nextProviders);
      setEditingProvider(undefined);
    },
    [providers, saveProviders],
  );

  const handleDelete = useCallback(
    (providerId: string) => {
      const provider = providers.find((p) => p.id === providerId);
      if (!provider) {
        return;
      }
      void confirmDialog({
        title: "Delete LLM provider",
        message: `Remove "${provider.label || provider.id}"?`,
        confirmLabel: "Delete",
        cancelLabel: "Cancel",
        destructive: true,
      }).then((confirmed) => {
        if (!confirmed) {
          return;
        }
        void saveProviders(providers.filter((p) => p.id !== providerId));
      });
    },
    [providers, saveProviders],
  );

  const handleToggleEnabled = useCallback(
    (providerId: string, enabled: boolean) => {
      void saveProviders(
        providers.map((p) => (p.id === providerId ? { ...p, enabled } : p)),
      );
    },
    [providers, saveProviders],
  );

  const addButton = useMemo(
    () =>
      hasServer && isConnected ? (
        <Pressable
          onPress={handleAdd}
          hitSlop={8}
          style={settingsStyles.sectionHeaderLink}
          accessibilityRole="button"
          accessibilityLabel="Add LLM provider"
          testID="add-llm-provider-button"
        >
          <Plus size={theme.iconSize.sm} color={theme.colors.foregroundMuted} />
        </Pressable>
      ) : undefined,
    [hasServer, isConnected, handleAdd, theme.iconSize.sm, theme.colors.foregroundMuted],
  );

  return (
    <SettingsSection
      title="LLM Providers"
      trailing={addButton}
      testID="llm-providers-section"
      style={styles.sectionSpacing}
    >
      {!hasServer || !isConnected ? (
        <View style={EMPTY_CARD_STYLE}>
          <Text style={styles.emptyText}>Connect to this host to manage LLM providers</Text>
        </View>
      ) : null}
      {hasServer && isConnected && providers.length === 0 ? (
        <View style={EMPTY_CARD_STYLE}>
          <Text style={styles.emptyText}>No LLM providers configured</Text>
        </View>
      ) : null}
      {hasServer && isConnected && providers.length > 0 ? (
        <View style={settingsStyles.card}>
          {providers.map((provider, index) => (
            <LLMProviderRow
              key={provider.id}
              provider={provider}
              isFirst={index === 0}
              onEdit={setEditingProvider}
              onDelete={handleDelete}
              onToggleEnabled={handleToggleEnabled}
            />
          ))}
        </View>
      ) : null}
      <LLMProviderModal
        provider={editingProvider === null ? undefined : editingProvider ?? undefined}
        visible={isModalOpen}
        onSave={handleSave}
        onClose={handleClose}
      />
    </SettingsSection>
  );
}

async function confirmDialog(options: {
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  destructive?: boolean;
}): Promise<boolean> {
  const { confirmDialog: showConfirmDialog } = await import("@/utils/confirm-dialog");
  return showConfirmDialog(options);
}

const styles = StyleSheet.create((theme) => ({
  sectionSpacing: {
    marginBottom: theme.spacing[4],
  },
  emptyCard: {
    padding: theme.spacing[4],
    alignItems: "center",
  },
  emptyText: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  rowContent: {
    flex: 1,
    marginRight: theme.spacing[3],
  },
  rowHint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    marginTop: theme.spacing[1],
  },
  rowActions: {
    flexDirection: "row",
    alignItems: "center",
    gap: theme.spacing[3],
  },
  form: {
    gap: theme.spacing[4],
  },
  formInput: {
    flex: 1,
    fontSize: theme.fontSize.base,
    paddingVertical: theme.spacing[2],
    paddingHorizontal: theme.spacing[3],
    borderRadius: theme.borderRadius.md,
    backgroundColor: theme.colors.surface2,
    borderWidth: 1,
    borderColor: theme.colors.surface3,
  },
  disabledInput: {
    opacity: 0.5,
  },
  formActions: {
    flexDirection: "row",
    justifyContent: "flex-end",
    gap: theme.spacing[2],
    marginTop: theme.spacing[2],
  },
  fieldColumn: {
    flex: 1,
    gap: theme.spacing[1.5],
  },
  fieldLabel: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize.sm,
    fontWeight: theme.fontWeight.medium,
  },
  fieldHint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
  },
}));

const EMPTY_CARD_STYLE = [settingsStyles.card, styles.emptyCard];

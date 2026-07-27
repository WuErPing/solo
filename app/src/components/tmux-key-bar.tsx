import React, { useEffect, useMemo } from "react";
import { Pressable, ScrollView, Text, View } from "react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { ChevronUp, MoreHorizontal } from "lucide-react-native";
import { useTmuxKeyBarStore } from "@/stores/tmux-keybar-store";
import { parseContextOptions } from "@/utils/tmux-option-parser";

export interface TmuxKeyBarExtraButton {
  key: string;
  label?: string;
  icon?: React.ComponentType<{ size: number; color: string }>;
  onPress: () => void;
  variant?: "default" | "active";
  disabled?: boolean;
  testID?: string;
}

export interface TmuxKeyBarProps {
  onSendKey: (key: string) => void;
  content: string;
  extraButtons?: TmuxKeyBarExtraButton[];
  testIDPrefix?: string;
}

const ANIMATION_DURATION = 220;
const ANIMATION_EASING = Easing.bezier(0.25, 0.1, 0.25, 1);
const EXPANDED_ROW_HEIGHT = 40;
const CONTEXT_STRIP_HEIGHT = 36;

const PRIMARY_KEYS: { label: string; key: string }[] = [
  { label: "↑", key: "Up" },
  { label: "↓", key: "Down" },
];

const ACTION_KEYS: { label: string; key: string }[] = [
  { label: "Enter", key: "Enter" },
  { label: "Esc", key: "Escape" },
  { label: "^C", key: "C-c" },
];

const EXPANDED_KEYS: { label: string; key: string }[] = [
  { label: "Tab", key: "Tab" },
  { label: "S-Tab", key: "BTab" },
  { label: "←", key: "Left" },
  { label: "→", key: "Right" },
  { label: "/", key: "/" },
  { label: "1", key: "1" },
  { label: "2", key: "2" },
  { label: "3", key: "3" },
  { label: "4", key: "4" },
  { label: "Home", key: "Home" },
];

export function TmuxKeyBar({ onSendKey, content, extraButtons, testIDPrefix = "tmux" }: TmuxKeyBarProps) {
  const { theme } = useUnistyles();
  const expanded = useTmuxKeyBarStore((s) => s.expanded);
  const toggleExpanded = useTmuxKeyBarStore((s) => s.toggleExpanded);

  const contextOptions = useMemo(() => parseContextOptions(content), [content]);
  const stripVisible = contextOptions.length > 0;

  const expandProgress = useSharedValue(expanded ? 1 : 0);
  const stripProgress = useSharedValue(stripVisible ? 1 : 0);

  useEffect(() => {
    expandProgress.value = withTiming(expanded ? 1 : 0, {
      duration: ANIMATION_DURATION,
      easing: ANIMATION_EASING,
    });
  }, [expanded, expandProgress]);

  useEffect(() => {
    stripProgress.value = withTiming(stripVisible ? 1 : 0, {
      duration: 150,
      easing: ANIMATION_EASING,
    });
  }, [stripVisible, stripProgress]);

  const expandedRowStyle = useAnimatedStyle(() => ({
    height: expandProgress.value * EXPANDED_ROW_HEIGHT,
    opacity: expandProgress.value,
    overflow: "hidden" as const,
  }));

  const stripStyle = useAnimatedStyle(() => ({
    height: stripProgress.value * CONTEXT_STRIP_HEIGHT,
    opacity: stripProgress.value,
    transform: [{ translateY: (1 - stripProgress.value) * -8 }],
    overflow: "hidden" as const,
  }));

  const moreIconStyle = useAnimatedStyle(() => ({
    transform: [{ rotate: `${expandProgress.value * 180}deg` }],
  }));

  const expandedKeys = stripVisible
    ? EXPANDED_KEYS.filter((k) => !["1", "2", "3", "4"].includes(k.key))
    : EXPANDED_KEYS;

  return (
    <View style={styles.container}>
      <Animated.View style={stripStyle}>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={styles.stripContent}
        >
          {contextOptions.map((option) => (
            <Pressable
              key={option.digit}
              testID={`${testIDPrefix}-option-${option.digit}`}
              onPress={() => onSendKey(option.digit)}
              android_ripple={{ color: theme.colors.surface2 }}
              style={({ pressed }) => [
                styles.optionChip,
                {
                  backgroundColor: theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
                pressed ? { backgroundColor: theme.colors.surface2 } : null,
              ]}
            >
              <View style={[styles.optionDigitBadge, { backgroundColor: theme.colors.primary }]}>
                <Text style={[styles.optionDigitText, { color: theme.colors.background }]}>
                  {option.digit}
                </Text>
              </View>
              <Text style={[styles.optionLabel, { color: theme.colors.foreground }]}>
                {option.label}
              </Text>
            </Pressable>
          ))}
        </ScrollView>
      </Animated.View>

      <Animated.View style={expandedRowStyle}>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={[styles.expandedRowContent, { backgroundColor: theme.colors.surface0 }]}
        >
          {expandedKeys.map(({ label, key }) => (
            <Pressable
              key={key}
              testID={`${testIDPrefix}-key-${key}`}
              onPress={() => onSendKey(key)}
              android_ripple={{ color: theme.colors.surface2 }}
              style={({ pressed }) => [
                styles.expandedKeyButton,
                {
                  backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
              ]}
            >
              <Text style={[styles.expandedKeyLabel, { color: theme.colors.foreground }]}>
                {label}
              </Text>
            </Pressable>
          ))}
        </ScrollView>
      </Animated.View>

      <View style={styles.primaryRow}>
        {PRIMARY_KEYS.map(({ label, key }) => (
          <Pressable
            key={key}
            testID={`${testIDPrefix}-key-${key}`}
            onPress={() => onSendKey(key)}
            android_ripple={{ color: theme.colors.surface2 }}
            style={({ pressed }) => [
              styles.primaryKeyButton,
              {
                backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                borderColor: theme.colors.border,
              },
            ]}
          >
            <Text style={[styles.primaryKeyLabel, { color: theme.colors.foreground }]}>
              {label}
            </Text>
          </Pressable>
        ))}

        <View style={[styles.divider, { backgroundColor: theme.colors.border }]} />

        {ACTION_KEYS.map(({ label, key }) => {
          if (key === "Enter") {
            return (
              <Pressable
                key={key}
                testID={`${testIDPrefix}-key-${key}`}
                onPress={() => onSendKey(key)}
                android_ripple={{ color: theme.colors.surface2 }}
                style={({ pressed }) => [
                  styles.primaryKeyButton,
                  styles.enterKeyButton,
                  { backgroundColor: theme.colors.primary },
                  pressed ? { opacity: 0.85 } : null,
                ]}
              >
                <Text style={[styles.primaryKeyLabel, { color: theme.colors.background }]}>
                  {label}
                </Text>
              </Pressable>
            );
          }
          if (key === "C-c") {
            return (
              <Pressable
                key={key}
                testID={`${testIDPrefix}-key-${key}`}
                onPress={() => onSendKey(key)}
                android_ripple={{ color: theme.colors.surface2 }}
                style={({ pressed }) => [
                  styles.primaryKeyButton,
                  {
                    backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                    borderColor: theme.colors.border,
                  },
                ]}
              >
                <Text style={[styles.primaryKeyLabel, { color: theme.colors.destructive }]}>
                  {label}
                </Text>
              </Pressable>
            );
          }
          return (
            <Pressable
              key={key}
              testID={`${testIDPrefix}-key-${key}`}
              onPress={() => onSendKey(key)}
              android_ripple={{ color: theme.colors.surface2 }}
              style={({ pressed }) => [
                styles.primaryKeyButton,
                {
                  backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
              ]}
            >
              <Text style={[styles.primaryKeyLabel, { color: theme.colors.foreground }]}>
                {label}
              </Text>
            </Pressable>
          );
        })}

        <View style={styles.primaryRowSpacer} />

        {extraButtons?.map((button) => {
          const Icon = button.icon;
          const isActive = button.variant === "active";
          return (
            <Pressable
              key={button.key}
              testID={button.testID}
              onPress={button.onPress}
              disabled={button.disabled}
              android_ripple={{ color: theme.colors.surface2 }}
              style={({ pressed }) => [
                styles.primaryKeyButton,
                {
                  backgroundColor: isActive
                    ? theme.colors.primary
                    : pressed
                      ? theme.colors.surface2
                      : theme.colors.surface1,
                  borderColor: theme.colors.border,
                },
                button.disabled ? { opacity: 0.4 } : null,
              ]}
            >
              {Icon ? (
                <Icon
                  size={14}
                  color={isActive ? theme.colors.background : theme.colors.foreground}
                />
              ) : (
                <Text
                  style={[
                    styles.primaryKeyLabel,
                    {
                      color: isActive
                        ? theme.colors.background
                        : button.key === "refresh"
                          ? theme.colors.primary
                          : theme.colors.foreground,
                    },
                  ]}
                >
                  {button.label}
                </Text>
              )}
            </Pressable>
          );
        })}

        <Pressable
          testID={`${testIDPrefix}-expand-toggle`}
          onPress={toggleExpanded}
          android_ripple={{ color: theme.colors.surface2 }}
          style={({ pressed }) => [
            styles.primaryKeyButton,
            {
              backgroundColor: pressed ? theme.colors.surface2 : theme.colors.surface1,
              borderColor: theme.colors.border,
            },
          ]}
        >
          <Animated.View style={moreIconStyle}>
            {expanded ? (
              <ChevronUp size={14} color={theme.colors.foreground} />
            ) : (
              <MoreHorizontal size={14} color={theme.colors.foreground} />
            )}
          </Animated.View>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    borderTopWidth: 1,
    borderTopColor: theme.colors.border,
  },
  stripContent: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    paddingHorizontal: 12,
    height: CONTEXT_STRIP_HEIGHT,
  },
  optionChip: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    borderRadius: 8,
    borderWidth: 1,
    paddingHorizontal: 8,
    paddingVertical: 4,
  },
  optionDigitBadge: {
    width: 16,
    height: 16,
    borderRadius: 8,
    alignItems: "center",
    justifyContent: "center",
  },
  optionDigitText: {
    fontSize: 10,
    fontWeight: "600",
  },
  optionLabel: {
    fontSize: 12,
    fontWeight: "500",
  },
  expandedRowContent: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 12,
    height: EXPANDED_ROW_HEIGHT,
  },
  expandedKeyButton: {
    borderWidth: 1,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
    minWidth: 34,
    alignItems: "center",
    justifyContent: "center",
  },
  expandedKeyLabel: {
    fontSize: 11,
    fontWeight: "500",
  },
  primaryRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 12,
    paddingVertical: 6,
  },
  primaryKeyButton: {
    borderWidth: 1,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 6,
    minWidth: 36,
    height: 32,
    alignItems: "center",
    justifyContent: "center",
  },
  enterKeyButton: {
    minWidth: 56,
    borderWidth: 0,
  },
  primaryKeyLabel: {
    fontSize: 12,
    fontWeight: "500",
  },
  divider: {
    width: 1,
    alignSelf: "stretch",
    marginVertical: 4,
    marginHorizontal: 2,
  },
  primaryRowSpacer: {
    flex: 1,
  },
}));

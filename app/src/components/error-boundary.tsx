import React from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";
import { useUnistyles } from "react-native-unistyles";

interface Props {
  children: React.ReactNode;
  fallbackLabel?: string;
}

interface State {
  hasError: boolean;
  error?: Error;
}

export class ErrorBoundary extends React.Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    console.error("[ErrorBoundary]", error, info.componentStack);
    this.setState({ error });
  }

  private handleRetry = () => {
    this.setState({ hasError: false, error: undefined });
  };

  render() {
    if (this.state.hasError) {
      return (
        <ErrorFallback
          label={this.props.fallbackLabel}
          error={this.state.error}
          onRetry={this.handleRetry}
        />
      );
    }
    return this.props.children;
  }
}

function ErrorFallback({
  label,
  error,
  onRetry,
}: {
  label?: string;
  error?: Error;
  onRetry: () => void;
}) {
  const { theme } = useUnistyles();

  return (
    <View style={[styles.container, { backgroundColor: theme.colors.background }]}>
      <Text style={[styles.text, { color: theme.colors.foregroundMuted }]}>
        {label ?? "Something went wrong"}
      </Text>
      {error ? (
        <Text style={[styles.errorText, { color: theme.colors.destructive }]}>
          {error.message}
        </Text>
      ) : null}
      <Pressable
        onPress={onRetry}
        style={[styles.button, { backgroundColor: theme.colors.surface0 }]}
      >
        <Text style={[styles.buttonText, { color: theme.colors.foreground }]}>Retry</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    justifyContent: "center",
    alignItems: "center",
    padding: 24,
    gap: 12,
  },
  text: {
    fontSize: 14,
    textAlign: "center",
  },
  button: {
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: 8,
  },
  buttonText: {
    fontSize: 14,
    fontWeight: "500",
  },
  errorText: {
    fontSize: 12,
    textAlign: "center",
    paddingHorizontal: 12,
  },
});

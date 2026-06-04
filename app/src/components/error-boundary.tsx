import React from "react";
import { View, Text, Pressable, StyleSheet } from "react-native";
import { useUnistyles } from "react-native-unistyles";

interface Props {
  children: React.ReactNode;
  fallbackLabel?: string;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends React.Component<Props, State> {
  state: State = { hasError: false };

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo): void {
    console.error("[ErrorBoundary]", error, info.componentStack);
  }

  private handleRetry = () => {
    this.setState({ hasError: false });
  };

  render() {
    if (this.state.hasError) {
      return <ErrorFallback label={this.props.fallbackLabel} onRetry={this.handleRetry} />;
    }
    return this.props.children;
  }
}

function ErrorFallback({
  label,
  onRetry,
}: {
  label?: string;
  onRetry: () => void;
}) {
  const { theme } = useUnistyles();

  return (
    <View style={[styles.container, { backgroundColor: theme.colors.background }]}>
      <Text style={[styles.text, { color: theme.colors.foregroundMuted }]}>
        {label ?? "Something went wrong"}
      </Text>
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
});

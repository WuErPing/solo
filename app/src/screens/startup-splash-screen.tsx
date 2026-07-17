import { useCallback, useEffect, useMemo, useState } from "react";
import { ScrollView, Text, View, type ViewStyle } from "react-native";
import Animated, {
  cancelAnimation,
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import MaskedView from "@react-native-masked-view/masked-view";
import Svg, { Defs, LinearGradient as SvgLinearGradient, Rect, Stop } from "react-native-svg";
import * as Clipboard from "expo-clipboard";
import { openExternalUrl } from "@/utils/open-external-url";
import { BookOpen, Copy, RotateCw, TriangleAlert } from "lucide-react-native";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { SoloLogo } from "@/components/icons/solo-logo";
import { Button } from "@/components/ui/button";
import { Fonts } from "@/constants/theme";
import { getDesktopDaemonLogs, type DesktopDaemonLogs } from "@/desktop/daemon/desktop-daemon";
import { TitlebarDragRegion } from "@/components/desktop/titlebar-drag-region";
import { isNative, isWeb } from "@/constants/platform";
import { useWebScrollbarStyle } from "@/hooks/use-web-scrollbar-style";

interface StartupSplashScreenProps {
  bootstrapState?: {
    phase: "starting-daemon" | "connecting" | "online" | "error";
    error: string | null;
    retry: () => void;
  };
}

const GITHUB_ISSUE_URL = "https://github.com/getsolo/solo/issues/new";
const DOCS_URL = "https://solo.sh/docs";

const LOGO_SIZE = 96;
const SHIMMER_PEAK_WIDTH = 120;
const SHIMMER_DURATION_MS = 1800;

// Plain style object — must NOT be a Unistyles style: on web those carry
// metadata that Reanimated's style validator rejects on Animated.View.
const NATIVE_SHIMMER_PEAK_BASE_STYLE: ViewStyle = {
  position: "absolute",
  top: 0,
  bottom: 0,
};

function openGithubIssue(): void {
  void openExternalUrl(GITHUB_ISSUE_URL);
}

function openDocs(): void {
  void openExternalUrl(DOCS_URL);
}

const WEB_SPLASH_SHIMMER_KEYFRAME_ID = "solo-splash-shimmer-keyframes";
const WEB_SPLASH_SHIMMER_ANIMATION_NAME = "solo-splash-shimmer";

const WEB_SPLASH_SHIMMER_KEYFRAME_CSS = `
  @keyframes ${WEB_SPLASH_SHIMMER_ANIMATION_NAME} {
    0% {
      background-position: -${LOGO_SIZE + SHIMMER_PEAK_WIDTH}px 0;
    }
    100% {
      background-position: ${LOGO_SIZE + SHIMMER_PEAK_WIDTH}px 0;
    }
  }
`;

let webSplashShimmerRegistered = false;

function ensureWebSplashShimmerKeyframes() {
  if (isNative) {
    return;
  }
  if (webSplashShimmerRegistered) {
    return;
  }
  const existing = document.getElementById(WEB_SPLASH_SHIMMER_KEYFRAME_ID);
  if (existing) {
    webSplashShimmerRegistered = true;
    return;
  }
  const styleElement = document.createElement("style");
  styleElement.id = WEB_SPLASH_SHIMMER_KEYFRAME_ID;
  styleElement.textContent = WEB_SPLASH_SHIMMER_KEYFRAME_CSS;
  document.head.appendChild(styleElement);
  webSplashShimmerRegistered = true;
}

function LogoShimmer() {
  const { theme } = useUnistyles();

  if (isWeb) {
    return <WebLogoShimmer color={theme.colors.foreground} />;
  }

  return <NativeLogoShimmer color={theme.colors.foreground} />;
}

function WebLogoShimmer({ color }: { color: string }) {
  useEffect(() => {
    ensureWebSplashShimmerKeyframes();
  }, []);

  const shimmerStyle = useMemo(
    () => ({
      width: LOGO_SIZE,
      height: LOGO_SIZE,
      WebkitMaskImage: `url("/solo-logo-mask.png")`,
      WebkitMaskSize: "contain",
      WebkitMaskRepeat: "no-repeat",
      WebkitMaskPosition: "center",
      background: `linear-gradient(90deg, ${color} 0%, ${color}88 40%, ${color}FF 50%, ${color}88 60%, ${color} 100%)`,
      backgroundSize: `${LOGO_SIZE + SHIMMER_PEAK_WIDTH * 2}px ${LOGO_SIZE}px`,
      animationName: WEB_SPLASH_SHIMMER_ANIMATION_NAME,
      animationDuration: `${SHIMMER_DURATION_MS}ms`,
      animationTimingFunction: "linear",
      animationIterationCount: "infinite",
    }),
    [color],
  );

  return <View style={shimmerStyle as never} />;
}

function NativeLogoShimmer({ color }: { color: string }) {
  const shimmerTranslateX = useSharedValue(-SHIMMER_PEAK_WIDTH);

  useEffect(() => {
    shimmerTranslateX.value = -SHIMMER_PEAK_WIDTH;
    shimmerTranslateX.value = withRepeat(
      withTiming(LOGO_SIZE + SHIMMER_PEAK_WIDTH, {
        duration: SHIMMER_DURATION_MS,
        easing: Easing.linear,
      }),
      -1,
      false,
    );
    return () => {
      cancelAnimation(shimmerTranslateX);
    };
  }, [shimmerTranslateX]);

  const peakStyle = useAnimatedStyle(() => ({
    transform: [{ translateX: shimmerTranslateX.value }],
  }));

  const trackStyle = useMemo(
    () => [styles.nativeShimmerTrack, { width: LOGO_SIZE, height: LOGO_SIZE }],
    [],
  );

  const peakCombinedStyle = useMemo(
    () => [
      NATIVE_SHIMMER_PEAK_BASE_STYLE,
      peakStyle,
      { width: SHIMMER_PEAK_WIDTH, height: LOGO_SIZE },
    ],
    [peakStyle],
  );

  const maskElement = useMemo(
    () => (
      <View style={styles.shimmerMask}>
        <SoloLogo size={LOGO_SIZE} color="#000000" />
      </View>
    ),
    [],
  );

  return (
    <MaskedView style={trackStyle} maskElement={maskElement}>
      <View style={trackStyle}>
        <View style={styles.nativeShimmerBase}>
          <SoloLogo size={LOGO_SIZE} color={color} />
        </View>
        <Animated.View style={peakCombinedStyle}>
          <Svg width="100%" height="100%" preserveAspectRatio="none">
            <Defs>
              <SvgLinearGradient id="splashShimmer" x1="0" y1="0" x2="1" y2="0">
                <Stop offset="0" stopColor="#FFFFFF" stopOpacity="0" />
                <Stop offset="0.5" stopColor="#FFFFFF" stopOpacity="0.4" />
                <Stop offset="1" stopColor="#FFFFFF" stopOpacity="0" />
              </SvgLinearGradient>
            </Defs>
            <Rect x="0" y="0" width="100%" height="100%" fill="url(#splashShimmer)" />
          </Svg>
        </Animated.View>
      </View>
    </MaskedView>
  );
}

const styles = StyleSheet.create((theme) => ({
  container: {
    position: "relative",
    flex: 1,
    alignItems: "center",
    justifyContent: "center",
    backgroundColor: theme.colors.surface0,
    paddingHorizontal: theme.spacing[8],
    paddingVertical: theme.spacing[8],
  },
  errorScreen: {
    position: "relative",
    flex: 1,
    backgroundColor: theme.colors.surface0,
  },
  errorScrollView: {
    flex: 1,
    ...(isWeb
      ? {
          overflowX: "auto",
          overflowY: "auto",
          WebkitAppRegion: "no-drag",
        }
      : null),
  },
  errorScrollContent: {
    flexGrow: 1,
    alignItems: "center",
    justifyContent: "flex-start",
    paddingHorizontal: theme.spacing[8],
    paddingVertical: theme.spacing[8],
    paddingTop: theme.spacing[16],
  },
  errorContent: {
    alignItems: "stretch",
    maxWidth: 720,
    width: "100%",
    gap: theme.spacing[6],
  },
  errorHeader: {
    alignItems: "flex-start",
  },
  title: {
    color: theme.colors.foreground,
    fontSize: theme.fontSize["3xl"],
    fontWeight: theme.fontWeight.semibold,
    textAlign: "left",
  },
  errorDescription: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.base,
    lineHeight: 22,
  },
  errorMessage: {
    color: theme.colors.destructive,
    fontSize: theme.fontSize.sm,
    lineHeight: 20,
    fontFamily: Fonts.mono,
  },
  logsMeta: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
  },
  logsContainer: {
    height: 200,
    borderRadius: theme.borderRadius.xl,
    backgroundColor: theme.colors.surface1,
    borderWidth: theme.borderWidth[1],
    borderColor: theme.colors.border,
    overflow: "hidden",
  },
  logsScroll: {
    flexGrow: 0,
  },
  logsContent: {
    padding: theme.spacing[4],
  },
  logsText: {
    fontFamily: Fonts.mono,
    fontSize: theme.fontSize.xs,
    color: theme.colors.foreground,
    lineHeight: 18,
    ...(isWeb
      ? {
          whiteSpace: "pre",
          overflowWrap: "normal",
        }
      : null),
  },
  actionRow: {
    flexDirection: "row",
    gap: theme.spacing[3],
    flexWrap: "wrap",
  },
  shimmerMask: {
    width: LOGO_SIZE,
    height: LOGO_SIZE,
    alignItems: "center",
    justifyContent: "center",
  },
  nativeShimmerTrack: {
    overflow: "hidden",
  },
  nativeShimmerBase: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
  },
}));

export function StartupSplashScreen({ bootstrapState }: StartupSplashScreenProps) {
  const { theme } = useUnistyles();
  const webScrollbarStyle = useWebScrollbarStyle();
  const errorScrollViewStyle = useMemo(
    () => [styles.errorScrollView, webScrollbarStyle],
    [webScrollbarStyle],
  );
  const logsScrollStyle = useMemo(
    () => [styles.logsScroll, webScrollbarStyle],
    [webScrollbarStyle],
  );
  const [daemonLogs, setDaemonLogs] = useState<DesktopDaemonLogs | null>(null);
  const [logsError, setLogsError] = useState<string | null>(null);
  const [isLoadingLogs, setIsLoadingLogs] = useState(false);

  const isError = bootstrapState?.phase === "error";

  useEffect(() => {
    if (!isError) {
      setDaemonLogs(null);
      setLogsError(null);
      setIsLoadingLogs(false);
      return;
    }

    let isCancelled = false;
    setIsLoadingLogs(true);
    setLogsError(null);

    void getDesktopDaemonLogs()
      .then((logs) => {
        if (isCancelled) {
          return;
        }
        setDaemonLogs(logs);
        return;
      })
      .catch((error) => {
        if (isCancelled) {
          return;
        }
        const message = error instanceof Error ? error.message : String(error);
        setDaemonLogs(null);
        setLogsError(`Unable to load daemon logs: ${message}`);
      })
      .finally(() => {
        if (!isCancelled) {
          setIsLoadingLogs(false);
        }
      });

    return () => {
      isCancelled = true;
    };
  }, [isError]);

  const logsText = useMemo(() => {
    if (isLoadingLogs) {
      return "Loading daemon logs...";
    }
    if (daemonLogs?.contents) {
      return daemonLogs.contents;
    }
    if (logsError) {
      return logsError;
    }
    return "No daemon logs available.";
  }, [daemonLogs?.contents, isLoadingLogs, logsError]);

  const handleCopyLogs = useCallback(() => {
    const payload = daemonLogs?.logPath
      ? `${daemonLogs.logPath}\n\n${daemonLogs.contents}`
      : logsText;
    void Clipboard.setStringAsync(payload);
  }, [daemonLogs?.logPath, daemonLogs?.contents, logsText]);

  const copyIcon = useMemo(
    () => <Copy size={16} color={theme.colors.foreground} />,
    [theme.colors.foreground],
  );
  const warningIcon = useMemo(
    () => <TriangleAlert size={16} color={theme.colors.foreground} />,
    [theme.colors.foreground],
  );
  const bookIcon = useMemo(
    () => <BookOpen size={16} color={theme.colors.foreground} />,
    [theme.colors.foreground],
  );
  const retryIcon = useMemo(
    () => <RotateCw size={16} color={theme.colors.palette.white} />,
    [theme.colors.palette.white],
  );

  if (!isError) {
    return (
      <View style={styles.container} testID="startup-splash-container">
        <TitlebarDragRegion />
        <LogoShimmer />
      </View>
    );
  }

  return (
    <View style={styles.errorScreen}>
      <TitlebarDragRegion />
      <ScrollView
        style={errorScrollViewStyle}
        contentContainerStyle={styles.errorScrollContent}
        showsVerticalScrollIndicator
      >
        <View style={styles.errorContent}>
          <View style={styles.errorHeader}>
            <SoloLogo size={64} />
            <Text style={styles.title}>Something went wrong</Text>
          </View>

          <Text style={styles.errorDescription}>
            The local server failed to start. If this keeps happening, please report the issue on
            GitHub and include the logs below.
          </Text>

          <Text style={styles.errorMessage}>{bootstrapState.error}</Text>

          {daemonLogs?.logPath ? <Text style={styles.logsMeta}>{daemonLogs.logPath}</Text> : null}

          <View style={styles.logsContainer}>
            <ScrollView
              style={logsScrollStyle}
              contentContainerStyle={styles.logsContent}
              showsVerticalScrollIndicator
            >
              <Text selectable style={styles.logsText}>
                {logsText}
              </Text>
            </ScrollView>
          </View>

          <View style={styles.actionRow}>
            <Button variant="secondary" leftIcon={copyIcon} onPress={handleCopyLogs}>
              Copy logs
            </Button>
            <Button variant="outline" leftIcon={warningIcon} onPress={openGithubIssue}>
              Open GitHub issue
            </Button>
            <Button variant="outline" leftIcon={bookIcon} onPress={openDocs}>
              Docs
            </Button>
            <Button variant="default" leftIcon={retryIcon} onPress={bootstrapState.retry}>
              Retry
            </Button>
          </View>
        </View>
      </ScrollView>
    </View>
  );
}

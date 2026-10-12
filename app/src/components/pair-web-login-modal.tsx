import { useEffect, useMemo, useState } from "react";
import { ActivityIndicator, Image, Text, View } from "react-native";
import * as QRCode from "qrcode";
import { StyleSheet, useUnistyles } from "react-native-unistyles";
import { RotateCw } from "lucide-react-native";
import { AdaptiveModalSheet } from "./adaptive-modal-sheet";
import { Button } from "@/components/ui/button";
import { usePairWebLogin, type PairWebLoginState } from "@/hooks/use-pair-web-login";
import { useNowMs } from "@/hooks/use-now";
import { formatPairWebCountdown } from "@/utils/pair-web";
import { Fonts } from "@/constants/theme";

const styles = StyleSheet.create((theme) => ({
  body: {
    alignItems: "center",
    gap: theme.spacing[4],
    paddingVertical: theme.spacing[2],
  },
  qrFrame: {
    width: 260,
    height: 260,
    borderRadius: theme.borderRadius.lg,
    borderWidth: 1,
    borderColor: theme.colors.border,
    backgroundColor: theme.colors.palette.white,
    padding: theme.spacing[2],
    alignItems: "center",
    justifyContent: "center",
  },
  qrFrameDimmed: {
    opacity: theme.opacity[50],
  },
  qrImage: {
    width: "100%",
    height: "100%",
  },
  hint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
  countdown: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    textAlign: "center",
  },
  fingerprint: {
    color: theme.colors.foregroundMuted,
    fontSize: theme.fontSize.xs,
    fontFamily: Fonts.mono,
    textAlign: "center",
  },
  error: {
    color: theme.colors.destructive,
    fontSize: theme.fontSize.sm,
    textAlign: "center",
  },
  actions: {
    flexDirection: "row",
    gap: theme.spacing[3],
  },
}));

export interface PairWebLoginModalProps {
  visible: boolean;
  onClose: () => void;
  onPaired: (serverId: string) => void;
}

export function PairWebLoginModal({ visible, onClose, onPaired }: PairWebLoginModalProps) {
  if (!visible) {
    return null;
  }
  return <PairWebLoginModalContent onClose={onClose} onPaired={onPaired} />;
}

function PairWebLoginModalContent({
  onClose,
  onPaired,
}: Omit<PairWebLoginModalProps, "visible">) {
  const { theme } = useUnistyles();
  const login = usePairWebLogin({ active: true, onPaired });
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null);
  const nowMs = useNowMs(1000);

  useEffect(() => {
    if (!login.qrContent) {
      return;
    }
    let cancelled = false;
    void QRCode.toDataURL(login.qrContent, { errorCorrectionLevel: "M", margin: 1, width: 520 })
      .then((url) => {
        if (!cancelled) {
          setQrDataUrl(url);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [login.qrContent]);

  const secondsLeft = useMemo(
    () =>
      login.expiresAtMs === null ? null : Math.max(0, Math.ceil((login.expiresAtMs - nowMs) / 1000)),
    [login.expiresAtMs, nowMs],
  );

  const regenerateIcon = useMemo(
    () => <RotateCw size={14} color={theme.colors.palette.white} />,
    [theme.colors.palette.white],
  );

  return (
    <AdaptiveModalSheet
      title="Sign in with QR code"
      visible
      onClose={onClose}
      testID="pair-web-login-modal"
      desktopMaxWidth={420}
    >
      <PairWebLoginBody
        login={login}
        qrDataUrl={qrDataUrl}
        secondsLeft={secondsLeft}
        regenerateIcon={regenerateIcon}
      />
    </AdaptiveModalSheet>
  );
}

interface PairWebLoginBodyProps {
  login: PairWebLoginState;
  qrDataUrl: string | null;
  secondsLeft: number | null;
  regenerateIcon: React.ReactElement;
}

function PairWebLoginBody({ login, qrDataUrl, secondsLeft, regenerateIcon }: PairWebLoginBodyProps) {
  if (login.status === "error") {
    return (
      <View style={styles.body}>
        <Text style={styles.error}>{login.errorMessage ?? "QR sign-in failed."}</Text>
        <View style={styles.actions}>
          <Button
            variant="default"
            size="sm"
            leftIcon={regenerateIcon}
            onPress={login.restart}
            testID="pair-web-login-retry"
          >
            Try again
          </Button>
        </View>
      </View>
    );
  }

  if (login.status === "expired") {
    return (
      <View style={styles.body}>
        <QrPreview qrDataUrl={qrDataUrl} dimmed />
        <Text style={styles.error}>This QR code expired.</Text>
        <View style={styles.actions}>
          <Button
            variant="default"
            size="sm"
            leftIcon={regenerateIcon}
            onPress={login.restart}
            testID="pair-web-login-regenerate"
          >
            Generate a new code
          </Button>
        </View>
      </View>
    );
  }

  if (login.status === "verifying" || login.status === "success") {
    return (
      <View style={styles.body}>
        <ActivityIndicator size="small" />
        <Text style={styles.hint}>
          {login.status === "verifying"
            ? "Approved — connecting to your computer…"
            : "Connected. Loading your workspace…"}
        </Text>
      </View>
    );
  }

  if (login.status !== "waiting") {
    return (
      <View style={styles.body}>
        <ActivityIndicator size="small" />
        <Text style={styles.hint}>Preparing a secure channel…</Text>
      </View>
    );
  }

  return (
    <View style={styles.body}>
      <QrPreview qrDataUrl={qrDataUrl} dimmed={false} />
      <Text style={styles.hint}>Open Solo on your phone and scan this code to sign in.</Text>
      {secondsLeft !== null ? (
        <Text style={styles.countdown} testID="pair-web-login-countdown">
          Expires in {formatPairWebCountdown(secondsLeft)}
        </Text>
      ) : null}
      {login.fingerprint ? (
        <Text style={styles.fingerprint} testID="pair-web-login-fingerprint">
          {`Key fingerprint  ${login.fingerprint}`}
        </Text>
      ) : null}
    </View>
  );
}

function QrPreview({ qrDataUrl, dimmed }: { qrDataUrl: string | null; dimmed: boolean }) {
  const frameStyle = useMemo(
    () => [styles.qrFrame, dimmed ? styles.qrFrameDimmed : null],
    [dimmed],
  );
  return (
    <View style={frameStyle}>
      {qrDataUrl ? (
        <Image source={{ uri: qrDataUrl }} style={styles.qrImage} resizeMode="contain" />
      ) : (
        <ActivityIndicator size="small" />
      )}
    </View>
  );
}
